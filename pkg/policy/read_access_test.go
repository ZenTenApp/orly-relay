package policy

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// TestReadAllowLogic tests the correct semantics of ReadAllow:
// ReadAllow should control WHO can read events of a kind,
// not which event authors can be read.
func TestReadAllowLogic(t *testing.T) {
	// Set up: Create 3 different users
	// - alice: will author an event
	// - bob: will be allowed to read (in ReadAllow list)
	// - charlie: will NOT be allowed to read (not in ReadAllow list)

	aliceSigner, alicePubkey := generateTestKeypair(t)
	_, bobPubkey := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)

	// Create an event authored by Alice (kind 30166)
	aliceEvent := createTestEvent(t, aliceSigner, "server heartbeat", 30166)

	// Create policy: Only Bob can READ kind 30166 events
	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			30166: {
				Description: "Private server heartbeat events",
				AccessControl: AccessControl{
					ReadAllow: []string{hex.Enc(bobPubkey)}, // Only Bob can read
				},
			},
		},
	}

	// Test 1: Bob (who is in ReadAllow) should be able to READ Alice's event
	t.Run("allowed_reader_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Bob should be allowed to READ Alice's event (Bob is in ReadAllow list)")
		}
	})

	// Test 2: Charlie (who is NOT in ReadAllow) should NOT be able to READ Alice's event
	t.Run("disallowed_reader_cannot_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Charlie should NOT be allowed to READ Alice's event (Charlie is not in ReadAllow list)")
		}
	})

	// Test 3: Alice (the author) should NOT be able to READ her own event if she's not in ReadAllow
	t.Run("author_not_in_readallow_cannot_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Alice should NOT be allowed to READ her own event (Alice is not in ReadAllow list)")
		}
	})

	// Test 4: Unauthenticated user should NOT be able to READ
	t.Run("unauthenticated_cannot_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, nil, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Unauthenticated user should NOT be allowed to READ (not in ReadAllow list)")
		}
	})
}

// TestReadDenyLogic tests the correct semantics of ReadDeny:
// ReadDeny should control WHO cannot read events of a kind,
// not which event authors cannot be read.
func TestReadDenyLogic(t *testing.T) {
	// Set up: Create 3 different users
	aliceSigner, alicePubkey := generateTestKeypair(t)
	_, bobPubkey := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)

	// Create an event authored by Alice
	aliceEvent := createTestEvent(t, aliceSigner, "test content", 1)

	// Create policy: Charlie cannot READ kind 1 events (but others can)
	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			1: {
				Description: "Test events",
				AccessControl: AccessControl{
					ReadDeny: []string{hex.Enc(charliePubkey)}, // Charlie cannot read
				},
			},
		},
	}

	// Test 1: Bob (who is NOT in ReadDeny) should be able to READ Alice's event
	t.Run("non_denied_reader_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Bob should be allowed to READ Alice's event (Bob is not in ReadDeny list)")
		}
	})

	// Test 2: Charlie (who IS in ReadDeny) should NOT be able to READ Alice's event
	t.Run("denied_reader_cannot_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Charlie should NOT be allowed to READ Alice's event (Charlie is in ReadDeny list)")
		}
	})

	// Test 3: Alice (the author, not in ReadDeny) should be able to READ her own event
	t.Run("author_not_denied_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Alice should be allowed to READ her own event (Alice is not in ReadDeny list)")
		}
	})
}

