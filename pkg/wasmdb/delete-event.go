//go:build js && wasm

package wasmdb

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	hexenc "next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/ints"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/tag/atag"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
	"next.orly.dev/pkg/utils"
)

// DeleteEvent removes an event from the database identified by `eid`.
func (w *W) DeleteEvent(c context.Context, eid []byte) (err error) {
	w.Logger.Warnf("deleting event %0x", eid)

	// Get the serial number for the event ID
	var ser *types.Uint40
	ser, err = w.GetSerialById(eid)
	if chk.E(err) {
		return
	}
	if ser == nil {
		// Event wasn't found, nothing to delete
		return
	}

	// Fetch the event to get its data
	var ev *event.E
	ev, err = w.FetchEventBySerial(ser)
	if chk.E(err) {
		return
	}
	if ev == nil {
		// Event wasn't found, nothing to delete
		return
	}

	if err = w.DeleteEventBySerial(c, ser, ev); chk.E(err) {
		return
	}
	return
}

// DeleteEventBySerial removes an event and all its indexes by serial number.
func (w *W) DeleteEventBySerial(c context.Context, ser *types.Uint40, ev *event.E) (err error) {
	w.Logger.Infof("DeleteEventBySerial: deleting event %0x (serial %d)", ev.ID, ser.Get())

	// Get all indexes for the event
	var idxs [][]byte
	idxs, err = database.GetIndexesForEvent(ev, ser.Get())
	if chk.E(err) {
		w.Logger.Errorf("DeleteEventBySerial: failed to get indexes for event %0x: %v", ev.ID, err)
		return
	}
	w.Logger.Infof("DeleteEventBySerial: found %d indexes for event %0x", len(idxs), ev.ID)

	// Collect all unique store names we need to access
	storeNames := make(map[string]struct{})
	for _, key := range idxs {
		if len(key) >= 3 {
			storeNames[string(key[:3])] = struct{}{}
		}
	}

	// Also include event stores
	storeNames[string(indexes.EventPrefix)] = struct{}{}
	storeNames[string(indexes.SmallEventPrefix)] = struct{}{}

	// Convert to slice
	storeList := make([]string, 0, len(storeNames))
	for name := range storeNames {
		storeList = append(storeList, name)
	}

	if len(storeList) == 0 {
		return nil
	}

	// Start a transaction to delete the event and all its indexes
	tx, err := w.db.Transaction(idb.TransactionReadWrite, storeList[0], storeList[1:]...)
	if err != nil {
		return fmt.Errorf("failed to start delete transaction: %w", err)
	}

	// Delete all indexes
	for _, key := range idxs {
		if len(key) < 3 {
			continue
		}
		storeName := string(key[:3])
		objStore, storeErr := tx.ObjectStore(storeName)
		if storeErr != nil {
			w.Logger.Warnf("DeleteEventBySerial: failed to get object store %s: %v", storeName, storeErr)
			continue
		}

		keyJS := bytesToSafeValue(key)
		if _, delErr := objStore.Delete(keyJS); delErr != nil {
			w.Logger.Warnf("DeleteEventBySerial: failed to delete index from %s: %v", storeName, delErr)
		}
	}

	// Delete from small event store
	sevKeyBuf := new(bytes.Buffer)
	if err = indexes.SmallEventEnc(ser).MarshalWrite(sevKeyBuf); err == nil {
		if objStore, storeErr := tx.ObjectStore(string(indexes.SmallEventPrefix)); storeErr == nil {
			// For small events, the key includes size and data, so we need to scan
			w.deleteKeysByPrefix(objStore, sevKeyBuf.Bytes())
		}
	}

	// Delete from large event store
	evtKeyBuf := new(bytes.Buffer)
	if err = indexes.EventEnc(ser).MarshalWrite(evtKeyBuf); err == nil {
		if objStore, storeErr := tx.ObjectStore(string(indexes.EventPrefix)); storeErr == nil {
			keyJS := bytesToSafeValue(evtKeyBuf.Bytes())
			objStore.Delete(keyJS)
		}
	}

	// Commit transaction
	if err = tx.Await(c); err != nil {
		return fmt.Errorf("failed to commit delete transaction: %w", err)
	}

	w.Logger.Infof("DeleteEventBySerial: successfully deleted event %0x and all indexes", ev.ID)
	return nil
}

