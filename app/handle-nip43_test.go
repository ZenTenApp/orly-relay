package app

import (
	"context"
	"os"
	"testing"
	"time"

	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/protocol/nip43"
	"git.smesh.lol/orly/pkg/protocol/publish"
)

// setupTestListener creates a test listener with NIP-43 enabled
func setupTestListener(t *testing.T) (*Listener, *database.D, func()) {
	tempDir, err := os.MkdirTemp("", "nip43_handler_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	db, err := database.New(ctx, cancel, tempDir, "info")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cfg := &config.C{
		NIP43Enabled:           true,
		NIP43PublishEvents:     true,
		NIP43PublishMemberList: true,
		NIP43InviteExpiry:      24 * time.Hour,
		RelayURL:               "wss://test.relay",
		Listen:                 "localhost",
		Port:                   3334,
		ACLMode:                "none",
	}

	server := &Server{
		Ctx:           ctx,
		Config:        cfg,
		DB:            db,
		publishers:    publish.New(NewPublisher(ctx)),
		InviteManager: nip43.NewInviteManager(cfg.NIP43InviteExpiry),
		cfg:           cfg,
		db:            db,
	}

	// Configure ACL registry
	acl.Registry.SetMode(cfg.ACLMode)
	if err = acl.Registry.Configure(cfg, db, ctx); err != nil {
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to configure ACL: %v", err)
	}

	listener := &Listener{
		Server:         server,
		ctx:            ctx,
		writeChan:      make(chan publish.WriteRequest, 100),
		writeDone:      make(chan struct{}),
		messageQueue:   make(chan messageRequest, 100),
		processingDone: make(chan struct{}),
	}
	listener.initActors()

	// Start write worker and message processor
	go listener.writeWorker()
	go listener.messageProcessor()

	cleanup := func() {
		// Close listener channels
		close(listener.writeChan)
		<-listener.writeDone
		close(listener.messageQueue)
		<-listener.processingDone
		db.Close()
		os.RemoveAll(tempDir)
	}

	return listener, db, cleanup
}

// TestHandleNIP43JoinRequest_ValidRequest tests a successful join request
func TestHandleNIP43JoinRequest_ValidRequest(t *testing.T) {
	listener, db, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate test user
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Generate invite code
	code, err := listener.Server.InviteManager.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate invite code: %v", err)
	}

	// Create join request event
	ev := event.New()
	ev.Kind = nip43.KindJoinRequest
	copy(ev.Pubkey, userPubkey)
	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))
	ev.Tags.Append(tag.NewFromAny("claim", code))
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	// Sign event
	if err = ev.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Handle join request
	err = listener.HandleNIP43JoinRequest(ev)
	if err != nil {
		t.Fatalf("failed to handle join request: %v", err)
	}

	// Verify user was added to database
	isMember, err := db.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("user was not added as member")
	}

	// Verify membership details
	membership, err := db.GetNIP43Membership(userPubkey)
	if err != nil {
		t.Fatalf("failed to get membership: %v", err)
	}
	if membership.InviteCode != code {
		t.Errorf("wrong invite code stored: got %s, want %s", membership.InviteCode, code)
	}
}

// TestHandleNIP43JoinRequest_InvalidCode tests join request with invalid code
func TestHandleNIP43JoinRequest_InvalidCode(t *testing.T) {
	listener, db, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate test user
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Create join request with invalid code
	ev := event.New()
	ev.Kind = nip43.KindJoinRequest
	copy(ev.Pubkey, userPubkey)
	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))
	ev.Tags.Append(tag.NewFromAny("claim", "invalid-code-123"))
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err = ev.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Handle join request - should succeed but not add member
	err = listener.HandleNIP43JoinRequest(ev)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Verify user was NOT added
	isMember, err := db.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("user was incorrectly added as member with invalid code")
	}
}

