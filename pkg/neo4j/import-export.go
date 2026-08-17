package neo4j

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// Import imports events from a reader (JSONL format)
func (n *N) Import(rr io.Reader) {
	n.ImportEventsFromReader(context.Background(), rr)
}

// Export exports events to a writer (JSONL format)
// If pubkeys are provided, only exports events from those authors
// Otherwise exports all events
func (n *N) Export(c context.Context, w io.Writer, pubkeys ...[]byte) {
	var cypher string
	params := make(map[string]any)

	if len(pubkeys) > 0 {
		// Export events for specific pubkeys
		pubkeyStrings := make([]string, len(pubkeys))
		for i, pk := range pubkeys {
			pubkeyStrings[i] = hex.Enc(pk)
		}
		params["pubkeys"] = pubkeyStrings
		cypher = `
MATCH (e:Event)
WHERE e.pubkey IN $pubkeys
RETURN e.id AS id, e.kind AS kind, e.pubkey AS pubkey,
       e.created_at AS created_at, e.content AS content,
       e.sig AS sig, e.tags AS tags
ORDER BY e.created_at ASC`
	} else {
		// Export all events
		cypher = `
MATCH (e:Event)
RETURN e.id AS id, e.kind AS kind, e.pubkey AS pubkey,
       e.created_at AS created_at, e.content AS content,
       e.sig AS sig, e.tags AS tags
ORDER BY e.created_at ASC`
	}

	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		n.Logger.Warningf("failed to query events for export: %v", err)
		fmt.Fprintf(w, "# Export failed: %v\n", err)
		return
	}

	count := 0
	for result.Next(c) {
		record := result.Record()
		if record == nil {
			continue
		}

		// Build event from record
		ev := &event.E{}

		// Parse ID
		if idRaw, found := record.Get("id"); found {
			if idStr, ok := idRaw.(string); ok {
				if idBytes, err := hex.Dec(idStr); err == nil && len(idBytes) == 32 {
					copy(ev.ID[:], idBytes)
				}
			}
		}

		// Parse kind
		if kindRaw, found := record.Get("kind"); found {
			if kindVal, ok := kindRaw.(int64); ok {
				ev.Kind = uint16(kindVal)
			}
		}

		// Parse pubkey
		if pkRaw, found := record.Get("pubkey"); found {
			if pkStr, ok := pkRaw.(string); ok {
				if pkBytes, err := hex.Dec(pkStr); err == nil && len(pkBytes) == 32 {
					copy(ev.Pubkey[:], pkBytes)
				}
			}
		}

		// Parse created_at
		if tsRaw, found := record.Get("created_at"); found {
			if tsVal, ok := tsRaw.(int64); ok {
				ev.CreatedAt = tsVal
			}
		}

		// Parse content
		if contentRaw, found := record.Get("content"); found {
			if contentStr, ok := contentRaw.(string); ok {
				ev.Content = []byte(contentStr)
			}
		}

		// Parse sig
		if sigRaw, found := record.Get("sig"); found {
			if sigStr, ok := sigRaw.(string); ok {
				if sigBytes, err := hex.Dec(sigStr); err == nil && len(sigBytes) == 64 {
					copy(ev.Sig[:], sigBytes)
				}
			}
		}

		// Parse tags (stored as JSON string)
		if tagsRaw, found := record.Get("tags"); found {
			if tagsStr, ok := tagsRaw.(string); ok {
				ev.Tags = &tag.S{}
				if err := json.Unmarshal([]byte(tagsStr), ev.Tags); err != nil {
					n.Logger.Warningf("failed to unmarshal tags: %v", err)
				}
			}
		}

		// Write event as JSON line
		if evJSON, err := json.Marshal(ev); err == nil {
			fmt.Fprintf(w, "%s\n", evJSON)
			count++
		}
	}

	n.Logger.Infof("exported %d events", count)
}

// ImportEventsFromReader imports events from a reader
func (n *N) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	scanner := bufio.NewScanner(rr)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max line size

	count := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Skip comments
		if line[0] == '#' {
			continue
		}

		// Parse event
		ev := &event.E{}
		if err := json.Unmarshal(line, ev); err != nil {
			n.Logger.Warningf("failed to parse event: %v", err)
			continue
		}

		// Save event
		if _, err := n.SaveEvent(ctx, ev); err != nil {
			n.Logger.Warningf("failed to import event: %v", err)
			continue
		}

		count++
		if count%1000 == 0 {
			n.Logger.Infof("imported %d events", count)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	n.Logger.Infof("import complete: %d events", count)
	return nil
}

// ImportEventsFromStrings imports events from JSON strings
func (n *N) ImportEventsFromStrings(
	ctx context.Context,
	eventJSONs []string,
	policyManager interface {
		CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (allowed bool, reason string, err error)
	},
) error {
	for _, eventJSON := range eventJSONs {
		ev := &event.E{}
		if err := json.Unmarshal([]byte(eventJSON), ev); err != nil {
			continue
		}

		// Check policy if manager is provided
		if policyManager != nil {
			if allowed, _, err := policyManager.CheckPolicy("write", ev, ev.Pubkey[:], "import"); err != nil || !allowed {
				continue
			}
		}

		// Save event
		if _, err := n.SaveEvent(ctx, ev); err != nil {
			n.Logger.Warningf("failed to import event: %v", err)
		}
	}

	return nil
}
