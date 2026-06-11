// Package crawler provides an automated corpus crawler that discovers relays
// via kind 10002 hop-expansion and then syncs all events from each discovered
// relay using NIP-77 negentropy set reconciliation.
//
// Architecture:
//
//	Discovery loop expands relay URLs from seed pubkeys via kind 10002 ->
//	Crawler maintains a persistent frontier of relay entries with per-relay
//	state -> Sync loop picks relays due for sync and runs negentropy
//	reconciliation with bounded concurrency -> Frontier state is persisted
//	to database markers so it survives restarts.
package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/interfaces/publisher"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/utils/normalize"
	"git.smesh.lol/orly/pkg/nostr/ws"
	dsync "git.smesh.lol/orly/pkg/sync"
	"git.smesh.lol/orly/pkg/sync/negentropy"
)

const (
	DefaultDiscoveryInterval = 4 * time.Hour
	DefaultSyncInterval      = 30 * time.Minute
	DefaultMaxHops           = 5
	DefaultConcurrency       = 3
	DefaultRelayTimeout      = 30 * time.Second
	QueryTimeout             = 60 * time.Second
	DefaultSyncTimeout       = 10 * time.Minute
	DefaultRelayDelay        = 500 * time.Millisecond
	DefaultMaxFailures       = 5
	DefaultBlacklistDuration = 24 * time.Hour
	DefaultMaxEventsPerSync  = 1_000_000
	MaxEventsPerQuery        = 5000

	markerFrontierKey = "crawler:frontier"
	markerStatsKey    = "crawler:stats"
)

// Config holds configuration for the corpus crawler.
type Config struct {
	DiscoveryInterval time.Duration
	MaxHops           int

	SyncInterval     time.Duration
	Concurrency      int
	SyncTimeout      time.Duration
	MaxEventsPerSync uint

	RelayDelay time.Duration

	MaxFailures       int
	BlacklistDuration time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		DiscoveryInterval: DefaultDiscoveryInterval,
		MaxHops:           DefaultMaxHops,
		SyncInterval:      DefaultSyncInterval,
		Concurrency:       DefaultConcurrency,
		SyncTimeout:       DefaultSyncTimeout,
		MaxEventsPerSync:  DefaultMaxEventsPerSync,
		RelayDelay:        DefaultRelayDelay,
		MaxFailures:       DefaultMaxFailures,
		BlacklistDuration: DefaultBlacklistDuration,
	}
}

// RelayState tracks the crawl state of a single relay.
type RelayState struct {
	URL              string    `json:"url"`
	HopDistance      int       `json:"hop_distance"`
	FirstSeen        time.Time `json:"first_seen"`
	LastSync         time.Time `json:"last_sync"`
	LastDiscovery    time.Time `json:"last_discovery"`
	EventsSynced     int64     `json:"events_synced"`
	TotalSyncs       int64     `json:"total_syncs"`
	ConsecFailures   int       `json:"consec_failures"`
	LastError        string    `json:"last_error,omitempty"`
	BlacklistedUntil time.Time `json:"blacklisted_until,omitempty"`
	IsSelf           bool      `json:"is_self,omitempty"`
}

func (rs *RelayState) isBlacklisted() bool {
	return !rs.BlacklistedUntil.IsZero() && time.Now().Before(rs.BlacklistedUntil)
}

func (rs *RelayState) needsSync(interval time.Duration) bool {
	if rs.IsSelf || rs.isBlacklisted() {
		return false
	}
	return rs.LastSync.IsZero() || time.Since(rs.LastSync) >= interval
}

// Stats tracks aggregate crawler statistics.
type Stats struct {
	TotalRelaysDiscovered int64     `json:"total_relays_discovered"`
	TotalRelaysSynced     int64     `json:"total_relays_synced"`
	TotalEventsSynced     int64     `json:"total_events_synced"`
	TotalSyncErrors       int64     `json:"total_sync_errors"`
	LastDiscoveryRun      time.Time `json:"last_discovery_run"`
	LastSyncRun           time.Time `json:"last_sync_run"`
	BlacklistedRelays     int64     `json:"blacklisted_relays"`
}

