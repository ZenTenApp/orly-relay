package grpc

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
)

func init() {
	database.RegisterGRPCFactory(NewFromConfig)
}

// NewFromConfig creates a new gRPC database client from the database config.
func NewFromConfig(ctx context.Context, cancel context.CancelFunc, cfg *database.DatabaseConfig) (database.Database, error) {
	clientCfg := &ClientConfig{
		ServerAddress:  cfg.GRPCServerAddress,
		ConnectTimeout: cfg.GRPCConnectTimeout,
	}
	return New(ctx, clientCfg)
}
