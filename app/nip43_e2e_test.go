package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"next.orly.dev/pkg/interfaces/signer/p8k"
	"os"
	"testing"
	"time"

	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/crypto/keys"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/protocol/relayinfo"
)

// newTestListener creates a properly initialized Listener for testing
func newTestListener(server *Server, ctx context.Context) *Listener {
	listener := &Listener{
		Server:         server,
		ctx:            ctx,
		writeChan:      make(chan publish.WriteRequest, 100),
		writeDone:      make(chan struct{}),
		messageQueue:   make(chan messageRequest, 100),
		processingDone: make(chan struct{}),
		subscriptions:  make(map[string]context.CancelFunc),
	}

	// Start write worker and message processor
	go listener.writeWorker()
	go listener.messageProcessor()

	return listener
}

// closeTestListener properly closes a test listener
func closeTestListener(listener *Listener) {
	close(listener.writeChan)
	<-listener.writeDone
	close(listener.messageQueue)
	<-listener.processingDone
}

// setupE2ETest creates a full test server for end-to-end testing
func setupE2ETest(t *testing.T) (*Server, *httptest.Server, func()) {
	tempDir, err := os.MkdirTemp("", "nip43_e2e_test_*")
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
		AppName:                "TestRelay",
		NIP43Enabled:           true,
		NIP43PublishEvents:     true,
		NIP43PublishMemberList: true,
		NIP43InviteExpiry:      24 * time.Hour,
		RelayURL:               "wss://test.relay",
		Listen:                 "localhost",
		Port:                   3334,
		ACLMode:                "none",
		AuthRequired:           false,
	}

	// Generate admin keys
	adminSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate admin secret: %v", err)
	}
	adminSigner, err := p8k.New()
	if err != nil {
		t.Fatalf("failed to create admin signer: %v", err)
	}
	if err = adminSigner.InitSec(adminSecret); err != nil {
		t.Fatalf("failed to initialize admin signer: %v", err)
	}
	adminPubkey := adminSigner.Pub()

	// Add admin to config for ACL
	cfg.Admins = []string{hex.Enc(adminPubkey)}

	server := &Server{
		Ctx:           ctx,
		Config:        cfg,
		DB:            db,
		publishers:    publish.New(NewPublisher(ctx)),
		Admins:        [][]byte{adminPubkey},
		InviteManager: nip43.NewInviteManager(cfg.NIP43InviteExpiry),
		cfg:           cfg,
		db:            db,
	}

	// Configure ACL registry
	acl.Registry.Active.Store(cfg.ACLMode)
	if err = acl.Registry.Configure(cfg, db, ctx); err != nil {
		db.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("failed to configure ACL: %v", err)
	}

	server.mux = http.NewServeMux()

	// Set up HTTP handlers
	server.mux.HandleFunc(
		"/", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Accept") == "application/nostr+json" {
				server.HandleRelayInfo(w, r)
				return
			}
			http.NotFound(w, r)
		},
	)

	httpServer := httptest.NewServer(server.mux)

	cleanup := func() {
		httpServer.Close()
		db.Close()
		os.RemoveAll(tempDir)
	}

	return server, httpServer, cleanup
}

// TestE2E_RelayInfoIncludesNIP43 tests that NIP-43 is advertised in relay info
func TestE2E_RelayInfoIncludesNIP43(t *testing.T) {
	server, httpServer, cleanup := setupE2ETest(t)
	defer cleanup()

	// Make request to relay info endpoint
	req, err := http.NewRequest("GET", httpServer.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Accept", "application/nostr+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Parse relay info
	var info relayinfo.T
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode relay info: %v", err)
	}

	// Verify NIP-43 is in supported NIPs
	hasNIP43 := false
	for _, nip := range info.Nips {
		if nip == 43 {
			hasNIP43 = true
			break
		}
	}

	if !hasNIP43 {
		t.Error("NIP-43 not advertised in supported_nips")
	}

	// Verify server name
	if info.Name != server.Config.AppName {
		t.Errorf(
			"wrong relay name: got %s, want %s", info.Name,
			server.Config.AppName,
		)
	}
}

