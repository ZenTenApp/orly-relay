// orly-db is a standalone gRPC database server for the ORLY relay.
// It wraps the Badger database implementation and exposes it via gRPC.
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
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database"
	orlydbv1 "git.smesh.lol/orly/pkg/proto/orlydb/v1"
)

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-db starting with log level: %s", cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create database configuration
	dbCfg := &database.DatabaseConfig{
		DataDir:            cfg.DataDir,
		LogLevel:           cfg.LogLevel,
		BlockCacheMB:       cfg.BlockCacheMB,
		IndexCacheMB:       cfg.IndexCacheMB,
		QueryCacheSizeMB:   cfg.QueryCacheSizeMB,
		QueryCacheMaxAge:   cfg.QueryCacheMaxAge,
		QueryCacheDisabled: cfg.QueryCacheDisabled,
		SerialCachePubkeys: cfg.SerialCachePubkeys,
		SerialCacheEventIds: cfg.SerialCacheEventIds,
		ZSTDLevel:          cfg.ZSTDLevel,
	}

	// Initialize database using existing Badger implementation
	log.I.F("initializing database at %s", cfg.DataDir)
	db, err := database.NewWithConfig(ctx, cancel, dbCfg)
	if chk.E(err) {
		log.E.F("failed to initialize database: %v", err)
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Create gRPC server with large message sizes for events
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(64<<20), // 64MB
		grpc.MaxSendMsgSize(64<<20), // 64MB
	)

	// Register database service
	service := NewDatabaseService(db, cfg)
	orlydbv1.RegisterDatabaseServiceServer(grpcServer, service)

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

		// Cancel context to stop all operations
		cancel()

		// Gracefully stop gRPC server with timeout
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

		// Sync and close database
		log.I.F("syncing database...")
		if err := db.Sync(); chk.E(err) {
			log.W.F("failed to sync database: %v", err)
		}
		log.I.F("closing database...")
		if err := db.Close(); chk.E(err) {
			log.W.F("failed to close database: %v", err)
		}
		log.I.F("shutdown complete")
	}()

	// Serve gRPC
	if err := grpcServer.Serve(lis); err != nil {
		log.E.F("gRPC server error: %v", err)
	}
}