// deleteKeysByPrefix deletes all keys starting with the given prefix from an object store
func (w *W) deleteKeysByPrefix(store *idb.ObjectStore, prefix []byte) {
	cursorReq, err := store.OpenCursor(idb.CursorNext)
	if err != nil {
		return
	}

	var keysToDelete [][]byte
	cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		keyBytes := safeValueToBytes(keyVal)
		if len(keyBytes) >= len(prefix) && bytes.HasPrefix(keyBytes, prefix) {
			keysToDelete = append(keysToDelete, keyBytes)
		}

		return cursor.Continue()
	})

	// Delete collected keys
	for _, key := range keysToDelete {
		keyJS := bytesToSafeValue(key)
		store.Delete(keyJS)
	}
}

// DeleteExpired scans for events with expiration timestamps that have passed and deletes them.
func (w *W) DeleteExpired() {
	now := time.Now().Unix()

	// Open read transaction to find expired events
	tx, err := w.db.Transaction(idb.TransactionReadOnly, string(indexes.ExpirationPrefix))
	if err != nil {
		w.Logger.Warnf("DeleteExpired: failed to start transaction: %v", err)
		return
	}

	objStore, err := tx.ObjectStore(string(indexes.ExpirationPrefix))
	if err != nil {
		w.Logger.Warnf("DeleteExpired: failed to get expiration store: %v", err)
		return
	}

	var expiredSerials types.Uint40s

	cursorReq, err := objStore.OpenCursor(idb.CursorNext)
	if err != nil {
		w.Logger.Warnf("DeleteExpired: failed to open cursor: %v", err)
		return
	}

	cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		keyBytes := safeValueToBytes(keyVal)
		if len(keyBytes) < 8 { // exp prefix (3) + expiration (variable) + serial (5)
			return cursor.Continue()
		}

		// Parse expiration key: exp|expiration_timestamp|serial
		exp, ser := indexes.ExpirationVars()
		buf := bytes.NewBuffer(keyBytes)
		if err := indexes.ExpirationDec(exp, ser).UnmarshalRead(buf); err != nil {
			return cursor.Continue()
		}

		if int64(exp.Get()) > now {
			// Not expired yet
			return cursor.Continue()
		}

		expiredSerials = append(expiredSerials, ser)
		return cursor.Continue()
	})

	// Delete expired events
	for _, ser := range expiredSerials {
		ev, fetchErr := w.FetchEventBySerial(ser)
		if fetchErr != nil || ev == nil {
			continue
		}
		if err := w.DeleteEventBySerial(context.Background(), ser, ev); err != nil {
			w.Logger.Warnf("DeleteExpired: failed to delete expired event: %v", err)
		}
	}
}

