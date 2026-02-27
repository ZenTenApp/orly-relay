package neo4j

import (
	"context"
	"fmt"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/database/indexes/types"
)

// DeleteEvent deletes an event by its ID
func (n *N) DeleteEvent(c context.Context, eid []byte) error {
	idStr := hex.Enc(eid)

	cypher := "MATCH (e:Event {id: $id}) DETACH DELETE e"
	params := map[string]any{"id": idStr}

	_, err := n.ExecuteWrite(c, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// DeleteEventBySerial deletes an event by its serial number
func (n *N) DeleteEventBySerial(c context.Context, ser *types.Uint40, ev *event.E) error {
	serial := ser.Get()

	cypher := "MATCH (e:Event {serial: $serial}) DETACH DELETE e"
	params := map[string]any{"serial": int64(serial)}

	_, err := n.ExecuteWrite(c, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	return nil
}

// DeleteExpired deletes expired events based on NIP-40 expiration tags
// Events with an expiration property > 0 and <= current time are deleted
func (n *N) DeleteExpired() {
	ctx := context.Background()
	now := time.Now().Unix()

	// Query for expired events (expiration > 0 means it has an expiration, and <= now means it's expired)
	cypher := `
MATCH (e:Event)
WHERE e.expiration > 0 AND e.expiration <= $now
RETURN e.serial AS serial, e.id AS id
LIMIT 1000`

	params := map[string]any{"now": now}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		n.Logger.Warningf("failed to query expired events: %v", err)
		return
	}

	// Collect serials to delete
	var deleteCount int
	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			continue
		}

		idRaw, found := record.Get("id")
		if !found {
			continue
		}

		idStr, ok := idRaw.(string)
		if !ok {
			continue
		}

		// Delete the expired event
		deleteCypher := "MATCH (e:Event {id: $id}) DETACH DELETE e"
		deleteParams := map[string]any{"id": idStr}

		if _, err := n.ExecuteWrite(ctx, deleteCypher, deleteParams); err != nil {
			n.Logger.Warningf("failed to delete expired event %s: %v", safePrefix(idStr, 16), err)
			continue
		}

		deleteCount++
	}

	if deleteCount > 0 {
		n.Logger.Infof("deleted %d expired events", deleteCount)
	}
}

// ProcessDelete processes a kind 5 deletion event
func (n *N) ProcessDelete(ev *event.E, admins [][]byte) error {
	// Deletion events (kind 5) can delete events by the same author
	// or by relay admins

	// Check if this is a kind 5 event
	if ev.Kind != 5 {
		return fmt.Errorf("not a deletion event")
	}

	// Get all 'e' tags (event IDs to delete)
	eTags := ev.Tags.GetAll([]byte{'e'})
	if len(eTags) == 0 {
		return nil // Nothing to delete
	}

	ctx := context.Background()
	isAdmin := false

	// Check if author is an admin
	for _, adminPk := range admins {
		if string(ev.Pubkey) == string(adminPk) {
			isAdmin = true
			break
		}
	}

	// For each event ID in e-tags, delete it if allowed
	for _, eTag := range eTags {
		if len(eTag.T) < 2 {
			continue
		}

		// Use ValueHex() to correctly handle both binary and hex storage formats
		eventIDStr := string(eTag.ValueHex())
		eventID, err := hex.Dec(eventIDStr)
		if err != nil {
			continue
		}

		// Fetch the event to check authorship
		cypher := "MATCH (e:Event {id: $id}) RETURN e.pubkey AS pubkey"
		params := map[string]any{"id": eventIDStr}

		result, err := n.ExecuteRead(ctx, cypher, params)
		if err != nil {
			continue
		}

		if result.Next(ctx) {
			record := result.Record()
			if record != nil {
				pubkeyValue, found := record.Get("pubkey")
				if found {
					if pubkeyStr, ok := pubkeyValue.(string); ok {
						pubkey, err := hex.Dec(pubkeyStr)
						if err != nil {
							continue
						}

						// Check if deletion is allowed (same author or admin)
						canDelete := isAdmin || string(ev.Pubkey) == string(pubkey)
						if canDelete {
							// Delete the event
							if err := n.DeleteEvent(ctx, eventID); err != nil {
								n.Logger.Warningf("failed to delete event %s: %v", eventIDStr, err)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// CheckForDeleted checks if an event has been deleted
func (n *N) CheckForDeleted(ev *event.E, admins [][]byte) error {
	// Query for kind 5 events that reference this event via Tag nodes
	ctx := context.Background()
	idStr := hex.Enc(ev.ID[:])

	// Build cypher query to find deletion events
	// Traverses through Tag nodes: Event-[:TAGGED_WITH]->Tag-[:REFERENCES]->Event
	cypher := `
MATCH (target:Event {id: $targetId})
MATCH (delete:Event {kind: 5})-[:TAGGED_WITH]->(t:Tag {type: 'e'})-[:REFERENCES]->(target)
WHERE delete.pubkey = $pubkey OR delete.pubkey IN $admins
RETURN delete.id AS id
LIMIT 1`

	adminPubkeys := make([]string, len(admins))
	for i, admin := range admins {
		adminPubkeys[i] = hex.Enc(admin)
	}

	params := map[string]any{
		"targetId": idStr,
		"pubkey":   hex.Enc(ev.Pubkey[:]),
		"admins":   adminPubkeys,
	}

	result, err := n.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil // Not deleted
	}

	if result.Next(ctx) {
		return fmt.Errorf("event has been deleted")
	}

	return nil
}
