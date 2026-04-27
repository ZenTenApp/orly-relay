package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// Helper function to create int64 pointer
func int64Ptr(i int64) *int64 {
	return &i
}

// Helper function to generate a keypair for testing
func generateTestKeypair(t *testing.T) (signer *p8k.Signer, pubkey []byte) {
	signer = p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		t.Fatalf("Failed to generate test keypair: %v", err)
	}
	pubkey = signer.Pub()
	return
}

// Helper function to generate a keypair for benchmarks
func generateTestKeypairB(b *testing.B) (signer *p8k.Signer, pubkey []byte) {
	signer = p8k.MustNew()
	if err := signer.Generate(); chk.E(err) {
		b.Fatalf("Failed to generate test keypair: %v", err)
	}
	pubkey = signer.Pub()
	return
}

// Helper function to create a real test event with proper signing
func createTestEvent(t *testing.T, signer *p8k.Signer, content string, kind uint16) *event.E {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Sign the event properly
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign test event: %v", err)
	}

	return ev
}

// Helper function to create a test event with a specific pubkey (for unauthorized tests)
func createTestEventWithPubkey(t *testing.T, signer *p8k.Signer, content string, kind uint16) *event.E {
	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = kind
	ev.Content = []byte(content)
	ev.Tags = tag.NewS()

	// Sign the event properly
	if err := ev.Sign(signer); chk.E(err) {
		t.Fatalf("Failed to sign test event: %v", err)
	}

	return ev
}

// Helper function to add p tag with hex-encoded pubkey to event
func addPTag(ev *event.E, pubkey []byte) {
	pTag := tag.NewFromAny("p", hex.Enc(pubkey))
	ev.Tags.Append(pTag)
}

// Helper function to add other tags to event
func addTag(ev *event.E, key, value string) {
	tagItem := tag.NewFromAny(key, value)
	ev.Tags.Append(tagItem)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		policyJSON  []byte
		expectError bool
		expectRules int
	}{
		{
			name:        "empty JSON",
			policyJSON:  []byte("{}"),
			expectError: false,
			expectRules: 0,
		},
		{
			name:        "valid policy JSON",
			policyJSON:  []byte(`{"kind":{"whitelist":[1,3,5]},"rules":{"1":{"description":"test"}}}`),
			expectError: false,
			expectRules: 1,
		},
		{
			name:        "invalid JSON",
			policyJSON:  []byte(`{"invalid": json}`),
			expectError: true,
			expectRules: 0,
		},
		{
			name:        "nil JSON",
			policyJSON:  nil,
			expectError: false,
			expectRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := New(tt.policyJSON)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if policy == nil {
				t.Errorf("Expected policy but got nil")
				return
			}
			if len(policy.rules) != tt.expectRules {
				t.Errorf("Expected %d rules, got %d", tt.expectRules, len(policy.rules))
			}
		})
	}
}

func TestCheckKindsPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policy   *P
		access   string // "read" or "write"
		kind     uint16
		expected bool
	}{
		{
			name: "no whitelist or blacklist - allow (no rules at all)",
			policy: &P{
				Kind:  Kinds{},
				rules: map[int]Rule{}, // No rules defined
			},
			access:   "write",
			kind:     1,
			expected: true, // Should be allowed (no rules = allow all kinds)
		},
		{
			name: "no whitelist or blacklist - deny (has other rules)",
			policy: &P{
				Kind: Kinds{},
				rules: map[int]Rule{
					2: {Description: "Rule for kind 2"},
				},
			},
			access:   "write",
			kind:     1,
			expected: false, // Should be denied (implicit whitelist, no rule for kind 1)
		},
		{
			name: "no whitelist or blacklist - allow (has rule)",
			policy: &P{
				Kind: Kinds{},
				rules: map[int]Rule{
					1: {Description: "Rule for kind 1"},
				},
			},
			access:   "write",
			kind:     1,
			expected: true, // Should be allowed (has rule)
		},
		{
			name: "no whitelist or blacklist - allow (has global rule)",
			policy: &P{
				Kind: Kinds{},
				Global: Rule{
					AccessControl: AccessControl{
						WriteAllow: []string{"test"}, // Global rule exists
					},
				},
				rules: map[int]Rule{}, // No specific rules
			},
			access:   "write",
			kind:     1,
			expected: true, // Should be allowed (global rule exists)
		},
		{
			name: "whitelist - kind allowed",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
			},
			access:   "write",
			kind:     1,
			expected: true,
		},
		{
			name: "whitelist - kind not allowed",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
			},
			access:   "write",
			kind:     2,
			expected: false,
		},
		{
			name: "blacklist - kind not blacklisted (no rule)",
			policy: &P{
				Kind: Kinds{
					Blacklist: []int{2, 4, 6},
				},
				rules: map[int]Rule{
					3: {Description: "Rule for kind 3"}, // Has at least one rule
				},
			},
			access:   "write",
			kind:     1,
			expected: false, // Should be denied (not blacklisted but no rule for kind 1)
		},
		{
			name: "blacklist - kind not blacklisted (has rule)",
			policy: &P{
				Kind: Kinds{
					Blacklist: []int{2, 4, 6},
				},
				rules: map[int]Rule{
					1: {Description: "Rule for kind 1"},
				},
			},
			access:   "write",
			kind:     1,
			expected: true, // Should be allowed (not blacklisted and has rule)
		},
		{
			name: "blacklist - kind blacklisted",
			policy: &P{
				Kind: Kinds{
					Blacklist: []int{2, 4, 6},
				},
			},
			access:   "write",
			kind:     2,
			expected: false,
		},
		{
			name: "whitelist overrides blacklist",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
					Blacklist: []int{1, 2, 3},
				},
			},
			access:   "write",
			kind:     1,
			expected: true,
		},
		// Tests for new permissive flags
		{
			name: "read_allow_permissive - allows read for non-whitelisted kind",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
				Global: Rule{
					AccessControl: AccessControl{
						ReadAllowPermissive: true,
					},
				},
			},
			access:   "read",
			kind:     2,
			expected: true, // Should be allowed (read permissive overrides whitelist)
		},
		{
			name: "read_allow_permissive - write still blocked for non-whitelisted kind",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
				Global: Rule{
					AccessControl: AccessControl{
						ReadAllowPermissive: true,
					},
				},
			},
			access:   "write",
			kind:     2,
			expected: false, // Should be denied (only read is permissive)
		},
		{
			name: "write_allow_permissive - allows write for non-whitelisted kind",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
				Global: Rule{
					AccessControl: AccessControl{
						WriteAllowPermissive: true,
					},
				},
			},
			access:   "write",
			kind:     2,
			expected: true, // Should be allowed (write permissive overrides whitelist)
		},
		{
			name: "write_allow_permissive - read still blocked for non-whitelisted kind",
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{1, 3, 5},
				},
				Global: Rule{
					AccessControl: AccessControl{
						WriteAllowPermissive: true,
					},
				},
			},
			access:   "read",
			kind:     2,
			expected: false, // Should be denied (only write is permissive)
		},
		{
			name: "blacklist - permissive flags do NOT override blacklist",
			policy: &P{
				Kind: Kinds{
					Blacklist: []int{2, 4, 6},
				},
				Global: Rule{
					AccessControl: AccessControl{
						ReadAllowPermissive:  true,
						WriteAllowPermissive: true,
					},
				},
			},
			access:   "write",
			kind:     2,
			expected: false, // Should be denied (blacklist always applies)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.policy.checkKindsPolicy(tt.access, tt.kind)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckRulePolicy(t *testing.T) {
	// Generate real keypairs for testing
	eventSigner, eventPubkey := generateTestKeypair(t)
	_, pTagPubkey := generateTestKeypair(t)
	_, unauthorizedPubkey := generateTestKeypair(t)

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)
	// Add p tag with hex-encoded pubkey
	addPTag(testEvent, pTagPubkey)
	addTag(testEvent, "expiration", "1234567890")

	tests := []struct {
		name           string
		access         string
		event          *event.E
		rule           Rule
		loggedInPubkey []byte
		expected       bool
	}{
		{
			name:   "write access - no restrictions",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "no restrictions",
			},
			loggedInPubkey: eventPubkey,
			expected:       true,
		},
		{
			name:   "write access - pubkey allowed",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "pubkey allowed",
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(testEvent.Pubkey)},
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       true,
		},
		{
			name:   "write access - pubkey not allowed",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "pubkey not allowed",
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(pTagPubkey)}, // Different pubkey
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       false,
		},
		{
			name:   "size limit - within limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "size limit",
				Constraints: Constraints{
					SizeLimit: int64Ptr(10000),
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       true,
		},
		{
			name:   "size limit - exceeds limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "size limit exceeded",
				Constraints: Constraints{
					SizeLimit: int64Ptr(10),
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       false,
		},
		{
			name:   "content limit - within limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "content limit",
				Constraints: Constraints{
					ContentLimit: int64Ptr(1000),
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       true,
		},
		{
			name:   "content limit - exceeds limit",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "content limit exceeded",
				Constraints: Constraints{
					ContentLimit: int64Ptr(5),
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       false,
		},
		{
			name:   "required tags - has required tag",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "required tags",
				TagValidationConfig: TagValidationConfig{
					MustHaveTags: []string{"p"},
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       true,
		},
		{
			name:   "required tags - missing required tag",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "required tags missing",
				TagValidationConfig: TagValidationConfig{
					MustHaveTags: []string{"e"},
				},
			},
			loggedInPubkey: eventPubkey,
			expected:       false,
		},
		{
			name:   "privileged write - event authored by logged in user (privileged doesn't affect write)",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: testEvent.Pubkey,
			expected:       true, // Privileged doesn't restrict write, uses default (allow)
		},
		{
			name:   "privileged write - event contains logged in user in p tag (privileged doesn't affect write)",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event with p tag",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: pTagPubkey,
			expected:       true, // Privileged doesn't restrict write, uses default (allow)
		},
		{
			name:   "privileged write - not authenticated (privileged doesn't affect write)",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event not authenticated",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: nil,
			expected:       true, // Privileged doesn't restrict write, uses default (allow)
		},
		{
			name:   "privileged write - authenticated but not authorized (privileged doesn't affect write)",
			access: "write",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event unauthorized user",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: unauthorizedPubkey,
			expected:       true, // Privileged doesn't restrict write, uses default (allow)
		},
		{
			name:   "privileged read - event authored by logged in user",
			access: "read",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event read access",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: testEvent.Pubkey,
			expected:       true,
		},
		{
			name:   "privileged read - event contains logged in user in p tag",
			access: "read",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event read access with p tag",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: pTagPubkey,
			expected:       true,
		},
		{
			name:   "privileged read - not authenticated",
			access: "read",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event read access not authenticated",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: nil,
			expected:       false,
		},
		{
			name:   "privileged read - authenticated but not authorized (different pubkey, not in p tags)",
			access: "read",
			event:  testEvent,
			rule: Rule{
				Description: "privileged event read access unauthorized user",
				Constraints: Constraints{
					Privileged: true,
				},
			},
			loggedInPubkey: unauthorizedPubkey,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{}
			result, err := policy.checkRulePolicy(tt.access, tt.event, tt.rule, tt.loggedInPubkey)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckPolicy(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	tests := []struct {
		name           string
		access         string
		event          *event.E
		policy         *P
		loggedInPubkey []byte
		ipAddress      string
		expected       bool
		expectError    bool
	}{
		{
			name:   "no policy rules - allow",
			access: "write",
			event:  testEvent,
			policy: &P{
				Kind:  Kinds{},
				rules: map[int]Rule{},
			},
			loggedInPubkey: eventPubkey,
			ipAddress:      "127.0.0.1",
			expected:       true,
			expectError:    false,
		},
		{
			name:   "kinds policy blocks - deny",
			access: "write",
			event:  testEvent,
			policy: &P{
				Kind: Kinds{
					Whitelist: []int{3, 5},
				},
				rules: map[int]Rule{},
			},
			loggedInPubkey: eventPubkey,
			ipAddress:      "127.0.0.1",
			expected:       false,
			expectError:    false,
		},
		{
			name:   "rule blocks - deny",
			access: "write",
			event:  testEvent,
			policy: &P{
				Kind: Kinds{},
				rules: map[int]Rule{
					1: {
						Description: "block test",
						AccessControl: AccessControl{
							WriteDeny: []string{hex.Enc(testEvent.Pubkey)},
						},
					},
				},
			},
			loggedInPubkey: eventPubkey,
			ipAddress:      "127.0.0.1",
			expected:       false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.policy.CheckPolicy(tt.access, tt.event, tt.loggedInPubkey, tt.ipAddress)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "policy.json")

	tests := []struct {
		name        string
		configData  string
		expectError bool
		expectRules int
	}{
		{
			name:        "valid policy file",
			configData:  `{"kind":{"whitelist":[1,3,5]},"rules":{"1":{"description":"test"}}}`,
			expectError: false,
			expectRules: 1,
		},
		{
			name:        "empty policy file",
			configData:  `{}`,
			expectError: false,
			expectRules: 0,
		},
		{
			name:        "invalid JSON",
			configData:  `{"invalid": json}`,
			expectError: true,
			expectRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test config file
			if tt.configData != "" {
				err := os.WriteFile(configPath, []byte(tt.configData), 0644)
				if err != nil {
					t.Fatalf("Failed to write test config file: %v", err)
				}
			}

			policy := &P{}
			err := policy.LoadFromFile(configPath)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if len(policy.rules) != tt.expectRules {
				t.Errorf("Expected %d rules, got %d", tt.expectRules, len(policy.rules))
			}
		})
	}

	// Test file not found
	t.Run("file not found", func(t *testing.T) {
		policy := &P{}
		err := policy.LoadFromFile("/nonexistent/policy.json")
		if err == nil {
			t.Errorf("Expected error for nonexistent file but got none")
		}
	})
}

func TestPolicyEventSerialization(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Create policy event
	policyEvent := &PolicyEvent{
		E:              testEvent,
		LoggedInPubkey: hex.Enc(eventPubkey),
		IPAddress:      "127.0.0.1",
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(policyEvent)
	if err != nil {
		t.Fatalf("Failed to marshal policy event: %v", err)
	}

	// Verify the JSON contains expected fields
	jsonStr := string(jsonData)
	t.Logf("Generated JSON: %s", jsonStr)

	if !strings.Contains(jsonStr, hex.Enc(eventPubkey)) {
		t.Error("JSON should contain logged_in_pubkey field")
	}
	if !strings.Contains(jsonStr, "127.0.0.1") {
		t.Error("JSON should contain ip_address field")
	}
	if !strings.Contains(jsonStr, hex.Enc(testEvent.ID)) {
		t.Error("JSON should contain event id field (hex encoded)")
	}

	// Test with nil event
	nilPolicyEvent := &PolicyEvent{
		E:              nil,
		LoggedInPubkey: "test-logged-in-pubkey",
		IPAddress:      "127.0.0.1",
	}

	jsonData2, err := json.Marshal(nilPolicyEvent)
	if err != nil {
		t.Fatalf("Failed to marshal nil policy event: %v", err)
	}

	jsonStr2 := string(jsonData2)
	if !strings.Contains(jsonStr2, "test-logged-in-pubkey") {
		t.Error("JSON should contain logged_in_pubkey field even with nil event")
	}
	if !strings.Contains(jsonStr2, "127.0.0.1") {
		t.Error("JSON should contain ip_address field even with nil event")
	}
}

func TestPolicyResponseSerialization(t *testing.T) {
	// Test JSON serialization
	response := &PolicyResponse{
		ID:     "test-id",
		Action: "accept",
		Msg:    "test message",
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal policy response: %v", err)
	}

	// Test JSON deserialization
	var deserializedResponse PolicyResponse
	err = json.Unmarshal(jsonData, &deserializedResponse)
	if err != nil {
		t.Fatalf("Failed to unmarshal policy response: %v", err)
	}

	// Verify fields
	if deserializedResponse.ID != response.ID {
		t.Errorf("Expected ID %s, got %s", response.ID, deserializedResponse.ID)
	}
	if deserializedResponse.Action != response.Action {
		t.Errorf("Expected Action %s, got %s", response.Action, deserializedResponse.Action)
	}
	if deserializedResponse.Msg != response.Msg {
		t.Errorf("Expected Msg %s, got %s", response.Msg, deserializedResponse.Msg)
	}
}

func TestNewWithManager(t *testing.T) {
	ctx := context.Background()
	appName := "test-app"

	// Test with disabled policy (doesn't require policy.json file)
	t.Run("disabled policy", func(t *testing.T) {
		enabled := false
		policy := NewWithManager(ctx, appName, enabled, "")

		if policy == nil {
			t.Fatal("Expected policy but got nil")
		}

		if policy.manager == nil {
			t.Fatal("Expected policy manager but got nil")
		}

		if policy.manager.IsEnabled() {
			t.Error("Expected policy manager to be disabled")
		}

		if policy.manager.IsRunning() {
			t.Error("Expected policy manager to not be running")
		}

		// Verify default policy was set
		if policy.DefaultPolicy != "allow" {
			t.Errorf("Expected default_policy='allow', got '%s'", policy.DefaultPolicy)
		}

		// Clean up
		policy.manager.Shutdown()
	})

	// Test with enabled policy and valid config file
	t.Run("enabled policy with valid config", func(t *testing.T) {
		// Create a temporary config directory with a valid policy.json
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "test-policy-enabled")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Write a minimal valid policy.json
		policyJSON := `{
			"default_policy": "allow",
			"kind": {
				"whitelist": [1, 3, 4]
			},
			"rules": {
				"1": {
					"description": "Text notes"
				}
			}
		}`
		policyPath := filepath.Join(configDir, "policy.json")
		if err := os.WriteFile(policyPath, []byte(policyJSON), 0644); err != nil {
			t.Fatalf("Failed to write policy.json: %v", err)
		}

		// Create policy manager manually to use custom config path
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		manager := &PolicyManager{
			ctx:        ctx,
			cancel:     cancel,
			configDir:  configDir,
			scriptPath: filepath.Join(configDir, "policy.sh"),
			enabled:    true,
			runners:    make(map[string]*ScriptRunner),
		}

		policy := &P{
			DefaultPolicy: "allow",
			manager:       manager,
		}

		// Load policy from our test file
		if err := policy.LoadFromFile(policyPath); err != nil {
			t.Fatalf("Failed to load policy: %v", err)
		}

		if policy.manager == nil {
			t.Fatal("Expected policy manager but got nil")
		}

		if !policy.manager.IsEnabled() {
			t.Error("Expected policy manager to be enabled")
		}

		// Verify policy was loaded correctly
		if len(policy.Kind.Whitelist) != 3 {
			t.Errorf("Expected 3 whitelisted kinds, got %d", len(policy.Kind.Whitelist))
		}

		if policy.DefaultPolicy != "allow" {
			t.Errorf("Expected default_policy='allow', got '%s'", policy.DefaultPolicy)
		}

		// Clean up
		policy.manager.Shutdown()
	})
}

func TestPolicyManagerLifecycle(t *testing.T) {
	// Test basic manager initialization without script execution
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  "/tmp",
		scriptPath: "/tmp/policy.sh",
		enabled:    true,
		runners:    make(map[string]*ScriptRunner),
	}

	// Test manager state
	if !manager.IsEnabled() {
		t.Error("Expected policy manager to be enabled")
	}

	if manager.IsRunning() {
		t.Error("Expected policy manager to not be running initially")
	}

	// Test getting or creating a runner for a non-existent script
	runner := manager.getOrCreateRunner("/tmp/policy.sh")
	if runner == nil {
		t.Fatal("Expected runner to be created")
	}

	// Test starting with non-existent script (should fail gracefully)
	err := runner.Start()
	if err == nil {
		t.Error("Expected error when starting script with non-existent file")
	}

	// Test stopping when not running (should fail gracefully)
	err = runner.Stop()
	if err == nil {
		t.Error("Expected error when stopping script that's not running")
	}
}

func TestPolicyManagerProcessEvent(t *testing.T) {
	// Test processing event when runner is not running (should fail gracefully)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  "/tmp",
		scriptPath: "/tmp/policy.sh",
		enabled:    true,
		runners:    make(map[string]*ScriptRunner),
	}

	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Create policy event
	policyEvent := &PolicyEvent{
		E:              testEvent,
		LoggedInPubkey: hex.Enc(eventPubkey),
		IPAddress:      "127.0.0.1",
	}

	// Get or create a runner
	runner := manager.getOrCreateRunner("/tmp/policy.sh")

	// Process event when not running (should fail gracefully)
	_, err := runner.ProcessEvent(policyEvent)
	if err == nil {
		t.Error("Expected error when processing event with non-running script")
	}
}

func TestEdgeCasesEmptyPolicy(t *testing.T) {
	policy := &P{}

	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow all events when policy is empty
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with empty policy")
	}
}

func TestEdgeCasesNilEvent(t *testing.T) {
	policy := &P{}

	// Should handle nil event gracefully
	allowed, err := policy.CheckPolicy("write", nil, []byte("test-pubkey"), "127.0.0.1")
	if err == nil {
		t.Error("Expected error when event is nil")
	}
	if allowed {
		t.Error("Expected event to be blocked when nil")
	}

	// Verify the error message
	if err != nil && !strings.Contains(err.Error(), "event cannot be nil") {
		t.Errorf("Expected error message to contain 'event cannot be nil', got: %v", err)
	}
}

func TestEdgeCasesLargeEvent(t *testing.T) {
	// Create large content
	largeContent := strings.Repeat("a", 100000) // 100KB content

	policy := &P{
		Kind: Kinds{},
		rules: map[int]Rule{
			1: {
				Description: "size limit test",
				Constraints: Constraints{
					SizeLimit:    int64Ptr(50000), // 50KB limit
					ContentLimit: int64Ptr(10000), // 10KB content limit
				},
			},
		},
	}

	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Create real test event with large content
	testEvent := createTestEvent(t, eventSigner, largeContent, 1)

	// Should block large event
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if allowed {
		t.Error("Expected large event to be blocked")
	}
}

func TestEdgeCasesWhitelistBlacklistConflict(t *testing.T) {
	policy := &P{
		Kind: Kinds{
			Whitelist: []int{1, 3, 5},
			Blacklist: []int{1, 2, 3}, // Overlap with whitelist
		},
	}

	// Test kind in both whitelist and blacklist - whitelist should win
	allowed := policy.checkKindsPolicy("write", 1)
	if !allowed {
		t.Error("Expected whitelist to override blacklist")
	}

	// Test kind in blacklist but not whitelist
	allowed = policy.checkKindsPolicy("write", 2)
	if allowed {
		t.Error("Expected kind in blacklist but not whitelist to be blocked")
	}

	// Test kind in whitelist but not blacklist
	allowed = policy.checkKindsPolicy("write", 5)
	if !allowed {
		t.Error("Expected kind in whitelist to be allowed")
	}
}

func TestEdgeCasesManagerWithInvalidScript(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "policy.sh")

	// Create invalid script (not executable, wrong shebang, etc.)
	scriptContent := `invalid script content`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644) // Not executable
	if err != nil {
		t.Fatalf("Failed to create invalid script: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  tempDir,
		scriptPath: scriptPath,
		enabled:    true,
		runners:    make(map[string]*ScriptRunner),
	}

	// Get runner and try to start with invalid script
	runner := manager.getOrCreateRunner(scriptPath)
	err = runner.Start()
	if err == nil {
		t.Error("Expected error when starting invalid script")
	}
}

func TestEdgeCasesManagerDoubleStart(t *testing.T) {
	// Test double start without actually starting (simpler test)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  "/tmp",
		scriptPath: "/tmp/policy.sh",
		enabled:    true,
		runners:    make(map[string]*ScriptRunner),
	}

	// Get runner
	runner := manager.getOrCreateRunner("/tmp/policy.sh")

	// Try to start with non-existent script - should fail
	err := runner.Start()
	if err == nil {
		t.Error("Expected error when starting script with non-existent file")
	}

	// Try to start again - should still fail
	err = runner.Start()
	if err == nil {
		t.Error("Expected error when starting script twice")
	}
}

