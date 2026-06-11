package spider

import (
	"context"
	"sync/atomic"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/utils/normalize"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/interfaces/publisher"
	dsync "git.smesh.lol/orly/pkg/sync"
)

const (
	// DirectorySpiderDefaultInterval is how often the directory spider runs
	DirectorySpiderDefaultInterval = 24 * time.Hour
	// DirectorySpiderDefaultMaxHops is the maximum hop distance for relay discovery
	DirectorySpiderDefaultMaxHops = 3
	// DirectorySpiderRelayTimeout is the timeout for connecting to and querying a relay
	DirectorySpiderRelayTimeout = 30 * time.Second
	// DirectorySpiderQueryTimeout is the timeout for waiting for EOSE on a query
	DirectorySpiderQueryTimeout = 60 * time.Second
	// DirectorySpiderRelayDelay is the delay between processing relays (rate limiting)
	DirectorySpiderRelayDelay = 500 * time.Millisecond
	// DirectorySpiderMaxEventsPerQuery is the limit for each query
	DirectorySpiderMaxEventsPerQuery = 5000
)

// DirectorySpider manages periodic relay discovery and metadata synchronization.
// It discovers relays by crawling kind 10002 (relay list) events, expanding outward
// in hops from seed pubkeys (whitelisted users), then fetches essential metadata
// events (kinds 0, 3, 10000, 10002) from all discovered relays.
type DirectorySpider struct {
	ctx    context.Context
	cancel context.CancelFunc
	db     *database.D
	pub    publisher.I

	// Configuration
	interval time.Duration
	maxHops  int

	// State
	running atomic.Bool
	lastRun time.Time

	// Relay discovery state (reset each run, owned by actor goroutine)
	discoveredRelays map[string]int  // URL -> hop distance
	processedRelays  map[string]bool // Already fetched metadata from

	// Self-detection (owned by actor goroutine)
	relayIdentityPubkey string
	selfURLs            map[string]bool
	nip11Cache          *dsync.NIP11Cache

	// Callback for getting seed pubkeys (owned by actor goroutine)
	getSeedPubkeys func() [][]byte

	// Actor channels
	setSeed    actor.Proc[func() [][]byte]
	getLastRun actor.Query[time.Time]
	trigger    actor.Inbox[struct{}]
	lc         actor.Lifecycle
}

// NewDirectorySpider creates a new DirectorySpider instance.
func NewDirectorySpider(
	ctx context.Context,
	db *database.D,
	pub publisher.I,
	interval time.Duration,
	maxHops int,
) (ds *DirectorySpider, err error) {
	if db == nil {
		err = errorf.E("database cannot be nil")
		return
	}

	if interval <= 0 {
		interval = DirectorySpiderDefaultInterval
	}
	if maxHops <= 0 {
		maxHops = DirectorySpiderDefaultMaxHops
	}

	ctx, cancel := context.WithCancel(ctx)

	// Get relay identity pubkey for self-detection
	var relayPubkey string
	if skb, err := db.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		pk, _ := keys.SecretBytesToPubKeyHex(skb)
		relayPubkey = pk
	}

	ds = &DirectorySpider{
		ctx:                 ctx,
		cancel:              cancel,
		db:                  db,
		pub:                 pub,
		interval:            interval,
		maxHops:             maxHops,
		discoveredRelays:    make(map[string]int),
		processedRelays:     make(map[string]bool),
		relayIdentityPubkey: relayPubkey,
		selfURLs:            make(map[string]bool),
		nip11Cache:          dsync.NewNIP11Cache(30 * time.Minute),
		setSeed:             actor.NewProc[func() [][]byte](),
		getLastRun:          actor.NewQuery[time.Time](),
		trigger:             actor.NewInbox[struct{}](1),
		lc:                  actor.NewLifecycle(),
	}

	return
}

// SetSeedCallback sets the callback function for getting seed pubkeys (whitelisted users).
func (ds *DirectorySpider) SetSeedCallback(getSeedPubkeys func() [][]byte) {
	if !ds.running.Load() {
		// Actor not running (pre-start), set directly.
		ds.getSeedPubkeys = getSeedPubkeys
		return
	}
	ds.setSeed.Call(getSeedPubkeys)
}

// Start begins the directory spider operation.
func (ds *DirectorySpider) Start() (err error) {
	if ds.running.Load() {
		err = errorf.E("directory spider already running")
		return
	}

	if ds.getSeedPubkeys == nil {
		err = errorf.E("seed callback must be set before starting")
		return
	}

	ds.running.Store(true)
	actor.Go(ds.lc, ds.actorLoop)

	log.I.F("directory spider: started (interval: %v, max hops: %d)", ds.interval, ds.maxHops)
	return
}

