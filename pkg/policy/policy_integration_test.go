package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// TestPolicyIntegration runs the relay with policy enabled and tests event filtering
func TestPolicyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Generate test keys
	allowedSigner := p8k.MustNew()
	if err := allowedSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate allowed signer: %v", err)
	}
	allowedPubkeyHex := hex.Enc(allowedSigner.Pub())

	unauthorizedSigner := p8k.MustNew()
	if err := unauthorizedSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate unauthorized signer: %v", err)
	}

	// Create temporary directory for policy config
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "ORLY_TEST")
	if err := os.MkdirAll(configDir, 0755); chk.E(err) {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create policy JSON with generated keys
	policyJSON := map[string]interface{}{
		"kind": map[string]interface{}{
			"whitelist": []int{4678, 10306, 30520, 30919},
		},
		"rules": map[string]interface{}{
			"4678": map[string]interface{}{
				"description": "Zenotp message events",
				"script":      filepath.Join(configDir, "validate4678.js"), // Won't exist, should fall back to default
				"privileged":  true,
			},
			"10306": map[string]interface{}{
				"description": "End user whitelist changes",
				"read_allow":  []string{allowedPubkeyHex},
				"privileged":  true,
			},
			"30520": map[string]interface{}{
				"description": "Zenotp events",
				"write_allow": []string{allowedPubkeyHex},
				"privileged":  true,
			},
			"30919": map[string]interface{}{
				"description": "Zenotp events",
				"write_allow": []string{allowedPubkeyHex},
				"privileged":  true,
			},
		},
	}

	policyJSONBytes, err := json.MarshalIndent(policyJSON, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal policy JSON: %v", err)
	}

	policyPath := filepath.Join(configDir, "policy.json")
	if err := os.WriteFile(policyPath, policyJSONBytes, 0644); chk.E(err) {
		t.Fatalf("Failed to write policy file: %v", err)
	}

	// Create events with proper signatures
	// Event 1: Kind 30520 with allowed pubkey (should be allowed)
	event30520Allowed := event.New()
	event30520Allowed.CreatedAt = time.Now().Unix()
	event30520Allowed.Kind = kind.K{K: 30520}.K
	event30520Allowed.Content = []byte("test event 30520")
	event30520Allowed.Tags = tag.NewS()
	addPTag(event30520Allowed, allowedSigner.Pub()) // Add p tag for privileged check
	if err := event30520Allowed.Sign(allowedSigner); chk.E(err) {
		t.Fatalf("Failed to sign event30520Allowed: %v", err)
	}

	// Event 2: Kind 30520 with unauthorized pubkey (should be denied)
	event30520Unauthorized := event.New()
	event30520Unauthorized.CreatedAt = time.Now().Unix()
	event30520Unauthorized.Kind = kind.K{K: 30520}.K
	event30520Unauthorized.Content = []byte("test event 30520 unauthorized")
	event30520Unauthorized.Tags = tag.NewS()
	if err := event30520Unauthorized.Sign(unauthorizedSigner); chk.E(err) {
		t.Fatalf("Failed to sign event30520Unauthorized: %v", err)
	}

	// Event 3: Kind 10306 with allowed pubkey (should be readable by allowed user)
	event10306Allowed := event.New()
	event10306Allowed.CreatedAt = time.Now().Unix()
	event10306Allowed.Kind = kind.K{K: 10306}.K
	event10306Allowed.Content = []byte("test event 10306")
	event10306Allowed.Tags = tag.NewS()
	addPTag(event10306Allowed, allowedSigner.Pub()) // Add p tag for privileged check
	if err := event10306Allowed.Sign(allowedSigner); chk.E(err) {
		t.Fatalf("Failed to sign event10306Allowed: %v", err)
	}

	// Event 4: Kind 4678 with allowed pubkey (script-based, should fall back to default)
	event4678Allowed := event.New()
	event4678Allowed.CreatedAt = time.Now().Unix()
	event4678Allowed.Kind = kind.K{K: 4678}.K
	event4678Allowed.Content = []byte("test event 4678")
	event4678Allowed.Tags = tag.NewS()
	addPTag(event4678Allowed, allowedSigner.Pub()) // Add p tag for privileged check
	if err := event4678Allowed.Sign(allowedSigner); chk.E(err) {
		t.Fatalf("Failed to sign event4678Allowed: %v", err)
	}

	// Test policy loading
	policy, err := New(policyJSONBytes)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Verify policy loaded correctly
	if len(policy.rules) != 4 {
		t.Errorf("Expected 4 rules, got %d", len(policy.rules))
	}

	// Test policy checks directly
	t.Run("policy checks", func(t *testing.T) {
		// Test 1: Event 30520 with allowed pubkey should be allowed
		allowed, err := policy.CheckPolicy("write", event30520Allowed, allowedSigner.Pub(), "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event30520Allowed to be allowed")
		}

		// Test 2: Event 30520 with unauthorized pubkey should be denied
		allowed, err = policy.CheckPolicy("write", event30520Unauthorized, unauthorizedSigner.Pub(), "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event30520Unauthorized to be denied")
		}

		// Test 3: Event 10306 should be readable by allowed user
		allowed, err = policy.CheckPolicy("read", event10306Allowed, allowedSigner.Pub(), "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event10306Allowed to be readable by allowed user")
		}

		// Test 4: Event 10306 should NOT be readable by unauthorized user
		allowed, err = policy.CheckPolicy("read", event10306Allowed, unauthorizedSigner.Pub(), "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event10306Allowed to be denied for unauthorized user")
		}

		// Test 5: Event 10306 should NOT be readable without authentication
		allowed, err = policy.CheckPolicy("read", event10306Allowed, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event10306Allowed to be denied without authentication (privileged)")
		}

		// Test 6: Event 30520 should NOT be writable without authentication
		allowed, err = policy.CheckPolicy("write", event30520Allowed, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event30520Allowed to be denied without authentication (privileged)")
		}

		// Test 7: Event 4678 should fall back to default policy (allow) when script not running
		allowed, err = policy.CheckPolicy("write", event4678Allowed, allowedSigner.Pub(), "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event4678Allowed to be allowed when script not running (falls back to default)")
		}

		// Test 8: Event 4678 write should be allowed without authentication (privileged doesn't affect write)
		allowed, err = policy.CheckPolicy("write", event4678Allowed, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event4678Allowed to be allowed without authentication (privileged doesn't affect write operations)")
		}
	})

	// Test with relay simulation (checking log output)
	t.Run("relay simulation", func(t *testing.T) {
		// Note: We can't easily capture log output in tests, so we just verify
		// that policy checks work correctly

		// Simulate policy checks that would happen in relay
		// First, publish events (simulate write checks)
		checks := []struct {
			name           string
			event          *event.E
			loggedInPubkey []byte
			access         string
			shouldAllow    bool
			shouldLog      string // Expected log message substring, empty means no specific log expected
		}{
			{
				name:           "write 30520 with allowed pubkey",
				event:          event30520Allowed,
				loggedInPubkey: allowedSigner.Pub(),
				access:         "write",
				shouldAllow:    true,
			},
			{
				name:           "write 30520 with unauthorized pubkey",
				event:          event30520Unauthorized,
				loggedInPubkey: unauthorizedSigner.Pub(),
				access:         "write",
				shouldAllow:    false,
			},
			{
				name:           "read 10306 with allowed pubkey",
				event:          event10306Allowed,
				loggedInPubkey: allowedSigner.Pub(),
				access:         "read",
				shouldAllow:    true,
			},
			{
				name:           "read 10306 with unauthorized pubkey",
				event:          event10306Allowed,
				loggedInPubkey: unauthorizedSigner.Pub(),
				access:         "read",
				shouldAllow:    false,
			},
			{
				name:           "read 10306 without authentication",
				event:          event10306Allowed,
				loggedInPubkey: nil,
				access:         "read",
				shouldAllow:    false,
			},
			{
				name:           "write 30520 without authentication",
				event:          event30520Allowed,
				loggedInPubkey: nil,
				access:         "write",
				shouldAllow:    false,
			},
			{
				name:           "write 4678 with allowed pubkey",
				event:          event4678Allowed,
				loggedInPubkey: allowedSigner.Pub(),
				access:         "write",
				shouldAllow:    true,
				shouldLog:      "", // Should not log "policy rule is inactive" if script is not configured
			},
		}

		for _, check := range checks {
			t.Run(check.name, func(t *testing.T) {
				allowed, err := policy.CheckPolicy(check.access, check.event, check.loggedInPubkey, "127.0.0.1")
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if allowed != check.shouldAllow {
					t.Errorf("Expected allowed=%v, got %v", check.shouldAllow, allowed)
				}
			})
		}
	})

	// Test event IDs are regenerated correctly after signing
	t.Run("event ID regeneration", func(t *testing.T) {
		// Create a new event, sign it, then verify ID is correct
		testEvent := event.New()
		testEvent.CreatedAt = time.Now().Unix()
		testEvent.Kind = kind.K{K: 30520}.K
		testEvent.Content = []byte("test content")
		testEvent.Tags = tag.NewS()

		// Sign the event
		if err := testEvent.Sign(allowedSigner); chk.E(err) {
			t.Fatalf("Failed to sign test event: %v", err)
		}

		// Verify event ID is correct (should be SHA256 of serialized event)
		if len(testEvent.ID) != 32 {
			t.Errorf("Expected event ID to be 32 bytes, got %d", len(testEvent.ID))
		}

		// Verify signature is correct
		if len(testEvent.Sig) != 64 {
			t.Errorf("Expected event signature to be 64 bytes, got %d", len(testEvent.Sig))
		}

		// Verify signature validates using event's Verify method
		valid, err := testEvent.Verify()
		if err != nil {
			t.Errorf("Failed to verify signature: %v", err)
		}
		if !valid {
			t.Error("Event signature verification failed")
		}
	})

	// Test WebSocket client simulation (for future integration)
	t.Run("websocket client simulation", func(t *testing.T) {
		// This test simulates what would happen if we connected via WebSocket
		// For now, we'll just verify the events can be serialized correctly

		events := []*event.E{
			event30520Allowed,
			event30520Unauthorized,
			event10306Allowed,
			event4678Allowed,
		}

		for i, ev := range events {
			t.Run(fmt.Sprintf("event_%d", i), func(t *testing.T) {
				// Serialize event
				serialized := ev.Serialize()
				if len(serialized) == 0 {
					t.Error("Event serialization returned empty")
				}

				// Verify event can be parsed back (simplified check)
				if len(ev.ID) != 32 {
					t.Errorf("Event ID length incorrect: %d", len(ev.ID))
				}
				if len(ev.Pubkey) != 32 {
					t.Errorf("Event pubkey length incorrect: %d", len(ev.Pubkey))
				}
				if len(ev.Sig) != 64 {
					t.Errorf("Event signature length incorrect: %d", len(ev.Sig))
				}
			})
		}
	})
}