// ProcessDelete processes a kind 5 deletion event, deleting referenced events.
func (w *W) ProcessDelete(ev *event.E, admins [][]byte) (err error) {
	eTags := ev.Tags.GetAll([]byte("e"))
	aTags := ev.Tags.GetAll([]byte("a"))
	kTags := ev.Tags.GetAll([]byte("k"))

	// Process e-tags: delete specific events by ID
	for _, eTag := range eTags {
		if eTag.Len() < 2 {
			continue
		}
		// Use ValueHex() to handle both binary and hex storage formats
		eventIdHex := eTag.ValueHex()
		if len(eventIdHex) != 64 { // hex encoded event ID
			continue
		}
		// Decode hex event ID
		var eid []byte
		if eid, err = hexenc.DecAppend(nil, eventIdHex); chk.E(err) {
			continue
		}
		// Fetch the event to verify ownership
		var ser *types.Uint40
		if ser, err = w.GetSerialById(eid); chk.E(err) || ser == nil {
			continue
		}
		var targetEv *event.E
		if targetEv, err = w.FetchEventBySerial(ser); chk.E(err) || targetEv == nil {
			continue
		}
		// Only allow users to delete their own events
		if !utils.FastEqual(targetEv.Pubkey, ev.Pubkey) {
			continue
		}
		// Delete the event
		if err = w.DeleteEvent(context.Background(), eid); chk.E(err) {
			w.Logger.Warnf("failed to delete event %x via e-tag: %v", eid, err)
			continue
		}
		w.Logger.Debugf("deleted event %x via e-tag deletion", eid)
	}

	// Process a-tags: delete addressable events by kind:pubkey:d-tag
	for _, aTag := range aTags {
		if aTag.Len() < 2 {
			continue
		}
		// Parse the 'a' tag value: kind:pubkey:d-tag (for parameterized) or kind:pubkey (for regular)
		split := bytes.Split(aTag.Value(), []byte{':'})
		if len(split) < 2 {
			continue
		}
		// Parse the kind
		kindStr := string(split[0])
		kindInt, parseErr := strconv.Atoi(kindStr)
		if parseErr != nil {
			continue
		}
		kk := kind.New(uint16(kindInt))
		// Parse the pubkey
		var pk []byte
		if pk, err = hexenc.DecAppend(nil, split[1]); chk.E(err) {
			continue
		}
		// Only allow users to delete their own events
		if !utils.FastEqual(pk, ev.Pubkey) {
			continue
		}

		// Build filter for events to delete
		delFilter := &filter.F{
			Authors: tag.NewFromBytesSlice(pk),
			Kinds:   kind.NewS(kk),
		}

		// For parameterized replaceable events, add d-tag filter
		if kind.IsParameterizedReplaceable(kk.K) && len(split) >= 3 {
			dValue := split[2]
			delFilter.Tags = tag.NewS(tag.NewFromAny([]byte("d"), dValue))
		}

		// Find matching events
		var idxs []database.Range
		if idxs, err = database.GetIndexesFromFilter(delFilter); chk.E(err) {
			continue
		}
		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = w.GetSerialsByRange(idx); chk.E(err) {
				continue
			}
			sers = append(sers, s...)
		}

		// Delete events older than the deletion event
		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = w.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				continue
			}
			idPkTss = append(idPkTss, tmp...)
			// Sort by timestamp
			sort.Slice(idPkTss, func(i, j int) bool {
				return idPkTss[i].Ts > idPkTss[j].Ts
			})
			for _, v := range idPkTss {
				if v.Ts < ev.CreatedAt {
					if err = w.DeleteEvent(context.Background(), v.Id); chk.E(err) {
						w.Logger.Warnf("failed to delete event %x via a-tag: %v", v.Id, err)
						continue
					}
					w.Logger.Debugf("deleted event %x via a-tag deletion", v.Id)
				}
			}
		}
	}

	// If there are no e or a tags, delete all replaceable events of the kinds
	// specified by the k tags for the pubkey of the delete event.
	if len(eTags) == 0 && len(aTags) == 0 {
		// Parse the kind tags
		var kinds []*kind.K
		for _, k := range kTags {
			kv := k.Value()
			iv := ints.New(0)
			if _, err = iv.Unmarshal(kv); chk.E(err) {
				continue
			}
			kinds = append(kinds, kind.New(iv.N))
		}

		var idxs []database.Range
		if idxs, err = database.GetIndexesFromFilter(
			&filter.F{
				Authors: tag.NewFromBytesSlice(ev.Pubkey),
				Kinds:   kind.NewS(kinds...),
			},
		); chk.E(err) {
			return
		}

		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = w.GetSerialsByRange(idx); chk.E(err) {
				return
			}
			sers = append(sers, s...)
		}

		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = w.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// Sort by timestamp
			sort.Slice(idPkTss, func(i, j int) bool {
				return idPkTss[i].Ts > idPkTss[j].Ts
			})
			for _, v := range idPkTss {
				if v.Ts < ev.CreatedAt {
					if err = w.DeleteEvent(context.Background(), v.Id); chk.E(err) {
						continue
					}
				}
			}
		}
	}
	return
}

