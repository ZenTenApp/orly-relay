package acl

import (
	"bytes"
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
	"next.orly.dev/pkg/database/indexes/types"
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
	"next.orly.dev/pkg/utils"
	"git.mleku.dev/mleku/nostr/utils/normalize"
	"git.mleku.dev/mleku/nostr/utils/values"
)

type Follows struct {
	Ctx context.Context
	cfg *config.C
	*database.D
	pubs       *publish.S
	followsMx  sync.RWMutex
	admins     [][]byte
	owners     [][]byte
	follows    [][]byte
	// Track last follow list fetch time
	lastFollowListFetch time.Time
	// Callback for external notification of follow list changes
	onFollowListUpdate func()
}

func (f *Follows) Configure(cfg ...any) (err error) {
	log.I.F("configuring follows ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			// log.D.F("setting ACL config: %v", c)
			f.cfg = c
		case *database.D:
			// log.D.F("setting ACL database: %s", c.Path())
			f.D = c
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
	if f.cfg == nil || f.D == nil {
		err = errorf.E("both config and database must be set")
		return
	}
	// add owners list
	for _, owner := range f.cfg.Owners {
		var own []byte
		if o, e := bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(e) {
			continue
		} else {
			own = o
		}
		f.owners = append(f.owners, own)
	}
	// find admin follow lists
	f.followsMx.Lock()
	defer f.followsMx.Unlock()
	// log.I.F("finding admins")
	f.follows, f.admins = nil, nil
	for _, admin := range f.cfg.Admins {
		// log.I.F("%s", admin)
		var adm []byte
		if a, e := bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(e) {
			continue
		} else {
			adm = a
		}
		// log.I.F("admin: %0x", adm)
		f.admins = append(f.admins, adm)
		fl := &filter.F{
			Authors: tag.NewFromAny(adm),
			Kinds:   kind.NewS(kind.New(kind.FollowList.K)),
		}
		var idxs []database.Range
		if idxs, err = database.GetIndexesFromFilter(fl); chk.E(err) {
			return
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = f.D.GetSerialsByRange(idx); chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}
		if len(sers) > 0 {
			for _, s := range sers {
				var ev *event.E
				if ev, err = f.D.FetchEventBySerial(s); chk.E(err) {
					continue
				}
				// log.I.F("admin follow list:\n%s", ev.Serialize())
				for _, v := range ev.Tags.GetAll([]byte("p")) {
					// log.I.F("adding follow: %s", v.ValueHex())
					// ValueHex() automatically handles both binary and hex storage formats
					if b, e := hex.DecodeString(string(v.ValueHex())); chk.E(e) {
						continue
					} else {
						f.follows = append(f.follows, b)
					}
				}
			}
		}
	}
	return
}

func (f *Follows) GetAccessLevel(pub []byte, address string) (level string) {
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()
	for _, v := range f.owners {
		if utils.FastEqual(v, pub) {
			return "owner"
		}
	}
	for _, v := range f.admins {
		if utils.FastEqual(v, pub) {
			return "admin"
		}
	}
	for _, v := range f.follows {
		if utils.FastEqual(v, pub) {
			return "write"
		}
	}
	if f.cfg == nil {
		return "write"
	}
	return "read"
}

func (f *Follows) GetACLInfo() (name, description, documentation string) {
	return "follows", "whitelist follows of admins",
		`This ACL mode searches for follow lists of admins and grants all followers write access`
}

func (f *Follows) Type() string { return "follows" }

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

	// First, try to get relay URLs from admin kind 10002 events
	for _, adm := range admins {
		fl := &filter.F{
			Authors: tag.NewFromAny(adm),
			Kinds:   kind.NewS(kind.New(kind.RelayListMetadata.K)),
		}
		idxs, err := database.GetIndexesFromFilter(fl)
		if chk.E(err) {
			continue
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			s, err := f.D.GetSerialsByRange(idx)
			if chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}
		for _, s := range sers {
			ev, err := f.D.FetchEventBySerial(s)
			if chk.E(err) || ev == nil {
				continue
			}
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
		if _, err := f.D.SaveEvent(f.Ctx, ev); err != nil {
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
		if _, err := f.D.SaveEvent(f.Ctx, ev); err != nil {
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
		if _, err := f.D.SaveEvent(f.Ctx, ev); err != nil {
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
	f.followsMx.RLock()
	defer f.followsMx.RUnlock()

	for _, admin := range f.admins {
		if utils.FastEqual(admin, pubkey) {
			return true
		}
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
	f.followsMx.Lock()
	defer f.followsMx.Unlock()
	for _, p := range f.follows {
		if bytes.Equal(p, pub) {
			return
		}
	}
	b := make([]byte, len(pub))
	copy(b, pub)
	f.follows = append(f.follows, b)
	log.I.F(
		"follows syncer: added new followed pubkey: %s",
		hex.EncodeToString(pub),
	)
	// notify external listeners (e.g., spider)
	if f.onFollowListUpdate != nil {
		go f.onFollowListUpdate()
	}
}

func init() {
	Registry.Register(new(Follows))
}
