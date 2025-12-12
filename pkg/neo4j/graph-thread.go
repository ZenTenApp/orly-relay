package neo4j

import (
	"context"
	"fmt"
	"strings"

	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/protocol/graph"
)

// TraverseThread performs BFS traversal of thread structure via e-tags.
// Starting from a seed event, it finds all replies/references at each depth.
//
// The traversal works bidirectionally using REFERENCES relationships:
//   - Inbound: Events that reference the seed (replies, reactions, reposts)
//   - Outbound: Events that the seed references (parents, quoted posts)
//
// Note: REFERENCES relationships are only created if the referenced event exists
// in the database at the time of saving. This means some references may be missing
// if events were stored out of order.
//
// Parameters:
//   - seedEventID: The event ID to start traversal from
//   - maxDepth: Maximum depth to traverse
//   - direction: "both" (default), "inbound" (replies to seed), "outbound" (seed's references)
func (n *N) TraverseThread(seedEventID []byte, maxDepth int, direction string) (graph.GraphResultI, error) {
	result := NewGraphResult()

	if len(seedEventID) != 32 {
		return result, fmt.Errorf("invalid event ID length: expected 32, got %d", len(seedEventID))
	}

	seedHex := strings.ToLower(hex.Enc(seedEventID))
	ctx := context.Background()

	// Normalize direction
	if direction == "" {
		direction = "both"
	}

	// Track visited events
	visited := make(map[string]bool)
	visited[seedHex] = true

	// Process each depth level separately for BFS semantics
	for depth := 1; depth <= maxDepth; depth++ {
		newEventsAtDepth := 0

		// Get events at current depth
		visitedList := make([]string, 0, len(visited))
		for id := range visited {
			visitedList = append(visitedList, id)
		}

		// Process inbound references (events that reference the seed or its children)
		if direction == "both" || direction == "inbound" {
			inboundEvents, err := n.getInboundReferencesAtDepth(ctx, seedHex, depth, visitedList)
			if err != nil {
				n.Logger.Warningf("TraverseThread: error getting inbound refs at depth %d: %v", depth, err)
			} else {
				for _, eventID := range inboundEvents {
					if !visited[eventID] {
						visited[eventID] = true
						result.AddEventAtDepth(eventID, depth)
						newEventsAtDepth++
					}
				}
			}
		}

		// Process outbound references (events that the seed or its children reference)
		if direction == "both" || direction == "outbound" {
			outboundEvents, err := n.getOutboundReferencesAtDepth(ctx, seedHex, depth, visitedList)
			if err != nil {
				n.Logger.Warningf("TraverseThread: error getting outbound refs at depth %d: %v", depth, err)
			} else {
				for _, eventID := range outboundEvents {
					if !visited[eventID] {
						visited[eventID] = true
						result.AddEventAtDepth(eventID, depth)
						newEventsAtDepth++
					}
				}
			}
		}

		n.Logger.Debugf("TraverseThread: depth %d found %d new events", depth, newEventsAtDepth)

		// Early termination if no new events found at this depth
		if newEventsAtDepth == 0 {
			break
		}
	}

	n.Logger.Debugf("TraverseThread: completed with %d total events", result.TotalEvents)

	return result, nil
}

// getInboundReferencesAtDepth finds events that reference the seed event at exactly the given depth.
// Uses variable-length path patterns to find events N hops away.
func (n *N) getInboundReferencesAtDepth(ctx context.Context, seedID string, depth int, visited []string) ([]string, error) {
	// Query for events at exactly this depth that haven't been seen yet
	// Direction: (referencing_event)-[:REFERENCES]->(seed)
	// At depth 1: direct replies
	// At depth 2: replies to replies, etc.
	cypher := fmt.Sprintf(`
		MATCH path = (ref:Event)-[:REFERENCES*%d]->(seed:Event {id: $seed})
		WHERE ref.id <> $seed
		  AND NOT ref.id IN $visited
		RETURN DISTINCT ref.id AS event_id
	`, depth)

	params := map[string]any{
		"seed":    seedID,
		"visited": visited,
	}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var events []string
	for result.Next(ctx) {
		record := result.Record()
		eventID, ok := record.Values[0].(string)
		if !ok || eventID == "" {
			continue
		}
		events = append(events, strings.ToLower(eventID))
	}

	return events, nil
}

