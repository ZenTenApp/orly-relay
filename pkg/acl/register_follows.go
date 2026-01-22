//go:build !(js && wasm)

package acl

import (
	"context"

	"next.orly.dev/pkg/database"
)

func init() {
	// Register the Follows driver with the driver registry.
	RegisterDriver("follows", "Whitelist based on admin follow lists", followsFactory)
}

// followsFactory creates a new Follows ACL instance.
func followsFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (I, error) {
	// Create a new Follows instance
	f := new(Follows)
	// The Follows ACL will be configured via the Configure method
	// which is called by the ACL server after creation.
	return f, nil
}
