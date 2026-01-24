package policy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"github.com/adrg/xdg"
	"github.com/sosodev/duration"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/utils"
)

// parseDuration parses an ISO-8601 duration string into seconds.
// ISO-8601 format: P[n]Y[n]M[n]DT[n]H[n]M[n]S
// Examples: "P1D" (1 day), "PT1H" (1 hour), "P7DT12H" (7 days 12 hours), "PT30M" (30 minutes)
// Uses the github.com/sosodev/duration library for strict ISO-8601 compliance.
// Note: Years and Months are converted to approximate time.Duration values
// (1 year ≈ 365.25 days, 1 month ≈ 30.44 days).
func parseDuration(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Parse using the ISO-8601 duration library
	d, err := duration.Parse(s)
	if err != nil {
		return 0, fmt.Errorf("invalid ISO-8601 duration %q: %v", s, err)
	}

	// Convert to time.Duration and then to seconds
	timeDur := d.ToTimeDuration()
	return int64(timeDur.Seconds()), nil
}

// Kinds defines whitelist and blacklist policies for event kinds.
// Whitelist takes precedence over blacklist - if whitelist is present, only whitelisted kinds are allowed.
// If only blacklist is present, all kinds except blacklisted ones are allowed.
type Kinds struct {
	// Whitelist is a list of event kinds that are allowed to be written to the relay. If any are present, implicitly all others are denied.
	Whitelist []int `json:"whitelist,omitempty"`
	// Blacklist is a list of event kinds that are not allowed to be written to the relay. If any are present, implicitly all others are allowed. Only takes effect in the absence of a Whitelist.
	Blacklist []int `json:"blacklist,omitempty"`
}

// Rule defines policy criteria for a specific event kind.
//
// Rules are evaluated in the following order:
// 1. If Script is present and running, it determines the outcome
// 2. If Script fails or is not running, falls back to default_policy
// 3. Otherwise, all specified criteria are evaluated as AND operations
//
// For pubkey allow/deny lists: whitelist takes precedence over blacklist.
// If whitelist has entries, only whitelisted pubkeys are allowed.
// If only blacklist has entries, all pubkeys except blacklisted ones are allowed.
// =============================================================================
// Rule Sub-Components (Value Objects)
// =============================================================================

// AccessControl defines who can read/write events.
// This is a value object that encapsulates access control configuration.
type AccessControl struct {
	// WriteAllow is a list of pubkeys allowed to write. If any present, all others denied.
	WriteAllow []string `json:"write_allow,omitempty"`
	// WriteDeny is a list of pubkeys denied write. Only effective without WriteAllow.
	WriteDeny []string `json:"write_deny,omitempty"`
	// ReadAllow is a list of pubkeys allowed to read. If any present, all others denied.
	ReadAllow []string `json:"read_allow,omitempty"`
	// ReadDeny is a list of pubkeys denied read. Only effective without ReadAllow.
	ReadDeny []string `json:"read_deny,omitempty"`
	// WriteAllowFollows grants access to policy admin follows when enabled.
	WriteAllowFollows bool `json:"write_allow_follows,omitempty"`
	// FollowsWhitelistAdmins specifies admin pubkeys whose follows are whitelisted.
	// DEPRECATED: Use ReadFollowsWhitelist and WriteFollowsWhitelist instead.
	FollowsWhitelistAdmins []string `json:"follows_whitelist_admins,omitempty"`
	// ReadFollowsWhitelist specifies pubkeys whose follows can READ events.
	ReadFollowsWhitelist []string `json:"read_follows_whitelist,omitempty"`
	// WriteFollowsWhitelist specifies pubkeys whose follows can WRITE events.
	WriteFollowsWhitelist []string `json:"write_follows_whitelist,omitempty"`
	// ReadAllowPermissive allows read access for ALL kinds on GLOBAL rule.
	ReadAllowPermissive bool `json:"read_allow_permissive,omitempty"`
	// WriteAllowPermissive allows write access bypassing kind whitelist on GLOBAL rule.
	WriteAllowPermissive bool `json:"write_allow_permissive,omitempty"`

	// Binary caches (internal, not serialized)
	writeAllowBin              [][]byte
	writeDenyBin               [][]byte
	readAllowBin               [][]byte
	readDenyBin                [][]byte
	followsWhitelistAdminsBin  [][]byte
	followsWhitelistFollowsBin [][]byte
	readFollowsWhitelistBin    [][]byte
	writeFollowsWhitelistBin   [][]byte
	readFollowsFollowsBin      [][]byte
	writeFollowsFollowsBin     [][]byte
}

// Constraints defines limits and restrictions on events.
// This is a value object that encapsulates event constraints.
type Constraints struct {
	// MaxExpiry is the maximum expiry time in seconds.
	// Deprecated: Use MaxExpiryDuration instead.
	MaxExpiry *int64 `json:"max_expiry,omitempty"` //nolint:staticcheck
	// MaxExpiryDuration is the max expiry in ISO-8601 duration format.
	MaxExpiryDuration string `json:"max_expiry_duration,omitempty"`
	// SizeLimit is the maximum total serialized size in bytes.
	SizeLimit *int64 `json:"size_limit,omitempty"`
	// ContentLimit is the maximum content field size in bytes.
	ContentLimit *int64 `json:"content_limit,omitempty"`
	// RateLimit is the write rate limit in bytes per second.
	RateLimit *int64 `json:"rate_limit,omitempty"`
	// MaxAgeOfEvent is the max age in seconds for created_at timestamps.
	MaxAgeOfEvent *int64 `json:"max_age_of_event,omitempty"`
	// MaxAgeEventInFuture is the max future offset for created_at timestamps.
	MaxAgeEventInFuture *int64 `json:"max_age_event_in_future,omitempty"`
	// ProtectedRequired requires events to have a "-" tag (NIP-70).
	ProtectedRequired bool `json:"protected_required,omitempty"`
	// Privileged means event is only sent to authenticated parties.
	Privileged bool `json:"privileged,omitempty"`

	// Parsed cache (internal, not serialized)
	maxExpirySeconds *int64
}

// TagValidationConfig defines tag validation rules.
// This is a value object that encapsulates tag validation configuration.
type TagValidationConfig struct {
	// MustHaveTags is a list of tag key letters that must be present.
	MustHaveTags []string `json:"must_have_tags,omitempty"`
	// TagValidation is a map of tag_name -> regex pattern for validation.
	TagValidation map[string]string `json:"tag_validation,omitempty"`
	// IdentifierRegex is a regex pattern for "d" tag identifiers.
	IdentifierRegex string `json:"identifier_regex,omitempty"`

	// Compiled cache (internal, not serialized)
	identifierRegexCache *regexp.Regexp
}

// =============================================================================
// Rule (Composed from Sub-Components)
// =============================================================================

// Rule defines policies for a specific event kind or as a global default.
// It is composed of sub-value objects for cleaner organization.
type Rule struct {
	// Description is a human-readable description of the rule.
	Description string `json:"description"`
	// Script is a path to a validation script.
	Script string `json:"script,omitempty"`

	// Embedded sub-components (fields are flattened in JSON for backward compatibility)
	AccessControl
	Constraints
	TagValidationConfig
}

// hasAnyRules checks if the rule has any constraints configured
func (r *Rule) hasAnyRules() bool {
	// Check for any configured constraints
	return len(r.WriteAllow) > 0 || len(r.WriteDeny) > 0 ||
		len(r.ReadAllow) > 0 || len(r.ReadDeny) > 0 ||
		len(r.writeAllowBin) > 0 || len(r.writeDenyBin) > 0 ||
		len(r.readAllowBin) > 0 || len(r.readDenyBin) > 0 ||
		r.SizeLimit != nil || r.ContentLimit != nil ||
		r.MaxAgeOfEvent != nil || r.MaxAgeEventInFuture != nil ||
		r.MaxExpiry != nil || r.MaxExpiryDuration != "" || r.maxExpirySeconds != nil || //nolint:staticcheck // Backward compat
		len(r.MustHaveTags) > 0 ||
		r.Script != "" || r.Privileged ||
		r.WriteAllowFollows || len(r.FollowsWhitelistAdmins) > 0 ||
		len(r.ReadFollowsWhitelist) > 0 || len(r.WriteFollowsWhitelist) > 0 ||
		len(r.readFollowsWhitelistBin) > 0 || len(r.writeFollowsWhitelistBin) > 0 ||
		len(r.TagValidation) > 0 ||
		r.ProtectedRequired || r.IdentifierRegex != "" ||
		r.ReadAllowPermissive || r.WriteAllowPermissive
}

// populateBinaryCache converts hex-encoded pubkey strings to binary for faster comparison.
// This should be called after unmarshaling the policy from JSON.
func (r *Rule) populateBinaryCache() error {
	var err error

	// Convert WriteAllow hex strings to binary
	if len(r.WriteAllow) > 0 {
		r.writeAllowBin = make([][]byte, 0, len(r.WriteAllow))
		for _, hexPubkey := range r.WriteAllow {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode WriteAllow pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.writeAllowBin = append(r.writeAllowBin, binPubkey)
		}
	}

	// Convert WriteDeny hex strings to binary
	if len(r.WriteDeny) > 0 {
		r.writeDenyBin = make([][]byte, 0, len(r.WriteDeny))
		for _, hexPubkey := range r.WriteDeny {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode WriteDeny pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.writeDenyBin = append(r.writeDenyBin, binPubkey)
		}
	}

	// Convert ReadAllow hex strings to binary
	if len(r.ReadAllow) > 0 {
		r.readAllowBin = make([][]byte, 0, len(r.ReadAllow))
		for _, hexPubkey := range r.ReadAllow {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode ReadAllow pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.readAllowBin = append(r.readAllowBin, binPubkey)
		}
	}

	// Convert ReadDeny hex strings to binary
	if len(r.ReadDeny) > 0 {
		r.readDenyBin = make([][]byte, 0, len(r.ReadDeny))
		for _, hexPubkey := range r.ReadDeny {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode ReadDeny pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.readDenyBin = append(r.readDenyBin, binPubkey)
		}
	}

	// Parse MaxExpiryDuration into maxExpirySeconds
	// MaxExpiryDuration takes precedence over MaxExpiry if both are set
	if r.MaxExpiryDuration != "" {
		seconds, parseErr := parseDuration(r.MaxExpiryDuration)
		if parseErr != nil {
			log.W.F("failed to parse MaxExpiryDuration %q: %v", r.MaxExpiryDuration, parseErr)
		} else {
			r.maxExpirySeconds = &seconds
		}
	} else if r.MaxExpiry != nil { //nolint:staticcheck // Backward compatibility
		// Fall back to MaxExpiry (raw seconds) if MaxExpiryDuration not set
		r.maxExpirySeconds = r.MaxExpiry //nolint:staticcheck // Backward compatibility
	}

	// Compile IdentifierRegex pattern
	if r.IdentifierRegex != "" {
		compiled, compileErr := regexp.Compile(r.IdentifierRegex)
		if compileErr != nil {
			log.W.F("failed to compile IdentifierRegex %q: %v", r.IdentifierRegex, compileErr)
		} else {
			r.identifierRegexCache = compiled
		}
	}

	// Convert FollowsWhitelistAdmins hex strings to binary (DEPRECATED)
	if len(r.FollowsWhitelistAdmins) > 0 {
		r.followsWhitelistAdminsBin = make([][]byte, 0, len(r.FollowsWhitelistAdmins))
		for _, hexPubkey := range r.FollowsWhitelistAdmins {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode FollowsWhitelistAdmins pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.followsWhitelistAdminsBin = append(r.followsWhitelistAdminsBin, binPubkey)
		}
	}

	// Convert ReadFollowsWhitelist hex strings to binary
	if len(r.ReadFollowsWhitelist) > 0 {
		r.readFollowsWhitelistBin = make([][]byte, 0, len(r.ReadFollowsWhitelist))
		for _, hexPubkey := range r.ReadFollowsWhitelist {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode ReadFollowsWhitelist pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.readFollowsWhitelistBin = append(r.readFollowsWhitelistBin, binPubkey)
		}
	}

	// Convert WriteFollowsWhitelist hex strings to binary
	if len(r.WriteFollowsWhitelist) > 0 {
		r.writeFollowsWhitelistBin = make([][]byte, 0, len(r.WriteFollowsWhitelist))
		for _, hexPubkey := range r.WriteFollowsWhitelist {
			binPubkey, decErr := hex.Dec(hexPubkey)
			if decErr != nil {
				log.W.F("failed to decode WriteFollowsWhitelist pubkey %q: %v", hexPubkey, decErr)
				continue
			}
			r.writeFollowsWhitelistBin = append(r.writeFollowsWhitelistBin, binPubkey)
		}
	}

	return err
}

