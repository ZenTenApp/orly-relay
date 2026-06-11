package acl

import (
	"context"
	"encoding/hex"
	"reflect"
	"strconv"
	"strings"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/utils"
)

// Default values for curating mode
const (
	DefaultDailyLimit      = 50
	DefaultIPDailyLimit    = 500 // Max events per IP per day (flood protection)
	DefaultFirstBanHours   = 1
	DefaultSecondBanHours  = 168 // 1 week
	CuratingConfigKind     = 30078
	CuratingConfigDTag     = "curating-config"
)

type curatingAccessArgs struct {
	Pub     []byte
	Address string
}

type curatingSetCacheArgs struct {
	PubkeyHex string
	Val       bool
}

type curatingRefreshArgs struct {
	Trusted     []database.TrustedPubkey
	Blacklisted []database.BlacklistedPubkey
	Config      database.CuratingConfig
}

// Curating implements the curating ACL mode with three-tier publisher classification:
// - Trusted: Unlimited publishing
// - Blacklisted: Cannot publish
// - Unclassified: Rate-limited publishing (default 50/day)
type Curating struct {
	// Ctx holds the context for the ACL.
	// Deprecated: Use Context() method instead of accessing directly.
	Ctx         context.Context
	cfg         *config.C
	db          database.Database
	curatingACL *database.CuratingACL
	owners      [][]byte
	admins      [][]byte

	getAccessLevel  actor.Func[curatingAccessArgs, string]
	checkBlacklisted actor.Func[string, bool]
	setBlacklisted  actor.Inbox[curatingSetCacheArgs]
	checkTrusted    actor.Func[string, bool]
	setTrusted      actor.Inbox[curatingSetCacheArgs]
	refreshCaches   actor.Proc[curatingRefreshArgs]
	getConfig       actor.Query[*database.CuratingConfig]
	setConfigCache  actor.Inbox[*database.CuratingConfig]
	trustPubkey     actor.Inbox[string]
	blacklistPubkey actor.Inbox[string]
	untrust         actor.Inbox[string]
	unblacklist     actor.Inbox[string]
	actor.Lifecycle
}

// Context returns the ACL context.
func (c *Curating) Context() context.Context {
	return c.Ctx
}