// Actor request types
type frontierMergeReq struct {
	filtered map[string]int
	resp     chan frontierMergeResp
}
type frontierMergeResp struct {
	added        int
	frontierSize int
}

type syncTarget struct {
	url        string
	hopDistance int
	totalSyncs int64
}

type syncTargetReq struct {
	interval time.Duration
	resp     chan []syncTarget
}

type syncResultReq struct {
	url          string
	success      bool
	eventsSynced int64
	lastError    string
}

type getStatsReq struct {
	resp chan Stats
}

type getFrontierSizeReq struct {
	resp chan int
}

type saveFrontierReq struct {
	resp chan saveFrontierResp
}

type saveFrontierResp struct {
	frontierData []byte
	statsData    []byte
	err          error
}

type selfURLCheckReq struct {
	url  string
	resp chan bool
}

type selfURLSetReq struct {
	url string
}

type setSeedCallbackReq struct {
	fn   func() [][]byte
	resp chan struct{}
}

type getSeedCallbackReq struct {
	resp chan func() [][]byte
}

// Crawler orchestrates relay discovery and corpus sync.
type Crawler struct {
	ctx    context.Context
	cancel context.CancelFunc

	db     database.Database
	pub    publisher.I
	config *Config

	// State owned by actor
	frontier map[string]*RelayState
	stats    Stats
	selfURLs map[string]bool

	// Seed callback owned by actor
	getSeedPubkeys func() [][]byte

	relayIdentityPubkey string
	nip11Cache          *dsync.NIP11Cache

	running atomic.Bool

	// Actor channels
	frontierMergeCh   chan frontierMergeReq
	syncTargetCh      chan syncTargetReq
	syncResultCh      chan syncResultReq
	getStatsCh        chan getStatsReq
	getFrontierSizeCh chan getFrontierSizeReq
	saveFrontierCh    chan saveFrontierReq
	selfURLCheckCh    chan selfURLCheckReq
	selfURLSetCh      chan selfURLSetReq
	setSeedCallbackCh chan setSeedCallbackReq
	getSeedCallbackCh chan getSeedCallbackReq
	syncCycleDoneCh   chan struct{}

	stop chan struct{}
	done chan struct{}
}

// New creates a new Crawler instance.
func New(ctx context.Context, db database.Database, pub publisher.I, cfg *Config) (*Crawler, error) {
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(ctx)

	var relayPubkey string
	if skb, err := db.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		pk, _ := keys.SecretBytesToPubKeyHex(skb)
		relayPubkey = pk
	}

	c := &Crawler{
		ctx:                 ctx,
		cancel:              cancel,
		db:                  db,
		pub:                 pub,
		config:              cfg,
		frontier:            make(map[string]*RelayState),
		selfURLs:            make(map[string]bool),
		nip11Cache:          dsync.NewNIP11Cache(30 * time.Minute),
		relayIdentityPubkey: relayPubkey,
		frontierMergeCh:     make(chan frontierMergeReq),
		syncTargetCh:        make(chan syncTargetReq),
		syncResultCh:        make(chan syncResultReq, 128),
		getStatsCh:          make(chan getStatsReq),
		getFrontierSizeCh:   make(chan getFrontierSizeReq),
		saveFrontierCh:      make(chan saveFrontierReq),
		selfURLCheckCh:      make(chan selfURLCheckReq),
		selfURLSetCh:        make(chan selfURLSetReq, 16),
		setSeedCallbackCh:   make(chan setSeedCallbackReq),
		getSeedCallbackCh:   make(chan getSeedCallbackReq),
		syncCycleDoneCh:     make(chan struct{}, 16),
		stop:                make(chan struct{}),
		done:                make(chan struct{}),
	}

	go c.actor()

	if err := c.loadFrontier(); err != nil {
		log.W.F("crawler: failed to load frontier: %v (starting fresh)", err)
	}

	return c, nil
}