// Stop stops the directory spider operation.
func (ds *DirectorySpider) Stop() {
	if !ds.running.Load() {
		return
	}

	ds.running.Store(false)
	ds.cancel()

	log.I.F("directory spider: stopped")
}

// Shutdown stops the directory spider and waits for the actor to exit.
func (ds *DirectorySpider) Shutdown() {
	ds.lc.Stop()
}

// TriggerNow forces an immediate run of the directory spider.
func (ds *DirectorySpider) TriggerNow() {
	if ds.trigger.TrySend(struct{}{}) {
		log.D.F("directory spider: manual trigger sent")
	} else {
		log.D.F("directory spider: trigger already pending")
	}
}

// LastRun returns the time of the last completed run.
func (ds *DirectorySpider) LastRun() time.Time {
	if !ds.running.Load() {
		return ds.lastRun
	}
	return ds.getLastRun.Call()
}

// actorLoop is the actor goroutine that owns all mutable discovery state.
func (ds *DirectorySpider) actorLoop() {
	// Run immediately on start
	ds.runOnce()

	ticker := time.NewTicker(ds.interval)
	defer ticker.Stop()

	log.D.F("directory spider: actor loop started, running every %v", ds.interval)

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ds.lc.Stopping():
			return
		case msg := <-ds.setSeed.Recv():
			ds.getSeedPubkeys = msg.Req
			msg.Done()
		case msg := <-ds.getLastRun.Recv():
			msg.Reply(ds.lastRun)
		case <-ds.trigger.Recv():
			log.D.F("directory spider: manual trigger received")
			ds.runOnce()
		case <-ticker.C:
			log.D.F("directory spider: scheduled run triggered")
			ds.runOnce()
		}
	}
}

// runOnce performs a single directory spider run.
func (ds *DirectorySpider) runOnce() {
	if !ds.running.Load() {
		return
	}

	log.D.F("directory spider: starting run")
	start := time.Now()

	// Reset state for this run (actor-owned, no lock needed)
	ds.discoveredRelays = make(map[string]int)
	ds.processedRelays = make(map[string]bool)

	// Phase 1: Discover relays via hop expansion
	if err := ds.discoverRelays(); err != nil {
		log.E.F("directory spider: relay discovery failed: %v", err)
		return
	}

	relayCount := len(ds.discoveredRelays)

	log.D.F("directory spider: discovered %d relays", relayCount)

	// Phase 2: Fetch metadata from all discovered relays
	if err := ds.fetchMetadataFromRelays(); err != nil {
		log.E.F("directory spider: metadata fetch failed: %v", err)
		return
	}

	ds.lastRun = time.Now()

	log.D.F("directory spider: completed run in %v", time.Since(start))
}

// discoverRelays performs the multi-hop relay discovery.
func (ds *DirectorySpider) discoverRelays() error {
	// Get seed pubkeys from callback
	seedPubkeys := ds.getSeedPubkeys()
	if len(seedPubkeys) == 0 {
		log.W.F("directory spider: no seed pubkeys available")
		return nil
	}

	log.D.F("directory spider: starting relay discovery with %d seed pubkeys", len(seedPubkeys))

	// Round 0: Get relay lists from seed pubkeys in local database
	seedRelays, err := ds.getRelaysFromLocalDB(seedPubkeys)
	if err != nil {
		return errorf.W("failed to get relays from local DB: %v", err)
	}

	// Filter out self-relays WITHOUT holding mu - isSelfRelay takes mu internally
	var nonSelfRelays []string
	for _, url := range seedRelays {
		if !ds.isSelfRelay(url) {
			nonSelfRelays = append(nonSelfRelays, url)
		}
	}

	// Add seed relays at hop 0 (actor-owned, no lock needed)
	for _, url := range nonSelfRelays {
		ds.discoveredRelays[url] = 0
	}

	log.D.F("directory spider: found %d seed relays from local database", len(seedRelays))

	// Rounds 1 to maxHops: Expand outward
	for hop := 1; hop <= ds.maxHops; hop++ {
		select {
		case <-ds.ctx.Done():
			return ds.ctx.Err()
		default:
		}

		// Get relays at previous hop level that haven't been processed
		var relaysToProcess []string
		for url, hopLevel := range ds.discoveredRelays {
			if hopLevel == hop-1 && !ds.processedRelays[url] {
				relaysToProcess = append(relaysToProcess, url)
			}
		}

		if len(relaysToProcess) == 0 {
			log.D.F("directory spider: no relays to process at hop %d", hop)
			break
		}

		log.D.F("directory spider: hop %d - processing %d relays", hop, len(relaysToProcess))

		newRelaysThisHop := 0

		// Process each relay serially
		for _, relayURL := range relaysToProcess {
			select {
			case <-ds.ctx.Done():
				return ds.ctx.Err()
			default:
			}

			// Fetch kind 10002 events from this relay
			events, err := ds.fetchRelayListsFromRelay(relayURL)
			if err != nil {
				log.W.F("directory spider: failed to fetch from %s: %v", relayURL, err)
				// Mark as processed even on failure to avoid retrying
				ds.processedRelays[relayURL] = true
				continue
			}

			// Extract new relay URLs
			newRelays := ds.extractRelaysFromEvents(events)

			// Filter unknown relays (actor-owned, no lock needed)
			var unknownRelays []string
			for _, newURL := range newRelays {
				if _, exists := ds.discoveredRelays[newURL]; !exists {
					unknownRelays = append(unknownRelays, newURL)
				}
			}
			ds.processedRelays[relayURL] = true

			var nonSelfNew []string
			for _, newURL := range unknownRelays {
				if !ds.isSelfRelay(newURL) {
					nonSelfNew = append(nonSelfNew, newURL)
				}
			}

			for _, newURL := range nonSelfNew {
				if _, exists := ds.discoveredRelays[newURL]; !exists {
					ds.discoveredRelays[newURL] = hop
					newRelaysThisHop++
				}
			}

			// Rate limiting delay between relays
			time.Sleep(DirectorySpiderRelayDelay)
		}

		log.D.F("directory spider: hop %d - discovered %d new relays", hop, newRelaysThisHop)
	}

	return nil
}

