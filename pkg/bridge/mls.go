package bridge

import (
	"context"
	"fmt"
	"path/filepath"

	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/protocol/marmot"
)

// relayAdapter adapts *RelayConn to the marmot.RelayConnection interface.
type relayAdapter struct {
	relay *RelayConn
}

func (a *relayAdapter) Publish(ctx context.Context, ev *event.E) error {
	return a.relay.Publish(ctx, ev)
}

func (a *relayAdapter) Subscribe(ctx context.Context, ff *filter.S) (marmot.EventStream, error) {
	sub, err := a.relay.Subscribe(ctx, ff)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// initMLS initializes the MLS client for the bridge. Called during Start()
// when MLSEnabled is true.
func (b *Bridge) initMLS() error {
	groupDir := filepath.Join(b.cfg.DataDir, "mls-groups")
	store, err := marmot.NewFileGroupStore(groupDir)
	if err != nil {
		return fmt.Errorf("create MLS group store: %w", err)
	}

	relayURL := b.cfg.PublicRelayURL
	if relayURL == "" {
		relayURL = b.cfg.RelayURL
	}

	adapter := &relayAdapter{relay: b.relay}
	client, err := marmot.NewClient(b.sign, store, adapter, relayURL)
	if err != nil {
		return fmt.Errorf("create MLS client: %w", err)
	}

	client.OnDM(b.handleMLSDM)
	b.mlsClient = client

	// Publish our key package so MLS peers can find us
	if err := client.PublishKeyPackage(b.ctx); err != nil {
		log.W.F("publish MLS key package: %v", err)
	} else {
		log.I.F("published MLS key package (kind 443)")
	}

	// Publish key package relay list
	if relayURL != "" {
		if err := client.PublishKeyPackageRelays(b.ctx, []string{relayURL}); err != nil {
			log.W.F("publish MLS key package relays: %v", err)
		} else {
			log.I.F("published MLS key package relay list (kind 10051)")
		}
	}

	// Start MLS subscription loop in background
	b.wg.Add(1)
	go b.mlsWatchLoop()

	return nil
}

// handleMLSDM is the callback for incoming decrypted MLS DMs.
func (b *Bridge) handleMLSDM(senderPub []byte, plaintext []byte) {
	senderPubHex := hex.Enc(senderPub)

	bridgePubHex := hex.Enc(b.sign.Pub())
	if senderPubHex == bridgePubHex {
		return
	}

	log.I.F("received MLS DM from %s: %d bytes", senderPubHex, len(plaintext))

	b.recordSenderFormat(senderPubHex, dmFormatMLS)

	if b.router != nil {
		b.router.RouteDM(b.ctx, senderPubHex, string(plaintext))
	}
}

// sendMLSDM sends an MLS-encrypted DM to the given recipient.
func (b *Bridge) sendMLSDM(pubkeyHex, content string) error {
	if b.mlsClient == nil {
		return fmt.Errorf("MLS client not initialized")
	}

	recipientPub, err := hex.Dec(pubkeyHex)
	if err != nil {
		return fmt.Errorf("decode recipient pubkey: %w", err)
	}

	return b.mlsClient.SendDM(b.ctx, recipientPub, []byte(content))
}

// mlsKeyPackageEvent returns a signed kind 443 event for broadcasting.
func (b *Bridge) mlsKeyPackageEvent() (*event.E, error) {
	return b.mlsClient.KeyPackageEvent()
}

// mlsKeyPackageRelaysEvent returns a signed kind 10051 event for broadcasting.
func (b *Bridge) mlsKeyPackageRelaysEvent(relayURL string) (*event.E, error) {
	return b.mlsClient.KeyPackageRelaysEvent([]string{relayURL})
}

// mlsWatchLoop subscribes to MLS events (kind 1059 welcomes + kind 445 group
// messages) and dispatches them to the MLS client. Reconnects with updated
// filters as new groups are established.
func (b *Bridge) mlsWatchLoop() {
	defer b.wg.Done()

	for {
		if err := b.mlsSubscribeAndProcess(); err != nil {
			if b.ctx.Err() != nil {
				return
			}
			log.W.F("MLS subscription error: %v", err)
		}
	}
}

func (b *Bridge) mlsSubscribeAndProcess() error {
	ff := b.mlsClient.SubscriptionFilters()

	sub, err := b.relay.Subscribe(b.ctx, ff)
	if err != nil {
		return fmt.Errorf("MLS subscribe: %w", err)
	}
	defer sub.Close()

	groupIDs := b.mlsClient.ActiveGroupIDs()
	log.I.F("MLS subscribed (welcomes + %d active groups)", len(groupIDs))

	for {
		select {
		case <-b.ctx.Done():
			return nil
		case <-b.mlsClient.GroupsChanged():
			log.I.F("MLS group added, refreshing subscription filters")
			return nil // causes mlsWatchLoop to re-subscribe with updated filters
		case ev, ok := <-sub.Events():
			if !ok {
				return fmt.Errorf("MLS event channel closed")
			}
			if err := b.mlsClient.HandleEvent(b.ctx, ev); err != nil {
				log.W.F("MLS event handling: %v", err)
			}
		}
	}
}
