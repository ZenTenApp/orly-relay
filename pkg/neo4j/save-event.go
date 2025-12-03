package neo4j

import (
	"context"
	"fmt"
	"strconv"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/database/indexes/types"
)

// parseInt64 parses a string to int64
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// SaveEvent stores a Nostr event in the Neo4j database.
// It creates event nodes and relationships for authors, tags, and references.
// This method leverages Neo4j's graph capabilities to model Nostr's social graph naturally.
//
// For social graph events (kinds 0, 3, 1984, 10000), it additionally processes them
// to maintain NostrUser nodes and FOLLOWS/MUTES/REPORTS relationships with event traceability.
func (n *N) SaveEvent(c context.Context, ev *event.E) (exists bool, err error) {
	eventID := hex.Enc(ev.ID[:])

	// Check if event already exists
	checkCypher := "MATCH (e:Event {id: $id}) RETURN e.id AS id"
	checkParams := map[string]any{"id": eventID}

	result, err := n.ExecuteRead(c, checkCypher, checkParams)
	if err != nil {
		return false, fmt.Errorf("failed to check event existence: %w", err)
	}

	// Check if we got a result
	ctx := context.Background()
	if result.Next(ctx) {
		// Event exists - check if it's a social event that needs reprocessing
		// (in case relationships changed)
		if ev.Kind == 0 || ev.Kind == 3 || ev.Kind == 1984 || ev.Kind == 10000 {
			processor := NewSocialEventProcessor(n)
			if err := processor.ProcessSocialEvent(c, ev); err != nil {
				n.Logger.Warningf("failed to reprocess social event %s: %v", eventID[:16], err)
				// Don't fail the whole save, social processing is supplementary
			}
		}
		return true, nil // Event already exists
	}

	// Get next serial number
	serial, err := n.getNextSerial()
	if err != nil {
		return false, fmt.Errorf("failed to get serial number: %w", err)
	}

	// Build and execute Cypher query to create event with all relationships
	// This creates Event and Author nodes for NIP-01 query support
	cypher, params := n.buildEventCreationCypher(ev, serial)

	if _, err = n.ExecuteWrite(c, cypher, params); err != nil {
		return false, fmt.Errorf("failed to save event: %w", err)
	}

	// Process social graph events (kinds 0, 3, 1984, 10000)
	// This creates NostrUser nodes and social relationships (FOLLOWS, MUTES, REPORTS)
	// with event traceability for diff-based updates
	if ev.Kind == 0 || ev.Kind == 3 || ev.Kind == 1984 || ev.Kind == 10000 {
		processor := NewSocialEventProcessor(n)
		if err := processor.ProcessSocialEvent(c, ev); err != nil {
			// Log error but don't fail the whole save
			// NIP-01 queries will still work even if social processing fails
			n.Logger.Errorf("failed to process social event kind %d, event %s: %v",
				ev.Kind, eventID[:16], err)
			// Consider: should we fail here or continue?
			// For now, continue - social graph is supplementary to base relay
		}
	}

	return false, nil
}

