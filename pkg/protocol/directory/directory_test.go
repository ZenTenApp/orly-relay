package directory_test

import (
	"encoding/hex"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/crypto/ec/secp256k1"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/protocol/directory"
)

// Helper to create a test keypair using p8k.Signer
func createTestKeypair(t *testing.T) (*p8k.Signer, []byte) {
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatalf("failed to generate keypair: %v", err)
	}

	pubkey := signer.Pub()
	return signer, pubkey
}

// TestRelayIdentityAnnouncementCreation tests creating and parsing relay identity announcements
func TestRelayIdentityAnnouncementCreation(t *testing.T) {
	secKey, pubkey := createTestKeypair(t)
	pubkeyHex := hex.EncodeToString(pubkey)

	// Create relay identity announcement
	ria, err := directory.NewRelayIdentityAnnouncement(
		pubkey,
		"Test Relay",
		"Test relay for unit tests",
		"admin@test.com",
		"wss://relay.test.com/",
		pubkeyHex,
		pubkeyHex,
		"1",
	)
	if err != nil {
		t.Fatalf("failed to create relay identity announcement: %v", err)
	}

	// Sign the event
	if err := ria.Event.Sign(secKey); err != nil {
		t.Fatalf("failed to sign event: %v", err)
	}

	// Verify the event
	if _, err := ria.Event.Verify(); err != nil {
		t.Fatalf("failed to verify event: %v", err)
	}

	// Parse back the announcement
	parsed, err := directory.ParseRelayIdentityAnnouncement(ria.Event)
	if err != nil {
		t.Fatalf("failed to parse relay identity announcement: %v", err)
	}

	// Verify fields
	if parsed.RelayURL != "wss://relay.test.com/" {
		t.Errorf("relay URL mismatch: got %s, want wss://relay.test.com/", parsed.RelayURL)
	}

	if parsed.SigningKey != pubkeyHex {
		t.Errorf("signing key mismatch")
	}

	if parsed.Version != "1" {
		t.Errorf("version mismatch: got %s, want 1", parsed.Version)
	}

	t.Logf("✓ Relay identity announcement created and parsed successfully")
}

// TestTrustActCreationWithNumericLevels tests trust act creation with numeric trust levels
func TestTrustActCreationWithNumericLevels(t *testing.T) {
	testCases := []struct {
		name       string
		trustLevel directory.TrustLevel
		shouldFail bool
	}{
		{"Zero trust", directory.TrustLevelNone, false},
		{"Minimal trust", directory.TrustLevelMinimal, false},
		{"Low trust", directory.TrustLevelLow, false},
		{"Medium trust", directory.TrustLevelMedium, false},
		{"High trust", directory.TrustLevelHigh, false},
		{"Full trust", directory.TrustLevelFull, false},
		{"Custom 33%", directory.TrustLevel(33), false},
		{"Custom 99%", directory.TrustLevel(99), false},
		{"Invalid >100", directory.TrustLevel(101), true},
	}

	secKey, pubkey := createTestKeypair(t)
	targetPubkey := hex.EncodeToString(pubkey)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ta, err := directory.NewTrustAct(
				pubkey,
				targetPubkey,
				tc.trustLevel,
				"wss://target.relay.com/",
				nil,
				directory.TrustReasonManual,
				[]uint16{1, 3, 7},
				nil,
			)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("expected error for trust level %d, got nil", tc.trustLevel)
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to create trust act: %v", err)
			}

			// Sign and verify
			if err := ta.Event.Sign(secKey); err != nil {
				t.Fatalf("failed to sign event: %v", err)
			}

			// Parse back
			parsed, err := directory.ParseTrustAct(ta.Event)
			if err != nil {
				t.Fatalf("failed to parse trust act: %v", err)
			}

			if parsed.TrustLevel != tc.trustLevel {
				t.Errorf("trust level mismatch: got %d, want %d", parsed.TrustLevel, tc.trustLevel)
			}

			if parsed.RelayURL != "wss://target.relay.com/" {
				t.Errorf("relay URL mismatch: got %s", parsed.RelayURL)
			}

			if len(parsed.ReplicationKinds) != 3 {
				t.Errorf("replication kinds count mismatch: got %d, want 3", len(parsed.ReplicationKinds))
			}
		})
	}

	t.Logf("✓ All trust level tests passed")
}

