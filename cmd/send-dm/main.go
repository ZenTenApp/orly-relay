package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/log"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "usage: send-dm <relay-url> <recipient-pubkey-hex> <message>\n")
		fmt.Fprintf(os.Stderr, "  set NOSTR_SECRET_KEY=<hex> to use a specific key\n")
		os.Exit(1)
	}
	relayURL := os.Args[1]
	recipientHex := os.Args[2]
	message := os.Args[3]

	var secretBytes []byte
	var err error
	if sk := os.Getenv("NOSTR_SECRET_KEY"); sk != "" {
		secretBytes, err = hex.Dec(sk)
		if err != nil {
			log.F.F("decode secret key: %v", err)
		}
	} else {
		secretBytes, err = keys.GenerateSecretKey()
		if err != nil {
			log.F.F("generate key: %v", err)
		}
	}
	signer, err := keys.SecretBytesToSigner(secretBytes)
	if err != nil {
		log.F.F("create signer: %v", err)
	}
	defer signer.Zero()

	pubHex := hex.Enc(signer.Pub())
	fmt.Printf("sender pubkey: %s\n", pubHex)

	// Encrypt content with NIP-44
	recipientPub, err := hex.Dec(recipientHex)
	if err != nil {
		log.F.F("decode recipient: %v", err)
	}
	convKey, err := encryption.GenerateConversationKey(signer.Sec(), recipientPub)
	if err != nil {
		log.F.F("conversation key: %v", err)
	}
	ciphertext, err := encryption.Encrypt(convKey, []byte(message), nil)
	if err != nil {
		log.F.F("encrypt: %v", err)
	}

	// Build kind 4 DM
	ev := &event.E{
		Content:   []byte(ciphertext),
		CreatedAt: time.Now().Unix(),
		Kind:      4,
		Tags:      tag.NewS(tag.NewFromAny("p", recipientHex)),
	}
	if err := ev.Sign(signer); err != nil {
		log.F.F("sign: %v", err)
	}

	fmt.Printf("event id: %s\n", hex.Enc(ev.ID))

	// Connect and publish
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		log.F.F("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Publish(ctx, ev); err != nil {
		log.F.F("publish: %v", err)
	}

	fmt.Println("DM sent successfully")
}
