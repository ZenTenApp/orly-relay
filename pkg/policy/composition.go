package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"git.mleku.dev/mleku/nostr/encoders/hex"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/utils"
)

// =============================================================================
// Policy Composition Types
// =============================================================================

// PolicyAdminContribution represents extensions/additions from a policy admin.
// Policy admins can extend the base owner policy but cannot modify protected fields
// (owners, policy_admins) or reduce owner-granted permissions.
type PolicyAdminContribution struct {
	// AdminPubkey is the hex-encoded pubkey of the policy admin who made this contribution
	AdminPubkey string `json:"admin_pubkey"`
	// CreatedAt is the Unix timestamp when this contribution was created
	CreatedAt int64 `json:"created_at"`
	// EventID is the Nostr event ID that created this contribution (for audit trail)
	EventID string `json:"event_id,omitempty"`

	// KindWhitelistAdd adds kinds to the whitelist (OR with owner's whitelist)
	KindWhitelistAdd []int `json:"kind_whitelist_add,omitempty"`
	// KindBlacklistAdd adds kinds to the blacklist (overrides whitelist)
	KindBlacklistAdd []int `json:"kind_blacklist_add,omitempty"`

	// RulesExtend extends existing rules defined by the owner
	RulesExtend map[int]RuleExtension `json:"rules_extend,omitempty"`
	// RulesAdd adds new rules for kinds not defined by the owner
	RulesAdd map[int]Rule `json:"rules_add,omitempty"`

	// GlobalExtend extends the global rule
	GlobalExtend *RuleExtension `json:"global_extend,omitempty"`
}

// RuleExtension defines how a policy admin can extend an existing owner rule.
// All fields are additive - they extend, not replace, the owner's configuration.
type RuleExtension struct {
	// WriteAllowAdd adds pubkeys to the write allow list
	WriteAllowAdd []string `json:"write_allow_add,omitempty"`
	// WriteDenyAdd adds pubkeys to the write deny list (overrides allow)
	WriteDenyAdd []string `json:"write_deny_add,omitempty"`
	// ReadAllowAdd adds pubkeys to the read allow list
	ReadAllowAdd []string `json:"read_allow_add,omitempty"`
	// ReadDenyAdd adds pubkeys to the read deny list (overrides allow)
	ReadDenyAdd []string `json:"read_deny_add,omitempty"`

	// SizeLimitOverride can only make the limit MORE permissive (larger)
	SizeLimitOverride *int64 `json:"size_limit_override,omitempty"`
	// ContentLimitOverride can only make the limit MORE permissive (larger)
	ContentLimitOverride *int64 `json:"content_limit_override,omitempty"`
	// MaxAgeOfEventOverride can only make the limit MORE permissive (older allowed)
	MaxAgeOfEventOverride *int64 `json:"max_age_of_event_override,omitempty"`
	// MaxAgeEventInFutureOverride can only make the limit MORE permissive (further future allowed)
	MaxAgeEventInFutureOverride *int64 `json:"max_age_event_in_future_override,omitempty"`

	// WriteAllowFollows extends the follow whitelist feature
	WriteAllowFollows *bool `json:"write_allow_follows,omitempty"`
	// FollowsWhitelistAdminsAdd adds admin pubkeys whose follows are whitelisted
	FollowsWhitelistAdminsAdd []string `json:"follows_whitelist_admins_add,omitempty"`
}

// ComposedPolicy manages the base owner policy and policy admin contributions.
// It computes an effective merged policy at runtime.
type ComposedPolicy struct {
	// OwnerPolicy is the base policy set by owners
	OwnerPolicy *P
	// Contributions is a map of event ID -> contribution for deduplication
	Contributions map[string]*PolicyAdminContribution
	// contributionsMx protects the contributions map
	contributionsMx sync.RWMutex
	// configDir is the directory where policy files are stored
	configDir string
}

// =============================================================================
// Protected Field Validation
// =============================================================================

// ProtectedFields are fields that only owners can modify
var ProtectedFields = []string{"owners", "policy_admins"}

// ValidateOwnerPolicy validates a policy update from an owner.
// Ensures owners list is non-empty.
func ValidateOwnerPolicy(policy *P) error {
	if len(policy.Owners) == 0 {
		return fmt.Errorf("owners list cannot be empty: at least one owner must be defined")
	}

	// Validate all owner pubkeys are valid hex
	for _, owner := range policy.Owners {
		if len(owner) != 64 {
			return fmt.Errorf("invalid owner pubkey length: %q (expected 64 hex characters)", owner)
		}
		if _, err := hex.Dec(owner); err != nil {
			return fmt.Errorf("invalid owner pubkey format: %q: %v", owner, err)
		}
	}

	// Validate all policy admin pubkeys are valid hex
	for _, admin := range policy.PolicyAdmins {
		if len(admin) != 64 {
			return fmt.Errorf("invalid policy_admin pubkey length: %q (expected 64 hex characters)", admin)
		}
		if _, err := hex.Dec(admin); err != nil {
			return fmt.Errorf("invalid policy_admin pubkey format: %q: %v", admin, err)
		}
	}

	return nil
}

