package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/bridge"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/protocol/marmot"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

// BridgeBot is a server-side marmot client that receives DMs and processes
// subscription commands (status, subscribe, subscribe <alias>).
type BridgeBot struct {
	client  *marmot.Client
	handler *bridge.SubscriptionHandler
	relay   *ws.Client
	adapter *marmot.WSRelayAdapter
	sign    *p8k.Signer
	cancel  context.CancelFunc
	eventLoopDone chan struct{}
}

// NewBridgeBot creates a bridge bot that connects to the given relay.
// If freeMode is true, subscriptions activate without payment.
// dataDir is used to persist the bot's keypair across restarts.
func NewBridgeBot(ctx context.Context, relayURL string, freeMode bool, dataDir string) (*BridgeBot, error) {
	sign, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err := loadOrGenerateKey(sign, dataDir); err != nil {
		return nil, err
	}

	relay, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, err
	}

	adapter := marmot.NewWSRelayAdapter(relay)
	client, err := marmot.NewClient(&marmot.LocalCrypto{Sign: sign}, marmot.NewMemoryGroupStore(), adapter, relayURL)
	if err != nil {
		relay.Close()
		return nil, err
	}

	subStore := bridge.NewMemorySubscriptionStore()
	var payments *bridge.PaymentProcessor
	if !freeMode {
		payments = bridge.NewPaymentProcessorWithClient(newAutoPayNWC(), 1000)
	}

	bot := &BridgeBot{
		client:  client,
		relay:   relay,
		adapter: adapter,
		sign:    sign,
	}

	sendDM := func(pubkeyHex string, content string) error {
		pub, err := hex.Dec(pubkeyHex)
		if err != nil {
			return err
		}
		return client.SendDM(ctx, pub, []byte(content))
	}

	bot.handler = bridge.NewSubscriptionHandler(subStore, payments, sendDM, 1000, nil, 2000, "")
	return bot, nil
}

// Start publishes the bot's key package and begins listening for DMs.
func (b *BridgeBot) Start(ctx context.Context) error {
	ctx, b.cancel = context.WithCancel(ctx)

	if err := b.client.PublishKeyPackage(ctx); err != nil {
		return err
	}
	log.I.F("bridge-bot: key package published, pubkey=%s", b.PubkeyHex())

	// Publish kind-0 metadata so the frontend can display the bot's name.
	metaEv := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      0,
		Tags:      tag.NewS(),
		Content:   []byte(`{"name":"smesh bridge","about":"MLS DM bridge bot"}`),
	}
	if err := metaEv.Sign(b.sign); err != nil {
		log.W.F("bridge-bot: failed to sign kind-0: %v", err)
	} else if err := b.relay.Publish(ctx, metaEv); err != nil {
		log.W.F("bridge-bot: failed to publish kind-0: %v", err)
	}

	b.client.OnDM(func(senderPub []byte, plaintext []byte) {
		senderHex := hex.Enc(senderPub)
		content := strings.TrimSpace(string(plaintext))
		log.I.F("bridge-bot: DM from %s: %s", senderHex[:16], content)

		switch {
		case content == "status":
			b.handler.HandleStatus(senderHex)
		case content == "subscribe":
			b.handler.HandleSubscribe(ctx, senderHex, "")
		case strings.HasPrefix(content, "subscribe "):
			alias := strings.TrimSpace(content[10:])
			b.handler.HandleSubscribe(ctx, senderHex, alias)
		default:
			// Echo for debugging
			sendDM := func(pub string, msg string) error {
				p, _ := hex.Dec(pub)
				return b.client.SendDM(ctx, p, []byte(msg))
			}
			sendDM(senderHex, "echo: "+content)
		}
	})

	b.eventLoopDone = make(chan struct{})
	go b.eventLoop(ctx)
	return nil
}

func (b *BridgeBot) eventLoop(ctx context.Context) {
	defer close(b.eventLoopDone)
	for {
		filters := b.client.SubscriptionFilters()
		stream, err := b.adapter.Subscribe(ctx, filters)
		if err != nil {
			log.W.F("bridge-bot: subscribe failed: %v", err)
			return
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			for ev := range stream.Events() {
				if err := b.client.HandleEvent(ctx, ev); err != nil {
					log.W.F("bridge-bot: handle event: %v", err)
				}
			}
		}()

		select {
		case <-ctx.Done():
			stream.Close()
			return
		case <-b.client.GroupsChanged():
			log.I.F("bridge-bot: groups changed, re-subscribing")
			stream.Close()
			<-done
		}
	}
}

// Stop shuts down the bridge bot.
func (b *BridgeBot) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	if b.eventLoopDone != nil {
		<-b.eventLoopDone
	}
	if b.relay != nil {
		b.relay.Close()
	}
}

// PubkeyHex returns the bot's public key in hex.
func (b *BridgeBot) PubkeyHex() string {
	return hex.Enc(b.sign.Pub())
}

// loadOrGenerateKey loads a persisted keypair from dataDir/bridgebot.nsec,
// or generates a new one and saves it. Supports ORLY_BRIDGE_BOT_NSEC env override.
func loadOrGenerateKey(sign *p8k.Signer, dataDir string) error {
	// Env var override — highest priority.
	if nsecHex := os.Getenv("ORLY_BRIDGE_BOT_NSEC"); nsecHex != "" {
		sec, err := hex.Dec(nsecHex)
		if err != nil || len(sec) != 32 {
			return fmt.Errorf("invalid ORLY_BRIDGE_BOT_NSEC")
		}
		return sign.InitSec(sec)
	}

	// Try loading from file.
	if dataDir != "" {
		nsecPath := filepath.Join(dataDir, "bridgebot.nsec")
		if data, err := os.ReadFile(nsecPath); err == nil {
			sec, err := hex.Dec(strings.TrimSpace(string(data)))
			if err == nil && len(sec) == 32 {
				log.I.F("bridge-bot: loaded key from %s", nsecPath)
				return sign.InitSec(sec)
			}
			log.W.F("bridge-bot: corrupt nsec file %s, generating new key", nsecPath)
		}
	}

	// Generate new key and persist.
	if err := sign.Generate(); err != nil {
		return err
	}
	if dataDir != "" {
		nsecPath := filepath.Join(dataDir, "bridgebot.nsec")
		os.MkdirAll(dataDir, 0700)
		if err := os.WriteFile(nsecPath, []byte(hex.Enc(sign.Sec())), 0600); err != nil {
			log.W.F("bridge-bot: failed to save key: %v", err)
		} else {
			log.I.F("bridge-bot: saved new key to %s", nsecPath)
		}
	}
	return nil
}