// TestHandleNIP43JoinRequest_DuplicateMember tests join request from existing member
func TestHandleNIP43JoinRequest_DuplicateMember(t *testing.T) {
	listener, db, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate test user
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Add user directly to database
	err = db.AddNIP43Member(userPubkey, "original-code")
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Generate new invite code
	code, err := listener.Server.InviteManager.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate invite code: %v", err)
	}

	// Create join request
	ev := event.New()
	ev.Kind = nip43.KindJoinRequest
	copy(ev.Pubkey, userPubkey)
	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))
	ev.Tags.Append(tag.NewFromAny("claim", code))
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err = ev.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Handle join request - should handle gracefully
	err = listener.HandleNIP43JoinRequest(ev)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Verify original membership is unchanged
	membership, err := db.GetNIP43Membership(userPubkey)
	if err != nil {
		t.Fatalf("failed to get membership: %v", err)
	}
	if membership.InviteCode != "original-code" {
		t.Errorf("invite code was changed: got %s, want original-code", membership.InviteCode)
	}
}

// TestHandleNIP43LeaveRequest_ValidRequest tests a successful leave request
func TestHandleNIP43LeaveRequest_ValidRequest(t *testing.T) {
	listener, db, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate test user
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Add user as member
	err = db.AddNIP43Member(userPubkey, "test-code")
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	// Create leave request
	ev := event.New()
	ev.Kind = nip43.KindLeaveRequest
	copy(ev.Pubkey, userPubkey)
	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err = ev.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Handle leave request
	err = listener.HandleNIP43LeaveRequest(ev)
	if err != nil {
		t.Fatalf("failed to handle leave request: %v", err)
	}

	// Verify user was removed
	isMember, err := db.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("user was not removed")
	}
}

// TestHandleNIP43LeaveRequest_NonMember tests leave request from non-member
func TestHandleNIP43LeaveRequest_NonMember(t *testing.T) {
	listener, _, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate test user (not a member)
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Create leave request
	ev := event.New()
	ev.Kind = nip43.KindLeaveRequest
	copy(ev.Pubkey, userPubkey)
	ev.Tags = tag.NewS()
	ev.Tags.Append(tag.NewFromAny("-"))
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte("")

	if err = ev.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Handle leave request - should handle gracefully
	err = listener.HandleNIP43LeaveRequest(ev)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
}

// TestHandleNIP43InviteRequest_ValidRequest tests invite request from admin
func TestHandleNIP43InviteRequest_ValidRequest(t *testing.T) {
	listener, _, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate admin user
	adminSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate admin secret: %v", err)
	}
	adminSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = adminSigner.InitSec(adminSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	adminPubkey := adminSigner.Pub()

	// Add admin to config and reconfigure ACL
	adminHex := hex.Enc(adminPubkey)
	listener.Server.Config.Admins = []string{adminHex}
	acl.Registry.SetMode("none")
	if err = acl.Registry.Configure(listener.Server.Config, listener.Server.DB, listener.ctx); err != nil {
		t.Fatalf("failed to reconfigure ACL: %v", err)
	}

	// Handle invite request
	inviteEvent, err := listener.Server.HandleNIP43InviteRequest(adminPubkey)
	if err != nil {
		t.Fatalf("failed to handle invite request: %v", err)
	}

	// Verify invite event
	if inviteEvent == nil {
		t.Fatal("invite event is nil")
	}
	if inviteEvent.Kind != nip43.KindInviteReq {
		t.Errorf("wrong event kind: got %d, want %d", inviteEvent.Kind, nip43.KindInviteReq)
	}

	// Verify claim tag
	claimTag := inviteEvent.Tags.GetFirst([]byte("claim"))
	if claimTag == nil {
		t.Fatal("missing claim tag")
	}
	if claimTag.Len() < 2 {
		t.Fatal("claim tag has no value")
	}
}

// TestHandleNIP43InviteRequest_Unauthorized tests invite request from non-admin
func TestHandleNIP43InviteRequest_Unauthorized(t *testing.T) {
	listener, _, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate regular user (not admin)
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Handle invite request - should fail
	_, err = listener.Server.HandleNIP43InviteRequest(userPubkey)
	if err == nil {
		t.Fatal("expected error for unauthorized user")
	}
}

