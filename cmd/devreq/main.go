package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

func fatal(m string, a ...any) {
	fmt.Fprintf(os.Stderr, m+"\n", a...)
	os.Exit(1)
}

func main() {
	var ids string
	flag.StringVar(&ids, "ids", "", "query by event id(s) instead of author")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: devreq [-ids <hex>] <relay-url> [<author-pubkey-hex>]\n  NOSTR_SECRET_KEY=recipient\n")
	}
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(1)
	}
	relayURL := args[0]

	skb, err := hex.Dec(os.Getenv("NOSTR_SECRET_KEY"))
	if err != nil {
		fatal("decode secret: %v", err)
	}
	signer, err := keys.SecretBytesToSigner(skb)
	if err != nil {
		fatal("signer: %v", err)
	}
	defer signer.Zero()
	myPubBytes := signer.Pub()
	myPubHex := hex.Enc(myPubBytes)
	fmt.Printf("client pubkey: %s\n", myPubHex)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		fatal("connect: %v", err)
	}
	defer conn.Close()

	time.Sleep(200 * time.Millisecond)
	if err := conn.Auth(ctx, signer); err != nil {
		fmt.Printf("auth: %v\n", err)
	}

	var ff *filter.S
	if ids != "" {
		idBytes, err := hex.Dec(ids)
		if err != nil {
			fatal("decode id: %v", err)
		}
		ff = filter.NewS(&filter.F{Ids: tag.NewFromBytesSlice(idBytes)})
	} else {
		authorBytes, err := hex.Dec(args[1])
		if err != nil {
			fatal("decode author: %v", err)
		}
		ff = filter.NewS(&filter.F{
			Kinds:   kind.NewS(kind.New(4)),
			Authors: tag.NewFromBytesSlice(authorBytes),
			Tags:    tag.NewS(tag.NewFromAny("p", myPubBytes)),
		})
	}

	sub, err := conn.Subscribe(ctx, ff)
	if err != nil {
		fatal("subscribe: %v", err)
	}
	defer sub.Unsub()
	fmt.Printf("subscribed: %s\n", relayURL)

	got := 0
	print := func(ev *event.E) {
		got++
		var exp string
		if t := ev.Tags.GetFirst([]byte("expiration")); t != nil {
			exp = string(t.Value())
		}
		fmt.Printf("  -> RECEIVED event id=%s kind=%d author=%s expiration=%s\n",
			hex.Enc(ev.ID), ev.Kind, hex.Enc(ev.Pubkey), exp)
	}
	for {
		select {
		case ev, ok := <-sub.Events:
			if !ok {
				fmt.Printf("subscription closed (events seen: %d)\n", got)
				return
			}
			print(ev)
		case <-time.After(10 * time.Second):
			fmt.Printf("waiting window over; events seen: %d\n", got)
			return
		}
	}
}
