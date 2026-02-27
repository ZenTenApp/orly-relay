// Package crawler provides an automated corpus crawler that discovers relays
// via kind 10002 hop-expansion and then syncs all events from each discovered
// relay using NIP-77 negentropy set reconciliation.
//
// Architecture:
//
//	Discovery loop expands relay URLs from seed pubkeys via kind 10002 →
//	Crawler maintains a persistent frontier of relay entries with per-relay
//	state → Sync loop picks relays due for sync and runs negentropy
//	reconciliation with bounded concurrency → Frontier state is persisted
//	to Badger markers so it survives restarts.
package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	gosync "sync"
	"sync/atomic"
	"time"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/interfaces/publisher"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/utils/normalize"
	"next.orly.dev/pkg/nostr/ws"
	dsync "next.orly.dev/pkg/sync"
	"next.orly.dev/pkg/sync/negentropy"
)

const (
	// DefaultDiscoveryInterval is how often the crawler runs relay discovery.
	DefaultDiscoveryInterval = 4 * time.Hour

	// DefaultSyncInterval is how often the crawler re-syncs known relays.
	DefaultSyncInterval = 30 * time.Minute

	// DefaultMaxHops is the maximum hop distance for relay discovery.
	DefaultMaxHops = 5

	// DefaultConcurrency is how many relays to sync concurrently.
	DefaultConcurrency = 3

	// DefaultRelayTimeout is the timeout for connecting to a relay.
	DefaultRelayTimeout = 30 * time.Second

	// QueryTimeout is the timeout for waiting for EOSE on a query.
	QueryTimeout = 60 * time.Second

	// DefaultSyncTimeout is the maximum time for a single negentropy sync.
	DefaultSyncTimeout = 10 * time.Minute

	// DefaultRelayDelay is the delay between processing relays (rate limit).
	DefaultRelayDelay = 500 * time.Millisecond

	// DefaultMaxFailures is how many consecutive failures before blacklisting.
	DefaultMaxFailures = 5

	// DefaultBlacklistDuration is how long a relay stays blacklisted.
	DefaultBlacklistDuration = 24 * time.Hour

	// DefaultMaxEventsPerSync is the max events for negentropy vector building.
	DefaultMaxEventsPerSync = 1_000_000

	// MaxEventsPerQuery caps the number of events in a single relay list query.
	MaxEventsPerQuery = 5000

	// markerFrontierKey is the Badger marker key for the full frontier.
	markerFrontierKey = "crawler:frontier"

	// markerStatsKey is the Badger marker key for crawler statistics.
	markerStatsKey = "crawler:stats"
)

// Config holds configuration for the corpus crawler.
type Config struct {
	// Discovery settings
	DiscoveryInterval time.Duration
	MaxHops           int

	// Sync settings
	SyncInterval     time.Duration
	Concurrency      int
	SyncTimeout      time.Duration
	MaxEventsPerSync uint

	// Rate limiting
	RelayDelay time.Duration

	// Failure handling
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

// Crawler orchestrates relay discovery and corpus sync.
type Crawler struct {
	ctx    context.Context
	cancel context.CancelFunc

	db     database.Database
	pub    publisher.I
	config *Config

	mu       gosync.RWMutex
	frontier map[string]*RelayState
	stats    Stats

	relayIdentityPubkey string
	selfURLs            map[string]bool
	nip11Cache          *dsync.NIP11Cache

	getSeedPubkeys func() [][]byte

	running  atomic.Bool
	stopChan chan struct{}
	wg       gosync.WaitGroup
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
	}

	if err := c.loadFrontier(); err != nil {
		log.W.F("crawler: failed to load frontier: %v (starting fresh)", err)
	}

	return c, nil
}

// SetSeedCallback sets the callback for getting seed pubkeys used in discovery.
func (c *Crawler) SetSeedCallback(fn func() [][]byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.getSeedPubkeys = fn
}