// buildEventCreationCypher constructs a Cypher query to create an event node with all relationships
// This is a single atomic operation that creates:
// - Event node with all properties
// - NostrUser node and AUTHORED_BY relationship (unified author + WoT node)
// - Tag nodes and TAGGED_WITH relationships
// - Reference relationships (REFERENCES for 'e' tags, MENTIONS for 'p' tags)
func (n *N) buildEventCreationCypher(ev *event.E, serial uint64) (string, map[string]any) {
	params := make(map[string]any)

	// Event properties
	eventID := hex.Enc(ev.ID[:])
	authorPubkey := hex.Enc(ev.Pubkey[:])

	params["eventId"] = eventID
	params["serial"] = serial
	params["kind"] = int64(ev.Kind)
	params["createdAt"] = ev.CreatedAt
	params["content"] = string(ev.Content)
	params["sig"] = hex.Enc(ev.Sig[:])
	params["pubkey"] = authorPubkey

	// Check for expiration tag (NIP-40)
	var expirationTs int64 = 0
	if ev.Tags != nil {
		if expTag := ev.Tags.GetFirst([]byte("expiration")); expTag != nil && len(expTag.T) >= 2 {
			if ts, err := parseInt64(string(expTag.T[1])); err == nil {
				expirationTs = ts
			}
		}
	}
	params["expiration"] = expirationTs

	// Serialize tags as JSON string for storage
	// Handle nil tags gracefully - nil means empty tags "[]"
	var tagsJSON []byte
	if ev.Tags != nil {
		tagsJSON, _ = ev.Tags.MarshalJSON()
	} else {
		tagsJSON = []byte("[]")
	}
	params["tags"] = string(tagsJSON)

	// Start building the Cypher query
	// Use MERGE to ensure idempotency for NostrUser nodes
	// NostrUser serves both NIP-01 author tracking and WoT social graph
	cypher := `
// Create or match NostrUser node (unified author + social graph)
MERGE (a:NostrUser {pubkey: $pubkey})
ON CREATE SET a.created_at = timestamp(), a.first_seen_event = $eventId

// Create event node with expiration for NIP-40 support
CREATE (e:Event {
  id: $eventId,
  serial: $serial,
  kind: $kind,
  created_at: $createdAt,
  content: $content,
  sig: $sig,
  pubkey: $pubkey,
  tags: $tags,
  expiration: $expiration
})

// Link event to author
CREATE (e)-[:AUTHORED_BY]->(a)
`

	// Process tags to create relationships
	// Different tag types create different relationship patterns
	tagNodeIndex := 0
	eTagIndex := 0
	pTagIndex := 0

	// Track if we need to add WITH clause before OPTIONAL MATCH
	// This is required because Cypher doesn't allow MATCH after CREATE without WITH
	needsWithClause := true

	// Only process tags if they exist
	if ev.Tags != nil {
		for _, tagItem := range *ev.Tags {
			if len(tagItem.T) < 2 {
				continue
			}

			tagType := string(tagItem.T[0])

			switch tagType {
			case "e": // Event reference - creates REFERENCES relationship
				// Use ExtractETagValue to handle binary encoding and normalize to lowercase hex
				tagValue := ExtractETagValue(tagItem)
				if tagValue == "" {
					continue // Skip invalid e-tags
				}

				// Create reference to another event (if it exists)
				paramName := fmt.Sprintf("eTag_%d", eTagIndex)
				params[paramName] = tagValue

				// Add WITH clause before OPTIONAL MATCH
				// This is required because:
				// 1. Cypher doesn't allow MATCH after CREATE without WITH
				// 2. Cypher doesn't allow MATCH after FOREACH without WITH
				// So we need WITH before EVERY OPTIONAL MATCH, not just the first
				if needsWithClause {
					cypher += `
// Carry forward event and author nodes for tag processing
WITH e, a
`
					needsWithClause = false
				} else {
					// After a FOREACH, we need WITH to transition back to MATCH
					cypher += `
WITH e, a
`
				}

				cypher += fmt.Sprintf(`
// Reference to event (e-tag)
OPTIONAL MATCH (ref%d:Event {id: $%s})
FOREACH (ignoreMe IN CASE WHEN ref%d IS NOT NULL THEN [1] ELSE [] END |
  CREATE (e)-[:REFERENCES]->(ref%d)
)
`, eTagIndex, paramName, eTagIndex, eTagIndex)

				eTagIndex++

			case "p": // Pubkey mention - creates MENTIONS relationship
				// Use ExtractPTagValue to handle binary encoding and normalize to lowercase hex
				tagValue := ExtractPTagValue(tagItem)
				if tagValue == "" {
					continue // Skip invalid p-tags
				}

				// Create mention to another NostrUser
				paramName := fmt.Sprintf("pTag_%d", pTagIndex)
				params[paramName] = tagValue

				cypher += fmt.Sprintf(`
// Mention of NostrUser (p-tag)
MERGE (mentioned%d:NostrUser {pubkey: $%s})
ON CREATE SET mentioned%d.created_at = timestamp()
CREATE (e)-[:MENTIONS]->(mentioned%d)
`, pTagIndex, paramName, pTagIndex, pTagIndex)

				pTagIndex++

			default: // Other tags - creates Tag nodes and TAGGED_WITH relationships
				// For non-e/p tags, use direct string conversion (no binary encoding)
				tagValue := string(tagItem.T[1])

				// Create tag node and relationship
				typeParam := fmt.Sprintf("tagType_%d", tagNodeIndex)
				valueParam := fmt.Sprintf("tagValue_%d", tagNodeIndex)
				params[typeParam] = tagType
				params[valueParam] = tagValue

				cypher += fmt.Sprintf(`
// Generic tag relationship
MERGE (tag%d:Tag {type: $%s, value: $%s})
CREATE (e)-[:TAGGED_WITH]->(tag%d)
`, tagNodeIndex, typeParam, valueParam, tagNodeIndex)

				tagNodeIndex++
			}
		}
	}

	// Return the created event
	cypher += `
RETURN e.id AS id`

	return cypher, params
}

