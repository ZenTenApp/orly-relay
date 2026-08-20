package policy

import (
	"encoding/json"
	"strings"
	"testing"

	"git.smesh.lol/orly/pkg/lol/chk"
)

// TestAllowedTagsBasic verifies the exclusive tag key whitelist rejects events
// that carry any tag key outside the allowed set (e.g. kind 37801 allows only
// "d" and "p" tags).
func TestAllowedTagsBasic(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-allowed-tags-basic")
	defer cleanup()

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"37801": {
				"description": "Only d and p tags permitted",
				"allowed_tags": ["d", "p"]
			}
		}
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	tests := []struct {
		name        string
		kind        uint16
		tags        map[string]string
		expectAllow bool
	}{
		{
			name:        "only d tag",
			kind:        37801,
			tags:        map[string]string{"d": "identifier"},
			expectAllow: true,
		},
		{
			name:        "d and p tags",
			kind:        37801,
			tags:        map[string]string{"d": "identifier", "p": "pubkeyref"},
			expectAllow: true,
		},
		{
			name:        "no tags at all",
			kind:        37801,
			tags:        map[string]string{},
			expectAllow: true,
		},
		{
			name:        "extra e tag rejected",
			kind:        37801,
			tags:        map[string]string{"d": "identifier", "e": "eventref"},
			expectAllow: false,
		},
		{
			name:        "single disallowed tag rejected",
			kind:        37801,
			tags:        map[string]string{"t": "topic"},
			expectAllow: false,
		},
		{
			name:        "different kind has no restriction",
			kind:        1,
			tags:        map[string]string{"e": "x", "p": "y", "t": "z"},
			expectAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, signer := createSignedTestEvent(t, tt.kind, "test content")
			for key, value := range tt.tags {
				addTagToEvent(ev, key, value)
			}
			if err := ev.Sign(signer); chk.E(err) {
				t.Fatalf("Failed to re-sign event: %v", err)
			}

			allowed, denyReason, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy returned error: %v", err)
			}
			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
			if !tt.expectAllow && !strings.HasPrefix(denyReason, "invalid: tag ") {
				t.Errorf("expected invalid: tag reason, got %q", denyReason)
			}
		})
	}
}

// TestAllowedTagsPerKind verifies different kinds can have independent allowed
// tag sets.
func TestAllowedTagsPerKind(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-allowed-tags-perkind")
	defer cleanup()

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"37801": { "allowed_tags": ["d", "p"] },
			"30023": { "allowed_tags": ["d", "t", "title"] }
		}
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// t tag allowed for 30023 but not 37801.
	ev, signer := createSignedTestEvent(t, 30023, "content")
	addTagToEvent(ev, "d", "slug")
	addTagToEvent(ev, "t", "topic")
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}
	allowed, _, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}
	if !allowed {
		t.Errorf("kind 30023 with d+t tags should be allowed")
	}

	ev2, signer2 := createSignedTestEvent(t, 37801, "content")
	addTagToEvent(ev2, "d", "slug")
	addTagToEvent(ev2, "t", "topic")
	if err := ev2.Sign(signer2); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}
	allowed2, _, err := policy.CheckPolicy("write", ev2, signer2.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}
	if allowed2 {
		t.Errorf("kind 37801 with t tag should be rejected")
	}
}

// TestAllowedTagsGlobalRule verifies allowed_tags applies to all kinds when set
// on the global rule.
func TestAllowedTagsGlobalRule(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-allowed-tags-global")
	defer cleanup()

	policyJSON := []byte(`{
		"default_policy": "allow",
		"global": { "allowed_tags": ["d", "p"] }
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Any kind with a disallowed tag should be rejected under the global rule.
	ev, signer := createSignedTestEvent(t, 1, "content")
	addTagToEvent(ev, "e", "eventref")
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}
	allowed, _, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}
	if allowed {
		t.Errorf("event with disallowed 'e' tag should be rejected by global allowed_tags")
	}

	// Allowed tags pass.
	ev2, signer2 := createSignedTestEvent(t, 1, "content")
	addTagToEvent(ev2, "d", "x")
	addTagToEvent(ev2, "p", "y")
	if err := ev2.Sign(signer2); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}
	allowed2, _, err := policy.CheckPolicy("write", ev2, signer2.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}
	if !allowed2 {
		t.Errorf("event with only allowed tags should pass global allowed_tags")
	}
}

// TestAllowedTagsAdminCannotRestrict verifies a policy admin cannot introduce or
// narrow an allowed_tags whitelist, since that is a restriction only owners may add.
func TestAllowedTagsAdminCannotRestrict(t *testing.T) {
	ownerPubkey := strings.Repeat("a", 64)
	adminPubkey := strings.Repeat("b", 64)

	baseJSON := `{
		"default_policy": "allow",
		"owners": ["` + ownerPubkey + `"],
		"policy_admins": ["` + adminPubkey + `"],
		"rules": {
			"37801": { "allowed_tags": ["d", "p"] },
			"1": { "description": "text" }
		}
	}`
	basePolicy := &P{}
	if err := json.Unmarshal([]byte(baseJSON), basePolicy); err != nil {
		t.Fatalf("failed to create base policy: %v", err)
	}
	adminBin := make([]byte, 32)
	for i := range adminBin {
		adminBin[i] = 0xbb
	}

	tests := []struct {
		name        string
		newPolicy   string
		expectError bool
	}{
		{
			name: "admin cannot narrow existing allowed_tags",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"rules": {
					"37801": { "allowed_tags": ["d"] },
					"1": { "description": "text" }
				}
			}`,
			expectError: true,
		},
		{
			name: "admin cannot introduce allowed_tags where none existed",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"rules": {
					"37801": { "allowed_tags": ["d", "p"] },
					"1": { "allowed_tags": ["e"] }
				}
			}`,
			expectError: true,
		},
		{
			name: "admin may keep allowed_tags unchanged",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"rules": {
					"37801": { "allowed_tags": ["d", "p"] },
					"1": { "description": "text" }
				}
			}`,
			expectError: false,
		},
		{
			name: "admin may widen allowed_tags",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"rules": {
					"37801": { "allowed_tags": ["d", "p", "t"] },
					"1": { "description": "text" }
				}
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := basePolicy.ValidatePolicyAdminUpdate([]byte(tt.newPolicy), adminBin)
			if tt.expectError && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestAllowedTagsReadUnaffected verifies allowed_tags is write-only and does not
// filter reads.
func TestAllowedTagsReadUnaffected(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-allowed-tags-read")
	defer cleanup()

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": { "37801": { "allowed_tags": ["d"] } }
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Event with a disallowed tag should still be readable (validation is write-only).
	ev, signer := createSignedTestEvent(t, 37801, "content")
	addTagToEvent(ev, "d", "id")
	addTagToEvent(ev, "e", "eventref")
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign: %v", err)
	}
	allowed, _, err := policy.CheckPolicy("read", ev, signer.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy error: %v", err)
	}
	if !allowed {
		t.Errorf("read should not be filtered by allowed_tags")
	}
}