// ValidatePolicyAdminContribution validates a contribution from a policy admin.
// Ensures no protected fields are modified and extensions are valid.
func ValidatePolicyAdminContribution(
	ownerPolicy *P,
	contribution *PolicyAdminContribution,
	existingContributions map[string]*PolicyAdminContribution,
) error {
	// Validate the admin pubkey is valid
	if len(contribution.AdminPubkey) != 64 {
		return fmt.Errorf("invalid admin pubkey length")
	}

	// Validate kind additions don't conflict with owner blacklist
	// (though PA can add to blacklist to override whitelist)

	// Validate rule extensions
	for kind, ext := range contribution.RulesExtend {
		ownerRule, exists := ownerPolicy.rules[kind]
		if !exists {
			return fmt.Errorf("cannot extend rule for kind %d: not defined in owner policy (use rules_add instead)", kind)
		}

		// Validate size limit overrides are more permissive
		if ext.SizeLimitOverride != nil && ownerRule.SizeLimit != nil {
			if *ext.SizeLimitOverride < *ownerRule.SizeLimit {
				return fmt.Errorf("size_limit_override for kind %d must be >= owner's limit (%d)", kind, *ownerRule.SizeLimit)
			}
		}

		if ext.ContentLimitOverride != nil && ownerRule.ContentLimit != nil {
			if *ext.ContentLimitOverride < *ownerRule.ContentLimit {
				return fmt.Errorf("content_limit_override for kind %d must be >= owner's limit (%d)", kind, *ownerRule.ContentLimit)
			}
		}

		if ext.MaxAgeOfEventOverride != nil && ownerRule.MaxAgeOfEvent != nil {
			if *ext.MaxAgeOfEventOverride < *ownerRule.MaxAgeOfEvent {
				return fmt.Errorf("max_age_of_event_override for kind %d must be >= owner's limit (%d)", kind, *ownerRule.MaxAgeOfEvent)
			}
		}

		// Validate pubkey formats in allow/deny lists
		for _, pk := range ext.WriteAllowAdd {
			if len(pk) != 64 {
				return fmt.Errorf("invalid pubkey in write_allow_add for kind %d: %q", kind, pk)
			}
		}
		for _, pk := range ext.WriteDenyAdd {
			if len(pk) != 64 {
				return fmt.Errorf("invalid pubkey in write_deny_add for kind %d: %q", kind, pk)
			}
		}
		for _, pk := range ext.ReadAllowAdd {
			if len(pk) != 64 {
				return fmt.Errorf("invalid pubkey in read_allow_add for kind %d: %q", kind, pk)
			}
		}
		for _, pk := range ext.ReadDenyAdd {
			if len(pk) != 64 {
				return fmt.Errorf("invalid pubkey in read_deny_add for kind %d: %q", kind, pk)
			}
		}
	}

	// Validate rules_add are for kinds not already defined by owner
	for kind := range contribution.RulesAdd {
		if _, exists := ownerPolicy.rules[kind]; exists {
			return fmt.Errorf("cannot add rule for kind %d: already defined in owner policy (use rules_extend instead)", kind)
		}
	}

	return nil
}

// =============================================================================
// Policy Composition Logic
// =============================================================================

// NewComposedPolicy creates a new composed policy from an owner policy.
func NewComposedPolicy(ownerPolicy *P, configDir string) *ComposedPolicy {
	return &ComposedPolicy{
		OwnerPolicy:   ownerPolicy,
		Contributions: make(map[string]*PolicyAdminContribution),
		configDir:     configDir,
	}
}

// AddContribution adds a policy admin contribution.
// Returns error if validation fails.
func (cp *ComposedPolicy) AddContribution(contribution *PolicyAdminContribution) error {
	cp.contributionsMx.Lock()
	defer cp.contributionsMx.Unlock()

	// Validate the contribution
	if err := ValidatePolicyAdminContribution(cp.OwnerPolicy, contribution, cp.Contributions); err != nil {
		return err
	}

	// Store the contribution
	cp.Contributions[contribution.EventID] = contribution

	// Persist to disk
	if err := cp.saveContribution(contribution); err != nil {
		log.W.F("failed to persist contribution: %v", err)
	}

	return nil
}

