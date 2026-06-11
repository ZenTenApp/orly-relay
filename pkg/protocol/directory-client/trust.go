package directory_client

import (
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/protocol/directory"
)

// --- TrustCalculator actor request types ---

type tcAddActReq struct {
	act *directory.TrustAct
}

type tcCalculateTrustReq struct {
	pubkey string
	resp   chan float64
}

type tcGetActsReq struct {
	pubkey string
	resp   chan []*directory.TrustAct
}

type tcGetActiveActsReq struct {
	pubkey string
	resp   chan []*directory.TrustAct
}

type tcClearReq struct {
	resp chan struct{}
}

type tcGetAllPubkeysReq struct {
	resp chan []string
}

// TrustCalculator computes aggregate trust scores from multiple trust acts.
type TrustCalculator struct {
	acts map[string][]*directory.TrustAct

	addActCh          chan tcAddActReq
	calculateTrustCh  chan tcCalculateTrustReq
	getActsCh         chan tcGetActsReq
	getActiveActsCh   chan tcGetActiveActsReq
	clearCh           chan tcClearReq
	getAllPubkeysCh   chan tcGetAllPubkeysReq

	stop chan struct{}
	done chan struct{}
}

// NewTrustCalculator creates a new trust calculator instance.
func NewTrustCalculator() *TrustCalculator {
	tc := &TrustCalculator{
		acts: make(map[string][]*directory.TrustAct),

		addActCh:         make(chan tcAddActReq, 16),
		calculateTrustCh: make(chan tcCalculateTrustReq),
		getActsCh:        make(chan tcGetActsReq),
		getActiveActsCh:  make(chan tcGetActiveActsReq),
		clearCh:          make(chan tcClearReq),
		getAllPubkeysCh:  make(chan tcGetAllPubkeysReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go tc.run()
	return tc
}

// Stop shuts down the actor goroutine.
func (tc *TrustCalculator) Stop() {
	close(tc.stop)
	<-tc.done
}

func (tc *TrustCalculator) run() {
	defer close(tc.done)
	for {
		select {
		case <-tc.stop:
			return
		case req := <-tc.addActCh:
			if req.act != nil {
				targetPubkey := req.act.TargetPubkey
				tc.acts[targetPubkey] = append(tc.acts[targetPubkey], req.act)
			}
		case req := <-tc.calculateTrustCh:
			req.resp <- tc.doCalculateTrust(req.pubkey)
		case req := <-tc.getActsCh:
			acts := tc.acts[req.pubkey]
			result := make([]*directory.TrustAct, len(acts))
			copy(result, acts)
			req.resp <- result
		case req := <-tc.getActiveActsCh:
			acts := tc.acts[req.pubkey]
			now := time.Now()
			result := make([]*directory.TrustAct, 0)
			for _, act := range acts {
				if act.Expiry == nil || act.Expiry.After(now) {
					result = append(result, act)
				}
			}
			req.resp <- result
		case req := <-tc.clearCh:
			tc.acts = make(map[string][]*directory.TrustAct)
			req.resp <- struct{}{}
		case req := <-tc.getAllPubkeysCh:
			pubkeys := make([]string, 0, len(tc.acts))
			for pubkey := range tc.acts {
				pubkeys = append(pubkeys, pubkey)
			}
			req.resp <- pubkeys
		}
	}
}

func (tc *TrustCalculator) doCalculateTrust(pubkey string) float64 {
	acts := tc.acts[pubkey]
	if len(acts) == 0 {
		return 0
	}

	now := time.Now()
	var total float64
	var count int

	weights := map[directory.TrustLevel]float64{
		directory.TrustLevelHigh:   100,
		directory.TrustLevelMedium: 50,
		directory.TrustLevelLow:    25,
	}

	for _, act := range acts {
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

// AddAct adds a trust act to the calculator.
func (tc *TrustCalculator) AddAct(act *directory.TrustAct) {
	if act == nil {
		return
	}
	select {
	case tc.addActCh <- tcAddActReq{act: act}:
	case <-tc.stop:
	}
}

// CalculateTrust calculates an aggregate trust score for a public key.
func (tc *TrustCalculator) CalculateTrust(pubkey string) float64 {
	resp := make(chan float64, 1)
	select {
	case tc.calculateTrustCh <- tcCalculateTrustReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-tc.stop:
		return 0
	}
}

// GetActs returns all trust acts for a specific public key.
func (tc *TrustCalculator) GetActs(pubkey string) []*directory.TrustAct {
	resp := make(chan []*directory.TrustAct, 1)
	select {
	case tc.getActsCh <- tcGetActsReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-tc.stop:
		return nil
	}
}

// GetActiveTrustActs returns only non-expired trust acts for a public key.
func (tc *TrustCalculator) GetActiveTrustActs(pubkey string) []*directory.TrustAct {
	resp := make(chan []*directory.TrustAct, 1)
	select {
	case tc.getActiveActsCh <- tcGetActiveActsReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-tc.stop:
		return nil
	}
}

// Clear removes all trust acts from the calculator.
func (tc *TrustCalculator) Clear() {
	resp := make(chan struct{}, 1)
	select {
	case tc.clearCh <- tcClearReq{resp: resp}:
		<-resp
	case <-tc.stop:
	}
}

// GetAllPubkeys returns all public keys that have trust acts.
func (tc *TrustCalculator) GetAllPubkeys() []string {
	resp := make(chan []string, 1)
	select {
	case tc.getAllPubkeysCh <- tcGetAllPubkeysReq{resp: resp}:
		return <-resp
	case <-tc.stop:
		return nil
	}
}

// --- ReplicationFilter actor request types ---

type rfAddTrustActReq struct {
	act *directory.TrustAct
}

type rfShouldReplicateReq struct {
	pubkey string
	resp   chan bool
}

type rfGetTrustedRelaysReq struct {
	resp chan []string
}

type rfGetTrustScoreReq struct {
	pubkey string
	resp   chan float64
}

type rfSetMinTrustScoreReq struct {
	minScore float64
	resp     chan struct{}
}

type rfGetMinTrustScoreReq struct {
	resp chan float64
}

type rfFilterEventsReq struct {
	events []*event.E
	resp   chan []*event.E
}

// ReplicationFilter manages replication decisions based on trust scores.
type ReplicationFilter struct {
	trustCalc     *TrustCalculator
	minTrustScore float64
	trustedRelays map[string]bool

	addTrustActCh      chan rfAddTrustActReq
	shouldReplicateCh  chan rfShouldReplicateReq
	getTrustedRelaysCh chan rfGetTrustedRelaysReq
	getTrustScoreCh    chan rfGetTrustScoreReq
	setMinTrustScoreCh chan rfSetMinTrustScoreReq
	getMinTrustScoreCh chan rfGetMinTrustScoreReq
	filterEventsCh     chan rfFilterEventsReq

	stop chan struct{}
	done chan struct{}
}

// NewReplicationFilter creates a new replication filter with a minimum trust score threshold.
func NewReplicationFilter(minTrustScore float64) *ReplicationFilter {
	rf := &ReplicationFilter{
		trustCalc:     NewTrustCalculator(),
		minTrustScore: minTrustScore,
		trustedRelays: make(map[string]bool),

		addTrustActCh:      make(chan rfAddTrustActReq, 16),
		shouldReplicateCh:  make(chan rfShouldReplicateReq),
		getTrustedRelaysCh: make(chan rfGetTrustedRelaysReq),
		getTrustScoreCh:    make(chan rfGetTrustScoreReq),
		setMinTrustScoreCh: make(chan rfSetMinTrustScoreReq),
		getMinTrustScoreCh: make(chan rfGetMinTrustScoreReq),
		filterEventsCh:     make(chan rfFilterEventsReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go rf.run()
	return rf
}

// Stop shuts down both the ReplicationFilter and its underlying TrustCalculator.
func (rf *ReplicationFilter) Stop() {
	close(rf.stop)
	<-rf.done
	rf.trustCalc.Stop()
}

func (rf *ReplicationFilter) run() {
	defer close(rf.done)
	for {
		select {
		case <-rf.stop:
			return
		case req := <-rf.addTrustActCh:
			if req.act != nil {
				rf.trustCalc.AddAct(req.act)
				score := rf.trustCalc.CalculateTrust(req.act.TargetPubkey)
				if score >= rf.minTrustScore {
					rf.trustedRelays[req.act.TargetPubkey] = true
				} else {
					delete(rf.trustedRelays, req.act.TargetPubkey)
				}
			}
		case req := <-rf.shouldReplicateCh:
			req.resp <- rf.trustedRelays[req.pubkey]
		case req := <-rf.getTrustedRelaysCh:
			relays := make([]string, 0, len(rf.trustedRelays))
			for pubkey := range rf.trustedRelays {
				relays = append(relays, pubkey)
			}
			req.resp <- relays
		case req := <-rf.getTrustScoreCh:
			req.resp <- rf.trustCalc.CalculateTrust(req.pubkey)
		case req := <-rf.setMinTrustScoreCh:
			rf.minTrustScore = req.minScore
			rf.trustedRelays = make(map[string]bool)
			for _, pubkey := range rf.trustCalc.GetAllPubkeys() {
				score := rf.trustCalc.CalculateTrust(pubkey)
				if score >= rf.minTrustScore {
					rf.trustedRelays[pubkey] = true
				}
			}
			req.resp <- struct{}{}
		case req := <-rf.getMinTrustScoreCh:
			req.resp <- rf.minTrustScore
		case req := <-rf.filterEventsCh:
			filtered := make([]*event.E, 0)
			for _, ev := range req.events {
				if rf.trustedRelays[string(ev.Pubkey)] {
					filtered = append(filtered, ev)
				}
			}
			req.resp <- filtered
		}
	}
}

// AddTrustAct adds a trust act and updates the trusted relays set.
func (rf *ReplicationFilter) AddTrustAct(act *directory.TrustAct) {
	if act == nil {
		return
	}
	select {
	case rf.addTrustActCh <- rfAddTrustActReq{act: act}:
	case <-rf.stop:
	}
}

// ShouldReplicate checks if a relay is trusted enough for replication.
func (rf *ReplicationFilter) ShouldReplicate(pubkey string) bool {
	resp := make(chan bool, 1)
	select {
	case rf.shouldReplicateCh <- rfShouldReplicateReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-rf.stop:
		return false
	}
}

// GetTrustedRelays returns all trusted relay public keys.
func (rf *ReplicationFilter) GetTrustedRelays() []string {
	resp := make(chan []string, 1)
	select {
	case rf.getTrustedRelaysCh <- rfGetTrustedRelaysReq{resp: resp}:
		return <-resp
	case <-rf.stop:
		return nil
	}
}

// GetTrustScore returns the trust score for a relay.
func (rf *ReplicationFilter) GetTrustScore(pubkey string) float64 {
	resp := make(chan float64, 1)
	select {
	case rf.getTrustScoreCh <- rfGetTrustScoreReq{pubkey: pubkey, resp: resp}:
		return <-resp
	case <-rf.stop:
		return 0
	}
}

// SetMinTrustScore updates the minimum trust score threshold and recalculates trusted relays.
func (rf *ReplicationFilter) SetMinTrustScore(minScore float64) {
	resp := make(chan struct{}, 1)
	select {
	case rf.setMinTrustScoreCh <- rfSetMinTrustScoreReq{minScore: minScore, resp: resp}:
		<-resp
	case <-rf.stop:
	}
}

// GetMinTrustScore returns the current minimum trust score threshold.
func (rf *ReplicationFilter) GetMinTrustScore() float64 {
	resp := make(chan float64, 1)
	select {
	case rf.getMinTrustScoreCh <- rfGetMinTrustScoreReq{resp: resp}:
		return <-resp
	case <-rf.stop:
		return 0
	}
}

// FilterEvents filters events to only those from trusted relays.
func (rf *ReplicationFilter) FilterEvents(events []*event.E) []*event.E {
	resp := make(chan []*event.E, 1)
	select {
	case rf.filterEventsCh <- rfFilterEventsReq{events: events, resp: resp}:
		return <-resp
	case <-rf.stop:
		return nil
	}
}
