package policy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrg/xdg"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/lol/chk"
)

// setupTagValidationTestPolicy creates a policy manager with a temporary config file for tag validation tests.
func setupTagValidationTestPolicy(t *testing.T, appName string) (*P, func()) {
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

	policy := NewWithManager(ctx, appName, true, "")
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

// createSignedTestEvent creates a signed event for testing
func createSignedTestEvent(t *testing.T, kind uint16, content string) (*event.E, *p8k.Signer) {
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	return ev, signer
}

// addTagToEvent adds a tag to an event
func addTagToEvent(ev *event.E, key, value string) {
	tagItem := tag.NewFromAny(key, value)
	ev.Tags.Append(tagItem)
}

// TestTagValidationBasic tests basic tag validation with regex patterns
func TestTagValidationBasic(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-tag-basic")
	defer cleanup()

	// Policy with tag validation for kind 30023 (long-form content)
	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"30023": {
				"description": "Long-form content with tag validation",
				"tag_validation": {
					"d": "^[a-z0-9-]{1,64}$",
					"t": "^[a-z0-9-]{1,32}$"
				}
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
			name: "valid d tag",
			kind: 30023,
			tags: map[string]string{
				"d": "my-article-slug",
			},
			expectAllow: true,
		},
		{
			name: "valid d and t tags",
			kind: 30023,
			tags: map[string]string{
				"d": "my-article-slug",
				"t": "nostr",
			},
			expectAllow: true,
		},
		{
			name: "invalid d tag - contains uppercase",
			kind: 30023,
			tags: map[string]string{
				"d": "My-Article-Slug",
			},
			expectAllow: false,
		},
		{
			name: "invalid d tag - contains spaces",
			kind: 30023,
			tags: map[string]string{
				"d": "my article slug",
			},
			expectAllow: false,
		},
		{
			name: "invalid d tag - too long",
			kind: 30023,
			tags: map[string]string{
				"d": "this-is-a-very-long-slug-that-exceeds-the-sixty-four-character-limit-set-in-policy",
			},
			expectAllow: false,
		},
		{
			name: "invalid t tag - contains special chars",
			kind: 30023,
			tags: map[string]string{
				"d": "valid-slug",
				"t": "nostr@tag",
			},
			expectAllow: false,
		},
		{
			name: "kind without tag validation - any tags allowed",
			kind: 1, // Kind 1 has no tag validation rules
			tags: map[string]string{
				"d": "ANYTHING_GOES!!!",
				"t": "spaces and Special Chars",
			},
			expectAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, signer := createSignedTestEvent(t, tt.kind, "test content")

			// Add tags to event
			for key, value := range tt.tags {
				addTagToEvent(ev, key, value)
			}

			// Re-sign after adding tags
			if err := ev.Sign(signer); chk.E(err) {
				t.Fatalf("Failed to re-sign event: %v", err)
			}

			allowed, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy returned error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// TestTagValidationMultipleSameTag tests validation when multiple tags have the same name
func TestTagValidationMultipleSameTag(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-tag-multi")
	defer cleanup()

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"30023": {
				"tag_validation": {
					"t": "^[a-z0-9-]+$"
				}
			}
		}
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	tests := []struct {
		name        string
		tags        []string // Multiple t tags
		expectAllow bool
	}{
		{
			name:        "all tags valid",
			tags:        []string{"nostr", "bitcoin", "lightning"},
			expectAllow: true,
		},
		{
			name:        "one invalid tag among valid ones",
			tags:        []string{"nostr", "INVALID", "lightning"},
			expectAllow: false,
		},
		{
			name:        "first tag invalid",
			tags:        []string{"INVALID", "nostr", "bitcoin"},
			expectAllow: false,
		},
		{
			name:        "last tag invalid",
			tags:        []string{"nostr", "bitcoin", "INVALID"},
			expectAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, signer := createSignedTestEvent(t, 30023, "test content")

			// Add multiple t tags
			for _, value := range tt.tags {
				addTagToEvent(ev, "t", value)
			}

			// Re-sign
			if err := ev.Sign(signer); chk.E(err) {
				t.Fatalf("Failed to re-sign event: %v", err)
			}

			allowed, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
			if err != nil {
				t.Fatalf("CheckPolicy returned error: %v", err)
			}

			if allowed != tt.expectAllow {
				t.Errorf("CheckPolicy() = %v, expected %v", allowed, tt.expectAllow)
			}
		})
	}
}

// TestTagValidationInvalidRegex tests that invalid regex patterns are caught during validation
func TestTagValidationInvalidRegex(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-tag-invalid-regex")
	defer cleanup()

	invalidRegexPolicies := []struct {
		name   string
		policy []byte
	}{
		{
			name: "unclosed bracket",
			policy: []byte(`{
				"rules": {
					"30023": {
						"tag_validation": {
							"d": "[invalid"
						}
					}
				}
			}`),
		},
		{
			name: "unclosed parenthesis",
			policy: []byte(`{
				"rules": {
					"30023": {
						"tag_validation": {
							"d": "(unclosed"
						}
					}
				}
			}`),
		},
		{
			name: "invalid escape sequence",
			policy: []byte(`{
				"rules": {
					"30023": {
						"tag_validation": {
							"d": "\\k"
						}
					}
				}
			}`),
		},
	}

	for _, tt := range invalidRegexPolicies {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.ValidateJSON(tt.policy)
			if err == nil {
				t.Error("Expected validation error for invalid regex, got none")
			}
		})
	}
}

