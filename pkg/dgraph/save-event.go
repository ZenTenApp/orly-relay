package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgraph-io/dgo/v230/protos/api"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
)

// SaveEvent stores a Nostr event in the Dgraph database.
// It creates event nodes and relationships for authors, tags, and references.
func (d *D) SaveEvent(c context.Context, ev *event.E) (exists bool, err error) {
	eventID := hex.Enc(ev.ID[:])

	// Check if event already exists
	query := fmt.Sprintf(`{
		event(func: eq(event.id, %q)) {
			uid
			event.id
		}
	}`, eventID)

	resp, err := d.Query(c, query)
	if err != nil {
		return false, fmt.Errorf("failed to check event existence: %w", err)
	}

	// Parse response to check if event exists
	var result struct {
		Event []map[string]interface{} `json:"event"`
	}
	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return false, fmt.Errorf("failed to parse query response: %w", err)
	}

	if len(result.Event) > 0 {
		return true, nil // Event already exists
	}

	// Get next serial number
	serial, err := d.getNextSerial()
	if err != nil {
		return false, fmt.Errorf("failed to get serial number: %w", err)
	}

	// Build N-Quads for the event with serial number
	nquads := d.buildEventNQuads(ev, serial)

	// Store the event
	mutation := &api.Mutation{
		SetNquads: []byte(nquads),
		CommitNow: true,
	}

	if _, err = d.Mutate(c, mutation); err != nil {
		return false, fmt.Errorf("failed to save event: %w", err)
	}

	return false, nil
}

// buildEventNQuads constructs RDF triples for a Nostr event
func (d *D) buildEventNQuads(ev *event.E, serial uint64) string {
	var nquads strings.Builder

	eventID := hex.Enc(ev.ID[:])
	authorPubkey := hex.Enc(ev.Pubkey)

	// Event node
	nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Event\" .\n", eventID))
	nquads.WriteString(fmt.Sprintf("_:%s <event.id> %q .\n", eventID, eventID))
	nquads.WriteString(fmt.Sprintf("_:%s <event.serial> \"%d\"^^<xs:int> .\n", eventID, serial))
	nquads.WriteString(fmt.Sprintf("_:%s <event.kind> \"%d\"^^<xs:int> .\n", eventID, ev.Kind))
	nquads.WriteString(fmt.Sprintf("_:%s <event.created_at> \"%d\"^^<xs:int> .\n", eventID, int64(ev.CreatedAt)))
	nquads.WriteString(fmt.Sprintf("_:%s <event.content> %q .\n", eventID, ev.Content))
	nquads.WriteString(fmt.Sprintf("_:%s <event.sig> %q .\n", eventID, hex.Enc(ev.Sig[:])))
	nquads.WriteString(fmt.Sprintf("_:%s <event.pubkey> %q .\n", eventID, authorPubkey))

	// Serialize tags as JSON string for storage
	tagsJSON, _ := json.Marshal(ev.Tags)
	nquads.WriteString(fmt.Sprintf("_:%s <event.tags> %q .\n", eventID, string(tagsJSON)))

	// Author relationship
	nquads.WriteString(fmt.Sprintf("_:%s <authored_by> _:%s .\n", eventID, authorPubkey))
	nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Author\" .\n", authorPubkey))
	nquads.WriteString(fmt.Sprintf("_:%s <author.pubkey> %q .\n", authorPubkey, authorPubkey))

	// Tag relationships
	for _, tag := range *ev.Tags {
		if len(tag.T) >= 2 {
			tagType := string(tag.T[0])
			tagValue := string(tag.T[1])

			switch tagType {
			case "e": // Event reference
				nquads.WriteString(fmt.Sprintf("_:%s <references> _:%s .\n", eventID, tagValue))
			case "p": // Pubkey mention
				nquads.WriteString(fmt.Sprintf("_:%s <mentions> _:%s .\n", eventID, tagValue))
				// Ensure mentioned author exists
				nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Author\" .\n", tagValue))
				nquads.WriteString(fmt.Sprintf("_:%s <author.pubkey> %q .\n", tagValue, tagValue))
			case "t": // Hashtag
				tagID := "tag_" + tagType + "_" + tagValue
				nquads.WriteString(fmt.Sprintf("_:%s <tagged_with> _:%s .\n", eventID, tagID))
				nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Tag\" .\n", tagID))
				nquads.WriteString(fmt.Sprintf("_:%s <tag.type> %q .\n", tagID, tagType))
				nquads.WriteString(fmt.Sprintf("_:%s <tag.value> %q .\n", tagID, tagValue))
			default:
				// Store other tag types
				tagID := "tag_" + tagType + "_" + tagValue
				nquads.WriteString(fmt.Sprintf("_:%s <tagged_with> _:%s .\n", eventID, tagID))
				nquads.WriteString(fmt.Sprintf("_:%s <dgraph.type> \"Tag\" .\n", tagID))
				nquads.WriteString(fmt.Sprintf("_:%s <tag.type> %q .\n", tagID, tagType))
				nquads.WriteString(fmt.Sprintf("_:%s <tag.value> %q .\n", tagID, tagValue))
			}
		}
	}

	return nquads.String()
}

// GetSerialsFromFilter returns event serials matching a filter
func (d *D) GetSerialsFromFilter(f *filter.F) (serials types.Uint40s, err error) {
	// Use QueryForSerials which already implements the proper filter logic
	return d.QueryForSerials(context.Background(), f)
}

// WouldReplaceEvent checks if an event would replace existing events
func (d *D) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	// Check for replaceable events (kinds 0, 3, and 10000-19999)
	isReplaceable := ev.Kind == 0 || ev.Kind == 3 || (ev.Kind >= 10000 && ev.Kind < 20000)
	if !isReplaceable {
		return false, nil, nil
	}

	// Query for existing events with same kind and pubkey
	authorPubkey := hex.Enc(ev.Pubkey)
	query := fmt.Sprintf(`{
		events(func: eq(event.pubkey, %q)) @filter(eq(event.kind, %d)) {
			uid
			event.serial
			event.created_at
		}
	}`, authorPubkey, ev.Kind)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return false, nil, fmt.Errorf("failed to query replaceable events: %w", err)
	}

	var result struct {
		Events []struct {
			UID       string `json:"uid"`
			Serial    int64  `json:"event.serial"`
			CreatedAt int64  `json:"event.created_at"`
		} `json:"events"`
	}
	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return false, nil, fmt.Errorf("failed to parse query response: %w", err)
	}

	// Check if our event is newer
	evTime := int64(ev.CreatedAt)
	var serials types.Uint40s
	wouldReplace := false

	for _, existing := range result.Events {
		if existing.CreatedAt < evTime {
			wouldReplace = true
			serial := types.Uint40{}
			serial.Set(uint64(existing.Serial))
			serials = append(serials, &serial)
		}
	}

	return wouldReplace, serials, nil
}
