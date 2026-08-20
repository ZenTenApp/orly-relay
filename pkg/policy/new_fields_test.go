package policy

import (
	"strconv"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/lol/chk"
)

// =============================================================================
// parseDuration Tests (ISO-8601 format)
// =============================================================================

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		// Basic ISO-8601 time units (require T separator)
		{name: "seconds only", input: "PT30S", expected: 30},
		{name: "minutes only", input: "PT5M", expected: 300},
		{name: "hours only", input: "PT2H", expected: 7200},

		// Basic ISO-8601 date units
		{name: "days only", input: "P1D", expected: 86400},
		{name: "7 days", input: "P7D", expected: 604800},
		{name: "30 days", input: "P30D", expected: 2592000},
		{name: "weeks", input: "P1W", expected: 604800},
		{name: "months", input: "P1M", expected: 2628000}, // ~30.44 days per library
		{name: "years", input: "P1Y", expected: 31536000},

		// Combinations
		{name: "hours and minutes", input: "PT1H30M", expected: 5400},
		{name: "days and hours", input: "P1DT12H", expected: 129600},
		{name: "days hours minutes", input: "P1DT2H30M", expected: 95400},
		{name: "full combo", input: "P1DT2H3M4S", expected: 93784},

		// Edge cases
		{name: "zero seconds", input: "PT0S", expected: 0},
		{name: "large days", input: "P365D", expected: 31536000},
		{name: "decimal values", input: "PT1.5H", expected: 5400},

		// Whitespace handling
		{name: "with leading space", input: " PT1H", expected: 3600},
		{name: "with trailing space", input: "PT1H ", expected: 3600},

		// Additional valid cases
		{name: "leading zeros", input: "P007D", expected: 604800},
		{name: "decimal days", input: "P0.5D", expected: 43200},
		{name: "fractional minutes", input: "PT0.5M", expected: 30},
		{name: "weeks with days", input: "P1W3D", expected: 864000},
		{name: "zero everything", input: "P0DT0H0M0S", expected: 0},

		// Errors (strict ISO-8601 via sosodev/duration library)
		{name: "empty string", input: "", expectError: true},
		{name: "whitespace only", input: "   ", expectError: true},
		{name: "missing P prefix", input: "1D", expectError: true},
		{name: "invalid unit", input: "P5X", expectError: true},
		{name: "H without T separator", input: "P1H", expectError: true},
		{name: "S without T separator", input: "P30S", expectError: true},
		{name: "D after T", input: "PT1D", expectError: true},
		{name: "Y after T", input: "PT1Y", expectError: true},
		{name: "W after T", input: "PT1W", expectError: true},
		{name: "negative number", input: "P-5D", expectError: true},
		{name: "unit without number", input: "PD", expectError: true},
		{name: "unit without number time", input: "PTH", expectError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got %d", tt.input, result)
				}
				return
			}
			if err != nil {
				t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("parseDuration(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// MaxExpiryDuration Tests
// =============================================================================

func TestMaxExpiryDuration(t *testing.T) {
	tests := []struct {
		name              string
		maxExpiryDuration string
		eventExpiry       int64 // offset from created_at
		hasExpiryTag      bool
		expectAllow       bool
	}{
		{
			name:              "valid expiry within limit",
			maxExpiryDuration: "PT1H",
			eventExpiry:       1800, // 30 minutes
			hasExpiryTag:      true,
			expectAllow:       true,
		},
		{
			name:              "expiry at exact limit rejected",
			maxExpiryDuration: "PT1H",
			eventExpiry:       3600, // exactly 1 hour - >= means this is rejected
			hasExpiryTag:      true,
			expectAllow:       false,
		},
		{
			name:              "expiry exceeds limit",
			maxExpiryDuration: "PT1H",
			eventExpiry:       7200, // 2 hours
			hasExpiryTag:      true,
			expectAllow:       false,
		},
		{
			name:              "missing expiry tag when required",
			maxExpiryDuration: "PT1H",
			hasExpiryTag:      false,
			expectAllow:       false,
		},
		{
			name:              "day-based duration",
			maxExpiryDuration: "P7D",
			eventExpiry:       86400, // 1 day
			hasExpiryTag:      true,
			expectAllow:       true,
		},
		{
			name:              "complex duration P1DT12H",
			maxExpiryDuration: "P1DT12H",
			eventExpiry:       86400, // 1 day (within 1.5 days)
			hasExpiryTag:      true,
			expectAllow:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, pubkey := generateTestKeypair(t)

			// Create policy with max_expiry_duration
			policyJSON := []byte(`{
				"default_policy": "allow",
				"rules": {
					"1": {
						"description": "Test kind 1 with expiry",
						"max_expiry_duration": "` + tt.maxExpiryDuration + `"
					}
				}
			}`)

			policy, err := New(policyJSON)
			if err != nil {
				t.Fatalf("Failed to create policy: %v", err)
			}

			// Create event
			ev := createTestEventForNewFields(t, signer, "test content", 1)

			// Add expiry tag if needed
			if tt.hasExpiryTag {
				expiryTs := ev.CreatedAt + tt.eventExpiry
				addTag(ev, "expiration", string(rune(expiryTs)))
				// Re-add as proper string
				ev.Tags = tag.NewS()
				addTagString(ev, "expiration", int64ToString(expiryTs))
				if err := ev.Sign(signer); chk.E(err) {
					t.Fatalf("Failed to re-sign event: %v", err)
				}
			}

			allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test MaxExpiryDuration takes precedence over MaxExpiry
func TestMaxExpiryDurationPrecedence(t *testing.T) {
	signer, pubkey := generateTestKeypair(t)

	// Policy where both max_expiry (seconds) and max_expiry_duration are set
	// max_expiry_duration should take precedence
	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"1": {
				"description": "Test precedence",
				"max_expiry": 60,
				"max_expiry_duration": "PT1H"
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create event with expiry at 30 minutes (would fail with max_expiry=60s, pass with PT1H)
	ev := createTestEventForNewFields(t, signer, "test", 1)
	expiryTs := ev.CreatedAt + 1800 // 30 minutes
	addTagString(ev, "expiration", int64ToString(expiryTs))
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}

	if !allowed {
		t.Error("MaxExpiryDuration should take precedence over MaxExpiry; expected allow")
	}
}

// Test that max_expiry_duration only applies to writes, not reads
func TestMaxExpiryDurationWriteOnly(t *testing.T) {
	signer, pubkey := generateTestKeypair(t)

	// Policy with strict max_expiry_duration
	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"4": {
				"description": "DM events with expiry",
				"max_expiry_duration": "PT10M",
				"privileged": true
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Create event WITHOUT an expiry tag - this would fail write validation
	// but should still be readable
	ev := createTestEventForNewFields(t, signer, "test DM", 4)
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Write should fail (no expiry tag when max_expiry_duration is set)
	allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy write error: %v", err)
	}
	if allowed {
		t.Error("Write should be denied for event without expiry tag when max_expiry_duration is set")
	}

	// Read should succeed (validation constraints don't apply to reads)
	allowed, _, err = policy.CheckPolicy("read", ev, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy read error: %v", err)
	}
	if !allowed {
		t.Error("Read should be allowed - max_expiry_duration is write-only validation")
	}

	// Also test with an event that has expiry exceeding the limit
	ev2 := createTestEventForNewFields(t, signer, "test DM 2", 4)
	expiryTs := ev2.CreatedAt + 7200 // 2 hours - exceeds 10 minute limit
	addTagString(ev2, "expiration", int64ToString(expiryTs))
	if err := ev2.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Write should fail (expiry exceeds limit)
	allowed, _, err = policy.CheckPolicy("write", ev2, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy write error: %v", err)
	}
	if allowed {
		t.Error("Write should be denied for event with expiry exceeding max_expiry_duration")
	}

	// Read should still succeed
	allowed, _, err = policy.CheckPolicy("read", ev2, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy read error: %v", err)
	}
	if !allowed {
		t.Error("Read should be allowed - max_expiry_duration is write-only validation")
	}
}

// =============================================================================
// ProtectedRequired Tests
// =============================================================================

func TestProtectedRequired(t *testing.T) {
	tests := []struct {
		name            string
		hasProtectedTag bool
		expectAllow     bool
	}{
		{
			name:            "has protected tag",
			hasProtectedTag: true,
			expectAllow:     true,
		},
		{
			name:            "missing protected tag",
			hasProtectedTag: false,
			expectAllow:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, pubkey := generateTestKeypair(t)

			policyJSON := []byte(`{
				"default_policy": "allow",
				"rules": {
					"1": {
						"description": "Protected events only",
						"protected_required": true
					}
				}
			}`)

			policy, err := New(policyJSON)
			if err != nil {
				t.Fatalf("Failed to create policy: %v", err)
			}

			ev := createTestEventForNewFields(t, signer, "test content", 1)

			if tt.hasProtectedTag {
				addTagString(ev, "-", "")
				if err := ev.Sign(signer); chk.E(err) {
					t.Fatalf("Failed to re-sign: %v", err)
				}
			}

			allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// =============================================================================
// IdentifierRegex Tests
// =============================================================================

func TestIdentifierRegex(t *testing.T) {
	tests := []struct {
		name        string
		regex       string
		dTagValue   string
		hasDTag     bool
		expectAllow bool
	}{
		{
			name:        "valid lowercase slug",
			regex:       "^[a-z0-9-]{1,64}$",
			dTagValue:   "my-article-slug",
			hasDTag:     true,
			expectAllow: true,
		},
		{
			name:        "invalid - contains uppercase",
			regex:       "^[a-z0-9-]{1,64}$",
			dTagValue:   "My-Article-Slug",
			hasDTag:     true,
			expectAllow: false,
		},
		{
			name:        "invalid - contains spaces",
			regex:       "^[a-z0-9-]{1,64}$",
			dTagValue:   "my article slug",
			hasDTag:     true,
			expectAllow: false,
		},
		{
			name:        "invalid - too long",
			regex:       "^[a-z0-9-]{1,10}$",
			dTagValue:   "this-is-too-long",
			hasDTag:     true,
			expectAllow: false,
		},
		{
			name:        "missing d tag when required",
			regex:       "^[a-z0-9-]{1,64}$",
			hasDTag:     false,
			expectAllow: false,
		},
		{
			name:        "UUID pattern",
			regex:       "^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$",
			dTagValue:   "550e8400-e29b-41d4-a716-446655440000",
			hasDTag:     true,
			expectAllow: true,
		},
		{
			name:        "alphanumeric only",
			regex:       "^[a-zA-Z0-9]+$",
			dTagValue:   "MyArticle123",
			hasDTag:     true,
			expectAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signer, pubkey := generateTestKeypair(t)

			policyJSON := []byte(`{
				"default_policy": "allow",
				"rules": {
					"30023": {
						"description": "Long-form with identifier regex",
						"identifier_regex": "` + tt.regex + `"
					}
				}
			}`)

			policy, err := New(policyJSON)
			if err != nil {
				t.Fatalf("Failed to create policy: %v", err)
			}

			ev := createTestEventForNewFields(t, signer, "test content", 30023)

			if tt.hasDTag {
				addTagString(ev, "d", tt.dTagValue)
				if err := ev.Sign(signer); chk.E(err) {
					t.Fatalf("Failed to re-sign: %v", err)
				}
			}

			allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test that IdentifierRegex validates multiple d tags
func TestIdentifierRegexMultipleDTags(t *testing.T) {
	signer, pubkey := generateTestKeypair(t)

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"30023": {
				"description": "Test multiple d tags",
				"identifier_regex": "^[a-z0-9-]+$"
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Test with one valid and one invalid d tag
	ev := createTestEventForNewFields(t, signer, "test", 30023)
	addTagString(ev, "d", "valid-slug")
	addTagString(ev, "d", "INVALID-SLUG") // uppercase should fail
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}

	if allowed {
		t.Error("Should deny when any d tag fails regex validation")
	}
}

// =============================================================================
// FollowsWhitelistAdmins Tests
// =============================================================================

func TestFollowsWhitelistAdmins(t *testing.T) {
	// Generate admin and user keypairs
	adminSigner, adminPubkey := generateTestKeypair(t)
	userSigner, userPubkey := generateTestKeypair(t)
	nonFollowSigner, nonFollowPubkey := generateTestKeypair(t)

	adminHex := hex.Enc(adminPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"description": "Only admin follows can write",
				"follows_whitelist_admins": ["` + adminHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Simulate loading admin's follows (user is followed by admin)
	policy.UpdateRuleFollowsWhitelist(1, [][]byte{userPubkey})

	tests := []struct {
		name        string
		signer      *p8k.Signer
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "followed user can write",
			signer:      userSigner,
			pubkey:      userPubkey,
			expectAllow: true,
		},
		{
			name:        "non-followed user denied",
			signer:      nonFollowSigner,
			pubkey:      nonFollowPubkey,
			expectAllow: false,
		},
		{
			name:        "admin can write (is in own follows conceptually)",
			signer:      adminSigner,
			pubkey:      adminPubkey,
			expectAllow: false, // Admin not in follows list in this test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEventForNewFields(t, tt.signer, "test content", 1)

			allowed, _, err := policy.CheckPolicy("write", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

func TestGetAllFollowsWhitelistAdmins(t *testing.T) {
	admin1 := "1111111111111111111111111111111111111111111111111111111111111111"
	admin2 := "2222222222222222222222222222222222222222222222222222222222222222"
	admin3 := "3333333333333333333333333333333333333333333333333333333333333333"

	policyJSON := []byte(`{
		"default_policy": "deny",
		"global": {
			"follows_whitelist_admins": ["` + admin1 + `"]
		},
		"rules": {
			"1": {
				"follows_whitelist_admins": ["` + admin2 + `"]
			},
			"30023": {
				"follows_whitelist_admins": ["` + admin2 + `", "` + admin3 + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	admins := policy.GetAllFollowsWhitelistAdmins()

	// Should have 3 unique admins (admin2 is deduplicated)
	if len(admins) != 3 {
		t.Errorf("Expected 3 unique admins, got %d", len(admins))
	}

	// Check all admins are present
	adminMap := make(map[string]bool)
	for _, a := range admins {
		adminMap[a] = true
	}

	for _, expected := range []string{admin1, admin2, admin3} {
		if !adminMap[expected] {
			t.Errorf("Missing admin %s", expected)
		}
	}
}

// =============================================================================
// Combinatorial Tests - New Fields with Existing Fields
// =============================================================================

// Test MaxExpiryDuration combined with SizeLimit
func TestMaxExpiryDurationWithSizeLimit(t *testing.T) {
	signer, pubkey := generateTestKeypair(t)

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"1": {
				"max_expiry_duration": "PT1H",
				"size_limit": 1000
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	tests := []struct {
		name        string
		contentSize int
		hasExpiry   bool
		expiryOK    bool
		expectAllow bool
	}{
		{
			name:        "both constraints satisfied",
			contentSize: 100,
			hasExpiry:   true,
			expiryOK:    true,
			expectAllow: true,
		},
		{
			name:        "size exceeded",
			contentSize: 2000,
			hasExpiry:   true,
			expiryOK:    true,
			expectAllow: false,
		},
		{
			name:        "expiry exceeded",
			contentSize: 100,
			hasExpiry:   true,
			expiryOK:    false,
			expectAllow: false,
		},
		{
			name:        "missing expiry",
			contentSize: 100,
			hasExpiry:   false,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := make([]byte, tt.contentSize)
			for i := range content {
				content[i] = 'a'
			}

			ev := event.New()
			ev.CreatedAt = time.Now().Unix()
			ev.Kind = 1
			ev.Content = content
			ev.Tags = tag.NewS()

			if tt.hasExpiry {
				var expiryOffset int64 = 1800 // 30 min (OK)
				if !tt.expiryOK {
					expiryOffset = 7200 // 2h (exceeds 1h limit)
				}
				addTagString(ev, "expiration", int64ToString(ev.CreatedAt+expiryOffset))
			}

			if err := ev.Sign(signer); chk.E(err) {
				t.Fatalf("Failed to sign: %v", err)
			}

			allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test ProtectedRequired combined with Privileged
func TestProtectedRequiredWithPrivileged(t *testing.T) {
	authorSigner, authorPubkey := generateTestKeypair(t)
	_, recipientPubkey := generateTestKeypair(t)
	_, outsiderPubkey := generateTestKeypair(t)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"4": {
				"description": "Encrypted DMs - protected and privileged",
				"protected_required": true,
				"privileged": true
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	tests := []struct {
		name         string
		hasProtected bool
		readerPubkey []byte
		isParty      bool // is reader author or in p-tag
		accessType   string
		expectAllow  bool
	}{
		{
			name:         "author can read protected event",
			hasProtected: true,
			readerPubkey: authorPubkey,
			isParty:      true,
			accessType:   "read",
			expectAllow:  true,
		},
		{
			name:         "recipient in p-tag can read",
			hasProtected: true,
			readerPubkey: recipientPubkey,
			isParty:      true,
			accessType:   "read",
			expectAllow:  true,
		},
		{
			name:         "outsider cannot read privileged event",
			hasProtected: true,
			readerPubkey: outsiderPubkey,
			isParty:      false,
			accessType:   "read",
			expectAllow:  false,
		},
		{
			name:         "missing protected tag - write denied",
			hasProtected: false,
			readerPubkey: authorPubkey,
			isParty:      true,
			accessType:   "write",
			expectAllow:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEventForNewFields(t, authorSigner, "encrypted content", 4)

			// Add recipient to p-tag
			addPTag(ev, recipientPubkey)

			if tt.hasProtected {
				addTagString(ev, "-", "")
			}

			if err := ev.Sign(authorSigner); chk.E(err) {
				t.Fatalf("Failed to sign: %v", err)
			}

			allowed, _, err := policy.CheckPolicy(tt.accessType, ev, tt.readerPubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test IdentifierRegex combined with TagValidation
func TestIdentifierRegexWithTagValidation(t *testing.T) {
	signer, pubkey := generateTestKeypair(t)

	// Both identifier_regex (for d tag) and tag_validation (for t tag)
	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"30023": {
				"identifier_regex": "^[a-z0-9-]+$",
				"tag_validation": {
					"t": "^[a-z]+$"
				}
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	tests := []struct {
		name        string
		dTag        string
		tTag        string
		hasDTag     bool
		hasTTag     bool
		expectAllow bool
	}{
		{
			name:        "both tags valid",
			dTag:        "my-article",
			tTag:        "nostr",
			hasDTag:     true,
			hasTTag:     true,
			expectAllow: true,
		},
		{
			name:        "d tag invalid",
			dTag:        "MY-ARTICLE",
			tTag:        "nostr",
			hasDTag:     true,
			hasTTag:     true,
			expectAllow: false,
		},
		{
			name:        "t tag invalid",
			dTag:        "my-article",
			tTag:        "NOSTR123",
			hasDTag:     true,
			hasTTag:     true,
			expectAllow: false,
		},
		{
			name:        "missing d tag",
			tTag:        "nostr",
			hasDTag:     false,
			hasTTag:     true,
			expectAllow: false,
		},
		{
			name:        "missing t tag - allowed (tag_validation only validates present tags)",
			dTag:        "my-article",
			hasDTag:     true,
			hasTTag:     false,
			expectAllow: true, // tag_validation doesn't require tags to exist, only validates if present
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEventForNewFields(t, signer, "article content", 30023)

			if tt.hasDTag {
				addTagString(ev, "d", tt.dTag)
			}
			if tt.hasTTag {
				addTagString(ev, "t", tt.tTag)
			}

			if err := ev.Sign(signer); chk.E(err) {
				t.Fatalf("Failed to sign: %v", err)
			}

			allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test FollowsWhitelistAdmins combined with WriteAllow
func TestFollowsWhitelistAdminsWithWriteAllow(t *testing.T) {
	_, adminPubkey := generateTestKeypair(t)
	followedSigner, followedPubkey := generateTestKeypair(t)
	explicitSigner, explicitPubkey := generateTestKeypair(t)
	_, deniedPubkey := generateTestKeypair(t)

	adminHex := hex.Enc(adminPubkey)
	explicitHex := hex.Enc(explicitPubkey)

	// Both follows whitelist and explicit write_allow
	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"1": {
				"follows_whitelist_admins": ["` + adminHex + `"],
				"write_allow": ["` + explicitHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Add followed user to whitelist
	policy.UpdateRuleFollowsWhitelist(1, [][]byte{followedPubkey})

	tests := []struct {
		name        string
		signer      *p8k.Signer
		pubkey      []byte
		expectAllow bool
	}{
		{
			name:        "followed user allowed",
			signer:      followedSigner,
			pubkey:      followedPubkey,
			expectAllow: true,
		},
		{
			name:        "explicit write_allow user allowed",
			signer:      explicitSigner,
			pubkey:      explicitPubkey,
			expectAllow: true,
		},
		{
			name:        "user not in either list denied",
			signer:      p8k.MustNew(),
			pubkey:      deniedPubkey,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate if needed
			if tt.signer.Pub() == nil {
				_ = tt.signer.Generate()
			}

			ev := createTestEventForNewFields(t, tt.signer, "test", 1)

			allowed, _, err := policy.CheckPolicy("write", ev, tt.pubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test all new fields combined
func TestAllNewFieldsCombined(t *testing.T) {
	_, adminPubkey := generateTestKeypair(t)
	userSigner, userPubkey := generateTestKeypair(t)

	adminHex := hex.Enc(adminPubkey)

	policyJSON := []byte(`{
		"default_policy": "deny",
		"rules": {
			"30023": {
				"description": "All new constraints",
				"max_expiry_duration": "P7D",
				"protected_required": true,
				"identifier_regex": "^[a-z0-9-]{1,32}$",
				"follows_whitelist_admins": ["` + adminHex + `"]
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Add user to follows whitelist
	policy.UpdateRuleFollowsWhitelist(30023, [][]byte{userPubkey})

	tests := []struct {
		name        string
		dTag        string
		hasExpiry   bool
		expiryOK    bool
		hasProtect  bool
		expectAllow bool
	}{
		{
			name:        "all constraints satisfied",
			dTag:        "my-article",
			hasExpiry:   true,
			expiryOK:    true,
			hasProtect:  true,
			expectAllow: true,
		},
		{
			name:        "missing protected tag",
			dTag:        "my-article",
			hasExpiry:   true,
			expiryOK:    true,
			hasProtect:  false,
			expectAllow: false,
		},
		{
			name:        "invalid d tag",
			dTag:        "INVALID",
			hasExpiry:   true,
			expiryOK:    true,
			hasProtect:  true,
			expectAllow: false,
		},
		{
			name:        "expiry too long",
			dTag:        "my-article",
			hasExpiry:   true,
			expiryOK:    false,
			hasProtect:  true,
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := createTestEventForNewFields(t, userSigner, "article content", 30023)

			addTagString(ev, "d", tt.dTag)

			if tt.hasExpiry {
				var offset int64 = 86400 // 1 day (OK)
				if !tt.expiryOK {
					offset = 864000 // 10 days (exceeds 7d)
				}
				addTagString(ev, "expiration", int64ToString(ev.CreatedAt+offset))
			}

			if tt.hasProtect {
				addTagString(ev, "-", "")
			}

			if err := ev.Sign(userSigner); chk.E(err) {
				t.Fatalf("Failed to sign: %v", err)
			}

			allowed, _, err := policy.CheckPolicy("write", ev, userPubkey, "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// Test new fields in global rule
// Global rule is ONLY used as fallback when NO kind-specific rule exists.
// If a kind-specific rule exists (even if empty), it takes precedence and global is ignored.
func TestNewFieldsInGlobalRule(t *testing.T) {
	signer, pubkey := generateTestKeypair(t)

	// Policy with global constraints and a kind-specific rule for kind 1
	policyJSON := []byte(`{
		"default_policy": "allow",
		"global": {
			"max_expiry_duration": "P1D",
			"protected_required": true
		},
		"rules": {
			"1": {
				"description": "Kind 1 events - has specific rule, so global is ignored"
			}
		}
	}`)

	policy, err := New(policyJSON)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Kind 1 has a specific rule, so global protected_required is IGNORED
	// Event should be ALLOWED even without protected tag
	ev := createTestEventForNewFields(t, signer, "test", 1)
	addTagString(ev, "expiration", int64ToString(ev.CreatedAt+3600))
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	allowed, _, err := policy.CheckPolicy("write", ev, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}

	if !allowed {
		t.Error("Kind 1 has specific rule - global protected_required should be ignored, event should be allowed")
	}

	// Now test kind 999 which has NO specific rule - global should apply
	ev2 := createTestEventForNewFields(t, signer, "test", 999)
	addTagString(ev2, "expiration", int64ToString(ev2.CreatedAt+3600))
	if err := ev2.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	allowed, _, err = policy.CheckPolicy("write", ev2, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}

	if allowed {
		t.Error("Kind 999 has NO specific rule - global protected_required should apply, event should be denied")
	}

	// Add protected tag to kind 999 event - should now be allowed
	addTagString(ev2, "-", "")
	if err := ev2.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}

	allowed, _, err = policy.CheckPolicy("write", ev2, pubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}

	if !allowed {
		t.Error("Kind 999 with protected tag and valid expiry should be allowed by global rule")
	}
}

// =============================================================================
// New() Validation Tests - Ensures invalid configs fail at load time
// =============================================================================

// TestNewRejectsInvalidMaxExpiryDuration verifies that New() fails fast when
// given an invalid max_expiry_duration format like "T10M" instead of "PT10M".
// This prevents silent failures where constraints are ignored.
func TestNewRejectsInvalidMaxExpiryDuration(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid PT10M format accepted",
			json: `{
				"rules": {
					"4": {"max_expiry_duration": "PT10M"}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid T10M format (missing P prefix) rejected",
			json: `{
				"rules": {
					"4": {"max_expiry_duration": "T10M"}
				}
			}`,
			expectError: true,
			errorMatch:  "max_expiry_duration",
		},
		{
			name: "invalid 10M format (missing PT prefix) rejected",
			json: `{
				"rules": {
					"4": {"max_expiry_duration": "10M"}
				}
			}`,
			expectError: true,
			errorMatch:  "max_expiry_duration",
		},
		{
			name: "valid P7D format accepted",
			json: `{
				"rules": {
					"1": {"max_expiry_duration": "P7D"}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid 7D format (missing P prefix) rejected",
			json: `{
				"rules": {
					"1": {"max_expiry_duration": "7D"}
				}
			}`,
			expectError: true,
			errorMatch:  "max_expiry_duration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := New([]byte(tt.json))

			if tt.expectError {
				if err == nil {
					t.Errorf("New() should have rejected invalid config, but returned policy: %+v", policy)
					return
				}
				if tt.errorMatch != "" && !contains(err.Error(), tt.errorMatch) {
					t.Errorf("Error %q should contain %q", err.Error(), tt.errorMatch)
				}
			} else {
				if err != nil {
					t.Errorf("New() unexpected error for valid config: %v", err)
				}
				if policy == nil {
					t.Error("New() returned nil policy for valid config")
				}
			}
		})
	}
}

// =============================================================================
// ValidateJSON Tests for New Fields
// =============================================================================

func TestValidateJSONNewFields(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
		errorMatch  string
	}{
		{
			name: "valid max_expiry_duration",
			json: `{
				"rules": {
					"1": {"max_expiry_duration": "P7DT12H30M"}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid max_expiry_duration - no P prefix",
			json: `{
				"rules": {
					"1": {"max_expiry_duration": "7D"}
				}
			}`,
			expectError: true,
			errorMatch:  "max_expiry_duration",
		},
		{
			name: "invalid max_expiry_duration - invalid format",
			json: `{
				"rules": {
					"1": {"max_expiry_duration": "invalid"}
				}
			}`,
			expectError: true,
			errorMatch:  "max_expiry_duration",
		},
		{
			name: "valid identifier_regex",
			json: `{
				"rules": {
					"30023": {"identifier_regex": "^[a-z0-9-]+$"}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid identifier_regex",
			json: `{
				"rules": {
					"30023": {"identifier_regex": "[invalid("}
				}
			}`,
			expectError: true,
			errorMatch:  "identifier_regex",
		},
		{
			name: "valid follows_whitelist_admins",
			json: `{
				"rules": {
					"1": {"follows_whitelist_admins": ["1111111111111111111111111111111111111111111111111111111111111111"]}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid follows_whitelist_admins - wrong length",
			json: `{
				"rules": {
					"1": {"follows_whitelist_admins": ["tooshort"]}
				}
			}`,
			expectError: true,
			errorMatch:  "follows_whitelist_admins",
		},
		{
			name: "invalid follows_whitelist_admins - not hex",
			json: `{
				"rules": {
					"1": {"follows_whitelist_admins": ["gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg"]}
				}
			}`,
			expectError: true,
			errorMatch:  "follows_whitelist_admins",
		},
		{
			name: "valid global rule new fields",
			json: `{
				"global": {
					"max_expiry_duration": "P1D",
					"identifier_regex": "^[a-z]+$",
					"protected_required": true
				}
			}`,
			expectError: false,
		},
		// Tests for read_allow_permissive and write_allow_permissive
		{
			name: "valid read_allow_permissive alone with whitelist",
			json: `{
				"kind": {"whitelist": [1, 3, 5]},
				"global": {"read_allow_permissive": true}
			}`,
			expectError: false,
		},
		{
			name: "valid write_allow_permissive alone with whitelist",
			json: `{
				"kind": {"whitelist": [1, 3, 5]},
				"global": {"write_allow_permissive": true}
			}`,
			expectError: false,
		},
		{
			name: "invalid both permissive flags with whitelist",
			json: `{
				"kind": {"whitelist": [1, 3, 5]},
				"global": {
					"read_allow_permissive": true,
					"write_allow_permissive": true
				}
			}`,
			expectError: true,
			errorMatch:  "read_allow_permissive and write_allow_permissive cannot be enabled together",
		},
		{
			name: "invalid both permissive flags with blacklist",
			json: `{
				"kind": {"blacklist": [2, 4, 6]},
				"global": {
					"read_allow_permissive": true,
					"write_allow_permissive": true
				}
			}`,
			expectError: true,
			errorMatch:  "read_allow_permissive and write_allow_permissive cannot be enabled together",
		},
		{
			name: "valid both permissive flags without any kind restriction",
			json: `{
				"global": {
					"read_allow_permissive": true,
					"write_allow_permissive": true
				}
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{}
			err := policy.ValidateJSON([]byte(tt.json))

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error, got nil")
					return
				}
				if tt.errorMatch != "" && !contains(err.Error(), tt.errorMatch) {
					t.Errorf("Error %q should contain %q", err.Error(), tt.errorMatch)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error: %v", err)
				}
			}
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestEventForNewFields(t *testing.T, signer *p8k.Signer, content string, kind uint16) *event.E {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign test event: %v", err)
	}

	return ev
}

func addTagString(ev *event.E, key, value string) {
	tagItem := tag.NewFromAny(key, value)
	ev.Tags.Append(tagItem)
}

func int64ToString(i int64) string {
	return strconv.FormatInt(i, 10)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
