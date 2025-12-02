package policy

import (
	"encoding/json"
	"testing"
)

// TestValidateOwnerPolicyUpdate tests owner-specific validation
func TestValidateOwnerPolicyUpdate(t *testing.T) {
	// Create a base policy
	basePolicy := &P{
		DefaultPolicy: "allow",
		Owners:        []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		PolicyAdmins:  []string{"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}

	tests := []struct {
		name        string
		newPolicy   string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid owner update with non-empty owners",
			newPolicy: `{
				"default_policy": "deny",
				"owners": ["cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"],
				"policy_admins": ["dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"]
			}`,
			expectError: false,
		},
		{
			name: "invalid - empty owners list",
			newPolicy: `{
				"default_policy": "deny",
				"owners": [],
				"policy_admins": ["dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"]
			}`,
			expectError: true,
			errorMsg:    "owners list cannot be empty",
		},
		{
			name: "invalid - missing owners field",
			newPolicy: `{
				"default_policy": "deny",
				"policy_admins": ["dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"]
			}`,
			expectError: true,
			errorMsg:    "owners list cannot be empty",
		},
		{
			name: "invalid - bad owner pubkey format",
			newPolicy: `{
				"default_policy": "deny",
				"owners": ["not-a-valid-pubkey"]
			}`,
			expectError: true,
			errorMsg:    "invalid owner pubkey",
		},
		{
			name: "valid - owner can add multiple owners",
			newPolicy: `{
				"default_policy": "deny",
				"owners": [
					"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
					"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
				]
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := basePolicy.ValidateOwnerPolicyUpdate([]byte(tt.newPolicy))
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestValidatePolicyAdminUpdate tests policy admin validation
func TestValidatePolicyAdminUpdate(t *testing.T) {
	// Create a base policy with known owners and admins
	ownerPubkey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	adminPubkey := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	allowedPubkey := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	baseJSON := `{
		"default_policy": "allow",
		"owners": ["` + ownerPubkey + `"],
		"policy_admins": ["` + adminPubkey + `"],
		"kind": {
			"whitelist": [1, 3, 7]
		},
		"rules": {
			"1": {
				"description": "Text notes",
				"write_allow": ["` + allowedPubkey + `"],
				"size_limit": 10000
			}
		}
	}`

	basePolicy := &P{}
	if err := json.Unmarshal([]byte(baseJSON), basePolicy); err != nil {
		t.Fatalf("failed to create base policy: %v", err)
	}

	adminPubkeyBin := make([]byte, 32)
	for i := range adminPubkeyBin {
		adminPubkeyBin[i] = 0xbb
	}

	tests := []struct {
		name        string
		newPolicy   string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid - policy admin can extend write_allow",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `", "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "valid - policy admin can add to kind whitelist",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7, 30023]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "valid - policy admin can increase size limit",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"size_limit": 20000
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid - policy admin cannot modify owners",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot modify the 'owners' field",
		},
		{
			name: "invalid - policy admin cannot modify policy_admins",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot modify the 'policy_admins' field",
		},
		{
			name: "invalid - policy admin cannot remove from kind whitelist",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot remove kind 7 from whitelist",
		},
		{
			name: "invalid - policy admin cannot remove from write_allow",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": [],
						"size_limit": 10000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot remove pubkey",
		},
		{
			name: "invalid - policy admin cannot reduce size limit",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"size_limit": 5000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot reduce size_limit",
		},
		{
			name: "invalid - policy admin cannot remove rule",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {}
			}`,
			expectError: true,
			errorMsg:    "cannot remove rule for kind 1",
		},
		{
			name: "valid - policy admin can add blacklist entries for non-admin users",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7],
					"blacklist": [4]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"write_deny": ["eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: false,
		},
		{
			name: "invalid - policy admin cannot blacklist owner in write_deny",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"write_deny": ["` + ownerPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot blacklist owner",
		},
		{
			name: "invalid - policy admin cannot blacklist other policy admin",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"write_deny": ["` + adminPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: true,
			errorMsg:    "cannot blacklist policy admin",
		},
		{
			name: "valid - policy admin can blacklist whitelisted non-admin user",
			newPolicy: `{
				"default_policy": "allow",
				"owners": ["` + ownerPubkey + `"],
				"policy_admins": ["` + adminPubkey + `"],
				"kind": {
					"whitelist": [1, 3, 7]
				},
				"rules": {
					"1": {
						"description": "Text notes",
						"write_allow": ["` + allowedPubkey + `"],
						"write_deny": ["` + allowedPubkey + `"],
						"size_limit": 10000
					}
				}
			}`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := basePolicy.ValidatePolicyAdminUpdate([]byte(tt.newPolicy), adminPubkeyBin)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestIsOwner tests the IsOwner method
func TestIsOwner(t *testing.T) {
	ownerPubkey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	nonOwnerPubkey := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	_ = nonOwnerPubkey // Silence unused variable warning

	policyJSON := `{
		"default_policy": "allow",
		"owners": ["` + ownerPubkey + `"]
	}`

	policy, err := New([]byte(policyJSON))
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	// Create binary pubkeys
	ownerBin := make([]byte, 32)
	for i := range ownerBin {
		ownerBin[i] = 0xaa
	}

	nonOwnerBin := make([]byte, 32)
	for i := range nonOwnerBin {
		nonOwnerBin[i] = 0xbb
	}

	tests := []struct {
		name     string
		pubkey   []byte
		expected bool
	}{
		{
			name:     "owner is recognized",
			pubkey:   ownerBin,
			expected: true,
		},
		{
			name:     "non-owner is not recognized",
			pubkey:   nonOwnerBin,
			expected: false,
		},
		{
			name:     "nil pubkey returns false",
			pubkey:   nil,
			expected: false,
		},
		{
			name:     "empty pubkey returns false",
			pubkey:   []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.IsOwner(tt.pubkey)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestStringSliceEqual tests the helper function
func TestStringSliceEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []string
		b        []string
		expected bool
	}{
		{
			name:     "equal slices same order",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "equal slices different order",
			a:        []string{"a", "b", "c"},
			b:        []string{"c", "a", "b"},
			expected: true,
		},
		{
			name:     "different lengths",
			a:        []string{"a", "b"},
			b:        []string{"a", "b", "c"},
			expected: false,
		},
		{
			name:     "different contents",
			a:        []string{"a", "b", "c"},
			b:        []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "empty slices",
			a:        []string{},
			b:        []string{},
			expected: true,
		},
		{
			name:     "nil slices",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "nil vs empty",
			a:        nil,
			b:        []string{},
			expected: true,
		},
		{
			name:     "duplicates in both",
			a:        []string{"a", "a", "b"},
			b:        []string{"a", "b", "a"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringSliceEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestPolicyAdminContributionValidation tests the contribution validation
func TestPolicyAdminContributionValidation(t *testing.T) {
	ownerPubkey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	adminPubkey := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	ownerPolicy := &P{
		DefaultPolicy: "allow",
		Owners:        []string{ownerPubkey},
		PolicyAdmins:  []string{adminPubkey},
		Kind: Kinds{
			Whitelist: []int{1, 3, 7},
		},
		rules: map[int]Rule{
			1: {
				Description: "Text notes",
				SizeLimit:   ptr(int64(10000)),
			},
		},
	}

	tests := []struct {
		name         string
		contribution *PolicyAdminContribution
		expectError  bool
		errorMsg     string
	}{
		{
			name: "valid - add kinds to whitelist",
			contribution: &PolicyAdminContribution{
				AdminPubkey:      adminPubkey,
				CreatedAt:        1234567890,
				EventID:          "event123",
				KindWhitelistAdd: []int{30023},
			},
			expectError: false,
		},
		{
			name: "valid - add to blacklist",
			contribution: &PolicyAdminContribution{
				AdminPubkey:      adminPubkey,
				CreatedAt:        1234567890,
				EventID:          "event123",
				KindBlacklistAdd: []int{4},
			},
			expectError: false,
		},
		{
			name: "valid - extend existing rule with larger limit",
			contribution: &PolicyAdminContribution{
				AdminPubkey: adminPubkey,
				CreatedAt:   1234567890,
				EventID:     "event123",
				RulesExtend: map[int]RuleExtension{
					1: {
						SizeLimitOverride: ptr(int64(20000)),
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid - extend non-existent rule",
			contribution: &PolicyAdminContribution{
				AdminPubkey: adminPubkey,
				CreatedAt:   1234567890,
				EventID:     "event123",
				RulesExtend: map[int]RuleExtension{
					999: {
						SizeLimitOverride: ptr(int64(20000)),
					},
				},
			},
			expectError: true,
			errorMsg:    "cannot extend rule for kind 999",
		},
		{
			name: "invalid - size limit override smaller than owner's",
			contribution: &PolicyAdminContribution{
				AdminPubkey: adminPubkey,
				CreatedAt:   1234567890,
				EventID:     "event123",
				RulesExtend: map[int]RuleExtension{
					1: {
						SizeLimitOverride: ptr(int64(5000)),
					},
				},
			},
			expectError: true,
			errorMsg:    "size_limit_override for kind 1 must be >=",
		},
		{
			name: "valid - add new rule for undefined kind",
			contribution: &PolicyAdminContribution{
				AdminPubkey: adminPubkey,
				CreatedAt:   1234567890,
				EventID:     "event123",
				RulesAdd: map[int]Rule{
					30023: {
						Description: "Long-form content",
						SizeLimit:   ptr(int64(100000)),
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid - add rule for already-defined kind",
			contribution: &PolicyAdminContribution{
				AdminPubkey: adminPubkey,
				CreatedAt:   1234567890,
				EventID:     "event123",
				RulesAdd: map[int]Rule{
					1: {
						Description: "Trying to override",
					},
				},
			},
			expectError: true,
			errorMsg:    "cannot add rule for kind 1: already defined",
		},
		{
			name: "invalid - bad pubkey length in extension",
			contribution: &PolicyAdminContribution{
				AdminPubkey: "short",
				CreatedAt:   1234567890,
				EventID:     "event123",
			},
			expectError: true,
			errorMsg:    "invalid admin pubkey length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicyAdminContribution(ownerPolicy, tt.contribution, nil)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !containsSubstring(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// Helper function for generic pointer
func ptr[T any](v T) *T {
	return &v
}
