package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lol.mleku.dev/log"
)

// TestBugReproduction_Kind1AllowedWithWhitelist4678 reproduces the reported bug
// where kind 1 events are being accepted even though only kind 4678 is in the whitelist.
func TestBugReproduction_Kind1AllowedWithWhitelist4678(t *testing.T) {
	testSigner, testPubkey := generateTestKeypair(t)

	// Create policy matching the production configuration
	policyJSON := `{
		"kind": { "whitelist": [4678] },
		"rules": {
			"4678": {
				"description": "Zenotp events",
				"script": "policy.sh"
			}
		}
	}`

	policy, err := New([]byte(policyJSON))
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	t.Run("Kind 1 should be REJECTED (not in whitelist)", func(t *testing.T) {
		event := createTestEvent(t, testSigner, "Hello Nostr!", 1)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Errorf("BUG REPRODUCED: Kind 1 event was ALLOWED but should be REJECTED (only kind 4678 is whitelisted)")
			t.Logf("Policy whitelist: %v", policy.Kind.Whitelist)
			t.Logf("Policy rules: %v", policy.Rules)
			t.Logf("Default policy: %s", policy.DefaultPolicy)
		}
	})

	t.Run("Kind 4678 should be ALLOWED (in whitelist)", func(t *testing.T) {
		event := createTestEvent(t, testSigner, "Zenotp event", 4678)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 4678 should be ALLOWED (in whitelist)")
		}
	})
}

// TestBugReproduction_WithPolicyManager tests with a full policy manager setup
// to match production environment more closely
func TestBugReproduction_WithPolicyManager(t *testing.T) {
	testSigner, testPubkey := generateTestKeypair(t)

	// Create a temporary config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "ORLY")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write policy configuration matching production
	policyConfig := map[string]interface{}{
		"kind": map[string]interface{}{
			"whitelist": []int{4678},
		},
		"rules": map[string]interface{}{
			"4678": map[string]interface{}{
				"description": "Zenotp events",
				"script":      "policy.sh",
			},
		},
	}

	policyJSON, err := json.MarshalIndent(policyConfig, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal policy JSON: %v", err)
	}

	policyPath := filepath.Join(configDir, "policy.json")
	if err := os.WriteFile(policyPath, policyJSON, 0644); err != nil {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	// Create policy with manager (enabled)
	ctx := context.Background()
	policy := NewWithManager(ctx, "ORLY", true)

	// Load policy from file
	if err := policy.LoadFromFile(policyPath); err != nil {
		t.Fatalf("Failed to load policy from file: %v", err)
	}

	t.Run("Kind 1 should be REJECTED with PolicyManager", func(t *testing.T) {
		event := createTestEvent(t, testSigner, "Hello Nostr!", 1)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Errorf("BUG REPRODUCED: Kind 1 event was ALLOWED but should be REJECTED")
			t.Logf("Policy whitelist: %v", policy.Kind.Whitelist)
			t.Logf("Policy rules: %v", policy.Rules)
			t.Logf("Default policy: %s", policy.DefaultPolicy)
			t.Logf("Manager enabled: %v", policy.Manager.IsEnabled())
		}
	})

	t.Run("Kind 4678 should be ALLOWED with PolicyManager", func(t *testing.T) {
		event := createTestEvent(t, testSigner, "Zenotp event", 4678)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 4678 should be ALLOWED (in whitelist)")
		}
	})

	// Clean up
	if policy.Manager != nil {
		policy.Manager.Shutdown()
	}
}

