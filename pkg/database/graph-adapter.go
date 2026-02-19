//go:build !(js && wasm)

package database

import (
	"next.orly.dev/pkg/protocol/graph"
)

// GraphAdapter wraps a database instance and implements graph.GraphDatabase interface.
type GraphAdapter struct {
	db *D
}

// NewGraphAdapter creates a new GraphAdapter wrapping the given database.
func NewGraphAdapter(db *D) *GraphAdapter {
	return &GraphAdapter{db: db}
}

// TraversePubkeyPubkey implements graph.GraphDatabase.
// Uses the ppg/gpp materialized index for single-hop prefix scans.
func (a *GraphAdapter) TraversePubkeyPubkey(seedPubkey []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.db.TraversePubkeyPubkey(seedPubkey, maxDepth, direction)
}

// TraversePubkeyEvent implements graph.GraphDatabase.
func (a *GraphAdapter) TraversePubkeyEvent(seedPubkey []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.db.TraversePubkeyEvent(seedPubkey, maxDepth, direction)
}

// TraverseEventEvent implements graph.GraphDatabase.
func (a *GraphAdapter) TraverseEventEvent(seedEventID []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.db.TraverseEventEventFromPubkey(seedEventID, maxDepth, direction)
}

// TraversePubkeyPubkeyBaseline implements graph.GraphDatabase.
// Uses multi-hop NIP-01 queries (no ppg/gpp index) for benchmark comparison.
func (a *GraphAdapter) TraversePubkeyPubkeyBaseline(seedPubkey []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.db.TraversePubkeyPubkeyBaseline(seedPubkey, maxDepth, direction)
}

// Verify GraphAdapter implements graph.GraphDatabase
var _ graph.GraphDatabase = (*GraphAdapter)(nil)
