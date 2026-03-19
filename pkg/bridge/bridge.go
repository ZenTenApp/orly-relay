package bridge

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	aclgrpc "next.orly.dev/pkg/acl/grpc"
	bridgesmtp "next.orly.dev/pkg/bridge/smtp"
	"next.orly.dev/pkg/nostr/protocol/marmot"
)

// dmFormat tracks which DM protocol a sender last used.
type dmFormat int

const (
	dmFormatKind4    dmFormat = iota // NIP-04 style (kind 4)
	dmFormatGiftWrap                 // NIP-17 gift wrap (kind 1059)
	dmFormatMLS                      // MLS/NIP-EE (kind 443/444/445)
)

// Bridge is the Nostr-Email bridge. It manages identity, relay connection,
// Marmot DM handling, and SMTP transport.
type Bridge struct {
	cfg    *Config
	sign   signer.I
	source IdentitySource
	relay  *RelayConn

	router     *Router
	smtpServer *bridgesmtp.Server

	aclClient *aclgrpc.Client
	mlsClient *marmot.Client

	// senderFormats tracks the DM format last used by each sender so the
	// bridge replies in the same format. Protected by senderFmtMu.
	senderFormats map[string]dmFormat
	senderFmtMu   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// dbGetter is a function that returns the relay identity secret key
	// from the database. Non-nil in monolithic mode, nil in standalone.
	dbGetter func() ([]byte, error)
}

// New creates a new Bridge. The dbGetter is optional — pass nil for standalone
// mode (the bridge will fall back to file-based identity).
func New(cfg *Config, dbGetter func() ([]byte, error)) *Bridge {
	return &Bridge{
		cfg:           cfg,
		dbGetter:      dbGetter,
		senderFormats: make(map[string]dmFormat),
	}
}

// Start initializes the bridge: resolves identity, connects to relay, and
// begins listening for events.
func (b *Bridge) Start(ctx context.Context) error {
	b.ctx, b.cancel = context.WithCancel(ctx)

	// Ensure data directory exists
	if err := os.MkdirAll(b.cfg.DataDir, 0700); err != nil {
		return fmt.Errorf("create bridge data dir: %w", err)
	}

	// Resolve identity
	sign, source, err := ResolveIdentity(b.cfg.NSEC, b.dbGetter, b.cfg.DataDir)
	if err != nil {
		return fmt.Errorf("resolve identity: %w", err)
	}
	b.sign = sign
	b.source = source

	pubHex := hex.Enc(b.sign.Pub())
	npub, _ := bech32encoding.BinToNpub(b.sign.Pub())
	sourceStr := identitySourceString(source)
	log.I.F("bridge identity: %s (%s) [source: %s]", string(npub), string(pubHex), sourceStr)

	if b.cfg.Domain != "" {
		log.I.F("bridge email domain: %s", b.cfg.Domain)
	}

	// Connect to ACL gRPC server if configured
	if b.cfg.ACLGRPCServer != "" {
		aclClient, err := aclgrpc.New(b.ctx, &aclgrpc.ClientConfig{
			ServerAddress:  b.cfg.ACLGRPCServer,
			ConnectTimeout: 15 * time.Second,
		})
		if err != nil {
			return fmt.Errorf("connect to ACL gRPC server: %w", err)
		}
		b.aclClient = aclClient
		log.I.F("bridge connected to ACL gRPC server at %s", b.cfg.ACLGRPCServer)
	}

	// Build the sendDM callback
	sendDM := b.makeSendDM()

	// Initialize components
	if err := b.initComponents(sendDM); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	// Start SMTP server for inbound email
	if b.cfg.Domain != "" {
		if err := b.startSMTPServer(sendDM); err != nil {
			return fmt.Errorf("start SMTP server: %w", err)
		}
	}

	// Connect to relay (standalone mode)
	if b.cfg.RelayURL != "" {
		b.relay = NewRelayConn(b.cfg.RelayURL, b.sign)
		if err := b.relay.Connect(b.ctx); err != nil {
			return fmt.Errorf("relay connection: %w", err)
		}

		// Start subscription loop in background
		b.wg.Add(1)
		go b.relayWatchLoop()

		// Publish kind 0 profile if template exists
		if err := b.publishProfile(); err != nil {
			log.W.F("publish bridge profile: %v", err)
		}

		// Publish kind 10002 + 10050 relay lists
		if err := b.publishRelayList(); err != nil {
			log.W.F("publish bridge relay list: %v", err)
		}

		// Broadcast identity to popular relays in background
		go b.broadcastIdentity()

		// Initialize MLS if enabled
		if b.cfg.MLSEnabled {
			if err := b.initMLS(); err != nil {
				log.W.F("MLS init failed (continuing without MLS): %v", err)
			} else {
				log.I.F("MLS (NIP-EE) enabled")
			}
		}
	}

	log.I.F("bridge started")
	return nil
}

