package neo4j

import (
	"context"
	"fmt"
	"strings"

	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/protocol/graph"
)

// FindMentions finds events that mention a pubkey via p-tags.
// This returns events grouped by depth, where depth represents how the events relate:
//   - Depth 1: Events that directly mention the seed pubkey
//   - Depth 2+: Not typically used for mentions (reserved for future expansion)
//
// The kinds parameter filters which event kinds to include (e.g., [1] for notes only,
// [1,7] for notes and reactions, etc.)
//
// Uses Neo4j MENTIONS relationships created by SaveEvent when processing p-tags.
func (n *N) FindMentions(pubkey []byte, kinds []uint16) (graph.GraphResultI, error) {
	result := NewGraphResult()

	if len(pubkey) != 32 {
		return result, fmt.Errorf("invalid pubkey length: expected 32, got %d", len(pubkey))
	}

	pubkeyHex := strings.ToLower(hex.Enc(pubkey))
	ctx := context.Background()

	// Build kinds filter if specified
	var kindsFilter string
	params := map[string]any{
		"pubkey": pubkeyHex,
	}

	if len(kinds) > 0 {
		// Convert uint16 slice to int64 slice for Neo4j
		kindsInt := make([]int64, len(kinds))
		for i, k := range kinds {
			kindsInt[i] = int64(k)
		}
		params["kinds"] = kindsInt
		kindsFilter = "AND e.kind IN $kinds"
	}

	// Query for events that mention this pubkey
	// The MENTIONS relationship is created by SaveEvent when processing p-tags
	cypher := fmt.Sprintf(`
		MATCH (e:Event)-[:MENTIONS]->(u:NostrUser {pubkey: $pubkey})
		WHERE true %s
		RETURN e.id AS event_id
		ORDER BY e.created_at DESC
	`, kindsFilter)

	queryResult, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return result, fmt.Errorf("failed to query mentions: %w", err)
	}

	// Add all found events at depth 1
	for queryResult.Next(ctx) {
		record := queryResult.Record()
		eventID, ok := record.Values[0].(string)
		if !ok || eventID == "" {
			continue
		}

		// Normalize to lowercase for consistency
		eventID = strings.ToLower(eventID)
		result.AddEventAtDepth(eventID, 1)
	}

	n.Logger.Debugf("FindMentions: found %d events mentioning pubkey %s", result.TotalEvents, safePrefix(pubkeyHex, 16))

	return result, nil
}

// FindMentionsFromHex is a convenience wrapper that accepts hex-encoded pubkey.
func (n *N) FindMentionsFromHex(pubkeyHex string, kinds []uint16) (*GraphResult, error) {
	pubkey, err := hex.Dec(pubkeyHex)
	if err != nil {
		return nil, err
	}
	result, err := n.FindMentions(pubkey, kinds)
	if err != nil {
		return nil, err
	}
	return result.(*GraphResult), nil
}

// FindMentionsByPubkeys returns events that mention any of the given pubkeys.
// Useful for finding mentions across a set of followed accounts.
func (n *N) FindMentionsByPubkeys(pubkeys []string, kinds []uint16) (*GraphResult, error) {
	result := NewGraphResult()

	if len(pubkeys) == 0 {
		return result, nil
	}

	ctx := context.Background()

	// Build kinds filter if specified
	var kindsFilter string
	params := map[string]any{
		"pubkeys": pubkeys,
	}

	if len(kinds) > 0 {
		kindsInt := make([]int64, len(kinds))
		for i, k := range kinds {
			kindsInt[i] = int64(k)
		}
		params["kinds"] = kindsInt
		kindsFilter = "AND e.kind IN $kinds"
	}

	// Query for events that mention any of the pubkeys
	cypher := fmt.Sprintf(`
		MATCH (e:Event)-[:MENTIONS]->(u:NostrUser)
		WHERE u.pubkey IN $pubkeys %s
		RETURN DISTINCT e.id AS event_id
		ORDER BY e.created_at DESC
	`, kindsFilter)

	queryResult, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return result, fmt.Errorf("failed to query mentions: %w", err)
	}

	for queryResult.Next(ctx) {
		record := queryResult.Record()
		eventID, ok := record.Values[0].(string)
		if !ok || eventID == "" {
			continue
		}

		eventID = strings.ToLower(eventID)
		result.AddEventAtDepth(eventID, 1)
	}

	return result, nil
}
