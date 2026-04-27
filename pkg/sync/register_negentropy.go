//go:build !(js && wasm)

package sync

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
)

func init() {
	// Register the Negentropy driver with the driver registry.
	RegisterDriver("negentropy", "NIP-77 negentropy set reconciliation", negentropyFactory)
}

// negentropyFactory creates a new Negentropy sync service instance.
func negentropyFactory(ctx context.Context, db database.Database, cfg *DriverConfig) (Service, error) {
	// Negentropy sync is implemented as a standalone service
	// The actual implementation would create a negentropy.Service here
	return &stubService{name: "negentropy"}, nil
}

// stubService is a placeholder for sync services not yet integrated
type stubService struct {
	name string
}

func (s *stubService) Start() error { return nil }
func (s *stubService) Stop() error  { return nil }
func (s *stubService) Type() string { return s.name }