// RemoveContribution removes a policy admin contribution by event ID.
func (cp *ComposedPolicy) RemoveContribution(eventID string) {
	cp.contributionsMx.Lock()
	defer cp.contributionsMx.Unlock()

	delete(cp.Contributions, eventID)

	// Remove from disk
	if cp.configDir != "" {
		contribPath := filepath.Join(cp.configDir, "policy-contributions", eventID+".json")
		os.Remove(contribPath)
	}
}

// GetEffectivePolicy computes the merged effective policy.
// Composition rules:
// - Whitelists are unioned (OR)
// - Blacklists are unioned and override whitelists
// - Limits use the most permissive value
// - Conflicts between PAs: oldest created_at wins (except deny always wins)
func (cp *ComposedPolicy) GetEffectivePolicy() *P {
	cp.contributionsMx.RLock()
	defer cp.contributionsMx.RUnlock()

	// Clone the owner policy as base
	effective := cp.cloneOwnerPolicy()

	// Sort contributions by created_at (oldest first for conflict resolution)
	sorted := cp.getSortedContributions()

	// Apply each contribution
	for _, contrib := range sorted {
		cp.applyContribution(effective, contrib)
	}

	// Repopulate binary caches
	effective.Global.populateBinaryCache()
	for kind := range effective.rules {
		rule := effective.rules[kind]
		rule.populateBinaryCache()
		effective.rules[kind] = rule
	}

	return effective
}

// cloneOwnerPolicy creates a deep copy of the owner policy.
func (cp *ComposedPolicy) cloneOwnerPolicy() *P {
	// Marshal and unmarshal to create a deep copy
	data, _ := json.Marshal(cp.OwnerPolicy)
	var cloned P
	json.Unmarshal(data, &cloned)

	// Copy the manager reference (not cloned)
	cloned.manager = cp.OwnerPolicy.manager

	return &cloned
}

// getSortedContributions returns contributions sorted by created_at.
func (cp *ComposedPolicy) getSortedContributions() []*PolicyAdminContribution {
	sorted := make([]*PolicyAdminContribution, 0, len(cp.Contributions))
	for _, contrib := range cp.Contributions {
		sorted = append(sorted, contrib)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt < sorted[j].CreatedAt
	})
	return sorted
}

// applyContribution applies a single contribution to the effective policy.
func (cp *ComposedPolicy) applyContribution(effective *P, contrib *PolicyAdminContribution) {
	// Apply kind whitelist additions (OR)
	for _, kind := range contrib.KindWhitelistAdd {
		if !containsInt(effective.Kind.Whitelist, kind) {
			effective.Kind.Whitelist = append(effective.Kind.Whitelist, kind)
		}
	}

	// Apply kind blacklist additions (OR, overrides whitelist)
	for _, kind := range contrib.KindBlacklistAdd {
		if !containsInt(effective.Kind.Blacklist, kind) {
			effective.Kind.Blacklist = append(effective.Kind.Blacklist, kind)
		}
	}

	// Apply rule extensions
	for kind, ext := range contrib.RulesExtend {
		if rule, exists := effective.rules[kind]; exists {
			cp.applyRuleExtension(&rule, &ext, contrib.CreatedAt)
			effective.rules[kind] = rule
		}
	}

	// Apply new rules
	for kind, rule := range contrib.RulesAdd {
		if _, exists := effective.rules[kind]; !exists {
			if effective.rules == nil {
				effective.rules = make(map[int]Rule)
			}
			effective.rules[kind] = rule
		}
	}

	// Apply global rule extension
	if contrib.GlobalExtend != nil {
		cp.applyRuleExtension(&effective.Global, contrib.GlobalExtend, contrib.CreatedAt)
	}
}

