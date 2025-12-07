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

// tagBatchSize is the maximum number of tags to process in a single transaction
// This prevents Neo4j stack overflow errors with events that have thousands of tags
const tagBatchSize = 500

// SaveEvent stores a Nostr event in the Neo4j database.
// It creates event nodes and relationships for authors, tags, and references.
// This method leverages Neo4j's graph capabilities to model Nostr's social graph naturally.
//
// For social graph events (kinds 0, 3, 1984, 10000), it additionally processes them
// to maintain NostrUser nodes and FOLLOWS/MUTES/REPORTS relationships with event traceability.
//
// To prevent Neo4j stack overflow errors with events containing thousands of tags,
// tags are processed in batches using UNWIND instead of generating inline Cypher.
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
				n.Logger.Warningf("failed to reprocess social event %s: %v", safePrefix(eventID, 16), err)
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

	// Step 1: Create base event with author (small, fixed-size query)
	cypher, params := n.buildBaseEventCypher(ev, serial)
	if _, err = n.ExecuteWrite(c, cypher, params); err != nil {
		return false, fmt.Errorf("failed to save event: %w", err)
	}

	// Step 2: Process tags in batches to avoid stack overflow
	if ev.Tags != nil {
		if err := n.addTagsInBatches(c, eventID, ev); err != nil {
			// Log but don't fail - base event is saved, tags are supplementary for queries
			n.Logger.Errorf("failed to add tags for event %s: %v", safePrefix(eventID, 16), err)
		}
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
				ev.Kind, safePrefix(eventID, 16), err)
			// Consider: should we fail here or continue?
			// For now, continue - social graph is supplementary to base relay
		}
	}

	return false, nil
}

// safePrefix returns up to n characters from a string, handling short strings gracefully
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// buildBaseEventCypher constructs a Cypher query to create just the base event node and author.
// Tags are added separately in batches to prevent stack overflow with large tag sets.
// This creates:
// - Event node with all properties
// - NostrUser node and AUTHORED_BY relationship (unified author + WoT node)
func (n *N) buildBaseEventCypher(ev *event.E, serial uint64) (string, map[string]any) {
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

	// Build Cypher query - just event + author, no tags (tags added in batches)
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

RETURN e.id AS id`

	return cypher, params
}

// tagTypeValue represents a generic tag with type and value for batch processing
type tagTypeValue struct {
	Type  string
	Value string
}

// addTagsInBatches processes event tags in batches using UNWIND to prevent Neo4j stack overflow.
// This handles e-tags (event references), p-tags (pubkey mentions), and other tags separately.
func (n *N) addTagsInBatches(c context.Context, eventID string, ev *event.E) error {
	if ev.Tags == nil {
		return nil
	}

	// Collect tags by type
	var eTags, pTags []string
	var otherTags []tagTypeValue

	for _, tagItem := range *ev.Tags {
		if len(tagItem.T) < 2 {
			continue
		}

		tagType := string(tagItem.T[0])

		switch tagType {
		case "e": // Event reference
			tagValue := ExtractETagValue(tagItem)
			if tagValue != "" {
				eTags = append(eTags, tagValue)
			}
		case "p": // Pubkey mention
			tagValue := ExtractPTagValue(tagItem)
			if tagValue != "" {
				pTags = append(pTags, tagValue)
			}
		default: // Other tags
			tagValue := string(tagItem.T[1])
			otherTags = append(otherTags, tagTypeValue{Type: tagType, Value: tagValue})
		}
	}

	// Add p-tags in batches (creates MENTIONS relationships)
	if len(pTags) > 0 {
		if err := n.addPTagsInBatches(c, eventID, pTags); err != nil {
			return fmt.Errorf("failed to add p-tags: %w", err)
		}
	}

	// Add e-tags in batches (creates REFERENCES relationships)
	if len(eTags) > 0 {
		if err := n.addETagsInBatches(c, eventID, eTags); err != nil {
			return fmt.Errorf("failed to add e-tags: %w", err)
		}
	}

	// Add other tags in batches (creates TAGGED_WITH relationships)
	if len(otherTags) > 0 {
		if err := n.addOtherTagsInBatches(c, eventID, otherTags); err != nil {
			return fmt.Errorf("failed to add other tags: %w", err)
		}
	}

	return nil
}

// addPTagsInBatches adds p-tag (pubkey mention) relationships using UNWIND for efficiency.
// Creates NostrUser nodes for mentioned pubkeys and MENTIONS relationships.
func (n *N) addPTagsInBatches(c context.Context, eventID string, pTags []string) error {
	// Process in batches to avoid memory issues
	for i := 0; i < len(pTags); i += tagBatchSize {
		end := i + tagBatchSize
		if end > len(pTags) {
			end = len(pTags)
		}
		batch := pTags[i:end]

		// Use UNWIND to process multiple p-tags in a single query
		cypher := `
MATCH (e:Event {id: $eventId})
UNWIND $pubkeys AS pubkey
MERGE (u:NostrUser {pubkey: pubkey})
ON CREATE SET u.created_at = timestamp()
CREATE (e)-[:MENTIONS]->(u)`

		params := map[string]any{
			"eventId": eventID,
			"pubkeys": batch,
		}

		if _, err := n.ExecuteWrite(c, cypher, params); err != nil {
			return fmt.Errorf("batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// addETagsInBatches adds e-tag (event reference) relationships using UNWIND for efficiency.
// Only creates REFERENCES relationships if the referenced event exists.
func (n *N) addETagsInBatches(c context.Context, eventID string, eTags []string) error {
	// Process in batches to avoid memory issues
	for i := 0; i < len(eTags); i += tagBatchSize {
		end := i + tagBatchSize
		if end > len(eTags) {
			end = len(eTags)
		}
		batch := eTags[i:end]

		// Use UNWIND to process multiple e-tags in a single query
		// OPTIONAL MATCH ensures we only create relationships if referenced event exists
		cypher := `
MATCH (e:Event {id: $eventId})
UNWIND $eventIds AS refId
OPTIONAL MATCH (ref:Event {id: refId})
WITH e, ref
WHERE ref IS NOT NULL
CREATE (e)-[:REFERENCES]->(ref)`

		params := map[string]any{
			"eventId":  eventID,
			"eventIds": batch,
		}

		if _, err := n.ExecuteWrite(c, cypher, params); err != nil {
			return fmt.Errorf("batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// addOtherTagsInBatches adds generic tag relationships using UNWIND for efficiency.
// Creates Tag nodes with type and value, and TAGGED_WITH relationships.
func (n *N) addOtherTagsInBatches(c context.Context, eventID string, tags []tagTypeValue) error {
	// Process in batches to avoid memory issues
	for i := 0; i < len(tags); i += tagBatchSize {
		end := i + tagBatchSize
		if end > len(tags) {
			end = len(tags)
		}
		batch := tags[i:end]

		// Convert to map slice for Neo4j parameter passing
		tagMaps := make([]map[string]string, len(batch))
		for j, t := range batch {
			tagMaps[j] = map[string]string{"type": t.Type, "value": t.Value}
		}

		// Use UNWIND to process multiple tags in a single query
		cypher := `
MATCH (e:Event {id: $eventId})
UNWIND $tags AS tag
MERGE (t:Tag {type: tag.type, value: tag.value})
CREATE (e)-[:TAGGED_WITH]->(t)`

		params := map[string]any{
			"eventId": eventID,
			"tags":    tagMaps,
		}

		if _, err := n.ExecuteWrite(c, cypher, params); err != nil {
			return fmt.Errorf("batch %d-%d: %w", i, end, err)
		}
	}

	return nil
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