// initComponents sets up the router, outbound processor, subscription handler,
// and payment processor.
func (b *Bridge) initComponents(sendDM func(string, string) error) error {
	// SMTP client for outbound email
	smtpCfg := bridgesmtp.ClientConfig{
		FromDomain:    b.cfg.Domain,
		RelayHost:     b.cfg.SMTPRelayHost,
		RelayPort:     b.cfg.SMTPRelayPort,
		RelayUsername: b.cfg.SMTPRelayUsername,
		RelayPassword: b.cfg.SMTPRelayPassword,
	}

	// Load DKIM signer if configured
	if b.cfg.DKIMKeyPath != "" {
		dkim, err := bridgesmtp.NewDKIMSigner(b.cfg.Domain, b.cfg.DKIMSelector, b.cfg.DKIMKeyPath)
		if err != nil {
			log.W.F("DKIM signer init failed, continuing without DKIM: %v", err)
		} else {
			smtpCfg.DKIMSigner = dkim
		}
	}

	smtpClient := bridgesmtp.NewClient(smtpCfg)

	// Rate limiter
	rateLimiter := NewRateLimiter(DefaultRateLimitConfig())

	// Subscription store + handler
	subStore, err := NewFileSubscriptionStore(b.cfg.DataDir)
	if err != nil {
		return fmt.Errorf("subscription store: %w", err)
	}

	var payments *PaymentProcessor
	if b.cfg.NWCURI != "" {
		payments, err = NewPaymentProcessor(b.cfg.NWCURI, b.cfg.MonthlyPriceSats)
		if err != nil {
			log.W.F("payment processor init failed: %v", err)
		}
	}

	aliasPriceSats := b.cfg.AliasPriceSats
	if aliasPriceSats == 0 && b.cfg.MonthlyPriceSats > 0 {
		aliasPriceSats = b.cfg.MonthlyPriceSats * 2
	}

	subHandler := NewSubscriptionHandler(subStore, payments, sendDM, b.cfg.MonthlyPriceSats, b.aclClient, aliasPriceSats)
	outbound := NewOutboundProcessor(smtpClient, rateLimiter, subHandler, b.cfg.Domain, sendDM, b.aclClient)
	b.router = NewRouter(subHandler, outbound, sendDM)

	return nil
}

// startSMTPServer starts the inbound SMTP server that receives forwarded mail.
func (b *Bridge) startSMTPServer(sendDM func(string, string) error) error {
	listenAddr := fmt.Sprintf("%s:%d", b.cfg.SMTPHost, b.cfg.SMTPPort)
	cfg := bridgesmtp.ServerConfig{
		Domain:          b.cfg.Domain,
		ListenAddr:      listenAddr,
		MaxMessageBytes: 25 * 1024 * 1024,
		MaxRecipients:   10,
		ReadTimeout:     60 * time.Second,
		WriteTimeout:    60 * time.Second,
	}

	// Inbound processor (no Blossom uploader for now)
	inbound := NewInboundProcessor(nil, b.cfg.ComposeURL, sendDM)

	handler := func(email *bridgesmtp.InboundEmail) error {
		for _, to := range email.To {
			pubkeyHex, err := resolveRecipientPubkey(to, b.cfg.Domain, b.aclClient)
			if err != nil {
				log.W.F("cannot resolve recipient %s: %v", to, err)
				continue
			}
			if err := inbound.ProcessInbound(email, pubkeyHex); err != nil {
				log.E.F("inbound processing failed for %s: %v", to, err)
			}
		}
		return nil
	}

	b.smtpServer = bridgesmtp.NewServer(cfg, handler)
	return b.smtpServer.Start()
}

