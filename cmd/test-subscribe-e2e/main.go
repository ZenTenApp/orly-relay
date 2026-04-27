// test-subscribe-e2e exercises the full Marmot bridge subscribe flow:
// send "subscribe" DM → receive invoice → pay via NWC → receive confirmation.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/protocol/nwc"
)

func main() {
	relayURL := envOr("RELAY_URL", "wss://relay.orly.dev")
	bridgePubHex := envOr("BRIDGE_PUBKEY", "cf1ae33ad5f229dabd7d733ce37b0165126aebf581e4094df9373f77e00cb696")
	nwcURI := os.Getenv("ORLY_NWC_URI")

	if nwcURI == "" {
		fatal("ORLY_NWC_URI must be set")
	}

	// Generate or load keypair
	var secretBytes []byte
	var err error
	if sk := os.Getenv("NOSTR_SECRET_KEY"); sk != "" {
		secretBytes, err = hex.Dec(sk)
		if err != nil {
			fatal("decode secret key: %v", err)
		}
	} else {
		secretBytes, err = keys.GenerateSecretKey()
		if err != nil {
			fatal("generate key: %v", err)
		}
	}
	signer, err := keys.SecretBytesToSigner(secretBytes)
	if err != nil {
		fatal("create signer: %v", err)
	}
	defer signer.Zero()

	myPubHex := hex.Enc(signer.Pub())
	fmt.Printf("client pubkey: %s\n", myPubHex)

	// Derive conversation key with bridge for decryption
	bridgePub, err := hex.Dec(bridgePubHex)
	if err != nil {
		fatal("decode bridge pubkey: %v", err)
	}
	convKey, err := encryption.GenerateConversationKey(signer.Sec(), bridgePub)
	if err != nil {
		fatal("conversation key: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Connect to relay
	fmt.Printf("connecting to %s...\n", relayURL)
	conn, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		fatal("relay connect: %v", err)
	}
	defer conn.Close()

	// Subscribe for DMs from the bridge to us
	since := time.Now().Unix() - 5
	sub, err := conn.Subscribe(ctx, filter.NewS(
		&filter.F{
			Kinds:   kind.NewS(kind.New(4)),
			Authors: tag.NewFromAny(bridgePub),
			Tags:    tag.NewS(tag.NewFromAny("p", myPubHex)),
			Since:   &timestamp.T{V: since},
		},
	))
	if err != nil {
		fatal("subscribe: %v", err)
	}
	defer sub.Unsub()
	fmt.Println("subscribed for bridge DMs")

	// Send "subscribe" DM to bridge
	fmt.Println("sending 'subscribe' DM...")
	plaintext := []byte("subscribe")
	ciphertext, err := encryption.Encrypt(convKey, plaintext, nil)
	if err != nil {
		fatal("encrypt: %v", err)
	}

	ev := &event.E{
		Content:   []byte(ciphertext),
		CreatedAt: time.Now().Unix(),
		Kind:      4,
		Tags:      tag.NewS(tag.NewFromAny("p", bridgePubHex)),
	}
	if err := ev.Sign(signer); err != nil {
		fatal("sign: %v", err)
	}
	if err := conn.Publish(ctx, ev); err != nil {
		fatal("publish: %v", err)
	}
	fmt.Printf("subscribe DM published (event %s)\n", hex.Enc(ev.ID))

	// Wait for invoice DM from bridge
	fmt.Println("waiting for invoice DM...")
	bolt11 := ""
	for bolt11 == "" {
		select {
		case <-ctx.Done():
			fatal("timeout waiting for invoice DM")
		case reason := <-sub.ClosedReason:
			fatal("subscription closed: %s", reason)
		case e := <-sub.Events:
			if e == nil {
				fatal("subscription channel closed")
			}
			decrypted, err := encryption.Decrypt(convKey, string(e.Content))
			if err != nil {
				fmt.Printf("  (could not decrypt DM: %v)\n", err)
				continue
			}
			fmt.Printf("  bridge DM: %s\n", truncate(decrypted, 200))

			// Extract bolt11 from the DM text
			bolt11 = extractBolt11(decrypted)
			if bolt11 == "" {
				fmt.Println("  (no bolt11 found in this DM, waiting for more...)")
			}
		}
	}

	fmt.Printf("extracted bolt11: %s...\n", truncate(bolt11, 40))

	// Pay the invoice via NWC
	fmt.Println("paying invoice via NWC...")
	nwcClient, err := nwc.NewClient(nwcURI)
	if err != nil {
		fatal("NWC client: %v", err)
	}

	var payResult map[string]any
	err = nwcClient.Request(ctx, "pay_invoice", map[string]any{
		"invoice": bolt11,
	}, &payResult)
	if err != nil {
		fatal("pay_invoice: %v", err)
	}
	fmt.Printf("payment sent: %v\n", payResult)

	// Wait for confirmation DM from bridge
	fmt.Println("waiting for confirmation DM...")
	confirmed := false
	deadline := time.After(2 * time.Minute)
	for !confirmed {
		select {
		case <-deadline:
			fmt.Println("WARNING: timeout waiting for confirmation DM (payment may still be processing)")
			confirmed = true // exit loop
		case <-ctx.Done():
			fatal("context cancelled waiting for confirmation")
		case reason := <-sub.ClosedReason:
			fatal("subscription closed: %s", reason)
		case e := <-sub.Events:
			if e == nil {
				fatal("subscription channel closed")
			}
			decrypted, err := encryption.Decrypt(convKey, string(e.Content))
			if err != nil {
				fmt.Printf("  (could not decrypt DM: %v)\n", err)
				continue
			}
			fmt.Printf("  bridge DM: %s\n", truncate(decrypted, 300))
			if strings.Contains(decrypted, "active") || strings.Contains(decrypted, "Payment received") {
				confirmed = true
				fmt.Println("\n=== PASS: Full subscribe flow completed ===")
			}
		}
	}
}

func extractBolt11(text string) string {
	// Look for lnbc... string (Lightning invoice)
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "lnbc")
	if idx < 0 {
		return ""
	}
	// Extract the bolt11 string (ends at whitespace or end of string)
	rest := text[idx:]
	end := strings.IndexAny(rest, " \n\r\t")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatal(format string, args ...any) {
	log.F.F(format, args...)
}
