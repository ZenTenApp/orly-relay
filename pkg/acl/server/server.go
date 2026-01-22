package server

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	orlyaclv1 "next.orly.dev/pkg/proto/orlyacl/v1"
)

// Server wraps a gRPC ACL server.
type Server struct {
	grpcServer *grpc.Server
	db         database.Database
	cfg        *Config
	listener   net.Listener
	ownsDB     bool // Whether we own the database and should close it
}

// New creates a new ACL gRPC server.
func New(db database.Database, cfg *Config, ownsDB bool) *Server {
	// Create gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(16<<20), // 16MB
		grpc.MaxSendMsgSize(16<<20), // 16MB
	)

	// Register ACL service
	service := NewACLService(cfg, db)
	orlyaclv1.RegisterACLServiceServer(grpcServer, service)

	// Register reflection for debugging with grpcurl
	reflection.Register(grpcServer)

	return &Server{
		grpcServer: grpcServer,
		db:         db,
		cfg:        cfg,
		ownsDB:     ownsDB,
	}
}

// ConfigureACL sets up the ACL mode and configures the registry.
func (s *Server) ConfigureACL(ctx context.Context) error {
	// Create app config for ACL configuration
	appCfg := &config.C{
		Owners:                  s.cfg.Owners,
		Admins:                  s.cfg.Admins,
		BootstrapRelays:         s.cfg.BootstrapRelays,
		RelayAddresses:          s.cfg.RelayAddresses,
		FollowListFrequency:     s.cfg.FollowListFrequency,
		FollowsThrottleEnabled:  s.cfg.FollowsThrottleEnabled,
		FollowsThrottlePerEvent: s.cfg.FollowsThrottlePerEvent,
		FollowsThrottleMaxDelay: s.cfg.FollowsThrottleMaxDelay,
	}

	// Set ACL mode and configure the registry
	acl.Registry.SetMode(s.cfg.ACLMode)
	if err := acl.Registry.Configure(appCfg, s.db, ctx); chk.E(err) {
		return err
	}

	// Start the syncer goroutine for background operations
	acl.Registry.Syncer()
	log.I.F("ACL syncer started for mode: %s", s.cfg.ACLMode)

	return nil
}

// ListenAndServe starts the gRPC server.
func (s *Server) ListenAndServe(ctx context.Context, cancel context.CancelFunc) error {
	// Start listening
	lis, err := net.Listen("tcp", s.cfg.Listen)
	if chk.E(err) {
		return err
	}
	s.listener = lis
	log.I.F("gRPC ACL server listening on %s", s.cfg.Listen)

	// Handle graceful shutdown
	go s.handleShutdown(ctx, cancel)

	// Serve gRPC
	return s.grpcServer.Serve(lis)
}

func (s *Server) handleShutdown(ctx context.Context, cancel context.CancelFunc) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-sigs:
		log.I.F("received signal %v, shutting down...", sig)
	case <-ctx.Done():
		log.I.F("context cancelled, shutting down...")
	}

	// Cancel context to stop all operations
	cancel()

	// Gracefully stop gRPC server with timeout
	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.I.F("gRPC server stopped gracefully")
	case <-time.After(5 * time.Second):
		log.W.F("gRPC graceful stop timed out, forcing stop")
		s.grpcServer.Stop()
	}

	// Sync and close database if we own it
	if s.ownsDB {
		log.I.F("syncing database...")
		if err := s.db.Sync(); chk.E(err) {
			log.W.F("failed to sync database: %v", err)
		}
		log.I.F("closing database...")
		if err := s.db.Close(); chk.E(err) {
			log.W.F("failed to close database: %v", err)
		}
	}
	log.I.F("shutdown complete")
}

// Stop stops the server.
func (s *Server) Stop() {
	s.grpcServer.Stop()
}