func (c *Curating) Configure(cfg ...any) (err error) {
	log.I.F("configuring curating ACL")
	for _, ca := range cfg {
		switch cv := ca.(type) {
		case *config.C:
			c.cfg = cv
		case database.Database:
			c.db = cv
			// CuratingACL requires the concrete Badger database type
			if d, ok := cv.(*database.D); ok {
				c.curatingACL = database.NewCuratingACL(d)
			} else {
				log.W.F("curating ACL: database is not Badger, curating ACL features will be limited")
			}
		case context.Context:
			c.Ctx = cv
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if c.cfg == nil || c.db == nil {
		err = errorf.E("both config and database must be set")
		return
	}

	// Load owners from config
	for _, owner := range c.cfg.Owners {
		var own []byte
		if o, e := bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(e) {
			continue
		} else {
			own = o
		}
		c.owners = append(c.owners, own)
	}

	// Load admins from config
	for _, admin := range c.cfg.Admins {
		var adm []byte
		if a, e := bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(e) {
			continue
		} else {
			adm = a
		}
		c.admins = append(c.admins, adm)
	}

	// Initialize actor channels
	c.getAccessLevel = actor.NewFunc[curatingAccessArgs, string]()
	c.checkBlacklisted = actor.NewFunc[string, bool]()
	c.setBlacklisted = actor.NewInbox[curatingSetCacheArgs](16)
	c.checkTrusted = actor.NewFunc[string, bool]()
	c.setTrusted = actor.NewInbox[curatingSetCacheArgs](16)
	c.refreshCaches = actor.NewProc[curatingRefreshArgs]()
	c.getConfig = actor.NewQuery[*database.CuratingConfig]()
	c.setConfigCache = actor.NewInbox[*database.CuratingConfig](16)
	c.trustPubkey = actor.NewInbox[string](16)
	c.blacklistPubkey = actor.NewInbox[string](16)
	c.untrust = actor.NewInbox[string](16)
	c.unblacklist = actor.NewInbox[string](16)
	c.Lifecycle = actor.NewLifecycle()
	actor.Go(c.Lifecycle, c.actorLoop)

	// Refresh caches from database
	if err = c.RefreshCaches(); err != nil {
		log.W.F("curating ACL: failed to refresh caches: %v", err)
	}

	return nil
}

func (c *Curating) actorLoop() {
	trustedCache := make(map[string]bool)
	blacklistedCache := make(map[string]bool)
	kindCache := make(map[int]bool)
	var configCache *database.CuratingConfig

	for {
		select {
		case <-c.Stopping():
			return

		case msg := <-c.getAccessLevel.Recv():
			pubkeyHex := hex.EncodeToString(msg.Req.Pub)
			level := "write"

			// Check owners first
			isOwnerOrAdmin := false
			for _, v := range c.owners {
				if utils.FastEqual(v, msg.Req.Pub) {
					level = "owner"
					isOwnerOrAdmin = true
					break
				}
			}
			if !isOwnerOrAdmin {
				for _, v := range c.admins {
					if utils.FastEqual(v, msg.Req.Pub) {
						level = "admin"
						isOwnerOrAdmin = true
						break
					}
				}
			}

			if !isOwnerOrAdmin && c.curatingACL != nil {
				if msg.Req.Address != "" {
					blocked, _, err := c.curatingACL.IsIPBlocked(msg.Req.Address)
					if err == nil && blocked {
						level = "blocked"
					}
				}
				if level == "write" && blacklistedCache[pubkeyHex] {
					level = "banned"
				}
				if level == "write" {
					if bl, _ := c.curatingACL.IsPubkeyBlacklisted(pubkeyHex); bl {
						blacklistedCache[pubkeyHex] = true
						level = "banned"
					}
				}
			}
			msg.Reply(level)

		case msg := <-c.checkBlacklisted.Recv():
			msg.Reply(blacklistedCache[msg.Req])

		case args := <-c.setBlacklisted.Recv():
			if args.Val {
				blacklistedCache[args.PubkeyHex] = true
			} else {
				delete(blacklistedCache, args.PubkeyHex)
			}

		case msg := <-c.checkTrusted.Recv():
			msg.Reply(trustedCache[msg.Req])

		case args := <-c.setTrusted.Recv():
			if args.Val {
				trustedCache[args.PubkeyHex] = true
			} else {
				delete(trustedCache, args.PubkeyHex)
			}

		case msg := <-c.refreshCaches.Recv():
			trustedCache = make(map[string]bool)
			for _, t := range msg.Req.Trusted {
				trustedCache[t.Pubkey] = true
			}
			blacklistedCache = make(map[string]bool)
			for _, b := range msg.Req.Blacklisted {
				blacklistedCache[b.Pubkey] = true
			}
			configCache = &msg.Req.Config
			kindCache = make(map[int]bool)
			for _, k := range msg.Req.Config.AllowedKinds {
				kindCache[k] = true
			}
			log.D.F("curating ACL: caches refreshed - %d trusted, %d blacklisted, %d allowed kinds",
				len(trustedCache), len(blacklistedCache), len(kindCache))
			msg.Done()

		case msg := <-c.getConfig.Recv():
			if configCache != nil {
				cp := *configCache
				msg.Reply(&cp)
			} else {
				msg.Reply(nil)
			}

		case cfg := <-c.setConfigCache.Recv():
			configCache = cfg

		case pubkeyHex := <-c.trustPubkey.Recv():
			trustedCache[pubkeyHex] = true
			delete(blacklistedCache, pubkeyHex)

		case pubkeyHex := <-c.blacklistPubkey.Recv():
			blacklistedCache[pubkeyHex] = true
			delete(trustedCache, pubkeyHex)

		case pubkeyHex := <-c.untrust.Recv():
			delete(trustedCache, pubkeyHex)

		case pubkeyHex := <-c.unblacklist.Recv():
			delete(blacklistedCache, pubkeyHex)
		}
	}
}

func (c *Curating) GetAccessLevel(pub []byte, address string) (level string) {
	return c.getAccessLevel.Call(curatingAccessArgs{Pub: pub, Address: address})
}

// CheckPolicy implements the PolicyChecker interface for event-level filtering
func (c *Curating) CheckPolicy(ev *event.E) (allowed bool, err error) {
	// If curatingACL is nil (non-Badger DB), allow everything
	if c.curatingACL == nil {
		return true, nil
	}

	pubkeyHex := hex.EncodeToString(ev.Pubkey)

	// Check if configured
	config, err := c.GetConfig()
	if err != nil {
		return false, errorf.E("failed to get config: %v", err)
	}
	if config.ConfigEventID == "" {
		return false, errorf.E("curating mode not configured: please publish a configuration event")
	}

	// Check if event is spam-flagged
	isSpam, _ := c.curatingACL.IsEventSpam(hex.EncodeToString(ev.ID[:]))
	if isSpam {
		return false, errorf.E("blocked: event is flagged as spam")
	}

	// Check if event kind is allowed
	if !c.curatingACL.IsKindAllowed(int(ev.Kind), &config) {
		return false, errorf.E("blocked: event kind %d is not in the allow list", ev.Kind)
	}

	// Check if pubkey is blacklisted
	isBlacklisted := c.checkBlacklisted.Call(pubkeyHex)
	if !isBlacklisted {
		isBlacklisted, _ = c.curatingACL.IsPubkeyBlacklisted(pubkeyHex)
	}
	if isBlacklisted {
		return false, errorf.E("blocked: pubkey is blacklisted")
	}

	// Check if pubkey is trusted (bypass rate limiting)
	isTrusted := c.checkTrusted.Call(pubkeyHex)
	if !isTrusted {
		isTrusted, _ = c.curatingACL.IsPubkeyTrusted(pubkeyHex)
		if isTrusted {
			// Update cache
			c.setTrusted.TrySend(curatingSetCacheArgs{PubkeyHex: pubkeyHex, Val: true})
		}
	}
	if isTrusted {
		return true, nil
	}

	// Check if owner or admin (bypass rate limiting)
	for _, v := range c.owners {
		if utils.FastEqual(v, ev.Pubkey) {
			return true, nil
		}
	}
	for _, v := range c.admins {
		if utils.FastEqual(v, ev.Pubkey) {
			return true, nil
		}
	}

	// For unclassified users, check rate limit
	today := time.Now().Format("2006-01-02")
	dailyLimit := config.DailyLimit
	if dailyLimit == 0 {
		dailyLimit = DefaultDailyLimit
	}

	count, err := c.curatingACL.GetEventCount(pubkeyHex, today)
	if err != nil {
		log.W.F("curating ACL: failed to get event count: %v", err)
		count = 0
	}

	if count >= dailyLimit {
		return false, errorf.E("rate limit exceeded: maximum %d events per day for unclassified users", dailyLimit)
	}

	// Increment the counter
	_, err = c.curatingACL.IncrementEventCount(pubkeyHex, today)
	if err != nil {
		log.W.F("curating ACL: failed to increment event count: %v", err)
	}

	return true, nil
}

// RateLimitCheck checks if an unclassified user can publish and handles IP tracking
// This is called separately when we have access to the IP address
func (c *Curating) RateLimitCheck(pubkeyHex, ip string) (allowed bool, message string, err error) {
	config, err := c.GetConfig()
	if err != nil {
		return false, "", errorf.E("failed to get config: %v", err)
	}

	today := time.Now().Format("2006-01-02")

	// Check IP flood limit first (applies to all non-trusted users from this IP)
	if ip != "" {
		ipDailyLimit := config.IPDailyLimit
		if ipDailyLimit == 0 {
			ipDailyLimit = DefaultIPDailyLimit
		}

		ipCount, err := c.curatingACL.GetIPEventCount(ip, today)
		if err != nil {
			ipCount = 0
		}

		if ipCount >= ipDailyLimit {
			// IP has exceeded flood limit - record offense and ban
			c.recordIPOffenseAndBan(ip, pubkeyHex, config, "IP flood limit exceeded")
			return false, "rate limit exceeded: too many events from this IP address", nil
		}
	}

	// Check per-pubkey daily limit
	dailyLimit := config.DailyLimit
	if dailyLimit == 0 {
		dailyLimit = DefaultDailyLimit
	}

	count, err := c.curatingACL.GetEventCount(pubkeyHex, today)
	if err != nil {
		count = 0
	}

	if count >= dailyLimit {
		// Record IP offense and potentially ban
		if ip != "" {
			c.recordIPOffenseAndBan(ip, pubkeyHex, config, "pubkey rate limit exceeded")
		}
		return false, "rate limit exceeded: maximum events per day for unclassified users", nil
	}

	// Increment IP event count for flood tracking (only for non-trusted users)
	if ip != "" {
		_, _ = c.curatingACL.IncrementIPEventCount(ip, today)
	}

	return true, "", nil
}

// recordIPOffenseAndBan records an offense for an IP and applies a ban if warranted
func (c *Curating) recordIPOffenseAndBan(ip, pubkeyHex string, config database.CuratingConfig, reason string) {
	offenseCount, _ := c.curatingACL.RecordIPOffense(ip, pubkeyHex)
	if offenseCount > 0 {
		firstBanHours := config.FirstBanHours
		if firstBanHours == 0 {
			firstBanHours = DefaultFirstBanHours
		}
		secondBanHours := config.SecondBanHours
		if secondBanHours == 0 {
			secondBanHours = DefaultSecondBanHours
		}

		var banDuration time.Duration
		if offenseCount >= 2 {
			banDuration = time.Duration(secondBanHours) * time.Hour
			log.W.F("curating ACL: IP %s banned for %d hours (offense #%d, reason: %s)", ip, secondBanHours, offenseCount, reason)
		} else {
			banDuration = time.Duration(firstBanHours) * time.Hour
			log.W.F("curating ACL: IP %s banned for %d hours (offense #%d, reason: %s)", ip, firstBanHours, offenseCount, reason)
		}
		c.curatingACL.BlockIP(ip, banDuration, reason)
	}
}

func (c *Curating) GetACLInfo() (name, description, documentation string) {
	return "curating", "curated relay with rate-limited unclassified publishers",
		`Curating ACL mode provides three-tier publisher classification:

- Trusted: Unlimited publishing, explicitly marked by admin
- Blacklisted: Cannot publish, events rejected
- Unclassified: Default state, rate-limited (default 50 events/day)

Features:
- Per-pubkey daily rate limiting for unclassified users (default 50/day)
- Per-IP daily rate limiting for flood protection (default 500/day)
- IP-based spam detection (tracks multiple rate-limited pubkeys)
- Automatic IP bans (1-hour first offense, 1-week second offense)
- Event kind allow-listing for content control
- Spam flagging (events hidden from queries without deletion)

Configuration via kind 30078 event with d-tag "curating-config".
The relay will not accept events until configured.

Management through NIP-86 API endpoints:
- trustpubkey, untrustpubkey, listtrustedpubkeys
- blacklistpubkey, unblacklistpubkey, listblacklistedpubkeys
- listunclassifiedusers
- markspam, unmarkspam, listspamevents
- setallowedkindcategories, getallowedkindcategories`
}

func (c *Curating) Type() string { return "curating" }

// IsEventVisible checks if an event should be visible to the given access level.
// Events from blacklisted pubkeys are only visible to admin/owner.
func (c *Curating) IsEventVisible(ev *event.E, accessLevel string) bool {
	// Admin and owner can see all events
	if accessLevel == "admin" || accessLevel == "owner" {
		return true
	}

	// Check if the event author is blacklisted
	pubkeyHex := hex.EncodeToString(ev.Pubkey)

	// Check cache first
	if c.checkBlacklisted.Call(pubkeyHex) {
		return false
	}

	// Check database if not in cache
	if blacklisted, _ := c.curatingACL.IsPubkeyBlacklisted(pubkeyHex); blacklisted {
		c.setBlacklisted.TrySend(curatingSetCacheArgs{PubkeyHex: pubkeyHex, Val: true})
		return false
	}

	return true
}

// FilterVisibleEvents filters a list of events, removing those from blacklisted pubkeys.
// Returns only events visible to the given access level.
func (c *Curating) FilterVisibleEvents(events []*event.E, accessLevel string) []*event.E {
	// Admin and owner can see all events
	if accessLevel == "admin" || accessLevel == "owner" {
		return events
	}

	// Filter out events from blacklisted pubkeys
	visible := make([]*event.E, 0, len(events))
	for _, ev := range events {
		if c.IsEventVisible(ev, accessLevel) {
			visible = append(visible, ev)
		}
	}
	return visible
}

// GetCuratingACL returns the database ACL instance for direct access
func (c *Curating) GetCuratingACL() *database.CuratingACL {
	return c.curatingACL
}

func (c *Curating) Syncer() {
	log.I.F("starting curating ACL syncer")

	// Start background cleanup goroutine
	go c.backgroundCleanup()
}

// backgroundCleanup periodically cleans up expired data
func (c *Curating) backgroundCleanup() {
	// Run cleanup every hour
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-c.Ctx.Done():
			log.D.F("curating ACL background cleanup stopped")
			return
		case <-ticker.C:
			c.runCleanup()
		}
	}
}

