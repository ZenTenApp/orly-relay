package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"git.mleku.dev/mleku/nostr/crypto/encryption"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	bridgesmtp "next.orly.dev/pkg/bridge/smtp"
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
		cfg:      cfg,
		dbGetter: dbGetter,
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

	subHandler := NewSubscriptionHandler(subStore, payments, sendDM, b.cfg.MonthlyPriceSats)
	outbound := NewOutboundProcessor(smtpClient, rateLimiter, subHandler, b.cfg.Domain, sendDM)
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
			pubkeyHex, err := resolveRecipientPubkey(to, b.cfg.Domain)
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
func (b *Bridge) subscribeAndProcess() error {
	bridgePubHex := hex.Enc(b.sign.Pub())

	ff := filter.NewS(
		&filter.F{
			Kinds: kind.NewS(kind.New(4)), // kind 4 = encrypted DM
			Tags: tag.NewS(
				tag.NewFromAny("p", bridgePubHex),
			),
			Since: &timestamp.T{V: time.Now().Unix()},
		},
	)

	sub, err := b.relay.Subscribe(b.ctx, ff)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	defer sub.Close()

	log.I.F("subscribed to DMs for bridge pubkey %s", bridgePubHex)

	for {
		select {
		case <-b.ctx.Done():
			return nil
		case ev, ok := <-sub.Events():
			if !ok {
				return fmt.Errorf("event channel closed")
			}
			b.handleDMEvent(ev)
		}
	}
}

// handleDMEvent decrypts a kind 4 DM event and routes it.
func (b *Bridge) handleDMEvent(ev *event.E) {
	senderPubHex := hex.Enc(ev.Pubkey[:])

	// Derive conversation key for NIP-44 decryption
	conversationKey, err := encryption.GenerateConversationKey(
		b.sign.Sec(), ev.Pubkey[:],
	)
	if err != nil {
		log.E.F("ECDH failed for sender %s: %v", senderPubHex, err)
		return
	}

	decrypted, err := encryption.Decrypt(conversationKey, string(ev.Content))
	if err != nil {
		log.W.F("DM decryption failed from %s: %v", senderPubHex, err)
		return
	}

	log.D.F("received DM from %s: %d bytes", senderPubHex, len(decrypted))

	if b.router != nil {
		b.router.RouteDM(b.ctx, senderPubHex, decrypted)
	}
}

// makeSendDM returns a callback that encrypts and publishes a kind 4 DM
// to the given pubkey via the relay.
func (b *Bridge) makeSendDM() func(pubkeyHex string, content string) error {
	return func(pubkeyHex string, content string) error {
		if b.relay == nil {
			return fmt.Errorf("no relay connection")
		}

		recipientPub, err := hex.Dec(pubkeyHex)
		if err != nil {
			return fmt.Errorf("decode recipient pubkey: %w", err)
		}

		// Derive conversation key
		conversationKey, err := encryption.GenerateConversationKey(
			b.sign.Sec(), recipientPub,
		)
		if err != nil {
			return fmt.Errorf("ECDH: %w", err)
		}

		// Encrypt content with NIP-44
		encrypted, err := encryption.Encrypt(conversationKey, []byte(content), nil)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}

		// Build kind 4 DM event
		ev := &event.E{
			Content:   []byte(encrypted),
			CreatedAt: time.Now().Unix(),
			Kind:      4,
			Tags: tag.NewS(
				tag.NewFromAny("p", pubkeyHex),
			),
		}

		if err := ev.Sign(b.sign); chk.E(err) {
			return fmt.Errorf("sign DM: %w", err)
		}

		if err := b.relay.Publish(b.ctx, ev); err != nil {
			return fmt.Errorf("publish DM: %w", err)
		}

		log.D.F("sent DM to %s (%d bytes)", pubkeyHex, len(content))
		return nil
	}
}

// resolveRecipientPubkey extracts the Nostr pubkey hex from an email address
// local part. The local part can be an npub (bech32) or a hex pubkey prefix.
func resolveRecipientPubkey(emailAddr, domain string) (string, error) {
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

	// Assume hex (may be truncated — we can only deliver if it's a full 64-char hex pubkey)
	if len(local) == 64 {
		_, err := hex.Dec(local)
		if err != nil {
			return "", fmt.Errorf("invalid hex pubkey: %w", err)
		}
		return local, nil
	}

	return "", fmt.Errorf("cannot resolve pubkey from local part %q (must be npub or 64-char hex)", local)
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
