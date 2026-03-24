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
	s := app.NewSmesh3Server(8090, dir, "/tmp/sm3sh-data", deployPub)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := s.Start(ctx); err != nil {
		log.E.F("failed to start: %v", err)
		os.Exit(1)
	}

	<-ctx.Done()
	s.Stop()
}
