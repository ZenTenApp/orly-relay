package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
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
	expSec := flag.Int("expiration", 0, "expire the DM N seconds from now (adds NIP-40 expiration tag)")
	clientName := flag.String("client", "send-dm", "value for the required client tag")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: send-dm [flags] <relay-url> <recipient-pubkey-hex> <message>\n")
		fmt.Fprintf(os.Stderr, "  set NOSTR_SECRET_KEY=<hex> to use a specific key (else a throwaway key is generated)\n")
		fmt.Fprintf(os.Stderr, "flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) < 3 {
		flag.Usage()
		os.Exit(1)
	}
	relayURL := args[0]
	recipientHex := args[1]
	message := args[2]

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

	// Build kind 4 DM. Policy on the test relay requires p, expiration, and client.
	if *clientName == "" {
		log.F.F("client tag value must not be empty")
	}
	tagList := []*tag.T{
		tag.NewFromAny("p", recipientHex),
		tag.NewFromAny("client", *clientName),
	}
	if *expSec > 0 {
		expTS := time.Now().Unix() + int64(*expSec)
		tagList = append(tagList, tag.NewFromAny("expiration", strconv.FormatInt(expTS, 10)))
		fmt.Printf("expiration: %s (in %d seconds)\n", time.Unix(expTS, 0).UTC().Format(time.RFC3339), *expSec)
	}
	fmt.Printf("client: %s\n", *clientName)
	tags := tag.NewS(tagList...)
	ev := &event.E{
		Content:   []byte(ciphertext),
		CreatedAt: time.Now().Unix(),
		Kind:      4,
		Tags:      tags,
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

	// NIP-42: wait briefly for the relay's AUTH challenge, then authenticate
	// so writes are accepted when the relay enforces auth for publish.
	time.Sleep(200 * time.Millisecond)
	authErr := conn.Auth(ctx, signer)
	if authErr != nil {
		log.F.F("auth: %v", authErr)
	}
	fmt.Println("authenticated to relay")

	if err := conn.Publish(ctx, ev); err != nil {
		log.F.F("publish: %v", err)
	}

	fmt.Println("DM sent successfully")
}