// Start begins the crawler's discovery and sync loops.
func (c *Crawler) Start() error {
	if c.running.Load() {
		return fmt.Errorf("crawler already running")
	}
	if c.getSeedPubkeys == nil {
		return fmt.Errorf("seed callback must be set before starting")
	}

	c.running.Store(true)
	c.stopChan = make(chan struct{})

	c.wg.Add(2)
	go c.discoveryLoop()
	go c.syncLoop()

	log.I.F("crawler: started (discovery: %v, sync: %v, hops: %d, concurrency: %d)",
		c.config.DiscoveryInterval, c.config.SyncInterval,
		c.config.MaxHops, c.config.Concurrency)

	return nil
}

// Stop stops the crawler.
func (c *Crawler) Stop() {
	if !c.running.Load() {
		return
	}
	c.running.Store(false)
	close(c.stopChan)
	c.cancel()
	c.wg.Wait()

	if err := c.saveFrontier(); err != nil {
		log.W.F("crawler: failed to save frontier on stop: %v", err)
	}

	log.I.F("crawler: stopped (frontier: %d relays, total events synced: %d)",
		len(c.frontier), c.stats.TotalEventsSynced)
}

// GetStats returns a snapshot of crawler statistics.
func (c *Crawler) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// GetFrontierSize returns the number of relays in the frontier.
func (c *Crawler) GetFrontierSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.frontier)
}

// discoveryLoop periodically discovers new relays via kind 10002 hop expansion.
func (c *Crawler) discoveryLoop() {
	defer c.wg.Done()

	c.runDiscovery()

	ticker := time.NewTicker(c.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.runDiscovery()
		}
	}
}

// syncLoop periodically syncs events from relays in the frontier.
func (c *Crawler) syncLoop() {
	defer c.wg.Done()

	// Wait for discovery to populate the frontier
	select {
	case <-c.stopChan:
		return
	case <-time.After(30 * time.Second):
	}

	c.runSyncCycle()

	ticker := time.NewTicker(c.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.runSyncCycle()
		}
	}
}

