//go:build js && wasm

package wasmdb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"lol.mleku.dev/chk"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
)

// Import reads events from a JSONL reader and imports them into the database
func (w *W) Import(rr io.Reader) {
	ctx := context.Background()
	scanner := bufio.NewScanner(rr)
	// Increase buffer size for large events
	buf := make([]byte, 1024*1024) // 1MB buffer
	scanner.Buffer(buf, len(buf))

	imported := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		ev := event.New()
		if err := json.Unmarshal(line, ev); err != nil {
			w.Logger.Warnf("Import: failed to unmarshal event: %v", err)
			continue
		}

		if _, err := w.SaveEvent(ctx, ev); err != nil {
			w.Logger.Debugf("Import: failed to save event: %v", err)
			continue
		}
		imported++
	}

	if err := scanner.Err(); err != nil {
		w.Logger.Errorf("Import: scanner error: %v", err)
	}

	w.Logger.Infof("Import: imported %d events", imported)
}

// Export writes events to a JSONL writer, optionally filtered by pubkeys
func (w *W) Export(c context.Context, wr io.Writer, pubkeys ...[]byte) {
	var evs event.S
	var err error

	// Query events
	if len(pubkeys) > 0 {
		// Export only events from specified pubkeys
		for _, pk := range pubkeys {
			// Get all serials for this pubkey
			serials, err := w.GetSerialsByPubkey(pk)
			if err != nil {
				w.Logger.Warnf("Export: failed to get serials for pubkey: %v", err)
				continue
			}

			for _, ser := range serials {
				ev, err := w.FetchEventBySerial(ser)
				if err != nil || ev == nil {
					continue
				}
				evs = append(evs, ev)
			}
		}
	} else {
		// Export all events
		evs, err = w.getAllEvents(c)
		if err != nil {
			w.Logger.Errorf("Export: failed to get all events: %v", err)
			return
		}
	}

	// Write events as JSONL
	exported := 0
	for _, ev := range evs {
		data, err := json.Marshal(ev)
		if err != nil {
			w.Logger.Warnf("Export: failed to marshal event: %v", err)
			continue
		}
		wr.Write(data)
		wr.Write([]byte("\n"))
		exported++
	}

	w.Logger.Infof("Export: exported %d events", exported)
}

// ImportEventsFromReader imports events from a JSONL reader with context support
func (w *W) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	scanner := bufio.NewScanner(rr)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, len(buf))

	imported := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			w.Logger.Infof("ImportEventsFromReader: cancelled after %d events", imported)
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		ev := event.New()
		if err := json.Unmarshal(line, ev); err != nil {
			w.Logger.Warnf("ImportEventsFromReader: failed to unmarshal: %v", err)
			continue
		}

		if _, err := w.SaveEvent(ctx, ev); err != nil {
			w.Logger.Debugf("ImportEventsFromReader: failed to save: %v", err)
			continue
		}
		imported++
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	w.Logger.Infof("ImportEventsFromReader: imported %d events", imported)
	return nil
}

// ImportEventsFromStrings imports events from JSON strings with policy checking
func (w *W) ImportEventsFromStrings(
	ctx context.Context,
	eventJSONs []string,
	policyManager interface {
		CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error)
	},
) error {
	imported := 0

	for _, jsonStr := range eventJSONs {
		select {
		case <-ctx.Done():
			w.Logger.Infof("ImportEventsFromStrings: cancelled after %d events", imported)
			return ctx.Err()
		default:
		}

		ev := event.New()
		if err := json.Unmarshal([]byte(jsonStr), ev); err != nil {
			w.Logger.Warnf("ImportEventsFromStrings: failed to unmarshal: %v", err)
			continue
		}

		// Check policy if manager is provided
		if policyManager != nil {
			allowed, err := policyManager.CheckPolicy("write", ev, ev.Pubkey, "import")
			if err != nil || !allowed {
				w.Logger.Debugf("ImportEventsFromStrings: policy rejected event")
				continue
			}
		}

		if _, err := w.SaveEvent(ctx, ev); err != nil {
			w.Logger.Debugf("ImportEventsFromStrings: failed to save: %v", err)
			continue
		}
		imported++
	}

	w.Logger.Infof("ImportEventsFromStrings: imported %d events", imported)
	return nil
}

// GetSerialsByPubkey returns all event serials for a given pubkey
func (w *W) GetSerialsByPubkey(pubkey []byte) ([]*types.Uint40, error) {
	// Build range for pubkey index
	idx, err := database.GetIndexesFromFilter(&filter.F{
		Authors: tag.NewFromBytesSlice(pubkey),
	})
	if chk.E(err) {
		return nil, err
	}

	var serials []*types.Uint40
	for _, r := range idx {
		sers, err := w.GetSerialsByRange(r)
		if err != nil {
			continue
		}
		serials = append(serials, sers...)
	}

	return serials, nil
}

// getAllEvents retrieves all events from the database
func (w *W) getAllEvents(c context.Context) (event.S, error) {
	// Scan through the small event store and large event store
	var events event.S

	// Get events from small event store
	sevEvents, err := w.scanEventStore(string(indexes.SmallEventPrefix), true)
	if err == nil {
		events = append(events, sevEvents...)
	}

	// Get events from large event store
	evtEvents, err := w.scanEventStore(string(indexes.EventPrefix), false)
	if err == nil {
		events = append(events, evtEvents...)
	}

	return events, nil
}

// scanEventStore scans an event store and returns all events
func (w *W) scanEventStore(storeName string, isSmallEvent bool) (event.S, error) {
	tx, err := w.db.Transaction(idb.TransactionReadOnly, storeName)
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(storeName)
	if err != nil {
		return nil, err
	}

	var events event.S

	cursorReq, err := store.OpenCursor(idb.CursorNext)
	if err != nil {
		return nil, err
	}

	err = cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		var eventData []byte

		if isSmallEvent {
			// Small events: data is embedded in the key
			keyVal, keyErr := cursor.Key()
			if keyErr != nil {
				return keyErr
			}
			keyBytes := safeValueToBytes(keyVal)
			// Format: sev|serial|size_uint16|event_data
			if len(keyBytes) > 10 { // 3 + 5 + 2 minimum
				sizeOffset := 8 // 3 prefix + 5 serial
				if len(keyBytes) > sizeOffset+2 {
					size := int(keyBytes[sizeOffset])<<8 | int(keyBytes[sizeOffset+1])
					if len(keyBytes) >= sizeOffset+2+size {
						eventData = keyBytes[sizeOffset+2 : sizeOffset+2+size]
					}
				}
			}
		} else {
			// Large events: data is in the value
			val, valErr := cursor.Value()
			if valErr != nil {
				return valErr
			}
			eventData = safeValueToBytes(val)
		}

		if len(eventData) > 0 {
			ev := event.New()
			if err := ev.UnmarshalBinary(bytes.NewReader(eventData)); err == nil {
				events = append(events, ev)
			}
		}

		return cursor.Continue()
	})

	return events, err
}