// CheckForDeleted checks if the event has been deleted, and returns an error with
// prefix "blocked:" if it is. This function also allows designating admin
// pubkeys that may also delete the event.
func (w *W) CheckForDeleted(ev *event.E, admins [][]byte) (err error) {
	keys := append([][]byte{ev.Pubkey}, admins...)
	authors := tag.NewFromBytesSlice(keys...)

	// If the event is addressable, check for a deletion event with the same
	// kind/pubkey/dtag
	if kind.IsParameterizedReplaceable(ev.Kind) {
		var idxs []database.Range
		// Construct an a-tag
		t := ev.Tags.GetFirst([]byte("d"))
		var dTagValue []byte
		if t != nil {
			dTagValue = t.Value()
		}
		a := atag.T{
			Kind:   kind.New(ev.Kind),
			Pubkey: ev.Pubkey,
			DTag:   dTagValue,
		}
		at := a.Marshal(nil)
		if idxs, err = database.GetIndexesFromFilter(
			&filter.F{
				Authors: authors,
				Kinds:   kind.NewS(kind.Deletion),
				Tags:    tag.NewS(tag.NewFromAny("#a", at)),
			},
		); chk.E(err) {
			return
		}

		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = w.GetSerialsByRange(idx); chk.E(err) {
				return
			}
			sers = append(sers, s...)
		}

		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = w.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// Find the newest deletion timestamp
			maxTs := idPkTss[0].Ts
			for i := 1; i < len(idPkTss); i++ {
				if idPkTss[i].Ts > maxTs {
					maxTs = idPkTss[i].Ts
				}
			}
			if ev.CreatedAt < maxTs {
				err = errorf.E(
					"blocked: %0x was deleted by address %s because it is older than the delete: event: %d delete: %d",
					ev.ID, at, ev.CreatedAt, maxTs,
				)
				return
			}
			return
		}
		return
	}

	// If the event is replaceable, check if there is a deletion event newer
	// than the event
	if kind.IsReplaceable(ev.Kind) {
		var idxs []database.Range
		if idxs, err = database.GetIndexesFromFilter(
			&filter.F{
				Authors: tag.NewFromBytesSlice(ev.Pubkey),
				Kinds:   kind.NewS(kind.Deletion),
				Tags: tag.NewS(
					tag.NewFromAny("#k", fmt.Sprint(ev.Kind)),
				),
			},
		); chk.E(err) {
			return
		}

		var sers types.Uint40s
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = w.GetSerialsByRange(idx); chk.E(err) {
				return
			}
			sers = append(sers, s...)
		}

		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = w.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// Find the newest deletion
			maxTs := idPkTss[0].Ts
			maxId := idPkTss[0].Id
			for i := 1; i < len(idPkTss); i++ {
				if idPkTss[i].Ts > maxTs {
					maxTs = idPkTss[i].Ts
					maxId = idPkTss[i].Id
				}
			}
			if ev.CreatedAt < maxTs {
				err = fmt.Errorf(
					"blocked: %0x was deleted: the event is older than the delete event %0x: event: %d delete: %d",
					ev.ID, maxId, ev.CreatedAt, maxTs,
				)
				return
			}
		}

		// This type of delete can also use an a tag to specify kind and author
		idxs = nil
		a := atag.T{
			Kind:   kind.New(ev.Kind),
			Pubkey: ev.Pubkey,
		}
		at := a.Marshal(nil)
		if idxs, err = database.GetIndexesFromFilter(
			&filter.F{
				Authors: authors,
				Kinds:   kind.NewS(kind.Deletion),
				Tags:    tag.NewS(tag.NewFromAny("#a", at)),
			},
		); chk.E(err) {
			return
		}

		sers = nil
		for _, idx := range idxs {
			var s types.Uint40s
			if s, err = w.GetSerialsByRange(idx); chk.E(err) {
				return
			}
			sers = append(sers, s...)
		}

		if len(sers) > 0 {
			var idPkTss []*store.IdPkTs
			var tmp []*store.IdPkTs
			if tmp, err = w.GetFullIdPubkeyBySerials(sers); chk.E(err) {
				return
			}
			idPkTss = append(idPkTss, tmp...)
			// Find the newest deletion
			maxTs := idPkTss[0].Ts
			for i := 1; i < len(idPkTss); i++ {
				if idPkTss[i].Ts > maxTs {
					maxTs = idPkTss[i].Ts
				}
			}
			if ev.CreatedAt < maxTs {
				err = errorf.E(
					"blocked: %0x was deleted by address %s because it is older than the delete: event: %d delete: %d",
					ev.ID, at, ev.CreatedAt, maxTs,
				)
				return
			}
			return
		}
		return
	}

	// Otherwise check for a delete by event id
	var idxs []database.Range
	if idxs, err = database.GetIndexesFromFilter(
		&filter.F{
			Authors: authors,
			Kinds:   kind.NewS(kind.Deletion),
			Tags: tag.NewS(
				tag.NewFromAny("e", hexenc.Enc(ev.ID)),
			),
		},
	); chk.E(err) {
		return
	}

	for _, idx := range idxs {
		var s types.Uint40s
		if s, err = w.GetSerialsByRange(idx); chk.E(err) {
			return
		}
		if len(s) > 0 {
			// Any e-tag deletion found means the exact event was deleted
			err = errorf.E("blocked: %0x has been deleted", ev.ID)
			return
		}
	}

	return
}
