package dgraph

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
)

// Import imports events from a reader (JSONL format)
func (d *D) Import(rr io.Reader) {
	d.ImportEventsFromReader(context.Background(), rr)
}

// Export exports events to a writer (JSONL format)
func (d *D) Export(c context.Context, w io.Writer, pubkeys ...[]byte) {
	// Build query based on whether pubkeys are specified
	var query string

	if len(pubkeys) > 0 {
		// Build pubkey filter
		pubkeyStrs := make([]string, len(pubkeys))
		for i, pk := range pubkeys {
			pubkeyStrs[i] = fmt.Sprintf("eq(event.pubkey, %q)", hex.Enc(pk))
		}
		pubkeyFilter := strings.Join(pubkeyStrs, " OR ")

		query = fmt.Sprintf(`{
			events(func: has(event.id)) @filter(%s) {
				event.id
				event.kind
				event.created_at
				event.content
				event.sig
				event.pubkey
				event.tags
			}
		}`, pubkeyFilter)
	} else {
		// Export all events
		query = `{
			events(func: has(event.id)) {
				event.id
				event.kind
				event.created_at
				event.content
				event.sig
				event.pubkey
				event.tags
			}
		}`
	}

	// Execute query
	resp, err := d.Query(c, query)
	if err != nil {
		d.Logger.Errorf("failed to query events for export: %v", err)
		fmt.Fprintf(w, "# Error: failed to query events: %v\n", err)
		return
	}

	// Parse events
	evs, err := d.parseEventsFromResponse(resp.Json)
	if err != nil {
		d.Logger.Errorf("failed to parse events for export: %v", err)
		fmt.Fprintf(w, "# Error: failed to parse events: %v\n", err)
		return
	}

	// Write header comment
	fmt.Fprintf(w, "# Exported %d events from dgraph\n", len(evs))

	// Write each event as JSONL
	count := 0
	for _, ev := range evs {
		jsonData, err := json.Marshal(ev)
		if err != nil {
			d.Logger.Warningf("failed to marshal event: %v", err)
			continue
		}

		if _, err := fmt.Fprintf(w, "%s\n", jsonData); err != nil {
			d.Logger.Errorf("failed to write event: %v", err)
			return
		}

		count++
		if count%1000 == 0 {
			d.Logger.Infof("exported %d events", count)
		}
	}

	d.Logger.Infof("export complete: %d events written", count)
}

// ImportEventsFromReader imports events from a reader
func (d *D) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
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
			d.Logger.Warningf("failed to parse event: %v", err)
			continue
		}

		// Save event
		if _, err := d.SaveEvent(ctx, ev); err != nil {
			d.Logger.Warningf("failed to import event: %v", err)
			continue
		}

		count++
		if count%1000 == 0 {
			d.Logger.Infof("imported %d events", count)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	d.Logger.Infof("import complete: %d events", count)
	return nil
}

// ImportEventsFromStrings imports events from JSON strings
func (d *D) ImportEventsFromStrings(
	ctx context.Context,
	eventJSONs []string,
	policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) },
) error {
	for _, eventJSON := range eventJSONs {
		ev := &event.E{}
		if err := json.Unmarshal([]byte(eventJSON), ev); err != nil {
			continue
		}

		// Check policy if manager is provided
		if policyManager != nil {
			if allowed, err := policyManager.CheckPolicy("write", ev, ev.Pubkey[:], "import"); err != nil || !allowed {
				continue
			}
		}

		// Save event
		if _, err := d.SaveEvent(ctx, ev); err != nil {
			d.Logger.Warningf("failed to import event: %v", err)
		}
	}

	return nil
}
