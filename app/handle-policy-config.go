package app

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/adrg/xdg"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// HandlePolicyConfigUpdate processes kind 12345 policy configuration events.
// Only policy admins can update policy configuration.
//
// Process flow:
// 1. Verify sender is policy admin (from current policy.policy_admins list)
// 2. Parse and validate JSON FIRST (before making any changes)
// 3. Pause ALL message processing (lock mutex)
// 4. Reload policy (pause policy engine, update, save, resume)
// 5. Resume message processing (unlock mutex)
//
// The message processing mutex is already released by the caller (HandleEvent),
// so we acquire it ourselves for the critical section.
func (l *Listener) HandlePolicyConfigUpdate(ev *event.E) error {
	log.I.F("received policy config update from pubkey: %s", hex.Enc(ev.Pubkey))

	// 1. Verify sender is policy admin (from current policy.policy_admins list)
	if l.policyManager == nil {
		return fmt.Errorf("policy system is not enabled")
	}

	isAdmin := l.policyManager.IsPolicyAdmin(ev.Pubkey)
	if !isAdmin {
		log.W.F("policy config update rejected: pubkey %s is not a policy admin", hex.Enc(ev.Pubkey))
		return fmt.Errorf("only policy administrators can update policy configuration")
	}

	log.I.F("policy admin verified: %s", hex.Enc(ev.Pubkey))

	// 2. Parse and validate JSON FIRST (before making any changes)
	policyJSON := []byte(ev.Content)
	if err := l.policyManager.ValidateJSON(policyJSON); chk.E(err) {
		log.E.F("policy config update validation failed: %v", err)
		return fmt.Errorf("invalid policy configuration: %v", err)
	}

	log.I.F("policy config validation passed")

	// Get config path for saving
	configPath := filepath.Join(xdg.ConfigHome, l.Config.AppName, "policy.json")

	// 3. Pause ALL message processing (lock mutex)
	// Note: We need to release the RLock first (which caller holds), then acquire exclusive Lock
	// Actually, the HandleMessage already released the lock after calling HandleEvent
	// So we can directly acquire the exclusive lock
	log.I.F("pausing message processing for policy update")
	l.Server.PauseMessageProcessing()
	defer l.Server.ResumeMessageProcessing()

	// 4. Reload policy (this will pause policy engine, update, save, and resume)
	log.I.F("applying policy configuration update")
	if err := l.policyManager.Reload(policyJSON, configPath); chk.E(err) {
		log.E.F("policy config update failed: %v", err)
		return fmt.Errorf("failed to apply policy configuration: %v", err)
	}

	log.I.F("policy configuration updated successfully by admin: %s", hex.Enc(ev.Pubkey))

	// 5. Message processing mutex will be unlocked by defer
	return nil
}

// HandlePolicyAdminFollowListUpdate processes kind 3 follow list events from policy admins.
// When a policy admin updates their follow list, we immediately refresh the policy follows cache.
//
// Process flow:
// 1. Check if sender is a policy admin
// 2. If yes, extract p-tags from the follow list
// 3. Pause message processing
// 4. Aggregate all policy admin follows and update cache
// 5. Resume message processing
func (l *Listener) HandlePolicyAdminFollowListUpdate(ev *event.E) error {
	// Only process if policy system is enabled
	if l.policyManager == nil || !l.policyManager.IsEnabled() {
		return nil // Not an error, just ignore
	}

	// Check if sender is a policy admin
	if !l.policyManager.IsPolicyAdmin(ev.Pubkey) {
		return nil // Not a policy admin, ignore
	}

	log.I.F("policy admin %s updated their follow list, refreshing policy follows", hex.Enc(ev.Pubkey))

	// Extract p-tags from this follow list event
	newFollows := extractFollowsFromEvent(ev)

	// Pause message processing for atomic update
	log.D.F("pausing message processing for follow list update")
	l.Server.PauseMessageProcessing()
	defer l.Server.ResumeMessageProcessing()

	// Get all current follows from database for all policy admins
	// For now, we'll merge the new follows with existing ones
	// A more complete implementation would re-fetch all admin follows from DB
	allFollows, err := l.fetchAllPolicyAdminFollows()
	if err != nil {
		log.W.F("failed to fetch all policy admin follows: %v, using new follows only", err)
		allFollows = newFollows
	} else {
		// Merge with the new follows (deduplicated)
		allFollows = mergeFollows(allFollows, newFollows)
	}

	// Update the policy follows cache
	l.policyManager.UpdatePolicyFollows(allFollows)

	log.I.F("policy follows cache updated with %d total pubkeys", len(allFollows))
	return nil
}

// extractFollowsFromEvent extracts p-tag pubkeys from a kind 3 follow list event.
// Returns binary pubkeys.
func extractFollowsFromEvent(ev *event.E) [][]byte {
	var follows [][]byte

	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pTag := range pTags {
		// ValueHex() handles both binary and hex storage formats automatically
		pt, err := hex.Dec(string(pTag.ValueHex()))
		if err != nil {
			continue
		}
		follows = append(follows, pt)
	}

	return follows
}

