package acl

import (
	"context"
	"encoding/hex"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/envelopes"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eoseenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/reqenvelope"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/protocol/publish"
	"git.mleku.dev/mleku/nostr/utils/normalize"
	"git.mleku.dev/mleku/nostr/utils/values"
)

type Follows struct {
	// Ctx holds the context for the ACL.
	// Deprecated: Use Context() method instead of accessing directly.
	Ctx context.Context
	cfg *config.C
	db  database.Database
	pubs       *publish.S
	followsMx  sync.RWMutex
	admins     [][]byte
	owners     [][]byte
	follows    [][]byte
	// Map-based caches for O(1) lookups (hex pubkey -> struct{})
	ownersSet  map[string]struct{}
	adminsSet  map[string]struct{}
	followsSet map[string]struct{}
	// Track last follow list fetch time
	lastFollowListFetch time.Time
	// Callback for external notification of follow list changes
	onFollowListUpdate func()
	// Progressive throttle for non-followed users (nil if disabled)
	throttle *ProgressiveThrottle
}

// Context returns the ACL context.
func (f *Follows) Context() context.Context {
	return f.Ctx
}

func (f *Follows) Configure(cfg ...any) (err error) {
	log.I.F("configuring follows ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			// log.D.F("setting ACL config: %v", c)
			f.cfg = c
		case database.Database:
			// log.D.F("setting ACL database: %s", c.Path())
			f.db = c
		case context.Context:
			// log.D.F("setting ACL context: %s", c.Value("id"))
			f.Ctx = c
		case *publish.S:
			// set publisher for dispatching new events
			f.pubs = c
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if f.cfg == nil || f.db == nil {
		err = errorf.E("both config and database must be set")
		return
	}

	// Build all lists in local variables WITHOUT holding the lock
	// This prevents blocking GetAccessLevel calls during slow database I/O
	var newOwners [][]byte
	newOwnersSet := make(map[string]struct{})
	var newAdmins [][]byte
	newAdminsSet := make(map[string]struct{})
	var newFollows [][]byte
	newFollowsSet := make(map[string]struct{})

	// add owners list
	for _, owner := range f.cfg.Owners {
		var own []byte
		if o, e := bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(e) {
			continue
		} else {
			own = o
		}
		newOwners = append(newOwners, own)
		newOwnersSet[hex.EncodeToString(own)] = struct{}{}
	}

	// parse admin pubkeys
	var adminBinaries [][]byte
	for _, admin := range f.cfg.Admins {
		var adm []byte
		if a, e := bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(e) {
			continue
		} else {
			adm = a
		}
		newAdmins = append(newAdmins, adm)
		newAdminsSet[hex.EncodeToString(adm)] = struct{}{}
		adminBinaries = append(adminBinaries, adm)
	}

	// Batch query all admin follow lists in a single DB call
	// Kind 3 is replaceable, so QueryEvents returns only the latest per author
	if len(adminBinaries) > 0 {
		ctx := f.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		fl := &filter.F{
			Authors: tag.NewFromBytesSlice(adminBinaries...),
			Kinds:   kind.NewS(kind.New(kind.FollowList.K)),
			Limit:   values.ToUintPointer(uint(len(adminBinaries))),
		}
		var evs event.S
		if evs, err = f.db.QueryEvents(ctx, fl); err != nil {
			log.W.F("follows ACL: error querying admin follow lists: %v", err)
			err = nil // Don't fail Configure on query error
		}
		for _, ev := range evs {
			for _, v := range ev.Tags.GetAll([]byte("p")) {
				// ValueHex() automatically handles both binary and hex storage formats
				if b, e := hex.DecodeString(string(v.ValueHex())); chk.E(e) {
					continue
				} else {
					hexKey := hex.EncodeToString(b)
					if _, exists := newFollowsSet[hexKey]; !exists {
						newFollows = append(newFollows, b)
						newFollowsSet[hexKey] = struct{}{}
					}
				}
			}
		}
	}

	// Now acquire the lock ONLY for the quick swap operation
	f.followsMx.Lock()
	f.owners = newOwners
	f.ownersSet = newOwnersSet
	f.admins = newAdmins
	f.adminsSet = newAdminsSet
	f.follows = newFollows
	f.followsSet = newFollowsSet
	f.followsMx.Unlock()

	log.I.F("follows ACL configured: %d owners, %d admins, %d follows",
		len(newOwners), len(newAdmins), len(newFollows))

	// Initialize progressive throttle if enabled
	if f.cfg.FollowsThrottleEnabled {
		perEvent := f.cfg.FollowsThrottlePerEvent
		if perEvent == 0 {
			perEvent = 25 * time.Millisecond
		}
		maxDelay := f.cfg.FollowsThrottleMaxDelay
		if maxDelay == 0 {
			maxDelay = 60 * time.Second
		}
		f.throttle = NewProgressiveThrottle(perEvent, maxDelay)
		log.I.F("follows ACL: progressive throttle enabled (increment: %v, max: %v)",
			perEvent, maxDelay)
	}

	return
}

func (f *Follows) GetAccessLevel(pub []byte, address string) (level string) {
	pubHex := hex.EncodeToString(pub)

	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	// O(1) map lookups instead of O(n) linear scans
	if f.ownersSet != nil {
		if _, ok := f.ownersSet[pubHex]; ok {
			return "owner"
		}
	}
	if f.adminsSet != nil {
		if _, ok := f.adminsSet[pubHex]; ok {
			return "admin"
		}
	}
	if f.followsSet != nil {
		if _, ok := f.followsSet[pubHex]; ok {
			return "write"
		}
	}

	if f.cfg == nil {
		return "write"
	}
	// If throttle enabled, non-followed users get write access (with delay applied in handle-event)
	if f.throttle != nil {
		return "write"
	}
	return "read"
}

func (f *Follows) GetACLInfo() (name, description, documentation string) {
	return "follows", "whitelist follows of admins",
		`This ACL mode searches for follow lists of admins and grants all followers write access`
}

func (f *Follows) Type() string { return "follows" }

// GetThrottleDelay returns the progressive throttle delay for this event.
// Returns 0 if throttle is disabled or if the user is exempt (owner/admin/followed).
func (f *Follows) GetThrottleDelay(pubkey []byte, ip string) time.Duration {
	if f.throttle == nil {
		return 0
	}

	pubkeyHex := hex.EncodeToString(pubkey)

	// Check if user is exempt from throttling using O(1) map lookups
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	// Owners bypass throttle
	if f.ownersSet != nil {
		if _, ok := f.ownersSet[pubkeyHex]; ok {
			return 0
		}
	}
	// Admins bypass throttle
	if f.adminsSet != nil {
		if _, ok := f.adminsSet[pubkeyHex]; ok {
			return 0
		}
	}
	// Followed users bypass throttle
	if f.followsSet != nil {
		if _, ok := f.followsSet[pubkeyHex]; ok {
			return 0
		}
	}

	// Non-followed users get throttled
	return f.throttle.GetDelay(ip, pubkeyHex)
}

func (f *Follows) adminRelays() (urls []string) {
	f.followsMx.RLock()
	admins := make([][]byte, len(f.admins))
	copy(admins, f.admins)
	f.followsMx.RUnlock()
	seen := make(map[string]struct{})
	// Build a set of normalized self relay addresses to avoid self-connections
	selfSet := make(map[string]struct{})
	selfHosts := make(map[string]struct{})
	if f.cfg != nil && len(f.cfg.RelayAddresses) > 0 {
		for _, a := range f.cfg.RelayAddresses {
			n := string(normalize.URL(a))
			if n == "" {
				continue
			}
			selfSet[n] = struct{}{}
			// Also record hostname (without port) for robust matching
			// Accept simple splitting; normalize.URL ensures scheme://host[:port]
			host := n
			if i := strings.Index(host, "://"); i >= 0 {
				host = host[i+3:]
			}
			if j := strings.Index(host, "/"); j >= 0 {
				host = host[:j]
			}
			if k := strings.Index(host, ":"); k >= 0 {
				host = host[:k]
			}
			if host != "" {
				selfHosts[host] = struct{}{}
			}
		}
	}

	// Batch query all admin relay list events in a single DB call
	// Kind 10002 is replaceable, so QueryEvents returns only the latest per author
	if len(admins) > 0 {
		ctx := f.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		fl := &filter.F{
			Authors: tag.NewFromBytesSlice(admins...),
			Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
			Limit:   values.ToUintPointer(uint(len(admins))),
		}
		evs, qerr := f.db.QueryEvents(ctx, fl)
		if qerr != nil {
			log.W.F("follows ACL: error querying admin relay lists: %v", qerr)
		}
		for _, ev := range evs {
			for _, v := range ev.Tags.GetAll([]byte("r")) {
				u := string(v.Value())
				n := string(normalize.URL(u))
				if n == "" {
					continue
				}
				// Skip if this URL is one of our configured self relay addresses or hosts
				if _, isSelf := selfSet[n]; isSelf {
					log.D.F("follows syncer: skipping configured self relay address: %s", n)
					continue
				}
				// Host match
				host := n
				if i := strings.Index(host, "://"); i >= 0 {
					host = host[i+3:]
				}
				if j := strings.Index(host, "/"); j >= 0 {
					host = host[:j]
				}
				if k := strings.Index(host, ":"); k >= 0 {
					host = host[:k]
				}
				if _, isSelfHost := selfHosts[host]; isSelfHost {
					log.D.F("follows syncer: skipping configured self relay address: %s", n)
					continue
				}
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				urls = append(urls, n)
			}
		}
	}

	// If no admin relays found, use bootstrap relays as fallback
	if len(urls) == 0 {
		log.I.F("no admin relays found in DB, checking bootstrap relays and failover relays")
		if len(f.cfg.BootstrapRelays) > 0 {
			log.I.F("using bootstrap relays: %v", f.cfg.BootstrapRelays)
			for _, relay := range f.cfg.BootstrapRelays {
				n := string(normalize.URL(relay))
				if n == "" {
					log.W.F("invalid bootstrap relay URL: %s", relay)
					continue
				}
				// Skip if this URL is one of our configured self relay addresses or hosts
				if _, isSelf := selfSet[n]; isSelf {
					log.D.F("follows syncer: skipping configured self relay address: %s", n)
					continue
				}
				// Host match
				host := n
				if i := strings.Index(host, "://"); i >= 0 {
					host = host[i+3:]
				}
				if j := strings.Index(host, "/"); j >= 0 {
					host = host[:j]
				}
				if k := strings.Index(host, ":"); k >= 0 {
					host = host[:k]
				}
				if _, isSelfHost := selfHosts[host]; isSelfHost {
					log.D.F("follows syncer: skipping configured self relay address: %s", n)
					continue
				}
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				urls = append(urls, n)
			}
		} else {
			log.I.F("no bootstrap relays configured, using failover relays")
		}

		// If still no relays found, use hardcoded failover relays
		// These relays will be used to fetch admin relay lists (kind 10002) and store them
		// in the database so they're found next time
		if len(urls) == 0 {
			failoverRelays := []string{
				"wss://nostr.land",
				"wss://nostr.wine",
				"wss://nos.lol",
				"wss://relay.damus.io",
			}
			log.I.F("using failover relays: %v", failoverRelays)
			for _, relay := range failoverRelays {
				n := string(normalize.URL(relay))
				if n == "" {
					log.W.F("invalid failover relay URL: %s", relay)
					continue
				}
				// Skip if this URL is one of our configured self relay addresses or hosts
				if _, isSelf := selfSet[n]; isSelf {
					log.D.F("follows syncer: skipping configured self relay address: %s", n)
					continue
				}
				// Host match
				host := n
				if i := strings.Index(host, "://"); i >= 0 {
					host = host[i+3:]
				}
				if j := strings.Index(host, "/"); j >= 0 {
					host = host[:j]
				}
				if k := strings.Index(host, ":"); k >= 0 {
					host = host[:k]
				}
				if _, isSelfHost := selfHosts[host]; isSelfHost {
					log.D.F("follows syncer: skipping configured self relay address: %s", n)
					continue
				}
				if _, ok := seen[n]; ok {
					continue
				}
				seen[n] = struct{}{}
				urls = append(urls, n)
			}
		}
	}

	return
}


func (f *Follows) Syncer() {
	log.I.F("starting follows syncer")

	// Start periodic follow list and metadata fetching
	go f.startPeriodicFollowListFetching()

	// Start throttle cleanup goroutine if throttle is enabled
	if f.throttle != nil {
		go f.throttleCleanup()
	}
}

// throttleCleanup periodically removes fully-decayed throttle entries
func (f *Follows) throttleCleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-f.Ctx.Done():
			return
		case <-ticker.C:
			f.throttle.Cleanup()
			ipCount, pubkeyCount := f.throttle.Stats()
			log.T.F("follows throttle: cleanup complete, tracking %d IPs and %d pubkeys",
				ipCount, pubkeyCount)
		}
	}
}

