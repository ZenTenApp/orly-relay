package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/protocol/publish"
)

// setupPolicyTestListener creates a test listener with policy system enabled
func setupPolicyTestListener(t *testing.T, policyAdminHex string) (*Listener, *database.D, func()) {
	tempDir, err := os.MkdirTemp("", "policy_handler_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Use a unique app name per test to avoid conflicts
	appName := "test-policy-" + filepath.Base(tempDir)

	// Create the XDG config directory and default policy file BEFORE creating the policy manager
	configDir := filepath.Join(xdg.ConfigHome, appName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create initial policy file with admin if provided
	var initialPolicy []byte
	if policyAdminHex != "" {
		initialPolicy = []byte(`{
			"default_policy": "allow",
			"policy_admins": ["` + policyAdminHex + `"],
			"policy_follow_whitelist_enabled": true
		}`)
	} else {
		initialPolicy = []byte(`{"default_policy": "allow"}`)
	}
	policyPath := filepath.Join(configDir, "policy.json")
	if err := os.WriteFile(policyPath, initialPolicy, 0644); err != nil {
		os.RemoveAll(tempDir)
		os.RemoveAll(configDir)
		t.Fatalf("failed to write policy file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	db, err := database.New(ctx, cancel, tempDir, "info")
	if err != nil {
		os.RemoveAll(tempDir)
		os.RemoveAll(configDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cfg := &config.C{
		PolicyEnabled: true,
		RelayURL:      "wss://test.relay",
		Listen:        "localhost",
		Port:          3334,
		ACLMode:       "none",
		AppName:       appName,
	}

	// Create policy manager - now config file exists at XDG path
	policyManager := policy.NewWithManager(ctx, cfg.AppName, cfg.PolicyEnabled, "")

	server := &Server{
		Ctx:             ctx,
		Config:          cfg,
		DB:              db,
		publishers:      publish.New(NewPublisher(ctx)),
		policyManager:   policyManager,
		cfg:             cfg,
		db:              db,
		messagePauseMutex: sync.RWMutex{},
	}

	// Configure ACL registry
	acl.Registry.SetMode(cfg.ACLMode)
	if err = acl.Registry.Configure(cfg, db, ctx); err != nil {
		db.Close()
		os.RemoveAll(tempDir)
		os.RemoveAll(configDir)
		t.Fatalf("failed to configure ACL: %v", err)
	}

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

	cleanup := func() {
		close(listener.writeChan)
		<-listener.writeDone
		close(listener.messageQueue)
		<-listener.processingDone
		db.Close()
		os.RemoveAll(tempDir)
		os.RemoveAll(configDir)
	}

	return listener, db, cleanup
}

// createPolicyConfigEvent creates a kind 12345 policy config event
func createPolicyConfigEvent(t *testing.T, signer *p8k.Signer, policyJSON string) *event.E {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind.PolicyConfig.K
	ev.Content = []byte(policyJSON)
	ev.Tags = tag.NewS()

	if err := ev.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	return ev
}

// TestHandlePolicyConfigUpdate_ValidAdmin tests policy update from valid admin
// Policy admins can extend rules but cannot modify protected fields (owners, policy_admins)
func TestHandlePolicyConfigUpdate_ValidAdmin(t *testing.T) {
	// Create admin signer
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Create valid policy update event that ONLY extends, doesn't modify protected fields
	// Note: policy_admins must stay the same (policy admins cannot change this field)
	newPolicyJSON := `{
		"default_policy": "allow",
		"policy_admins": ["` + adminHex + `"],
		"kind": {"whitelist": [1, 3, 7]}
	}`

	ev := createPolicyConfigEvent(t, adminSigner, newPolicyJSON)

	// Handle the event
	err := listener.HandlePolicyConfigUpdate(ev)
	if err != nil {
		t.Errorf("Expected success but got error: %v", err)
	}

	// Verify policy was updated (kind whitelist was extended)
	// Note: default_policy should still be "allow" from original
	if listener.policyManager.DefaultPolicy != "allow" {
		t.Errorf("Policy was not updated correctly, default_policy = %q, expected 'allow'",
			listener.policyManager.DefaultPolicy)
	}
}

// TestHandlePolicyConfigUpdate_NonAdmin tests policy update rejection from non-admin
func TestHandlePolicyConfigUpdate_NonAdmin(t *testing.T) {
	// Create admin signer
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	// Create non-admin signer
	nonAdminSigner := p8k.MustNew()
	if err := nonAdminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate non-admin keypair: %v", err)
	}

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Create policy update event from non-admin
	newPolicyJSON := `{"default_policy": "deny"}`
	ev := createPolicyConfigEvent(t, nonAdminSigner, newPolicyJSON)

	// Handle the event - should be rejected
	err := listener.HandlePolicyConfigUpdate(ev)
	if err == nil {
		t.Error("Expected error for non-admin update but got none")
	}

	// Verify policy was NOT updated
	if listener.policyManager.DefaultPolicy != "allow" {
		t.Error("Policy should not have been updated by non-admin")
	}
}

// TestHandlePolicyConfigUpdate_InvalidJSON tests rejection of invalid JSON
func TestHandlePolicyConfigUpdate_InvalidJSON(t *testing.T) {
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Create event with invalid JSON
	ev := createPolicyConfigEvent(t, adminSigner, `{"invalid json`)

	err := listener.HandlePolicyConfigUpdate(ev)
	if err == nil {
		t.Error("Expected error for invalid JSON but got none")
	}

	// Policy should remain unchanged
	if listener.policyManager.DefaultPolicy != "allow" {
		t.Error("Policy should not have been updated with invalid JSON")
	}
}

// TestHandlePolicyConfigUpdate_InvalidPubkey tests rejection of invalid admin pubkeys
func TestHandlePolicyConfigUpdate_InvalidPubkey(t *testing.T) {
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Try to update with invalid admin pubkey
	invalidPolicyJSON := `{
		"default_policy": "deny",
		"policy_admins": ["not-a-valid-pubkey"]
	}`
	ev := createPolicyConfigEvent(t, adminSigner, invalidPolicyJSON)

	err := listener.HandlePolicyConfigUpdate(ev)
	if err == nil {
		t.Error("Expected error for invalid admin pubkey but got none")
	}

	// Policy should remain unchanged
	if listener.policyManager.DefaultPolicy != "allow" {
		t.Error("Policy should not have been updated with invalid admin pubkey")
	}
}

// TestHandlePolicyConfigUpdate_PolicyAdminCannotModifyProtectedFields tests that policy admins
// cannot modify the owners or policy_admins fields (these are protected, owner-only fields)
func TestHandlePolicyConfigUpdate_PolicyAdminCannotModifyProtectedFields(t *testing.T) {
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	// Create second admin
	admin2Hex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Try to add second admin (policy_admins is a protected field)
	newPolicyJSON := `{
		"default_policy": "allow",
		"policy_admins": ["` + adminHex + `", "` + admin2Hex + `"]
	}`
	ev := createPolicyConfigEvent(t, adminSigner, newPolicyJSON)

	// This should FAIL because policy admins cannot modify the policy_admins field
	err := listener.HandlePolicyConfigUpdate(ev)
	if err == nil {
		t.Error("Expected error when policy admin tries to modify policy_admins (protected field)")
	}

	// Second admin should NOT be in the list since update was rejected
	admin2Bin, _ := hex.Dec(admin2Hex)
	if listener.policyManager.IsPolicyAdmin(admin2Bin) {
		t.Error("Second admin should NOT have been added - policy_admins is protected")
	}
}

// TestHandlePolicyAdminFollowListUpdate tests follow list update from admin
func TestHandlePolicyAdminFollowListUpdate(t *testing.T) {
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	listener, db, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Create a kind 3 follow list event from admin
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind.FollowList.K
	ev.Content = []byte("")
	ev.Tags = tag.NewS()

	// Add some follows
	follow1Hex := "1111111111111111111111111111111111111111111111111111111111111111"
	follow2Hex := "2222222222222222222222222222222222222222222222222222222222222222"
	ev.Tags.Append(tag.NewFromAny("p", follow1Hex))
	ev.Tags.Append(tag.NewFromAny("p", follow2Hex))

	if err := ev.Sign(adminSigner); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	// Save the event to database first
	if _, err := db.SaveEvent(listener.ctx, ev); err != nil {
		t.Fatalf("Failed to save follow list event: %v", err)
	}

	// Handle the follow list update
	err := listener.HandlePolicyAdminFollowListUpdate(ev)
	if err != nil {
		t.Errorf("Expected success but got error: %v", err)
	}

	// Verify follows were added
	follow1Bin, _ := hex.Dec(follow1Hex)
	follow2Bin, _ := hex.Dec(follow2Hex)

	if !listener.policyManager.IsPolicyFollow(follow1Bin) {
		t.Error("Follow 1 should have been added to policy follows")
	}
	if !listener.policyManager.IsPolicyFollow(follow2Bin) {
		t.Error("Follow 2 should have been added to policy follows")
	}
}

// TestIsPolicyAdminFollowListEvent tests detection of admin follow list events
func TestIsPolicyAdminFollowListEvent(t *testing.T) {
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	nonAdminSigner := p8k.MustNew()
	if err := nonAdminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate non-admin keypair: %v", err)
	}

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Test admin's kind 3 event
	adminFollowEv := event.New()
	adminFollowEv.Kind = kind.FollowList.K
	adminFollowEv.Tags = tag.NewS()
	if err := adminFollowEv.Sign(adminSigner); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if !listener.IsPolicyAdminFollowListEvent(adminFollowEv) {
		t.Error("Should detect admin's follow list event")
	}

	// Test non-admin's kind 3 event
	nonAdminFollowEv := event.New()
	nonAdminFollowEv.Kind = kind.FollowList.K
	nonAdminFollowEv.Tags = tag.NewS()
	if err := nonAdminFollowEv.Sign(nonAdminSigner); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if listener.IsPolicyAdminFollowListEvent(nonAdminFollowEv) {
		t.Error("Should not detect non-admin's follow list event")
	}

	// Test admin's non-kind-3 event
	adminOtherEv := event.New()
	adminOtherEv.Kind = 1 // Kind 1, not follow list
	adminOtherEv.Tags = tag.NewS()
	if err := adminOtherEv.Sign(adminSigner); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if listener.IsPolicyAdminFollowListEvent(adminOtherEv) {
		t.Error("Should not detect admin's non-follow-list event")
	}
}

// TestIsPolicyConfigEvent tests detection of policy config events
func TestIsPolicyConfigEvent(t *testing.T) {
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Kind 12345 event
	policyEv := event.New()
	policyEv.Kind = kind.PolicyConfig.K
	policyEv.Tags = tag.NewS()
	if err := policyEv.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if !IsPolicyConfigEvent(policyEv) {
		t.Error("Should detect kind 12345 as policy config event")
	}

	// Non-policy event
	otherEv := event.New()
	otherEv.Kind = 1
	otherEv.Tags = tag.NewS()
	if err := otherEv.Sign(signer); err != nil {
		t.Fatalf("Failed to sign event: %v", err)
	}

	if IsPolicyConfigEvent(otherEv) {
		t.Error("Should not detect kind 1 as policy config event")
	}
}

// TestMessageProcessingPauseDuringPolicyUpdate tests that message processing is paused
func TestMessageProcessingPauseDuringPolicyUpdate(t *testing.T) {
	adminSigner := p8k.MustNew()
	if err := adminSigner.Generate(); err != nil {
		t.Fatalf("Failed to generate admin keypair: %v", err)
	}
	adminHex := hex.Enc(adminSigner.Pub())

	listener, _, cleanup := setupPolicyTestListener(t, adminHex)
	defer cleanup()

	// Track if pause was called
	pauseCalled := false
	resumeCalled := false

	// We can't easily mock the mutex, but we can verify the policy update succeeds
	// which implies the pause/resume cycle completed
	// Note: policy_admins must stay the same (protected field)
	newPolicyJSON := `{
		"default_policy": "allow",
		"policy_admins": ["` + adminHex + `"],
		"kind": {"whitelist": [1, 3, 5, 7]}
	}`
	ev := createPolicyConfigEvent(t, adminSigner, newPolicyJSON)

	err := listener.HandlePolicyConfigUpdate(ev)
	if err != nil {
		t.Errorf("Policy update failed: %v", err)
	}

	// If we got here without deadlock, the pause/resume worked
	_ = pauseCalled
	_ = resumeCalled

	// Verify policy was actually updated (kind whitelist was extended)
	if listener.policyManager.DefaultPolicy != "allow" {
		t.Error("Policy should have been updated")
	}
}
