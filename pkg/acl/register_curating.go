//go:build !(js && wasm)

package acl

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
)

func init() {
	// Register the Curating driver with the driver registry.
	RegisterDriver("curating", "Rate-limited trust tier system", curatingFactory)
}

// curatingFactory creates a new Curating ACL instance.
func curatingFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (I, error) {
	// Create a new Curating instance
	c := new(Curating)
	// The Curating ACL will be configured via the Configure method
	// which is called by the ACL server after creation.
	return c, nil
}
