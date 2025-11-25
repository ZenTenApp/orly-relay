package policy

import (
	"encoding/json"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// TestPrivilegedOnlyBug tests the reported bug where privileged flag
// doesn't work when read_allow is missing
func TestPrivilegedOnlyBug(t *testing.T) {
	aliceSigner, alicePubkey := generateTestKeypair(t)
	_, bobPubkey := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)

	// Create policy with ONLY privileged, no read_allow
	policyJSON := map[string]interface{}{
		"rules": map[string]interface{}{
			"4": map[string]interface{}{
				"description": "DM - privileged only",
				"privileged":  true,
			},
		},
	}

	policyBytes, err := json.Marshal(policyJSON)
	if err != nil {
		t.Fatalf("Failed to marshal policy: %v", err)
	}
	t.Logf("Policy JSON: %s", policyBytes)

	policy, err := New(policyBytes)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Verify the rule was loaded correctly
	rule, hasRule := policy.rules[4]
	if !hasRule {
		t.Fatal("Rule for kind 4 was not loaded")
	}
	t.Logf("Loaded rule: %+v", rule)
	t.Logf("Privileged flag: %v", rule.Privileged)
	t.Logf("ReadAllow: %v", rule.ReadAllow)
	t.Logf("readAllowBin: %v", rule.readAllowBin)

	if !rule.Privileged {
		t.Fatal("BUG: Privileged flag was not set to true!")
	}

	// Create a kind 4 (DM) event from Alice to Bob
	ev := createTestEvent(t, aliceSigner, "Secret message from Alice to Bob", 4)
	addPTag(ev, bobPubkey) // Bob is recipient

	t.Logf("Event author: %s", hex.Enc(ev.Pubkey))
	t.Logf("Event p-tags: %v", ev.Tags.GetAll([]byte("p")))

	// Test 1: Alice (author) should be able to read
	t.Run("alice_author_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("BUG! Author should be able to read their own privileged event")
		}
	})

	// Test 2: Bob (in p-tag) should be able to read
	t.Run("bob_recipient_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("BUG! Recipient (in p-tag) should be able to read privileged event")
		}
	})

	// Test 3: Charlie (third party) should NOT be able to read
	t.Run("charlie_third_party_denied", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("BUG! Third party should NOT be able to read privileged event")
		}
	})

	// Test 4: Unauthenticated user should NOT be able to read
	t.Run("unauthenticated_denied", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, nil, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("BUG! Unauthenticated user should NOT be able to read privileged event")
		}
	})
}

// TestPrivilegedWithReadAllowBug tests the scenario where read_allow is present
// and checks if the OR logic works correctly
func TestPrivilegedWithReadAllowBug(t *testing.T) {
	aliceSigner, alicePubkey := generateTestKeypair(t)
	_, bobPubkey := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)
	_, davePubkey := generateTestKeypair(t)

	// Create policy with privileged AND read_allow
	// Expected: OR logic - user can read if in read_allow OR party to event
	policyJSON := map[string]interface{}{
		"rules": map[string]interface{}{
			"4": map[string]interface{}{
				"description": "DM - privileged with read_allow",
				"privileged":  true,
				"read_allow":  []string{hex.Enc(davePubkey)}, // Dave is in read_allow
			},
		},
	}

	policyBytes, err := json.Marshal(policyJSON)
	if err != nil {
		t.Fatalf("Failed to marshal policy: %v", err)
	}
	t.Logf("Policy JSON: %s", policyBytes)

	policy, err := New(policyBytes)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create a kind 4 (DM) event from Alice to Bob (Dave is NOT in p-tag)
	ev := createTestEvent(t, aliceSigner, "Secret message from Alice to Bob", 4)
	addPTag(ev, bobPubkey) // Bob is recipient, Dave is NOT in p-tag

	t.Logf("Event author: %s", hex.Enc(ev.Pubkey))
	t.Logf("Event p-tags: %v", ev.Tags.GetAll([]byte("p")))
	t.Logf("Dave (in read_allow, NOT in p-tag): %s", hex.Enc(davePubkey))

	// Test 1: Alice (author, party) should be able to read via privileged
	t.Run("alice_party_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("BUG! Author should be able to read (privileged)")
		}
	})

	// Test 2: Bob (in p-tag, party) should be able to read via privileged
	t.Run("bob_party_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("BUG! Recipient (in p-tag) should be able to read (privileged)")
		}
	})

	// Test 3: Dave (in read_allow, NOT party) should be able to read via OR logic
	t.Run("dave_read_allow_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, davePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("BUG! User in read_allow should be able to read (OR logic)")
		}
	})

	// Test 4: Charlie (NOT in read_allow, NOT party) should NOT be able to read
	t.Run("charlie_denied", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("BUG! Third party should NOT be able to read")
		}
	})
}

// TestNoReadAllowNoPrivileged tests what happens when a rule has neither
// read_allow nor privileged - should default to allow
func TestNoReadAllowNoPrivileged(t *testing.T) {
	aliceSigner, _ := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)

	// Create policy with a rule that has no read_allow and no privileged
	policyJSON := map[string]interface{}{
		"default_policy": "allow",
		"rules": map[string]interface{}{
			"1": map[string]interface{}{
				"description": "Regular text note - no restrictions",
			},
		},
	}

	policyBytes, err := json.Marshal(policyJSON)
	if err != nil {
		t.Fatalf("Failed to marshal policy: %v", err)
	}
	t.Logf("Policy JSON: %s", policyBytes)

	policy, err := New(policyBytes)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Check that privileged is false for this rule
	rule := policy.rules[1]
	t.Logf("Privileged: %v, ReadAllow: %v", rule.Privileged, rule.ReadAllow)

	// Create a kind 1 event
	ev := createTestEvent(t, aliceSigner, "Hello world", 1)

	// Test: Third party should be able to read (no restrictions)
	t.Run("charlie_can_read_unrestricted", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Third party should be able to read unrestricted events")
		}
	})

	// Test: Unauthenticated should also be able to read
	t.Run("unauthenticated_can_read_unrestricted", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, nil, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Unauthenticated user should be able to read unrestricted events")
		}
	})
}