// getOutboundReferencesAtDepth finds events that the seed event references at exactly the given depth.
// Uses variable-length path patterns to find events N hops away.
func (n *N) getOutboundReferencesAtDepth(ctx context.Context, seedID string, depth int, visited []string) ([]string, error) {
	// Query for events at exactly this depth that haven't been seen yet
	// Direction: (seed)-[:REFERENCES]->(referenced_event)
	// At depth 1: direct parents/quotes
	// At depth 2: grandparents, etc.
	cypher := fmt.Sprintf(`
		MATCH path = (seed:Event {id: $seed})-[:REFERENCES*%d]->(ref:Event)
		WHERE ref.id <> $seed
		  AND NOT ref.id IN $visited
		RETURN DISTINCT ref.id AS event_id
	`, depth)

	params := map[string]any{
		"seed":    seedID,
		"visited": visited,
	}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	var events []string
	for result.Next(ctx) {
		record := result.Record()
		eventID, ok := record.Values[0].(string)
		if !ok || eventID == "" {
			continue
		}
		events = append(events, strings.ToLower(eventID))
	}

	return events, nil
}

// TraverseThreadFromHex is a convenience wrapper that accepts hex-encoded event ID.
func (n *N) TraverseThreadFromHex(seedEventIDHex string, maxDepth int, direction string) (*GraphResult, error) {
	seedEventID, err := hex.Dec(seedEventIDHex)
	if err != nil {
		return nil, err
	}
	result, err := n.TraverseThread(seedEventID, maxDepth, direction)
	if err != nil {
		return nil, err
	}
	return result.(*GraphResult), nil
}

// GetThreadReplies finds all direct replies to an event.
// This is a convenience method that returns events at depth 1 with inbound direction.
func (n *N) GetThreadReplies(eventID []byte, kinds []uint16) (*GraphResult, error) {
	result := NewGraphResult()

	if len(eventID) != 32 {
		return result, fmt.Errorf("invalid event ID length: expected 32, got %d", len(eventID))
	}

	eventIDHex := strings.ToLower(hex.Enc(eventID))
	ctx := context.Background()

	// Build kinds filter if specified
	var kindsFilter string
	params := map[string]any{
		"eventId": eventIDHex,
	}

	if len(kinds) > 0 {
		kindsInt := make([]int64, len(kinds))
		for i, k := range kinds {
			kindsInt[i] = int64(k)
		}
		params["kinds"] = kindsInt
		kindsFilter = "AND reply.kind IN $kinds"
	}

	// Query for direct replies
	cypher := fmt.Sprintf(`
		MATCH (reply:Event)-[:REFERENCES]->(e:Event {id: $eventId})
		WHERE true %s
		RETURN reply.id AS event_id
		ORDER BY reply.created_at DESC
	`, kindsFilter)

	queryResult, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return result, fmt.Errorf("failed to query replies: %w", err)
	}

	for queryResult.Next(ctx) {
		record := queryResult.Record()
		replyID, ok := record.Values[0].(string)
		if !ok || replyID == "" {
			continue
		}
		result.AddEventAtDepth(strings.ToLower(replyID), 1)
	}

	return result, nil
}

// GetThreadParents finds events that a given event references (its parents/quotes).
func (n *N) GetThreadParents(eventID []byte) (*GraphResult, error) {
	result := NewGraphResult()

	if len(eventID) != 32 {
		return result, fmt.Errorf("invalid event ID length: expected 32, got %d", len(eventID))
	}

	eventIDHex := strings.ToLower(hex.Enc(eventID))
	ctx := context.Background()

	params := map[string]any{
		"eventId": eventIDHex,
	}

	// Query for events that this event references
	cypher := `
		MATCH (e:Event {id: $eventId})-[:REFERENCES]->(parent:Event)
		RETURN parent.id AS event_id
		ORDER BY parent.created_at ASC
	`

	queryResult, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return result, fmt.Errorf("failed to query parents: %w", err)
	}

	for queryResult.Next(ctx) {
		record := queryResult.Record()
		parentID, ok := record.Values[0].(string)
		if !ok || parentID == "" {
			continue
		}
		result.AddEventAtDepth(strings.ToLower(parentID), 1)
	}

	return result, nil
}