// TestSamplePolicyFromUser tests the exact policy configuration provided by the user
func TestSamplePolicyFromUser(t *testing.T) {
	policyJSON := []byte(`{
		"kind": {
			"whitelist": [4678, 10306, 30520, 30919, 30166]
		},
		"rules": {
			"4678": {
				"description": "Zenotp message events",
				"write_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5",
					"e4101949fb0367c72f5105fc9bd810cde0e0e0f950da26c1f47a6af5f77ded31",
					"3f5fefcdc3fb41f3b299732acad7dc9c3649e8bde97d4f238380dde547b5e0e0"
				],
				"privileged": true
			},
			"10306": {
				"description": "End user whitelist change requests",
				"read_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
				],
				"privileged": true
			},
			"30520": {
				"description": "End user whitelist events",
				"write_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
				],
				"privileged": true
			},
			"30919": {
				"description": "Customer indexing events",
				"write_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
				],
				"privileged": true
			},
			"30166": {
				"description": "Private server heartbeat events",
				"write_allow": [
					"4d13154d82477a2d2e07a5c0d52def9035fdf379ae87cd6f0a5fb87801a4e5e4",
					"e400106ed10310ea28b039e81824265434bf86ece58722655c7a98f894406112"
				],
				"read_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5",
					"4d13154d82477a2d2e07a5c0d52def9035fdf379ae87cd6f0a5fb87801a4e5e4",
					"e400106ed10310ea28b039e81824265434bf86ece58722655c7a98f894406112"
				]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Define the test users
	adminPubkeyHex := "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
	server1PubkeyHex := "4d13154d82477a2d2e07a5c0d52def9035fdf379ae87cd6f0a5fb87801a4e5e4"
	server2PubkeyHex := "e400106ed10310ea28b039e81824265434bf86ece58722655c7a98f894406112"

	adminPubkey, _ := hex.Dec(adminPubkeyHex)
	server1Pubkey, _ := hex.Dec(server1PubkeyHex)
	server2Pubkey, _ := hex.Dec(server2PubkeyHex)

	// Create a random user not in any allow list
	randomSigner, randomPubkey := generateTestKeypair(t)

	// Test Kind 30166 (Private server heartbeat events)
	t.Run("kind_30166_read_access", func(t *testing.T) {
		// We can't sign with the exact pubkey without the private key,
		// so we'll create a generic event and manually set the pubkey for testing
		heartbeatEvent := createTestEvent(t, randomSigner, "heartbeat data", 30166)
		heartbeatEvent.Pubkey = server1Pubkey // Set to server1's pubkey

		// Test 1: Admin (in read_allow) should be able to READ the heartbeat
		allowed, err := policy.CheckPolicy("read", heartbeatEvent, adminPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Admin should be allowed to READ kind 30166 events (admin is in read_allow list)")
		}

		// Test 2: Server1 (in read_allow) should be able to READ the heartbeat
		allowed, err = policy.CheckPolicy("read", heartbeatEvent, server1Pubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Server1 should be allowed to READ kind 30166 events (server1 is in read_allow list)")
		}

		// Test 3: Server2 (in read_allow) should be able to READ the heartbeat
		allowed, err = policy.CheckPolicy("read", heartbeatEvent, server2Pubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Server2 should be allowed to READ kind 30166 events (server2 is in read_allow list)")
		}

		// Test 4: Random user (NOT in read_allow) should NOT be able to READ the heartbeat
		allowed, err = policy.CheckPolicy("read", heartbeatEvent, randomPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Random user should NOT be allowed to READ kind 30166 events (not in read_allow list)")
		}

		// Test 5: Unauthenticated user should NOT be able to READ (privileged + read_allow)
		allowed, err = policy.CheckPolicy("read", heartbeatEvent, nil, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Unauthenticated user should NOT be allowed to READ kind 30166 events (privileged)")
		}
	})

	// Test Kind 10306 (End user whitelist change requests)
	t.Run("kind_10306_read_access", func(t *testing.T) {
		// Create an event authored by a random user
		requestEvent := createTestEvent(t, randomSigner, "whitelist change request", 10306)
		// Add admin to p tag to satisfy privileged requirement
		addPTag(requestEvent, adminPubkey)

		// Test 1: Admin (in read_allow) should be able to READ the request
		allowed, err := policy.CheckPolicy("read", requestEvent, adminPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Admin should be allowed to READ kind 10306 events (admin is in read_allow list)")
		}

		// Test 2: Server1 (NOT in read_allow for kind 10306) should NOT be able to READ
		// Even though server1 might be allowed for kind 30166
		allowed, err = policy.CheckPolicy("read", requestEvent, server1Pubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Server1 should NOT be allowed to READ kind 10306 events (not in read_allow list for this kind)")
		}

		// Test 3: Random user (author) SHOULD be able to READ
		// OR logic: Random user is the author so privileged check passes -> ALLOWED
		allowed, err = policy.CheckPolicy("read", requestEvent, randomPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Random user SHOULD be allowed to READ kind 10306 events (author - privileged check passes, OR logic)")
		}
	})
}

// TestReadAllowWithPrivileged tests interaction between read_allow and privileged
func TestReadAllowWithPrivileged(t *testing.T) {
	aliceSigner, alicePubkey := generateTestKeypair(t)
	_, bobPubkey := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)

	// Create policy: Kind 100 is privileged AND has read_allow
	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			100: {
				Description: "Privileged with read_allow",
				Constraints: Constraints{
					Privileged: true,
				},
				AccessControl: AccessControl{
					ReadAllow: []string{hex.Enc(bobPubkey)}, // Only Bob can read
				},
			},
		},
	}

	// Create event authored by Alice, with Bob in p tag
	ev := createTestEvent(t, aliceSigner, "secret message", 100)
	addPTag(ev, bobPubkey)

	// Test 1: Bob (in ReadAllow AND in p tag) should be able to READ
	t.Run("bob_in_readallow_and_ptag", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Bob should be allowed to READ (in ReadAllow AND satisfies privileged)")
		}
	})

	// Test 2: Alice (author, but NOT in ReadAllow) SHOULD be able to READ
	// OR logic: Alice is involved (author) so privileged check passes -> ALLOWED
	t.Run("alice_author_but_not_in_readallow", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Alice SHOULD be allowed to READ (privileged check passes - she's the author, OR logic)")
		}
	})

	// Test 3: Charlie (NOT in ReadAllow, NOT in p tag) should NOT be able to READ
	t.Run("charlie_not_authorized", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", ev, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Charlie should NOT be allowed to READ (not in ReadAllow)")
		}
	})

	// Test 4: Create event with Charlie in p tag but Charlie not in ReadAllow
	evWithCharlie := createTestEvent(t, aliceSigner, "message for charlie", 100)
	addPTag(evWithCharlie, charliePubkey)

	t.Run("charlie_in_ptag_but_not_readallow", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", evWithCharlie, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Charlie SHOULD be allowed to READ (privileged check passes - he's in p-tag, OR logic)")
		}
	})
}

// TestReadAllowWriteAllowIndependent verifies that read_allow and write_allow are independent
func TestReadAllowWriteAllowIndependent(t *testing.T) {
	aliceSigner, alicePubkey := generateTestKeypair(t)
	bobSigner, bobPubkey := generateTestKeypair(t)
	_, charliePubkey := generateTestKeypair(t)

	// Create policy:
	// - Alice can WRITE
	// - Bob can READ
	// - Charlie can do neither
	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			200: {
				Description: "Write/Read separation test",
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(alicePubkey)}, // Only Alice can write
					ReadAllow:  []string{hex.Enc(bobPubkey)},   // Only Bob can read
				},
			},
		},
	}

	// Alice creates an event
	aliceEvent := createTestEvent(t, aliceSigner, "alice's message", 200)

	// Test 1: Alice can WRITE her own event
	t.Run("alice_can_write", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("write", aliceEvent, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Alice should be allowed to WRITE (in WriteAllow)")
		}
	})

	// Test 2: Alice CANNOT READ her own event (not in ReadAllow)
	t.Run("alice_cannot_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, alicePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Alice should NOT be allowed to READ (not in ReadAllow, even though she wrote it)")
		}
	})

	// Bob creates an event (will be denied on write)
	bobEvent := createTestEvent(t, bobSigner, "bob's message", 200)

	// Test 3: Bob CANNOT WRITE (not in WriteAllow)
	t.Run("bob_cannot_write", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("write", bobEvent, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Bob should NOT be allowed to WRITE (not in WriteAllow)")
		}
	})

	// Test 4: Bob CAN READ Alice's event (in ReadAllow)
	t.Run("bob_can_read", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", aliceEvent, bobPubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Bob should be allowed to READ Alice's event (in ReadAllow)")
		}
	})

	// Test 5: Charlie cannot write or read
	t.Run("charlie_cannot_write_or_read", func(t *testing.T) {
		// Create an event authored by Charlie
		charlieSigner := p8k.MustNew()
		charlieSigner.Generate()
		charlieEvent := createTestEvent(t, charlieSigner, "charlie's message", 200)
		charlieEvent.Pubkey = charliePubkey // Set to Charlie's pubkey

		// Charlie's event should be denied for write (Charlie not in WriteAllow)
		allowed, err := policy.CheckPolicy("write", charlieEvent, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Charlie should NOT be allowed to WRITE events of kind 200 (not in WriteAllow)")
		}

		// Charlie should not be able to READ Alice's event (not in ReadAllow)
		allowed, err = policy.CheckPolicy("read", aliceEvent, charliePubkey, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Charlie should NOT be allowed to READ (not in ReadAllow)")
		}
	})
}

// TestReadAccessEdgeCases tests edge cases like nil pubkeys
func TestReadAccessEdgeCases(t *testing.T) {
	aliceSigner, _ := generateTestKeypair(t)

	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			300: {
				Description: "Test edge cases",
				AccessControl: AccessControl{
					ReadAllow: []string{"somepubkey"}, // Non-empty ReadAllow
				},
			},
		},
	}

	event := createTestEvent(t, aliceSigner, "test", 300)

	// Test 1: Nil loggedInPubkey with ReadAllow should be denied
	t.Run("nil_pubkey_with_readallow", func(t *testing.T) {
		allowed, err := policy.CheckPolicy("read", event, nil, "127.0.0.1")
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Nil pubkey should NOT be allowed when ReadAllow is set")
		}
	})

	// Test 2: Verify hex.Enc(nil) doesn't accidentally match anything
	t.Run("hex_enc_nil_no_match", func(t *testing.T) {
		emptyStringHex := hex.Enc(nil)
		t.Logf("hex.Enc(nil) = %q (len=%d)", emptyStringHex, len(emptyStringHex))

		// Verify it's empty string
		if emptyStringHex != "" {
			t.Errorf("Expected hex.Enc(nil) to be empty string, got %q", emptyStringHex)
		}
	})
}