// runDiscovery performs one relay discovery cycle using kind 10002 hop expansion.
func (c *Crawler) runDiscovery() {
	log.I.F("crawler: starting relay discovery (max hops: %d)", c.config.MaxHops)

	seeds := c.getSeedPubkeys()
	if len(seeds) == 0 {
		log.W.F("crawler: no seed pubkeys, skipping discovery")
		return
	}

	discovered := make(map[string]int) // URL -> hop distance

	// Hop 0: get relay lists from local DB for seed pubkeys
	localRelays := c.getRelaysFromLocalDB(seeds)
	for url := range localRelays {
		discovered[url] = 0
	}

	log.I.F("crawler: hop 0 discovered %d relays from %d seed pubkeys", len(discovered), len(seeds))

	// Hops 1..N: expand by fetching kind 10002 from relays at each hop
	for hop := 1; hop <= c.config.MaxHops; hop++ {
		select {
		case <-c.stopChan:
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
			case <-c.stopChan:
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

	// Merge into frontier
	c.mu.Lock()
	added := 0
	for url, hopDist := range discovered {
		normURL := string(normalize.URL(url))
		if normURL == "" {
			continue
		}
		if c.isSelfRelay(normURL) {
			continue
		}

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
	c.mu.Unlock()

	log.I.F("crawler: discovery complete — %d new relays added, frontier size: %d",
		added, len(c.frontier))

	if err := c.saveFrontier(); err != nil {
		log.W.F("crawler: failed to save frontier: %v", err)
	}
}

// runSyncCycle syncs events from relays that are due for a sync.
func (c *Crawler) runSyncCycle() {
	c.mu.RLock()
	var due []*RelayState
	for _, rs := range c.frontier {
		if rs.needsSync(c.config.SyncInterval) {
			due = append(due, rs)
		}
	}
	c.mu.RUnlock()

	if len(due) == 0 {
		log.D.F("crawler: no relays due for sync")
		return
	}

	log.I.F("crawler: starting sync cycle — %d relays due", len(due))

	sem := make(chan struct{}, c.config.Concurrency)
	var syncWg gosync.WaitGroup

	for _, rs := range due {
		select {
		case <-c.stopChan:
			syncWg.Wait()
			return
		default:
		}

		sem <- struct{}{}
		syncWg.Add(1)

		go func(relay *RelayState) {
			defer syncWg.Done()
			defer func() { <-sem }()
			c.syncRelay(relay)
		}(rs)
	}

	syncWg.Wait()

	c.mu.Lock()
	c.stats.LastSyncRun = time.Now()
	c.mu.Unlock()

	if err := c.saveFrontier(); err != nil {
		log.W.F("crawler: failed to save frontier: %v", err)
	}

	log.I.F("crawler: sync cycle complete")
}

// syncRelay performs a negentropy sync with a single relay.
func (c *Crawler) syncRelay(rs *RelayState) {
	ctx, cancel := context.WithTimeout(c.ctx, c.config.SyncTimeout)
	defer cancel()

	log.I.F("crawler: syncing %s (hop %d, syncs: %d)", rs.URL, rs.HopDistance, rs.TotalSyncs)

	// Create a one-shot negentropy manager for this relay
	negCfg := &negentropy.Config{
		Peers:        []string{rs.URL},
		SyncInterval: 60 * time.Second, // Not used — we trigger manually
		FrameSize:    128 * 1024,
		IDSize:       16,
		MaxEvents:    c.config.MaxEventsPerSync,
	}

	negMgr := negentropy.NewManager(c.db, negCfg)

	// Trigger a one-shot sync (blocks until complete via syncWithPeer)
	negMgr.TriggerSync(ctx, rs.URL)

	// Read back the peer state to get results
	peerState, ok := negMgr.GetPeerState(rs.URL)

	c.mu.Lock()
	rs.LastSync = time.Now()
	rs.TotalSyncs++

	if !ok || peerState.Status == "error" {
		rs.ConsecFailures++
		if ok && peerState.LastError != "" {
			rs.LastError = peerState.LastError
		} else {
			rs.LastError = "sync failed"
		}

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
		eventsSynced := peerState.EventsSynced
		rs.ConsecFailures = 0
		rs.LastError = ""
		rs.EventsSynced += eventsSynced
		c.stats.TotalEventsSynced += eventsSynced
		c.stats.TotalRelaysSynced++

		if eventsSynced > 0 {
			log.I.F("crawler: synced %d events from %s", eventsSynced, rs.URL)
		}
	}
	c.mu.Unlock()
}

// getRelaysFromLocalDB queries the local database for kind 10002 events from
// the given seed pubkeys and extracts relay URLs.
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

// fetchRelayListsFromRelay connects to a relay and fetches kind 10002 events
// to discover more relay URLs.
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

			// Store event locally
			c.db.SaveEvent(c.ctx, ev)
			if c.pub != nil {
				c.pub.Deliver(ev)
			}

			// Extract relay URLs from "r" tags
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
// NIP-11 pubkeys.
func (c *Crawler) isSelfRelay(relayURL string) bool {
	if c.selfURLs[relayURL] {
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
		c.selfURLs[relayURL] = true
		return true
	}
	return false
}

// loadFrontier loads persisted frontier state from database markers.
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

	c.frontier = frontier
	log.I.F("crawler: loaded frontier with %d relays", len(frontier))

	statsData, err := c.db.GetMarker(markerStatsKey)
	if err == nil && len(statsData) > 0 {
		json.Unmarshal(statsData, &c.stats)
	}

	return nil
}

// saveFrontier persists the frontier state to database markers.
func (c *Crawler) saveFrontier() error {
	c.mu.RLock()
	data, err := json.Marshal(c.frontier)
	if err != nil {
		c.mu.RUnlock()
		return fmt.Errorf("marshal frontier: %w", err)
	}
	statsData, err := json.Marshal(c.stats)
	c.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal stats: %w", err)
	}

	if err := c.db.SetMarker(markerFrontierKey, data); err != nil {
		return fmt.Errorf("save frontier: %w", err)
	}
	if err := c.db.SetMarker(markerStatsKey, statsData); err != nil {
		return fmt.Errorf("save stats: %w", err)
	}

	return nil
}