// getRelaysFromLocalDB queries the local database for kind 10002 events from seed pubkeys.
func (ds *DirectorySpider) getRelaysFromLocalDB(seedPubkeys [][]byte) ([]string, error) {
	ctx, cancel := context.WithTimeout(ds.ctx, 30*time.Second)
	defer cancel()

	// Query for kind 10002 from seed pubkeys
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(seedPubkeys...),
		Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
	}

	events, err := ds.db.QueryEvents(ctx, f)
	if err != nil {
		return nil, err
	}

	return ds.extractRelaysFromEvents(events), nil
}

// fetchRelayListsFromRelay connects to a relay and fetches all kind 10002 events.
func (ds *DirectorySpider) fetchRelayListsFromRelay(relayURL string) ([]*event.E, error) {
	ctx, cancel := context.WithTimeout(ds.ctx, DirectorySpiderRelayTimeout)
	defer cancel()

	log.D.F("directory spider: connecting to %s", relayURL)

	client, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, errorf.W("failed to connect: %v", err)
	}
	defer client.Close()

	// Query for all kind 10002 events
	limit := uint(DirectorySpiderMaxEventsPerQuery)
	f := filter.NewS(&filter.F{
		Kinds: kind.NewS(kind.New(kind.RelayListMetadata.K)),
		Limit: &limit,
	})

	sub, err := client.Subscribe(ctx, f)
	if err != nil {
		return nil, errorf.W("failed to subscribe: %v", err)
	}
	defer sub.Unsub()

	var events []*event.E
	queryCtx, queryCancel := context.WithTimeout(ctx, DirectorySpiderQueryTimeout)
	defer queryCancel()

	// Collect events until EOSE or timeout
	for {
		select {
		case <-queryCtx.Done():
			log.D.F("directory spider: query timeout for %s, got %d events", relayURL, len(events))
			return events, nil
		case <-sub.EndOfStoredEvents:
			log.D.F("directory spider: EOSE from %s, got %d events", relayURL, len(events))
			return events, nil
		case ev := <-sub.Events:
			if ev == nil {
				return events, nil
			}
			events = append(events, ev)
		}
	}
}

// extractRelaysFromEvents parses kind 10002 events and extracts relay URLs from "r" tags.
func (ds *DirectorySpider) extractRelaysFromEvents(events []*event.E) []string {
	seen := make(map[string]bool)
	var relays []string

	for _, ev := range events {
		// Get all "r" tags
		rTags := ev.Tags.GetAll([]byte("r"))
		for _, rTag := range rTags {
			if len(rTag.T) < 2 {
				continue
			}
			urlBytes := rTag.Value()
			if len(urlBytes) == 0 {
				continue
			}

			// Normalize the URL
			normalized := string(normalize.URL(string(urlBytes)))
			if normalized == "" {
				continue
			}

			if !seen[normalized] {
				seen[normalized] = true
				relays = append(relays, normalized)
			}
		}
	}

	return relays
}