// GetSerialsFromFilter returns event serials matching a filter
func (n *N) GetSerialsFromFilter(f *filter.F) (serials types.Uint40s, err error) {
	// Use QueryForSerials with background context
	return n.QueryForSerials(context.Background(), f)
}

// WouldReplaceEvent checks if an event would replace existing events
// This handles replaceable events (kinds 0, 3, and 10000-19999)
// and parameterized replaceable events (kinds 30000-39999)
func (n *N) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	// Check for replaceable events (kinds 0, 3, and 10000-19999)
	isReplaceable := ev.Kind == 0 || ev.Kind == 3 || (ev.Kind >= 10000 && ev.Kind < 20000)

	// Check for parameterized replaceable events (kinds 30000-39999)
	isParameterizedReplaceable := ev.Kind >= 30000 && ev.Kind < 40000

	if !isReplaceable && !isParameterizedReplaceable {
		return false, nil, nil
	}

	authorPubkey := hex.Enc(ev.Pubkey[:])
	ctx := context.Background()

	var cypher string
	params := map[string]any{
		"pubkey":    authorPubkey,
		"kind":      int64(ev.Kind),
		"createdAt": ev.CreatedAt,
	}

	if isParameterizedReplaceable {
		// For parameterized replaceable events, we need to match on d-tag as well
		dTag := ev.Tags.GetFirst([]byte{'d'})
		if dTag == nil {
			return false, nil, nil
		}

		dValue := ""
		if len(dTag.T) >= 2 {
			dValue = string(dTag.T[1])
		}

		params["dValue"] = dValue

		// Query for existing parameterized replaceable events with same kind, pubkey, and d-tag
		cypher = `
MATCH (e:Event {kind: $kind, pubkey: $pubkey})-[:TAGGED_WITH]->(t:Tag {type: 'd', value: $dValue})
WHERE e.created_at < $createdAt
RETURN e.serial AS serial, e.created_at AS created_at
ORDER BY e.created_at DESC`

	} else {
		// Query for existing replaceable events with same kind and pubkey
		cypher = `
MATCH (e:Event {kind: $kind, pubkey: $pubkey})
WHERE e.created_at < $createdAt
RETURN e.serial AS serial, e.created_at AS created_at
ORDER BY e.created_at DESC`
	}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return false, nil, fmt.Errorf("failed to query replaceable events: %w", err)
	}

	// Parse results
	var serials types.Uint40s
	wouldReplace := false

	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			continue
		}

		serialRaw, found := record.Get("serial")
		if !found {
			continue
		}

		serialVal, ok := serialRaw.(int64)
		if !ok {
			continue
		}

		wouldReplace = true
		serial := types.Uint40{}
		serial.Set(uint64(serialVal))
		serials = append(serials, &serial)
	}

	return wouldReplace, serials, nil
}