// TestE2E_CompleteJoinFlow tests the complete user join flow
func TestE2E_CompleteJoinFlow(t *testing.T) {
	server, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Step 1: Admin requests invite code
	adminPubkey := server.Admins[0]
	inviteEvent, err := server.HandleNIP43InviteRequest(adminPubkey)
	if err != nil {
		t.Fatalf("failed to generate invite: %v", err)
	}

	// Extract invite code
	claimTag := inviteEvent.Tags.GetFirst([]byte("claim"))
	if claimTag == nil || claimTag.Len() < 2 {
		t.Fatal("invite event missing claim tag")
	}
	inviteCode := string(claimTag.T[1])

	// Step 2: User creates join request
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userPubkey, err := keys.SecretBytesToPubKeyBytes(userSecret)
	if err != nil {
		t.Fatalf("failed to get user pubkey: %v", err)
	}
	signer, err := keys.SecretBytesToSigner(userSecret)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	joinEv := event.New()
	joinEv.Kind = nip43.KindJoinRequest
	copy(joinEv.Pubkey, userPubkey)
	joinEv.Tags = tag.NewS()
	joinEv.Tags.Append(tag.NewFromAny("-"))
	joinEv.Tags.Append(tag.NewFromAny("claim", inviteCode))
	joinEv.CreatedAt = time.Now().Unix()
	joinEv.Content = []byte("")
	if err = joinEv.Sign(signer); err != nil {
		t.Fatalf("failed to sign join event: %v", err)
	}

	// Step 3: Process join request
	listener := newTestListener(server, server.Ctx)
	defer closeTestListener(listener)
	err = listener.HandleNIP43JoinRequest(joinEv)
	if err != nil {
		t.Fatalf("failed to handle join request: %v", err)
	}

	// Step 4: Verify membership
	isMember, err := server.DB.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if !isMember {
		t.Error("user was not added as member")
	}

	membership, err := server.DB.GetNIP43Membership(userPubkey)
	if err != nil {
		t.Fatalf("failed to get membership: %v", err)
	}
	if membership.InviteCode != inviteCode {
		t.Errorf(
			"wrong invite code: got %s, want %s", membership.InviteCode,
			inviteCode,
		)
	}
}

// TestE2E_InviteCodeReuse tests that invite codes can only be used once
func TestE2E_InviteCodeReuse(t *testing.T) {
	server, _, cleanup := setupE2ETest(t)
	defer cleanup()

	// Generate invite code
	code, err := server.InviteManager.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate invite code: %v", err)
	}

	listener := newTestListener(server, server.Ctx)
	defer closeTestListener(listener)

	// First user uses the code
	user1Secret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user1 secret: %v", err)
	}
	user1Pubkey, err := keys.SecretBytesToPubKeyBytes(user1Secret)
	if err != nil {
		t.Fatalf("failed to get user1 pubkey: %v", err)
	}
	signer1, err := keys.SecretBytesToSigner(user1Secret)
	if err != nil {
		t.Fatalf("failed to create signer1: %v", err)
	}

	joinEv1 := event.New()
	joinEv1.Kind = nip43.KindJoinRequest
	copy(joinEv1.Pubkey, user1Pubkey)
	joinEv1.Tags = tag.NewS()
	joinEv1.Tags.Append(tag.NewFromAny("-"))
	joinEv1.Tags.Append(tag.NewFromAny("claim", code))
	joinEv1.CreatedAt = time.Now().Unix()
	joinEv1.Content = []byte("")
	if err = joinEv1.Sign(signer1); err != nil {
		t.Fatalf("failed to sign join event 1: %v", err)
	}

	err = listener.HandleNIP43JoinRequest(joinEv1)
	if err != nil {
		t.Fatalf("failed to handle join request 1: %v", err)
	}

	// Verify first user is member
	isMember, err := server.DB.IsNIP43Member(user1Pubkey)
	if err != nil {
		t.Fatalf("failed to check user1 membership: %v", err)
	}
	if !isMember {
		t.Error("user1 was not added")
	}

	// Second user tries to use same code
	user2Secret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user2 secret: %v", err)
	}
	user2Pubkey, err := keys.SecretBytesToPubKeyBytes(user2Secret)
	if err != nil {
		t.Fatalf("failed to get user2 pubkey: %v", err)
	}
	signer2, err := keys.SecretBytesToSigner(user2Secret)
	if err != nil {
		t.Fatalf("failed to create signer2: %v", err)
	}

	joinEv2 := event.New()
	joinEv2.Kind = nip43.KindJoinRequest
	copy(joinEv2.Pubkey, user2Pubkey)
	joinEv2.Tags = tag.NewS()
	joinEv2.Tags.Append(tag.NewFromAny("-"))
	joinEv2.Tags.Append(tag.NewFromAny("claim", code))
	joinEv2.CreatedAt = time.Now().Unix()
	joinEv2.Content = []byte("")
	if err = joinEv2.Sign(signer2); err != nil {
		t.Fatalf("failed to sign join event 2: %v", err)
	}

	// Should handle without error but not add user
	err = listener.HandleNIP43JoinRequest(joinEv2)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Verify second user is NOT member
	isMember, err = server.DB.IsNIP43Member(user2Pubkey)
	if err != nil {
		t.Fatalf("failed to check user2 membership: %v", err)
	}
	if isMember {
		t.Error("user2 was incorrectly added with reused code")
	}
}

