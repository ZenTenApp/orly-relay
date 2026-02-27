package spider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/ws"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/interfaces/publisher"
	dsync "next.orly.dev/pkg/sync"
)

const (
	// BatchSize is the number of pubkeys per subscription batch
	BatchSize = 20
	// CatchupWindow is the extra time added to disconnection periods for catch-up
	CatchupWindow = 30 * time.Minute
	// ReconnectDelay is the initial delay between reconnection attempts
	ReconnectDelay = 10 * time.Second
	// MaxReconnectDelay is the maximum delay before switching to blackout
	MaxReconnectDelay = 1 * time.Hour
	// BlackoutPeriod is the duration to blacklist a relay after max backoff is reached
	BlackoutPeriod = 24 * time.Hour
	// BatchCreationDelay is the delay between creating each batch subscription
	BatchCreationDelay = 500 * time.Millisecond
	// RateLimitBackoffDuration is how long to wait when we get a rate limit error
	RateLimitBackoffDuration = 1 * time.Minute
	// RateLimitBackoffMultiplier is the factor by which we increase backoff on repeated rate limits
	RateLimitBackoffMultiplier = 2
	// MaxRateLimitBackoff is the maximum backoff duration for rate limiting
	MaxRateLimitBackoff = 30 * time.Minute
	// MainLoopInterval is how often the spider checks for updates
	MainLoopInterval = 5 * time.Minute
	// EventHandlerBufferSize is the buffer size for event channels
	EventHandlerBufferSize = 100
)

// Spider manages connections to admin relays and syncs events for followed pubkeys
type Spider struct {
	ctx    context.Context
	cancel context.CancelFunc
	db     *database.D
	pub    publisher.I
	mode   string

	// Configuration
	adminRelays       []string
	followList        [][]byte
	relayIdentityPubkey string          // Our relay's identity pubkey (hex)
	selfURLs          map[string]bool // URLs discovered to be ourselves (for fast lookups)

	// State management
	mu          sync.RWMutex
	connections map[string]*RelayConnection
	running     bool

	// Callbacks for getting updated data
	getAdminRelays func() []string
	getFollowList  func() [][]byte

	// Notification channel for follow list updates
	followListUpdated chan struct{}
}

// RelayConnection manages a single relay connection and its subscriptions
type RelayConnection struct {
	url    string
	client *ws.Client
	ctx    context.Context
	cancel context.CancelFunc
	spider *Spider

	// Subscription management
	mu            sync.RWMutex
	subscriptions map[string]*BatchSubscription

	// Disconnection tracking
	lastDisconnect      time.Time
	reconnectDelay      time.Duration
	connectionStartTime time.Time

	// Blackout tracking for IP filters
	blackoutUntil time.Time

	// Rate limiting tracking
	rateLimitBackoff time.Duration
	rateLimitUntil   time.Time
}

// BatchSubscription represents a subscription for a batch of pubkeys
type BatchSubscription struct {
	id        string
	pubkeys   [][]byte
	startTime time.Time
	sub       *ws.Subscription
	relay     *RelayConnection

	// Track disconnection periods for catch-up
	disconnectedAt *time.Time
}

// DisconnectionPeriod tracks when a subscription was disconnected
type DisconnectionPeriod struct {
	Start time.Time
	End   time.Time
}

// New creates a new Spider instance
func New(ctx context.Context, db *database.D, pub publisher.I, mode string) (s *Spider, err error) {
	if db == nil {
		err = errorf.E("database cannot be nil")
		return
	}

	// Validate mode
	switch mode {
	case "follows", "none":
		// Valid modes
	default:
		err = errorf.E("invalid spider mode: %s (valid modes: none, follows)", mode)
		return
	}

	ctx, cancel := context.WithCancel(ctx)

	// Get relay identity pubkey for self-detection
	var relayPubkey string
	if skb, err := db.GetRelayIdentitySecret(); err == nil && len(skb) == 32 {
		pk, _ := keys.SecretBytesToPubKeyHex(skb)
		relayPubkey = pk
	}

	s = &Spider{
		ctx:                 ctx,
		cancel:              cancel,
		db:                  db,
		pub:                 pub,
		mode:                mode,
		relayIdentityPubkey: relayPubkey,
		selfURLs:            make(map[string]bool),
		connections:         make(map[string]*RelayConnection),
		followListUpdated:   make(chan struct{}, 1),
	}

	return
}