// startPeriodicFollowListFetching starts periodic fetching of admin follow lists
func (f *Follows) startPeriodicFollowListFetching() {
	frequency := f.cfg.FollowListFrequency
	if frequency == 0 {
		frequency = time.Hour // Default to 1 hour
	}

	log.I.F("starting periodic follow list fetching every %v", frequency)

	ticker := time.NewTicker(frequency)
	defer ticker.Stop()

	// Fetch immediately on startup
	f.fetchAdminFollowLists()

	for {
		select {
		case <-f.Ctx.Done():
			log.D.F("periodic follow list fetching stopped due to context cancellation")
			return
		case <-ticker.C:
			f.fetchAdminFollowLists()
		}
	}
}

// fetchAdminFollowLists fetches follow lists for admins and metadata for all follows
func (f *Follows) fetchAdminFollowLists() {
	log.I.F("follows syncer: fetching admin follow lists and follows metadata")

	urls := f.adminRelays()
	if len(urls) == 0 {
		log.W.F("follows syncer: no relays available for follow list fetching (no admin relays, bootstrap relays, or failover relays)")
		return
	}

	// build authors lists: admins for follow lists, all follows for metadata
	f.followsMx.RLock()
	admins := make([][]byte, len(f.admins))
	copy(admins, f.admins)
	allFollows := make([][]byte, 0, len(f.admins)+len(f.follows))
	allFollows = append(allFollows, f.admins...)
	allFollows = append(allFollows, f.follows...)
	f.followsMx.RUnlock()

	if len(admins) == 0 {
		log.W.F("follows syncer: no admins to fetch follow lists for")
		return
	}

	log.I.F("follows syncer: fetching from %d relays: follow lists for %d admins, metadata for %d follows",
		len(urls), len(admins), len(allFollows))

	for _, u := range urls {
		go f.fetchFollowListsFromRelay(u, admins, allFollows)
	}
}

