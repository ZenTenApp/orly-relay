//go:build !(js && wasm)

package acl

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
)

func init() {
	RegisterDriver("paid", "Lightning payment-gated access control", paidFactory)
}

func paidFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (I, error) {
	p := new(Paid)
	return p, nil
}