// Stop gracefully shuts down the bridge.
func (b *Bridge) Stop() {
	log.I.F("bridge stopping")
	if b.cancel != nil {
		b.cancel()
	}
	if b.smtpServer != nil {
		b.smtpServer.Stop(context.Background())
	}
	if b.relay != nil {
		b.relay.Close()
	}
	if b.aclClient != nil {
		b.aclClient.Close()
	}
	b.wg.Wait()
	log.I.F("bridge stopped")
}

// Signer returns the bridge's identity signer.
func (b *Bridge) Signer() signer.I {
	return b.sign
}

// IdentitySource returns how the identity was resolved.
func (b *Bridge) IdentitySource() IdentitySource {
	return b.source
}

// relayWatchLoop subscribes to kind 4 DMs addressed to the bridge and routes
// them through the router. It reconnects and resubscribes on disconnection.
func (b *Bridge) relayWatchLoop() {
	defer b.wg.Done()

	if b.relay == nil || b.sign == nil {
		<-b.ctx.Done()
		return
	}

	for {
		if err := b.subscribeAndProcess(); err != nil {
			if b.ctx.Err() != nil {
				return // context cancelled, clean exit
			}
			log.W.F("subscription loop error: %v, reconnecting...", err)
			if err := b.relay.Reconnect(); err != nil {
				if b.ctx.Err() != nil {
					return
				}
				log.E.F("reconnect failed: %v", err)
			}
		}
	}
}

// subscribeAndProcess creates a subscription for DMs to the bridge and
// processes events until the channel closes or context is cancelled.
// Subscribes to both kind 4 (NIP-04) and kind 1059 (NIP-17 gift wrap).
func (b *Bridge) subscribeAndProcess() error {
	bridgePubHex := hex.Enc(b.sign.Pub())

	// Since = now - 3 days: gift-wrap timestamps are randomized up to
	// -2 days (NIP-59), so we need a lookback window to catch them.
	since := time.Now().Add(-3 * 24 * time.Hour).Unix()
	ff := filter.NewS(
		&filter.F{
			Kinds: kind.NewS(kind.New(4), kind.New(1059)),
			Tags: tag.NewS(
				tag.NewFromAny("p", bridgePubHex),
			),
			Since: &timestamp.T{V: since},
		},
	)

	sub, err := b.relay.Subscribe(b.ctx, ff)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	defer sub.Close()

	log.I.F("subscribed to DMs (kind 4 + 1059) for bridge pubkey %s", bridgePubHex)

	for {
		select {
		case <-b.ctx.Done():
			return nil
		case ev, ok := <-sub.Events():
			if !ok {
				return fmt.Errorf("event channel closed")
			}
			switch ev.Kind {
			case 1059:
				b.handleGiftWrapEvent(ev)
			default:
				b.handleDMEvent(ev)
			}
		}
	}
}

// handleDMEvent decrypts a kind 4 DM event and routes it.
// Tries NIP-44 decryption first, falls back to NIP-04.
func (b *Bridge) handleDMEvent(ev *event.E) {
	senderPubHex := hex.Enc(ev.Pubkey[:])

	// Skip our own outbound DMs to prevent feedback loop
	bridgePubHex := hex.Enc(b.sign.Pub())
	if senderPubHex == bridgePubHex {
		return
	}

	// Try NIP-44 first
	conversationKey, err := encryption.GenerateConversationKey(
		b.sign.Sec(), ev.Pubkey[:],
	)
	if err != nil {
		log.E.F("ECDH failed for sender %s: %v", senderPubHex, err)
		return
	}

	decrypted, err := encryption.Decrypt(conversationKey, string(ev.Content))
	if err != nil {
		log.I.F("NIP-44 decrypt failed for sender %s, trying NIP-04: %v", senderPubHex, err)
		// Fall back to NIP-04 decryption
		nip4Key, keyErr := encryption.GenerateNip4Key(b.sign.Sec(), ev.Pubkey[:])
		if keyErr != nil {
			log.E.F("NIP-04 key generation failed for sender %s: %v", senderPubHex, keyErr)
			return
		}
		nip4Decrypted, nip4Err := encryption.DecryptNip4(ev.Content, nip4Key)
		if nip4Err != nil {
			log.I.F("DM decryption failed from %s (tried NIP-44 and NIP-04): %v", senderPubHex, nip4Err)
			return
		}
		decrypted = string(nip4Decrypted)
		log.I.F("NIP-04 fallback succeeded for sender %s", senderPubHex)
	}

	log.I.F("received kind 4 DM from %s: %d bytes", senderPubHex, len(decrypted))

	b.recordSenderFormat(senderPubHex, dmFormatKind4)

	if b.router != nil {
		b.router.RouteDM(b.ctx, senderPubHex, decrypted)
	}
}

