//go:build js && wasm

package wasmdb

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"github.com/hack-pad/safejs"
	"lol.mleku.dev/chk"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
)

var (
	// ErrOlderThanExisting is returned when a candidate event is older than an existing replaceable/addressable event.
	ErrOlderThanExisting = errors.New("older than existing event")
	// ErrMissingDTag is returned when a parameterized replaceable event lacks the required 'd' tag.
	ErrMissingDTag = errors.New("event is missing a d tag identifier")
)

// SaveEvent saves an event to the database, generating all necessary indexes.
func (w *W) SaveEvent(c context.Context, ev *event.E) (replaced bool, err error) {
	if ev == nil {
		err = errors.New("nil event")
		return
	}

	// Reject ephemeral events (kinds 20000-29999) - they should never be stored
	if ev.Kind >= 20000 && ev.Kind <= 29999 {
		err = errors.New("blocked: ephemeral events should not be stored")
		return
	}

	// Validate kind 3 (follow list) events have at least one p tag
	if ev.Kind == 3 {
		hasPTag := false
		if ev.Tags != nil {
			for _, t := range *ev.Tags {
				if t != nil && t.Len() >= 2 {
					key := t.Key()
					if len(key) == 1 && key[0] == 'p' {
						hasPTag = true
						break
					}
				}
			}
		}
		if !hasPTag {
			w.Logger.Warnf("SaveEvent: rejecting kind 3 event without p tags from pubkey %x", ev.Pubkey)
			err = errors.New("blocked: kind 3 follow list events must have at least one p tag")
			return
		}
	}

	// Check if the event already exists
	var ser *types.Uint40
	if ser, err = w.GetSerialById(ev.ID); err == nil && ser != nil {
		err = errors.New("blocked: event already exists: " + hex.Enc(ev.ID[:]))
		return
	}

	// If the error is "id not found", we can proceed
	if err != nil && strings.Contains(err.Error(), "id not found") {
		err = nil
	} else if err != nil {
		return
	}

	// Check for replacement - only validate, don't delete old events
	if kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind) {
		var werr error
		if replaced, _, werr = w.WouldReplaceEvent(ev); werr != nil {
			if errors.Is(werr, ErrOlderThanExisting) {
				if kind.IsReplaceable(ev.Kind) {
					err = errors.New("blocked: event is older than existing replaceable event")
				} else {
					err = errors.New("blocked: event is older than existing addressable event")
				}
				return
			}
			if errors.Is(werr, ErrMissingDTag) {
				err = ErrMissingDTag
				return
			}
			return
		}
	}

	// Get the next sequence number for the event
	serial, err := w.nextEventSerial()
	if err != nil {
		return
	}

	// Generate all indexes for the event
	idxs, err := database.GetIndexesForEvent(ev, serial)
	if err != nil {
		return
	}

	// Serialize event to binary
	eventDataBuf := new(bytes.Buffer)
	ev.MarshalBinary(eventDataBuf)
	eventData := eventDataBuf.Bytes()

	// Determine storage strategy
	smallEventThreshold := 1024 // Could be made configurable
	isSmallEvent := len(eventData) <= smallEventThreshold
	isReplaceableEvent := kind.IsReplaceable(ev.Kind)
	isAddressableEvent := kind.IsParameterizedReplaceable(ev.Kind)

	// Create serial type
	ser = new(types.Uint40)
	if err = ser.Set(serial); chk.E(err) {
		return
	}

	// Start a transaction to save the event and all its indexes
	// We need to include all object stores we'll write to
	storesToWrite := []string{
		string(indexes.IdPrefix),
		string(indexes.FullIdPubkeyPrefix),
		string(indexes.CreatedAtPrefix),
		string(indexes.PubkeyPrefix),
		string(indexes.KindPrefix),
		string(indexes.KindPubkeyPrefix),
		string(indexes.TagPrefix),
		string(indexes.TagKindPrefix),
		string(indexes.TagPubkeyPrefix),
		string(indexes.TagKindPubkeyPrefix),
		string(indexes.WordPrefix),
	}

	// Add event storage store
	if isSmallEvent {
		storesToWrite = append(storesToWrite, string(indexes.SmallEventPrefix))
	} else {
		storesToWrite = append(storesToWrite, string(indexes.EventPrefix))
	}

	// Add specialized stores if needed
	if isAddressableEvent && isSmallEvent {
		storesToWrite = append(storesToWrite, string(indexes.AddressableEventPrefix))
	} else if isReplaceableEvent && isSmallEvent {
		storesToWrite = append(storesToWrite, string(indexes.ReplaceableEventPrefix))
	}

	// Start transaction
	tx, err := w.db.Transaction(idb.TransactionReadWrite, storesToWrite[0], storesToWrite[1:]...)
	if err != nil {
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}

	// Save each index to its respective object store
	for _, key := range idxs {
		if len(key) < 3 {
			continue
		}
		// Extract store name from 3-byte prefix
		storeName := string(key[:3])

		store, storeErr := tx.ObjectStore(storeName)
		if storeErr != nil {
			w.Logger.Warnf("SaveEvent: failed to get object store %s: %v", storeName, storeErr)
			continue
		}

		// Use the full key as the IndexedDB key, empty value
		keyJS := bytesToSafeValue(key)
		_, putErr := store.PutKey(keyJS, safejs.Null())
		if putErr != nil {
			w.Logger.Warnf("SaveEvent: failed to put index %s: %v", storeName, putErr)
		}
	}

	// Store the event data
	if isSmallEvent {
		// Small event: store inline with sev prefix
		// Format: sev|serial|size_uint16|event_data
		keyBuf := new(bytes.Buffer)
		if err = indexes.SmallEventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
			return
		}
		// Append size as uint16 big-endian
		sizeBytes := []byte{byte(len(eventData) >> 8), byte(len(eventData))}
		keyBuf.Write(sizeBytes)
		keyBuf.Write(eventData)

		store, storeErr := tx.ObjectStore(string(indexes.SmallEventPrefix))
		if storeErr == nil {
			keyJS := bytesToSafeValue(keyBuf.Bytes())
			store.PutKey(keyJS, safejs.Null())
		}
	} else {
		// Large event: store separately with evt prefix
		keyBuf := new(bytes.Buffer)
		if err = indexes.EventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
			return
		}

		store, storeErr := tx.ObjectStore(string(indexes.EventPrefix))
		if storeErr == nil {
			keyJS := bytesToSafeValue(keyBuf.Bytes())
			valueJS := bytesToSafeValue(eventData)
			store.PutKey(keyJS, valueJS)
		}
	}

	// Store specialized keys for replaceable/addressable events
	if isAddressableEvent && isSmallEvent {
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag != nil {
			pubHash := new(types.PubHash)
			pubHash.FromPubkey(ev.Pubkey)
			kindVal := new(types.Uint16)
			kindVal.Set(ev.Kind)
			dTagHash := new(types.Ident)
			dTagHash.FromIdent(dTag.Value())

			keyBuf := new(bytes.Buffer)
			if err = indexes.AddressableEventEnc(pubHash, kindVal, dTagHash).MarshalWrite(keyBuf); chk.E(err) {
				return
			}
			sizeBytes := []byte{byte(len(eventData) >> 8), byte(len(eventData))}
			keyBuf.Write(sizeBytes)
			keyBuf.Write(eventData)

			store, storeErr := tx.ObjectStore(string(indexes.AddressableEventPrefix))
			if storeErr == nil {
				keyJS := bytesToSafeValue(keyBuf.Bytes())
				store.PutKey(keyJS, safejs.Null())
			}
		}
	} else if isReplaceableEvent && isSmallEvent {
		pubHash := new(types.PubHash)
		pubHash.FromPubkey(ev.Pubkey)
		kindVal := new(types.Uint16)
		kindVal.Set(ev.Kind)

		keyBuf := new(bytes.Buffer)
		if err = indexes.ReplaceableEventEnc(pubHash, kindVal).MarshalWrite(keyBuf); chk.E(err) {
			return
		}
		sizeBytes := []byte{byte(len(eventData) >> 8), byte(len(eventData))}
		keyBuf.Write(sizeBytes)
		keyBuf.Write(eventData)

		store, storeErr := tx.ObjectStore(string(indexes.ReplaceableEventPrefix))
		if storeErr == nil {
			keyJS := bytesToSafeValue(keyBuf.Bytes())
			store.PutKey(keyJS, safejs.Null())
		}
	}

	// Commit transaction
	if err = tx.Await(c); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	w.Logger.Debugf("SaveEvent: saved event %x (kind %d, %d bytes, %d indexes)",
		ev.ID[:8], ev.Kind, len(eventData), len(idxs))

	return
}