// applyRuleExtension applies a rule extension to an existing rule.
func (cp *ComposedPolicy) applyRuleExtension(rule *Rule, ext *RuleExtension, _ int64) {
	// Add to allow lists (OR)
	for _, pk := range ext.WriteAllowAdd {
		if !containsString(rule.WriteAllow, pk) {
			rule.WriteAllow = append(rule.WriteAllow, pk)
		}
	}
	for _, pk := range ext.ReadAllowAdd {
		if !containsString(rule.ReadAllow, pk) {
			rule.ReadAllow = append(rule.ReadAllow, pk)
		}
	}

	// Add to deny lists (OR, overrides allow) - deny always wins
	for _, pk := range ext.WriteDenyAdd {
		if !containsString(rule.WriteDeny, pk) {
			rule.WriteDeny = append(rule.WriteDeny, pk)
		}
	}
	for _, pk := range ext.ReadDenyAdd {
		if !containsString(rule.ReadDeny, pk) {
			rule.ReadDeny = append(rule.ReadDeny, pk)
		}
	}

	// Apply limit overrides (most permissive wins)
	if ext.SizeLimitOverride != nil {
		if rule.SizeLimit == nil || *ext.SizeLimitOverride > *rule.SizeLimit {
			rule.SizeLimit = ext.SizeLimitOverride
		}
	}
	if ext.ContentLimitOverride != nil {
		if rule.ContentLimit == nil || *ext.ContentLimitOverride > *rule.ContentLimit {
			rule.ContentLimit = ext.ContentLimitOverride
		}
	}
	if ext.MaxAgeOfEventOverride != nil {
		if rule.MaxAgeOfEvent == nil || *ext.MaxAgeOfEventOverride > *rule.MaxAgeOfEvent {
			rule.MaxAgeOfEvent = ext.MaxAgeOfEventOverride
		}
	}
	if ext.MaxAgeEventInFutureOverride != nil {
		if rule.MaxAgeEventInFuture == nil || *ext.MaxAgeEventInFutureOverride > *rule.MaxAgeEventInFuture {
			rule.MaxAgeEventInFuture = ext.MaxAgeEventInFutureOverride
		}
	}

	// Enable WriteAllowFollows if requested (OR logic)
	if ext.WriteAllowFollows != nil && *ext.WriteAllowFollows {
		rule.WriteAllowFollows = true
	}

	// Add to follows whitelist admins
	for _, pk := range ext.FollowsWhitelistAdminsAdd {
		if !containsString(rule.FollowsWhitelistAdmins, pk) {
			rule.FollowsWhitelistAdmins = append(rule.FollowsWhitelistAdmins, pk)
		}
	}
}

// =============================================================================
// Persistence
// =============================================================================

// saveContribution persists a contribution to disk.
func (cp *ComposedPolicy) saveContribution(contrib *PolicyAdminContribution) error {
	if cp.configDir == "" {
		return nil
	}

	contribDir := filepath.Join(cp.configDir, "policy-contributions")
	if err := os.MkdirAll(contribDir, 0755); err != nil {
		return err
	}

	contribPath := filepath.Join(contribDir, contrib.EventID+".json")
	data, err := json.MarshalIndent(contrib, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(contribPath, data, 0644)
}

// LoadContributions loads all contributions from disk.
func (cp *ComposedPolicy) LoadContributions() error {
	if cp.configDir == "" {
		return nil
	}

	contribDir := filepath.Join(cp.configDir, "policy-contributions")
	if _, err := os.Stat(contribDir); os.IsNotExist(err) {
		return nil // No contributions yet
	}

	entries, err := os.ReadDir(contribDir)
	if err != nil {
		return err
	}

	cp.contributionsMx.Lock()
	defer cp.contributionsMx.Unlock()

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		contribPath := filepath.Join(contribDir, entry.Name())
		data, err := os.ReadFile(contribPath)
		if err != nil {
			log.W.F("failed to read contribution %s: %v", entry.Name(), err)
			continue
		}

		var contrib PolicyAdminContribution
		if err := json.Unmarshal(data, &contrib); err != nil {
			log.W.F("failed to parse contribution %s: %v", entry.Name(), err)
			continue
		}

		// Validate against current owner policy
		if err := ValidatePolicyAdminContribution(cp.OwnerPolicy, &contrib, cp.Contributions); err != nil {
			log.W.F("contribution %s is no longer valid: %v (skipping)", entry.Name(), err)
			continue
		}

		cp.Contributions[contrib.EventID] = &contrib
	}

	log.I.F("loaded %d policy admin contributions", len(cp.Contributions))
	return nil
}

// =============================================================================
// Owner Detection
// =============================================================================

// IsOwner checks if the given pubkey is an owner.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
func (p *P) IsOwner(pubkey []byte) bool {
	if len(pubkey) == 0 {
		return false
	}

	p.policyFollowsMx.RLock()
	defer p.policyFollowsMx.RUnlock()

	for _, owner := range p.ownersBin {
		if utils.FastEqual(owner, pubkey) {
			return true
		}
	}
	return false
}

// IsOwnerOrPolicyAdmin checks if the given pubkey is an owner or policy admin.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
func (p *P) IsOwnerOrPolicyAdmin(pubkey []byte) bool {
	return p.IsOwner(pubkey) || p.IsPolicyAdmin(pubkey)
}

// =============================================================================
// Helper Functions
// =============================================================================

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

func containsString(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
