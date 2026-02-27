// orly-acl is a standalone gRPC ACL server for the ORLY relay.
// It wraps the ACL implementations and exposes them via gRPC.
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
	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	databasegrpc "next.orly.dev/pkg/database/grpc"
	orlyaclv1 "next.orly.dev/pkg/proto/orlyacl/v1"
)

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-acl starting with log level: %s, mode: %s", cfg.LogLevel, cfg.ACLMode)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize database (direct Badger or gRPC client)
	var db database.Database
	var err error

	if cfg.DBType == "grpc" {
		// Use gRPC database client
		log.I.F("connecting to gRPC database server at %s", cfg.GRPCDBServer)
		dbClient, err := databasegrpc.New(ctx, &databasegrpc.ClientConfig{
			ServerAddress:  cfg.GRPCDBServer,
			ConnectTimeout: 30 * time.Second,
		})
		if chk.E(err) {
			log.E.F("failed to connect to gRPC database: %v", err)
			os.Exit(1)
		}
		db = dbClient
	} else {
		// Use direct Badger database
		dbCfg := &database.DatabaseConfig{
			DataDir:             cfg.DataDir,
			LogLevel:            cfg.LogLevel,
			BlockCacheMB:        cfg.BlockCacheMB,
			IndexCacheMB:        cfg.IndexCacheMB,
			QueryCacheSizeMB:    cfg.QueryCacheSizeMB,
			QueryCacheMaxAge:    cfg.QueryCacheMaxAge,
			QueryCacheDisabled:  cfg.QueryCacheDisabled,
			SerialCachePubkeys:  cfg.SerialCachePubkeys,
			SerialCacheEventIds: cfg.SerialCacheEventIds,
			ZSTDLevel:           cfg.ZSTDLevel,
		}

		log.I.F("initializing Badger database at %s", cfg.DataDir)
		db, err = database.NewWithConfig(ctx, cancel, dbCfg)
		if chk.E(err) {
			log.E.F("failed to initialize database: %v", err)
			os.Exit(1)
		}
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Create gRPC server EARLY so launcher can detect it's alive
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16<<20), // 16MB
		grpc.MaxSendMsgSize(16<<20), // 16MB
	)

	// Register ACL service (will be configured below)
	service := NewACLService(cfg, db)
	orlyaclv1.RegisterACLServiceServer(grpcServer, service)

	// Register reflection for debugging with grpcurl
	reflection.Register(grpcServer)

	// Start listening immediately so launcher knows we're alive
	lis, err := net.Listen("tcp", cfg.Listen)
	if chk.E(err) {
		log.E.F("failed to listen on %s: %v", cfg.Listen, err)
		os.Exit(1)
	}
	log.I.F("gRPC ACL server listening on %s", cfg.Listen)

	// Start serving in background while we configure
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.E.F("gRPC server error: %v", err)
		}
	}()

	// Create app config for ACL configuration
	appCfg := &config.C{
		Owners:                  cfg.GetOwners(),
		Admins:                  cfg.GetAdmins(),
		BootstrapRelays:         cfg.GetBootstrapRelays(),
		RelayAddresses:          cfg.GetRelayAddresses(),
		FollowListFrequency:     cfg.FollowListFrequency,
		FollowsThrottleEnabled:  cfg.FollowsThrottleEnabled,
		FollowsThrottlePerEvent: cfg.FollowsThrottlePerEvent,
		FollowsThrottleMaxDelay: cfg.FollowsThrottleMaxDelay,
	}

	// Set ACL mode first
	acl.Registry.SetMode(cfg.ACLMode)

	// Mark service as ready IMMEDIATELY so relay can start
	// Configure runs in background and loads follow lists asynchronously
	service.SetReady(true)
	log.I.F("ACL service ready (follow lists loading in background)")

	// Run Configure in background (may take time to load follow lists)
	go func() {
		if err := acl.Registry.Configure(appCfg, db, ctx); chk.E(err) {
			log.E.F("failed to configure ACL: %v", err)
			// Don't exit - service can still function with limited ACL
		}
		log.I.F("ACL configuration complete")
		// Start the syncer goroutine for background operations
		acl.Registry.Syncer()
		log.I.F("ACL syncer started for mode: %s", cfg.ACLMode)
	}()

	// Handle graceful shutdown - block until signal received
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

	// Sync and close database (only for direct Badger)
	if cfg.DBType != "grpc" {
		log.I.F("syncing database...")
		if err := db.Sync(); chk.E(err) {
			log.W.F("failed to sync database: %v", err)
		}
		log.I.F("closing database...")
		if err := db.Close(); chk.E(err) {
			log.W.F("failed to close database: %v", err)
		}
	}
	log.I.F("shutdown complete")
}
