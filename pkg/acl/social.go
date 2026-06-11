//go:build !(js && wasm)

package acl

import (
	"context"
	"encoding/hex"
	"math"
	"reflect"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	nhex "git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// social actor request types

type socialGetAccessLevelReq struct {
	pub     []byte
	address string
	resp    chan string
}

type socialGetThrottleDelayReq struct {
	pubkey []byte
	ip     string
	resp   chan time.Duration
}

type socialRecordInteractionReq struct {
	outsiderPubkeyHex string
}

type socialGetTrustMultiplierReq struct {
	pubkeyHex string
	resp      chan float64
}

type socialDecayAndCleanReq struct {
	resp chan struct{}
}

// Social is an ACL driver that uses WoT graph topology with inbound-trust
// rate limiting. It assigns throttle tiers based on BFS depth from anchor
// pubkeys (owners/admins), and modulates outsider throttling based on
// interaction signals from trusted users.
type Social struct {
	Ctx context.Context
	cfg *config.C
	db  database.Database

	// Concrete database for graph traversal (type-asserted from database.Database)
	badgerDB *database.D

	// Identity sets (immutable after Configure, read by actor)
	owners    [][]byte
	admins    [][]byte
	ownersSet map[string]struct{}
	adminsSet map[string]struct{}

	// WoT depth map (shared with GC)
	wotMap *WoTDepthMap

	// Per-depth throttles
	depth2Throttle   *ProgressiveThrottle
	depth3Throttle   *ProgressiveThrottle
	outsiderThrottle *ProgressiveThrottle

	// Actor channels
	getAccessLevelCh      chan socialGetAccessLevelReq
	getThrottleDelayCh    chan socialGetThrottleDelayReq
	recordInteractionCh   chan socialRecordInteractionReq
	getTrustMultiplierCh  chan socialGetTrustMultiplierReq
	decayAndCleanCh       chan socialDecayAndCleanReq
	stop                  chan struct{}
	done                  chan struct{}
}

// TrustSignal records the accumulated trust an outsider has earned from
// interactions by trusted users. Count decays by halving every 24 hours.
type TrustSignal struct {
	Count     float64
	LastDecay time.Time
}

func (s *Social) Configure(cfg ...any) (err error) {
	log.I.F("configuring social ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			s.cfg = c
		case database.Database:
			s.db = c
		case context.Context:
			s.Ctx = c
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if s.cfg == nil || s.db == nil {
		return errorf.E("both config and database must be set")
	}

	// Type-assert to concrete Badger database for graph traversal
	if d, ok := s.db.(*database.D); ok {
		s.badgerDB = d
	} else {
		log.W.F("social ACL: database is not Badger, graph traversal will be unavailable")
	}

	// Parse owner and admin pubkeys
	s.ownersSet = make(map[string]struct{})
	s.adminsSet = make(map[string]struct{})

	for _, owner := range s.cfg.Owners {
		if own, e := bech32encoding.NpubOrHexToPublicKeyBinary(owner); !chk.E(e) {
			s.owners = append(s.owners, own)
			s.ownersSet[hex.EncodeToString(own)] = struct{}{}
		}
	}
	for _, admin := range s.cfg.Admins {
		if adm, e := bech32encoding.NpubOrHexToPublicKeyBinary(admin); !chk.E(e) {
			s.admins = append(s.admins, adm)
			s.adminsSet[hex.EncodeToString(adm)] = struct{}{}
		}
	}

	// Build anchor set for WoT (owners + admins)
	anchors := make([][]byte, 0, len(s.owners)+len(s.admins))
	anchors = append(anchors, s.owners...)
	anchors = append(anchors, s.admins...)

	// Get social config values
	d2Inc, d2Max, d3Inc, d3Max, outInc, outMax, wotDepth, _ := s.cfg.GetSocialConfigValues()

	// Initialize WoT depth map
	s.wotMap = NewWoTDepthMap(anchors, wotDepth)

	// Initialize throttles for each depth tier
	s.depth2Throttle = NewProgressiveThrottle(d2Inc, d2Max)
	s.depth3Throttle = NewProgressiveThrottle(d3Inc, d3Max)
	s.outsiderThrottle = NewProgressiveThrottle(outInc, outMax)

	// Start actor goroutine
	s.getAccessLevelCh = make(chan socialGetAccessLevelReq)
	s.getThrottleDelayCh = make(chan socialGetThrottleDelayReq)
	s.recordInteractionCh = make(chan socialRecordInteractionReq, 128)
	s.getTrustMultiplierCh = make(chan socialGetTrustMultiplierReq)
	s.decayAndCleanCh = make(chan socialDecayAndCleanReq)
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.actor()

	log.I.F("social ACL configured: %d owners, %d admins, WoT depth %d",
		len(s.owners), len(s.admins), wotDepth)
	log.I.F("social ACL throttles: d2=%v/%v d3=%v/%v outsider=%v/%v",
		d2Inc, d2Max, d3Inc, d3Max, outInc, outMax)

	return nil
}

func (s *Social) actor() {
	defer close(s.done)

	trustSignals := make(map[string]*TrustSignal)

	for {
		select {
		case <-s.stop:
			return

		case req := <-s.getAccessLevelCh:
			pubHex := hex.EncodeToString(req.pub)
			level := "write"
			if _, ok := s.ownersSet[pubHex]; ok {
				level = "owner"
			} else if _, ok := s.adminsSet[pubHex]; ok {
				level = "admin"
			}
			req.resp <- level

		case req := <-s.getThrottleDelayCh:
			pubkeyHex := hex.EncodeToString(req.pubkey)
			if _, ok := s.ownersSet[pubkeyHex]; ok {
				req.resp <- 0
				continue
			}
			if _, ok := s.adminsSet[pubkeyHex]; ok {
				req.resp <- 0
				continue
			}
			depth := s.wotMap.GetDepthByHex(pubkeyHex)
			var d time.Duration
			switch {
			case depth == 0:
				d = 0
			case depth == 1:
				d = 0
			case depth == 2:
				d = s.depth2Throttle.GetDelay(req.ip, pubkeyHex)
			case depth == 3:
				d = s.depth3Throttle.GetDelay(req.ip, pubkeyHex)
			default:
				baseDelay := s.outsiderThrottle.GetDelay(req.ip, pubkeyHex)
				trustMult := s.getTrustMultiplierInternal(pubkeyHex, trustSignals)
				d = time.Duration(float64(baseDelay) * (1.0 - trustMult))
			}
			req.resp <- d

		case req := <-s.recordInteractionCh:
			signal, ok := trustSignals[req.outsiderPubkeyHex]
			if !ok {
				signal = &TrustSignal{LastDecay: time.Now()}
				trustSignals[req.outsiderPubkeyHex] = signal
			}
			applyDecay(signal)
			signal.Count++

		case req := <-s.getTrustMultiplierCh:
			req.resp <- s.getTrustMultiplierInternal(req.pubkeyHex, trustSignals)

		case req := <-s.decayAndCleanCh:
			for key, signal := range trustSignals {
				applyDecay(signal)
				if signal.Count < 0.01 {
					delete(trustSignals, key)
				}
			}
			log.T.F("social ACL: trust signals cleanup, %d active", len(trustSignals))
			req.resp <- struct{}{}
		}
	}
}

// getTrustMultiplierInternal computes trust multiplier from owned state. Must be called from actor.
func (s *Social) getTrustMultiplierInternal(pubkeyHex string, trustSignals map[string]*TrustSignal) float64 {
	signal, ok := trustSignals[pubkeyHex]
	if !ok {
		return 0.0
	}
	applyDecay(signal)
	if signal.Count < 0.01 {
		return 0.0
	}
	return 1.0 - (1.0 / (1.0 + signal.Count/5.0))
}

func (s *Social) GetAccessLevel(pub []byte, address string) (level string) {
	resp := make(chan string, 1)
	s.getAccessLevelCh <- socialGetAccessLevelReq{pub: pub, address: address, resp: resp}
	return <-resp
}

func (s *Social) GetACLInfo() (name, description, documentation string) {
	return "social",
		"WoT graph topology with inbound-trust rate limiting",
		`Social ACL assigns throttle tiers based on WoT depth from relay owners/admins.
Depth 1 (direct follows): no throttle.
Depth 2: light throttle.
Depth 3: moderate throttle.
Outsiders: heavy throttle, reduced by inbound trust signals from trusted users.`
}

func (s *Social) Type() string { return "social" }

// GetThrottleDelay returns the rate-limit delay for this pubkey/IP based on
// WoT depth and inbound trust signals.
func (s *Social) GetThrottleDelay(pubkey []byte, ip string) time.Duration {
	resp := make(chan time.Duration, 1)
	s.getThrottleDelayCh <- socialGetThrottleDelayReq{pubkey: pubkey, ip: ip, resp: resp}
	return <-resp
}

// CheckPolicy implements PolicyChecker. It inspects events from trusted users
// (WoT depth 1-3) to detect interactions with outsiders and records trust signals.
// It never rejects events.
func (s *Social) CheckPolicy(ev *event.E) (bool, error) {
	if s.wotMap == nil {
		return true, nil
	}

	authorHex := nhex.Enc(ev.Pubkey)
	authorDepth := s.wotMap.GetDepthByHex(authorHex)

	// Only record trust signals from WoT members (depth 0-3)
	if authorDepth < 0 {
		return true, nil
	}

	switch ev.Kind {
	case 1: // Text note — check for reply e-tags
		s.recordInteractionsFromETags(ev)
	case 7: // Reaction — check for e-tag target
		s.recordInteractionsFromETags(ev)
	case 9735: // Zap receipt — check for p-tag target
		s.recordInteractionsFromPTags(ev)
	case 3: // Follow list — check for p-tag targets
		s.recordInteractionsFromPTags(ev)
	}

	return true, nil
}

// recordInteractionsFromETags finds e-tag targets, resolves their authors,
// and records trust signals for outsiders.
func (s *Social) recordInteractionsFromETags(ev *event.E) {
	if s.badgerDB == nil {
		return
	}
	eTags := ev.Tags.GetAll([]byte("e"))
	for _, eTag := range eTags {
		if eTag.Len() < 2 {
			continue
		}
		targetID, err := nhex.Dec(string(eTag.ValueHex()))
		if err != nil || len(targetID) != 32 {
			continue
		}
		// Look up the target event to find its author
		ser, err := s.badgerDB.GetSerialById(targetID)
		if err != nil || ser == nil {
			continue
		}
		targetEv, err := s.badgerDB.FetchEventBySerial(ser)
		if err != nil || targetEv == nil {
			continue
		}
		targetHex := nhex.Enc(targetEv.Pubkey)
		// Only record if target is an outsider
		if s.wotMap.GetDepthByHex(targetHex) < 0 {
			s.recordInteraction(targetHex)
		}
	}
}

// recordInteractionsFromPTags records trust signals for outsiders referenced
// in p-tags of the event.
func (s *Social) recordInteractionsFromPTags(ev *event.E) {
	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pTag := range pTags {
		if pTag.Len() < 2 {
			continue
		}
		targetHex := string(pTag.ValueHex())
		if len(targetHex) != 64 {
			continue
		}
		// Only record if target is an outsider
		if s.wotMap.GetDepthByHex(targetHex) < 0 {
			s.recordInteraction(targetHex)
		}
	}
}

// recordInteraction increments the trust signal count for an outsider pubkey.
// Fire-and-forget via buffered channel.
func (s *Social) recordInteraction(outsiderPubkeyHex string) {
	select {
	case s.recordInteractionCh <- socialRecordInteractionReq{outsiderPubkeyHex: outsiderPubkeyHex}:
	default:
		// Drop if buffer full - acceptable for trust signals
	}
}

// applyDecay halves the trust signal count for each 24-hour period elapsed.
func applyDecay(signal *TrustSignal) {
	hoursSince := time.Since(signal.LastDecay).Hours()
	if hoursSince >= 24 {
		halvings := int(hoursSince / 24)
		signal.Count *= math.Pow(0.5, float64(halvings))
		signal.LastDecay = time.Now()
	}
}

// GetWoTDepthMap returns the shared WoT depth map for GC integration.
func (s *Social) GetWoTDepthMap() *WoTDepthMap {
	return s.wotMap
}

// Syncer starts background goroutines for WoT recomputation, throttle cleanup,
// and trust signal maintenance.
func (s *Social) Syncer() {
	log.D.F("starting social syncer")

	// Compute WoT map on startup
	if s.badgerDB != nil {
		go s.wotRefreshLoop()
	}

	// Throttle cleanup for all three tiers
	go s.throttleCleanupLoop()

	// Trust signal decay and cleanup
	go s.trustCleanupLoop()
}

func (s *Social) wotRefreshLoop() {
	// Initial computation
	if err := s.wotMap.Recompute(s.badgerDB); err != nil {
		log.W.F("social ACL: initial WoT recompute failed: %v", err)
	}

	_, _, _, _, _, _, _, refreshInterval := s.cfg.GetSocialConfigValues()
	if refreshInterval <= 0 {
		refreshInterval = time.Hour
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.Ctx.Done():
			return
		case <-ticker.C:
			if err := s.wotMap.Recompute(s.badgerDB); err != nil {
				log.W.F("social ACL: WoT recompute failed: %v", err)
			}
		}
	}
}

func (s *Social) throttleCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.Ctx.Done():
			return
		case <-ticker.C:
			s.depth2Throttle.Cleanup()
			s.depth3Throttle.Cleanup()
			s.outsiderThrottle.Cleanup()
		}
	}
}

func (s *Social) trustCleanupLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.Ctx.Done():
			return
		case <-ticker.C:
			s.decayAndCleanTrustSignals()
		}
	}
}

// decayAndCleanTrustSignals applies decay to all signals and removes dead ones.
func (s *Social) decayAndCleanTrustSignals() {
	resp := make(chan struct{}, 1)
	s.decayAndCleanCh <- socialDecayAndCleanReq{resp: resp}
	<-resp
}
