//go:build js && wasm

package wasmdb

import (
	"bytes"
	"errors"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"git.smesh.lol/orly/pkg/lol/chk"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/database/indexes"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/interfaces/store"
)

// FetchEventBySerial retrieves an event by its serial number
func (w *W) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	if ser == nil {
		return nil, errors.New("nil serial")
	}

	// First try small event store (sev prefix)
	ev, err = w.fetchSmallEvent(ser)
	if err == nil && ev != nil {
		return ev, nil
	}

	// Then try large event store (evt prefix)
	ev, err = w.fetchLargeEvent(ser)
	if err == nil && ev != nil {
		return ev, nil
	}

	return nil, errors.New("event not found")
}

// fetchSmallEvent fetches an event from the small event store
func (w *W) fetchSmallEvent(ser *types.Uint40) (*event.E, error) {
	// Build the key prefix
	keyBuf := new(bytes.Buffer)
	if err := indexes.SmallEventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
		return nil, err
	}
	prefix := keyBuf.Bytes()

	// Open transaction
	tx, err := w.db.Transaction(idb.TransactionReadOnly, string(indexes.SmallEventPrefix))
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(string(indexes.SmallEventPrefix))
	if err != nil {
		return nil, err
	}

	// Use cursor to find matching key
	cursorReq, err := store.OpenCursor(idb.CursorNext)
	if err != nil {
		return nil, err
	}

	var foundEvent *event.E
	err = cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		keyBytes := safeValueToBytes(keyVal)
		if len(keyBytes) >= len(prefix) && bytes.HasPrefix(keyBytes, prefix) {
			// Found matching key
			// Format: sev|serial(5)|size(2)|data(variable)
			if len(keyBytes) > 10 { // 3 + 5 + 2 = 10 minimum
				sizeOffset := 8 // 3 prefix + 5 serial
				if len(keyBytes) > sizeOffset+2 {
					size := int(keyBytes[sizeOffset])<<8 | int(keyBytes[sizeOffset+1])
					dataStart := sizeOffset + 2
					if len(keyBytes) >= dataStart+size {
						eventData := keyBytes[dataStart : dataStart+size]
						ev := new(event.E)
						if unmarshalErr := ev.UnmarshalBinary(bytes.NewReader(eventData)); unmarshalErr == nil {
							foundEvent = ev
							return errors.New("found") // Stop iteration
						}
					}
				}
			}
		}

		return cursor.Continue()
	})

	if foundEvent != nil {
		return foundEvent, nil
	}
	if err != nil && err.Error() != "found" {
		return nil, err
	}

	return nil, errors.New("small event not found")
}

// fetchLargeEvent fetches an event from the large event store
func (w *W) fetchLargeEvent(ser *types.Uint40) (*event.E, error) {
	// Build the key
	keyBuf := new(bytes.Buffer)
	if err := indexes.EventEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
		return nil, err
	}

	// Open transaction
	tx, err := w.db.Transaction(idb.TransactionReadOnly, string(indexes.EventPrefix))
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(string(indexes.EventPrefix))
	if err != nil {
		return nil, err
	}

	// Get the value directly
	keyJS := bytesToSafeValue(keyBuf.Bytes())
	req, err := store.Get(keyJS)
	if err != nil {
		return nil, err
	}

	val, err := req.Await(w.ctx)
	if err != nil {
		return nil, err
	}

	if val.IsUndefined() || val.IsNull() {
		return nil, errors.New("large event not found")
	}

	eventData := safeValueToBytes(val)
	if len(eventData) == 0 {
		return nil, errors.New("empty event data")
	}

	ev := new(event.E)
	if err := ev.UnmarshalBinary(bytes.NewReader(eventData)); err != nil {
		return nil, err
	}

	return ev, nil
}

// FetchEventsBySerials retrieves multiple events by their serial numbers
func (w *W) FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error) {
	events = make(map[uint64]*event.E)

	for _, ser := range serials {
		if ser == nil {
			continue
		}
		ev, fetchErr := w.FetchEventBySerial(ser)
		if fetchErr == nil && ev != nil {
			events[ser.Get()] = ev
		}
	}

	return events, nil
}

// GetFullIdPubkeyBySerial retrieves the ID, pubkey hash, and timestamp for a serial
func (w *W) GetFullIdPubkeyBySerial(ser *types.Uint40) (fidpk *store.IdPkTs, err error) {
	if ser == nil {
		return nil, errors.New("nil serial")
	}

	// Build the prefix to search for
	keyBuf := new(bytes.Buffer)
	indexes.FullIdPubkeyEnc(ser, nil, nil, nil).MarshalWrite(keyBuf)
	prefix := keyBuf.Bytes()[:8] // 3 prefix + 5 serial

	// Search in the fpc object store
	tx, err := w.db.Transaction(idb.TransactionReadOnly, string(indexes.FullIdPubkeyPrefix))
	if err != nil {
		return nil, err
	}

	objStore, err := tx.ObjectStore(string(indexes.FullIdPubkeyPrefix))
	if err != nil {
		return nil, err
	}

	// Use cursor to find matching key
	cursorReq, err := objStore.OpenCursor(idb.CursorNext)
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
			// Found matching key
			// Format: fpc|serial(5)|id(32)|pubkey_hash(8)|timestamp(8)
			if len(keyBytes) >= 56 { // 3 + 5 + 32 + 8 + 8 = 56
				fidpk = &store.IdPkTs{
					Id:  make([]byte, 32),
					Pub: make([]byte, 8),
					Ts:  0,
				}
				copy(fidpk.Id, keyBytes[8:40])
				copy(fidpk.Pub, keyBytes[40:48])
				// Parse timestamp (big-endian uint64)
				var ts int64
				for i := 0; i < 8; i++ {
					ts = (ts << 8) | int64(keyBytes[48+i])
				}
				fidpk.Ts = ts
				fidpk.Ser = ser.Get()
				return errors.New("found") // Stop iteration
			}
		}

		return cursor.Continue()
	})

	if fidpk != nil {
		return fidpk, nil
	}
	if err != nil && err.Error() != "found" {
		return nil, err
	}

	return nil, errors.New("full id pubkey not found")
}

// GetFullIdPubkeyBySerials retrieves ID/pubkey/timestamp for multiple serials
func (w *W) GetFullIdPubkeyBySerials(sers []*types.Uint40) (fidpks []*store.IdPkTs, err error) {
	fidpks = make([]*store.IdPkTs, 0, len(sers))

	for _, ser := range sers {
		if ser == nil {
			continue
		}
		fidpk, fetchErr := w.GetFullIdPubkeyBySerial(ser)
		if fetchErr == nil && fidpk != nil {
			fidpks = append(fidpks, fidpk)
		}
	}

	return fidpks, nil
}