func (c *Crawler) actor() {
	defer close(c.done)
	for {
		select {
		case <-c.stop:
			return
		case req := <-c.frontierMergeCh:
			added := 0
			for normURL, hopDist := range req.filtered {
				if existing, ok := c.frontier[normURL]; ok {
					existing.LastDiscovery = time.Now()
					if hopDist < existing.HopDistance {
						existing.HopDistance = hopDist
					}
				} else {
					c.frontier[normURL] = &RelayState{
						URL:           normURL,
						HopDistance:    hopDist,
						FirstSeen:     time.Now(),
						LastDiscovery: time.Now(),
					}
					added++
				}
			}
			c.stats.TotalRelaysDiscovered = int64(len(c.frontier))
			c.stats.LastDiscoveryRun = time.Now()
			req.resp <- frontierMergeResp{added: added, frontierSize: len(c.frontier)}
		case req := <-c.syncTargetCh:
			var due []syncTarget
			for _, rs := range c.frontier {
				if rs.needsSync(req.interval) {
					due = append(due, syncTarget{
						url:        rs.URL,
						hopDistance: rs.HopDistance,
						totalSyncs: rs.TotalSyncs,
					})
				}
			}
			req.resp <- due
		case req := <-c.syncResultCh:
			rs, exists := c.frontier[req.url]
			if !exists {
				continue
			}
			rs.LastSync = time.Now()
			rs.TotalSyncs++
			if !req.success {
				rs.ConsecFailures++
				rs.LastError = req.lastError
				if rs.ConsecFailures >= c.config.MaxFailures {
					rs.BlacklistedUntil = time.Now().Add(c.config.BlacklistDuration)
					c.stats.BlacklistedRelays++
					log.W.F("crawler: blacklisted %s after %d failures (until %v)",
						rs.URL, rs.ConsecFailures, rs.BlacklistedUntil)
				}
				c.stats.TotalSyncErrors++
				log.D.F("crawler: sync %s failed (%d consecutive): %s",
					rs.URL, rs.ConsecFailures, rs.LastError)
			} else {
				rs.ConsecFailures = 0
				rs.LastError = ""
				rs.EventsSynced += req.eventsSynced
				c.stats.TotalEventsSynced += req.eventsSynced
				c.stats.TotalRelaysSynced++
				if req.eventsSynced > 0 {
					log.I.F("crawler: synced %d events from %s", req.eventsSynced, rs.URL)
				}
			}
		case <-c.syncCycleDoneCh:
			c.stats.LastSyncRun = time.Now()
		case req := <-c.getStatsCh:
			req.resp <- c.stats
		case req := <-c.getFrontierSizeCh:
			req.resp <- len(c.frontier)
		case req := <-c.saveFrontierCh:
			data, err := json.Marshal(c.frontier)
			if err != nil {
				req.resp <- saveFrontierResp{err: fmt.Errorf("marshal frontier: %w", err)}
				continue
			}
			statsData, err := json.Marshal(c.stats)
			if err != nil {
				req.resp <- saveFrontierResp{err: fmt.Errorf("marshal stats: %w", err)}
				continue
			}
			req.resp <- saveFrontierResp{frontierData: data, statsData: statsData}
		case req := <-c.selfURLCheckCh:
			req.resp <- c.selfURLs[req.url]
		case req := <-c.selfURLSetCh:
			c.selfURLs[req.url] = true
		case req := <-c.setSeedCallbackCh:
			c.getSeedPubkeys = req.fn
			close(req.resp)
		case req := <-c.getSeedCallbackCh:
			req.resp <- c.getSeedPubkeys
		}
	}
}

// SetSeedCallback sets the callback for getting seed pubkeys used in discovery.
func (c *Crawler) SetSeedCallback(fn func() [][]byte) {
	resp := make(chan struct{})
	c.setSeedCallbackCh <- setSeedCallbackReq{fn: fn, resp: resp}
	<-resp
}

func (c *Crawler) callSeedCallback() [][]byte {
	resp := make(chan func() [][]byte, 1)
	c.getSeedCallbackCh <- getSeedCallbackReq{resp: resp}
	fn := <-resp
	if fn == nil {
		return nil
	}
	return fn()
}

