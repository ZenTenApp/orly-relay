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
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/utils"
)

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
type Rule struct {
	// Description is a human-readable description of the rule.
	Description string `json:"description"`
	// Script is a path to a script that will be used to determine if the event should be allowed to be written to the relay. The script should be a standard bash script or whatever is native to the platform. The script will return its opinion to be one of the criteria that must be met for the event to be allowed to be written to the relay (AND).
	Script string `json:"script,omitempty"`
	// WriteAllow is a list of pubkeys that are allowed to write this event kind to the relay. If any are present, implicitly all others are denied.
	WriteAllow []string `json:"write_allow,omitempty"`
	// WriteDeny is a list of pubkeys that are not allowed to write this event kind to the relay. If any are present, implicitly all others are allowed. Only takes effect in the absence of a WriteAllow.
	WriteDeny []string `json:"write_deny,omitempty"`
	// ReadAllow is a list of pubkeys that are allowed to read this event kind from the relay. If any are present, implicitly all others are denied.
	ReadAllow []string `json:"read_allow,omitempty"`
	// ReadDeny is a list of pubkeys that are not allowed to read this event kind from the relay. If any are present, implicitly all others are allowed. Only takes effect in the absence of a ReadAllow.
	ReadDeny []string `json:"read_deny,omitempty"`
	// MaxExpiry is the maximum expiry time in seconds for events written to the relay. If 0, there is no maximum expiry. Events must have an expiry time if this is set, and it must be no more than this value in the future compared to the event's created_at time.
	MaxExpiry *int64 `json:"max_expiry,omitempty"`
	// MustHaveTags is a list of tag key letters that must be present on the event for it to be allowed to be written to the relay.
	MustHaveTags []string `json:"must_have_tags,omitempty"`
	// SizeLimit is the maximum size in bytes for the event's total serialized size.
	SizeLimit *int64 `json:"size_limit,omitempty"`
	// ContentLimit is the maximum size in bytes for the event's content field.
	ContentLimit *int64 `json:"content_limit,omitempty"`
	// Privileged means that this event is either authored by the authenticated pubkey, or has a p tag that contains the authenticated pubkey. This type of event is only sent to users who are authenticated and are party to the event.
	Privileged bool `json:"privileged,omitempty"`
	// RateLimit is the amount of data can be written to the relay per second by the authenticated pubkey. If 0, there is no rate limit. This is applied via the use of an EWMA of the event publication history on the authenticated connection
	RateLimit *int64 `json:"rate_limit,omitempty"`
	// MaxAgeOfEvent is the offset in seconds that is the oldest timestamp allowed for an event's created_at time. If 0, there is no maximum age. Events must have a created_at time if this is set, and it must be no more than this value in the past compared to the current time.
	MaxAgeOfEvent *int64 `json:"max_age_of_event,omitempty"`
	// MaxAgeEventInFuture is the offset in seconds that is the newest timestamp allowed for an event's created_at time ahead of the current time.
	MaxAgeEventInFuture *int64 `json:"max_age_event_in_future,omitempty"`

	// WriteAllowFollows grants BOTH read and write access to policy admin follows when enabled.
	// Requires PolicyFollowWhitelistEnabled=true at the policy level.
	WriteAllowFollows bool `json:"write_allow_follows,omitempty"`

	// TagValidation is a map of tag_name -> regex pattern for validating tag values.
	// Each tag present in the event must match its corresponding regex pattern.
	// Example: {"d": "^[a-z0-9-]{1,64}$", "t": "^[a-z0-9-]{1,32}$"}
	TagValidation map[string]string `json:"tag_validation,omitempty"`

	// Binary caches for faster comparison (populated from hex strings above)
	// These are not exported and not serialized to JSON
	writeAllowBin [][]byte
	writeDenyBin  [][]byte
	readAllowBin  [][]byte
	readDenyBin   [][]byte
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
		r.MaxExpiry != nil || len(r.MustHaveTags) > 0 ||
		r.Script != "" || r.Privileged ||
		r.WriteAllowFollows || len(r.TagValidation) > 0
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

	return err
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
	scriptPath string // Default script path for backward compatibility
	enabled    bool
	mutex      sync.RWMutex
	runners    map[string]*ScriptRunner // Map of script path -> runner
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

	// Unexported binary caches for faster comparison (populated from hex strings above)
	policyAdminsBin [][]byte     // Binary cache for policy admin pubkeys
	policyFollows   [][]byte     // Cached follow list from policy admins (kind 3 events)
	policyFollowsMx sync.RWMutex // Protect follows list access

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

	return nil
}