// fetchFollowListsFromRelay fetches follow lists for admins and metadata for all follows from a specific relay
func (f *Follows) fetchFollowListsFromRelay(relayURL string, admins [][]byte, allFollows [][]byte) {
	ctx, cancel := context.WithTimeout(f.Ctx, 60*time.Second)
	defer cancel()

	// Create proper headers for the WebSocket connection
	headers := http.Header{}
	headers.Set("User-Agent", "ORLY-Relay/0.9.2")
	headers.Set("Origin", "https://orly.dev")

	// Use proper WebSocket dial options
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	c, resp, err := dialer.DialContext(ctx, relayURL, headers)
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil {
		log.W.F("follows syncer: failed to connect to %s for follow list fetch: %v", relayURL, err)
		return
	}
	defer c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "follow list fetch complete"), time.Now().Add(time.Second))

	log.I.F("follows syncer: fetching follow lists and metadata from relay %s", relayURL)

	// Create filters:
	// - kind 3 (follow lists) for admins only
	// - kind 0 (metadata) + kind 10002 (relay lists) for all follows
	ff := &filter.S{}

	// Filter for admin follow lists (kind 3)
	f1 := &filter.F{
		Authors: tag.NewFromBytesSlice(admins...),
		Kinds:   kind.NewS(kind.New(kind.FollowList.K)),
		Limit:   values.ToUintPointer(uint(len(admins) * 2)),
	}

	// Filter for metadata (kind 0) for all follows
	f2 := &filter.F{
		Authors: tag.NewFromBytesSlice(allFollows...),
		Kinds:   kind.NewS(kind.New(kind.ProfileMetadata.K)),
		Limit:   values.ToUintPointer(uint(len(allFollows) * 2)),
	}

	// Filter for relay lists (kind 10002) for all follows
	f3 := &filter.F{
		Authors: tag.NewFromBytesSlice(allFollows...),
		Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
		Limit:   values.ToUintPointer(uint(len(allFollows) * 2)),
	}
	*ff = append(*ff, f1, f2, f3)

	// Use a specific subscription ID for follow list fetching
	subID := "follow-lists-fetch"
	req := reqenvelope.NewFrom([]byte(subID), ff)
	reqBytes := req.Marshal(nil)
	log.T.F("follows syncer: outbound REQ to %s: %s", relayURL, string(reqBytes))
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err = c.WriteMessage(websocket.TextMessage, reqBytes); chk.E(err) {
		log.W.F("follows syncer: failed to send follow list REQ to %s: %v", relayURL, err)
		return
	}

	log.T.F("follows syncer: sent follow list, metadata, and relay list REQ to %s", relayURL)

	// Collect all events before processing
	var followListEvents []*event.E
	var metadataEvents []*event.E
	var relayListEvents []*event.E

	// Read events with timeout (longer timeout for larger fetches)
	timeout := time.After(30 * time.Second)
	for {
		select {
		case <-ctx.Done():
			goto processEvents
		case <-timeout:
			log.T.F("follows syncer: timeout reading events from %s", relayURL)
			goto processEvents
		default:
		}

		c.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, data, err := c.ReadMessage()
		if err != nil {
			log.T.F("follows syncer: error reading events from %s: %v", relayURL, err)
			goto processEvents
		}

		label, rem, err := envelopes.Identify(data)
		if chk.E(err) {
			continue
		}

		switch label {
		case eventenvelope.L:
			res, _, err := eventenvelope.ParseResult(rem)
			if chk.E(err) || res == nil || res.Event == nil {
				continue
			}

			// Collect events by kind
			switch res.Event.Kind {
			case kind.FollowList.K:
				log.T.F("follows syncer: received follow list from %s on relay %s",
					hex.EncodeToString(res.Event.Pubkey), relayURL)
				followListEvents = append(followListEvents, res.Event)
			case kind.ProfileMetadata.K:
				log.T.F("follows syncer: received metadata from %s on relay %s",
					hex.EncodeToString(res.Event.Pubkey), relayURL)
				metadataEvents = append(metadataEvents, res.Event)
			case kind.RelayListMetadata.K:
				log.T.F("follows syncer: received relay list from %s on relay %s",
					hex.EncodeToString(res.Event.Pubkey), relayURL)
				relayListEvents = append(relayListEvents, res.Event)
			}
		case eoseenvelope.L:
			log.T.F("follows syncer: end of events from %s", relayURL)
			goto processEvents
		default:
			// ignore other labels
		}
	}