func TestCheckGlobalRulePolicy(t *testing.T) {
	// Generate real keypairs for testing
	eventSigner, _ := generateTestKeypair(t)
	_, loggedInPubkey := generateTestKeypair(t)

	tests := []struct {
		name           string
		globalRule     Rule
		event          *event.E
		loggedInPubkey []byte
		expected       bool
	}{
		{
			name: "global rule with write allow - submitter allowed",
			globalRule: Rule{
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(loggedInPubkey)}, // Allow the submitter
				},
			},
			event:          createTestEvent(t, eventSigner, "test content", 1),
			loggedInPubkey: loggedInPubkey,
			expected:       true,
		},
		{
			name: "global rule with write deny - submitter denied",
			globalRule: Rule{
				AccessControl: AccessControl{
					WriteDeny: []string{hex.Enc(loggedInPubkey)}, // Deny the submitter
				},
			},
			event:          createTestEvent(t, eventSigner, "test content", 1),
			loggedInPubkey: loggedInPubkey,
			expected:       false,
		},
		{
			name: "global rule with size limit - event too large",
			globalRule: Rule{
				Constraints: Constraints{
					SizeLimit: func() *int64 { v := int64(10); return &v }(),
				},
			},
			event:          createTestEvent(t, eventSigner, "this is a very long content that exceeds the size limit", 1),
			loggedInPubkey: loggedInPubkey,
			expected:       false,
		},
		{
			name: "global rule with max age of event - event too old",
			globalRule: Rule{
				Constraints: Constraints{
					MaxAgeOfEvent: func() *int64 { v := int64(3600); return &v }(), // 1 hour
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() - 7200 // 2 hours ago
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       false,
		},
		{
			name: "global rule with max age event in future - event too far in future",
			globalRule: Rule{
				Constraints: Constraints{
					MaxAgeEventInFuture: func() *int64 { v := int64(3600); return &v }(), // 1 hour
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 7200 // 2 hours in future
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{
				Global: tt.globalRule,
			}

			result := policy.checkGlobalRulePolicy("write", tt.event, tt.loggedInPubkey)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCheckPolicyWithGlobalRule(t *testing.T) {
	// Generate real keypairs for testing
	eventSigner, eventPubkey := generateTestKeypair(t)
	_, loggedInPubkey := generateTestKeypair(t)

	// Test that global rule is applied first
	policy := &P{
		Global: Rule{
			AccessControl: AccessControl{
				WriteDeny: []string{hex.Enc(eventPubkey)}, // Deny event pubkey globally
			},
		},
		Kind: Kinds{
			Whitelist: []int{1}, // Allow kind 1
		},
		rules: map[int]Rule{
			1: {
				AccessControl: AccessControl{
					WriteAllow: []string{hex.Enc(eventPubkey)}, // Allow event pubkey for kind 1
				},
			},
		},
	}

	event := createTestEvent(t, eventSigner, "test content", 1)

	// Global rule should deny this event even though kind-specific rule would allow it
	allowed, err := policy.CheckPolicy("write", event, loggedInPubkey, "127.0.0.1")
	if err != nil {
		t.Fatalf("CheckPolicy failed: %v", err)
	}

	if allowed {
		t.Error("Expected event to be denied by global rule, but it was allowed")
	}
}

func TestMaxAgeChecks(t *testing.T) {
	// Generate real keypairs for testing
	eventSigner, _ := generateTestKeypair(t)
	_, loggedInPubkey := generateTestKeypair(t)

	tests := []struct {
		name           string
		rule           Rule
		event          *event.E
		loggedInPubkey []byte
		expected       bool
	}{
		{
			name: "max age of event - event within allowed age",
			rule: Rule{
				Constraints: Constraints{
					MaxAgeOfEvent: func() *int64 { v := int64(3600); return &v }(), // 1 hour
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() - 1800 // 30 minutes ago
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       true,
		},
		{
			name: "max age of event - event too old",
			rule: Rule{
				Constraints: Constraints{
					MaxAgeOfEvent: func() *int64 { v := int64(3600); return &v }(), // 1 hour
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() - 7200 // 2 hours ago
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       false,
		},
		{
			name: "max age event in future - event within allowed future time",
			rule: Rule{
				Constraints: Constraints{
					MaxAgeEventInFuture: func() *int64 { v := int64(3600); return &v }(), // 1 hour
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 1800 // 30 minutes in future
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       true,
		},
		{
			name: "max age event in future - event too far in future",
			rule: Rule{
				Constraints: Constraints{
					MaxAgeEventInFuture: func() *int64 { v := int64(3600); return &v }(), // 1 hour
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 7200 // 2 hours in future
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       false,
		},
		{
			name: "both age checks - event within both limits",
			rule: Rule{
				Constraints: Constraints{
					MaxAgeOfEvent:       func() *int64 { v := int64(3600); return &v }(), // 1 hour
					MaxAgeEventInFuture: func() *int64 { v := int64(1800); return &v }(), // 30 minutes
				},
			},
			event: func() *event.E {
				ev := createTestEvent(t, eventSigner, "test content", 1)
				ev.CreatedAt = time.Now().Unix() + 900 // 15 minutes in future
				return ev
			}(),
			loggedInPubkey: loggedInPubkey,
			expected:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &P{}

			allowed, err := policy.checkRulePolicy("write", tt.event, tt.rule, tt.loggedInPubkey)
			if err != nil {
				t.Fatalf("checkRulePolicy failed: %v", err)
			}

			if allowed != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, allowed)
			}
		})
	}
}

func TestScriptPolicyDisabledFallsBackToDefault(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Create a policy with a script rule but policy is disabled, default policy is "allow"
	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			1: {
				Description: "script rule",
				Script:      "policy.sh",
			},
		},
		manager: &PolicyManager{
			enabled: false, // Policy is disabled
			runners: make(map[string]*ScriptRunner),
		},
	}

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow the event when policy is disabled (falls back to default "allow")
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed when policy is disabled (should fall back to default policy 'allow')")
	}

	// Test with default policy "deny"
	policy.DefaultPolicy = "deny"
	allowed2, err2 := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error: %v", err2)
	}
	if allowed2 {
		t.Error("Expected event to be denied when policy is disabled and default policy is 'deny'")
	}
}

func TestDefaultPolicyAllow(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Test default policy "allow" behavior
	policy := &P{
		DefaultPolicy: "allow",
		Kind:          Kinds{},
		rules:         map[int]Rule{}, // No specific rules
	}

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow the event with default policy "allow"
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with default_policy 'allow'")
	}
}

func TestDefaultPolicyDeny(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Test default policy "deny" behavior
	policy := &P{
		DefaultPolicy: "deny",
		Kind:          Kinds{},
		rules:         map[int]Rule{}, // No specific rules
	}

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should deny the event with default policy "deny"
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if allowed {
		t.Error("Expected event to be denied with default_policy 'deny'")
	}
}

func TestDefaultPolicyEmpty(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Test empty default policy (should default to "allow")
	policy := &P{
		DefaultPolicy: "",
		Kind:          Kinds{},
		rules:         map[int]Rule{}, // No specific rules
	}

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow the event with empty default policy (defaults to "allow")
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with empty default_policy (should default to 'allow')")
	}
}

func TestDefaultPolicyInvalid(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Test invalid default policy (should default to "allow")
	policy := &P{
		DefaultPolicy: "invalid",
		Kind:          Kinds{},
		rules:         map[int]Rule{}, // No specific rules
	}

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow the event with invalid default policy (defaults to "allow")
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed with invalid default_policy (should default to 'allow')")
	}
}

func TestDefaultPolicyWithSpecificRule(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Test that specific rules override default policy
	policy := &P{
		DefaultPolicy: "deny", // Default is deny
		Kind:          Kinds{},
		rules: map[int]Rule{
			1: {
				Description: "allow kind 1",
				AccessControl: AccessControl{
					WriteAllow: []string{}, // Allow all for kind 1
				},
			},
		},
	}

	// Create real test event with proper signing for kind 1 (has specific rule)
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow the event because specific rule allows it, despite default policy being "deny"
	allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed by specific rule, despite default_policy 'deny'")
	}

	// Create real test event with proper signing for kind 2 (no specific rule exists)
	testEvent2 := createTestEvent(t, eventSigner, "test content", 2)

	// Should deny the event because no specific rule and default policy is "deny"
	allowed2, err2 := policy.CheckPolicy("write", testEvent2, eventPubkey, "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error: %v", err2)
	}
	if allowed2 {
		t.Error("Expected event to be denied with default_policy 'deny' for kind without specific rule")
	}
}

func TestNewPolicyDefaultsToAllow(t *testing.T) {
	// Test that New() function sets default policy to "allow"
	policy, err := New([]byte(`{}`))
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if policy.DefaultPolicy != "allow" {
		t.Errorf("Expected default policy to be 'allow', got '%s'", policy.DefaultPolicy)
	}
}

func TestNewPolicyWithDefaultPolicyJSON(t *testing.T) {
	// Test loading default policy from JSON
	jsonConfig := `{"default_policy": "deny"}`
	policy, err := New([]byte(jsonConfig))
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if policy.DefaultPolicy != "deny" {
		t.Errorf("Expected default policy to be 'deny', got '%s'", policy.DefaultPolicy)
	}
}

func TestScriptProcessingDisabledFallsBackToDefault(t *testing.T) {
	// Generate real keypair for testing
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Test that when policy is disabled, it falls back to default policy
	policy := &P{
		DefaultPolicy: "allow",
		rules: map[int]Rule{
			1: {
				Description: "script rule",
				Script:      "policy.sh",
			},
		},
		manager: &PolicyManager{
			enabled: false, // Policy is disabled
			runners: make(map[string]*ScriptRunner),
		},
	}

	// Create real test event with proper signing
	testEvent := createTestEvent(t, eventSigner, "test content", 1)

	// Should allow the event when policy is disabled (falls back to default "allow")
	allowed, err := policy.checkScriptPolicy("write", testEvent, "policy.sh", eventPubkey, "127.0.0.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !allowed {
		t.Error("Expected event to be allowed when policy is disabled (should fall back to default policy 'allow')")
	}

	// Test with default policy "deny"
	policy.DefaultPolicy = "deny"
	allowed2, err2 := policy.checkScriptPolicy("write", testEvent, "policy.sh", eventPubkey, "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error: %v", err2)
	}
	if allowed2 {
		t.Error("Expected event to be denied when policy is disabled and default policy is 'deny'")
	}
}

