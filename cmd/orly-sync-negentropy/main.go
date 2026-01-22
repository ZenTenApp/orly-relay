// orly-sync-negentropy is a standalone gRPC negentropy sync service for ORLY.
// It provides NIP-77 negentropy-based set reconciliation for both
// relay-to-relay sync and client-facing WebSocket operations.
package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"next.orly.dev/pkg/database"
	databasegrpc "next.orly.dev/pkg/database/grpc"
	negentropyv1 "next.orly.dev/pkg/proto/orlysync/negentropy/v1"
	"next.orly.dev/pkg/sync/negentropy"
	negentropyserver "next.orly.dev/pkg/sync/negentropy/server"
)

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-sync-negentropy starting with log level: %s", cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database connection
	var db database.Database
	var dbCloser func()

	if cfg.DBType == "grpc" {
		log.I.F("connecting to gRPC database server at %s", cfg.GRPCDBServer)
		dbClient, err := databasegrpc.New(ctx, &databasegrpc.ClientConfig{
			ServerAddress:  cfg.GRPCDBServer,
			ConnectTimeout: 30 * time.Second,
		})
		if chk.E(err) {
			log.E.F("failed to connect to database server: %v", err)
			os.Exit(1)
		}
		db = dbClient
		dbCloser = func() { dbClient.Close() }
	} else {
		log.E.F("badger mode not yet implemented for sync-negentropy")
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	log.I.F("initializing negentropy sync manager with %d peers", len(cfg.Peers))

	// Create negentropy manager
	mgrConfig := &negentropy.Config{
		Peers:                cfg.Peers,
		SyncInterval:         cfg.SyncInterval,
		FrameSize:            cfg.FrameSize,
		IDSize:               cfg.IDSize,
		ClientSessionTimeout: cfg.ClientSessionTimeout,
	}
	negentropyMgr := negentropy.NewManager(db, mgrConfig)

	// Start background sync loop if we have peers
	if len(cfg.Peers) > 0 {
		log.I.F("starting background sync with %d peers", len(cfg.Peers))
		negentropyMgr.Start()
	}

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16<<20), // 16MB
		grpc.MaxSendMsgSize(16<<20), // 16MB
	)

	// Register negentropy service
	service := negentropyserver.NewService(db, negentropyMgr)
	negentropyv1.RegisterNegentropyServiceServer(grpcServer, service)

	// Register reflection for debugging with grpcurl
	reflection.Register(grpcServer)

	// Start listening
	lis, err := net.Listen("tcp", cfg.Listen)
	if chk.E(err) {
		log.E.F("failed to listen on %s: %v", cfg.Listen, err)
		os.Exit(1)
	}
	log.I.F("gRPC server listening on %s", cfg.Listen)

	// Handle graceful shutdown
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
		sig := <-sigs
		log.I.F("received signal %v, shutting down...", sig)

		cancel()

		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()

		select {
		case <-stopped:
			log.I.F("gRPC server stopped gracefully")
		case <-time.After(5 * time.Second):
			log.W.F("gRPC graceful stop timed out, forcing stop")
			grpcServer.Stop()
		}

		// Stop the negentropy manager
		log.I.F("stopping negentropy manager...")
		negentropyMgr.Stop()

		if dbCloser != nil {
			log.I.F("closing database connection...")
			dbCloser()
		}
		log.I.F("shutdown complete")
	}()

	// Serve gRPC
	if err := grpcServer.Serve(lis); err != nil {
		log.E.F("gRPC server error: %v", err)
	}
}
