package validation

import (
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

func TestNew(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.cfg == nil {
		t.Fatal("New() returned service with nil config")
	}
	if s.cfg.MaxFutureSeconds != 3600 {
		t.Errorf("expected MaxFutureSeconds=3600, got %d", s.cfg.MaxFutureSeconds)
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := &Config{MaxFutureSeconds: 7200}
	s := NewWithConfig(cfg)
	if s.cfg.MaxFutureSeconds != 7200 {
		t.Errorf("expected MaxFutureSeconds=7200, got %d", s.cfg.MaxFutureSeconds)
	}

	// Test nil config defaults
	s = NewWithConfig(nil)
	if s.cfg.MaxFutureSeconds != 3600 {
		t.Errorf("expected default MaxFutureSeconds=3600, got %d", s.cfg.MaxFutureSeconds)
	}
}

func TestResultConstructors(t *testing.T) {
	// Test OK
	r := OK()
	if !r.Valid || r.Code != ReasonNone || r.Msg != "" {
		t.Error("OK() should return Valid=true with no code/msg")
	}

	// Test Blocked
	r = Blocked("test blocked")
	if r.Valid || r.Code != ReasonBlocked || r.Msg != "test blocked" {
		t.Error("Blocked() should return Valid=false with ReasonBlocked")
	}

	// Test Invalid
	r = Invalid("test invalid")
	if r.Valid || r.Code != ReasonInvalid || r.Msg != "test invalid" {
		t.Error("Invalid() should return Valid=false with ReasonInvalid")
	}

	// Test Error
	r = Error("test error")
	if r.Valid || r.Code != ReasonError || r.Msg != "test error" {
		t.Error("Error() should return Valid=false with ReasonError")
	}
}

func TestValidateRawJSON_LowercaseHex(t *testing.T) {
	s := New()

	// Valid lowercase hex
	validJSON := []byte(`["EVENT",{"id":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","created_at":1234567890,"kind":1,"tags":[],"content":"test","sig":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}]`)

	result := s.ValidateRawJSON(validJSON)
	if !result.Valid {
		t.Errorf("valid lowercase JSON should pass: %s", result.Msg)
	}

	// Invalid - uppercase in id
	invalidID := []byte(`["EVENT",{"id":"ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789","pubkey":"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210","created_at":1234567890,"kind":1,"tags":[],"content":"test","sig":"abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}]`)

	result = s.ValidateRawJSON(invalidID)
	if result.Valid {
		t.Error("uppercase in id should fail validation")
	}
	if result.Code != ReasonBlocked {
		t.Error("uppercase hex should return ReasonBlocked")
	}
}

func TestValidateEvent_ValidEvent(t *testing.T) {
	s := New()

	// Create and sign a valid event
	sign := p8k.MustNew()
	if err := sign.Generate(); err != nil {
		t.Fatalf("failed to generate signer: %v", err)
	}

	ev := event.New()
	ev.Kind = 1
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("test content")
	ev.Tags = tag.NewS()

	if err := ev.Sign(sign); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	result := s.ValidateEvent(ev)
	if !result.Valid {
		t.Errorf("valid event should pass validation: %s", result.Msg)
	}
}

func TestValidateEvent_InvalidID(t *testing.T) {
	s := New()

	// Create a valid event then corrupt the ID
	sign := p8k.MustNew()
	if err := sign.Generate(); err != nil {
		t.Fatalf("failed to generate signer: %v", err)
	}

	ev := event.New()
	ev.Kind = 1
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("test content")
	ev.Tags = tag.NewS()

	if err := ev.Sign(sign); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Corrupt the ID
	ev.ID[0] ^= 0xFF

	result := s.ValidateEvent(ev)
	if result.Valid {
		t.Error("event with corrupted ID should fail validation")
	}
	if result.Code != ReasonInvalid {
		t.Errorf("invalid ID should return ReasonInvalid, got %d", result.Code)
	}
}

func TestValidateEvent_FutureTimestamp(t *testing.T) {
	// Use short max future time for testing
	s := NewWithConfig(&Config{MaxFutureSeconds: 10})

	sign := p8k.MustNew()
	if err := sign.Generate(); err != nil {
		t.Fatalf("failed to generate signer: %v", err)
	}

	ev := event.New()
	ev.Kind = 1
	ev.CreatedAt = time.Now().Unix() + 3600 // 1 hour in future
	ev.Content = []byte("test content")
	ev.Tags = tag.NewS()

	if err := ev.Sign(sign); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	result := s.ValidateEvent(ev)
	if result.Valid {
		t.Error("event with future timestamp should fail validation")
	}
	if result.Code != ReasonInvalid {
		t.Errorf("future timestamp should return ReasonInvalid, got %d", result.Code)
	}
}

func TestValidateProtectedTag_NoTag(t *testing.T) {
	s := New()

	ev := event.New()
	ev.Kind = 1
	ev.Tags = tag.NewS()

	result := s.ValidateProtectedTag(ev, []byte("somepubkey"))
	if !result.Valid {
		t.Error("event without protected tag should pass validation")
	}
}

func TestValidateProtectedTag_MatchingPubkey(t *testing.T) {
	s := New()

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)
	for i := range ev.Pubkey {
		ev.Pubkey[i] = byte(i)
	}
	ev.Tags = tag.NewS()
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("-"))

	result := s.ValidateProtectedTag(ev, ev.Pubkey)
	if !result.Valid {
		t.Errorf("protected tag with matching pubkey should pass: %s", result.Msg)
	}
}

func TestValidateProtectedTag_MismatchedPubkey(t *testing.T) {
	s := New()

	ev := event.New()
	ev.Kind = 1
	ev.Pubkey = make([]byte, 32)
	for i := range ev.Pubkey {
		ev.Pubkey[i] = byte(i)
	}
	ev.Tags = tag.NewS()
	*ev.Tags = append(*ev.Tags, tag.NewFromAny("-"))

	// Different pubkey for auth
	differentPubkey := make([]byte, 32)
	for i := range differentPubkey {
		differentPubkey[i] = byte(i + 100)
	}

	result := s.ValidateProtectedTag(ev, differentPubkey)
	if result.Valid {
		t.Error("protected tag with different pubkey should fail validation")
	}
	if result.Code != ReasonBlocked {
		t.Errorf("mismatched protected tag should return ReasonBlocked, got %d", result.Code)
	}
}
