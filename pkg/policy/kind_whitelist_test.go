package policy

import (
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// TestKindWhitelistComprehensive verifies that kind whitelisting properly rejects
// unlisted kinds in all scenarios: explicit whitelist, implicit whitelist (rules), and combinations
func TestKindWhitelistComprehensive(t *testing.T) {
	testSigner, testPubkey := generateTestKeypair(t)

	t.Run("Explicit Whitelist - kind IN whitelist, HAS rule", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow", // Changed to allow so rules without constraints allow by default
			Kind: Kinds{
				Whitelist: []int{1, 3, 5}, // Explicit whitelist
			},
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
				3: {Description: "Rule for kind 3"},
				5: {Description: "Rule for kind 5"},
			},
		}

		event := createTestEvent(t, testSigner, "test", 1)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 1 should be ALLOWED (in whitelist, has rule, passes rule check)")
		}
	})

	t.Run("Explicit Whitelist - kind IN whitelist, NO rule", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Whitelist: []int{1, 3, 5}, // Explicit whitelist
			},
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
				// Kind 3 has no rule
			},
		}

		event := createTestEvent(t, testSigner, "test", 3)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 3 should be ALLOWED (in whitelist, no rule, default policy is allow)")
		}
	})

	t.Run("Explicit Whitelist - kind NOT in whitelist, HAS rule", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Whitelist: []int{1, 3, 5}, // Explicit whitelist - kind 10 NOT included
			},
			rules: map[int]Rule{
				1:  {Description: "Rule for kind 1"},
				10: {Description: "Rule for kind 10"}, // Has rule but not in whitelist!
			},
		}

		event := createTestEvent(t, testSigner, "test", 10)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 10 should be REJECTED (NOT in whitelist, even though it has a rule)")
		}
	})

	t.Run("Explicit Whitelist - kind NOT in whitelist, NO rule", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Whitelist: []int{1, 3, 5}, // Explicit whitelist
			},
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
			},
		}

		event := createTestEvent(t, testSigner, "test", 99)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 99 should be REJECTED (NOT in whitelist)")
		}
	})

	t.Run("Implicit Whitelist (rules) - kind HAS rule", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow", // Changed to allow so rules without constraints allow by default
			// No explicit whitelist
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
				3: {Description: "Rule for kind 3"},
			},
		}

		event := createTestEvent(t, testSigner, "test", 1)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 1 should be ALLOWED (has rule, implicit whitelist)")
		}
	})

	t.Run("Implicit Whitelist (rules) - kind NO rule", func(t *testing.T) {
		policy := &P{
			// DefaultPolicy not set (empty) - uses implicit whitelist when rules exist
			// No explicit whitelist
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
				3: {Description: "Rule for kind 3"},
			},
		}

		event := createTestEvent(t, testSigner, "test", 99)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 99 should be REJECTED (no rule, implicit whitelist mode)")
		}
	})

	t.Run("Explicit Whitelist + Global Rule - kind NOT in whitelist", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Whitelist: []int{1, 3, 5}, // Explicit whitelist
			},
			Global: Rule{
				Description: "Global rule applies to all kinds",
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(testPubkey)},
				},
			},
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
			},
		}

		// Even with global rule, kind not in whitelist should be rejected
		event := createTestEvent(t, testSigner, "test", 99)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 99 should be REJECTED (NOT in whitelist, even with global rule)")
		}
	})

	t.Run("Blacklist + Rules - kind in blacklist but has rule", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Blacklist: []int{10, 20}, // Blacklist
			},
			rules: map[int]Rule{
				1:  {Description: "Rule for kind 1"},
				10: {Description: "Rule for kind 10"}, // Has rule but blacklisted!
			},
		}

		// Kind 10 is blacklisted, should be rejected
		event := createTestEvent(t, testSigner, "test", 10)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 10 should be REJECTED (in blacklist)")
		}
	})

	t.Run("Blacklist + Rules - kind NOT in blacklist but NO rule (implicit whitelist)", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Blacklist: []int{10, 20}, // Blacklist
			},
			rules: map[int]Rule{
				1: {Description: "Rule for kind 1"},
				3: {Description: "Rule for kind 3"},
			},
		}

		// Kind 99 is not blacklisted but has no rule
		// With blacklist present + rules, implicit whitelist applies
		event := createTestEvent(t, testSigner, "test", 99)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 99 should be REJECTED (not in blacklist but no rule, implicit whitelist)")
		}
	})

	t.Run("Whitelist takes precedence over Blacklist", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			Kind: Kinds{
				Whitelist: []int{1, 3, 5, 10}, // Whitelist includes 10
				Blacklist: []int{10, 20},      // Blacklist also includes 10
			},
			rules: map[int]Rule{
				1:  {Description: "Rule for kind 1"},
				10: {Description: "Rule for kind 10"},
			},
		}

		// Kind 10 is in BOTH whitelist and blacklist - whitelist should win
		event := createTestEvent(t, testSigner, "test", 10)
		allowed, err := policy.CheckPolicy("write", event, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 10 should be ALLOWED (whitelist takes precedence over blacklist)")
		}
	})
}

// TestKindWhitelistRealWorld tests real-world scenarios from the documentation
func TestKindWhitelistRealWorld(t *testing.T) {
	testSigner, testPubkey := generateTestKeypair(t)
	_, otherPubkey := generateTestKeypair(t)

	t.Run("Real World: Only allow kinds 1, 3, 30023", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow", // Allow by default for kinds in whitelist
			Kind: Kinds{
				Whitelist: []int{1, 3, 30023},
			},
			rules: map[int]Rule{
				1: {
					Description: "Text notes",
					// No WriteAllow = anyone authenticated can write
				},
				3: {
					Description: "Contact lists",
					// No WriteAllow = anyone authenticated can write
				},
				30023: {
					Description: "Long-form content",
					AccessControl: AccessControl{
						WriteAllow: []string{hex.Enc(testPubkey)}, // Only specific user can write
					},
				},
			},
		}

		// Test kind 1 (allowed)
		event1 := createTestEvent(t, testSigner, "Hello world", 1)
		allowed, err := policy.CheckPolicy("write", event1, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 1 should be ALLOWED")
		}

		// Test kind 4 (NOT in whitelist, should be rejected)
		event4 := createTestEvent(t, testSigner, "DM", 4)
		allowed, err = policy.CheckPolicy("write", event4, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 4 should be REJECTED (not in whitelist)")
		}

		// Test kind 30023 by authorized user (allowed)
		event30023Auth := createTestEvent(t, testSigner, "Article", 30023)
		allowed, err = policy.CheckPolicy("write", event30023Auth, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Kind 30023 should be ALLOWED for authorized user")
		}

		// Test kind 30023 by unauthorized user (should fail rule check)
		event30023Unauth := createTestEvent(t, testSigner, "Article", 30023)
		allowed, err = policy.CheckPolicy("write", event30023Unauth, otherPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 30023 should be REJECTED for unauthorized user")
		}

		// Test kind 9735 (NOT in whitelist, should be rejected even with valid signature)
		event9735 := createTestEvent(t, testSigner, "Zap", 9735)
		allowed, err = policy.CheckPolicy("write", event9735, testPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Kind 9735 should be REJECTED (not in whitelist)")
		}
	})
}
