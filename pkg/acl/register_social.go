//go:build !(js && wasm)

package acl

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
)

func init() {
	RegisterDriver("social", "WoT graph topology with inbound-trust rate limiting", socialFactory)
}

func socialFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (I, error) {
	s := new(Social)
	return s, nil
}