// WouldReplaceEvent checks if the provided event would replace existing events
func (w *W) WouldReplaceEvent(ev *event.E) (bool, types.Uint40s, error) {
	// Only relevant for replaceable or parameterized replaceable kinds
	if !(kind.IsReplaceable(ev.Kind) || kind.IsParameterizedReplaceable(ev.Kind)) {
		return false, nil, nil
	}

	// Build filter for existing events
	var f interface{}
	if kind.IsReplaceable(ev.Kind) {
		// For now, simplified check - would need full filter implementation
		return false, nil, nil
	} else {
		// Parameterized replaceable requires 'd' tag
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag == nil {
			return false, nil, ErrMissingDTag
		}
		// Simplified - full implementation would query existing events
		_ = f
	}

	// Simplified implementation - assume no conflicts for now
	// Full implementation would query the database and compare timestamps
	return false, nil, nil
}

// GetSerialById looks up the serial number for an event ID
func (w *W) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	if len(id) != 32 {
		return nil, errors.New("invalid event ID length")
	}

	// Create ID hash
	idHash := new(types.IdHash)
	if err = idHash.FromId(id); chk.E(err) {
		return nil, err
	}

	// Build the prefix to search for
	keyBuf := new(bytes.Buffer)
	indexes.IdEnc(idHash, nil).MarshalWrite(keyBuf)
	prefix := keyBuf.Bytes()[:11] // 3 prefix + 8 id hash

	// Search in the eid object store
	tx, err := w.db.Transaction(idb.TransactionReadOnly, string(indexes.IdPrefix))
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(string(indexes.IdPrefix))
	if err != nil {
		return nil, err
	}

	// Use cursor to find matching key
	cursorReq, err := store.OpenCursor(idb.CursorNext)
	if err != nil {
		return nil, err
	}

	err = cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		keyBytes := safeValueToBytes(keyVal)
		if len(keyBytes) >= len(prefix) && bytes.HasPrefix(keyBytes, prefix) {
			// Found matching key, extract serial from last 5 bytes
			if len(keyBytes) >= 16 { // 3 + 8 + 5
				ser = new(types.Uint40)
				ser.UnmarshalRead(bytes.NewReader(keyBytes[11:16]))
				return errors.New("found") // Stop iteration
			}
		}

		return cursor.Continue()
	})

	if ser != nil {
		return ser, nil
	}
	if err != nil && err.Error() != "found" {
		return nil, err
	}

	return nil, errors.New("id not found in database")
}