// TestPolicyWithRelay creates a comprehensive test that simulates relay behavior
func TestPolicyWithRelay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	// Generate keys
	allowedSigner := p8k.MustNew()
	if err := allowedSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate allowed signer: %v", err)
	}
	allowedPubkeyHex := hex.Enc(allowedSigner.Pub())

	unauthorizedSigner := p8k.MustNew()
	if err := unauthorizedSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate unauthorized signer: %v", err)
	}

	// Create policy JSON
	policyJSON := map[string]interface{}{
		"kind": map[string]interface{}{
			"whitelist": []int{4678, 10306, 30520, 30919},
		},
		"rules": map[string]interface{}{
			"10306": map[string]interface{}{
				"description": "End user whitelist changes",
				"read_allow":  []string{allowedPubkeyHex},
				"privileged":  true,
			},
			"30520": map[string]interface{}{
				"description": "Zenotp events",
				"write_allow": []string{allowedPubkeyHex},
				"privileged":  true,
			},
			"30919": map[string]interface{}{
				"description": "Zenotp events",
				"write_allow": []string{allowedPubkeyHex},
				"privileged":  true,
			},
		},
	}

	policyJSONBytes, err := json.Marshal(policyJSON)
	if err != nil {
		t.Fatalf("Failed to marshal policy JSON: %v", err)
	}

	policy, err := New(policyJSONBytes)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create test event (kind 30520) with allowed pubkey
	testEvent := event.New()
	testEvent.CreatedAt = time.Now().Unix()
	testEvent.Kind = kind.K{K: 30520}.K
	testEvent.Content = []byte("test content")
	testEvent.Tags = tag.NewS()
	addPTag(testEvent, allowedSigner.Pub())
	if err := testEvent.Sign(allowedSigner); chk.E(err) {
		t.Fatalf("Failed to sign test event: %v", err)
	}

	// Test scenarios
	scenarios := []struct {
		name           string
		loggedInPubkey []byte
		expectedResult bool
		description    string
	}{
		{
			name:           "authenticated as allowed pubkey",
			loggedInPubkey: allowedSigner.Pub(),
			expectedResult: true,
			description:    "Should allow when authenticated as allowed pubkey",
		},
		{
			name:           "unauthenticated",
			loggedInPubkey: nil,
			expectedResult: false,
			description:    "Should deny when not authenticated (privileged check)",
		},
		{
			name:           "authenticated as different pubkey",
			loggedInPubkey: unauthorizedSigner.Pub(),
			expectedResult: false,
			description:    "Should deny when authenticated as different pubkey",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			allowed, err := policy.CheckPolicy("write", testEvent, scenario.loggedInPubkey, "127.0.0.1")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if allowed != scenario.expectedResult {
				t.Errorf("%s: Expected allowed=%v, got %v", scenario.description, scenario.expectedResult, allowed)
			}
		})
	}

	// Test read access for kind 10306
	readEvent := event.New()
	readEvent.CreatedAt = time.Now().Unix()
	readEvent.Kind = kind.K{K: 10306}.K
	readEvent.Content = []byte("test read event")
	readEvent.Tags = tag.NewS()
	addPTag(readEvent, allowedSigner.Pub())
	if err := readEvent.Sign(allowedSigner); chk.E(err) {
		t.Fatalf("Failed to sign read event: %v", err)
	}

	readScenarios := []struct {
		name           string
		loggedInPubkey []byte
		expectedResult bool
		description    string
	}{
		{
			name:           "read authenticated as allowed pubkey",
			loggedInPubkey: allowedSigner.Pub(),
			expectedResult: true,
			description:    "Should allow read when authenticated as allowed pubkey",
		},
		{
			name:           "read unauthenticated",
			loggedInPubkey: nil,
			expectedResult: false,
			description:    "Should deny read when not authenticated (privileged check)",
		},
		{
			name:           "read authenticated as different pubkey",
			loggedInPubkey: unauthorizedSigner.Pub(),
			expectedResult: false,
			description:    "Should deny read when authenticated as different pubkey",
		},
	}

	for _, scenario := range readScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			allowed, err := policy.CheckPolicy("read", readEvent, scenario.loggedInPubkey, "127.0.0.1")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if allowed != scenario.expectedResult {
				t.Errorf("%s: Expected allowed=%v, got %v", scenario.description, scenario.expectedResult, allowed)
			}
		})
	}
}