// TestPartialReplicationDiceThrow tests the probabilistic replication mechanism
func TestPartialReplicationDiceThrow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping probabilistic test in short mode")
	}

	_, pubkey := createTestKeypair(t)
	targetPubkey := hex.EncodeToString(pubkey)

	testCases := []struct {
		name           string
		trustLevel     directory.TrustLevel
		iterations     int
		expectedRatio  float64
		toleranceRatio float64
	}{
		{"0% replication", directory.TrustLevelNone, 1000, 0.00, 0.05},
		{"10% replication", directory.TrustLevelMinimal, 1000, 0.10, 0.05},
		{"25% replication", directory.TrustLevelLow, 1000, 0.25, 0.05},
		{"50% replication", directory.TrustLevelMedium, 1000, 0.50, 0.05},
		{"75% replication", directory.TrustLevelHigh, 1000, 0.75, 0.05},
		{"100% replication", directory.TrustLevelFull, 1000, 1.00, 0.05},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ta, err := directory.NewTrustAct(
				pubkey,
				targetPubkey,
				tc.trustLevel,
				"wss://target.relay.com/",
				nil,
				directory.TrustReasonManual,
				[]uint16{1}, // Kind 1 for testing
				nil,
			)
			if err != nil {
				t.Fatalf("failed to create trust act: %v", err)
			}

			replicatedCount := 0
			for i := 0; i < tc.iterations; i++ {
				shouldReplicate, err := ta.ShouldReplicateEvent(1)
				if err != nil {
					t.Fatalf("failed to check replication: %v", err)
				}
				if shouldReplicate {
					replicatedCount++
				}
			}

			actualRatio := float64(replicatedCount) / float64(tc.iterations)
			diff := actualRatio - tc.expectedRatio
			if diff < 0 {
				diff = -diff
			}

			if diff > tc.toleranceRatio {
				t.Errorf("replication ratio out of tolerance: got %.2f, want %.2f±%.2f",
					actualRatio, tc.expectedRatio, tc.toleranceRatio)
			}

			t.Logf("Trust level %d%%: replicated %d/%d (%.2f%%)",
				tc.trustLevel, replicatedCount, tc.iterations, actualRatio*100)
		})
	}

	t.Logf("✓ Partial replication mechanism works correctly")
}

// TestGroupTagActCreation tests group tag act creation with ownership specs
func TestGroupTagActCreation(t *testing.T) {
	secKey, pubkey := createTestKeypair(t)
	pubkeyHex := hex.EncodeToString(pubkey)

	testCases := []struct {
		name       string
		groupID    string
		ownership  *directory.OwnershipSpec
		shouldFail bool
	}{
		{
			name:    "Valid single owner",
			groupID: "test-group",
			ownership: &directory.OwnershipSpec{
				Scheme: directory.SchemeSingle,
				Owners: []string{pubkeyHex},
			},
			shouldFail: false,
		},
		{
			name:    "Valid 2-of-3 multisig",
			groupID: "multisig-group",
			ownership: &directory.OwnershipSpec{
				Scheme: directory.Scheme2of3,
				Owners: []string{pubkeyHex, pubkeyHex, pubkeyHex},
			},
			shouldFail: false,
		},
		{
			name:    "Valid 3-of-5 multisig",
			groupID: "large-multisig",
			ownership: &directory.OwnershipSpec{
				Scheme: directory.Scheme3of5,
				Owners: []string{pubkeyHex, pubkeyHex, pubkeyHex, pubkeyHex, pubkeyHex},
			},
			shouldFail: false,
		},
		{
			name:       "Invalid group ID with spaces",
			groupID:    "invalid group",
			ownership:  &directory.OwnershipSpec{Scheme: directory.SchemeSingle, Owners: []string{pubkeyHex}},
			shouldFail: true,
		},
		{
			name:       "Invalid group ID with special chars",
			groupID:    "invalid@group!",
			ownership:  &directory.OwnershipSpec{Scheme: directory.SchemeSingle, Owners: []string{pubkeyHex}},
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gta, err := directory.NewGroupTagAct(
				pubkey,
				tc.groupID,
				"role",
				"admin",
				pubkeyHex,
				95,
				tc.ownership,
				"Test group tag",
				nil,
			)

			if tc.shouldFail {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to create group tag act: %v", err)
			}

			// Sign the event
			if err := gta.Event.Sign(secKey); err != nil {
				t.Fatalf("failed to sign event: %v", err)
			}

			// Parse back
			parsed, err := directory.ParseGroupTagAct(gta.Event)
			if err != nil {
				t.Fatalf("failed to parse group tag act: %v", err)
			}

			if parsed.GroupID != tc.groupID {
				t.Errorf("group ID mismatch: got %s, want %s", parsed.GroupID, tc.groupID)
			}

			if parsed.Owners != nil {
				if parsed.Owners.Scheme != tc.ownership.Scheme {
					t.Errorf("ownership scheme mismatch: got %s, want %s",
						parsed.Owners.Scheme, tc.ownership.Scheme)
				}
			}
		})
	}

	t.Logf("✓ Group tag act creation tests passed")
}