// handleGiftWrapEvent unwraps a kind 1059 event. Both NIP-17 DMs and MLS
// Welcomes arrive as kind 1059. Strategy: try NIP-17 first (expects inner
// kind 13 seal -> kind 14 DM). If that fails and MLS is enabled, pass to
// the MLS client (expects inner kind 13 seal -> kind 444 Welcome).
func (b *Bridge) handleGiftWrapEvent(ev *event.E) {
	dm, err := unwrapGiftWrap(ev, b.sign)
	if err != nil {
		// NIP-17 unwrap failed. Try MLS Welcome if enabled.
		if b.mlsClient != nil {
			if mlsErr := b.mlsClient.HandleEvent(b.ctx, ev); mlsErr != nil {
				log.D.F("kind 1059 unwrap failed (NIP-17: %v, MLS: %v)", err, mlsErr)
			}
		} else {
			log.I.F("gift wrap unwrap failed: %v", err)
		}
		return
	}

	// Skip our own outbound DMs to prevent feedback loop
	bridgePubHex := hex.Enc(b.sign.Pub())
	if dm.SenderPubHex == bridgePubHex {
		log.I.F("skipping own gift wrap DM")
		return
	}

	log.I.F("received NIP-17 DM from %s: %d bytes", dm.SenderPubHex, len(dm.Content))

	b.recordSenderFormat(dm.SenderPubHex, dmFormatGiftWrap)

	if b.router != nil {
		b.router.RouteDM(b.ctx, dm.SenderPubHex, dm.Content)
	}
}

// recordSenderFormat tracks the DM format a sender used.
func (b *Bridge) recordSenderFormat(pubkeyHex string, format dmFormat) {
	b.senderFmtMu.Lock()
	b.senderFormats[pubkeyHex] = format
	b.senderFmtMu.Unlock()
}

// getSenderFormat returns the last DM format used by a sender.
// Returns dmFormatKind4 as default for unknown senders (backward compatible).
func (b *Bridge) getSenderFormat(pubkeyHex string) dmFormat {
	b.senderFmtMu.RLock()
	defer b.senderFmtMu.RUnlock()
	f, ok := b.senderFormats[pubkeyHex]
	if !ok {
		return dmFormatKind4
	}
	return f
}

// makeSendDM returns a callback that encrypts and publishes DMs to the given
// pubkey via the relay. Replies in the same format the sender used: kind 4
// for NIP-04 senders, kind 1059 gift wrap for NIP-17 senders. For unknown
// senders (inbound email), sends kind 4 as the default.
func (b *Bridge) makeSendDM() func(pubkeyHex string, content string) error {
	return func(pubkeyHex string, content string) error {
		if b.relay == nil {
			return fmt.Errorf("no relay connection")
		}

		format := b.getSenderFormat(pubkeyHex)

		log.I.F("sending DM reply to %s (format=%d, %d bytes)", pubkeyHex, format, len(content))

		switch format {
		case dmFormatMLS:
			if err := b.sendMLSDM(pubkeyHex, content); err != nil {
				return fmt.Errorf("MLS DM: %w", err)
			}
			log.I.F("sent MLS DM to %s (%d bytes)", pubkeyHex, len(content))
		case dmFormatGiftWrap:
			if err := b.sendGiftWrapDM(pubkeyHex, content); err != nil {
				return fmt.Errorf("gift wrap DM: %w", err)
			}
			log.I.F("sent NIP-17 DM to %s (%d bytes)", pubkeyHex, len(content))
		default:
			if err := b.sendKind4DM(pubkeyHex, content); err != nil {
				return fmt.Errorf("kind 4 DM: %w", err)
			}
			log.I.F("sent kind 4 DM to %s (%d bytes)", pubkeyHex, len(content))
		}

		return nil
	}
}

