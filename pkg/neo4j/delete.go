package neo4j

import (
	"context"
	"fmt"

	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
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

// DeleteExpired deletes expired events (stub implementation)
func (n *N) DeleteExpired() {
	// This would need to implement expiration logic based on event.expiration tag (NIP-40)
	// For now, this is a no-op
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
		if string(ev.Pubkey[:]) == string(adminPk) {
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
						canDelete := isAdmin || string(ev.Pubkey[:]) == string(pubkey)
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
	// Query for kind 5 events that reference this event
	ctx := context.Background()
	idStr := hex.Enc(ev.ID[:])

	// Build cypher query to find deletion events
	cypher := `
MATCH (target:Event {id: $targetId})
MATCH (delete:Event {kind: 5})-[:REFERENCES]->(target)
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