// TestE2E_MembershipListGeneration tests membership list event generation
func TestE2E_MembershipListGeneration(t *testing.T) {
	server, _, cleanup := setupE2ETest(t)
	defer cleanup()

	listener := newTestListener(server, server.Ctx)
	defer closeTestListener(listener)

	// Add multiple members
	memberCount := 5
	members := make([][]byte, memberCount)

	for i := 0; i < memberCount; i++ {
		userSecret, err := keys.GenerateSecretKey()
		if err != nil {
			t.Fatalf("failed to generate user secret %d: %v", i, err)
		}
		userPubkey, err := keys.SecretBytesToPubKeyBytes(userSecret)
		if err != nil {
			t.Fatalf("failed to get user pubkey %d: %v", i, err)
		}
		members[i] = userPubkey

		// Add directly to database for speed
		err = server.DB.AddNIP43Member(userPubkey, "code")
		if err != nil {
			t.Fatalf("failed to add member %d: %v", i, err)
		}
	}

	// Generate membership list
	err := listener.publishMembershipList()
	if err != nil {
		t.Fatalf("failed to publish membership list: %v", err)
	}

	// Note: In a real test, you would verify the event was published
	// through the publishers system. For now, we just verify no error.
}

// TestE2E_ExpiredInviteCode tests that expired codes are rejected
func TestE2E_ExpiredInviteCode(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "nip43_expired_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.New(ctx, cancel, tempDir, "info")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	cfg := &config.C{
		NIP43Enabled:      true,
		NIP43InviteExpiry: 1 * time.Millisecond, // Very short expiry
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

	listener := newTestListener(server, ctx)
	defer closeTestListener(listener)

	// Generate invite code
	code, err := server.InviteManager.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate invite code: %v", err)
	}

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	// Try to use expired code
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userPubkey, err := keys.SecretBytesToPubKeyBytes(userSecret)
	if err != nil {
		t.Fatalf("failed to get user pubkey: %v", err)
	}
	signer, err := keys.SecretBytesToSigner(userSecret)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	joinEv := event.New()
	joinEv.Kind = nip43.KindJoinRequest
	copy(joinEv.Pubkey, userPubkey)
	joinEv.Tags = tag.NewS()
	joinEv.Tags.Append(tag.NewFromAny("-"))
	joinEv.Tags.Append(tag.NewFromAny("claim", code))
	joinEv.CreatedAt = time.Now().Unix()
	joinEv.Content = []byte("")
	if err = joinEv.Sign(signer); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	err = listener.HandleNIP43JoinRequest(joinEv)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Verify user was NOT added
	isMember, err := db.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("user was added with expired code")
	}
}

// TestE2E_InvalidTimestampRejected tests that events with invalid timestamps are rejected
func TestE2E_InvalidTimestampRejected(t *testing.T) {
	server, _, cleanup := setupE2ETest(t)
	defer cleanup()

	listener := newTestListener(server, server.Ctx)
	defer closeTestListener(listener)

	// Generate invite code
	code, err := server.InviteManager.GenerateCode()
	if err != nil {
		t.Fatalf("failed to generate invite code: %v", err)
	}

	// Create user
	userSecret, err := keys.GenerateSecretKey()
	if err != nil {
		t.Fatalf("failed to generate user secret: %v", err)
	}
	userPubkey, err := keys.SecretBytesToPubKeyBytes(userSecret)
	if err != nil {
		t.Fatalf("failed to get user pubkey: %v", err)
	}
	signer, err := keys.SecretBytesToSigner(userSecret)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	// Create join request with timestamp far in the past
	joinEv := event.New()
	joinEv.Kind = nip43.KindJoinRequest
	copy(joinEv.Pubkey, userPubkey)
	joinEv.Tags = tag.NewS()
	joinEv.Tags.Append(tag.NewFromAny("-"))
	joinEv.Tags.Append(tag.NewFromAny("claim", code))
	joinEv.CreatedAt = time.Now().Unix() - 700 // More than 10 minutes ago
	joinEv.Content = []byte("")
	if err = joinEv.Sign(signer); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Should handle without error but not add user
	err = listener.HandleNIP43JoinRequest(joinEv)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Verify user was NOT added
	isMember, err := server.DB.IsNIP43Member(userPubkey)
	if err != nil {
		t.Fatalf("failed to check membership: %v", err)
	}
	if isMember {
		t.Error("user was added with invalid timestamp")
	}
}

// BenchmarkJoinRequestProcessing benchmarks join request processing
func BenchmarkJoinRequestProcessing(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "nip43_bench_*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.New(ctx, cancel, tempDir, "error")
	if err != nil {
		b.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	cfg := &config.C{
		NIP43Enabled:      true,
		NIP43InviteExpiry: 24 * time.Hour,
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

	listener := newTestListener(server, ctx)
	defer closeTestListener(listener)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Generate unique user and code for each iteration
		userSecret, _ := keys.GenerateSecretKey()
		userPubkey, _ := keys.SecretBytesToPubKeyBytes(userSecret)
		signer, _ := keys.SecretBytesToSigner(userSecret)
		code, _ := server.InviteManager.GenerateCode()

		joinEv := event.New()
		joinEv.Kind = nip43.KindJoinRequest
		copy(joinEv.Pubkey, userPubkey)
		joinEv.Tags = tag.NewS()
		joinEv.Tags.Append(tag.NewFromAny("-"))
		joinEv.Tags.Append(tag.NewFromAny("claim", code))
		joinEv.CreatedAt = time.Now().Unix()
		joinEv.Content = []byte("")
		joinEv.Sign(signer)

		listener.HandleNIP43JoinRequest(joinEv)
	}
}