// TestJoinAndLeaveFlow tests the complete join and leave flow
func TestJoinAndLeaveFlow(t *testing.T) {
	listener, db, cleanup := setupTestListener(t)
	defer cleanup()

	// Generate test user
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	if err = userSigner.InitSec(userSecret); err != nil {
		t.Fatalf("failed to initialize signer: %v", err)
	}
	userPubkey := userSigner.Pub()

	// Step 1: Generate invite code
	code, err := listener.Server.InviteManager.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate invite code: %v", err)
	}

	// Step 2: User sends join request
	joinEv := event.New()
	joinEv.Kind = nip43.KindJoinRequest
	copy(joinEv.Pubkey, userPubkey)
	joinEv.Tags = tag.NewS()
	joinEv.Tags.Append(tag.NewFromAny("-"))
	joinEv.Tags.Append(tag.NewFromAny("claim", code))
	joinEv.CreatedAt = time.Now().Unix()
	joinEv.Content = []byte("")
	if err = joinEv.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign join event: %v", err)
	}

	err = listener.HandleNIP43JoinRequest(joinEv)
	if err != nil {
		t.Fatalf("failed to handle join request: %v", err)
	}

	// Verify user is member
	isMember, err := db.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership after join: %v", err)
	}
	if !isMember {
		t.Fatal("user is not a member after join")
	}

	// Step 3: User sends leave request
	leaveEv := event.New()
	leaveEv.Kind = nip43.KindLeaveRequest
	copy(leaveEv.Pubkey, userPubkey)
	leaveEv.Tags = tag.NewS()
	leaveEv.Tags.Append(tag.NewFromAny("-"))
	leaveEv.CreatedAt = time.Now().Unix()
	leaveEv.Content = []byte("")
	if err = leaveEv.Sign(userSigner); err != nil {
		t.Fatalf("failed to sign leave event: %v", err)
	}

	err = listener.HandleNIP43LeaveRequest(leaveEv)
	if err != nil {
		t.Fatalf("failed to handle leave request: %v", err)
	}

	// Verify user is no longer member
	isMember, err = db.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership after leave: %v", err)
	}
	if isMember {
		t.Fatal("user is still a member after leave")
	}
}

// TestMultipleUsersJoining tests multiple users joining concurrently
func TestMultipleUsersJoining(t *testing.T) {
	listener, db, cleanup := setupTestListener(t)
	defer cleanup()

	userCount := 10
	done := make(chan bool, userCount)

	for i := 0; i < userCount; i++ {
		go func(index int) {
			// Generate user
			userSecret, err := keys.GenerateSecretKey()
			if err != nil {
				t.Errorf("failed to generate user secret %d: %v", index, err)
				done <- false
				return
			}
			userSigner, err := p8k.New()
			if err != nil {
				t.Errorf("failed to create signer %d: %v", index, err)
				done <- false
				return
			}
			if err = userSigner.InitSec(userSecret); err != nil {
				t.Errorf("failed to initialize signer %d: %v", index, err)
				done <- false
				return
			}
			userPubkey := userSigner.Pub()

			// Generate invite code
			code, err := listener.Server.InviteManager.GenerateCode()
			if err != nil {
				t.Errorf("failed to generate invite code %d: %v", index, err)
				done <- false
				return
			}

			// Create join request
			joinEv := event.New()
			joinEv.Kind = nip43.KindJoinRequest
			copy(joinEv.Pubkey, userPubkey)
			joinEv.Tags = tag.NewS()
			joinEv.Tags.Append(tag.NewFromAny("-"))
			joinEv.Tags.Append(tag.NewFromAny("claim", code))
			joinEv.CreatedAt = time.Now().Unix()
			joinEv.Content = []byte("")
			if err = joinEv.Sign(userSigner); err != nil {
				t.Errorf("failed to sign event %d: %v", index, err)
				done <- false
				return
			}

			// Handle join request
			if err = listener.HandleNIP43JoinRequest(joinEv); err != nil {
				t.Errorf("failed to handle join request %d: %v", index, err)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	successCount := 0
	for i := 0; i < userCount; i++ {
		if <-done {
			successCount++
		}
	}

	if successCount != userCount {
		t.Errorf("not all users joined successfully: %d/%d", successCount, userCount)
	}

	// Verify member count
	members, err := db.GetAllNIP43Members()
	if err != nil {
		t.Fatalf("failed to get all members: %v", err)
	}

	if len(members) != successCount {
		t.Errorf("wrong member count: got %d, want %d", len(members), successCount)
	}
}