// fetchAllPolicyAdminFollows fetches kind 3 events for all policy admins from the database
// and aggregates their follows.
func (l *Listener) fetchAllPolicyAdminFollows() ([][]byte, error) {
	var allFollows [][]byte
	seen := make(map[string]bool)

	// Get policy admin pubkeys
	admins := l.policyManager.GetPolicyAdminsBin()
	if len(admins) == 0 {
		return nil, fmt.Errorf("no policy admins configured")
	}

	// For each admin, query their latest kind 3 event
	for _, adminPubkey := range admins {
		// Build proper filter for kind 3 from this admin
		f := filter.New()
		f.Authors = tag.NewFromAny(adminPubkey)
		f.Kinds = kind.NewS(kind.FollowList)
		limit := uint(1)
		f.Limit = &limit

		// Query the database for kind 3 events from this admin
		events, err := l.DB.QueryEvents(l.ctx, f)
		if err != nil {
			log.W.F("failed to query follows for admin %s: %v", hex.Enc(adminPubkey), err)
			continue
		}

		// events is []*event.E - iterate over the slice
		for _, ev := range events {
			// Extract p-tags from this follow list
			follows := extractFollowsFromEvent(ev)
			for _, follow := range follows {
				key := string(follow)
				if !seen[key] {
					seen[key] = true
					allFollows = append(allFollows, follow)
				}
			}
		}
	}

	return allFollows, nil
}

// mergeFollows merges two follow lists, removing duplicates.
func mergeFollows(existing, newFollows [][]byte) [][]byte {
	seen := make(map[string]bool)
	var result [][]byte

	for _, f := range existing {
		key := string(f)
		if !seen[key] {
			seen[key] = true
			result = append(result, f)
		}
	}

	for _, f := range newFollows {
		key := string(f)
		if !seen[key] {
			seen[key] = true
			result = append(result, f)
		}
	}

	return result
}

// IsPolicyConfigEvent returns true if the event is a policy configuration event (kind 12345)
func IsPolicyConfigEvent(ev *event.E) bool {
	return ev.Kind == kind.PolicyConfig.K
}

// IsPolicyAdminFollowListEvent returns true if this is a follow list event from a policy admin.
// Used to detect when we need to refresh the policy follows cache.
func (l *Listener) IsPolicyAdminFollowListEvent(ev *event.E) bool {
	// Must be kind 3 (follow list)
	if ev.Kind != kind.FollowList.K {
		return false
	}

	// Policy system must be enabled
	if l.policyManager == nil || !l.policyManager.IsEnabled() {
		return false
	}

	// Sender must be a policy admin
	return l.policyManager.IsPolicyAdmin(ev.Pubkey)
}

// isPolicyAdmin checks if a pubkey is in the list of policy admins
func isPolicyAdmin(pubkey []byte, admins [][]byte) bool {
	for _, admin := range admins {
		if bytes.Equal(pubkey, admin) {
			return true
		}
	}
	return false
}

// InitializePolicyFollows loads the follow lists of all policy admins at startup.
// This should be called after the policy manager is initialized but before
// the relay starts accepting connections.
// It's a method on Server so it can be called from main.go during initialization.
func (s *Server) InitializePolicyFollows() error {
	// Skip if policy system is not enabled
	if s.policyManager == nil || !s.policyManager.IsEnabled() {
		log.D.F("policy system not enabled, skipping follow list initialization")
		return nil
	}

	// Skip if PolicyFollowWhitelistEnabled is false
	if !s.policyManager.IsPolicyFollowWhitelistEnabled() {
		log.D.F("policy follow whitelist not enabled, skipping follow list initialization")
		return nil
	}

	log.I.F("initializing policy follows from database")

	// Get policy admin pubkeys
	admins := s.policyManager.GetPolicyAdminsBin()
	if len(admins) == 0 {
		log.W.F("no policy admins configured, skipping follow list initialization")
		return nil
	}

	var allFollows [][]byte
	seen := make(map[string]bool)

	// For each admin, query their latest kind 3 event
	for _, adminPubkey := range admins {
		// Build proper filter for kind 3 from this admin
		f := filter.New()
		f.Authors = tag.NewFromAny(adminPubkey)
		f.Kinds = kind.NewS(kind.FollowList)
		limit := uint(1)
		f.Limit = &limit

		// Query the database for kind 3 events from this admin
		events, err := s.DB.QueryEvents(s.Ctx, f)
		if err != nil {
			log.W.F("failed to query follows for admin %s: %v", hex.Enc(adminPubkey), err)
			continue
		}

		// Extract p-tags from each follow list event
		for _, ev := range events {
			follows := extractFollowsFromEvent(ev)
			for _, follow := range follows {
				key := string(follow)
				if !seen[key] {
					seen[key] = true
					allFollows = append(allFollows, follow)
				}
			}
		}
	}

	// Update the policy follows cache
	s.policyManager.UpdatePolicyFollows(allFollows)

	log.I.F("policy follows initialized with %d pubkeys from %d admin(s)",
		len(allFollows), len(admins))

	return nil
}
