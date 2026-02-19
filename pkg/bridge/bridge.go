package bridge

import (
	"context"
	"fmt"
	"os"
	"sync"

	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
	"lol.mleku.dev/log"
)

// Bridge is the Nostr-Email bridge. It manages identity, relay connection,
// Marmot DM handling, and SMTP transport.
type Bridge struct {
	cfg    *Config
	sign   signer.I
	source IdentitySource
	relay  *RelayConn

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

	// Connect to relay (standalone mode)
	if b.cfg.RelayURL != "" {
		b.relay = NewRelayConn(b.cfg.RelayURL)
		if err := b.relay.Connect(b.ctx); err != nil {
			return fmt.Errorf("relay connection: %w", err)
		}

		// Start reconnect loop in background
		b.wg.Add(1)
		go b.relayWatchLoop()
	}

	log.I.F("bridge started")
	return nil
}

// Stop gracefully shuts down the bridge.
func (b *Bridge) Stop() {
	log.I.F("bridge stopping")
	if b.cancel != nil {
		b.cancel()
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

// relayWatchLoop monitors the relay connection and reconnects if needed.
func (b *Bridge) relayWatchLoop() {
	defer b.wg.Done()

	// For now this is a placeholder. In later phases, this will run the
	// subscription loop that feeds events to the Marmot client and
	// routes incoming DMs to the appropriate handler.
	<-b.ctx.Done()
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