// New creates a new policy from JSON configuration.
// If policyJSON is empty, returns a policy with default settings.
// The default_policy field defaults to "allow" if not specified.
func New(policyJSON []byte) (p *P, err error) {
	p = &P{
		DefaultPolicy: "allow", // Set default value
	}
	if len(policyJSON) > 0 {
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
		rule := p.rules[kind]  // Get a copy
		rule.populateBinaryCache()
		p.rules[kind] = rule  // Store the modified copy back
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
func NewWithManager(ctx context.Context, appName string, enabled bool) *P {
	configDir := filepath.Join(xdg.ConfigHome, appName)
	scriptPath := filepath.Join(configDir, "policy.sh")
	configPath := filepath.Join(configDir, "policy.json")

	ctx, cancel := context.WithCancel(ctx)

	manager := &PolicyManager{
		ctx:        ctx,
		cancel:     cancel,
		configDir:  configDir,
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
func (sr *ScriptRunner) logOutput(stdout, stderr io.ReadCloser) {
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
// Policy evaluation order: global rules → kind filtering → specific rules → default policy.
func (p *P) CheckPolicy(
	access string, ev *event.E, loggedInPubkey []byte, ipAddress string,
) (allowed bool, err error) {
	// Handle nil event
	if ev == nil {
		return false, fmt.Errorf("event cannot be nil")
	}

	// First check global rule filter (applies to all events)
	if !p.checkGlobalRulePolicy(access, ev, loggedInPubkey) {
		return false, nil
	}

	// Then check kinds white/blacklist
	if !p.checkKindsPolicy(ev.Kind) {
		return false, nil
	}

	// Get rule for this kind
	rule, hasRule := p.rules[int(ev.Kind)]
	if !hasRule {
		// No specific rule for this kind, use default policy
		return p.getDefaultPolicyAction(), nil
	}

	// Check if script is present and enabled
	if rule.Script != "" && p.manager != nil {
		if p.manager.IsEnabled() {
			// Check if script file exists before trying to use it
			if _, err := os.Stat(rule.Script); err == nil {
				// Script exists, try to use it
				log.D.F(
					"using policy script for kind %d: %s", ev.Kind, rule.Script,
				)
				allowed, err := p.checkScriptPolicy(
					access, ev, rule.Script, loggedInPubkey, ipAddress,
				)
				if err == nil {
					// Script ran successfully, return its decision
					return allowed, nil
				}
				// Script failed, fall through to apply other criteria
				log.W.F(
					"policy script check failed for kind %d: %v, applying other criteria",
					ev.Kind, err,
				)
			} else {
				// Script configured but doesn't exist
				log.W.F(
					"policy script configured for kind %d but not found at %s: %v, applying other criteria",
					ev.Kind, rule.Script, err,
				)
			}
			// Script doesn't exist or failed, fall through to apply other criteria
		} else {
			// Policy manager is disabled, fall back to default policy
			log.D.F(
				"policy manager is disabled for kind %d, falling back to default policy (%s)",
				ev.Kind, p.DefaultPolicy,
			)
			return p.getDefaultPolicyAction(), nil
		}
	}

	// Apply rule-based filtering
	return p.checkRulePolicy(access, ev, rule, loggedInPubkey)
}

// checkKindsPolicy checks if the event kind is allowed.
// Logic:
// 1. If explicit whitelist exists, use it (backwards compatibility)
// 2. If explicit blacklist exists, use it (backwards compatibility)
// 3. Otherwise, kinds with defined rules are implicitly allowed, others denied
func (p *P) checkKindsPolicy(kind uint16) bool {
	// If whitelist is present, only allow whitelisted kinds
	if len(p.Kind.Whitelist) > 0 {
		for _, allowedKind := range p.Kind.Whitelist {
			if kind == uint16(allowedKind) {
				return true
			}
		}
		return false
	}

	// If blacklist is present, deny blacklisted kinds
	if len(p.Kind.Blacklist) > 0 {
		for _, deniedKind := range p.Kind.Blacklist {
			if kind == uint16(deniedKind) {
				return false
			}
		}
		// Not in blacklist - check if rule exists for implicit whitelist
		_, hasRule := p.rules[int(kind)]
		return hasRule // Only allow if there's a rule defined
	}

	// No explicit whitelist or blacklist
	// If there are specific rules defined, use implicit whitelist
	// If there's only a global rule (no specific rules), fall back to default policy
	// If there are NO rules at all, fall back to default policy
	if len(p.rules) > 0 {
		// Implicit whitelist mode - only allow kinds with specific rules
		_, hasRule := p.rules[int(kind)]
		return hasRule
	}
	// No specific rules (maybe global rule exists) - fall back to default policy
	return p.getDefaultPolicyAction()
}

// checkGlobalRulePolicy checks if the event passes the global rule filter
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

// checkRulePolicy evaluates rule-based access control with corrected evaluation order.
// Evaluation order:
// 1. Universal constraints (size, tags, age) - apply to everyone
// 2. Explicit denials (deny lists) - highest priority blacklist
// 3. Privileged access - parties involved get special access (ONLY if no allow lists)
// 4. Explicit allows (allow lists) - exclusive and authoritative when present
// 5. Default policy - fallback when no rules apply
//
// IMPORTANT: When both privileged AND allow lists are specified, allow lists are
// authoritative - even parties involved must be in the allow list.
func (p *P) checkRulePolicy(
	access string, ev *event.E, rule Rule, loggedInPubkey []byte,
) (allowed bool, err error) {
	// ===================================================================
	// STEP 1: Universal Constraints (apply to everyone)
	// ===================================================================

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

	// Check expiry time
	if rule.MaxExpiry != nil {
		expiryTag := ev.Tags.GetFirst([]byte("expiration"))
		if expiryTag == nil {
			return false, nil // Must have expiry if MaxExpiry is set
		}
		// TODO: Parse and validate expiry time
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
	// Only apply for write access - we validate what goes in, not what comes out
	if access == "write" && len(rule.TagValidation) > 0 {
		for tagName, regexPattern := range rule.TagValidation {
			// Compile regex pattern (errors should have been caught in ValidateJSON)
			regex, compileErr := regexp.Compile(regexPattern)
			if compileErr != nil {
				log.E.F("invalid regex pattern for tag %q: %v (skipping validation)", tagName, compileErr)
				continue
			}

			// Get all tags with this name
			tags := ev.Tags.GetAll([]byte(tagName))

			// If no tags found and rule requires this tag, validation fails
			if len(tags) == 0 {
				log.D.F("tag validation failed: required tag %q not found", tagName)
				return false, nil
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
	// STEP 2.5: Write Allow Follows (grants BOTH read AND write access)
	// ===================================================================

	// WriteAllowFollows grants both read and write access to policy admin follows
	// This check applies to BOTH read and write access types
	if rule.WriteAllowFollows && p.PolicyFollowWhitelistEnabled {
		if p.IsPolicyFollow(loggedInPubkey) {
			log.D.F("policy admin follow granted %s access for kind %d", access, ev.Kind)
			return true, nil // Allow access from policy admin follow
		}
	}

	// ===================================================================
	// STEP 3: Check Read Access with OR Logic (Allow List OR Privileged)
	// ===================================================================

	// For read operations, check if user has access via allow list OR privileged
	if access == "read" {
		hasAllowList := len(rule.readAllowBin) > 0 || len(rule.ReadAllow) > 0
		userInAllowList := false
		userIsPrivileged := rule.Privileged && IsPartyInvolved(ev, loggedInPubkey)

		// Check if user is in read allow list
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

		// Handle different cases:
		// 1. If there's an allow list: use OR logic (in list OR privileged)
		// 2. If no allow list but privileged: only involved parties allowed
		// 3. If no allow list and not privileged: continue to other checks

		if hasAllowList {
			// OR logic when allow list exists
			if userInAllowList || userIsPrivileged {
				return true, nil
			}
			// Not in allow list AND not privileged -> deny
			return false, nil
		} else if rule.Privileged {
			// No allow list but privileged -> only involved parties
			if userIsPrivileged {
				return true, nil
			}
			// Not involved in privileged event -> deny
			return false, nil
		}
		// No allow list and not privileged -> continue to other checks
	}

	// ===================================================================
	// STEP 4: Explicit Allows (exclusive access - ONLY these users)
	// ===================================================================

	if access == "write" {
		// Check write allow list (exclusive - ONLY these users can write)
		// Special case: empty list (but not nil) means allow all
		if rule.WriteAllow != nil && len(rule.WriteAllow) == 0 && len(rule.writeAllowBin) == 0 {
			// Empty allow list explicitly set - allow all writers
			return true, nil
		}

		if len(rule.writeAllowBin) > 0 {
			// Check if logged-in user (submitter) is allowed to write
			allowed = false
			for _, allowedPubkey := range rule.writeAllowBin {
				if utils.FastEqual(loggedInPubkey, allowedPubkey) {
					allowed = true
					break
				}
			}
			if !allowed {
				return false, nil // Submitter not in exclusive allow list
			}
			// Submitter is in allow list
			return true, nil
		} else if len(rule.WriteAllow) > 0 {
			// Fallback: binary cache not populated, use hex comparison
			// Check if logged-in user (submitter) is allowed to write
			loggedInPubkeyHex := hex.Enc(loggedInPubkey)
			allowed = false
			for _, allowedPubkey := range rule.WriteAllow {
				if loggedInPubkeyHex == allowedPubkey {
					allowed = true
					break
				}
			}
			if !allowed {
				return false, nil // Submitter not in exclusive allow list
			}
			// Submitter is in allow list
			return true, nil
		}

		// If we have ONLY a deny list (no allow list), and user is not denied, allow
		if (len(rule.WriteDeny) > 0 || len(rule.writeDenyBin) > 0) &&
			len(rule.WriteAllow) == 0 && len(rule.writeAllowBin) == 0 {
			// Only deny list exists, user wasn't denied above, so allow
			return true, nil
		}
	} else if access == "read" {
		// Read access already handled in STEP 3 with OR logic (allow list OR privileged)
		// Only need to handle special cases here

		// Special case: empty list (but not nil) means allow all
		// BUT if privileged, still need to check if user is involved
		if rule.ReadAllow != nil && len(rule.ReadAllow) == 0 && len(rule.readAllowBin) == 0 {
			if rule.Privileged {
				// Empty allow list with privileged - only involved parties
				return IsPartyInvolved(ev, loggedInPubkey), nil
			}
			// Empty allow list without privileged - allow all readers
			return true, nil
		}

		// If we have ONLY a deny list (no allow list), and user is not denied, allow
		if (len(rule.ReadDeny) > 0 || len(rule.readDenyBin) > 0) &&
			len(rule.ReadAllow) == 0 && len(rule.readAllowBin) == 0 {
			// Only deny list exists, user wasn't denied above, so allow
			return true, nil
		}
	}

	// ===================================================================
	// STEP 5: No Additional Privileged Check Needed
	// ===================================================================

	// Privileged access for read operations is already handled in STEP 3 with OR logic
	// No additional check needed here

	// ===================================================================
	// STEP 6: Default Policy
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

	// Validate regex patterns in tag_validation rules
	for kind, rule := range tempPolicy.rules {
		for tagName, pattern := range rule.TagValidation {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid regex pattern for tag %q in kind %d: %v", tagName, kind, err)
			}
		}
	}

	// Validate global rule tag_validation patterns
	for tagName, pattern := range tempPolicy.Global.TagValidation {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern for tag %q in global rule: %v", tagName, err)
		}
	}

	// Validate default_policy value
	if tempPolicy.DefaultPolicy != "" && tempPolicy.DefaultPolicy != "allow" && tempPolicy.DefaultPolicy != "deny" {
		return fmt.Errorf("invalid default_policy value: %q (must be \"allow\" or \"deny\")", tempPolicy.DefaultPolicy)
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
	p.policyFollowsMx.Lock()
	p.Kind = tempPolicy.Kind
	p.rules = tempPolicy.rules
	p.Global = tempPolicy.Global
	p.DefaultPolicy = tempPolicy.DefaultPolicy
	p.PolicyAdmins = tempPolicy.PolicyAdmins
	p.PolicyFollowWhitelistEnabled = tempPolicy.PolicyFollowWhitelistEnabled
	p.policyAdminsBin = tempPolicy.policyAdminsBin
	// Note: policyFollows is NOT reset here - it will be refreshed separately
	p.policyFollowsMx.Unlock()

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

	p.policyFollowsMx.RLock()
	defer p.policyFollowsMx.RUnlock()

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

	p.policyFollowsMx.RLock()
	defer p.policyFollowsMx.RUnlock()

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
	p.policyFollowsMx.Lock()
	defer p.policyFollowsMx.Unlock()

	p.policyFollows = follows
	log.I.F("policy follows list updated with %d pubkeys", len(follows))
}

// GetPolicyAdminsBin returns a copy of the binary policy admin pubkeys.
// Used for checking if an event author is a policy admin.
func (p *P) GetPolicyAdminsBin() [][]byte {
	p.policyFollowsMx.RLock()
	defer p.policyFollowsMx.RUnlock()

	// Return a copy to prevent external modification
	result := make([][]byte, len(p.policyAdminsBin))
	for i, admin := range p.policyAdminsBin {
		adminCopy := make([]byte, len(admin))
		copy(adminCopy, admin)
		result[i] = adminCopy
	}
	return result
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