// Start begins the crawler's discovery and sync loops.
func (c *Crawler) Start() error {
	if c.running.Load() {
		return fmt.Errorf("crawler already running")
	}

	resp := make(chan func() [][]byte, 1)
	c.getSeedCallbackCh <- getSeedCallbackReq{resp: resp}
	fn := <-resp
	if fn == nil {
		return fmt.Errorf("seed callback must be set before starting")
	}

	c.running.Store(true)

	go c.discoveryLoop()
	go c.syncLoop()

	log.I.F("crawler: started (discovery: %v, sync: %v, hops: %d, concurrency: %d)",
		c.config.DiscoveryInterval, c.config.SyncInterval,
		c.config.MaxHops, c.config.Concurrency)

	return nil
}

// Stop stops the crawler gracefully.
func (c *Crawler) Stop() {
	if !c.running.Load() {
		return
	}
	c.running.Store(false)

	c.cancel()
	close(c.stop)
	<-c.done

	// Save frontier after actor is stopped - frontier data is now uncontested
	data, _ := json.Marshal(c.frontier)
	statsData, _ := json.Marshal(c.stats)
	if data != nil {
		if err := c.db.SetMarker(markerFrontierKey, data); err != nil {
			log.W.F("crawler: failed to save frontier on stop: %v", err)
		}
	}
	if statsData != nil {
		if err := c.db.SetMarker(markerStatsKey, statsData); err != nil {
			log.W.F("crawler: failed to save stats on stop: %v", err)
		}
	}

	log.I.F("crawler: stopped (frontier: %d relays, total events synced: %d)",
		len(c.frontier), c.stats.TotalEventsSynced)
}

// GetStats returns a snapshot of crawler statistics.
func (c *Crawler) GetStats() Stats {
	resp := make(chan Stats, 1)
	c.getStatsCh <- getStatsReq{resp: resp}
	return <-resp
}

// GetFrontierSize returns the number of relays in the frontier.
func (c *Crawler) GetFrontierSize() int {
	resp := make(chan int, 1)
	c.getFrontierSizeCh <- getFrontierSizeReq{resp: resp}
	return <-resp
}

func (c *Crawler) discoveryLoop() {
	c.runDiscovery()

	ticker := time.NewTicker(c.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.runDiscovery()
		}
	}
}

func (c *Crawler) syncLoop() {
	select {
	case <-c.stop:
		return
	case <-time.After(30 * time.Second):
	}

	c.runSyncCycle()

	ticker := time.NewTicker(c.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.runSyncCycle()
		}
	}
}

// runDiscovery performs one relay discovery cycle using kind 10002 hop expansion.
func (c *Crawler) runDiscovery() {
	log.I.F("crawler: starting relay discovery (max hops: %d)", c.config.MaxHops)

	seeds := c.callSeedCallback()
	if len(seeds) == 0 {
		log.W.F("crawler: no seed pubkeys, skipping discovery")
		return
	}

	discovered := make(map[string]int) // URL -> hop distance

	localRelays := c.getRelaysFromLocalDB(seeds)
	for url := range localRelays {
		discovered[url] = 0
	}

	log.I.F("crawler: hop 0 discovered %d relays from %d seed pubkeys", len(discovered), len(seeds))

	for hop := 1; hop <= c.config.MaxHops; hop++ {
		select {
		case <-c.stop:
			return
		default:
		}

		var prevHopRelays []string
		for url, h := range discovered {
			if h == hop-1 {
				prevHopRelays = append(prevHopRelays, url)
			}
		}

		if len(prevHopRelays) == 0 {
			log.I.F("crawler: no relays at hop %d, stopping expansion", hop-1)
			break
		}

		newCount := 0
		for _, relayURL := range prevHopRelays {
			select {
			case <-c.stop:
				return
			default:
			}

			if c.isSelfRelay(relayURL) {
				continue
			}

			relays, err := c.fetchRelayListsFromRelay(relayURL)
			if err != nil {
				log.D.F("crawler: hop %d fetch from %s failed: %v", hop, relayURL, err)
				continue
			}

			for _, newURL := range relays {
				if _, exists := discovered[newURL]; !exists {
					discovered[newURL] = hop
					newCount++
				}
			}

			time.Sleep(c.config.RelayDelay)
		}

		log.I.F("crawler: hop %d discovered %d new relays from %d sources",
			hop, newCount, len(prevHopRelays))
	}

	// Filter self-relays before merging (isSelfRelay does network IO).
	filtered := make(map[string]int, len(discovered))
	for url, hopDist := range discovered {
		normURL := string(normalize.URL(url))
		if normURL == "" {
			continue
		}
		if c.isSelfRelay(normURL) {
			continue
		}
		filtered[normURL] = hopDist
	}

	// Merge into frontier via actor.
	resp := make(chan frontierMergeResp, 1)
	c.frontierMergeCh <- frontierMergeReq{filtered: filtered, resp: resp}
	result := <-resp

	log.I.F("crawler: discovery complete - %d new relays added, frontier size: %d",
		result.added, result.frontierSize)

	if err := c.saveFrontier(); err != nil {
		log.W.F("crawler: failed to save frontier: %v", err)
	}
}

