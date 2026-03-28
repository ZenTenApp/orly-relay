package main

import (
	"context"
	"os"
	"os/signal"

	"next.orly.dev/app"
	"next.orly.dev/pkg/lol/log"
)

func main() {
	dir := "app/smesh3"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	deployPub := os.Getenv("ORLY_DEPLOY_PUBKEY")
	clientTag := os.Getenv("ORLY_CLIENT_TAG")
	if clientTag == "" {
		clientTag = "smesh.lol"
	}
	s := app.NewSmesh3Server(8090, dir, deployPub, clientTag)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := s.Start(ctx); err != nil {
		log.E.F("failed to start: %v", err)
		os.Exit(1)
	}

	bot, err := app.NewBridgeBot(ctx, "wss://relay.orly.dev", true, "")
	if err != nil {
		log.E.F("bridge bot init failed: %v", err)
	} else {
		if err := bot.Start(ctx); err != nil {
			log.E.F("bridge bot start failed: %v", err)
		} else {
			log.I.F("bridge bot pubkey: %s", bot.PubkeyHex())
			defer bot.Stop()
		}
	}

	<-ctx.Done()
	s.Stop()
}
