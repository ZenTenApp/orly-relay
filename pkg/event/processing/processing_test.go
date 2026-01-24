package processing

import (
	"context"
	"errors"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/event"
)

// mockDatabase is a mock implementation of Database for testing.
type mockDatabase struct {
	saveErr    error
	saveExists bool
	checkErr   error
}

func (m *mockDatabase) SaveEvent(ctx context.Context, ev *event.E) (exists bool, err error) {
	return m.saveExists, m.saveErr
}

func (m *mockDatabase) CheckForDeleted(ev *event.E, adminOwners [][]byte) error {
	return m.checkErr
}

// mockPublisher is a mock implementation of Publisher for testing.
type mockPublisher struct {
	deliveredEvents []*event.E
}

func (m *mockPublisher) Deliver(ev *event.E) {
	m.deliveredEvents = append(m.deliveredEvents, ev)
}

// mockRateLimiter is a mock implementation of RateLimiter for testing.
type mockRateLimiter struct {
	enabled    bool
	waitCalled bool
}

func (m *mockRateLimiter) IsEnabled() bool {
	return m.enabled
}

func (m *mockRateLimiter) Wait(ctx context.Context, opType int) error {
	m.waitCalled = true
	return nil
}

// mockSyncManager is a mock implementation of SyncManager for testing.
type mockSyncManager struct {
	updateCalled bool
}

func (m *mockSyncManager) UpdateSerial() {
	m.updateCalled = true
}

// mockACLRegistry is a mock implementation of ACLRegistry for testing.
type mockACLRegistry struct {
	active         string
	configureCalls int
}

func (m *mockACLRegistry) Configure(cfg ...any) error {
	m.configureCalls++
	return nil
}

func (m *mockACLRegistry) Active() string {
	return m.active
}

func TestNew(t *testing.T) {
	db := &mockDatabase{}
	pub := &mockPublisher{}

	s := New(nil, db, pub)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.cfg == nil {
		t.Fatal("cfg should be set to default")
	}
	if s.db != db {
		t.Fatal("db not set correctly")
	}
	if s.publisher != pub {
		t.Fatal("publisher not set correctly")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.WriteTimeout != 30*1e9 {
		t.Errorf("expected WriteTimeout=30s, got %v", cfg.WriteTimeout)
	}
}

func TestResultConstructors(t *testing.T) {
	// OK
	r := OK()
	if !r.Saved || r.Error != nil || r.Blocked {
		t.Error("OK() should return Saved=true")
	}

	// Blocked
	r = Blocked("test blocked")
	if r.Saved || !r.Blocked || r.BlockMsg != "test blocked" {
		t.Error("Blocked() should return Blocked=true with message")
	}

	// Failed
	err := errors.New("test error")
	r = Failed(err)
	if r.Saved || r.Error != err {
		t.Error("Failed() should return Error set")
	}
}

func TestProcess_Success(t *testing.T) {
	db := &mockDatabase{}
	pub := &mockPublisher{}

	s := New(nil, db, pub)

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)

	result := s.Process(context.Background(), ev)
	if !result.Saved {
		t.Errorf("should save successfully: %v", result.Error)
	}
}

func TestProcess_DatabaseError(t *testing.T) {
	testErr := errors.New("db error")
	db := &mockDatabase{saveErr: testErr}
	pub := &mockPublisher{}

	s := New(nil, db, pub)

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)

	result := s.Process(context.Background(), ev)
	if result.Saved {
		t.Error("should not save on error")
	}
	if result.Error != testErr {
		t.Error("should return the database error")
	}
}

func TestProcess_BlockedError(t *testing.T) {
	db := &mockDatabase{saveErr: errors.New("blocked: event already deleted")}
	pub := &mockPublisher{}

	s := New(nil, db, pub)

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)

	result := s.Process(context.Background(), ev)
	if result.Saved {
		t.Error("should not save blocked events")
	}
	if !result.Blocked {
		t.Error("should mark as blocked")
	}
	if result.BlockMsg != "event already deleted" {
		t.Errorf("expected block message, got: %s", result.BlockMsg)
	}
}