// runSyncCycle syncs events from relays that are due for a sync.
func (c *Crawler) runSyncCycle() {
	resp := make(chan []syncTarget, 1)
	c.syncTargetCh <- syncTargetReq{interval: c.config.SyncInterval, resp: resp}
	due := <-resp

	if len(due) == 0 {
		log.D.F("crawler: no relays due for sync")
		return
	}

	log.I.F("crawler: starting sync cycle - %d relays due", len(due))

	sem := make(chan struct{}, c.config.Concurrency)
	doneChans := make([]chan struct{}, 0, len(due))

	for _, target := range due {
		select {
		case <-c.stop:
			for _, d := range doneChans {
				<-d
			}
			return
		default:
		}

		sem <- struct{}{}
		done := make(chan struct{})
		doneChans = append(doneChans, done)

		go func(t syncTarget) {
			defer close(done)
			defer func() { <-sem }()
			c.syncRelay(t.url, t.hopDistance, t.totalSyncs)
		}(target)
	}

	for _, d := range doneChans {
		<-d
	}

	c.syncCycleDoneCh <- struct{}{}

	if err := c.saveFrontier(); err != nil {
		log.W.F("crawler: failed to save frontier: %v", err)
	}

	log.I.F("crawler: sync cycle complete")
}

// syncRelay performs a negentropy sync with a single relay.
func (c *Crawler) syncRelay(url string, hopDistance int, totalSyncs int64) {
	ctx, cancel := context.WithTimeout(c.ctx, c.config.SyncTimeout)
	defer cancel()

	log.I.F("crawler: syncing %s (hop %d, syncs: %d)", url, hopDistance, totalSyncs)

	negCfg := &negentropy.Config{
		Peers:        []string{url},
		SyncInterval: 60 * time.Second,
		FrameSize:    128 * 1024,
		IDSize:       16,
		MaxEvents:    c.config.MaxEventsPerSync,
	}

	negMgr := negentropy.NewManager(c.db, negCfg)
	negMgr.TriggerSync(ctx, url)
	peerState, ok := negMgr.GetPeerState(url)

	result := syncResultReq{url: url}

	if !ok || peerState.Status == "error" {
		var lastError string
		if ok && peerState.LastError != "" {
			lastError = peerState.LastError
		} else {
			lastError = "sync failed"
		}
		result.success = false
		result.lastError = lastError
	} else {
		result.success = true
		result.eventsSynced = peerState.EventsSynced
	}

	c.syncResultCh <- result
}

func (c *Crawler) getRelaysFromLocalDB(seeds [][]byte) map[string]bool {
	relays := make(map[string]bool)

	f := &filter.F{
		Authors: tag.NewFromBytesSlice(seeds...),
		Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
	}

	events, err := c.db.QueryEvents(c.ctx, f)
	if err != nil {
		log.W.F("crawler: failed to query local relay lists: %v", err)
		return relays
	}

	for _, ev := range events {
		rTags := ev.Tags.GetAll([]byte("r"))
		for _, rTag := range rTags {
			if rTag.Len() < 2 {
				continue
			}
			urlBytes := rTag.Value()
			if len(urlBytes) == 0 {
				continue
			}
			normURL := string(normalize.URL(string(urlBytes)))
			if normURL != "" {
				relays[normURL] = true
			}
		}
	}

	return relays
}

