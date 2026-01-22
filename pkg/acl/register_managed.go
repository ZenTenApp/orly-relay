//go:build !(js && wasm)

package acl

import (
	"context"

	"next.orly.dev/pkg/database"
)

func init() {
	// Register the Managed driver with the driver registry.
	RegisterDriver("managed", "NIP-86 fine-grained access control", managedFactory)
}

// managedFactory creates a new Managed ACL instance.
func managedFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (I, error) {
	// Create a new Managed instance
	m := new(Managed)
	// The Managed ACL will be configured via the Configure method
	// which is called by the ACL server after creation.
	return m, nil
}
