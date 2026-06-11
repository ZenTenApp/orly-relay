package app

import (
	"context"
	"encoding/json"
	"time"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	hexenc "git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// channelAccessInfo holds parsed channel access control data from kind 41 metadata.
type channelAccessInfo struct {
	creator    string // hex pubkey of channel creator
	accessMode string // "open", "whitelist", "blacklist"
	mods       map[string]bool
	members    map[string]bool // whitelisted members
	blocked    map[string]bool // blacklisted users
	invited    map[string]bool // pending invites (have access)
	rejected   map[string]bool // rejected requests (no access)
	cachedAt   time.Time
}

const channelCacheTTL = 30 * time.Second

// channelRefCacheEntry caches whether an event ID references a channel event.
type channelRefCacheEntry struct {
	channelIDHex string
	isChannel    bool
	cachedAt     time.Time
}

// -- actor request types --

type cmCacheLoadReq struct {
	key  string
	resp chan cmCacheLoadResp
}
type cmCacheLoadResp struct {
	info *channelAccessInfo
	ok   bool
}
type cmCacheStoreReq struct {
	key  string
	info *channelAccessInfo
}
type cmCacheDeleteReq struct {
	key string
}
type cmRefLoadReq struct {
	key  string
	resp chan cmRefLoadResp
}
type cmRefLoadResp struct {
	entry *channelRefCacheEntry
	ok    bool
}
type cmRefStoreReq struct {
	key   string
	entry *channelRefCacheEntry
}

// ChannelMembership manages channel access control lookups with caching.
type ChannelMembership struct {
	db database.Database

	// Actor channels for cache and refCache
	cacheLoadCh   chan cmCacheLoadReq
	cacheStoreCh  chan cmCacheStoreReq
	cacheDeleteCh chan cmCacheDeleteReq
	refLoadCh     chan cmRefLoadReq
	refStoreCh    chan cmRefStoreReq
	done          chan struct{}
}

// NewChannelMembership creates a new channel membership checker.
func NewChannelMembership(db database.Database) *ChannelMembership {
	cm := &ChannelMembership{
		db:            db,
		cacheLoadCh:   make(chan cmCacheLoadReq),
		cacheStoreCh:  make(chan cmCacheStoreReq, 16),
		cacheDeleteCh: make(chan cmCacheDeleteReq, 16),
		refLoadCh:     make(chan cmRefLoadReq),
		refStoreCh:    make(chan cmRefStoreReq, 16),
		done:          make(chan struct{}),
	}
	go cm.actor()
	return cm
}

func (cm *ChannelMembership) actor() {
	defer close(cm.done)
	cache := make(map[string]*channelAccessInfo)
	refCache := make(map[string]*channelRefCacheEntry)

	for {
		select {
		case req := <-cm.cacheLoadCh:
			info, ok := cache[req.key]
			req.resp <- cmCacheLoadResp{info: info, ok: ok}
		case req := <-cm.cacheStoreCh:
			cache[req.key] = req.info
		case req := <-cm.cacheDeleteCh:
			delete(cache, req.key)
		case req := <-cm.refLoadCh:
			entry, ok := refCache[req.key]
			req.resp <- cmRefLoadResp{entry: entry, ok: ok}
		case req := <-cm.refStoreCh:
			refCache[req.key] = req.entry
		}
	}
}

// InvalidateChannel removes a channel's cached access info, forcing a re-fetch
// on the next check. Call this when a new kind 41 event is ingested.
func (cm *ChannelMembership) InvalidateChannel(channelIDHex string) {
	cm.cacheDeleteCh <- cmCacheDeleteReq{key: channelIDHex}
}

// IsChannelMember checks whether the given pubkey (binary) is allowed to access
// channel events of the given kind. Returns true if access is granted.
func (cm *ChannelMembership) IsChannelMember(
	ev *event.E,
	userPubkey []byte,
	ctx context.Context,
) bool {
	if len(userPubkey) == 0 {
		return false
	}

	// Kinds 40 and 41 are always readable for discovery
	if kind.IsDiscoverableChannelKind(ev.Kind) {
		return true
	}

	// For kinds 42-44, extract channel ID from #e tag
	channelIDHex := extractChannelID(ev)
	if channelIDHex == "" {
		return true
	}

	userHex := hexenc.Enc(userPubkey)

	info, err := cm.getChannelInfo(ctx, channelIDHex)
	if err != nil || info == nil {
		log.D.F("channel membership check: no info for channel %s, allowing", channelIDHex)
		return true
	}

	if info.creator == userHex {
		return true
	}
	if info.mods[userHex] {
		return true
	}

	switch info.accessMode {
	case "whitelist":
		return info.members[userHex] || info.invited[userHex]
	case "blacklist":
		return !info.blocked[userHex] && !info.rejected[userHex]
	default: // "open"
		return true
	}
}

// IsChannelMemberByID checks membership using a channel ID directly (not from an event).
func (cm *ChannelMembership) IsChannelMemberByID(
	channelIDHex string,
	eventKind uint16,
	userPubkey []byte,
	ctx context.Context,
) bool {
	if len(userPubkey) == 0 {
		return false
	}
	if kind.IsDiscoverableChannelKind(eventKind) {
		return true
	}
	if channelIDHex == "" {
		return true
	}

	userHex := hexenc.Enc(userPubkey)

	info, err := cm.getChannelInfo(ctx, channelIDHex)
	if err != nil || info == nil {
		return true
	}

	if info.creator == userHex {
		return true
	}
	if info.mods[userHex] {
		return true
	}

	switch info.accessMode {
	case "whitelist":
		return info.members[userHex] || info.invited[userHex]
	case "blacklist":
		return !info.blocked[userHex] && !info.rejected[userHex]
	default:
		return true
	}
}

// ReferencesChannelEvent checks whether any e-tag in the event references a
// restricted channel event (kind 42-44).
func (cm *ChannelMembership) ReferencesChannelEvent(
	ev *event.E,
	ctx context.Context,
) (channelIDHex string, isChannel bool) {
	if ev.Tags == nil {
		return "", false
	}
	eTags := ev.Tags.GetAll([]byte("e"))
	if len(eTags) == 0 {
		return "", false
	}

	for _, et := range eTags {
		if et.Len() < 2 {
			continue
		}
		refIDHex := string(et.ValueHex())
		if refIDHex == "" {
			continue
		}

		// Check reference cache first
		resp := make(chan cmRefLoadResp, 1)
		cm.refLoadCh <- cmRefLoadReq{key: refIDHex, resp: resp}
		r := <-resp
		if r.ok {
			entry := r.entry
			if time.Since(entry.cachedAt) < channelCacheTTL {
				if entry.isChannel {
					return entry.channelIDHex, true
				}
				continue
			}
		}

		// Look up the referenced event in the database
		refIDBytes, err := hexenc.Dec(refIDHex)
		if err != nil {
			continue
		}
		ser, err := cm.db.GetSerialById(refIDBytes)
		if err != nil || ser == nil {
			cm.refStoreCh <- cmRefStoreReq{key: refIDHex, entry: &channelRefCacheEntry{
				cachedAt: time.Now(),
			}}
			continue
		}
		refEv, err := cm.db.FetchEventBySerial(ser)
		if err != nil || refEv == nil {
			cm.refStoreCh <- cmRefStoreReq{key: refIDHex, entry: &channelRefCacheEntry{
				cachedAt: time.Now(),
			}}
			continue
		}

		if kind.IsChannelKind(refEv.Kind) && !kind.IsDiscoverableChannelKind(refEv.Kind) {
			chID := extractChannelID(refEv)
			cm.refStoreCh <- cmRefStoreReq{key: refIDHex, entry: &channelRefCacheEntry{
				channelIDHex: chID,
				isChannel:    true,
				cachedAt:     time.Now(),
			}}
			return chID, true
		}

		cm.refStoreCh <- cmRefStoreReq{key: refIDHex, entry: &channelRefCacheEntry{
			cachedAt: time.Now(),
		}}
	}
	return "", false
}

// getChannelInfo fetches (from cache or DB) the access control info for a channel.
func (cm *ChannelMembership) getChannelInfo(
	ctx context.Context,
	channelIDHex string,
) (*channelAccessInfo, error) {
	// Check cache
	resp := make(chan cmCacheLoadResp, 1)
	cm.cacheLoadCh <- cmCacheLoadReq{key: channelIDHex, resp: resp}
	r := <-resp
	if r.ok {
		info := r.info
		if time.Since(info.cachedAt) < channelCacheTTL {
			return info, nil
		}
	}

	// Query for latest kind 41 metadata event for this channel
	f := filter.New()
	f.Kinds = kind.NewS(kind.ChannelMetadata)

	channelIDBytes, err := hexenc.Dec(channelIDHex)
	if err != nil {
		return nil, err
	}
	eTag := tag.NewFromBytesSlice([]byte("e"), channelIDBytes)
	f.Tags = tag.NewSWithCap(1)
	*f.Tags = append(*f.Tags, eTag)

	limit := uint(1)
	f.Limit = &limit

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	events, err := cm.db.QueryEvents(queryCtx, f)
	if chk.E(err) {
		return nil, err
	}

	var info *channelAccessInfo

	if len(events) > 0 {
		info = parseChannelMetadata(events[0])
	} else {
		f2 := filter.New()
		f2.Ids = tag.NewFromBytesSlice(channelIDBytes)
		f2.Kinds = kind.NewS(kind.ChannelCreation)
		limit2 := uint(1)
		f2.Limit = &limit2

		events2, err2 := cm.db.QueryEvents(queryCtx, f2)
		if chk.E(err2) || len(events2) == 0 {
			return nil, err2
		}

		info = &channelAccessInfo{
			creator:    hexenc.Enc(events2[0].Pubkey),
			accessMode: "open",
			mods:       make(map[string]bool),
			members:    make(map[string]bool),
			blocked:    make(map[string]bool),
			invited:    make(map[string]bool),
			rejected:   make(map[string]bool),
		}
	}

	info.cachedAt = time.Now()
	cm.cacheStoreCh <- cmCacheStoreReq{key: channelIDHex, info: info}
	return info, nil
}

// parseChannelMetadata extracts access control info from a kind 41 event.
func parseChannelMetadata(ev *event.E) *channelAccessInfo {
	info := &channelAccessInfo{
		creator:    hexenc.Enc(ev.Pubkey),
		accessMode: "open",
		mods:       make(map[string]bool),
		members:    make(map[string]bool),
		blocked:    make(map[string]bool),
		invited:    make(map[string]bool),
		rejected:   make(map[string]bool),
	}

	if len(ev.Content) > 0 {
		var content struct {
			AccessMode string `json:"access_mode"`
			InviteOnly bool   `json:"invite_only"`
		}
		if err := json.Unmarshal(ev.Content, &content); err == nil {
			if content.AccessMode != "" {
				info.accessMode = content.AccessMode
			} else if content.InviteOnly {
				info.accessMode = "whitelist"
			}
		}
	}

	pTags := ev.Tags.GetAll([]byte("p"))
	for _, pt := range pTags {
		if pt.Len() < 3 {
			continue
		}
		pkHex := string(pt.ValueHex())
		role := string(pt.T[2])

		switch role {
		case "mod":
			info.mods[pkHex] = true
		case "member":
			info.members[pkHex] = true
		case "blocked":
			info.blocked[pkHex] = true
		case "invited":
			info.invited[pkHex] = true
		case "requested":
			// Requested users don't have access
		case "rejected":
			info.rejected[pkHex] = true
		}
	}

	return info
}

// extractChannelID gets the channel ID (hex) from an event's #e tag.
func extractChannelID(ev *event.E) string {
	if ev.Tags == nil {
		return ""
	}
	eTags := ev.Tags.GetAll([]byte("e"))
	for _, et := range eTags {
		if et.Len() >= 2 {
			val := et.ValueHex()
			if len(val) > 0 {
				return string(val)
			}
		}
	}
	return ""
}

// ExtractChannelIDFromEvent is the exported version of extractChannelID
// for use by the publisher.
func ExtractChannelIDFromEvent(ev *event.E) string {
	return extractChannelID(ev)
}

// IsChannelEvent returns true if the event is a channel kind (40-44).
func IsChannelEvent(ev *event.E) bool {
	return kind.IsChannelKind(ev.Kind)
}