// SetCallbacks sets the callback functions for getting updated admin relays and follow lists
func (s *Spider) SetCallbacks(getAdminRelays func() []string, getFollowList func() [][]byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getAdminRelays = getAdminRelays
	s.getFollowList = getFollowList
}

// NotifyFollowListUpdate signals the spider that the follow list has been updated
func (s *Spider) NotifyFollowListUpdate() {
	if s.followListUpdated != nil {
		select {
		case s.followListUpdated <- struct{}{}:
			log.D.F("spider: follow list update notification sent")
		default:
			// Channel full, update already pending
			log.D.F("spider: follow list update notification already pending")
		}
	}
}

// Start begins the spider operation
func (s *Spider) Start() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		err = errorf.E("spider already running")
		return
	}

	// Handle 'none' mode - no-op
	if s.mode == "none" {
		log.I.F("spider: mode is 'none', not starting")
		return
	}

	if s.getAdminRelays == nil || s.getFollowList == nil {
		err = errorf.E("callbacks must be set before starting")
		return
	}

	s.running = true

	// Start the main loop
	go s.mainLoop()

	log.I.F("spider: started in '%s' mode", s.mode)
	return
}

// Stop stops the spider operation
func (s *Spider) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	s.cancel()

	// Close all connections
	for _, conn := range s.connections {
		conn.close()
	}
	s.connections = make(map[string]*RelayConnection)

	log.I.F("spider: stopped")
}

// mainLoop is the main spider loop that manages connections and subscriptions
func (s *Spider) mainLoop() {
	ticker := time.NewTicker(MainLoopInterval)
	defer ticker.Stop()

	log.I.F("spider: main loop started, checking every %v", MainLoopInterval)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.followListUpdated:
			log.I.F("spider: follow list updated, refreshing connections")
			s.updateConnections()
		case <-ticker.C:
			log.D.F("spider: periodic check triggered")
			s.updateConnections()
		}
	}
}

// updateConnections updates relay connections based on current admin relays and follow lists
func (s *Spider) updateConnections() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}

	// Get current admin relays and follow list
	adminRelays := s.getAdminRelays()
	followList := s.getFollowList()
	s.mu.Unlock()

	if len(adminRelays) == 0 || len(followList) == 0 {
		log.D.F("spider: no admin relays (%d) or follow list (%d) available",
			len(adminRelays), len(followList))
		return
	}

	// Filter out self-relays WITHOUT holding mu — isSelfRelay takes mu internally
	var filteredRelays []string
	for _, url := range adminRelays {
		if s.isSelfRelay(url) {
			log.D.F("spider: skipping self-relay: %s", url)
			continue
		}
		filteredRelays = append(filteredRelays, url)
	}

	// Re-acquire lock for the mutation phase
	s.mu.Lock()
	defer s.mu.Unlock()

	currentRelays := make(map[string]bool)
	for _, url := range filteredRelays {
		currentRelays[url] = true

		if conn, exists := s.connections[url]; exists {
			// Update existing connection
			conn.updateSubscriptions(followList)
		} else {
			// Create new connection
			s.createConnection(url, followList)
		}
	}

	// Remove connections for relays no longer in admin list
	for url, conn := range s.connections {
		if !currentRelays[url] {
			log.I.F("spider: removing connection to %s (no longer in admin relays)", url)
			conn.close()
			delete(s.connections, url)
		}
	}
}

// createConnection creates a new relay connection
func (s *Spider) createConnection(url string, followList [][]byte) {
	log.I.F("spider: creating connection to %s", url)

	ctx, cancel := context.WithCancel(s.ctx)
	conn := &RelayConnection{
		url:            url,
		ctx:            ctx,
		cancel:         cancel,
		spider:         s,
		subscriptions:  make(map[string]*BatchSubscription),
		reconnectDelay: ReconnectDelay,
	}

	s.connections[url] = conn

	// Start connection in goroutine
	go conn.manage(followList)
}

