//go:build !(js && wasm)

package sync

import (
	"context"

	"next.orly.dev/pkg/database"
)

func init() {
	// Register the RelayGroup driver with the driver registry.
	RegisterDriver("relaygroup", "Relay group configuration management", relaygroupFactory)
}

// relaygroupFactory creates a new RelayGroup sync service instance.
func relaygroupFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (Service, error) {
	// RelayGroup is implemented as a manager service
	// The actual implementation would create a relaygroup.Manager here
	return &stubService{name: "relaygroup"}, nil
}
