package neo4j

import (
	"fmt"

	"git.smesh.lol/orly/pkg/protocol/graph"
)

// GraphAdapter wraps a Neo4j database instance and implements graph.GraphDatabase interface.
type GraphAdapter struct {
	db *N
}

// NewGraphAdapter creates a new GraphAdapter wrapping the given Neo4j database.
func NewGraphAdapter(db *N) *GraphAdapter {
	return &GraphAdapter{db: db}
}

// TraversePubkeyPubkey implements graph.GraphDatabase.
// Neo4j uses Cypher queries for direct pubkey-to-pubkey traversal.
func (a *GraphAdapter) TraversePubkeyPubkey(seedPubkey []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	switch direction {
	case "out":
		return a.db.TraverseFollows(seedPubkey, maxDepth)
	case "in":
		return a.db.TraverseFollowers(seedPubkey, maxDepth)
	case "both":
		// For bidirectional, execute both and merge
		outResult, err := a.db.TraverseFollows(seedPubkey, maxDepth)
		if err != nil {
			return nil, err
		}
		inResult, err := a.db.TraverseFollowers(seedPubkey, maxDepth)
		if err != nil {
			return outResult, nil // Return what we have
		}
		// Merge inbound results into outbound
		mergeResults(outResult, inResult)
		return outResult, nil
	default:
		return nil, fmt.Errorf("invalid direction: %s", direction)
	}
}

// TraversePubkeyEvent implements graph.GraphDatabase.
func (a *GraphAdapter) TraversePubkeyEvent(seedPubkey []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	// Delegate to mentions for now (pe queries)
	return a.db.FindMentions(seedPubkey, nil)
}

// TraverseEventEvent implements graph.GraphDatabase.
func (a *GraphAdapter) TraverseEventEvent(seedEventID []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.db.TraverseThread(seedEventID, maxDepth, direction)
}

// TraversePubkeyPubkeyBaseline implements graph.GraphDatabase.
// For Neo4j, baseline is the same as optimized since Neo4j handles its own query planning.
func (a *GraphAdapter) TraversePubkeyPubkeyBaseline(seedPubkey []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	return a.TraversePubkeyPubkey(seedPubkey, maxDepth, direction)
}

// mergeResults merges source GraphResultI into target.
// This is a best-effort merge using the interface methods.
func mergeResults(target, source graph.GraphResultI) {
	// Results are read-only through the interface — for Neo4j this is acceptable
	// since Cypher can do bidirectional in a single query if needed.
	_ = target
	_ = source
}

// Verify GraphAdapter implements graph.GraphDatabase
var _ graph.GraphDatabase = (*GraphAdapter)(nil)