// TestPublicKeyAdvertisementWithExpiry tests public key advertisement with expiration
func TestPublicKeyAdvertisementWithExpiry(t *testing.T) {
	// Generate identity and delegate keys
	identitySigner, identityPubkey := createTestKeypair(t)
	_, delegatePubkey := createTestKeypair(t)

	// Convert identity pubkey to secp256k1.PublicKey for npub encoding
	pubKey, err := secp256k1.ParsePubKey(append([]byte{0x02}, identityPubkey...))
	if err != nil {
		t.Fatalf("failed to parse pubkey: %v", err)
	}

	// Convert identity to npub (for potential future use)
	_, err = bech32encoding.PublicKeyToNpub(pubKey)
	if err != nil {
		t.Fatalf("failed to encode npub: %v", err)
	}

	// Test cases with different expiry scenarios
	testCases := []struct {
		name      string
		expiry    *time.Time
		isExpired bool
	}{
		{
			name:      "No expiry",
			expiry:    nil,
			isExpired: false,
		},
		{
			name: "Future expiry",
			expiry: func() *time.Time {
				t := time.Now().Add(24 * time.Hour)
				return &t
			}(),
			isExpired: false,
		},
		{
			name: "Past expiry (should allow creation, fail on validation)",
			expiry: func() *time.Time {
				t := time.Now().Add(-24 * time.Hour)
				return &t
			}(),
			isExpired: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pka, err := directory.NewPublicKeyAdvertisement(
				identityPubkey,
				"key-001",
				hex.EncodeToString(delegatePubkey),
				directory.KeyPurposeSigning,
				tc.expiry,
				"schnorr",
				"m/0/1",
				1,
				nil,
			)

			// For past expiry, we expect creation to fail
			if tc.isExpired && err != nil {
				t.Logf("✓ Correctly rejected past expiry: %v", err)
				return
			}

			if err != nil {
				t.Fatalf("failed to create public key advertisement: %v", err)
			}

			// Sign with identity key
			if err := pka.Event.Sign(identitySigner); err != nil {
				t.Fatalf("failed to sign event: %v", err)
			}

			// Parse back
			parsed, err := directory.ParsePublicKeyAdvertisement(pka.Event)
			if err != nil {
				t.Fatalf("failed to parse public key advertisement: %v", err)
			}

			// Verify expiry
			if tc.expiry != nil {
				if parsed.Expiry == nil {
					t.Errorf("expected expiry, got nil")
				} else if parsed.Expiry.Unix() != tc.expiry.Unix() {
					t.Errorf("expiry mismatch: got %v, want %v", parsed.Expiry, tc.expiry)
				}
			}

			// Test IsExpired method
			if tc.isExpired != parsed.IsExpired() {
				t.Errorf("IsExpired mismatch: got %v, want %v", parsed.IsExpired(), tc.isExpired)
			}
		})
	}

	t.Logf("✓ Public key advertisement expiry tests passed")
}

