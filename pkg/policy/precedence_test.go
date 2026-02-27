package policy

import (
	"testing"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

// TestPolicyPrecedenceRules verifies the correct evaluation order and precedence
// of different policy fields, clarifying the exact behavior after fixes.
//
// Evaluation Order (as fixed):
// 1. Universal constraints (size, tags, timestamps)
// 2. Explicit denials (highest priority)
// 3. Privileged access (ONLY if no allow lists)
// 4. Exclusive allow lists (authoritative when present)
// 5. Privileged final check
// 6. Default policy
func TestPolicyPrecedenceRules(t *testing.T) {
	// Generate test keypairs
	aliceSigner := p8k.MustNew()
	if err := aliceSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate alice signer: %v", err)
	}
	alicePubkey := aliceSigner.Pub()

	bobSigner := p8k.MustNew()
	if err := bobSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate bob signer: %v", err)
	}
	bobPubkey := bobSigner.Pub()

	charlieSigner := p8k.MustNew()
	if err := charlieSigner.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate charlie signer: %v", err)
	}
	charliePubkey := charlieSigner.Pub()

	// ===================================================================
	// Test 1: Deny List Has Highest Priority
	// ===================================================================
	t.Run("Deny List Overrides Everything", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			rules: map[int]Rule{
				100: {
					Description: "Deny overrides allow and privileged",
					AccessControl: AccessControl{
						WriteAllow: []string{hex.Enc(alicePubkey)}, // Alice in allow list
						WriteDeny:  []string{hex.Enc(alicePubkey)}, // But also in deny list
					},
					Constraints: Constraints{
						Privileged: true, // And it's privileged
					},
				},
			},
		}

		// Alice creates an event (she's author, in allow list, but also in deny list)
		event := createTestEvent(t, aliceSigner, "test", 100)

		// Should be DENIED because deny list has highest priority
		allowed, err := policy.CheckPolicy("write", event, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: User in deny list should be denied even if in allow list and privileged")
		} else {
			t.Log("PASS: Deny list correctly overrides allow list and privileged")
		}
	})

	// ===================================================================
	// Test 2: Allow List OR Privileged (Either grants access)
	// ===================================================================
	t.Run("Allow List OR Privileged Access", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			rules: map[int]Rule{
				200: {
					Description: "Privileged with allow list",
					AccessControl: AccessControl{
						ReadAllow: []string{hex.Enc(bobPubkey)}, // Only Bob in allow list
					},
					Constraints: Constraints{
						Privileged: true,
					},
				},
			},
		}

		// Alice creates event
		event := createTestEvent(t, aliceSigner, "secret", 200)

		// Test 2a: Alice is author (privileged) but NOT in allow list - should be ALLOWED (OR logic)
		allowed, err := policy.CheckPolicy("read", event, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: Author should be allowed via privileged (OR logic)")
		} else {
			t.Log("PASS: Author allowed via privileged despite not in allow list (OR logic)")
		}

		// Test 2b: Bob is in allow list - should be ALLOWED
		allowed, err = policy.CheckPolicy("read", event, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: User in allow list should be allowed")
		} else {
			t.Log("PASS: User in allow list correctly allowed")
		}

		// Test 2c: Charlie in p-tag but not in allow list - should be ALLOWED (OR logic)
		addPTag(event, charliePubkey)
		allowed, err = policy.CheckPolicy("read", event, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: User in p-tag should be allowed via privileged (OR logic)")
		} else {
			t.Log("PASS: User in p-tag allowed via privileged despite not in allow list (OR logic)")
		}
	})

	// ===================================================================
	// Test 3: Privileged Without Allow List Grants Access
	// ===================================================================
	t.Run("Privileged Grants Access When No Allow List", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "deny", // Default deny to make test clearer
			rules: map[int]Rule{
				300: {
					Description: "Privileged without allow list",
					Constraints: Constraints{
						Privileged: true,
					},
					// NO ReadAllow or WriteAllow specified
				},
			},
		}

		// Alice creates event with Bob in p-tag
		event := createTestEvent(t, aliceSigner, "message", 300)
		addPTag(event, bobPubkey)

		// Test 3a: Alice (author) should be ALLOWED (privileged, no allow list)
		allowed, err := policy.CheckPolicy("read", event, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}


		if !allowed {
			t.Error("FAIL: Author should be allowed when privileged and no allow list")
		} else {
			t.Log("PASS: Privileged correctly grants access to author when no allow list")
		}

		// Test 3b: Bob (in p-tag) should be ALLOWED (privileged, no allow list)
		allowed, err = policy.CheckPolicy("read", event, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: P-tagged user should be allowed when privileged and no allow list")
		} else {
			t.Log("PASS: Privileged correctly grants access to p-tagged user when no allow list")
		}

		// Test 3c: Charlie (not involved) should be DENIED
		allowed, err = policy.CheckPolicy("read", event, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: Non-involved user should be denied for privileged event")
		} else {
			t.Log("PASS: Privileged correctly denies non-involved user")
		}
	})

	// ===================================================================
	// Test 4: Allow List Without Privileged Is Exclusive
	// ===================================================================
	t.Run("Allow List Exclusive Without Privileged", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow", // Even with allow default
			rules: map[int]Rule{
				400: {
					Description: "Allow list only",
					AccessControl: AccessControl{
						WriteAllow: []string{hex.Enc(alicePubkey)}, // Only Alice
					},
					// NO Privileged flag
				},
			},
		}

		// Test 4a: Alice should be ALLOWED (in allow list)
		aliceEvent := createTestEvent(t, aliceSigner, "alice msg", 400)
		allowed, err := policy.CheckPolicy("write", aliceEvent, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: User in allow list should be allowed")
		} else {
			t.Log("PASS: Allow list correctly allows listed user")
		}

		// Test 4b: Bob should be DENIED (not in allow list, even with allow default)
		bobEvent := createTestEvent(t, bobSigner, "bob msg", 400)
		allowed, err = policy.CheckPolicy("write", bobEvent, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: User not in allow list should be denied despite allow default")
		} else {
			t.Log("PASS: Allow list correctly excludes non-listed user")
		}
	})

	// ===================================================================
	// Test 5: Complex Precedence Chain
	// ===================================================================
	t.Run("Complex Precedence Chain", func(t *testing.T) {
		policy := &P{
			DefaultPolicy: "allow",
			rules: map[int]Rule{
				500: {
					Description: "Complex rules",
					AccessControl: AccessControl{
						WriteAllow: []string{hex.Enc(alicePubkey), hex.Enc(bobPubkey)},
						WriteDeny:  []string{hex.Enc(bobPubkey)}, // Bob denied despite being in allow
					},
					Constraints: Constraints{
						Privileged: true,
					},
				},
			},
		}

		// Test 5a: Alice in allow, not in deny - ALLOWED
		aliceEvent := createTestEvent(t, aliceSigner, "alice", 500)
		allowed, err := policy.CheckPolicy("write", aliceEvent, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: Alice should be allowed (in allow, not in deny)")
		} else {
			t.Log("PASS: User in allow and not in deny is allowed")
		}

		// Test 5b: Bob in allow AND deny - DENIED (deny wins)
		bobEvent := createTestEvent(t, bobSigner, "bob", 500)
		allowed, err = policy.CheckPolicy("write", bobEvent, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: Bob should be denied (deny list overrides allow list)")
		} else {
			t.Log("PASS: Deny list correctly overrides allow list")
		}

		// Test 5c: Charlie not in allow - DENIED (even though he's author of his event)
		charlieEvent := createTestEvent(t, charlieSigner, "charlie", 500)
		allowed, err = policy.CheckPolicy("write", charlieEvent, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: Charlie should be denied (not in allow list)")
		} else {
			t.Log("PASS: Allow list correctly excludes non-listed privileged author")
		}
	})

	// ===================================================================
	// Test 6: Default Policy Application
	// ===================================================================
	t.Run("Default Policy Only When No Rules", func(t *testing.T) {
		// Test 6a: With allow default and no rules
		policyAllow := &P{
			DefaultPolicy: "allow",
			rules: map[int]Rule{
				// No rule for kind 600
			},
		}

		event := createTestEvent(t, aliceSigner, "test", 600)
		allowed, err := policyAllow.CheckPolicy("write", event, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("FAIL: Default allow should permit when no rules")
		} else {
			t.Log("PASS: Default allow correctly applied when no rules")
		}

		// Test 6b: With deny default and no rules
		policyDeny := &P{
			DefaultPolicy: "deny",
			rules: map[int]Rule{
				// No rule for kind 600
			},
		}

		allowed, err = policyDeny.CheckPolicy("write", event, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: Default deny should block when no rules")
		} else {
			t.Log("PASS: Default deny correctly applied when no rules")
		}

		// Test 6c: Default does NOT apply when allow list exists
		policyWithRule := &P{
			DefaultPolicy: "allow", // Allow default
			rules: map[int]Rule{
				700: {
					AccessControl: AccessControl{
						WriteAllow: []string{hex.Enc(bobPubkey)}, // Only Bob
					},
				},
			},
		}

		eventKind700 := createTestEvent(t, aliceSigner, "alice", 700)
		allowed, err = policyWithRule.CheckPolicy("write", eventKind700, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("FAIL: Default allow should NOT override exclusive allow list")
		} else {
			t.Log("PASS: Allow list correctly overrides default policy")
		}
	})
}