// TestTagValidationEmptyTag tests behavior when a tag has no value
func TestTagValidationEmptyTag(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-tag-empty")
	defer cleanup()

	policyJSON := []byte(`{
		"default_policy": "allow",
		"rules": {
			"30023": {
				"tag_validation": {
					"d": "^[a-z0-9-]+$"
				}
			}
		}
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Create event with empty d tag value
	ev, signer := createSignedTestEvent(t, 30023, "test content")
	addTagToEvent(ev, "d", "")
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	allowed, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy returned error: %v", err)
	}

	// Empty string doesn't match ^[a-z0-9-]+$ (+ requires at least one char)
	if allowed {
		t.Error("Expected empty tag value to be rejected")
	}
}

// TestTagValidationWithWriteAllowFollows tests interaction between tag validation and follow whitelist
func TestTagValidationWithWriteAllowFollows(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-tag-follows")
	defer cleanup()

	// Create a test signer who will be a "follow"
	signer := p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Set up policy with tag validation AND write_allow_follows
	adminHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	policyJSON := []byte(`{
		"default_policy": "deny",
		"policy_admins": ["` + adminHex + `"],
		"policy_follow_whitelist_enabled": true,
		"rules": {
			"30023": {
				"write_allow_follows": true,
				"tag_validation": {
					"d": "^[a-z0-9-]+$"
				}
			}
		}
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Add the signer as a follow
	policy.UpdatePolicyFollows([][]byte{signer.Pub()})

	// Test: Follow with valid tag should be allowed
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = 30023
	ev.Content = []byte("test content")
	ev.Tags = tag.NewS()
	addTagToEvent(ev, "d", "valid-slug")
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	allowed, err := policy.CheckPolicy("write", ev, signer.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy returned error: %v", err)
	}

	if !allowed {
		t.Error("Expected follow with valid tag to be allowed")
	}

	// Test: Follow with invalid tag should still be rejected (tag validation applies)
	ev2 := event.New()
	ev2.CreatedAt = time.Now().Unix()
	ev2.Kind = 30023
	ev2.Content = []byte("test content")
	ev2.Tags = tag.NewS()
	addTagToEvent(ev2, "d", "INVALID_SLUG")
	if err := ev2.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	allowed2, err := policy.CheckPolicy("write", ev2, signer.Pub(), "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy returned error: %v", err)
	}

	if allowed2 {
		t.Error("Expected follow with invalid tag to be rejected (tag validation should still apply)")
	}
}

// TestTagValidationGlobalRule tests tag validation in global rules
func TestTagValidationGlobalRule(t *testing.T) {
	policy, cleanup := setupTagValidationTestPolicy(t, "test-tag-global")
	defer cleanup()

	// Policy with global tag validation (applies to all kinds)
	policyJSON := []byte(`{
		"default_policy": "allow",
		"global": {
			"tag_validation": {
				"e": "^[a-f0-9]{64}$"
			}
		}
	}`)

	tmpDir := t.TempDir()
	if err := policy.Reload(policyJSON, tmpDir+"/policy.json"); err != nil {
		t.Fatalf("Failed to reload policy: %v", err)
	}

	// Valid e tag (64 hex chars)
	ev1, signer1 := createSignedTestEvent(t, 1, "test")
	addTagToEvent(ev1, "e", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err := ev1.Sign(signer1); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	allowed1, _ := policy.CheckPolicy("write", ev1, signer1.Pub(), "127.0.0.1")
	if !allowed1 {
		t.Error("Expected valid e tag to be allowed")
	}

	// Invalid e tag (not 64 hex chars)
	ev2, signer2 := createSignedTestEvent(t, 1, "test")
	addTagToEvent(ev2, "e", "not-a-valid-event-id")
	if err := ev2.Sign(signer2); chk.E(err) {
		t.Fatalf("Failed to sign event: %v", err)
	}

	allowed2, _ := policy.CheckPolicy("write", ev2, signer2.Pub(), "127.0.0.1")
	if allowed2 {
		t.Error("Expected invalid e tag to be rejected")
	}
}