// IsInFollowsWhitelist checks if the given pubkey is in this rule's follows whitelist.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
func (r *Rule) IsInFollowsWhitelist(pubkey []byte) bool {
	if len(pubkey) == 0 || len(r.followsWhitelistFollowsBin) == 0 {
		return false
	}
	for _, follow := range r.followsWhitelistFollowsBin {
		if utils.FastEqual(pubkey, follow) {
			return true
		}
	}
	return false
}

// UpdateFollowsWhitelist sets the follows list for this rule's FollowsWhitelistAdmins.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
func (r *Rule) UpdateFollowsWhitelist(follows [][]byte) {
	r.followsWhitelistFollowsBin = follows
}

// GetFollowsWhitelistAdminsBin returns the binary-encoded admin pubkeys for this rule.
func (r *Rule) GetFollowsWhitelistAdminsBin() [][]byte {
	return r.followsWhitelistAdminsBin
}

// HasFollowsWhitelistAdmins returns true if this rule has FollowsWhitelistAdmins configured.
// DEPRECATED: Use HasReadFollowsWhitelist and HasWriteFollowsWhitelist instead.
func (r *Rule) HasFollowsWhitelistAdmins() bool {
	return len(r.FollowsWhitelistAdmins) > 0
}

// HasReadFollowsWhitelist returns true if this rule has ReadFollowsWhitelist configured.
func (r *Rule) HasReadFollowsWhitelist() bool {
	return len(r.ReadFollowsWhitelist) > 0
}

// HasWriteFollowsWhitelist returns true if this rule has WriteFollowsWhitelist configured.
func (r *Rule) HasWriteFollowsWhitelist() bool {
	return len(r.WriteFollowsWhitelist) > 0
}

// GetReadFollowsWhitelistBin returns the binary-encoded pubkeys for ReadFollowsWhitelist.
func (r *Rule) GetReadFollowsWhitelistBin() [][]byte {
	return r.readFollowsWhitelistBin
}

// GetWriteFollowsWhitelistBin returns the binary-encoded pubkeys for WriteFollowsWhitelist.
func (r *Rule) GetWriteFollowsWhitelistBin() [][]byte {
	return r.writeFollowsWhitelistBin
}

// UpdateReadFollowsWhitelist sets the follows list for this rule's ReadFollowsWhitelist.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
func (r *Rule) UpdateReadFollowsWhitelist(follows [][]byte) {
	r.readFollowsFollowsBin = follows
}

// UpdateWriteFollowsWhitelist sets the follows list for this rule's WriteFollowsWhitelist.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
func (r *Rule) UpdateWriteFollowsWhitelist(follows [][]byte) {
	r.writeFollowsFollowsBin = follows
}

// IsInReadFollowsWhitelist checks if the given pubkey is in this rule's read follows whitelist.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
// Returns true if either:
// 1. The pubkey is one of the ReadFollowsWhitelist pubkeys themselves, OR
// 2. The pubkey is in the follows list of the ReadFollowsWhitelist pubkeys.
func (r *Rule) IsInReadFollowsWhitelist(pubkey []byte) bool {
	if len(pubkey) == 0 {
		return false
	}
	// Check if pubkey is one of the whitelist pubkeys themselves
	for _, wlPubkey := range r.readFollowsWhitelistBin {
		if utils.FastEqual(pubkey, wlPubkey) {
			return true
		}
	}
	// Check if pubkey is in the follows list
	for _, follow := range r.readFollowsFollowsBin {
		if utils.FastEqual(pubkey, follow) {
			return true
		}
	}
	return false
}

// IsInWriteFollowsWhitelist checks if the given pubkey is in this rule's write follows whitelist.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
// Returns true if either:
// 1. The pubkey is one of the WriteFollowsWhitelist pubkeys themselves, OR
// 2. The pubkey is in the follows list of the WriteFollowsWhitelist pubkeys.
func (r *Rule) IsInWriteFollowsWhitelist(pubkey []byte) bool {
	if len(pubkey) == 0 {
		return false
	}
	// Check if pubkey is one of the whitelist pubkeys themselves
	for _, wlPubkey := range r.writeFollowsWhitelistBin {
		if utils.FastEqual(pubkey, wlPubkey) {
			return true
		}
	}
	// Check if pubkey is in the follows list
	for _, follow := range r.writeFollowsFollowsBin {
		if utils.FastEqual(pubkey, follow) {
			return true
		}
	}
	return false
}

// PolicyEvent represents an event with additional context for policy scripts.
// It embeds the Nostr event and adds authentication and network context.
type PolicyEvent struct {
	*event.E
	LoggedInPubkey string `json:"logged_in_pubkey,omitempty"`
	IPAddress      string `json:"ip_address,omitempty"`
	AccessType     string `json:"access_type,omitempty"` // "read" or "write"
}

// MarshalJSON implements custom JSON marshaling for PolicyEvent.
// It safely serializes the embedded event and additional context fields.
func (pe *PolicyEvent) MarshalJSON() ([]byte, error) {
	if pe.E == nil {
		return json.Marshal(
			map[string]interface{}{
				"logged_in_pubkey": pe.LoggedInPubkey,
				"ip_address":       pe.IPAddress,
			},
		)
	}

	// Create a safe copy of the event for JSON marshaling
	safeEvent := map[string]interface{}{
		"id":         hex.Enc(pe.E.ID),
		"pubkey":     hex.Enc(pe.E.Pubkey),
		"created_at": pe.E.CreatedAt,
		"kind":       pe.E.Kind,
		"content":    string(pe.E.Content),
		"tags":       pe.E.Tags,
		"sig":        hex.Enc(pe.E.Sig),
	}

	// Add policy-specific fields
	if pe.LoggedInPubkey != "" {
		safeEvent["logged_in_pubkey"] = pe.LoggedInPubkey
	}
	if pe.IPAddress != "" {
		safeEvent["ip_address"] = pe.IPAddress
	}
	if pe.AccessType != "" {
		safeEvent["access_type"] = pe.AccessType
	}

	return json.Marshal(safeEvent)
}

// PolicyResponse represents a response from the policy script.
// The script should return JSON with these fields to indicate its decision.
type PolicyResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"` // accept, reject, or shadowReject
	Msg    string `json:"msg"`    // NIP-20 response message (only used for reject)
}

// ScriptRunner manages a single policy script process.
// Each unique script path gets its own independent runner with its own goroutine.
type ScriptRunner struct {
	ctx           context.Context
	cancel        context.CancelFunc
	configDir     string
	scriptPath    string
	currentCmd    *exec.Cmd
	currentCancel context.CancelFunc
	mutex         sync.RWMutex
	isRunning     bool
	isStarting    bool
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	responseChan  chan PolicyResponse
	startupChan   chan error
}

// PolicyManager handles multiple policy script runners.
// It manages the lifecycle of policy scripts, handles communication with them,
// and provides resilient operation with automatic restart capabilities.
// Each unique script path gets its own ScriptRunner instance.
type PolicyManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	configDir  string
	configPath string // Path to policy.json file
	scriptPath string // Default script path for backward compatibility
	enabled    bool
	mutex      sync.RWMutex
	runners    map[string]*ScriptRunner // Map of script path -> runner
}

// ConfigPath returns the path to the policy configuration file.
// This is used by hot-reload handlers to know where to save updated policy.
func (pm *PolicyManager) ConfigPath() string {
	return pm.configPath
}

// P represents a complete policy configuration for a Nostr relay.
// It defines access control rules, kind filtering, and default behavior.
// Policies are evaluated in order: global rules, kind filtering, specific rules, then default policy.
type P struct {
	// Kind is policies for accepting or rejecting events by kind number.
	Kind Kinds `json:"kind"`
	// rules is a map of rules for criteria that must be met for the event to be allowed to be written to the relay.
	// Unexported to enforce use of public API methods (CheckPolicy, IsEnabled).
	rules map[int]Rule
	// Global is a rule set that applies to all events.
	Global Rule `json:"global"`
	// DefaultPolicy determines the default behavior when no rules deny an event ("allow" or "deny", defaults to "allow")
	DefaultPolicy string `json:"default_policy"`

	// PolicyAdmins is a list of hex-encoded pubkeys that can update policy configuration via kind 12345 events.
	// These are SEPARATE from ACL relay admins - policy admins manage policy only.
	PolicyAdmins []string `json:"policy_admins,omitempty"`
	// PolicyFollowWhitelistEnabled enables automatic whitelisting of pubkeys followed by policy admins.
	// When true and a rule has WriteAllowFollows=true, policy admin follows get read+write access.
	PolicyFollowWhitelistEnabled bool `json:"policy_follow_whitelist_enabled,omitempty"`

	// Owners is a list of hex-encoded pubkeys that have full control of the relay.
	// These are merged with owners from the ORLY_OWNERS environment variable.
	// Useful for cloud deployments where environment variables cannot be modified.
	Owners []string `json:"owners,omitempty"`

	// Unexported binary caches for faster comparison (populated from hex strings above)
	policyAdminsBin [][]byte // Binary cache for policy admin pubkeys
	policyFollows   [][]byte // Cached follow list from policy admins (kind 3 events)
	ownersBin       [][]byte // Binary cache for policy-defined owner pubkeys

	// followsMx protects all follows-related caches from concurrent access.
	// This includes policyFollows, Global.readFollowsFollowsBin, Global.writeFollowsFollowsBin,
	// and rule-specific follows whitelists.
	// Use RLock for reads (CheckPolicy) and Lock for writes (Update*Follows*).
	followsMx sync.RWMutex

	// manager handles policy script execution.
	// Unexported to enforce use of public API methods (CheckPolicy, IsEnabled).
	manager *PolicyManager
}

