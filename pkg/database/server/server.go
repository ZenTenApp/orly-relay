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
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database"
	orlydbv1 "git.smesh.lol/orly/pkg/proto/orlydb/v1"
)

// Server wraps a gRPC database server.
type Server struct {
	grpcServer *grpc.Server
	db         database.Database
	cfg        *Config
	listener   net.Listener
}

// New creates a new database gRPC server.
func New(db database.Database, cfg *Config) *Server {
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

	return &Server{
		grpcServer: grpcServer,
		db:         db,
		cfg:        cfg,
	}
}

// ListenAndServe starts the gRPC server.
func (s *Server) ListenAndServe(ctx context.Context, cancel context.CancelFunc) error {
	// Start listening
	lis, err := net.Listen("tcp", s.cfg.Listen)
	if chk.E(err) {
		return err
	}
	s.listener = lis
	log.I.F("gRPC database server listening on %s", s.cfg.Listen)

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

	// Sync and close database
	log.I.F("syncing database...")
	if err := s.db.Sync(); chk.E(err) {
		log.W.F("failed to sync database: %v", err)
	}
	log.I.F("closing database...")
	if err := s.db.Close(); chk.E(err) {
		log.W.F("failed to close database: %v", err)
	}
	log.I.F("shutdown complete")
}

// Stop stops the server.
func (s *Server) Stop() {
	s.grpcServer.Stop()
}
