//go:build !(js && wasm)

package database

import (
	"context"
)

func init() {
	// Register the Badger driver with the driver registry.
	// This is always available on non-wasm builds.
	RegisterDriver("badger", "Badger LSM database (default)", badgerFactory)
}

// badgerFactory creates a new Badger database instance.
func badgerFactory(ctx context.Context, cancel context.CancelFunc, cfg *DatabaseConfig) (Database, error) {
	return NewWithConfig(ctx, cancel, cfg)
}