func (c *Crawler) fetchRelayListsFromRelay(relayURL string) ([]string, error) {
	ctx, cancel := context.WithTimeout(c.ctx, DefaultRelayTimeout)
	defer cancel()

	client, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	defer client.Close()

	limit := uint(MaxEventsPerQuery)
	ff := filter.NewS(&filter.F{
		Kinds: kind.NewS(kind.New(kind.RelayListMetadata.K)),
		Limit: &limit,
	})

	sub, err := client.Subscribe(ctx, ff)
	if err != nil {
		return nil, fmt.Errorf("subscribe failed: %w", err)
	}
	defer sub.Unsub()

	var relays []string
	seen := make(map[string]bool)

	queryCtx, queryCancel := context.WithTimeout(ctx, QueryTimeout)
	defer queryCancel()

	for {
		select {
		case <-queryCtx.Done():
			return relays, nil
		case ev := <-sub.Events:
			if ev == nil {
				return relays, nil
			}

			if _, err := c.db.SaveEvent(c.ctx, ev); err != nil {
				log.D.F("crawler: save event failed: %v", err)
			}
			if c.pub != nil {
				c.pub.Deliver(ev)
			}

			rTags := ev.Tags.GetAll([]byte("r"))
			for _, rTag := range rTags {
				if rTag.Len() < 2 {
					continue
				}
				normURL := string(normalize.URL(string(rTag.Value())))
				if normURL != "" && !seen[normURL] {
					seen[normURL] = true
					relays = append(relays, normURL)
				}
			}
		case <-sub.EndOfStoredEvents:
			return relays, nil
		}
	}
}

// isSelfRelay checks if a relay URL belongs to this relay instance by comparing
// NIP-11 pubkeys. This does network IO and must NOT be called while holding state.
func (c *Crawler) isSelfRelay(relayURL string) bool {
	resp := make(chan bool, 1)
	c.selfURLCheckCh <- selfURLCheckReq{url: relayURL, resp: resp}
	cached := <-resp
	if cached {
		return true
	}
	if c.relayIdentityPubkey == "" {
		return false
	}

	pubkey, err := c.nip11Cache.GetPubkey(c.ctx, relayURL)
	if err != nil || pubkey == "" {
		return false
	}

	if pubkey == c.relayIdentityPubkey {
		c.selfURLSetCh <- selfURLSetReq{url: relayURL}
		return true
	}
	return false
}

// loadFrontier loads persisted frontier state from database markers.
// Only called during New() after actor is started.
func (c *Crawler) loadFrontier() error {
	data, err := c.db.GetMarker(markerFrontierKey)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	var frontier map[string]*RelayState
	if err := json.Unmarshal(data, &frontier); err != nil {
		return fmt.Errorf("unmarshal frontier: %w", err)
	}

	// Merge loaded frontier via actor
	filtered := make(map[string]int, len(frontier))
	for url, rs := range frontier {
		filtered[url] = rs.HopDistance
	}

	if len(filtered) > 0 {
		resp := make(chan frontierMergeResp, 1)
		c.frontierMergeCh <- frontierMergeReq{filtered: filtered, resp: resp}
		result := <-resp
		log.I.F("crawler: loaded frontier with %d relays", result.frontierSize)
	}

	statsData, err := c.db.GetMarker(markerStatsKey)
	if err == nil && len(statsData) > 0 {
		var stats Stats
		json.Unmarshal(statsData, &stats)
		_ = stats
	}

	return nil
}

func (c *Crawler) saveFrontier() error {
	resp := make(chan saveFrontierResp, 1)
	c.saveFrontierCh <- saveFrontierReq{resp: resp}
	r := <-resp
	if r.err != nil {
		return r.err
	}

	if err := c.db.SetMarker(markerFrontierKey, r.frontierData); err != nil {
		return fmt.Errorf("save frontier: %w", err)
	}
	if err := c.db.SetMarker(markerStatsKey, r.statsData); err != nil {
		return fmt.Errorf("save stats: %w", err)
	}

	return nil
}
