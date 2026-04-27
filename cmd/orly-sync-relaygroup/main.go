// orly-sync-relaygroup is a standalone gRPC relay group service for ORLY.
// It provides relay group configuration discovery from Kind 39105 events.
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
	databasegrpc "git.smesh.lol/orly/pkg/database/grpc"
	"git.smesh.lol/orly/pkg/sync/relaygroup"
	relaygroupv1 "git.smesh.lol/orly/pkg/proto/orlysync/relaygroup/v1"
)

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-sync-relaygroup starting with log level: %s", cfg.LogLevel)

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
		log.E.F("badger mode not yet implemented for sync-relaygroup")
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Create relay group manager configuration
	relaygroupCfg := &relaygroup.ManagerConfig{
		AdminNpubs: cfg.AdminNpubs,
	}

	log.I.F("initializing relay group manager with %d admins", len(cfg.AdminNpubs))

	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16<<20), // 16MB
		grpc.MaxSendMsgSize(16<<20), // 16MB
	)

	// Register relay group service
	// TODO: Create and register service once server implementation is done
	// service := relaygroup.NewService(relaygroupMgr, cfg)
	// relaygroupv1.RegisterRelayGroupServiceServer(grpcServer, service)
	_ = relaygroupCfg
	_ = db
	_ = relaygroupv1.RelayGroupService_ServiceDesc

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