// manage handles the lifecycle of a relay connection
func (rc *RelayConnection) manage(followList [][]byte) {
	for {
		// Check context first
		select {
		case <-rc.ctx.Done():
			log.D.F("spider: connection manager for %s stopping (context done)", rc.url)
			return
		default:
		}

		// Check if relay is blacked out
		if rc.isBlackedOut() {
			waitDuration := time.Until(rc.blackoutUntil)
			log.I.F("spider: %s is blacked out for %v more", rc.url, waitDuration)

			// Wait for blackout to expire or context cancellation
			select {
			case <-rc.ctx.Done():
				return
			case <-time.After(waitDuration):
				// Blackout expired, reset delay and try again
				rc.reconnectDelay = ReconnectDelay
				log.I.F("spider: blackout period ended for %s, retrying", rc.url)
			}
			continue
		}

		// Attempt to connect
		log.D.F("spider: attempting to connect to %s (backoff: %v)", rc.url, rc.reconnectDelay)
		if err := rc.connect(); chk.E(err) {
			log.W.F("spider: failed to connect to %s: %v", rc.url, err)
			rc.waitBeforeReconnect()
			continue
		}

		log.I.F("spider: connected to %s", rc.url)
		rc.connectionStartTime = time.Now()

		// Only reset reconnect delay on successful connection
		// (don't reset if we had a quick disconnect before)
		if rc.reconnectDelay > ReconnectDelay*8 {
			// Gradual recovery: reduce by half instead of full reset
			rc.reconnectDelay = rc.reconnectDelay / 2
			log.D.F("spider: reducing backoff for %s to %v", rc.url, rc.reconnectDelay)
		} else {
			rc.reconnectDelay = ReconnectDelay
		}
		rc.blackoutUntil = time.Time{} // Clear blackout on successful connection

		// Create subscriptions for follow list
		rc.createSubscriptions(followList)

		// Wait for disconnection
		<-rc.client.Context().Done()

		log.W.F("spider: disconnected from %s: %v", rc.url, rc.client.ConnectionCause())

		// Check if disconnection happened very quickly (likely IP filter or ban)
		connectionDuration := time.Since(rc.connectionStartTime)
		const quickDisconnectThreshold = 2 * time.Minute
		if connectionDuration < quickDisconnectThreshold {
			log.W.F("spider: quick disconnection from %s after %v (likely connection issue/ban)", rc.url, connectionDuration)
			// Don't reset the delay, keep the backoff and increase it
			rc.waitBeforeReconnect()
		} else {
			// Normal disconnection after decent uptime - gentle backoff
			log.I.F("spider: normal disconnection from %s after %v uptime", rc.url, connectionDuration)
			// Small delay before reconnecting
			select {
			case <-rc.ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}

		rc.handleDisconnection()

		// Clean up
		rc.client = nil
		rc.clearSubscriptions()
	}
}

// connect establishes a websocket connection to the relay
func (rc *RelayConnection) connect() (err error) {
	connectCtx, cancel := context.WithTimeout(rc.ctx, 10*time.Second)
	defer cancel()

	// Create client with notice handler to detect rate limiting
	rc.client, err = ws.RelayConnect(connectCtx, rc.url, ws.WithNoticeHandler(rc.handleNotice))
	if chk.E(err) {
		return
	}

	return
}

// handleNotice processes NOTICE messages from the relay
func (rc *RelayConnection) handleNotice(notice []byte) {
	noticeStr := string(notice)
	log.D.F("spider: NOTICE from %s: '%s'", rc.url, noticeStr)

	// Check for rate limiting errors
	if strings.Contains(noticeStr, "too many concurrent REQs") ||
		strings.Contains(noticeStr, "rate limit") ||
		strings.Contains(noticeStr, "slow down") {
		rc.handleRateLimit()
	}
}

// handleRateLimit applies backoff when rate limiting is detected
func (rc *RelayConnection) handleRateLimit() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Initialize backoff if not set
	if rc.rateLimitBackoff == 0 {
		rc.rateLimitBackoff = RateLimitBackoffDuration
	} else {
		// Exponential backoff
		rc.rateLimitBackoff *= RateLimitBackoffMultiplier
		if rc.rateLimitBackoff > MaxRateLimitBackoff {
			rc.rateLimitBackoff = MaxRateLimitBackoff
		}
	}

	rc.rateLimitUntil = time.Now().Add(rc.rateLimitBackoff)
	log.W.F("spider: rate limit detected on %s, backing off for %v until %v",
		rc.url, rc.rateLimitBackoff, rc.rateLimitUntil)

	// Close all current subscriptions to reduce load
	rc.clearSubscriptionsLocked()
}

