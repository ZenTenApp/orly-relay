package directory_client

import (
	"sync"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/protocol/directory"
)

// TrustCalculator computes aggregate trust scores from multiple trust acts.
//
// It maintains a collection of trust acts and provides methods to calculate
// weighted trust scores for relay public keys.
type TrustCalculator struct {
	mu   sync.RWMutex
	acts map[string][]*directory.TrustAct
}

// NewTrustCalculator creates a new trust calculator instance.
func NewTrustCalculator() *TrustCalculator {
	return &TrustCalculator{
		acts: make(map[string][]*directory.TrustAct),
	}
}

// AddAct adds a trust act to the calculator.
func (tc *TrustCalculator) AddAct(act *directory.TrustAct) {
	if act == nil {
		return
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	targetPubkey := act.TargetPubkey
	tc.acts[targetPubkey] = append(tc.acts[targetPubkey], act)
}

// CalculateTrust calculates an aggregate trust score for a public key.
//
// The score is computed as a weighted average where:
//   - high trust = 100
//   - medium trust = 50
//   - low trust = 25
//
// Expired trust acts are excluded from the calculation.
// Returns a score between 0 and 100.
func (tc *TrustCalculator) CalculateTrust(pubkey string) float64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	acts := tc.acts[pubkey]
	if len(acts) == 0 {
		return 0
	}

	now := time.Now()
	var total float64
	var count int

	// Weight mapping
	weights := map[directory.TrustLevel]float64{
		directory.TrustLevelHigh:   100,
		directory.TrustLevelMedium: 50,
		directory.TrustLevelLow:    25,
	}

	for _, act := range acts {
		// Skip expired acts
		if act.Expiry != nil && act.Expiry.Before(now) {
			continue
		}

		weight, ok := weights[act.TrustLevel]
		if !ok {
			continue
		}

		total += weight
		count++
	}

	if count == 0 {
		return 0
	}

	return total / float64(count)
}

// GetActs returns all trust acts for a specific public key.
func (tc *TrustCalculator) GetActs(pubkey string) []*directory.TrustAct {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	acts := tc.acts[pubkey]
	result := make([]*directory.TrustAct, len(acts))
	copy(result, acts)
	return result
}

// GetActiveTrustActs returns only non-expired trust acts for a public key.
func (tc *TrustCalculator) GetActiveTrustActs(pubkey string) []*directory.TrustAct {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	acts := tc.acts[pubkey]
	now := time.Now()
	result := make([]*directory.TrustAct, 0)

	for _, act := range acts {
		if act.Expiry == nil || act.Expiry.After(now) {
			result = append(result, act)
		}
	}

	return result
}

// Clear removes all trust acts from the calculator.
func (tc *TrustCalculator) Clear() {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.acts = make(map[string][]*directory.TrustAct)
}

// GetAllPubkeys returns all public keys that have trust acts.
func (tc *TrustCalculator) GetAllPubkeys() []string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	pubkeys := make([]string, 0, len(tc.acts))
	for pubkey := range tc.acts {
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys
}

// ReplicationFilter manages replication decisions based on trust scores.
//
// It uses a TrustCalculator to compute trust scores and determines which
// relays are trusted enough for replication based on a minimum threshold.
type ReplicationFilter struct {
	mu            sync.RWMutex
	trustCalc     *TrustCalculator
	minTrustScore float64
	trustedRelays map[string]bool
}

// NewReplicationFilter creates a new replication filter with a minimum trust score threshold.
func NewReplicationFilter(minTrustScore float64) *ReplicationFilter {
	return &ReplicationFilter{
		trustCalc:     NewTrustCalculator(),
		minTrustScore: minTrustScore,
		trustedRelays: make(map[string]bool),
	}
}

// AddTrustAct adds a trust act and updates the trusted relays set.
func (rf *ReplicationFilter) AddTrustAct(act *directory.TrustAct) {
	if act == nil {
		return
	}

	rf.trustCalc.AddAct(act)

	// Update trusted relays based on new trust score
	score := rf.trustCalc.CalculateTrust(act.TargetPubkey)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if score >= rf.minTrustScore {
		rf.trustedRelays[act.TargetPubkey] = true
	} else {
		delete(rf.trustedRelays, act.TargetPubkey)
	}
}

// ShouldReplicate checks if a relay is trusted enough for replication.
func (rf *ReplicationFilter) ShouldReplicate(pubkey string) bool {
	rf.mu.RLock()
	defer rf.mu.RUnlock()

	return rf.trustedRelays[pubkey]
}

// GetTrustedRelays returns all trusted relay public keys.
func (rf *ReplicationFilter) GetTrustedRelays() []string {
	rf.mu.RLock()
	defer rf.mu.RUnlock()

	relays := make([]string, 0, len(rf.trustedRelays))
	for pubkey := range rf.trustedRelays {
		relays = append(relays, pubkey)
	}
	return relays
}

// GetTrustScore returns the trust score for a relay.
func (rf *ReplicationFilter) GetTrustScore(pubkey string) float64 {
	return rf.trustCalc.CalculateTrust(pubkey)
}

// SetMinTrustScore updates the minimum trust score threshold and recalculates trusted relays.
func (rf *ReplicationFilter) SetMinTrustScore(minScore float64) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	rf.minTrustScore = minScore

	// Recalculate trusted relays with new threshold
	rf.trustedRelays = make(map[string]bool)
	for _, pubkey := range rf.trustCalc.GetAllPubkeys() {
		score := rf.trustCalc.CalculateTrust(pubkey)
		if score >= rf.minTrustScore {
			rf.trustedRelays[pubkey] = true
		}
	}
}

// GetMinTrustScore returns the current minimum trust score threshold.
func (rf *ReplicationFilter) GetMinTrustScore() float64 {
	rf.mu.RLock()
	defer rf.mu.RUnlock()

	return rf.minTrustScore
}

// FilterEvents filters events to only those from trusted relays.
func (rf *ReplicationFilter) FilterEvents(events []*event.E) []*event.E {
	rf.mu.RLock()
	defer rf.mu.RUnlock()

	filtered := make([]*event.E, 0)
	for _, ev := range events {
		if rf.trustedRelays[string(ev.Pubkey)] {
			filtered = append(filtered, ev)
		}
	}
	return filtered
}