// pJSON is a shadow struct for JSON unmarshalling with exported fields.
type pJSON struct {
	Kind                         Kinds        `json:"kind"`
	Rules                        map[int]Rule `json:"rules"`
	Global                       Rule         `json:"global"`
	DefaultPolicy                string       `json:"default_policy"`
	PolicyAdmins                 []string     `json:"policy_admins,omitempty"`
	PolicyFollowWhitelistEnabled bool         `json:"policy_follow_whitelist_enabled,omitempty"`
	Owners                       []string     `json:"owners,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshalling to handle unexported fields.
func (p *P) UnmarshalJSON(data []byte) error {
	var shadow pJSON
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	p.Kind = shadow.Kind
	p.rules = shadow.Rules
	p.Global = shadow.Global
	p.DefaultPolicy = shadow.DefaultPolicy
	p.PolicyAdmins = shadow.PolicyAdmins
	p.PolicyFollowWhitelistEnabled = shadow.PolicyFollowWhitelistEnabled
	p.Owners = shadow.Owners

	// Populate binary cache for policy admins
	if len(p.PolicyAdmins) > 0 {
		p.policyAdminsBin = make([][]byte, 0, len(p.PolicyAdmins))
		for _, hexPubkey := range p.PolicyAdmins {
			binPubkey, err := hex.Dec(hexPubkey)
			if err != nil {
				log.W.F("failed to decode PolicyAdmin pubkey %q: %v", hexPubkey, err)
				continue
			}
			p.policyAdminsBin = append(p.policyAdminsBin, binPubkey)
		}
	}

	// Populate binary cache for policy-defined owners
	if len(p.Owners) > 0 {
		p.ownersBin = make([][]byte, 0, len(p.Owners))
		for _, hexPubkey := range p.Owners {
			binPubkey, err := hex.Dec(hexPubkey)
			if err != nil {
				log.W.F("failed to decode owner pubkey %q: %v", hexPubkey, err)
				continue
			}
			p.ownersBin = append(p.ownersBin, binPubkey)
		}
	}

	return nil
}

// New creates a new policy from JSON configuration.
// If policyJSON is empty, returns a policy with default settings.
// The default_policy field defaults to "allow" if not specified.
// Returns an error if the policy JSON contains invalid values (e.g., invalid
// ISO-8601 duration format for max_expiry_duration, invalid regex patterns, etc.).
func New(policyJSON []byte) (p *P, err error) {
	p = &P{
		DefaultPolicy: "allow", // Set default value
	}
	if len(policyJSON) > 0 {
		// Validate JSON before loading to fail fast on invalid configurations.
		// This prevents silent failures where invalid values (like "T10M" instead
		// of "PT10M" for max_expiry_duration) are ignored and constraints don't apply.
		if err = p.ValidateJSON(policyJSON); err != nil {
			return nil, fmt.Errorf("policy validation failed: %v", err)
		}
		if err = json.Unmarshal(policyJSON, p); chk.E(err) {
			return nil, fmt.Errorf("failed to unmarshal policy JSON: %v", err)
		}
	}
	// Ensure default policy is valid
	if p.DefaultPolicy == "" {
		p.DefaultPolicy = "allow"
	}

	// Populate binary caches for all rules (including global rule)
	p.Global.populateBinaryCache()
	for kind := range p.rules {
		rule := p.rules[kind] // Get a copy
		rule.populateBinaryCache()
		p.rules[kind] = rule // Store the modified copy back
	}

	return
}

// IsPartyInvolved checks if the given pubkey is a party involved in the event.
// A party is involved if they are either:
// 1. The author of the event (ev.Pubkey == userPubkey)
// 2. Mentioned in a p-tag of the event
//
// Both ev.Pubkey and userPubkey must be binary ([]byte), not hex-encoded.
// P-tags may be stored in either binary-optimized format (33 bytes) or hex format.
//
// This is the single source of truth for "parties_involved" / "privileged" checks.
func IsPartyInvolved(ev *event.E, userPubkey []byte) bool {
	// Must be authenticated
	if len(userPubkey) == 0 {
		return false
	}

	// Check if user is the author
	if bytes.Equal(ev.Pubkey, userPubkey) {
		return true
	}

	// Check if user is in p tags
	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pTag := range pTags {
		// ValueHex() handles both binary and hex storage formats automatically
		pt, err := hex.Dec(string(pTag.ValueHex()))
		if err != nil {
			// Skip malformed tags
			continue
		}
		if bytes.Equal(pt, userPubkey) {
			return true
		}
	}

	return false
}

// IsEnabled returns whether the policy system is enabled and ready to process events.
// This is the public API for checking if policy filtering should be applied.
func (p *P) IsEnabled() bool {
	return p != nil && p.manager != nil && p.manager.IsEnabled()
}

// ConfigPath returns the path to the policy configuration file.
// Delegates to the internal PolicyManager.
func (p *P) ConfigPath() string {
	if p == nil || p.manager == nil {
		return ""
	}
	return p.manager.ConfigPath()
}

// getDefaultPolicyAction returns true if the default policy is "allow", false if "deny"
func (p *P) getDefaultPolicyAction() (allowed bool) {
	switch p.DefaultPolicy {
	case "deny":
		return false
	case "allow", "":
		return true
	default:
		// Invalid value, default to allow
		return true
	}
}

// NewWithManager creates a new policy with a policy manager for script execution.
// It initializes the policy manager, loads configuration from files, and starts
// background processes for script management and periodic health checks.
//
// The customPolicyPath parameter allows overriding the default policy file location.
// If empty, uses the default path: $HOME/.config/{appName}/policy.json
// If provided, it MUST be an absolute path (starting with /) or the function will panic.
func NewWithManager(ctx context.Context, appName string, enabled bool, customPolicyPath string) *P {
	configDir := filepath.Join(xdg.ConfigHome, appName)
	scriptPath := filepath.Join(configDir, "policy.sh")

	// Determine the policy config path
	var configPath string
	if customPolicyPath != "" {
		// Validate that custom path is absolute
		if !filepath.IsAbs(customPolicyPath) {
			panic(fmt.Sprintf("FATAL: ORLY_POLICY_PATH must be an ABSOLUTE path (starting with /), got: %q", customPolicyPath))
		}
		configPath = customPolicyPath
		// Update configDir to match the custom path's directory for script resolution
		configDir = filepath.Dir(customPolicyPath)
		scriptPath = filepath.Join(configDir, "policy.sh")
		log.I.F("using custom policy path: %s", configPath)
	} else {
		configPath = filepath.Join(configDir, "policy.json")
	}

	ctx, cancel := context.WithCancel(ctx)

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  configDir,
		configPath: configPath,
		scriptPath: scriptPath,
		enabled:    enabled,
		runners:    make(map[string]*ScriptRunner),
	}

	// Load policy configuration from JSON file
	policy := &P{
		DefaultPolicy: "allow", // Set default value
		manager:       manager,
	}

	if enabled {
		if err := policy.LoadFromFile(configPath); err != nil {
			log.E.F(
				"FATAL: Policy system is ENABLED (ORLY_POLICY_ENABLED=true) but configuration failed to load from %s: %v",
				configPath, err,
			)
			log.E.F("The relay cannot start with an invalid policy configuration.")
			log.E.F("Fix: Either disable the policy system (ORLY_POLICY_ENABLED=false) or ensure %s exists and contains valid JSON", configPath)
			panic(fmt.Sprintf("fatal policy configuration error: %v", err))
		}
		log.I.F("loaded policy configuration from %s", configPath)

		// Start the policy script if it exists and is enabled
		go manager.startPolicyIfExists()
		// Start periodic check for policy script availability
		go manager.periodicCheck()
	}

	return policy
}

// getOrCreateRunner gets an existing runner for the script path or creates a new one.
// This method is thread-safe and ensures only one runner exists per unique script path.
func (pm *PolicyManager) getOrCreateRunner(scriptPath string) *ScriptRunner {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Check if runner already exists
	if runner, exists := pm.runners[scriptPath]; exists {
		return runner
	}

	// Create new runner
	runnerCtx, runnerCancel := context.WithCancel(pm.ctx)
	runner := &ScriptRunner{
		ctx:          runnerCtx,
		cancel:       runnerCancel,
		configDir:    pm.configDir,
		scriptPath:   scriptPath,
		responseChan: make(chan PolicyResponse, 100),
		startupChan:  make(chan error, 1),
	}

	pm.runners[scriptPath] = runner

	// Start periodic check for this runner
	go runner.periodicCheck()

	return runner
}

// ScriptRunner methods

// IsRunning returns whether the script is currently running.
func (sr *ScriptRunner) IsRunning() bool {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	return sr.isRunning
}

// ensureRunning ensures the script is running, starting it if necessary.
func (sr *ScriptRunner) ensureRunning() error {
	sr.mutex.Lock()
	// Check if already running
	if sr.isRunning {
		sr.mutex.Unlock()
		return nil
	}

	// Check if already starting
	if sr.isStarting {
		sr.mutex.Unlock()
		// Wait for startup to complete
		select {
		case err := <-sr.startupChan:
			if err != nil {
				return fmt.Errorf("script startup failed: %v", err)
			}
			// Double-check it's actually running after receiving signal
			sr.mutex.RLock()
			running := sr.isRunning
			sr.mutex.RUnlock()
			if !running {
				return fmt.Errorf("script startup completed but process is not running")
			}
			return nil
		case <-time.After(10 * time.Second):
			return fmt.Errorf("script startup timeout")
		case <-sr.ctx.Done():
			return fmt.Errorf("script context cancelled")
		}
	}

	// Mark as starting
	sr.isStarting = true
	sr.mutex.Unlock()

	// Start the script in a goroutine
	go func() {
		err := sr.Start()
		sr.mutex.Lock()
		sr.isStarting = false
		sr.mutex.Unlock()
		// Signal startup completion (non-blocking)
		// Drain any stale value first, then send
		select {
		case <-sr.startupChan:
		default:
		}
		select {
		case sr.startupChan <- err:
		default:
			// Channel should be empty now, but if it's full, try again
			sr.startupChan <- err
		}
	}()

	// Wait for startup to complete
	select {
	case err := <-sr.startupChan:
		if err != nil {
			return fmt.Errorf("script startup failed: %v", err)
		}
		// Double-check it's actually running after receiving signal
		sr.mutex.RLock()
		running := sr.isRunning
		sr.mutex.RUnlock()
		if !running {
			return fmt.Errorf("script startup completed but process is not running")
		}
		return nil
	case <-time.After(10 * time.Second):
		sr.mutex.Lock()
		sr.isStarting = false
		sr.mutex.Unlock()
		return fmt.Errorf("script startup timeout")
	case <-sr.ctx.Done():
		sr.mutex.Lock()
		sr.isStarting = false
		sr.mutex.Unlock()
		return fmt.Errorf("script context cancelled")
	}
}

// Start starts the script process.
func (sr *ScriptRunner) Start() error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	if sr.isRunning {
		return fmt.Errorf("script is already running")
	}

	if _, err := os.Stat(sr.scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script does not exist at %s", sr.scriptPath)
	}

	// Create a new context for this command
	cmdCtx, cmdCancel := context.WithCancel(sr.ctx)

	// Make the script executable
	if err := os.Chmod(sr.scriptPath, 0755); chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	// Start the script
	cmd := exec.CommandContext(cmdCtx, sr.scriptPath)
	cmd.Dir = sr.configDir

	// Set up stdio pipes for communication
	stdin, err := cmd.StdinPipe()
	if chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if chk.E(err) {
		cmdCancel()
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if chk.E(err) {
		cmdCancel()
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); chk.E(err) {
		cmdCancel()
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start script: %v", err)
	}

	sr.currentCmd = cmd
	sr.currentCancel = cmdCancel
	sr.stdin = stdin
	sr.stdout = stdout
	sr.stderr = stderr
	sr.isRunning = true

	// Start response reader in background
	go sr.readResponses()

	// Log stderr output in background
	go sr.logOutput(stdout, stderr)

	// Monitor the process
	go sr.monitorProcess()

	log.I.F(
		"policy script started: %s (pid=%d)", sr.scriptPath, cmd.Process.Pid,
	)
	return nil
}

// Stop stops the script gracefully.
func (sr *ScriptRunner) Stop() error {
	sr.mutex.Lock()

	if !sr.isRunning || sr.currentCmd == nil {
		sr.mutex.Unlock()
		return fmt.Errorf("script is not running")
	}

	// Close stdin first to signal the script to exit
	if sr.stdin != nil {
		sr.stdin.Close()
	}

	// Cancel the context
	if sr.currentCancel != nil {
		sr.currentCancel()
	}

	// Get the process reference before releasing the lock
	process := sr.currentCmd.Process
	sr.mutex.Unlock()

	// Wait for graceful shutdown with timeout
	// Note: monitorProcess() is the one that calls cmd.Wait() and cleans up
	// We just wait for it to finish by polling isRunning
	gracefulShutdown := false
	for i := 0; i < 50; i++ { // 5 seconds total (50 * 100ms)
		time.Sleep(100 * time.Millisecond)
		sr.mutex.RLock()
		running := sr.isRunning
		sr.mutex.RUnlock()
		if !running {
			gracefulShutdown = true
			log.I.F("policy script stopped gracefully: %s", sr.scriptPath)
			break
		}
	}

	if !gracefulShutdown {
		// Force kill after timeout
		log.W.F(
			"policy script did not stop gracefully, sending SIGKILL: %s",
			sr.scriptPath,
		)
		if process != nil {
			if err := process.Kill(); chk.E(err) {
				log.E.F("failed to kill script process: %v", err)
			}
		}

		// Wait a bit more for monitorProcess to clean up
		for i := 0; i < 30; i++ { // 3 more seconds
			time.Sleep(100 * time.Millisecond)
			sr.mutex.RLock()
			running := sr.isRunning
			sr.mutex.RUnlock()
			if !running {
				break
			}
		}
	}

	return nil
}

// ProcessEvent sends an event to the script and waits for a response.
func (sr *ScriptRunner) ProcessEvent(evt *PolicyEvent) (
	*PolicyResponse, error,
) {
	log.D.F("processing event: %s", evt.Serialize())
	sr.mutex.RLock()
	if !sr.isRunning || sr.stdin == nil {
		sr.mutex.RUnlock()
		return nil, fmt.Errorf("script is not running")
	}
	stdin := sr.stdin
	sr.mutex.RUnlock()

	// Serialize the event to JSON
	eventJSON, err := json.Marshal(evt)
	if chk.E(err) {
		return nil, fmt.Errorf("failed to serialize event: %v", err)
	}

	// Send the event JSON to the script (newline-terminated)
	if _, err := stdin.Write(append(eventJSON, '\n')); chk.E(err) {
		// Check if it's a broken pipe error, which means the script has died
		if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "closed pipe") {
			log.E.F(
				"policy script %s stdin closed (broken pipe) - script may have crashed or exited prematurely",
				sr.scriptPath,
			)
			// Mark as not running so it will be restarted on next periodic check
			sr.mutex.Lock()
			sr.isRunning = false
			sr.mutex.Unlock()
		}
		return nil, fmt.Errorf("failed to write event to script: %v", err)
	}

	// Wait for response with timeout
	select {
	case response := <-sr.responseChan:
		log.D.S("response", response)
		return &response, nil
	case <-time.After(5 * time.Second):
		log.W.F(
			"policy script %s response timeout - script may not be responding correctly (check for debug output on stdout)",
			sr.scriptPath,
		)
		return nil, fmt.Errorf("script response timeout")
	case <-sr.ctx.Done():
		return nil, fmt.Errorf("script context cancelled")
	}
}

// readResponses reads JSONL responses from the script
func (sr *ScriptRunner) readResponses() {
	if sr.stdout == nil {
		return
	}

	scanner := bufio.NewScanner(sr.stdout)
	nonJSONLineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		log.D.F("policy response: %s", line)
		var response PolicyResponse
		if err := json.Unmarshal([]byte(line), &response); chk.E(err) {
			// Check if this looks like debug output
			if strings.HasPrefix(line, "{") {
				// Looks like JSON but failed to parse
				log.E.F(
					"failed to parse policy response from %s: %v\nLine: %s",
					sr.scriptPath, err, line,
				)
			} else {
				// Definitely not JSON - probably debug output
				nonJSONLineCount++
				if nonJSONLineCount <= 3 {
					log.W.F(
						"policy script %s produced non-JSON output on stdout (should only output JSONL): %q",
						sr.scriptPath, line,
					)
				} else if nonJSONLineCount == 4 {
					log.W.F(
						"policy script %s continues to produce non-JSON output - suppressing further warnings",
						sr.scriptPath,
					)
				}
				log.W.F(
					"IMPORTANT: Policy scripts must ONLY write JSON responses to stdout. Use stderr or a log file for debug output.",
				)
			}
			continue
		}

		// Send response to channel (non-blocking)
		select {
		case sr.responseChan <- response:
		default:
			log.W.F(
				"policy response channel full for %s, dropping response",
				sr.scriptPath,
			)
		}
	}

	if err := scanner.Err(); chk.E(err) {
		log.E.F(
			"error reading policy responses from %s: %v", sr.scriptPath, err,
		)
	}
}

// logOutput logs the output from stderr
func (sr *ScriptRunner) logOutput(_ /* stdout */, stderr io.ReadCloser) {
	defer stderr.Close()

	// Only log stderr, stdout is used by readResponses
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				// Log script stderr output through relay logging system
				log.I.F("[policy script %s] %s", sr.scriptPath, line)
			}
		}
		if err := scanner.Err(); chk.E(err) {
			log.E.F("error reading stderr from policy script %s: %v", sr.scriptPath, err)
		}
	}()
}

// monitorProcess monitors the script process and cleans up when it exits
func (sr *ScriptRunner) monitorProcess() {
	if sr.currentCmd == nil {
		return
	}

	err := sr.currentCmd.Wait()

	sr.mutex.Lock()
	defer sr.mutex.Unlock()

	// Clean up pipes
	if sr.stdin != nil {
		sr.stdin.Close()
		sr.stdin = nil
	}
	if sr.stdout != nil {
		sr.stdout.Close()
		sr.stdout = nil
	}
	if sr.stderr != nil {
		sr.stderr.Close()
		sr.stderr = nil
	}

	sr.isRunning = false
	sr.currentCmd = nil
	sr.currentCancel = nil

	if err != nil {
		log.E.F(
			"policy script exited with error: %s: %v, will retry periodically",
			sr.scriptPath, err,
		)
	} else {
		log.I.F("policy script exited normally: %s", sr.scriptPath)
	}
}

// periodicCheck periodically checks if script becomes available and attempts to restart failed scripts.
func (sr *ScriptRunner) periodicCheck() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sr.ctx.Done():
			return
		case <-ticker.C:
			sr.mutex.RLock()
			running := sr.isRunning
			sr.mutex.RUnlock()

			// Check if script is not running and try to start it
			if !running {
				if _, err := os.Stat(sr.scriptPath); err == nil {
					// Script exists but not running, try to start
					go func() {
						if err := sr.Start(); err != nil {
							log.E.F(
								"failed to restart policy script %s: %v, will retry in next cycle",
								sr.scriptPath, err,
							)
						} else {
							log.I.F(
								"policy script restarted successfully: %s",
								sr.scriptPath,
							)
						}
					}()
				}
			}
		}
	}
}

// LoadFromFile loads policy configuration from a JSON file.
// Returns an error if the file doesn't exist, can't be read, or contains invalid JSON.
func (p *P) LoadFromFile(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"policy configuration file does not exist: %s", configPath,
		)
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read policy configuration file: %v", err)
	}

	if len(configData) == 0 {
		return fmt.Errorf("policy configuration file is empty")
	}

	if err := json.Unmarshal(configData, p); err != nil {
		return fmt.Errorf("failed to parse policy configuration JSON: %v", err)
	}

	// Populate binary caches for all rules (including global rule)
	p.Global.populateBinaryCache()
	for kind, rule := range p.rules {
		rule.populateBinaryCache()
		p.rules[kind] = rule // Update the map with the modified rule
	}

	return nil
}

// CheckPolicy checks if an event is allowed based on the policy configuration.
// The access parameter should be "write" for accepting events or "read" for filtering events.
// Returns true if the event is allowed, false if denied, and an error if validation fails.
//
// Policy evaluation order (more specific rules take precedence):
// 1. Kinds whitelist/blacklist - if kind is blocked, deny immediately
// 2. Kind-specific rule - if exists for this kind, use it exclusively
// 3. Global rule - fallback if no kind-specific rule exists
// 4. Default policy - fallback if no rules apply
//
// Thread-safety: Uses followsMx.RLock to protect reads of follows whitelists during policy checks.
// Write operations (Update*) acquire the write lock, which blocks concurrent reads.
func (p *P) CheckPolicy(
	access string, ev *event.E, loggedInPubkey []byte, ipAddress string,
) (allowed bool, err error) {
	// Handle nil policy - this should not happen if policy is enabled
	// If policy is enabled but p is nil, it's a configuration error
	if p == nil {
		log.F.Ln("FATAL: CheckPolicy called on nil policy - this indicates misconfiguration. " +
			"If ORLY_POLICY_ENABLED=true, ensure policy configuration is valid.")
		return false, fmt.Errorf("policy is nil but policy checking is enabled - check configuration")
	}

	// Handle nil event
	if ev == nil {
		return false, fmt.Errorf("event cannot be nil")
	}

	// Acquire read lock to protect follows whitelists during policy check
	p.followsMx.RLock()
	defer p.followsMx.RUnlock()

	// ==========================================================================
	// STEP 1: Check kinds whitelist/blacklist (applies before any rule checks)
	// ==========================================================================
	if !p.checkKindsPolicy(access, ev.Kind) {
		return false, nil
	}

	// ==========================================================================
	// STEP 2: Check KIND-SPECIFIC rule FIRST (more specific = higher priority)
	// ==========================================================================
	// If kind-specific rule exists and accepts, that's final - global is ignored.
	rule, hasKindRule := p.rules[int(ev.Kind)]
	if hasKindRule {
		// Check if script is present and enabled for this kind
		if rule.Script != "" && p.manager != nil {
			if p.manager.IsEnabled() {
				// Check if script file exists before trying to use it
				if _, err := os.Stat(rule.Script); err == nil {
					// Script exists, try to use it
					log.D.F("using policy script for kind %d: %s", ev.Kind, rule.Script)
					allowed, err := p.checkScriptPolicy(
						access, ev, rule.Script, loggedInPubkey, ipAddress,
					)
					if err == nil {
						// Script ran successfully, return its decision
						return allowed, nil
					}
					// Script failed, fall through to apply other criteria
					log.W.F("policy script check failed for kind %d: %v, applying other criteria",
						ev.Kind, err)
				} else {
					// Script configured but doesn't exist
					log.W.F("policy script configured for kind %d but not found at %s: %v, applying other criteria",
						ev.Kind, rule.Script, err)
				}
				// Script doesn't exist or failed, fall through to apply other criteria
			} else {
				// Policy manager is disabled, fall back to default policy
				log.D.F("policy manager is disabled for kind %d, falling back to default policy (%s)",
					ev.Kind, p.DefaultPolicy)
				return p.getDefaultPolicyAction(), nil
			}
		}

		// Apply kind-specific rule-based filtering
		return p.checkRulePolicy(access, ev, rule, loggedInPubkey)
	}

	// ==========================================================================
	// STEP 3: No kind-specific rule - check GLOBAL rule as fallback
	// ==========================================================================

	// Check if global rule has any configuration
	if p.Global.hasAnyRules() {
		// Apply global rule filtering
		return p.checkRulePolicy(access, ev, p.Global, loggedInPubkey)
	}

	// ==========================================================================
	// STEP 4: No kind-specific or global rules - use default policy
	// ==========================================================================
	return p.getDefaultPolicyAction(), nil
}

// checkKindsPolicy checks if the event kind is allowed for the given access type.
// Logic:
// 1. If explicit whitelist exists, use it (but respect permissive flags for read/write)
// 2. If explicit blacklist exists, use it (but respect permissive flags for read/write)
// 3. Otherwise, kinds with defined rules are implicitly allowed, others denied (with permissive overrides)
//
// Permissive flags (set on Global rule):
// - ReadAllowPermissive: Allows READ access for kinds not in whitelist (write still restricted)
// - WriteAllowPermissive: Allows WRITE access for kinds not in whitelist (uses global rule constraints)
func (p *P) checkKindsPolicy(access string, kind uint16) bool {
	// If whitelist is present, only allow whitelisted kinds (with permissive overrides)
	if len(p.Kind.Whitelist) > 0 {
		for _, allowedKind := range p.Kind.Whitelist {
			if kind == uint16(allowedKind) {
				return true
			}
		}
		// Kind not in whitelist - check permissive flags
		if access == "read" && p.Global.ReadAllowPermissive {
			log.D.F("read_allow_permissive: allowing read for kind %d not in whitelist", kind)
			return true // Allow read even though kind not whitelisted
		}
		if access == "write" && p.Global.WriteAllowPermissive {
			log.D.F("write_allow_permissive: allowing write for kind %d not in whitelist (global rules apply)", kind)
			return true // Allow write even though kind not whitelisted, global rule will be applied
		}
		return false
	}

	// If blacklist is present, deny blacklisted kinds
	if len(p.Kind.Blacklist) > 0 {
		for _, deniedKind := range p.Kind.Blacklist {
			if kind == uint16(deniedKind) {
				// Kind is explicitly blacklisted - permissive flags don't override blacklist
				return false
			}
		}
		// Not in blacklist - check if rule exists for implicit whitelist
		_, hasRule := p.rules[int(kind)]
		if hasRule {
			return true
		}
		// No kind-specific rule - check permissive flags
		if access == "read" && p.Global.ReadAllowPermissive {
			log.D.F("read_allow_permissive: allowing read for kind %d (not blacklisted, no rule)", kind)
			return true
		}
		if access == "write" && p.Global.WriteAllowPermissive {
			log.D.F("write_allow_permissive: allowing write for kind %d (not blacklisted, no rule)", kind)
			return true
		}
		return false // Only allow if there's a rule defined
	}

	// No explicit whitelist or blacklist
	// Behavior depends on whether default_policy is explicitly set:
	// - If default_policy is explicitly "allow", allow all kinds (rules add constraints, not restrictions)
	// - If default_policy is unset or "deny", use implicit whitelist (only allow kinds with rules)
	// - If global rule has any configuration, allow kinds through for global rule checking
	// - Permissive flags can override implicit whitelist behavior
	if len(p.rules) > 0 {
		// If default_policy is explicitly "allow", don't use implicit whitelist
		if p.DefaultPolicy == "allow" {
			return true
		}
		// Implicit whitelist mode - only allow kinds with specific rules
		_, hasRule := p.rules[int(kind)]
		if hasRule {
			return true
		}
		// No kind-specific rule, but check if global rule exists
		if p.Global.hasAnyRules() {
			return true // Allow through for global rule check
		}
		// Check permissive flags for implicit whitelist override
		if access == "read" && p.Global.ReadAllowPermissive {
			log.D.F("read_allow_permissive: allowing read for kind %d (implicit whitelist override)", kind)
			return true
		}
		if access == "write" && p.Global.WriteAllowPermissive {
			log.D.F("write_allow_permissive: allowing write for kind %d (implicit whitelist override)", kind)
			return true
		}
		return false
	}
	// No kind-specific rules - check if global rule exists
	if p.Global.hasAnyRules() {
		return true // Allow through for global rule check
	}
	// No rules at all - fall back to default policy
	return p.getDefaultPolicyAction()
}

// checkGlobalFollowsWhitelistAccess checks if the user is explicitly granted access
// via the global rule's follows whitelists (read_follows_whitelist or write_follows_whitelist).
// This grants access that bypasses the default policy for kinds without specific rules.
// Note: p should never be nil here - caller (CheckPolicy) already validates this.
func (p *P) checkGlobalFollowsWhitelistAccess(access string, loggedInPubkey []byte) bool {
	if len(loggedInPubkey) == 0 {
		return false
	}

	if access == "read" {
		// Check if user is in global read follows whitelist
		if p.Global.HasReadFollowsWhitelist() && p.Global.IsInReadFollowsWhitelist(loggedInPubkey) {
			return true
		}
		// Also check legacy WriteAllowFollows and FollowsWhitelistAdmins for read access
		if p.Global.WriteAllowFollows && p.PolicyFollowWhitelistEnabled && p.IsPolicyFollow(loggedInPubkey) {
			return true
		}
		if p.Global.HasFollowsWhitelistAdmins() && p.Global.IsInFollowsWhitelist(loggedInPubkey) {
			return true
		}
	} else if access == "write" {
		// Check if user is in global write follows whitelist
		if p.Global.HasWriteFollowsWhitelist() && p.Global.IsInWriteFollowsWhitelist(loggedInPubkey) {
			return true
		}
		// Also check legacy WriteAllowFollows and FollowsWhitelistAdmins for write access
		if p.Global.WriteAllowFollows && p.PolicyFollowWhitelistEnabled && p.IsPolicyFollow(loggedInPubkey) {
			return true
		}
		if p.Global.HasFollowsWhitelistAdmins() && p.Global.IsInFollowsWhitelist(loggedInPubkey) {
			return true
		}
	}

	return false
}

// checkGlobalRulePolicy checks if the event passes the global rule filter
// Note: p should never be nil here - caller (CheckPolicy) already validates this.
func (p *P) checkGlobalRulePolicy(
	access string, ev *event.E, loggedInPubkey []byte,
) bool {
	// Skip if no global rules are configured
	if !p.Global.hasAnyRules() {
		return true
	}

	// Apply global rule filtering
	allowed, err := p.checkRulePolicy(access, ev, p.Global, loggedInPubkey)
	if err != nil {
		log.E.F("global rule policy check failed: %v", err)
		return false
	}
	return allowed
}

// checkRulePolicy evaluates rule-based access control with the following logic:
//
// READ ACCESS (default-permissive):
//   - Denied if in read_deny list
//   - If read_allow, read_follows_whitelist, or privileged is set, user must pass one of those checks
//   - Otherwise, read is allowed by default
//
// WRITE ACCESS (default-permissive):
//   - Denied if in write_deny list
//   - Universal constraints (size, tags, age) apply to writes only
//   - If write_allow or write_follows_whitelist is set, user must pass one of those checks
//   - Otherwise, write is allowed by default
//
// PRIVILEGED: Only applies to READ operations (party-involved check)
func (p *P) checkRulePolicy(
	access string, ev *event.E, rule Rule, loggedInPubkey []byte,
) (allowed bool, err error) {
	log.T.F("checkRulePolicy: access=%s kind=%d readFollowsFollowsBin_len=%d readFollowsWhitelistBin_len=%d HasReadFollowsWhitelist=%v",
		access, ev.Kind, len(rule.readFollowsFollowsBin), len(rule.readFollowsWhitelistBin), rule.HasReadFollowsWhitelist())

	// ===================================================================
	// STEP 1: Universal Constraints (WRITE ONLY - apply to everyone)
	// ===================================================================

	if access == "write" {
		// Check size limits
		if rule.SizeLimit != nil {
			eventSize := int64(len(ev.Serialize()))
			if eventSize > *rule.SizeLimit {
				return false, nil
			}
		}

		if rule.ContentLimit != nil {
			contentSize := int64(len(ev.Content))
			if contentSize > *rule.ContentLimit {
				return false, nil
			}
		}

		// Check required tags
		if len(rule.MustHaveTags) > 0 {
			for _, requiredTag := range rule.MustHaveTags {
				if ev.Tags.GetFirst([]byte(requiredTag)) == nil {
					return false, nil
				}
			}
		}

		// Check expiry time (uses maxExpirySeconds which is parsed from MaxExpiryDuration or MaxExpiry)
		if rule.maxExpirySeconds != nil && *rule.maxExpirySeconds > 0 {
			expiryTag := ev.Tags.GetFirst([]byte("expiration"))
			if expiryTag == nil {
				return false, nil // Must have expiry if max_expiry is set
			}
			// Parse expiry timestamp and validate it's within allowed duration from created_at
			expiryStr := string(expiryTag.Value())
			expiryTs, parseErr := strconv.ParseInt(expiryStr, 10, 64)
			if parseErr != nil {
				log.D.F("invalid expiration tag value %q: %v", expiryStr, parseErr)
				return false, nil // Invalid expiry format
			}
			maxAllowedExpiry := ev.CreatedAt + *rule.maxExpirySeconds
			if expiryTs >= maxAllowedExpiry {
				log.D.F("expiration %d exceeds max allowed %d (created_at %d + max_expiry %d)",
					expiryTs, maxAllowedExpiry, ev.CreatedAt, *rule.maxExpirySeconds)
				return false, nil // Expiry too far in the future
			}
		}

		// Check ProtectedRequired (NIP-70: events must have "-" tag)
		if rule.ProtectedRequired {
			protectedTag := ev.Tags.GetFirst([]byte("-"))
			if protectedTag == nil {
				log.D.F("protected_required: event missing '-' tag (NIP-70)")
				return false, nil // Must have protected tag
			}
		}

		// Check IdentifierRegex (validates "d" tag values)
		if rule.identifierRegexCache != nil {
			dTags := ev.Tags.GetAll([]byte("d"))
			if len(dTags) == 0 {
				log.D.F("identifier_regex: event missing 'd' tag")
				return false, nil // Must have d tag if identifier_regex is set
			}
			for _, dTag := range dTags {
				value := string(dTag.Value())
				if !rule.identifierRegexCache.MatchString(value) {
					log.D.F("identifier_regex: d tag value %q does not match pattern %q",
						value, rule.IdentifierRegex)
					return false, nil
				}
			}
		}

		// Check MaxAgeOfEvent (maximum age of event in seconds)
		if rule.MaxAgeOfEvent != nil && *rule.MaxAgeOfEvent > 0 {
			currentTime := time.Now().Unix()
			maxAllowedTime := currentTime - *rule.MaxAgeOfEvent
			if ev.CreatedAt < maxAllowedTime {
				return false, nil // Event is too old
			}
		}

		// Check MaxAgeEventInFuture (maximum time event can be in the future in seconds)
		if rule.MaxAgeEventInFuture != nil && *rule.MaxAgeEventInFuture > 0 {
			currentTime := time.Now().Unix()
			maxFutureTime := currentTime + *rule.MaxAgeEventInFuture
			if ev.CreatedAt > maxFutureTime {
				return false, nil // Event is too far in the future
			}
		}

		// Check tag validation rules (regex patterns)
		// NOTE: TagValidation only validates tags that ARE present on the event.
		// To REQUIRE a tag to exist, use MustHaveTags instead.
		if len(rule.TagValidation) > 0 {
			for tagName, regexPattern := range rule.TagValidation {
				// Compile regex pattern (errors should have been caught in ValidateJSON)
				regex, compileErr := regexp.Compile(regexPattern)
				if compileErr != nil {
					log.E.F("invalid regex pattern for tag %q: %v (skipping validation)", tagName, compileErr)
					continue
				}

				// Get all tags with this name
				tags := ev.Tags.GetAll([]byte(tagName))

				// If no tags found, skip validation for this tag type
				// (TagValidation validates format, not presence - use MustHaveTags for presence)
				if len(tags) == 0 {
					continue
				}

				// Validate each tag value against regex
				for _, t := range tags {
					value := string(t.Value())
					if !regex.MatchString(value) {
						log.D.F("tag validation failed: tag %q value %q does not match pattern %q",
							tagName, value, regexPattern)
						return false, nil
					}
				}
			}
		}
	}

	// ===================================================================
	// STEP 2: Explicit Denials (highest priority blacklist)
	// ===================================================================

	if access == "write" {
		// Check write deny list - deny specific users from submitting events
		if len(rule.writeDenyBin) > 0 {
			for _, deniedPubkey := range rule.writeDenyBin {
				if utils.FastEqual(loggedInPubkey, deniedPubkey) {
					return false, nil // Submitter explicitly denied
				}
			}
		} else if len(rule.WriteDeny) > 0 {
			// Fallback: binary cache not populated, use hex comparison
			loggedInPubkeyHex := hex.Enc(loggedInPubkey)
			for _, deniedPubkey := range rule.WriteDeny {
				if loggedInPubkeyHex == deniedPubkey {
					return false, nil // Submitter explicitly denied
				}
			}
		}
	} else if access == "read" {
		// Check read deny list
		if len(rule.readDenyBin) > 0 {
			for _, deniedPubkey := range rule.readDenyBin {
				if utils.FastEqual(loggedInPubkey, deniedPubkey) {
					return false, nil // Explicitly denied
				}
			}
		} else if len(rule.ReadDeny) > 0 {
			// Fallback: binary cache not populated, use hex comparison
			loggedInPubkeyHex := hex.Enc(loggedInPubkey)
			for _, deniedPubkey := range rule.ReadDeny {
				if loggedInPubkeyHex == deniedPubkey {
					return false, nil // Explicitly denied
				}
			}
		}
	}

	// ===================================================================
	// STEP 3: Legacy WriteAllowFollows (grants BOTH read AND write access)
	// ===================================================================

	// WriteAllowFollows grants both read and write access to policy admin follows
	// This check applies to BOTH read and write access types (legacy behavior)
	if rule.WriteAllowFollows && p.PolicyFollowWhitelistEnabled {
		if p.IsPolicyFollow(loggedInPubkey) {
			log.D.F("policy admin follow granted %s access for kind %d", access, ev.Kind)
			return true, nil // Allow access from policy admin follow
		}
	}

	// FollowsWhitelistAdmins grants access to follows of specific admin pubkeys for this rule
	// This is a per-rule alternative to WriteAllowFollows which uses global PolicyAdmins (DEPRECATED)
	if rule.HasFollowsWhitelistAdmins() {
		if rule.IsInFollowsWhitelist(loggedInPubkey) {
			log.D.F("follows_whitelist_admins granted %s access for kind %d", access, ev.Kind)
			return true, nil // Allow access from rule-specific admin follow
		}
	}

	// ===================================================================
	// STEP 4: New Follows Whitelist Checks (separate read/write)
	// ===================================================================

	if access == "read" {
		// Check ReadFollowsWhitelist - if set, it acts as a whitelist
		if rule.HasReadFollowsWhitelist() {
			if rule.IsInReadFollowsWhitelist(loggedInPubkey) {
				log.D.F("read_follows_whitelist granted read access for kind %d", ev.Kind)
				return true, nil
			}
			// ReadFollowsWhitelist is set but user is not in it
			// Continue to check other access methods (privileged, read_allow)
		}
	} else if access == "write" {
		// Check WriteFollowsWhitelist - if set, it acts as a whitelist
		if rule.HasWriteFollowsWhitelist() {
			if rule.IsInWriteFollowsWhitelist(loggedInPubkey) {
				log.D.F("write_follows_whitelist granted write access for kind %d", ev.Kind)
				return true, nil
			}
			// WriteFollowsWhitelist is set but user is not in it - must check write_allow too
		}
	}

	// ===================================================================
	// STEP 5: Read Access Control
	// ===================================================================

	if access == "read" {
		hasReadAllowList := len(rule.readAllowBin) > 0 || len(rule.ReadAllow) > 0
		hasReadFollowsWhitelist := rule.HasReadFollowsWhitelist()
		// Include deprecated FollowsWhitelistAdmins for backward compatibility (it grants read+write)
		hasLegacyFollowsWhitelist := rule.HasFollowsWhitelistAdmins()
		userIsPrivileged := rule.Privileged && IsPartyInvolved(ev, loggedInPubkey)

		// Check if user is in read allow list
		userInAllowList := false
		if len(rule.readAllowBin) > 0 {
			for _, allowedPubkey := range rule.readAllowBin {
				if utils.FastEqual(loggedInPubkey, allowedPubkey) {
					userInAllowList = true
					break
				}
			}
		} else if len(rule.ReadAllow) > 0 {
			loggedInPubkeyHex := hex.Enc(loggedInPubkey)
			for _, allowedPubkey := range rule.ReadAllow {
				if loggedInPubkeyHex == allowedPubkey {
					userInAllowList = true
					break
				}
			}
		}

		// Determine if any read whitelist restriction is active
		// Note: Legacy FollowsWhitelistAdmins also counts as a read restriction for backward compatibility
		hasReadRestriction := hasReadAllowList || hasReadFollowsWhitelist || hasLegacyFollowsWhitelist || rule.Privileged

		if hasReadRestriction {
			// User must pass one of the configured access methods
			if userInAllowList {
				return true, nil
			}
			if userIsPrivileged {
				return true, nil
			}
			// User is in ReadFollowsWhitelist was already checked in STEP 4
			// User in legacy FollowsWhitelistAdmins was already checked in STEP 3
			// If we reach here with a read restriction, deny access
			return false, nil
		}

		// No read restriction configured - read is permissive by default
		return true, nil
	}

	// ===================================================================
	// STEP 6: Write Access Control
	// ===================================================================

	if access == "write" {
		hasWriteAllowList := len(rule.writeAllowBin) > 0 || len(rule.WriteAllow) > 0
		hasWriteFollowsWhitelist := rule.HasWriteFollowsWhitelist()
		// Include deprecated FollowsWhitelistAdmins for backward compatibility
		hasLegacyFollowsWhitelist := rule.HasFollowsWhitelistAdmins()

		// Check if user is in write allow list
		userInAllowList := false
		if len(rule.writeAllowBin) > 0 {
			for _, allowedPubkey := range rule.writeAllowBin {
				if utils.FastEqual(loggedInPubkey, allowedPubkey) {
					userInAllowList = true
					break
				}
			}
		} else if len(rule.WriteAllow) > 0 {
			loggedInPubkeyHex := hex.Enc(loggedInPubkey)
			for _, allowedPubkey := range rule.WriteAllow {
				if loggedInPubkeyHex == allowedPubkey {
					userInAllowList = true
					break
				}
			}
		}

		// Determine if any write whitelist restriction is active
		// Note: Legacy FollowsWhitelistAdmins also counts as a write restriction for backward compatibility
		hasWriteRestriction := hasWriteAllowList || hasWriteFollowsWhitelist || hasLegacyFollowsWhitelist

		if hasWriteRestriction {
			// User must pass one of the configured access methods
			if userInAllowList {
				return true, nil
			}
			// User in WriteFollowsWhitelist was already checked in STEP 4
			// User in legacy FollowsWhitelistAdmins was already checked in STEP 3
			// If we reach here with a write restriction, deny access
			return false, nil
		}

		// No write restriction configured - write is permissive by default
		return true, nil
	}

	// ===================================================================
	// STEP 7: Default Policy (fallback)
	// ===================================================================

	// If no specific rules matched, use the configured default policy
	return p.getDefaultPolicyAction(), nil
}

// checkScriptPolicy runs the policy script to determine if event should be allowed
func (p *P) checkScriptPolicy(
	access string, ev *event.E, scriptPath string, loggedInPubkey []byte,
	ipAddress string,
) (allowed bool, err error) {
	if p.manager == nil {
		return false, fmt.Errorf("policy manager is not initialized")
	}

	// If policy is disabled, fall back to default policy immediately
	if !p.manager.IsEnabled() {
		log.W.F(
			"policy rule for kind %d is inactive (policy disabled), falling back to default policy (%s)",
			ev.Kind, p.DefaultPolicy,
		)
		return p.getDefaultPolicyAction(), nil
	}

	// Check if script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Script doesn't exist, return error so caller can fall back to other criteria
		return false, fmt.Errorf(
			"policy script does not exist at %s", scriptPath,
		)
	}

	// Get or create a runner for this specific script path
	runner := p.manager.getOrCreateRunner(scriptPath)

	// Policy is enabled, check if this runner is running
	if !runner.IsRunning() {
		// Try to start this runner and wait for it
		log.D.F("starting policy script for kind %d: %s", ev.Kind, scriptPath)
		if err := runner.ensureRunning(); err != nil {
			// Startup failed, return error so caller can fall back to other criteria
			return false, fmt.Errorf(
				"failed to start policy script %s: %v", scriptPath, err,
			)
		}
		log.I.F("policy script started for kind %d: %s", ev.Kind, scriptPath)
	}

	// Create policy event with additional context
	policyEvent := &PolicyEvent{
		E:              ev,
		LoggedInPubkey: hex.Enc(loggedInPubkey),
		IPAddress:      ipAddress,
		AccessType:     access,
	}

	// Process event through policy script
	response, scriptErr := runner.ProcessEvent(policyEvent)
	if chk.E(scriptErr) {
		log.E.F(
			"policy rule for kind %d failed (script processing error: %v), falling back to default policy (%s)",
			ev.Kind, scriptErr, p.DefaultPolicy,
		)
		// Fall back to default policy on script failure
		return p.getDefaultPolicyAction(), nil
	}

	// Handle script response
	switch response.Action {
	case "accept":
		return true, nil
	case "reject":
		return false, nil
	case "shadowReject":
		return false, nil // Treat as reject for policy purposes
	default:
		log.W.F(
			"policy rule for kind %d returned unknown action '%s', falling back to default policy (%s)",
			ev.Kind, response.Action, p.DefaultPolicy,
		)
		// Fall back to default policy for unknown actions
		return p.getDefaultPolicyAction(), nil
	}
}

// PolicyManager methods

// periodicCheck periodically checks if the default policy script becomes available.
// This is for backward compatibility with the default script path.
func (pm *PolicyManager) periodicCheck() {
	// Get or create runner for the default script path
	// This will also start its own periodic check
	pm.getOrCreateRunner(pm.scriptPath)
}

// startPolicyIfExists starts the default policy script if the file exists.
// This is for backward compatibility with the default script path.
// Only logs if the default script actually exists - missing default scripts are normal
// when users configure rule-specific scripts.
func (pm *PolicyManager) startPolicyIfExists() {
	if _, err := os.Stat(pm.scriptPath); err == nil {
		// Default script exists, try to start it
		log.I.F("found default policy script at %s, starting...", pm.scriptPath)
		runner := pm.getOrCreateRunner(pm.scriptPath)
		if err := runner.Start(); err != nil {
			log.E.F(
				"failed to start default policy script: %v, will retry periodically",
				err,
			)
		}
	}
	// Silently ignore if default script doesn't exist - it's fine if rules use custom scripts
}

// IsEnabled returns whether the policy manager is enabled.
func (pm *PolicyManager) IsEnabled() bool {
	return pm.enabled
}

// IsRunning returns whether the default policy script is currently running.
// Deprecated: Use getOrCreateRunner(scriptPath).IsRunning() for specific scripts.
func (pm *PolicyManager) IsRunning() bool {
	pm.mutex.RLock()
	defer pm.mutex.RUnlock()

	// Check if default script runner exists and is running
	if runner, exists := pm.runners[pm.scriptPath]; exists {
		return runner.IsRunning()
	}
	return false
}

// GetScriptPath returns the default script path.
func (pm *PolicyManager) GetScriptPath() string {
	return pm.scriptPath
}

// Shutdown gracefully shuts down the policy manager and all running scripts.
func (pm *PolicyManager) Shutdown() {
	pm.cancel()

	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	// Stop all running scripts
	for path, runner := range pm.runners {
		if runner.IsRunning() {
			log.I.F("stopping policy script: %s", path)
			runner.Stop()
		}
		// Cancel the runner's context
		runner.cancel()
	}

	// Clear runners map
	pm.runners = make(map[string]*ScriptRunner)
}

// =============================================================================
// Policy Hot Reload Methods
// =============================================================================

// ValidateJSON validates policy JSON without applying changes.
// This is called BEFORE any modifications to ensure JSON is valid.
// Returns error if validation fails - no changes are made to current policy.
func (p *P) ValidateJSON(policyJSON []byte) error {
	// Try to unmarshal into a temporary policy struct
	tempPolicy := &P{}
	if err := json.Unmarshal(policyJSON, tempPolicy); err != nil {
		return fmt.Errorf("invalid JSON syntax: %v", err)
	}

	// Validate policy_admins are valid hex pubkeys (64 characters)
	for _, admin := range tempPolicy.PolicyAdmins {
		if len(admin) != 64 {
			return fmt.Errorf("invalid policy_admin pubkey length: %q (expected 64 hex characters)", admin)
		}
		if _, err := hex.Dec(admin); err != nil {
			return fmt.Errorf("invalid policy_admin pubkey format: %q: %v", admin, err)
		}
	}

	// Validate owners are valid hex pubkeys (64 characters)
	for _, owner := range tempPolicy.Owners {
		if len(owner) != 64 {
			return fmt.Errorf("invalid owner pubkey length: %q (expected 64 hex characters)", owner)
		}
		if _, err := hex.Dec(owner); err != nil {
			return fmt.Errorf("invalid owner pubkey format: %q: %v", owner, err)
		}
	}

	// Note: Owner-specific validation (non-empty owners) is done in ValidateOwnerPolicyUpdate

	// Validate regex patterns in tag_validation rules and new fields
	for kind, rule := range tempPolicy.rules {
		for tagName, pattern := range rule.TagValidation {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid regex pattern for tag %q in kind %d: %v", tagName, kind, err)
			}
		}
		// Validate IdentifierRegex pattern
		if rule.IdentifierRegex != "" {
			if _, err := regexp.Compile(rule.IdentifierRegex); err != nil {
				return fmt.Errorf("invalid identifier_regex pattern in kind %d: %v", kind, err)
			}
		}
		// Validate MaxExpiryDuration format
		if rule.MaxExpiryDuration != "" {
			if _, err := parseDuration(rule.MaxExpiryDuration); err != nil {
				return fmt.Errorf("invalid max_expiry_duration %q in kind %d: %v (format must be ISO-8601 duration, e.g. \"PT10M\" for 10 minutes, \"P7D\" for 7 days, \"P1DT12H\" for 1 day 12 hours)", rule.MaxExpiryDuration, kind, err)
			}
		}
		// Validate FollowsWhitelistAdmins pubkeys
		for _, admin := range rule.FollowsWhitelistAdmins {
			if len(admin) != 64 {
				return fmt.Errorf("invalid follows_whitelist_admins pubkey length in kind %d: %q (expected 64 hex characters)", kind, admin)
			}
			if _, err := hex.Dec(admin); err != nil {
				return fmt.Errorf("invalid follows_whitelist_admins pubkey format in kind %d: %q: %v", kind, admin, err)
			}
		}
	}

	// Validate global rule tag_validation patterns
	for tagName, pattern := range tempPolicy.Global.TagValidation {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern for tag %q in global rule: %v", tagName, err)
		}
	}

	// Validate global rule IdentifierRegex pattern
	if tempPolicy.Global.IdentifierRegex != "" {
		if _, err := regexp.Compile(tempPolicy.Global.IdentifierRegex); err != nil {
			return fmt.Errorf("invalid identifier_regex pattern in global rule: %v", err)
		}
	}

	// Validate global rule MaxExpiryDuration format
	if tempPolicy.Global.MaxExpiryDuration != "" {
		if _, err := parseDuration(tempPolicy.Global.MaxExpiryDuration); err != nil {
			return fmt.Errorf("invalid max_expiry_duration %q in global rule: %v (format must be ISO-8601 duration, e.g. \"PT10M\" for 10 minutes, \"P7D\" for 7 days, \"P1DT12H\" for 1 day 12 hours)", tempPolicy.Global.MaxExpiryDuration, err)
		}
	}

	// Validate global rule FollowsWhitelistAdmins pubkeys
	for _, admin := range tempPolicy.Global.FollowsWhitelistAdmins {
		if len(admin) != 64 {
			return fmt.Errorf("invalid follows_whitelist_admins pubkey length in global rule: %q (expected 64 hex characters)", admin)
		}
		if _, err := hex.Dec(admin); err != nil {
			return fmt.Errorf("invalid follows_whitelist_admins pubkey format in global rule: %q: %v", admin, err)
		}
	}

	// Validate default_policy value
	if tempPolicy.DefaultPolicy != "" && tempPolicy.DefaultPolicy != "allow" && tempPolicy.DefaultPolicy != "deny" {
		return fmt.Errorf("invalid default_policy value: %q (must be \"allow\" or \"deny\")", tempPolicy.DefaultPolicy)
	}

	// Validate permissive flags: if both read_allow_permissive AND write_allow_permissive are set
	// with a kind whitelist or blacklist, this makes the whitelist/blacklist meaningless
	hasKindRestriction := len(tempPolicy.Kind.Whitelist) > 0 || len(tempPolicy.Kind.Blacklist) > 0
	if hasKindRestriction && tempPolicy.Global.ReadAllowPermissive && tempPolicy.Global.WriteAllowPermissive {
		return fmt.Errorf("invalid policy: both read_allow_permissive and write_allow_permissive cannot be enabled together with a kind whitelist or blacklist (this would make the kind restriction meaningless)")
	}

	log.D.F("policy JSON validation passed")
	return nil
}

// Reload loads policy from JSON bytes and applies it to the existing policy instance.
// This validates JSON FIRST, then pauses the policy manager, updates configuration, and resumes.
// Returns error if validation fails - no changes are made on validation failure.
func (p *P) Reload(policyJSON []byte, configPath string) error {
	// Step 1: Validate JSON FIRST (before making any changes)
	if err := p.ValidateJSON(policyJSON); err != nil {
		return fmt.Errorf("validation failed: %v", err)
	}

	// Step 2: Pause policy manager (stop script runners)
	if err := p.Pause(); err != nil {
		log.W.F("failed to pause policy manager (continuing anyway): %v", err)
	}

	// Step 3: Unmarshal JSON into a temporary struct
	tempPolicy := &P{}
	if err := json.Unmarshal(policyJSON, tempPolicy); err != nil {
		// Resume before returning error
		p.Resume()
		return fmt.Errorf("failed to unmarshal policy JSON: %v", err)
	}

	// Step 4: Apply the new configuration (preserve manager reference)
	p.followsMx.Lock()
	p.Kind = tempPolicy.Kind
	p.rules = tempPolicy.rules
	p.Global = tempPolicy.Global
	p.DefaultPolicy = tempPolicy.DefaultPolicy
	p.PolicyAdmins = tempPolicy.PolicyAdmins
	p.PolicyFollowWhitelistEnabled = tempPolicy.PolicyFollowWhitelistEnabled
	p.Owners = tempPolicy.Owners
	p.policyAdminsBin = tempPolicy.policyAdminsBin
	p.ownersBin = tempPolicy.ownersBin
	// Note: policyFollows is NOT reset here - it will be refreshed separately
	p.followsMx.Unlock()

	// Step 5: Populate binary caches for all rules
	p.Global.populateBinaryCache()
	for kind := range p.rules {
		rule := p.rules[kind]
		rule.populateBinaryCache()
		p.rules[kind] = rule
	}

	// Step 6: Save to file (atomic write)
	if err := p.SaveToFile(configPath); err != nil {
		log.E.F("failed to persist policy to disk: %v (policy was updated in memory)", err)
		// Continue anyway - policy is loaded in memory
	}

	// Step 7: Resume policy manager (restart script runners)
	if err := p.Resume(); err != nil {
		log.W.F("failed to resume policy manager: %v", err)
	}

	log.I.F("policy configuration reloaded successfully")
	return nil
}

// Pause pauses the policy manager and stops all script runners.
func (p *P) Pause() error {
	if p.manager == nil {
		return fmt.Errorf("policy manager is not initialized")
	}

	p.manager.mutex.Lock()
	defer p.manager.mutex.Unlock()

	// Stop all running scripts
	for path, runner := range p.manager.runners {
		if runner.IsRunning() {
			log.I.F("pausing policy script: %s", path)
			if err := runner.Stop(); err != nil {
				log.W.F("failed to stop runner %s: %v", path, err)
			}
		}
	}

	log.I.F("policy manager paused")
	return nil
}

// Resume resumes the policy manager and restarts script runners.
func (p *P) Resume() error {
	if p.manager == nil {
		return fmt.Errorf("policy manager is not initialized")
	}

	// Restart the default policy script if it exists
	go p.manager.startPolicyIfExists()

	// Restart rule-specific scripts
	for _, rule := range p.rules {
		if rule.Script != "" {
			if _, err := os.Stat(rule.Script); err == nil {
				runner := p.manager.getOrCreateRunner(rule.Script)
				go func(r *ScriptRunner, script string) {
					if err := r.Start(); err != nil {
						log.W.F("failed to restart policy script %s: %v", script, err)
					}
				}(runner, rule.Script)
			}
		}
	}

	log.I.F("policy manager resumed")
	return nil
}

// SaveToFile persists the current policy configuration to disk using atomic write.
// Uses temp file + rename pattern to ensure atomic writes.
func (p *P) SaveToFile(configPath string) error {
	// Create shadow struct for JSON marshalling
	shadow := pJSON{
		Kind:                         p.Kind,
		Rules:                        p.rules,
		Global:                       p.Global,
		DefaultPolicy:                p.DefaultPolicy,
		PolicyAdmins:                 p.PolicyAdmins,
		PolicyFollowWhitelistEnabled: p.PolicyFollowWhitelistEnabled,
		Owners:                       p.Owners,
	}

	// Marshal to JSON with indentation for readability
	jsonData, err := json.MarshalIndent(shadow, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal policy to JSON: %v", err)
	}

	// Write to temp file first (atomic write pattern)
	tempPath := configPath + ".tmp"
	if err := os.WriteFile(tempPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %v", err)
	}

	// Rename temp file to actual config file (atomic on most filesystems)
	if err := os.Rename(tempPath, configPath); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %v", err)
	}

	log.I.F("policy configuration saved to %s", configPath)
	return nil
}

// =============================================================================
// Policy Admin and Follow Checking Methods
// =============================================================================

// IsPolicyAdmin checks if the given pubkey is in the policy_admins list.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
func (p *P) IsPolicyAdmin(pubkey []byte) bool {
	if len(pubkey) == 0 {
		return false
	}

	p.followsMx.RLock()
	defer p.followsMx.RUnlock()

	for _, admin := range p.policyAdminsBin {
		if utils.FastEqual(admin, pubkey) {
			return true
		}
	}
	return false
}

// IsPolicyFollow checks if the given pubkey is in the policy admin follows list.
// The pubkey parameter should be binary ([]byte), not hex-encoded.
func (p *P) IsPolicyFollow(pubkey []byte) bool {
	if len(pubkey) == 0 {
		return false
	}

	p.followsMx.RLock()
	defer p.followsMx.RUnlock()

	for _, follow := range p.policyFollows {
		if utils.FastEqual(pubkey, follow) {
			return true
		}
	}
	return false
}

// UpdatePolicyFollows replaces the policy follows list with a new set of pubkeys.
// This is called when policy admins update their follow lists (kind 3 events).
// The pubkeys should be binary ([]byte), not hex-encoded.
func (p *P) UpdatePolicyFollows(follows [][]byte) {
	p.followsMx.Lock()
	defer p.followsMx.Unlock()

	p.policyFollows = follows
	log.I.F("policy follows list updated with %d pubkeys", len(follows))
}

// GetPolicyAdminsBin returns a copy of the binary policy admin pubkeys.
// Used for checking if an event author is a policy admin.
func (p *P) GetPolicyAdminsBin() [][]byte {
	p.followsMx.RLock()
	defer p.followsMx.RUnlock()

	// Return a copy to prevent external modification
	result := make([][]byte, len(p.policyAdminsBin))
	for i, admin := range p.policyAdminsBin {
		adminCopy := make([]byte, len(admin))
		copy(adminCopy, admin)
		result[i] = adminCopy
	}
	return result
}

// GetOwnersBin returns a copy of the binary owner pubkeys defined in the policy.
// These are merged with environment-defined owners by the application layer.
// Useful for cloud deployments where environment variables cannot be modified.
func (p *P) GetOwnersBin() [][]byte {
	if p == nil {
		return nil
	}

	p.followsMx.RLock()
	defer p.followsMx.RUnlock()

	// Return a copy to prevent external modification
	result := make([][]byte, len(p.ownersBin))
	for i, owner := range p.ownersBin {
		ownerCopy := make([]byte, len(owner))
		copy(ownerCopy, owner)
		result[i] = ownerCopy
	}
	return result
}

// GetOwners returns the hex-encoded owner pubkeys defined in the policy.
// These are merged with environment-defined owners by the application layer.
func (p *P) GetOwners() []string {
	if p == nil {
		return nil
	}
	return p.Owners
}

// IsPolicyFollowWhitelistEnabled returns whether the policy follow whitelist feature is enabled.
// When enabled, pubkeys followed by policy admins are automatically whitelisted for access
// when rules have WriteAllowFollows=true.
func (p *P) IsPolicyFollowWhitelistEnabled() bool {
	if p == nil {
		return false
	}
	return p.PolicyFollowWhitelistEnabled
}

// =============================================================================
// FollowsWhitelistAdmins Methods
// =============================================================================

// GetAllFollowsWhitelistAdmins returns all unique admin pubkeys from FollowsWhitelistAdmins
// across all rules (including global). Returns hex-encoded pubkeys.
// This is used at startup to validate that kind 3 events exist for these admins.
func (p *P) GetAllFollowsWhitelistAdmins() []string {
	if p == nil {
		return nil
	}

	// Use map to deduplicate
	admins := make(map[string]struct{})

	// Check global rule
	for _, admin := range p.Global.FollowsWhitelistAdmins {
		admins[admin] = struct{}{}
	}

	// Check all kind-specific rules
	for _, rule := range p.rules {
		for _, admin := range rule.FollowsWhitelistAdmins {
			admins[admin] = struct{}{}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(admins))
	for admin := range admins {
		result = append(result, admin)
	}
	return result
}

// GetRuleForKind returns the Rule for a specific kind, or nil if no rule exists.
// This allows external code to access and modify rule-specific follows whitelists.
func (p *P) GetRuleForKind(kind int) *Rule {
	if p == nil || p.rules == nil {
		return nil
	}
	if rule, exists := p.rules[kind]; exists {
		return &rule
	}
	return nil
}

// UpdateRuleFollowsWhitelist updates the follows whitelist for a specific kind's rule.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
// Thread-safe: uses followsMx to protect concurrent access.
func (p *P) UpdateRuleFollowsWhitelist(kind int, follows [][]byte) {
	if p == nil || p.rules == nil {
		return
	}
	p.followsMx.Lock()
	defer p.followsMx.Unlock()
	if rule, exists := p.rules[kind]; exists {
		rule.UpdateFollowsWhitelist(follows)
		p.rules[kind] = rule
	}
}

// UpdateGlobalFollowsWhitelist updates the follows whitelist for the global rule.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
// Note: We directly modify p.Global's unexported field because Global is a value type (not *Rule),
// so calling p.Global.UpdateFollowsWhitelist() would operate on a copy and discard changes.
// Thread-safe: uses followsMx to protect concurrent access.
func (p *P) UpdateGlobalFollowsWhitelist(follows [][]byte) {
	if p == nil {
		return
	}
	p.followsMx.Lock()
	defer p.followsMx.Unlock()
	p.Global.followsWhitelistFollowsBin = follows
}

// GetGlobalRule returns a pointer to the global rule for modification.
func (p *P) GetGlobalRule() *Rule {
	if p == nil {
		return nil
	}
	return &p.Global
}

// GetRules returns the rules map for iteration.
// Note: Returns a copy of the map keys to prevent modification.
func (p *P) GetRulesKinds() []int {
	if p == nil || p.rules == nil {
		return nil
	}
	kinds := make([]int, 0, len(p.rules))
	for kind := range p.rules {
		kinds = append(kinds, kind)
	}
	return kinds
}

// =============================================================================
// ReadFollowsWhitelist and WriteFollowsWhitelist Methods
// =============================================================================

// GetAllReadFollowsWhitelistPubkeys returns all unique pubkeys from ReadFollowsWhitelist
// across all rules (including global). Returns hex-encoded pubkeys.
// This is used at startup to validate that kind 3 events exist for these pubkeys.
func (p *P) GetAllReadFollowsWhitelistPubkeys() []string {
	if p == nil {
		return nil
	}

	// Use map to deduplicate
	pubkeys := make(map[string]struct{})

	// Check global rule
	for _, pk := range p.Global.ReadFollowsWhitelist {
		pubkeys[pk] = struct{}{}
	}

	// Check all kind-specific rules
	for _, rule := range p.rules {
		for _, pk := range rule.ReadFollowsWhitelist {
			pubkeys[pk] = struct{}{}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(pubkeys))
	for pk := range pubkeys {
		result = append(result, pk)
	}
	return result
}

// GetAllWriteFollowsWhitelistPubkeys returns all unique pubkeys from WriteFollowsWhitelist
// across all rules (including global). Returns hex-encoded pubkeys.
// This is used at startup to validate that kind 3 events exist for these pubkeys.
func (p *P) GetAllWriteFollowsWhitelistPubkeys() []string {
	if p == nil {
		return nil
	}

	// Use map to deduplicate
	pubkeys := make(map[string]struct{})

	// Check global rule
	for _, pk := range p.Global.WriteFollowsWhitelist {
		pubkeys[pk] = struct{}{}
	}

	// Check all kind-specific rules
	for _, rule := range p.rules {
		for _, pk := range rule.WriteFollowsWhitelist {
			pubkeys[pk] = struct{}{}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(pubkeys))
	for pk := range pubkeys {
		result = append(result, pk)
	}
	return result
}

// GetAllFollowsWhitelistPubkeys returns all unique pubkeys from both ReadFollowsWhitelist
// and WriteFollowsWhitelist across all rules (including global). Returns hex-encoded pubkeys.
// This is a convenience method for startup validation to check all required kind 3 events.
func (p *P) GetAllFollowsWhitelistPubkeys() []string {
	if p == nil {
		return nil
	}

	// Use map to deduplicate
	pubkeys := make(map[string]struct{})

	// Get read follows whitelist pubkeys
	for _, pk := range p.GetAllReadFollowsWhitelistPubkeys() {
		pubkeys[pk] = struct{}{}
	}

	// Get write follows whitelist pubkeys
	for _, pk := range p.GetAllWriteFollowsWhitelistPubkeys() {
		pubkeys[pk] = struct{}{}
	}

	// Also include deprecated FollowsWhitelistAdmins for backward compatibility
	for _, pk := range p.GetAllFollowsWhitelistAdmins() {
		pubkeys[pk] = struct{}{}
	}

	// Convert map to slice
	result := make([]string, 0, len(pubkeys))
	for pk := range pubkeys {
		result = append(result, pk)
	}
	return result
}

// UpdateRuleReadFollowsWhitelist updates the read follows whitelist for a specific kind's rule.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
// Thread-safe: uses followsMx to protect concurrent access.
func (p *P) UpdateRuleReadFollowsWhitelist(kind int, follows [][]byte) {
	if p == nil || p.rules == nil {
		return
	}
	p.followsMx.Lock()
	defer p.followsMx.Unlock()
	if rule, exists := p.rules[kind]; exists {
		rule.UpdateReadFollowsWhitelist(follows)
		p.rules[kind] = rule
	}
}

// UpdateRuleWriteFollowsWhitelist updates the write follows whitelist for a specific kind's rule.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
// Thread-safe: uses followsMx to protect concurrent access.
func (p *P) UpdateRuleWriteFollowsWhitelist(kind int, follows [][]byte) {
	if p == nil || p.rules == nil {
		return
	}
	p.followsMx.Lock()
	defer p.followsMx.Unlock()
	if rule, exists := p.rules[kind]; exists {
		rule.UpdateWriteFollowsWhitelist(follows)
		p.rules[kind] = rule
	}
}

// UpdateGlobalReadFollowsWhitelist updates the read follows whitelist for the global rule.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
// Note: We directly modify p.Global's unexported field because Global is a value type (not *Rule),
// so calling p.Global.UpdateReadFollowsWhitelist() would operate on a copy and discard changes.
// Thread-safe: uses followsMx to protect concurrent access.
func (p *P) UpdateGlobalReadFollowsWhitelist(follows [][]byte) {
	if p == nil {
		return
	}
	p.followsMx.Lock()
	defer p.followsMx.Unlock()
	p.Global.readFollowsFollowsBin = follows
}

// UpdateGlobalWriteFollowsWhitelist updates the write follows whitelist for the global rule.
// The follows should be binary pubkeys ([]byte), not hex-encoded.
// Note: We directly modify p.Global's unexported field because Global is a value type (not *Rule),
// so calling p.Global.UpdateWriteFollowsWhitelist() would operate on a copy and discard changes.
// Thread-safe: uses followsMx to protect concurrent access.
func (p *P) UpdateGlobalWriteFollowsWhitelist(follows [][]byte) {
	if p == nil {
		return
	}
	p.followsMx.Lock()
	defer p.followsMx.Unlock()
	p.Global.writeFollowsFollowsBin = follows
}

// =============================================================================
// Owner vs Policy Admin Update Validation
// =============================================================================

// ValidateOwnerPolicyUpdate validates a full policy update from an owner.
// Owners can modify all fields but the owners list must be non-empty.
func (p *P) ValidateOwnerPolicyUpdate(policyJSON []byte) error {
	// First run standard validation
	if err := p.ValidateJSON(policyJSON); err != nil {
		return err
	}

	// Parse the new policy
	tempPolicy := &P{}
	if err := json.Unmarshal(policyJSON, tempPolicy); err != nil {
		return fmt.Errorf("failed to parse policy JSON: %v", err)
	}

	// Owner-specific validation: owners list cannot be empty
	if len(tempPolicy.Owners) == 0 {
		return fmt.Errorf("owners list cannot be empty: at least one owner must be defined to prevent lockout")
	}

	return nil
}

// ValidatePolicyAdminUpdate validates a policy update from a policy admin.
// Policy admins CANNOT modify: owners, policy_admins
// Policy admins CAN: extend rules, add blacklists, add new kind rules
func (p *P) ValidatePolicyAdminUpdate(policyJSON []byte, adminPubkey []byte) error {
	// First run standard validation
	if err := p.ValidateJSON(policyJSON); err != nil {
		return err
	}

	// Parse the new policy
	tempPolicy := &P{}
	if err := json.Unmarshal(policyJSON, tempPolicy); err != nil {
		return fmt.Errorf("failed to parse policy JSON: %v", err)
	}

	// Protected field check: owners must match current
	if !stringSliceEqual(tempPolicy.Owners, p.Owners) {
		return fmt.Errorf("policy admins cannot modify the 'owners' field: this is a protected field that only owners can change")
	}

	// Protected field check: policy_admins must match current
	if !stringSliceEqual(tempPolicy.PolicyAdmins, p.PolicyAdmins) {
		return fmt.Errorf("policy admins cannot modify the 'policy_admins' field: this is a protected field that only owners can change")
	}

	// Validate that the admin is not reducing owner-granted permissions
	// This check ensures policy admins can only extend, not restrict
	if err := p.validateNoPermissionReduction(tempPolicy); err != nil {
		return fmt.Errorf("policy admins cannot reduce owner-granted permissions: %v", err)
	}

	return nil
}

// validateNoPermissionReduction checks that the new policy doesn't reduce
// permissions that were granted in the current (owner) policy.
//
// Policy admins CAN:
//   - ADD to allow lists (write_allow, read_allow)
//   - ADD to deny lists (write_deny, read_deny) to blacklist non-admin users
//   - INCREASE limits (size_limit, content_limit, max_age_of_event)
//   - ADD new kinds to whitelist or blacklist
//   - ADD new rules for kinds not defined by owner
//
// Policy admins CANNOT:
//   - REMOVE from allow lists
//   - DECREASE limits
//   - REMOVE kinds from whitelist
//   - REMOVE rules defined by owner
//   - ADD new required tags (restrictions)
//   - BLACKLIST owners or other policy admins
func (p *P) validateNoPermissionReduction(newPolicy *P) error {
	// Check kind whitelist - new policy must include all current whitelisted kinds
	for _, kind := range p.Kind.Whitelist {
		found := false
		for _, newKind := range newPolicy.Kind.Whitelist {
			if kind == newKind {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("cannot remove kind %d from whitelist", kind)
		}
	}

	// Check each rule in the current policy
	for kind, currentRule := range p.rules {
		newRule, exists := newPolicy.rules[kind]
		if !exists {
			return fmt.Errorf("cannot remove rule for kind %d", kind)
		}

		// Check write_allow - new rule must include all current pubkeys
		for _, pk := range currentRule.WriteAllow {
			if !containsString(newRule.WriteAllow, pk) {
				return fmt.Errorf("cannot remove pubkey %s from write_allow for kind %d", pk, kind)
			}
		}

		// Check read_allow - new rule must include all current pubkeys
		for _, pk := range currentRule.ReadAllow {
			if !containsString(newRule.ReadAllow, pk) {
				return fmt.Errorf("cannot remove pubkey %s from read_allow for kind %d", pk, kind)
			}
		}

		// Check write_deny - cannot blacklist owners or policy admins
		for _, pk := range newRule.WriteDeny {
			if containsString(p.Owners, pk) {
				return fmt.Errorf("cannot blacklist owner %s in write_deny for kind %d", pk, kind)
			}
			if containsString(p.PolicyAdmins, pk) {
				return fmt.Errorf("cannot blacklist policy admin %s in write_deny for kind %d", pk, kind)
			}
		}

		// Check read_deny - cannot blacklist owners or policy admins
		for _, pk := range newRule.ReadDeny {
			if containsString(p.Owners, pk) {
				return fmt.Errorf("cannot blacklist owner %s in read_deny for kind %d", pk, kind)
			}
			if containsString(p.PolicyAdmins, pk) {
				return fmt.Errorf("cannot blacklist policy admin %s in read_deny for kind %d", pk, kind)
			}
		}

		// Check size limits - new limit cannot be smaller
		if currentRule.SizeLimit != nil && newRule.SizeLimit != nil {
			if *newRule.SizeLimit < *currentRule.SizeLimit {
				return fmt.Errorf("cannot reduce size_limit for kind %d from %d to %d", kind, *currentRule.SizeLimit, *newRule.SizeLimit)
			}
		}

		// Check content limits - new limit cannot be smaller
		if currentRule.ContentLimit != nil && newRule.ContentLimit != nil {
			if *newRule.ContentLimit < *currentRule.ContentLimit {
				return fmt.Errorf("cannot reduce content_limit for kind %d from %d to %d", kind, *currentRule.ContentLimit, *newRule.ContentLimit)
			}
		}

		// Check max_age_of_event - new limit cannot be smaller (smaller = more restrictive)
		if currentRule.MaxAgeOfEvent != nil && newRule.MaxAgeOfEvent != nil {
			if *newRule.MaxAgeOfEvent < *currentRule.MaxAgeOfEvent {
				return fmt.Errorf("cannot reduce max_age_of_event for kind %d from %d to %d", kind, *currentRule.MaxAgeOfEvent, *newRule.MaxAgeOfEvent)
			}
		}

		// Check must_have_tags - cannot add new required tags (more restrictive)
		for _, tag := range newRule.MustHaveTags {
			found := false
			for _, currentTag := range currentRule.MustHaveTags {
				if tag == currentTag {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("cannot add required tag %q for kind %d (only owners can add restrictions)", tag, kind)
			}
		}
	}

	// Check global rule write_deny - cannot blacklist owners or policy admins
	for _, pk := range newPolicy.Global.WriteDeny {
		if containsString(p.Owners, pk) {
			return fmt.Errorf("cannot blacklist owner %s in global write_deny", pk)
		}
		if containsString(p.PolicyAdmins, pk) {
			return fmt.Errorf("cannot blacklist policy admin %s in global write_deny", pk)
		}
	}

	// Check global rule read_deny - cannot blacklist owners or policy admins
	for _, pk := range newPolicy.Global.ReadDeny {
		if containsString(p.Owners, pk) {
			return fmt.Errorf("cannot blacklist owner %s in global read_deny", pk)
		}
		if containsString(p.PolicyAdmins, pk) {
			return fmt.Errorf("cannot blacklist policy admin %s in global read_deny", pk)
		}
	}

	// Check global rule size limits
	if p.Global.SizeLimit != nil && newPolicy.Global.SizeLimit != nil {
		if *newPolicy.Global.SizeLimit < *p.Global.SizeLimit {
			return fmt.Errorf("cannot reduce global size_limit from %d to %d", *p.Global.SizeLimit, *newPolicy.Global.SizeLimit)
		}
	}

	return nil
}

// ReloadAsOwner reloads the policy from an owner's kind 12345 event.
// Owners can modify all fields but the owners list must be non-empty.
func (p *P) ReloadAsOwner(policyJSON []byte, configPath string) error {
	// Validate as owner update
	if err := p.ValidateOwnerPolicyUpdate(policyJSON); err != nil {
		return fmt.Errorf("owner policy validation failed: %v", err)
	}

	// Use existing Reload logic
	return p.Reload(policyJSON, configPath)
}

// ReloadAsPolicyAdmin reloads the policy from a policy admin's kind 12345 event.
// Policy admins cannot modify protected fields (owners, policy_admins) and
// cannot reduce owner-granted permissions.
func (p *P) ReloadAsPolicyAdmin(policyJSON []byte, configPath string, adminPubkey []byte) error {
	// Validate as policy admin update
	if err := p.ValidatePolicyAdminUpdate(policyJSON, adminPubkey); err != nil {
		return fmt.Errorf("policy admin validation failed: %v", err)
	}

	// Use existing Reload logic
	return p.Reload(policyJSON, configPath)
}

// stringSliceEqual checks if two string slices are equal (order-independent).
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps for comparison
	aMap := make(map[string]int)
	for _, v := range a {
		aMap[v]++
	}

	bMap := make(map[string]int)
	for _, v := range b {
		bMap[v]++
	}

	// Compare maps
	for k, v := range aMap {
		if bMap[k] != v {
			return false
		}
	}

	return true
}