// waitBeforeReconnect waits before attempting to reconnect with exponential backoff
func (rc *RelayConnection) waitBeforeReconnect() {
	log.I.F("spider: waiting %v before reconnecting to %s", rc.reconnectDelay, rc.url)

	select {
	case <-rc.ctx.Done():
		return
	case <-time.After(rc.reconnectDelay):
	}

	// Exponential backoff - double every time
	// 10s -> 20s -> 40s -> 80s (1.3m) -> 160s (2.7m) -> 320s (5.3m) -> 640s (10.7m) -> 1280s (21m) -> 2560s (42m) -> 3600s (1h)
	rc.reconnectDelay *= 2

	// Cap at MaxReconnectDelay (1 hour), then switch to 24-hour blackout
	if rc.reconnectDelay >= MaxReconnectDelay {
		rc.blackoutUntil = time.Now().Add(BlackoutPeriod)
		rc.reconnectDelay = ReconnectDelay // Reset for after blackout
		log.W.F("spider: max reconnect backoff reached for %s, entering 24-hour blackout period", rc.url)
	}
}

// isBlackedOut returns true if the relay is currently blacked out
func (rc *RelayConnection) isBlackedOut() bool {
	return !rc.blackoutUntil.IsZero() && time.Now().Before(rc.blackoutUntil)
}

// handleDisconnection records disconnection time for catch-up logic
func (rc *RelayConnection) handleDisconnection() {
	now := time.Now()
	rc.lastDisconnect = now

	// Mark all subscriptions as disconnected
	rc.mu.Lock()
	defer rc.mu.Unlock()

	for _, sub := range rc.subscriptions {
		if sub.disconnectedAt == nil {
			sub.disconnectedAt = &now
		}
	}
}

// createSubscriptions creates batch subscriptions for the follow list
func (rc *RelayConnection) createSubscriptions(followList [][]byte) {
	rc.mu.Lock()

	// Check if we're in a rate limit backoff period
	if time.Now().Before(rc.rateLimitUntil) {
		remaining := time.Until(rc.rateLimitUntil)
		rc.mu.Unlock()
		log.W.F("spider: skipping subscription creation for %s, rate limited for %v more", rc.url, remaining)

		// Schedule retry after backoff period
		go func() {
			time.Sleep(remaining)
			rc.createSubscriptions(followList)
		}()
		return
	}

	// Clear rate limit backoff on successful subscription attempt
	rc.rateLimitBackoff = 0
	rc.rateLimitUntil = time.Time{}

	// Clear existing subscriptions
	rc.clearSubscriptionsLocked()

	// Create batches of pubkeys
	batches := rc.createBatches(followList)

	log.I.F("spider: creating %d subscription batches for %d pubkeys on %s",
		len(batches), len(followList), rc.url)

	// Release lock before creating subscriptions to avoid holding it during delays
	rc.mu.Unlock()

	for i, batch := range batches {
		// Check context before creating each batch
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		batchID := fmt.Sprintf("batch-%d", i)

		rc.mu.Lock()
		rc.createBatchSubscription(batchID, batch)
		rc.mu.Unlock()

		// Add delay between batches to avoid overwhelming the relay
		if i < len(batches)-1 { // Don't delay after the last batch
			time.Sleep(BatchCreationDelay)
		}
	}
}

// createBatches splits the follow list into batches of BatchSize
func (rc *RelayConnection) createBatches(followList [][]byte) (batches [][][]byte) {
	for i := 0; i < len(followList); i += BatchSize {
		end := i + BatchSize
		if end > len(followList) {
			end = len(followList)
		}

		batch := make([][]byte, end-i)
		copy(batch, followList[i:end])
		batches = append(batches, batch)
	}
	return
}

