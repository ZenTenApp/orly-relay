//go:build !(js && wasm)

package database

import (
	"next.orly.dev/pkg/protocol/graph"
)

// GraphAdapter wraps a database instance and implements graph.GraphDatabase interface.
// This allows the graph executor to call database traversal methods without
// the database package importing the graph package.
type GraphAdapter struct {
	db *D
}

// NewGraphAdapter creates a new GraphAdapter wrapping the given database.
func NewGraphAdapter(db *D) *GraphAdapter {
	return &GraphAdapter{db: db}
}

// TraverseFollows implements graph.GraphDatabase.
func (a *GraphAdapter) TraverseFollows(seedPubkey []byte, maxDepth int) (graph.GraphResultI, error) {
	return a.db.TraverseFollows(seedPubkey, maxDepth)
}

// TraverseFollowers implements graph.GraphDatabase.
func (a *GraphAdapter) TraverseFollowers(seedPubkey []byte, maxDepth int) (graph.GraphResultI, error) {
	return a.db.TraverseFollowers(seedPubkey, maxDepth)
}

// FindMentions implements graph.GraphDatabase.
func (a *GraphAdapter) FindMentions(pubkey []byte, kinds []uint16) (graph.GraphResultI, error) {
	return a.db.FindMentions(pubkey, kinds)
}

// TraverseThread implements graph.GraphDatabase.
func (a *GraphAdapter) TraverseThread(seedEventID []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.db.TraverseThread(seedEventID, maxDepth, direction)
}

// CollectInboundRefs implements graph.GraphDatabase.
// It collects events that reference items in the result.
func (a *GraphAdapter) CollectInboundRefs(result graph.GraphResultI, depth int, kinds []uint16) error {
	// Type assert to get the concrete GraphResult
	graphResult, ok := result.(*GraphResult)
	if !ok {
		return nil // Can't collect refs if we don't have a GraphResult
	}
	return a.db.AddInboundRefsToResult(graphResult, depth, kinds)
}

// CollectOutboundRefs implements graph.GraphDatabase.
// It collects events referenced by items in the result.
func (a *GraphAdapter) CollectOutboundRefs(result graph.GraphResultI, depth int, kinds []uint16) error {
	// Type assert to get the concrete GraphResult
	graphResult, ok := result.(*GraphResult)
	if !ok {
		return nil
	}
	return a.db.AddOutboundRefsToResult(graphResult, depth, kinds)
}

// Verify GraphAdapter implements graph.GraphDatabase
var _ graph.GraphDatabase = (*GraphAdapter)(nil)
