// orly-sync-distributed is a standalone gRPC distributed sync service for ORLY.
// It provides serial-based peer-to-peer synchronization between relay instances.
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
	"next.orly.dev/pkg/sync/distributed"
	distributedv1 "next.orly.dev/pkg/proto/orlysync/distributed/v1"
)

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-sync-distributed starting with log level: %s", cfg.LogLevel)

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
		log.E.F("badger mode not yet implemented for sync-distributed")
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Create sync manager configuration
	syncCfg := &distributed.Config{
		NodeID:        cfg.NodeID,
		RelayURL:      cfg.RelayURL,
		Peers:         cfg.Peers,
		SyncInterval:  cfg.SyncInterval,
		NIP11CacheTTL: cfg.NIP11CacheTTL,
	}

	// The distributed.Manager currently requires *database.D, not the interface.
	// For gRPC mode, we need the manager to be updated to use the database.Database interface.
	// For now, log that this mode requires refactoring.
	if cfg.DBType == "grpc" {
		log.W.F("distributed sync manager needs refactoring to use database.Database interface")
		log.W.F("currently only stub service is available in gRPC mode")
	}

	log.I.F("initializing distributed sync manager with %d peers", len(cfg.Peers))

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16<<20), // 16MB
		grpc.MaxSendMsgSize(16<<20), // 16MB
	)

	// Register distributed sync service
	// Note: The manager needs refactoring to use the interface before this works fully
	// For now, register a stub service that will return appropriate responses
	_ = syncCfg
	_ = distributedv1.DistributedSyncService_ServiceDesc
	// TODO: Once distributed.Manager uses database.Database interface:
	// syncMgr := distributed.NewManager(ctx, db, syncCfg, nil)
	// service := distributedserver.NewService(db, syncMgr)
	// distributedv1.RegisterDistributedSyncServiceServer(grpcServer, service)

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

		// Close database connection
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
