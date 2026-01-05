package neo4j

import (
	"context"
	"fmt"
	"strings"
)

// AddInboundRefsToResult collects inbound references (events that reference discovered items)
// for events at a specific depth in the result.
//
// For example, if you have a follows graph result and want to find all kind-7 reactions
// to posts by users at depth 1, this collects those reactions and adds them to result.InboundRefs.
//
// Parameters:
// - result: The graph result to augment with ref data
// - depth: The depth at which to collect refs (0 = all depths)
// - kinds: Event kinds to collect (e.g., [7] for reactions, [6] for reposts)
func (n *N) AddInboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error {
	ctx := context.Background()

	// Get pubkeys to find refs for
	var pubkeys []string
	if depth == 0 {
		pubkeys = result.GetAllPubkeys()
	} else {
		pubkeys = result.GetPubkeysAtDepth(depth)
	}

	if len(pubkeys) == 0 {
		n.Logger.Debugf("AddInboundRefsToResult: no pubkeys at depth %d", depth)
		return nil
	}

	// Convert kinds to int64 for Neo4j
	kindsInt := make([]int64, len(kinds))
	for i, k := range kinds {
		kindsInt[i] = int64(k)
	}

	// Query for events by these pubkeys and their inbound references
	// This finds: (ref:Event)-[:REFERENCES]->(authored:Event)<-[:AUTHORED_BY]-(u:NostrUser)
	// where the referencing event has the specified kinds
	cypher := `
		UNWIND $pubkeys AS pk
		MATCH (u:NostrUser {pubkey: pk})<-[:AUTHORED_BY]-(authored:Event)
		WHERE authored.kind IN [1, 30023]
		MATCH (ref:Event)-[:REFERENCES]->(authored)
		WHERE ref.kind IN $kinds
		RETURN authored.id AS target_id, ref.id AS ref_id, ref.kind AS ref_kind
	`

	params := map[string]any{
		"pubkeys": pubkeys,
		"kinds":   kindsInt,
	}

	queryResult, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to query inbound refs: %w", err)
	}

	refCount := 0
	for queryResult.Next(ctx) {
		record := queryResult.Record()

		targetID, ok := record.Values[0].(string)
		if !ok || targetID == "" {
			continue
		}

		refID, ok := record.Values[1].(string)
		if !ok || refID == "" {
			continue
		}

		refKind, ok := record.Values[2].(int64)
		if !ok {
			continue
		}

		result.AddInboundRef(uint16(refKind), strings.ToLower(targetID), strings.ToLower(refID))
		refCount++
	}

	n.Logger.Debugf("AddInboundRefsToResult: collected %d refs for %d pubkeys", refCount, len(pubkeys))

	return nil
}

// AddOutboundRefsToResult collects outbound references (events referenced by discovered items).
//
// For example, find all events that posts by users at depth 1 reference (quoted posts, replied-to posts).
func (n *N) AddOutboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error {
	ctx := context.Background()

	// Get pubkeys to find refs for
	var pubkeys []string
	if depth == 0 {
		pubkeys = result.GetAllPubkeys()
	} else {
		pubkeys = result.GetPubkeysAtDepth(depth)
	}

	if len(pubkeys) == 0 {
		n.Logger.Debugf("AddOutboundRefsToResult: no pubkeys at depth %d", depth)
		return nil
	}

	// Convert kinds to int64 for Neo4j
	kindsInt := make([]int64, len(kinds))
	for i, k := range kinds {
		kindsInt[i] = int64(k)
	}

	// Query for events by these pubkeys and their outbound references
	// This finds: (authored:Event)-[:REFERENCES]->(ref:Event)
	// where the authored event has the specified kinds
	cypher := `
		UNWIND $pubkeys AS pk
		MATCH (u:NostrUser {pubkey: pk})<-[:AUTHORED_BY]-(authored:Event)
		WHERE authored.kind IN $kinds
		MATCH (authored)-[:REFERENCES]->(ref:Event)
		RETURN authored.id AS source_id, ref.id AS ref_id, authored.kind AS source_kind
	`

	params := map[string]any{
		"pubkeys": pubkeys,
		"kinds":   kindsInt,
	}

	queryResult, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to query outbound refs: %w", err)
	}

	refCount := 0
	for queryResult.Next(ctx) {
		record := queryResult.Record()

		sourceID, ok := record.Values[0].(string)
		if !ok || sourceID == "" {
			continue
		}

		refID, ok := record.Values[1].(string)
		if !ok || refID == "" {
			continue
		}

		sourceKind, ok := record.Values[2].(int64)
		if !ok {
			continue
		}

		result.AddOutboundRef(uint16(sourceKind), strings.ToLower(sourceID), strings.ToLower(refID))
		refCount++
	}

	n.Logger.Debugf("AddOutboundRefsToResult: collected %d refs from %d pubkeys", refCount, len(pubkeys))

	return nil
}