processEvents:
	// Process collected events - keep only the newest per pubkey and save to database
	f.processCollectedEvents(relayURL, followListEvents, metadataEvents, relayListEvents)
}

// processCollectedEvents processes the collected events, keeping only the newest per pubkey
func (f *Follows) processCollectedEvents(relayURL string, followListEvents, metadataEvents, relayListEvents []*event.E) {
	// Process follow list events (kind 3) - keep newest per pubkey
	latestFollowLists := make(map[string]*event.E)
	for _, ev := range followListEvents {
		pubkeyHex := hex.EncodeToString(ev.Pubkey)
		existing, exists := latestFollowLists[pubkeyHex]
		if !exists || ev.CreatedAt > existing.CreatedAt {
			latestFollowLists[pubkeyHex] = ev
		}
	}

	// Process metadata events (kind 0) - keep newest per pubkey
	latestMetadata := make(map[string]*event.E)
	for _, ev := range metadataEvents {
		pubkeyHex := hex.EncodeToString(ev.Pubkey)
		existing, exists := latestMetadata[pubkeyHex]
		if !exists || ev.CreatedAt > existing.CreatedAt {
			latestMetadata[pubkeyHex] = ev
		}
	}

	// Process relay list events (kind 10002) - keep newest per pubkey
	latestRelayLists := make(map[string]*event.E)
	for _, ev := range relayListEvents {
		pubkeyHex := hex.EncodeToString(ev.Pubkey)
		existing, exists := latestRelayLists[pubkeyHex]
		if !exists || ev.CreatedAt > existing.CreatedAt {
			latestRelayLists[pubkeyHex] = ev
		}
	}

	// Save and process the newest events
	savedFollowLists := 0
	savedMetadata := 0
	savedRelayLists := 0

	// Save follow list events to database and extract follows
	for pubkeyHex, ev := range latestFollowLists {
		if _, err := f.db.SaveEvent(f.Ctx, ev); err != nil {
			if !strings.HasPrefix(err.Error(), "blocked:") {
				log.W.F("follows syncer: failed to save follow list from %s: %v", pubkeyHex, err)
			}
		} else {
			savedFollowLists++
			log.T.F("follows syncer: saved follow list from %s (created_at: %d) from relay %s",
				pubkeyHex, ev.CreatedAt, relayURL)
		}

		// Extract followed pubkeys from admin follow lists
		if f.isAdminPubkey(ev.Pubkey) {
			log.I.F("follows syncer: processing admin follow list from %s", pubkeyHex)
			f.extractFollowedPubkeys(ev)
		}
	}

	// Save metadata events to database
	for pubkeyHex, ev := range latestMetadata {
		if _, err := f.db.SaveEvent(f.Ctx, ev); err != nil {
			if !strings.HasPrefix(err.Error(), "blocked:") {
				log.W.F("follows syncer: failed to save metadata from %s: %v", pubkeyHex, err)
			}
		} else {
			savedMetadata++
			log.T.F("follows syncer: saved metadata from %s (created_at: %d) from relay %s",
				pubkeyHex, ev.CreatedAt, relayURL)
		}
	}

	// Save relay list events to database
	for pubkeyHex, ev := range latestRelayLists {
		if _, err := f.db.SaveEvent(f.Ctx, ev); err != nil {
			if !strings.HasPrefix(err.Error(), "blocked:") {
				log.W.F("follows syncer: failed to save relay list from %s: %v", pubkeyHex, err)
			}
		} else {
			savedRelayLists++
			log.T.F("follows syncer: saved relay list from %s (created_at: %d) from relay %s",
				pubkeyHex, ev.CreatedAt, relayURL)
		}
	}

	log.I.F("follows syncer: from %s - received: %d follow lists, %d metadata, %d relay lists; saved: %d, %d, %d",
		relayURL, len(followListEvents), len(metadataEvents), len(relayListEvents),
		savedFollowLists, savedMetadata, savedRelayLists)
}