func (c *Curating) runCleanup() {
	log.D.F("curating ACL: running background cleanup")

	// Clean up expired IP blocks
	if err := c.curatingACL.CleanupExpiredIPBlocks(); err != nil {
		log.W.F("curating ACL: failed to cleanup expired IP blocks: %v", err)
	}

	// Clean up old event counts (older than 7 days)
	cutoffDate := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	if err := c.curatingACL.CleanupOldEventCounts(cutoffDate); err != nil {
		log.W.F("curating ACL: failed to cleanup old event counts: %v", err)
	}

	// Refresh caches
	if err := c.RefreshCaches(); err != nil {
		log.W.F("curating ACL: failed to refresh caches: %v", err)
	}
}

// RefreshCaches refreshes all in-memory caches from the database
func (c *Curating) RefreshCaches() error {
	// Fetch data outside the actor
	trusted, err := c.curatingACL.ListTrustedPubkeys()
	if err != nil {
		return errorf.E("failed to list trusted pubkeys: %v", err)
	}

	blacklisted, err := c.curatingACL.ListBlacklistedPubkeys()
	if err != nil {
		return errorf.E("failed to list blacklisted pubkeys: %v", err)
	}

	config, err := c.curatingACL.GetConfig()
	if err != nil {
		return errorf.E("failed to get config: %v", err)
	}

	// Send to actor for atomic swap
	c.refreshCaches.Call(curatingRefreshArgs{
		Trusted:     trusted,
		Blacklisted: blacklisted,
		Config:      config,
	})

	return nil
}

