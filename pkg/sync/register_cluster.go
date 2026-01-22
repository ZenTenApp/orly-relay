//go:build !(js && wasm)

package sync

import (
	"context"

	"next.orly.dev/pkg/database"
)

func init() {
	// Register the Cluster driver with the driver registry.
	RegisterDriver("cluster", "Cluster-based replication", clusterFactory)
}

// clusterFactory creates a new Cluster sync service instance.
func clusterFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (Service, error) {
	// Cluster sync is implemented as a standalone service
	// The actual implementation would create a cluster.Manager here
	return &stubService{name: "cluster"}, nil
}