// GetFollowedPubkeys returns a copy of the followed pubkeys list
func (f *Follows) GetFollowedPubkeys() [][]byte {
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	followedPubkeys := make([][]byte, len(f.follows))
	copy(followedPubkeys, f.follows)
	return followedPubkeys
}

// isAdminPubkey checks if a pubkey belongs to an admin
func (f *Follows) isAdminPubkey(pubkey []byte) bool {
	pubkeyHex := hex.EncodeToString(pubkey)

	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	if f.adminsSet != nil {
		_, ok := f.adminsSet[pubkeyHex]
		return ok
	}
	return false
}

// extractFollowedPubkeys extracts followed pubkeys from 'p' tags in kind 3 events
func (f *Follows) extractFollowedPubkeys(event *event.E) {
	if event.Kind != kind.FollowList.K {
		return
	}

	// Extract all 'p' tags (followed pubkeys) from the kind 3 event
	for _, tag := range event.Tags.GetAll([]byte("p")) {
		// First try binary format (optimized storage: 33 bytes = 32 hash + null)
		if pubkey := tag.ValueBinary(); pubkey != nil {
			f.AddFollow(pubkey)
			continue
		}
		// Fall back to hex decoding for non-binary values
		// Use ValueHex() which handles both binary and hex storage formats
		if pubkey, err := hex.DecodeString(string(tag.ValueHex())); err == nil && len(pubkey) == 32 {
			f.AddFollow(pubkey)
		}
	}
}