// GetConfig returns the current configuration
func (c *Curating) GetConfig() (database.CuratingConfig, error) {
	cached := c.getConfig.Call()
	if cached != nil {
		return *cached, nil
	}
	return c.curatingACL.GetConfig()
}

// IsConfigured returns true if the relay has been configured
func (c *Curating) IsConfigured() (bool, error) {
	return c.curatingACL.IsConfigured()
}

// ProcessConfigEvent processes a kind 30078 event to extract curating configuration
func (c *Curating) ProcessConfigEvent(ev *event.E) error {
	if ev.Kind != CuratingConfigKind {
		return errorf.E("invalid event kind: expected %d, got %d", CuratingConfigKind, ev.Kind)
	}

	// Check d-tag
	dTag := ev.Tags.GetFirst([]byte("d"))
	if dTag == nil || string(dTag.Value()) != CuratingConfigDTag {
		return errorf.E("invalid d-tag: expected %s", CuratingConfigDTag)
	}

	// Check if pubkey is owner or admin
	pubkeyHex := hex.EncodeToString(ev.Pubkey)
	isOwner := false
	isAdmin := false
	for _, v := range c.owners {
		if utils.FastEqual(v, ev.Pubkey) {
			isOwner = true
			break
		}
	}
	if !isOwner {
		for _, v := range c.admins {
			if utils.FastEqual(v, ev.Pubkey) {
				isAdmin = true
				break
			}
		}
	}
	if !isOwner && !isAdmin {
		return errorf.E("config event must be from owner or admin")
	}

	// Parse configuration from tags
	config := database.CuratingConfig{
		ConfigEventID: hex.EncodeToString(ev.ID[:]),
		ConfigPubkey:  pubkeyHex,
		ConfiguredAt:  ev.CreatedAt,
		DailyLimit:    DefaultDailyLimit,
		FirstBanHours: DefaultFirstBanHours,
		SecondBanHours: DefaultSecondBanHours,
	}

	for _, tag := range *ev.Tags {
		if tag.Len() < 2 {
			continue
		}
		key := string(tag.Key())
		value := string(tag.Value())

		switch key {
		case "daily_limit":
			if v, err := strconv.Atoi(value); err == nil && v > 0 {
				config.DailyLimit = v
			}
		case "ip_daily_limit":
			if v, err := strconv.Atoi(value); err == nil && v > 0 {
				config.IPDailyLimit = v
			}
		case "first_ban_hours":
			if v, err := strconv.Atoi(value); err == nil && v > 0 {
				config.FirstBanHours = v
			}
		case "second_ban_hours":
			if v, err := strconv.Atoi(value); err == nil && v > 0 {
				config.SecondBanHours = v
			}
		case "kind_category":
			config.KindCategories = append(config.KindCategories, value)
		case "kind_range":
			config.AllowedRanges = append(config.AllowedRanges, value)
		case "kind":
			if k, err := strconv.Atoi(value); err == nil {
				config.AllowedKinds = append(config.AllowedKinds, k)
			}
		}
	}

	// Save configuration
	if err := c.curatingACL.SaveConfig(config); err != nil {
		return errorf.E("failed to save config: %v", err)
	}

	// Refresh caches
	cp := config
	c.setConfigCache.TrySend(&cp)

	log.I.F("curating ACL: configuration updated from event %s by %s",
		config.ConfigEventID, config.ConfigPubkey)

	return nil
}

