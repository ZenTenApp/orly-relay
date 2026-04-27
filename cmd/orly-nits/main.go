// orly-nits is a gRPC shim that manages a bitcoind (nits) process and exposes
// health/status information for the orly-launcher supervisor.
package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"go-simpler.org/env"
	"google.golang.org/grpc"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	orlynitsv1 "git.smesh.lol/orly/pkg/proto/orlynits/v1"
)

// Config holds the orly-nits shim configuration.
type Config struct {
	// Listen is the gRPC server listen address for the shim
	Listen string `env:"ORLY_NITS_LISTEN" default:"127.0.0.1:50070" usage:"gRPC shim listen address"`

	// Binary is the path to the bitcoind/nits binary
	Binary string `env:"ORLY_NITS_BINARY" default:"nits" usage:"path to bitcoind/nits binary"`

	// DataDir is the bitcoind data directory
	DataDir string `env:"ORLY_NITS_DATA_DIR" usage:"bitcoind data directory"`

	// RPCPort is the bitcoind JSON-RPC port
	RPCPort int `env:"ORLY_NITS_RPC_PORT" default:"8332" usage:"bitcoind JSON-RPC port"`

	// RPCUser is the bitcoind RPC username (if not using cookie auth)
	RPCUser string `env:"ORLY_NITS_RPC_USER" usage:"bitcoind RPC username"`

	// RPCPass is the bitcoind RPC password (if not using cookie auth)
	RPCPass string `env:"ORLY_NITS_RPC_PASS" usage:"bitcoind RPC password"`

	// Network is the bitcoin network
	Network string `env:"ORLY_NITS_NETWORK" default:"mainnet" usage:"bitcoin network (mainnet, testnet, signet, regtest)"`

	// PruneSize is the prune target in MB (0 = no pruning)
	PruneSize int `env:"ORLY_NITS_PRUNE_MB" default:"2048" usage:"prune target in MB (0 = no pruning)"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_NITS_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`
}

func main() {
	cfg := loadConfig()

	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-nits starting")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.I.F("received signal %v, shutting down", sig)
		cancel()
	}()

	// Start bitcoind process
	node := NewBitcoind(cfg)
	if err := node.Start(ctx); err != nil {
		log.E.F("failed to start bitcoind: %v", err)
		os.Exit(1)
	}
	defer node.Stop()

	// Start gRPC server
	lis, err := net.Listen("tcp", cfg.Listen)
	if chk.E(err) {
		log.E.F("failed to listen on %s: %v", cfg.Listen, err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	svc := NewNitsService(node)
	orlynitsv1.RegisterNitsServiceServer(grpcServer, svc)

	log.I.F("orly-nits gRPC server listening on %s", cfg.Listen)

	// Serve until context is cancelled
	go func() {
		<-ctx.Done()
		log.I.F("stopping gRPC server")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		log.E.F("gRPC server error: %v", err)
	}

	log.I.F("orly-nits stopped")
}

func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Load(cfg, nil); chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}

	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if chk.E(err) {
			log.E.F("failed to get home directory: %v", err)
			os.Exit(1)
		}
		cfg.DataDir = filepath.Join(home, ".local", "share", "orly", "nits")
	}

	if err := os.MkdirAll(cfg.DataDir, 0700); chk.E(err) {
		log.E.F("failed to create data directory %s: %v", cfg.DataDir, err)
		os.Exit(1)
	}

	return cfg
}