// AdminRelays returns the admin relay URLs
func (f *Follows) AdminRelays() []string {
	return f.adminRelays()
}

// SetFollowListUpdateCallback sets a callback to be called when the follow list is updated
func (f *Follows) SetFollowListUpdateCallback(callback func()) {
	f.followsMx.Lock()
	defer f.followsMx.Unlock()
	f.onFollowListUpdate = callback
}

// AddFollow appends a pubkey to the in-memory follows list if not already present
// and signals the syncer to refresh subscriptions.
func (f *Follows) AddFollow(pub []byte) {
	if len(pub) == 0 {
		return
	}
	pubHex := hex.EncodeToString(pub)

	f.followsMx.Lock()
	defer f.followsMx.Unlock()

	// Use map for O(1) duplicate detection
	if f.followsSet == nil {
		f.followsSet = make(map[string]struct{})
	}
	if _, exists := f.followsSet[pubHex]; exists {
		return
	}

	b := make([]byte, len(pub))
	copy(b, pub)
	f.follows = append(f.follows, b)
	f.followsSet[pubHex] = struct{}{}

	log.I.F(
		"follows syncer: added new followed pubkey: %s",
		pubHex,
	)
	// notify external listeners (e.g., spider)
	if f.onFollowListUpdate != nil {
		go f.onFollowListUpdate()
	}
}

func init() {
	Registry.Register(new(Follows))
}
