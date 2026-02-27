package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

// setupTestPolicy creates a policy manager with a temporary config file.
// Returns the policy and a cleanup function.
func setupTestPolicy(t *testing.T, appName string) (*P, func()) {
	t.Helper()

	// Create config directory at XDG path
	configDir := filepath.Join(xdg.ConfigHome, appName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create default policy.json
	configPath := filepath.Join(configDir, "policy.json")
	defaultPolicy := []byte(`{"default_policy": "allow"}`)
	if err := os.WriteFile(configPath, defaultPolicy, 0644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	policy := NewWithManager(ctx, appName, true, "")
	if policy == nil {
		cancel()
		os.RemoveAll(configDir)
		t.Fatal("Failed to create policy manager")
	}

	cleanup := func() {
		cancel()
		os.RemoveAll(configDir)
	}

	return policy, cleanup
}

// TestIsPolicyAdmin tests the IsPolicyAdmin method
func TestIsPolicyAdmin(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-policy-admin")
	defer cleanup()

	// Set up policy with admins
	admin1Hex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	admin2Hex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	nonAdminHex := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	policyJSON := []byte(`{
		"policy_admins": [
			"` + admin1Hex + `",
			"` + admin2Hex + `"
		]
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Convert hex to bytes for testing
	admin1Bin, _ := hex.Dec(admin1Hex)
	admin2Bin, _ := hex.Dec(admin2Hex)
	nonAdminBin, _ := hex.Dec(nonAdminHex)

	tests := []struct {
		name     string
		pubkey   []byte
		expected bool
	}{
		{
			name:     "first admin is recognized",
			pubkey:   admin1Bin,
			expected: true,
		},
		{
			name:     "second admin is recognized",
			pubkey:   admin2Bin,
			expected: true,
		},
		{
			name:     "non-admin is not recognized",
			pubkey:   nonAdminBin,
			expected: false,
		},
		{
			name:     "nil pubkey returns false",
			pubkey:   nil,
			expected: false,
		},
		{
			name:     "empty pubkey returns false",
			pubkey:   []byte{},
			expected: false,
		},
		{
			name:     "wrong length pubkey returns false",
			pubkey:   []byte{0x01, 0x02, 0x03},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.IsPolicyAdmin(tt.pubkey)
			if result != tt.expected {
				t.Errorf("IsPolicyAdmin() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestIsPolicyFollow tests the IsPolicyFollow method
func TestIsPolicyFollow(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-policy-follow")
	defer cleanup()

	// Set up some follows
	follow1Hex := "1111111111111111111111111111111111111111111111111111111111111111"
	follow2Hex := "2222222222222222222222222222222222222222222222222222222222222222"
	nonFollowHex := "3333333333333333333333333333333333333333333333333333333333333333"

	follow1Bin, _ := hex.Dec(follow1Hex)
	follow2Bin, _ := hex.Dec(follow2Hex)
	nonFollowBin, _ := hex.Dec(nonFollowHex)

	// Update policy follows directly
	policy.UpdatePolicyFollows([][]byte{follow1Bin, follow2Bin})

	tests := []struct {
		name     string
		pubkey   []byte
		expected bool
	}{
		{
			name:     "first follow is recognized",
			pubkey:   follow1Bin,
			expected: true,
		},
		{
			name:     "second follow is recognized",
			pubkey:   follow2Bin,
			expected: true,
		},
		{
			name:     "non-follow is not recognized",
			pubkey:   nonFollowBin,
			expected: false,
		},
		{
			name:     "nil pubkey returns false",
			pubkey:   nil,
			expected: false,
		},
		{
			name:     "empty pubkey returns false",
			pubkey:   []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.IsPolicyFollow(tt.pubkey)
			if result != tt.expected {
				t.Errorf("IsPolicyFollow() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestUpdatePolicyFollows tests the UpdatePolicyFollows method
func TestUpdatePolicyFollows(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-update-follows")
	defer cleanup()

	// Initially no follows
	testPubkey, _ := hex.Dec("1111111111111111111111111111111111111111111111111111111111111111")
	if policy.IsPolicyFollow(testPubkey) {
		t.Error("Expected no follows initially")
	}

	// Add follows
	follows := [][]byte{testPubkey}
	policy.UpdatePolicyFollows(follows)

	if !policy.IsPolicyFollow(testPubkey) {
		t.Error("Expected pubkey to be a follow after update")
	}

	// Update with empty list
	policy.UpdatePolicyFollows([][]byte{})
	if policy.IsPolicyFollow(testPubkey) {
		t.Error("Expected pubkey to not be a follow after clearing")
	}

	// Update with nil
	policy.UpdatePolicyFollows(nil)
	if policy.IsPolicyFollow(testPubkey) {
		t.Error("Expected pubkey to not be a follow after nil update")
	}
}

// TestIsPolicyFollowWhitelistEnabled tests the IsPolicyFollowWhitelistEnabled method
func TestIsPolicyFollowWhitelistEnabled(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-whitelist-enabled")
	defer cleanup()

	tmpDir := t.TempDir()

	// Test with disabled
	policyJSON := []byte(`{"policy_follow_whitelist_enabled": false}`)
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	if policy.IsPolicyFollowWhitelistEnabled() {
		t.Error("Expected follow whitelist to be disabled")
	}

	// Test with enabled
	policyJSON = []byte(`{"policy_follow_whitelist_enabled": true}`)
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	if !policy.IsPolicyFollowWhitelistEnabled() {
		t.Error("Expected follow whitelist to be enabled")
	}
}

// TestGetPolicyAdminsBin tests the GetPolicyAdminsBin method
func TestGetPolicyAdminsBin(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-get-admins-bin")
	defer cleanup()

	admin1Hex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	admin2Hex := "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	policyJSON := []byte(`{
		"policy_admins": ["` + admin1Hex + `", "` + admin2Hex + `"]
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	admins := policy.GetPolicyAdminsBin()
	if len(admins) != 2 {
		t.Errorf("Expected 2 admins, got %d", len(admins))
	}

	// Verify it's a copy (modification shouldn't affect original)
	if len(admins) > 0 {
		admins[0][0] = 0xFF
		originalAdmins := policy.GetPolicyAdminsBin()
		if originalAdmins[0][0] == 0xFF {
			t.Error("GetPolicyAdminsBin should return a copy, not the original slice")
		}
	}
}

// TestFollowListConcurrency tests concurrent access to follow list
func TestFollowListConcurrency(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-concurrency")
	defer cleanup()

	testPubkey, _ := hex.Dec("1111111111111111111111111111111111111111111111111111111111111111")

	// Run concurrent reads and writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				policy.UpdatePolicyFollows([][]byte{testPubkey})
				_ = policy.IsPolicyFollow(testPubkey)
				_ = policy.IsPolicyAdmin(testPubkey)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestPolicyAdminAndFollowInteraction tests the interaction between admin and follow checks
func TestPolicyAdminAndFollowInteraction(t *testing.T) {
	policy, cleanup := setupTestPolicy(t, "test-admin-follow-interaction")
	defer cleanup()

	// An admin who is also followed
	adminHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	adminBin, _ := hex.Dec(adminHex)

	policyJSON := []byte(`{
		"policy_admins": ["` + adminHex + `"],
		"policy_follow_whitelist_enabled": true
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Admin should be recognized as admin
	if !policy.IsPolicyAdmin(adminBin) {
		t.Error("Expected admin to be recognized as admin")
	}

	// Admin is not automatically a follow
	if policy.IsPolicyFollow(adminBin) {
		t.Error("Admin should not automatically be a follow")
	}

	// Now add admin as a follow
	policy.UpdatePolicyFollows([][]byte{adminBin})

	// Should be both admin and follow
	if !policy.IsPolicyAdmin(adminBin) {
		t.Error("Expected admin to still be recognized as admin")
	}
	if !policy.IsPolicyFollow(adminBin) {
		t.Error("Expected admin to now be recognized as follow")
	}
}