func TestDefaultPolicyLogicWithRules(t *testing.T) {
	// Generate real keypairs for testing
	testSigner, _ := generateTestKeypair(t)
	_, deniedPubkey := generateTestKeypair(t)  // Only need pubkey for denied user
	_, loggedInPubkey := generateTestKeypair(t)

	// Test that default policy logic works correctly with rules

	// Test 1: default_policy "deny" - should only allow if rule explicitly allows
	policy1 := &P{
		DefaultPolicy: "deny",
		Kind: Kinds{
			Whitelist: []int{1, 2, 3}, // Allow kinds 1, 2, 3
		},
		rules: map[int]Rule{
			1: {
				Description: "allow all for kind 1",
				AccessControl: AccessControl{
					WriteAllow: []string{}, // Empty means allow all
				},
			},
			2: {
				Description: "deny specific pubkey for kind 2",
				AccessControl: AccessControl{
					WriteDeny: []string{hex.Enc(deniedPubkey)},
				},
			},
			// No rule for kind 3
		},
	}

	// Kind 1: has rule that allows all - should be allowed
	event1 := createTestEvent(t, testSigner, "content", 1)
	allowed1, err1 := policy1.CheckPolicy("write", event1, loggedInPubkey, "127.0.0.1")
	if err1 != nil {
		t.Errorf("Unexpected error for kind 1: %v", err1)
	}
	if !allowed1 {
		t.Error("Expected kind 1 to be allowed (rule allows all)")
	}

	// Kind 2: has rule that denies specific pubkey - should be allowed for other pubkeys
	event2 := createTestEvent(t, testSigner, "content", 2)
	allowed2, err2 := policy1.CheckPolicy("write", event2, loggedInPubkey, "127.0.0.1")
	if err2 != nil {
		t.Errorf("Unexpected error for kind 2: %v", err2)
	}
	if !allowed2 {
		t.Error("Expected kind 2 to be allowed for non-denied pubkey")
	}

	// Kind 2: submitter in deny list should be denied
	event2Denied := createTestEvent(t, testSigner, "content", 2)  // Event can be from anyone
	allowed2Denied, err2Denied := policy1.CheckPolicy("write", event2Denied, deniedPubkey, "127.0.0.1")  // But submitted by denied user
	if err2Denied != nil {
		t.Errorf("Unexpected error for kind 2 denied: %v", err2Denied)
	}
	if allowed2Denied {
		t.Error("Expected kind 2 to be denied when submitter is in deny list")
	}

	// Kind 3: whitelisted but no rule - should follow default policy (deny)
	event3 := createTestEvent(t, testSigner, "content", 3)
	allowed3, err3 := policy1.CheckPolicy("write", event3, loggedInPubkey, "127.0.0.1")
	if err3 != nil {
		t.Errorf("Unexpected error for kind 3: %v", err3)
	}
	if allowed3 {
		t.Error("Expected kind 3 to be denied (no rule, default policy is deny)")
	}

	// Test 2: default_policy "allow" - should allow unless rule explicitly denies
	policy2 := &P{
		DefaultPolicy: "allow",
		Kind: Kinds{
			Whitelist: []int{1, 2, 3}, // Allow kinds 1, 2, 3
		},
		rules: map[int]Rule{
			1: {
				Description: "deny specific pubkey for kind 1",
				AccessControl: AccessControl{
					WriteDeny: []string{hex.Enc(deniedPubkey)},
				},
			},
			// No rules for kind 2, 3
		},
	}

	// Kind 1: has rule that denies specific pubkey - should be allowed for other pubkeys
	event1Allow := createTestEvent(t, testSigner, "content", 1)
	allowed1Allow, err1Allow := policy2.CheckPolicy("write", event1Allow, loggedInPubkey, "127.0.0.1")
	if err1Allow != nil {
		t.Errorf("Unexpected error for kind 1 allow: %v", err1Allow)
	}
	if !allowed1Allow {
		t.Error("Expected kind 1 to be allowed for non-denied pubkey")
	}

	// Kind 1: denied pubkey should be denied when they try to submit
	event1Deny := createTestEvent(t, testSigner, "content", 1)  // Event can be authored by anyone
	allowed1Deny, err1Deny := policy2.CheckPolicy("write", event1Deny, deniedPubkey, "127.0.0.1")  // But denied user cannot submit
	if err1Deny != nil {
		t.Errorf("Unexpected error for kind 1 deny: %v", err1Deny)
	}
	if allowed1Deny {
		t.Error("Expected kind 1 to be denied for denied pubkey")
	}

	// Kind 2: whitelisted but no rule - should follow default policy (allow)
	event2Allow := createTestEvent(t, testSigner, "content", 2)
	allowed2Allow, err2Allow := policy2.CheckPolicy("write", event2Allow, loggedInPubkey, "127.0.0.1")
	if err2Allow != nil {
		t.Errorf("Unexpected error for kind 2 allow: %v", err2Allow)
	}
	if !allowed2Allow {
		t.Error("Expected kind 2 to be allowed (no rule, default policy is allow)")
	}
}