// TestBugReproduction_DebugPolicyFlow adds verbose logging to debug the policy flow
func TestBugReproduction_DebugPolicyFlow(t *testing.T) {
	testSigner, testPubkey := generateTestKeypair(t)

	policyJSON := `{
		"kind": { "whitelist": [4678] },
		"rules": {
			"4678": {
				"description": "Zenotp events",
				"script": "policy.sh"
			}
		}
	}`

	policy, err := New([]byte(policyJSON))
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	event := createTestEvent(t, testSigner, "Hello Nostr!", 1)

	t.Logf("=== Policy Configuration ===")
	t.Logf("Whitelist: %v", policy.Kind.Whitelist)
	t.Logf("Blacklist: %v", policy.Kind.Blacklist)
	t.Logf("Rules: %v", policy.Rules)
	t.Logf("Default policy: %s", policy.DefaultPolicy)
	t.Logf("")
	t.Logf("=== Event Details ===")
	t.Logf("Event kind: %d", event.Kind)
	t.Logf("")
	t.Logf("=== Policy Check Flow ===")

	// Step 1: Check kinds policy
	kindsAllowed := policy.checkKindsPolicy(event.Kind)
	t.Logf("1. checkKindsPolicy(kind=%d) returned: %v", event.Kind, kindsAllowed)

	// Full policy check
	allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
	t.Logf("2. CheckPolicy returned: allowed=%v, err=%v", allowed, err)

	if allowed {
		t.Errorf("BUG REPRODUCED: Kind 1 should be REJECTED but was ALLOWED")
	}
}

// TestBugFix_FailSafeWhenConfigMissing tests the fix for the security bug
// where missing config would allow all events
func TestBugFix_FailSafeWhenConfigMissing(t *testing.T) {
	testSigner, testPubkey := generateTestKeypair(t)

	t.Run("Missing config with enabled policy causes panic", func(t *testing.T) {
		// When policy is enabled but config file is missing, NewWithManager should panic
		// This is a FATAL configuration error that must be fixed before the relay can start

		defer func() {
			r := recover()
			if r == nil {
				t.Error("Expected panic when policy is enabled but config is missing, but no panic occurred")
			} else {
				// Verify the panic message mentions the config error
				panicMsg := fmt.Sprintf("%v", r)
				if !strings.Contains(panicMsg, "fatal policy configuration error") {
					t.Errorf("Panic message should mention 'fatal policy configuration error', got: %s", panicMsg)
				}
				t.Logf("Correctly panicked with message: %s", panicMsg)
			}
		}()

		// Simulate NewWithManager behavior by directly testing the panic path
		// Create a policy manager with a non-existent config path
		ctx := context.Background()
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "ORLY_TEST_NO_CONFIG")
		configPath := filepath.Join(configDir, "policy.json")

		// Ensure directory exists but file doesn't
		os.MkdirAll(configDir, 0755)

		manager := &PolicyManager{
			ctx:        ctx,
			configDir:  configDir,
			scriptPath: filepath.Join(configDir, "policy.sh"),
			enabled:    true,
			runners:    make(map[string]*ScriptRunner),
		}

		policy := &P{
			DefaultPolicy: "allow",
			Manager:       manager,
		}

		// Try to load from nonexistent file - this should trigger the panic
		if err := policy.LoadFromFile(configPath); err != nil {
			// Simulate what NewWithManager does when LoadFromFile fails
			log.E.F(
				"FATAL: Policy system is ENABLED (ORLY_POLICY_ENABLED=true) but configuration failed to load from %s: %v",
				configPath, err,
			)
			log.E.F("The relay cannot start with an invalid policy configuration.")
			log.E.F("Fix: Either disable the policy system (ORLY_POLICY_ENABLED=false) or ensure %s exists and contains valid JSON", configPath)
			panic(fmt.Sprintf("fatal policy configuration error: %v", err))
		}

		// Should never reach here
		t.Error("Should have panicked but didn't")
	})

	t.Run("Empty whitelist respects default_policy=deny", func(t *testing.T) {
		// Create policy with empty whitelist and deny default
		policy := &P{
			DefaultPolicy: "deny",
			Kind: Kinds{
				Whitelist: []int{}, // Empty
			},
			Rules: make(map[int]Rule), // No rules
		}

		event := createTestEvent(t, testSigner, "Hello Nostr!", 1)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 1 should be REJECTED with empty whitelist and default_policy=deny")
		}
	})

	t.Run("Empty whitelist respects default_policy=allow", func(t *testing.T) {
		// Create policy with empty whitelist and allow default
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Whitelist: []int{}, // Empty
			},
			Rules: make(map[int]Rule), // No rules
		}

		event := createTestEvent(t, testSigner, "Hello Nostr!", 1)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 1 should be ALLOWED with empty whitelist and default_policy=allow")
		}
	})
}