// IsTrusted checks if a pubkey is trusted
func (c *Curating) IsTrusted(pubkeyHex string) bool {
	if c.checkTrusted.Call(pubkeyHex) {
		return true
	}
	trusted, _ := c.curatingACL.IsPubkeyTrusted(pubkeyHex)
	return trusted
}

// IsBlacklisted checks if a pubkey is blacklisted
func (c *Curating) IsBlacklisted(pubkeyHex string) bool {
	if c.checkBlacklisted.Call(pubkeyHex) {
		return true
	}
	blacklisted, _ := c.curatingACL.IsPubkeyBlacklisted(pubkeyHex)
	return blacklisted
}

// TrustPubkey adds a pubkey to the trusted list
func (c *Curating) TrustPubkey(pubkeyHex, note string) error {
	pubkeyHex = strings.ToLower(pubkeyHex)
	if err := c.curatingACL.SaveTrustedPubkey(pubkeyHex, note); err != nil {
		return err
	}
	// Update cache via actor
	c.trustPubkey.TrySend(pubkeyHex)
	// Also remove from blacklist in DB
	c.curatingACL.RemoveBlacklistedPubkey(pubkeyHex)
	return nil
}

// UntrustPubkey removes a pubkey from the trusted list
func (c *Curating) UntrustPubkey(pubkeyHex string) error {
	pubkeyHex = strings.ToLower(pubkeyHex)
	if err := c.curatingACL.RemoveTrustedPubkey(pubkeyHex); err != nil {
		return err
	}
	// Update cache via actor
	c.untrust.TrySend(pubkeyHex)
	return nil
}

// BlacklistPubkey adds a pubkey to the blacklist
func (c *Curating) BlacklistPubkey(pubkeyHex, reason string) error {
	pubkeyHex = strings.ToLower(pubkeyHex)
	if err := c.curatingACL.SaveBlacklistedPubkey(pubkeyHex, reason); err != nil {
		return err
	}
	// Update cache via actor
	c.blacklistPubkey.TrySend(pubkeyHex)
	// Also remove from trusted list in DB
	c.curatingACL.RemoveTrustedPubkey(pubkeyHex)
	return nil
}

// UnblacklistPubkey removes a pubkey from the blacklist
func (c *Curating) UnblacklistPubkey(pubkeyHex string) error {
	pubkeyHex = strings.ToLower(pubkeyHex)
	if err := c.curatingACL.RemoveBlacklistedPubkey(pubkeyHex); err != nil {
		return err
	}
	// Update cache via actor
	c.unblacklist.TrySend(pubkeyHex)
	return nil
}

func init() {
	Registry.Register(new(Curating))
}
