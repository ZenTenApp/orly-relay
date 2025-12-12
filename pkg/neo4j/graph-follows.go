package neo4j

import (
	"context"
	"fmt"
	"strings"

	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/protocol/graph"
)

// TraverseFollows performs BFS traversal of the follow graph starting from a seed pubkey.
// Returns pubkeys grouped by first-discovered depth (no duplicates across depths).
//
// Uses Neo4j's native path queries with FOLLOWS relationships created by
// the social event processor from kind 3 contact list events.
//
// The traversal works by using variable-length path patterns:
//   - Depth 1: Direct follows (seed)-[:FOLLOWS]->(followed)
//   - Depth 2: Follows of follows (seed)-[:FOLLOWS*2]->(followed)
//   - etc.
//
// Each pubkey appears only at the depth where it was first discovered.
func (n *N) TraverseFollows(seedPubkey []byte, maxDepth int) (graph.GraphResultI, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, fmt.Errorf("invalid pubkey length: expected 32, got %d", len(seedPubkey))
	}

	seedHex := strings.ToLower(hex.Enc(seedPubkey))
	ctx := context.Background()

	// Track visited pubkeys to ensure each appears only at first-discovered depth
	visited := make(map[string]bool)
	visited[seedHex] = true // Seed is at depth 0, not included in results

	// Process each depth level separately to maintain BFS semantics
	for depth := 1; depth <= maxDepth; depth++ {
		// Query for pubkeys at exactly this depth that haven't been seen yet
		// We use a variable-length path of exactly 'depth' hops
		cypher := fmt.Sprintf(`
			MATCH path = (seed:NostrUser {pubkey: $seed})-[:FOLLOWS*%d]->(target:NostrUser)
			WHERE target.pubkey <> $seed
			  AND NOT target.pubkey IN $visited
			RETURN DISTINCT target.pubkey AS pubkey
		`, depth)

		// Convert visited map to slice for query
		visitedList := make([]string, 0, len(visited))
		for pk := range visited {
			visitedList = append(visitedList, pk)
		}

		params := map[string]any{
			"seed":    seedHex,
			"visited": visitedList,
		}

		queryResult, err := n.ExecuteRead(ctx, cypher, params)
		if err != nil {
			n.Logger.Warningf("TraverseFollows: error at depth %d: %v", depth, err)
			continue
		}

		newPubkeysAtDepth := 0
		for queryResult.Next(ctx) {
			record := queryResult.Record()
			pubkey, ok := record.Values[0].(string)
			if !ok || pubkey == "" {
				continue
			}

			// Normalize to lowercase for consistency
			pubkey = strings.ToLower(pubkey)

			// Add to result if not already seen
			if !visited[pubkey] {
				visited[pubkey] = true
				result.AddPubkeyAtDepth(pubkey, depth)
				newPubkeysAtDepth++
			}
		}

		n.Logger.Debugf("TraverseFollows: depth %d found %d new pubkeys", depth, newPubkeysAtDepth)

		// Early termination if no new pubkeys found at this depth
		if newPubkeysAtDepth == 0 {
			break
		}
	}

	n.Logger.Debugf("TraverseFollows: completed with %d total pubkeys across %d depths",
		result.TotalPubkeys, len(result.PubkeysByDepth))

	return result, nil
}

// TraverseFollowers performs BFS traversal to find who follows the seed pubkey.
// This is the reverse of TraverseFollows - it finds users whose kind-3 lists
// contain the target pubkey(s).
//
// Uses Neo4j's native path queries, but in reverse direction:
//   - Depth 1: Users who directly follow the seed (follower)-[:FOLLOWS]->(seed)
//   - Depth 2: Users who follow anyone at depth 1 (followers of followers)
//   - etc.
func (n *N) TraverseFollowers(seedPubkey []byte, maxDepth int) (graph.GraphResultI, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, fmt.Errorf("invalid pubkey length: expected 32, got %d", len(seedPubkey))
	}

	seedHex := strings.ToLower(hex.Enc(seedPubkey))
	ctx := context.Background()

	// Track visited pubkeys
	visited := make(map[string]bool)
	visited[seedHex] = true

	// Process each depth level separately for BFS semantics
	for depth := 1; depth <= maxDepth; depth++ {
		// Query for pubkeys at exactly this depth that haven't been seen yet
		// Direction is reversed: we find users who follow the targets
		cypher := fmt.Sprintf(`
			MATCH path = (follower:NostrUser)-[:FOLLOWS*%d]->(seed:NostrUser {pubkey: $seed})
			WHERE follower.pubkey <> $seed
			  AND NOT follower.pubkey IN $visited
			RETURN DISTINCT follower.pubkey AS pubkey
		`, depth)

		visitedList := make([]string, 0, len(visited))
		for pk := range visited {
			visitedList = append(visitedList, pk)
		}

		params := map[string]any{
			"seed":    seedHex,
			"visited": visitedList,
		}

		queryResult, err := n.ExecuteRead(ctx, cypher, params)
		if err != nil {
			n.Logger.Warningf("TraverseFollowers: error at depth %d: %v", depth, err)
			continue
		}

		newPubkeysAtDepth := 0
		for queryResult.Next(ctx) {
			record := queryResult.Record()
			pubkey, ok := record.Values[0].(string)
			if !ok || pubkey == "" {
				continue
			}

			pubkey = strings.ToLower(pubkey)

			if !visited[pubkey] {
				visited[pubkey] = true
				result.AddPubkeyAtDepth(pubkey, depth)
				newPubkeysAtDepth++
			}
		}

		n.Logger.Debugf("TraverseFollowers: depth %d found %d new pubkeys", depth, newPubkeysAtDepth)

		if newPubkeysAtDepth == 0 {
			break
		}
	}

	n.Logger.Debugf("TraverseFollowers: completed with %d total pubkeys", result.TotalPubkeys)

	return result, nil
}

// TraverseFollowsFromHex is a convenience wrapper that accepts hex-encoded pubkey.
func (n *N) TraverseFollowsFromHex(seedPubkeyHex string, maxDepth int) (*GraphResult, error) {
	seedPubkey, err := hex.Dec(seedPubkeyHex)
	if err != nil {
		return nil, err
	}
	result, err := n.TraverseFollows(seedPubkey, maxDepth)
	if err != nil {
		return nil, err
	}
	return result.(*GraphResult), nil
}

// TraverseFollowersFromHex is a convenience wrapper that accepts hex-encoded pubkey.
func (n *N) TraverseFollowersFromHex(seedPubkeyHex string, maxDepth int) (*GraphResult, error) {
	seedPubkey, err := hex.Dec(seedPubkeyHex)
	if err != nil {
		return nil, err
	}
	result, err := n.TraverseFollowers(seedPubkey, maxDepth)
	if err != nil {
		return nil, err
	}
	return result.(*GraphResult), nil
}
