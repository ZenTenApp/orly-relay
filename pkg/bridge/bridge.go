package bridge

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/lol/log"

	aclgrpc "git.smesh.lol/orly/pkg/acl/grpc"
	bridgesmtp "git.smesh.lol/orly/pkg/bridge/smtp"
	"git.smesh.lol/orly/pkg/nostr/protocol/marmot"
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

		// Initialize MLS (mandatory — the only DM protocol)
		if err := b.initMLS(); err != nil {
			return fmt.Errorf("MLS init: %w", err)
		}
		log.I.F("MLS (marmot) enabled")
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
		MXPort:        b.cfg.SMTPMXPort,
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

	subHandler := NewSubscriptionHandler(subStore, payments, sendDM, b.cfg.MonthlyPriceSats, b.aclClient, aliasPriceSats, b.cfg.Domain)
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

// makeSendDM returns a callback that sends MLS-encrypted DMs via marmot.
func (b *Bridge) makeSendDM() func(pubkeyHex string, content string) error {
	return func(pubkeyHex string, content string) error {
		if b.relay == nil {
			return fmt.Errorf("no relay connection")
		}
		log.I.F("sending MLS DM to %s (%d bytes)", pubkeyHex, len(content))
		if err := b.sendMLSDM(pubkeyHex, content); err != nil {
			return fmt.Errorf("MLS DM: %w", err)
		}
		log.I.F("sent MLS DM to %s (%d bytes)", pubkeyHex, len(content))
		return nil
	}
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