// sendKind4DM sends a kind 4 encrypted DM using NIP-04.
func (b *Bridge) sendKind4DM(pubkeyHex, content string) error {
	recipientPub, err := hex.Dec(pubkeyHex)
	if err != nil {
		return fmt.Errorf("decode recipient pubkey: %w", err)
	}

	sharedKey, err := encryption.GenerateNip4Key(b.sign.Sec(), recipientPub)
	if err != nil {
		return fmt.Errorf("NIP-04 shared key: %w", err)
	}

	encrypted, err := encryption.EncryptNip4([]byte(content), sharedKey)
	if err != nil {
		return fmt.Errorf("NIP-04 encrypt: %w", err)
	}

	now := time.Now().Unix()
	expiration := strconv.FormatInt(now+defaultDMExpirationSeconds, 10)
	ev := &event.E{
		Content:   encrypted,
		CreatedAt: now,
		Kind:      4,
		Tags: tag.NewS(
			tag.NewFromAny("p", pubkeyHex),
			tag.NewFromAny("expiration", expiration),
		),
	}
	if err := ev.Sign(b.sign); chk.E(err) {
		return fmt.Errorf("sign: %w", err)
	}
	return b.relay.Publish(b.ctx, ev)
}

// sendGiftWrapDM sends a NIP-17 gift-wrapped DM (kind 1059).
// Publishes both a recipient copy and a self-addressed copy per NIP-17.
func (b *Bridge) sendGiftWrapDM(pubkeyHex, content string) error {
	recipientWrap, selfWrap, err := wrapGiftWrap(pubkeyHex, content, b.sign)
	if err != nil {
		return fmt.Errorf("wrap: %w", err)
	}
	if err := b.relay.Publish(b.ctx, recipientWrap); err != nil {
		return fmt.Errorf("publish recipient wrap: %w", err)
	}
	// Self-copy — best-effort, don't fail the send
	if err := b.relay.Publish(b.ctx, selfWrap); err != nil {
		log.W.F("failed to publish self-copy gift wrap: %v", err)
	}
	return nil
}

// resolveRecipientPubkey extracts the Nostr pubkey hex from an email address
// local part. The local part can be an npub (bech32), a hex pubkey, or an alias.
// When aclClient is non-nil, alias lookup is attempted.
func resolveRecipientPubkey(emailAddr, domain string, aclClient *aclgrpc.Client) (string, error) {
	parts := strings.SplitN(emailAddr, "@", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid email address: %s", emailAddr)
	}
	local := strings.ToLower(parts[0])

	// Try npub first
	if strings.HasPrefix(local, "npub1") {
		pubkey, err := bech32encoding.NpubToBytes([]byte(local))
		if err != nil {
			return "", fmt.Errorf("invalid npub: %w", err)
		}
		return hex.Enc(pubkey), nil
	}

	// Try full 64-char hex pubkey
	if len(local) == 64 {
		_, err := hex.Dec(local)
		if err == nil {
			return local, nil
		}
	}

	// Try alias lookup via ACL
	if aclClient != nil {
		pubkey, err := aclClient.GetPubkeyByAlias(local)
		if err == nil && pubkey != "" {
			return pubkey, nil
		}
	}

	return "", fmt.Errorf("cannot resolve pubkey from local part %q (must be npub, 64-char hex, or alias)", local)
}

func identitySourceString(s IdentitySource) string {
	switch s {
	case IdentityFromConfig:
		return "config"
	case IdentityFromDB:
		return "database"
	case IdentityFromFile:
		return "file"
	default:
		return "unknown"
	}
}