func TestRuleScriptLoading(t *testing.T) {
	// This test validates that a policy script loads for a specific Rule
	// and properly processes events

	// Create temporary directory for test files
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "test-rule-script.sh")

	// Create a test script that accepts events with "allowed" in content
	scriptContent := `#!/bin/bash
while IFS= read -r line; do
    if echo "$line" | grep -q 'allowed'; then
        echo '{"action":"accept","msg":"Content approved"}'
    else
        echo '{"action":"reject","msg":"Content not allowed"}'
    fi
done
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	// Create policy manager with script support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  tempDir,
		scriptPath: filepath.Join(tempDir, "default-policy.sh"), // Different from rule script
		enabled:    true,
		runners:    make(map[string]*ScriptRunner),
	}

	// Create policy with a rule that uses the script
	policy := &P{
		DefaultPolicy: "deny",
		manager:       manager,
		rules: map[int]Rule{
			4678: {
				Description: "Test rule with custom script",
				Script:      scriptPath, // Rule-specific script path
			},
		},
	}

	// Generate test keypairs
	eventSigner, eventPubkey := generateTestKeypair(t)

	// Pre-start the script before running tests
	runner := manager.getOrCreateRunner(scriptPath)
	err = runner.Start()
	if err != nil {
		t.Fatalf("Failed to start script: %v", err)
	}

	// Wait for script to be ready
	time.Sleep(200 * time.Millisecond)

	if !runner.IsRunning() {
		t.Fatal("Script should be running after Start()")
	}

	// Test sending a warmup event to ensure script is responsive
	signer := p8k.MustNew()
	signer.Generate()
	warmupEv := event.New()
	warmupEv.CreatedAt = time.Now().Unix()
	warmupEv.Kind = 4678
	warmupEv.Content = []byte("warmup")
	warmupEv.Tags = tag.NewS()
	warmupEv.Sign(signer)

	warmupEvent := &PolicyEvent{
		E:         warmupEv,
		IPAddress: "127.0.0.1",
	}

	// Send warmup event to verify script is responding
	_, err = runner.ProcessEvent(warmupEvent)
	if err != nil {
		t.Fatalf("Script not responding to warmup event: %v", err)
	}

	t.Log("Script is ready and responding")

	// Test 1: Event with "allowed" content should be accepted
	t.Run("script_accepts_allowed_content", func(t *testing.T) {
		testEvent := createTestEvent(t, eventSigner, "this is allowed content", 4678)

		allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
		if err != nil {
			t.Logf("Policy check failed: %v", err)
			// Check if script exists
			if _, statErr := os.Stat(scriptPath); statErr != nil {
				t.Errorf("Script file error: %v", statErr)
			}
			t.Fatalf("Unexpected error during policy check: %v", err)
		}
		if !allowed {
			t.Error("Expected event with 'allowed' content to be accepted by script")
			t.Logf("Event content: %s", string(testEvent.Content))
		}

		// Verify the script runner was created and is running
		manager.mutex.RLock()
		runner, exists := manager.runners[scriptPath]
		manager.mutex.RUnlock()

		if !exists {
			t.Fatal("Expected script runner to be created for rule script path")
		}
		if !runner.IsRunning() {
			t.Error("Expected script runner to be running after processing event")
		}
	})

	// Test 2: Event without "allowed" content should be rejected
	t.Run("script_rejects_disallowed_content", func(t *testing.T) {
		testEvent := createTestEvent(t, eventSigner, "this is not permitted", 4678)

		allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event without 'allowed' content to be rejected by script")
		}
	})

	// Test 3: Verify script path is correct (rule-specific, not default)
	t.Run("script_path_is_rule_specific", func(t *testing.T) {
		manager.mutex.RLock()
		runner, exists := manager.runners[scriptPath]
		_, defaultExists := manager.runners[manager.scriptPath]
		manager.mutex.RUnlock()

		if !exists {
			t.Fatal("Expected rule-specific script runner to exist")
		}
		if defaultExists {
			t.Error("Default script runner should not be created when only rule-specific scripts are used")
		}

		// Verify the runner is using the correct script path
		if runner.scriptPath != scriptPath {
			t.Errorf("Expected runner to use script path %s, got %s", scriptPath, runner.scriptPath)
		}
	})

	// Test 4: Multiple events should use the same script instance
	t.Run("script_reused_for_multiple_events", func(t *testing.T) {
		// Get initial runner
		manager.mutex.RLock()
		initialRunner, _ := manager.runners[scriptPath]
		initialRunnerCount := len(manager.runners)
		manager.mutex.RUnlock()

		// Process multiple events
		for i := 0; i < 5; i++ {
			content := "this is allowed message " + string(rune('0'+i))
			testEvent := createTestEvent(t, eventSigner, content, 4678)
			_, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
			if err != nil {
				t.Errorf("Unexpected error on event %d: %v", i, err)
			}
		}

		// Verify same runner is used
		manager.mutex.RLock()
		currentRunner, _ := manager.runners[scriptPath]
		currentRunnerCount := len(manager.runners)
		manager.mutex.RUnlock()

		if currentRunner != initialRunner {
			t.Error("Expected same runner instance to be reused for multiple events")
		}
		if currentRunnerCount != initialRunnerCount {
			t.Errorf("Expected runner count to stay at %d, got %d", initialRunnerCount, currentRunnerCount)
		}
	})

	// Test 5: Different kind without script should use default policy
	t.Run("different_kind_uses_default_policy", func(t *testing.T) {
		testEvent := createTestEvent(t, eventSigner, "any content", 1) // Kind 1 has no rule

		allowed, err := policy.CheckPolicy("write", testEvent, eventPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Should be denied by default policy (deny)
		if allowed {
			t.Error("Expected event of kind without rule to be denied by default policy")
		}
	})

	// Cleanup: Stop the script
	manager.mutex.RLock()
	runner, exists := manager.runners[scriptPath]
	manager.mutex.RUnlock()
	if exists && runner.IsRunning() {
		runner.Stop()
	}
}

func TestPolicyFilterProcessing(t *testing.T) {
	// Test policy filter processing using the provided filter JSON specification
	filterJSON := []byte(`{
		"kind": {
			"whitelist": [4678, 10306, 30520, 30919]
		},
		"rules": {
			"4678": {
				"description": "Zenotp message events",
				"script": "/home/orly/.config/ORLY/validate4678.js",
				"privileged": true
			},
			"10306": {
				"description": "End user whitelist changes",
				"read_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
				],
				"privileged": true
			},
			"30520": {
				"description": "Zenotp events",
				"write_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
				],
				"privileged": true
			},
			"30919": {
				"description": "Zenotp events",
				"write_allow": [
					"04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
				],
				"privileged": true
			}
		}
	}`)

	// Create policy from JSON
	policy, err := New(filterJSON)
	if err != nil {
		t.Fatalf("Failed to create policy from JSON: %v", err)
	}

	// Verify whitelist is set correctly
	if len(policy.Kind.Whitelist) != 4 {
		t.Errorf("Expected whitelist to have 4 kinds, got %d", len(policy.Kind.Whitelist))
	}
	expectedKinds := map[int]bool{4678: true, 10306: true, 30520: true, 30919: true}
	for _, kind := range policy.Kind.Whitelist {
		if !expectedKinds[kind] {
			t.Errorf("Unexpected kind in whitelist: %d", kind)
		}
	}

	// Verify rules are loaded correctly
	if len(policy.rules) != 4 {
		t.Errorf("Expected 4 rules, got %d", len(policy.rules))
	}

	// Verify rule 4678 (script-based)
	rule4678, ok := policy.rules[4678]
	if !ok {
		t.Fatal("Rule 4678 not found")
	}
	if rule4678.Description != "Zenotp message events" {
		t.Errorf("Expected description 'Zenotp message events', got '%s'", rule4678.Description)
	}
	if rule4678.Script != "/home/orly/.config/ORLY/validate4678.js" {
		t.Errorf("Expected script path '/home/orly/.config/ORLY/validate4678.js', got '%s'", rule4678.Script)
	}
	if !rule4678.Privileged {
		t.Error("Expected rule 4678 to be privileged")
	}

	// Verify rule 10306 (read_allow)
	rule10306, ok := policy.rules[10306]
	if !ok {
		t.Fatal("Rule 10306 not found")
	}
	if rule10306.Description != "End user whitelist changes" {
		t.Errorf("Expected description 'End user whitelist changes', got '%s'", rule10306.Description)
	}
	if len(rule10306.ReadAllow) != 1 {
		t.Errorf("Expected 1 read_allow pubkey, got %d", len(rule10306.ReadAllow))
	}
	if rule10306.ReadAllow[0] != "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5" {
		t.Errorf("Expected read_allow pubkey '04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5', got '%s'", rule10306.ReadAllow[0])
	}
	if !rule10306.Privileged {
		t.Error("Expected rule 10306 to be privileged")
	}

	// Verify rule 30520 (write_allow)
	rule30520, ok := policy.rules[30520]
	if !ok {
		t.Fatal("Rule 30520 not found")
	}
	if rule30520.Description != "Zenotp events" {
		t.Errorf("Expected description 'Zenotp events', got '%s'", rule30520.Description)
	}
	if len(rule30520.WriteAllow) != 1 {
		t.Errorf("Expected 1 write_allow pubkey, got %d", len(rule30520.WriteAllow))
	}
	if rule30520.WriteAllow[0] != "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5" {
		t.Errorf("Expected write_allow pubkey '04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5', got '%s'", rule30520.WriteAllow[0])
	}
	if !rule30520.Privileged {
		t.Error("Expected rule 30520 to be privileged")
	}

	// Verify rule 30919 (write_allow)
	rule30919, ok := policy.rules[30919]
	if !ok {
		t.Fatal("Rule 30919 not found")
	}
	if rule30919.Description != "Zenotp events" {
		t.Errorf("Expected description 'Zenotp events', got '%s'", rule30919.Description)
	}
	if len(rule30919.WriteAllow) != 1 {
		t.Errorf("Expected 1 write_allow pubkey, got %d", len(rule30919.WriteAllow))
	}
	if rule30919.WriteAllow[0] != "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5" {
		t.Errorf("Expected write_allow pubkey '04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5', got '%s'", rule30919.WriteAllow[0])
	}
	if !rule30919.Privileged {
		t.Error("Expected rule 30919 to be privileged")
	}

	// Test kind whitelist filtering
	t.Run("kind whitelist filtering", func(t *testing.T) {
		eventSigner, eventPubkey := generateTestKeypair(t)
		_, loggedInPubkey := generateTestKeypair(t)

		// Test whitelisted kind (should pass kind check, but may fail other checks)
		whitelistedEvent := createTestEvent(t, eventSigner, "test content", 4678)
		addPTag(whitelistedEvent, loggedInPubkey)
		allowed, err := policy.CheckPolicy("write", whitelistedEvent, loggedInPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Should be allowed because script isn't running (falls back to default "allow")
		// and privileged check passes (loggedInPubkey is in p tag)
		if !allowed {
			t.Error("Expected whitelisted kind to be allowed when script not running and privileged check passes")
		}

		// Test non-whitelisted kind (should be denied)
		nonWhitelistedEvent := createTestEvent(t, eventSigner, "test content", 1)
		allowed, err = policy.CheckPolicy("write", nonWhitelistedEvent, eventPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected non-whitelisted kind to be denied")
		}
	})

	// Test read access for kind 10306
	t.Run("read access for kind 10306", func(t *testing.T) {
		allowedPubkeyHex := "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
		allowedPubkeyBytes, err := hex.Dec(allowedPubkeyHex)
		if err != nil {
			t.Fatalf("Failed to decode allowed pubkey: %v", err)
		}

		// Create event with allowed pubkey using a temporary signer
		// Note: We can't actually sign with just the pubkey, so we'll create the event
		// with a random signer and then manually set the pubkey for testing
		tempSigner, _ := generateTestKeypair(t)
		event10306 := createTestEvent(t, tempSigner, "test content", 10306)
		event10306.Pubkey = allowedPubkeyBytes
		// Re-sign won't work without private key, but policy checks use the pubkey field
		// So we'll test with the pubkey set correctly

		// Test read access with allowed pubkey (logged in user matches event pubkey)
		allowed, err := policy.CheckPolicy("read", event10306, allowedPubkeyBytes, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event to be allowed for read access with allowed pubkey")
		}

		// Test read access with non-allowed pubkey
		_, unauthorizedPubkey := generateTestKeypair(t)
		allowed, err = policy.CheckPolicy("read", event10306, unauthorizedPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied for read access with non-allowed pubkey")
		}

		// Test read access without authentication (privileged check)
		allowed, err = policy.CheckPolicy("read", event10306, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied for read access without authentication (privileged)")
		}
	})

	// Test write access for kind 30520
	t.Run("write access for kind 30520", func(t *testing.T) {
		allowedPubkeyHex := "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
		allowedPubkeyBytes, err := hex.Dec(allowedPubkeyHex)
		if err != nil {
			t.Fatalf("Failed to decode allowed pubkey: %v", err)
		}

		// Create event with allowed pubkey using a temporary signer
		// We'll set the pubkey manually for testing since we don't have the private key
		tempSigner, _ := generateTestKeypair(t)
		event30520 := createTestEvent(t, tempSigner, "test content", 30520)
		event30520.Pubkey = allowedPubkeyBytes

		// Test write access with allowed pubkey (event pubkey matches write_allow)
		allowed, err := policy.CheckPolicy("write", event30520, allowedPubkeyBytes, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event to be allowed for write access with allowed pubkey")
		}

		// Test write access with non-allowed pubkey
		unauthorizedSigner, unauthorizedPubkey := generateTestKeypair(t)
		unauthorizedEvent := createTestEvent(t, unauthorizedSigner, "test content", 30520)
		allowed, err = policy.CheckPolicy("write", unauthorizedEvent, unauthorizedPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied for write access with non-allowed pubkey")
		}

		// Test write access without authentication (privileged check)
		allowed, err = policy.CheckPolicy("write", event30520, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied for write access without authentication (privileged)")
		}
	})

	// Test write access for kind 30919
	t.Run("write access for kind 30919", func(t *testing.T) {
		allowedPubkeyHex := "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
		allowedPubkeyBytes, err := hex.Dec(allowedPubkeyHex)
		if err != nil {
			t.Fatalf("Failed to decode allowed pubkey: %v", err)
		}

		// Create event with allowed pubkey using a temporary signer
		// We'll set the pubkey manually for testing since we don't have the private key
		tempSigner, _ := generateTestKeypair(t)
		event30919 := createTestEvent(t, tempSigner, "test content", 30919)
		event30919.Pubkey = allowedPubkeyBytes

		// Test write access with allowed pubkey (event pubkey matches write_allow)
		allowed, err := policy.CheckPolicy("write", event30919, allowedPubkeyBytes, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !allowed {
			t.Error("Expected event to be allowed for write access with allowed pubkey")
		}

		// Test write access with non-allowed pubkey
		unauthorizedSigner, unauthorizedPubkey := generateTestKeypair(t)
		unauthorizedEvent := createTestEvent(t, unauthorizedSigner, "test content", 30919)
		allowed, err = policy.CheckPolicy("write", unauthorizedEvent, unauthorizedPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied for write access with non-allowed pubkey")
		}

		// Test write access without authentication (privileged check)
		allowed, err = policy.CheckPolicy("write", event30919, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied for write access without authentication (privileged)")
		}
	})

	// Test privileged flag behavior with p tags
	t.Run("privileged flag with p tags", func(t *testing.T) {
		eventSigner, _ := generateTestKeypair(t)
		_, loggedInPubkey := generateTestKeypair(t)
		allowedPubkeyHex := "04eeb1ed409c0b9205e722f8bf1780f553b61876ef323aff16c9f80a9d8ee9f5"
		allowedPubkeyBytes, err := hex.Dec(allowedPubkeyHex)
		if err != nil {
			t.Fatalf("Failed to decode allowed pubkey: %v", err)
		}

		// Create event with allowed pubkey and logged-in pubkey in p tag
		event30520 := createTestEvent(t, eventSigner, "test content", 30520)
		event30520.Pubkey = allowedPubkeyBytes
		addPTag(event30520, loggedInPubkey)

		// Test that event is DENIED when submitter (logged-in pubkey) is not in write_allow
		// Even though the submitter is in p-tag, write_allow is about who can submit
		allowed, err := policy.CheckPolicy("write", event30520, loggedInPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied when submitter (logged-in pubkey) is not in write_allow")
		}

		// Test that event is denied when submitter is not in write_allow (even without p-tag)
		event30520NoPTag := createTestEvent(t, eventSigner, "test content", 30520)
		event30520NoPTag.Pubkey = allowedPubkeyBytes
		allowed, err = policy.CheckPolicy("write", event30520NoPTag, loggedInPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if allowed {
			t.Error("Expected event to be denied when submitter is not in write_allow")
		}
	})

	// Test script-based rule (kind 4678)
	t.Run("script-based rule for kind 4678", func(t *testing.T) {
		eventSigner, _ := generateTestKeypair(t)
		_, loggedInPubkey := generateTestKeypair(t)

		event4678 := createTestEvent(t, eventSigner, "test content", 4678)
		addPTag(event4678, loggedInPubkey)

		// Test with script not running (should fall back to default policy)
		allowed, err := policy.CheckPolicy("write", event4678, loggedInPubkey, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Should allow because default policy is "allow" and script is not running
		// and privileged check passes (loggedInPubkey is in p tag)
		if !allowed {
			t.Error("Expected event to be allowed when script is not running (falls back to default 'allow') and privileged check passes")
		}

		// Test without authentication (privileged doesn't affect write operations)
		allowed, err = policy.CheckPolicy("write", event4678, nil, "127.0.0.1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		// Should be allowed because privileged doesn't affect write operations
		// Falls back to default policy which is "allow"
		if !allowed {
			t.Error("Expected event to be allowed without authentication (privileged doesn't affect write)")
		}
	})
}