// fetchMetadataFromRelays iterates through all discovered relays and fetches metadata.
func (ds *DirectorySpider) fetchMetadataFromRelays() error {
	// Copy relay list (actor-owned, no lock needed)
	var relays []string
	for url := range ds.discoveredRelays {
		relays = append(relays, url)
	}

	log.D.F("directory spider: fetching metadata from %d relays", len(relays))

	// Kinds to fetch: 0 (profile), 3 (follow list), 10000 (mute list), 10002 (relay list)
	kindsToFetch := []uint16{
		kind.ProfileMetadata.K,   // 0
		kind.FollowList.K,        // 3
		kind.MuteList.K,          // 10000
		kind.RelayListMetadata.K, // 10002
	}

	totalSaved := 0
	totalDuplicates := 0

	for _, relayURL := range relays {
		select {
		case <-ds.ctx.Done():
			return ds.ctx.Err()
		default:
		}

		alreadyProcessed := ds.processedRelays[relayURL]

		if alreadyProcessed {
			continue
		}

		log.D.F("directory spider: fetching metadata from %s", relayURL)

		for _, k := range kindsToFetch {
			select {
			case <-ds.ctx.Done():
				return ds.ctx.Err()
			default:
			}

			events, err := ds.fetchKindFromRelay(relayURL, k)
			if err != nil {
				log.W.F("directory spider: failed to fetch kind %d from %s: %v", k, relayURL, err)
				continue
			}

			saved, duplicates := ds.storeEvents(events)
			totalSaved += saved
			totalDuplicates += duplicates

			log.D.F("directory spider: kind %d from %s: %d saved, %d duplicates",
				k, relayURL, saved, duplicates)
		}

		ds.processedRelays[relayURL] = true

		// Rate limiting delay between relays
		time.Sleep(DirectorySpiderRelayDelay)
	}

	log.D.F("directory spider: metadata fetch complete - %d events saved, %d duplicates",
		totalSaved, totalDuplicates)

	return nil
}

// fetchKindFromRelay connects to a relay and fetches events of a specific kind.
func (ds *DirectorySpider) fetchKindFromRelay(relayURL string, k uint16) ([]*event.E, error) {
	ctx, cancel := context.WithTimeout(ds.ctx, DirectorySpiderRelayTimeout)
	defer cancel()

	client, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, errorf.W("failed to connect: %v", err)
	}
	defer client.Close()

	// Query for events of this kind
	limit := uint(DirectorySpiderMaxEventsPerQuery)
	f := filter.NewS(&filter.F{
		Kinds: kind.NewS(kind.New(k)),
		Limit: &limit,
	})

	sub, err := client.Subscribe(ctx, f)
	if err != nil {
		return nil, errorf.W("failed to subscribe: %v", err)
	}
	defer sub.Unsub()

	var events []*event.E
	queryCtx, queryCancel := context.WithTimeout(ctx, DirectorySpiderQueryTimeout)
	defer queryCancel()

	for {
		select {
		case <-queryCtx.Done():
			return events, nil
		case <-sub.EndOfStoredEvents:
			return events, nil
		case ev := <-sub.Events:
			if ev == nil {
				return events, nil
			}
			events = append(events, ev)
		}
	}
}

// storeEvents saves events to the database and publishes new ones.
func (ds *DirectorySpider) storeEvents(events []*event.E) (saved, duplicates int) {
	for _, ev := range events {
		_, err := ds.db.SaveEvent(ds.ctx, ev)
		if err != nil {
			if chk.T(err) {
				// Most errors are duplicates, which is expected
				duplicates++
			}
			continue
		}
		saved++

		// Publish event to active subscribers
		if ds.pub != nil {
			go ds.pub.Deliver(ev)
		}
	}
	return
}

// isSelfRelay checks if a relay URL is ourselves by comparing NIP-11 pubkeys.
func (ds *DirectorySpider) isSelfRelay(relayURL string) bool {
	// If we don't have a relay identity pubkey, can't compare
	if ds.relayIdentityPubkey == "" {
		return false
	}

	// Fast path: check if we already know this URL is ours (actor-owned, no lock needed)
	if ds.selfURLs[relayURL] {
		return true
	}

	// Slow path: check via NIP-11 pubkey
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	peerPubkey, err := ds.nip11Cache.GetPubkey(ctx, relayURL)
	if err != nil {
		// Can't determine, assume not self
		return false
	}

	if peerPubkey == ds.relayIdentityPubkey {
		log.D.F("directory spider: discovered self-relay: %s", relayURL)
		ds.selfURLs[relayURL] = true
		return true
	}

	return false
}