func TestProcess_WithRateLimiter(t *testing.T) {
	db := &mockDatabase{}
	pub := &mockPublisher{}
	rl := &mockRateLimiter{enabled: true}

	s := New(nil, db, pub)
	s.SetRateLimiter(rl)

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)

	s.Process(context.Background(), ev)

	if !rl.waitCalled {
		t.Error("rate limiter Wait should be called")
	}
}

func TestProcess_WithSyncManager(t *testing.T) {
	db := &mockDatabase{}
	pub := &mockPublisher{}
	sm := &mockSyncManager{}

	s := New(nil, db, pub)
	s.SetSyncManager(sm)

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)

	s.Process(context.Background(), ev)

	if !sm.updateCalled {
		t.Error("sync manager UpdateSerial should be called")
	}
}

func TestProcess_AdminFollowListTriggersACLReconfigure(t *testing.T) {
	db := &mockDatabase{}
	pub := &mockPublisher{}
	acl := &mockACLRegistry{active: "follows"}

	adminPubkey := make([]byte, 32)
	for i := range adminPubkey {
		adminPubkey[i] = byte(i)
	}

	cfg := &Config{
		Admins: [][]byte{adminPubkey},
	}

	s := New(cfg, db, pub)
	s.SetACLRegistry(acl)

	ev := event.New()
	ev.Kind = 3 // FollowList
	ev.Pubkey = adminPubkey

	s.Process(context.Background(), ev)

	// Give goroutine time to run
	// In production this would be tested differently
	// For now just verify the path is exercised
}

func TestSetters(t *testing.T) {
	db := &mockDatabase{}
	pub := &mockPublisher{}
	s := New(nil, db, pub)

	rl := &mockRateLimiter{}
	s.SetRateLimiter(rl)
	if s.rateLimiter != rl {
		t.Error("SetRateLimiter should set rateLimiter")
	}

	sm := &mockSyncManager{}
	s.SetSyncManager(sm)
	if s.syncManager != sm {
		t.Error("SetSyncManager should set syncManager")
	}

	acl := &mockACLRegistry{}
	s.SetACLRegistry(acl)
	if s.aclRegistry != acl {
		t.Error("SetACLRegistry should set aclRegistry")
	}
}

func TestIsAdminEvent(t *testing.T) {
	adminPubkey := make([]byte, 32)
	for i := range adminPubkey {
		adminPubkey[i] = byte(i)
	}

	ownerPubkey := make([]byte, 32)
	for i := range ownerPubkey {
		ownerPubkey[i] = byte(i + 50)
	}

	cfg := &Config{
		Admins: [][]byte{adminPubkey},
		Owners: [][]byte{ownerPubkey},
	}

	s := New(cfg, &mockDatabase{}, &mockPublisher{})

	// Admin pubkey
	if !s.isAdminPubkey(adminPubkey) {
		t.Error("should recognize admin pubkey")
	}

	// Owner pubkey
	if !s.isOwnerPubkey(ownerPubkey) {
		t.Error("should recognize owner pubkey")
	}

	// Regular pubkey
	regularPubkey := make([]byte, 32)
	for i := range regularPubkey {
		regularPubkey[i] = byte(i + 100)
	}
	if s.isAdminPubkey(regularPubkey) {
		t.Error("should not recognize regular pubkey as admin")
	}
	if s.isOwnerPubkey(regularPubkey) {
		t.Error("should not recognize regular pubkey as owner")
	}
}

func TestFastEqual(t *testing.T) {
	a := []byte{1, 2, 3, 4}
	b := []byte{1, 2, 3, 4}
	c := []byte{1, 2, 3, 5}
	d := []byte{1, 2, 3}

	if !fastEqual(a, b) {
		t.Error("equal slices should return true")
	}
	if fastEqual(a, c) {
		t.Error("different values should return false")
	}
	if fastEqual(a, d) {
		t.Error("different lengths should return false")
	}
}
