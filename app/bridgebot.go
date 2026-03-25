package app

import (
	"context"
	"strings"
	"sync"

	"next.orly.dev/pkg/bridge"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/nostr/protocol/marmot"
	"next.orly.dev/pkg/nostr/ws"
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
	wg      sync.WaitGroup
}

// NewBridgeBot creates a bridge bot that connects to the given relay.
func NewBridgeBot(ctx context.Context, relayURL string) (*BridgeBot, error) {
	sign, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err := sign.Generate(); err != nil {
		return nil, err
	}

	relay, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		return nil, err
	}

	adapter := marmot.NewWSRelayAdapter(relay)
	client, err := marmot.NewClient(sign, marmot.NewMemoryGroupStore(), adapter, relayURL)
	if err != nil {
		relay.Close()
		return nil, err
	}

	subStore := bridge.NewMemorySubscriptionStore()
	payments := bridge.NewPaymentProcessorWithClient(newAutoPayNWC(), 1000)

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

	bot.handler = bridge.NewSubscriptionHandler(subStore, payments, sendDM, 1000, nil, 2000)
	return bot, nil
}

// Start publishes the bot's key package and begins listening for DMs.
func (b *BridgeBot) Start(ctx context.Context) error {
	ctx, b.cancel = context.WithCancel(ctx)

	if err := b.client.PublishKeyPackage(ctx); err != nil {
		return err
	}
	log.I.F("bridge-bot: key package published, pubkey=%s", b.PubkeyHex())

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

	b.wg.Add(1)
	go b.eventLoop(ctx)
	return nil
}

func (b *BridgeBot) eventLoop(ctx context.Context) {
	defer b.wg.Done()
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
	b.wg.Wait()
	if b.relay != nil {
		b.relay.Close()
	}
}

// PubkeyHex returns the bot's public key in hex.
func (b *BridgeBot) PubkeyHex() string {
	return hex.Enc(b.sign.Pub())
}
