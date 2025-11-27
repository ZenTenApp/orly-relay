package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
)

// setupHotreloadTestPolicy creates a policy manager with a temporary config file for hotreload tests.
func setupHotreloadTestPolicy(t *testing.T, appName string) (*P, func()) {
	t.Helper()

	configDir := filepath.Join(xdg.ConfigHome, appName)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configPath := filepath.Join(configDir, "policy.json")
	defaultPolicy := []byte(`{"default_policy": "allow"}`)
	if err := os.WriteFile(configPath, defaultPolicy, 0644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	policy := NewWithManager(ctx, appName, true)
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

// TestValidateJSON tests the ValidateJSON method with various inputs
func TestValidateJSON(t *testing.T) {
	policy, cleanup := setupHotreloadTestPolicy(t, "test-validate-json")
	defer cleanup()

	tests := []struct {
		name        string
		json        []byte
		expectError bool
		errorSubstr string
	}{
		{
			name:        "valid empty policy",
			json:        []byte(`{}`),
			expectError: false,
		},
		{
			name: "valid complete policy",
			json: []byte(`{
				"kind": {"whitelist": [1, 3, 7]},
				"global": {"size_limit": 65536},
				"rules": {
					"1": {"description": "Short text notes", "content_limit": 8192}
				},
				"default_policy": "allow",
				"policy_admins": ["0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"],
				"policy_follow_whitelist_enabled": true
			}`),
			expectError: false,
		},
		{
			name:        "invalid JSON syntax",
			json:        []byte(`{"invalid": json}`),
			expectError: true,
			errorSubstr: "invalid character",
		},
		{
			name:        "invalid JSON - missing closing brace",
			json:        []byte(`{"kind": {"whitelist": [1]}`),
			expectError: true,
		},
		{
			name: "invalid policy_admins - wrong length",
			json: []byte(`{
				"policy_admins": ["not-64-chars"]
			}`),
			expectError: true,
			errorSubstr: "invalid policy_admin pubkey",
		},
		{
			name: "invalid policy_admins - non-hex characters",
			json: []byte(`{
				"policy_admins": ["zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"]
			}`),
			expectError: true,
			errorSubstr: "invalid policy_admin pubkey",
		},
		{
			name: "valid policy_admins - multiple admins",
			json: []byte(`{
				"policy_admins": [
					"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
				]
			}`),
			expectError: false,
		},
		{
			name: "invalid tag_validation regex",
			json: []byte(`{
				"rules": {
					"30023": {
						"tag_validation": {
							"d": "[invalid(regex"
						}
					}
				}
			}`),
			expectError: true,
			errorSubstr: "invalid regex",
		},
		{
			name: "valid tag_validation regex",
			json: []byte(`{
				"rules": {
					"30023": {
						"tag_validation": {
							"d": "^[a-z0-9-]{1,64}$",
							"t": "^[a-z0-9-]{1,32}$"
						}
					}
				}
			}`),
			expectError: false,
		},
		{
			name: "invalid default_policy",
			json: []byte(`{
				"default_policy": "invalid"
			}`),
			expectError: true,
			errorSubstr: "default_policy",
		},
		{
			name: "valid default_policy allow",
			json: []byte(`{
				"default_policy": "allow"
			}`),
			expectError: false,
		},
		{
			name: "valid default_policy deny",
			json: []byte(`{
				"default_policy": "deny"
			}`),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.ValidateJSON(tt.json)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorSubstr != "" && !containsSubstring(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorSubstr, err)
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestReload tests the Reload method
func TestReload(t *testing.T) {
	policy, cleanup := setupHotreloadTestPolicy(t, "test-reload")
	defer cleanup()

	// Create temp directory for policy files
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "policy.json")

	tests := []struct {
		name         string
		initialJSON  []byte
		reloadJSON   []byte
		expectError  bool
		checkAfter   func(t *testing.T, p *P)
	}{
		{
			name:        "reload with valid policy",
			initialJSON: []byte(`{"default_policy": "allow"}`),
			reloadJSON: []byte(`{
				"default_policy": "deny",
				"kind": {"whitelist": [1, 3]},
				"policy_admins": ["0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]
			}`),
			expectError: false,
			checkAfter: func(t *testing.T, p *P) {
				if p.DefaultPolicy != "deny" {
					t.Errorf("Expected default_policy to be 'deny', got %q", p.DefaultPolicy)
				}
				if len(p.Kind.Whitelist) != 2 {
					t.Errorf("Expected 2 whitelisted kinds, got %d", len(p.Kind.Whitelist))
				}
				if len(p.PolicyAdmins) != 1 {
					t.Errorf("Expected 1 policy admin, got %d", len(p.PolicyAdmins))
				}
			},
		},
		{
			name:        "reload with invalid JSON fails without changes",
			initialJSON: []byte(`{"default_policy": "allow"}`),
			reloadJSON:  []byte(`{"invalid json`),
			expectError: true,
			checkAfter: func(t *testing.T, p *P) {
				// Policy should remain unchanged
				if p.DefaultPolicy != "allow" {
					t.Errorf("Expected default_policy to remain 'allow', got %q", p.DefaultPolicy)
				}
			},
		},
		{
			name:        "reload with invalid admin pubkey fails without changes",
			initialJSON: []byte(`{"default_policy": "allow"}`),
			reloadJSON: []byte(`{
				"default_policy": "deny",
				"policy_admins": ["invalid-pubkey"]
			}`),
			expectError: true,
			checkAfter: func(t *testing.T, p *P) {
				// Policy should remain unchanged
				if p.DefaultPolicy != "allow" {
					t.Errorf("Expected default_policy to remain 'allow', got %q", p.DefaultPolicy)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize policy with initial JSON
			if tt.initialJSON != nil {
				if err := policy.Reload(tt.initialJSON, configPath); err != nil {
					t.Fatalf("Failed to set initial policy: %v", err)
				}
			}

			// Attempt reload
			err := policy.Reload(tt.reloadJSON, configPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}

			// Run post-reload checks
			if tt.checkAfter != nil {
				tt.checkAfter(t, policy)
			}
		})
	}
}

// TestSaveToFile tests atomic file writing
func TestSaveToFile(t *testing.T) {
	policy, cleanup := setupHotreloadTestPolicy(t, "test-save-file")
	defer cleanup()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "policy.json")

	// Load a policy
	policyJSON := []byte(`{
		"default_policy": "allow",
		"kind": {"whitelist": [1, 3, 7]},
		"policy_admins": ["0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"]
	}`)

	if err := policy.Reload(policyJSON, configPath); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Verify file was saved
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Policy file was not created at %s", configPath)
	}

	// Read and verify contents
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read policy file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Policy file is empty")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Policy file contains invalid JSON: %v", err)
	}
}

// TestPauseResume tests the Pause and Resume methods
func TestPauseResume(t *testing.T) {
	policy, cleanup := setupHotreloadTestPolicy(t, "test-pause-resume")
	defer cleanup()

	// Test Pause
	if err := policy.Pause(); err != nil {
		t.Errorf("Pause failed: %v", err)
	}

	// Test Resume
	if err := policy.Resume(); err != nil {
		t.Errorf("Resume failed: %v", err)
	}

	// Test multiple pause/resume cycles
	for i := 0; i < 3; i++ {
		if err := policy.Pause(); err != nil {
			t.Errorf("Pause %d failed: %v", i, err)
		}
		if err := policy.Resume(); err != nil {
			t.Errorf("Resume %d failed: %v", i, err)
		}
	}
}

// TestReloadPreservesExistingOnFailure verifies that failed reloads don't corrupt state
func TestReloadPreservesExistingOnFailure(t *testing.T) {
	policy, cleanup := setupHotreloadTestPolicy(t, "test-reload-preserve")
	defer cleanup()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "policy.json")

	// Set up initial valid policy
	initialJSON := []byte(`{
		"default_policy": "allow",
		"kind": {"whitelist": [1, 3, 7]},
		"policy_admins": ["0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"],
		"policy_follow_whitelist_enabled": true
	}`)

	if err := policy.Reload(initialJSON, configPath); err != nil {
		t.Fatalf("Failed to set initial policy: %v", err)
	}

	// Store initial state
	initialDefaultPolicy := policy.DefaultPolicy
	initialKindWhitelist := len(policy.Kind.Whitelist)
	initialAdminCount := len(policy.PolicyAdmins)
	initialFollowEnabled := policy.PolicyFollowWhitelistEnabled

	// Attempt to reload with invalid JSON
	invalidJSON := []byte(`{"policy_admins": ["invalid"]}`)
	err := policy.Reload(invalidJSON, configPath)
	if err == nil {
		t.Fatal("Expected error for invalid policy_admins but got none")
	}

	// Verify state is preserved
	if policy.DefaultPolicy != initialDefaultPolicy {
		t.Errorf("DefaultPolicy changed from %q to %q after failed reload",
			initialDefaultPolicy, policy.DefaultPolicy)
	}
	if len(policy.Kind.Whitelist) != initialKindWhitelist {
		t.Errorf("Kind.Whitelist length changed from %d to %d after failed reload",
			initialKindWhitelist, len(policy.Kind.Whitelist))
	}
	if len(policy.PolicyAdmins) != initialAdminCount {
		t.Errorf("PolicyAdmins length changed from %d to %d after failed reload",
			initialAdminCount, len(policy.PolicyAdmins))
	}
	if policy.PolicyFollowWhitelistEnabled != initialFollowEnabled {
		t.Errorf("PolicyFollowWhitelistEnabled changed from %v to %v after failed reload",
			initialFollowEnabled, policy.PolicyFollowWhitelistEnabled)
	}
}

// containsSubstring checks if a string contains a substring (case-insensitive)
func containsSubstring(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
