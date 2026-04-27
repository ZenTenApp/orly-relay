package app

import (
	"context"
	"encoding/json"
	"sync"
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

// ChannelMembership manages channel access control lookups with caching.
type ChannelMembership struct {
	db       database.Database
	cache    sync.Map // map[string]*channelAccessInfo (channel ID hex → info)
	refCache sync.Map // map[string]*channelRefCacheEntry (event ID hex → channel ref info)
}

// NewChannelMembership creates a new channel membership checker.
func NewChannelMembership(db database.Database) *ChannelMembership {
	return &ChannelMembership{db: db}
}

// InvalidateChannel removes a channel's cached access info, forcing a re-fetch
// on the next check. Call this when a new kind 41 event is ingested.
func (cm *ChannelMembership) InvalidateChannel(channelIDHex string) {
	cm.cache.Delete(channelIDHex)
}

// IsChannelMember checks whether the given pubkey (binary) is allowed to access
// channel events of the given kind. Returns true if access is granted.
//
// Access rules:
//   - Kinds 40, 41 (create, metadata): always allowed for any authenticated user (discovery)
//   - Kinds 42-44 (message, hide, mute): depends on channel access mode
//   - "open": all authenticated users allowed
//   - "whitelist": only creator, mods, members, and invited users
//   - "blacklist": everyone except blocked and rejected users
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
		// No channel reference — allow (might be malformed, let other checks handle)
		return true
	}

	userHex := hexenc.Enc(userPubkey)

	info, err := cm.getChannelInfo(ctx, channelIDHex)
	if err != nil || info == nil {
		// If we can't determine channel info, allow access (fail open for now)
		log.D.F("channel membership check: no info for channel %s, allowing", channelIDHex)
		return true
	}

	// Creator always has access
	if info.creator == userHex {
		return true
	}

	// Mods always have access
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
// Used by the publisher when delivering events.
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
// restricted channel event (kind 42-44). If so, returns the channel ID and true.
// Used to enforce channel membership for non-channel kinds (reactions, reposts,
// reports, zaps, deletions) that reference channel events.
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
		if cached, ok := cm.refCache.Load(refIDHex); ok {
			entry := cached.(*channelRefCacheEntry)
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
			// Cache negative result
			cm.refCache.Store(refIDHex, &channelRefCacheEntry{
				cachedAt: time.Now(),
			})
			continue
		}
		refEv, err := cm.db.FetchEventBySerial(ser)
		if err != nil || refEv == nil {
			cm.refCache.Store(refIDHex, &channelRefCacheEntry{
				cachedAt: time.Now(),
			})
			continue
		}

		if kind.IsChannelKind(refEv.Kind) && !kind.IsDiscoverableChannelKind(refEv.Kind) {
			// It's a restricted channel event (42-44). Extract the channel ID.
			chID := extractChannelID(refEv)
			cm.refCache.Store(refIDHex, &channelRefCacheEntry{
				channelIDHex: chID,
				isChannel:    true,
				cachedAt:     time.Now(),
			})
			return chID, true
		}

		// Not a channel event — cache that too
		cm.refCache.Store(refIDHex, &channelRefCacheEntry{
			cachedAt: time.Now(),
		})
	}
	return "", false
}

// getChannelInfo fetches (from cache or DB) the access control info for a channel.
func (cm *ChannelMembership) getChannelInfo(
	ctx context.Context,
	channelIDHex string,
) (*channelAccessInfo, error) {
	// Check cache
	if cached, ok := cm.cache.Load(channelIDHex); ok {
		info := cached.(*channelAccessInfo)
		if time.Since(info.cachedAt) < channelCacheTTL {
			return info, nil
		}
		// Expired, fall through to re-fetch
	}

	// Query for latest kind 41 metadata event for this channel
	f := filter.New()
	f.Kinds = kind.NewS(kind.ChannelMetadata)

	// Build #e tag filter for the channel ID
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
		// No kind 41 found. Try kind 40 (channel creation) to get creator.
		f2 := filter.New()
		f2.Ids = tag.NewFromBytesSlice(channelIDBytes)
		f2.Kinds = kind.NewS(kind.ChannelCreation)
		limit2 := uint(1)
		f2.Limit = &limit2

		events2, err2 := cm.db.QueryEvents(queryCtx, f2)
		if chk.E(err2) || len(events2) == 0 {
			return nil, err2
		}

		// Default to open if no metadata exists
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
	cm.cache.Store(channelIDHex, info)
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

	// Parse content JSON for access_mode
	if len(ev.Content) > 0 {
		var content struct {
			AccessMode string `json:"access_mode"`
			InviteOnly bool   `json:"invite_only"` // backward compat
		}
		if err := json.Unmarshal(ev.Content, &content); err == nil {
			if content.AccessMode != "" {
				info.accessMode = content.AccessMode
			} else if content.InviteOnly {
				info.accessMode = "whitelist"
			}
		}
	}

	// Parse p-tags for roles
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
// For kinds 42-44, the channel reference is in the first #e tag.
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
// Convenience wrapper around kind.IsChannelKind.
func IsChannelEvent(ev *event.E) bool {
	return kind.IsChannelKind(ev.Kind)
}