// GetSerialsByIds looks up serial numbers for multiple event IDs
func (w *W) GetSerialsByIds(ids *tag.T) (serials map[string]*types.Uint40, err error) {
	serials = make(map[string]*types.Uint40)

	if ids == nil {
		return
	}

	for i := 1; i < ids.Len(); i++ {
		idBytes := ids.T[i]
		if len(idBytes) == 64 {
			// Hex encoded ID
			var decoded []byte
			decoded, err = hex.Dec(string(idBytes))
			if err != nil {
				continue
			}
			idBytes = decoded
		}

		if len(idBytes) == 32 {
			var ser *types.Uint40
			ser, err = w.GetSerialById(idBytes)
			if err == nil && ser != nil {
				serials[hex.Enc(idBytes)] = ser
			}
		}
	}

	err = nil
	return
}

// GetSerialsByIdsWithFilter looks up serial numbers with a filter function
func (w *W) GetSerialsByIdsWithFilter(ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool) (serials map[string]*types.Uint40, err error) {
	allSerials, err := w.GetSerialsByIds(ids)
	if err != nil {
		return nil, err
	}

	if fn == nil {
		return allSerials, nil
	}

	serials = make(map[string]*types.Uint40)
	for idHex, ser := range allSerials {
		ev, fetchErr := w.FetchEventBySerial(ser)
		if fetchErr != nil {
			continue
		}
		if fn(ev, ser) {
			serials[idHex] = ser
		}
	}

	return serials, nil
}