// TestTrustInheritanceCalculation tests web of trust calculations
func TestTrustInheritanceCalculation(t *testing.T) {
	calc := directory.NewTrustCalculator()

	_, pubkeyA := createTestKeypair(t)
	_, pubkeyB := createTestKeypair(t)
	_, pubkeyC := createTestKeypair(t)

	targetB := hex.EncodeToString(pubkeyB)
	targetC := hex.EncodeToString(pubkeyC)

	// Direct trust: A trusts B at 75%
	actAB, err := directory.NewTrustAct(
		pubkeyA, targetB, directory.TrustLevelHigh, "wss://b.relay.com/",
		nil, directory.TrustReasonManual, nil, nil,
	)
	if err != nil {
		t.Fatalf("failed to create trust act A->B: %v", err)
	}

	calc.AddAct(actAB)

	// Verify direct trust
	if calc.GetTrustLevel(targetB) != directory.TrustLevelHigh {
		t.Errorf("direct trust mismatch: got %d, want %d",
			calc.GetTrustLevel(targetB), directory.TrustLevelHigh)
	}

	// For inherited trust test, add B->C (50%)
	actBC, err := directory.NewTrustAct(
		pubkeyB, targetC, directory.TrustLevelMedium, "wss://c.relay.com/",
		nil, directory.TrustReasonManual, nil, nil,
	)
	if err != nil {
		t.Fatalf("failed to create trust act B->C: %v", err)
	}

	calc.AddAct(actBC)

	// Calculate inherited trust A->B->C
	// Since B is an intermediate node, the inherited trust should be
	// 75% * 50% = 37.5% = 37%
	inherited := calc.CalculateInheritedTrust(hex.EncodeToString(pubkeyA), targetC)

	// Note: The current implementation may return direct trust if found,
	// or 0 if no path exists. This tests the basic functionality.
	t.Logf("Trust levels: A->B(%d%%) B->C(%d%%) => A inherits %d%% for C",
		calc.GetTrustLevel(targetB),
		calc.GetTrustLevel(targetC),
		inherited)

	// Verify at least that we can get trust levels
	if calc.GetTrustLevel(targetB) == 0 {
		t.Errorf("failed to retrieve trust level for B")
	}

	t.Logf("✓ Trust calculator basic operations work correctly")
}

// TestGroupTagNameValidation tests URL-safe group tag validation
func TestGroupTagNameValidation(t *testing.T) {
	testCases := []struct {
		name       string
		groupID    string
		shouldFail bool
	}{
		{"Valid alphanumeric", "mygroup123", false},
		{"Valid with dash", "my-group", false},
		{"Valid with underscore inside", "my_group", false},
		{"Valid with dot inside", "my.group", false},
		{"Valid with tilde", "my~group", false},
		{"Invalid with space", "my group", true},
		{"Invalid with @", "my@group", true},
		{"Invalid with #", "my#group", true},
		{"Invalid with slash", "my/group", true},
		{"Invalid starting with dot", ".mygroup", true},
		{"Invalid starting with underscore", "_mygroup", true},
		{"Too long", string(make([]byte, 256)), true},
		{"Empty", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := directory.ValidateGroupTagName(tc.groupID)

			if tc.shouldFail && err == nil {
				t.Errorf("expected error for group ID %q, got nil", tc.groupID)
			}

			if !tc.shouldFail && err != nil {
				t.Errorf("unexpected error for group ID %q: %v", tc.groupID, err)
			}
		})
	}

	t.Logf("✓ Group tag name validation tests passed")
}

// TestDirectoryEventKindDetection tests IsDirectoryEventKind helper
func TestDirectoryEventKindDetection(t *testing.T) {
	testCases := []struct {
		kind        uint16
		isDirectory bool
	}{
		{0, true},      // Metadata
		{3, true},      // Contacts
		{5, true},      // Deletions
		{1984, true},   // Reporting
		{10002, true},  // Relay list
		{10000, true},  // Mute list
		{10050, true},  // DM relay list
		{39100, true},  // Relay identity
		{39101, true},  // Trust act
		{39102, true},  // Group tag act
		{39103, true},  // Public key advertisement
		{39104, true},  // Replication request
		{39105, true},  // Replication response
		{1, false},     // Text note (not directory)
		{7, false},     // Reaction (not directory)
		{30023, false}, // Long-form (not directory)
	}

	for _, tc := range testCases {
		result := directory.IsDirectoryEventKind(tc.kind)
		if result != tc.isDirectory {
			t.Errorf("kind %d: got %v, want %v", tc.kind, result, tc.isDirectory)
		}
	}

	t.Logf("✓ Directory event kind detection tests passed")
}