// createBatchSubscription creates a subscription for a batch of pubkeys
func (rc *RelayConnection) createBatchSubscription(batchID string, pubkeys [][]byte) {
	if rc.client == nil {
		return
	}

	// Create filters: one for authors, one for p tags
	// For #p tag filters, all pubkeys must be in a single tag array as hex-encoded strings
	tagElements := [][]byte{[]byte("p")} // First element is the key
	for _, pk := range pubkeys {
		pkHex := hex.EncAppend(nil, pk)
		tagElements = append(tagElements, pkHex)
	}
	pTags := &tag.S{tag.NewFromBytesSlice(tagElements...)}

	filters := filter.NewS(
		&filter.F{
			Authors: tag.NewFromBytesSlice(pubkeys...),
		},
		&filter.F{
			Tags: pTags,
		},
	)

	// Subscribe
	sub, err := rc.client.Subscribe(rc.ctx, filters)
	if chk.E(err) {
		log.E.F("spider: failed to create subscription %s on %s: %v", batchID, rc.url, err)
		return
	}

	batchSub := &BatchSubscription{
		id:        batchID,
		pubkeys:   pubkeys,
		startTime: time.Now(),
		sub:       sub,
		relay:     rc,
	}

	rc.subscriptions[batchID] = batchSub

	// Start event handler
	go batchSub.handleEvents()

	log.D.F("spider: created subscription %s for %d pubkeys on %s",
		batchID, len(pubkeys), rc.url)
}

// handleEvents processes events from the subscription
func (bs *BatchSubscription) handleEvents() {
	// Throttle event processing to avoid CPU spikes
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-bs.relay.ctx.Done():
			return
		case ev := <-bs.sub.Events:
			if ev == nil {
				return // Subscription closed
			}

			// Wait for throttle tick to avoid processing events too rapidly
			<-ticker.C

			// Save event to database
			if _, err := bs.relay.spider.db.SaveEvent(bs.relay.ctx, ev); err != nil {
				// Ignore duplicate events and other errors
				log.T.F("spider: failed to save event from %s: %v", bs.relay.url, err)
			} else {
				// Publish event if it was newly saved
				if bs.relay.spider.pub != nil {
					go bs.relay.spider.pub.Deliver(ev)
				}
				log.T.F("spider: saved event from %s", bs.relay.url)
			}
		}
	}
}

// updateSubscriptions updates subscriptions for a connection with new follow list
func (rc *RelayConnection) updateSubscriptions(followList [][]byte) {
	if rc.client == nil || !rc.client.IsConnected() {
		return // Will be handled on reconnection
	}

	rc.mu.Lock()

	// Check if we're in a rate limit backoff period
	if time.Now().Before(rc.rateLimitUntil) {
		remaining := time.Until(rc.rateLimitUntil)
		rc.mu.Unlock()
		log.D.F("spider: deferring subscription update for %s, rate limited for %v more", rc.url, remaining)
		return
	}

	// Check if we need to perform catch-up for disconnected subscriptions
	now := time.Now()
	needsCatchup := false

	for _, sub := range rc.subscriptions {
		if sub.disconnectedAt != nil {
			needsCatchup = true
			rc.performCatchup(sub, *sub.disconnectedAt, now, followList)
			sub.disconnectedAt = nil // Clear disconnection marker
		}
	}

	if needsCatchup {
		log.I.F("spider: performed catch-up for disconnected subscriptions on %s", rc.url)
	}

	// Recreate subscriptions with updated follow list
	rc.clearSubscriptionsLocked()

	batches := rc.createBatches(followList)

	// Release lock before creating subscriptions
	rc.mu.Unlock()

	for i, batch := range batches {
		// Check context before creating each batch
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		batchID := fmt.Sprintf("batch-%d", i)

		rc.mu.Lock()
		rc.createBatchSubscription(batchID, batch)
		rc.mu.Unlock()

		// Add delay between batches
		if i < len(batches)-1 {
			time.Sleep(BatchCreationDelay)
		}
	}
}

