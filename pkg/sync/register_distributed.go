//go:build !(js && wasm)

package sync

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
)

func init() {
	// Register the Distributed driver with the driver registry.
	RegisterDriver("distributed", "Distributed multi-node synchronization", distributedFactory)
}

// distributedFactory creates a new Distributed sync service instance.
func distributedFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (Service, error) {
	// Distributed sync is implemented as a standalone service
	// The actual implementation would create a distributed.Manager here
	return &stubService{name: "distributed"}, nil
}