// performCatchup queries for events missed during disconnection
func (rc *RelayConnection) performCatchup(sub *BatchSubscription, disconnectTime, reconnectTime time.Time, followList [][]byte) {
	// Expand time window by CatchupWindow on both sides
	since := disconnectTime.Add(-CatchupWindow)
	until := reconnectTime.Add(CatchupWindow)

	log.I.F("spider: performing catch-up for %s from %v to %v (expanded window)",
		rc.url, since, until)

	// Create catch-up filters with time constraints
	sinceTs := timestamp.T{V: since.Unix()}
	untilTs := timestamp.T{V: until.Unix()}

	// Create filters with hex-encoded pubkeys for #p tags
	// All pubkeys must be in a single tag array
	tagElements := [][]byte{[]byte("p")} // First element is the key
	for _, pk := range sub.pubkeys {
		pkHex := hex.EncAppend(nil, pk)
		tagElements = append(tagElements, pkHex)
	}
	pTags := &tag.S{tag.NewFromBytesSlice(tagElements...)}

	filters := filter.NewS(
		&filter.F{
			Authors: tag.NewFromBytesSlice(sub.pubkeys...),
			Since:   &sinceTs,
			Until:   &untilTs,
		},
		&filter.F{
			Tags:  pTags,
			Since: &sinceTs,
			Until: &untilTs,
		},
	)

	// Create temporary subscription for catch-up
	catchupCtx, cancel := context.WithTimeout(rc.ctx, 30*time.Second)
	defer cancel()

	catchupSub, err := rc.client.Subscribe(catchupCtx, filters)
	if chk.E(err) {
		log.E.F("spider: failed to create catch-up subscription on %s: %v", rc.url, err)
		return
	}
	defer catchupSub.Unsub()

	// Process catch-up events with throttling
	eventCount := 0
	timeout := time.After(60 * time.Second) // Increased timeout for catch-up
	throttle := time.NewTicker(20 * time.Millisecond)
	defer throttle.Stop()

	for {
		select {
		case <-catchupCtx.Done():
			log.I.F("spider: catch-up completed on %s, processed %d events", rc.url, eventCount)
			return
		case <-timeout:
			log.I.F("spider: catch-up timeout on %s, processed %d events", rc.url, eventCount)
			return
		case <-catchupSub.EndOfStoredEvents:
			log.I.F("spider: catch-up EOSE on %s, processed %d events", rc.url, eventCount)
			return
		case ev := <-catchupSub.Events:
			if ev == nil {
				return
			}

			// Throttle event processing
			<-throttle.C

			eventCount++

			// Save event to database
			if _, err := rc.spider.db.SaveEvent(rc.ctx, ev); err != nil {
				// Silently ignore errors (mostly duplicates)
			} else {
				// Publish event if it was newly saved
				if rc.spider.pub != nil {
					go rc.spider.pub.Deliver(ev)
				}
				log.T.F("spider: catch-up saved event %s from %s",
					hex.Enc(ev.ID[:]), rc.url)
			}
		}
	}
}

// clearSubscriptions clears all subscriptions (with lock)
func (rc *RelayConnection) clearSubscriptions() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.clearSubscriptionsLocked()
}

// clearSubscriptionsLocked clears all subscriptions (without lock)
func (rc *RelayConnection) clearSubscriptionsLocked() {
	for _, sub := range rc.subscriptions {
		if sub.sub != nil {
			sub.sub.Unsub()
		}
	}
	rc.subscriptions = make(map[string]*BatchSubscription)
}

// close closes the relay connection
func (rc *RelayConnection) close() {
	rc.clearSubscriptions()

	if rc.client != nil {
		rc.client.Close()
		rc.client = nil
	}

	rc.cancel()
}

// isSelfRelay checks if a relay URL is actually ourselves by comparing NIP-11 pubkeys
func (s *Spider) isSelfRelay(relayURL string) bool {
	// If we don't have a relay identity pubkey, can't compare
	if s.relayIdentityPubkey == "" {
		return false
	}

	s.mu.RLock()
	// Fast path: check if we already know this URL is ours
	if s.selfURLs[relayURL] {
		s.mu.RUnlock()
		log.D.F("spider: skipping self-relay (known URL): %s", relayURL)
		return true
	}
	s.mu.RUnlock()

	// Slow path: check via NIP-11 pubkey
	nip11Cache := dsync.NewNIP11Cache(30 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	peerPubkey, err := nip11Cache.GetPubkey(ctx, relayURL)
	if err != nil {
		log.D.F("spider: couldn't fetch NIP-11 for %s: %v", relayURL, err)
		return false
	}

	if peerPubkey == s.relayIdentityPubkey {
		log.I.F("spider: discovered self-relay: %s (pubkey: %s)", relayURL, s.relayIdentityPubkey)
		// Cache this URL as ours for future fast lookups
		s.mu.Lock()
		s.selfURLs[relayURL] = true
		s.mu.Unlock()
		return true
	}

	return false
}
