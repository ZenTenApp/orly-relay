//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"

	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database/indexes"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// DeleteEvent removes an event from the database identified by `eid`. If
// noTombstone is false or not provided, a tombstone is created for the event.
func (d *D) DeleteEvent(c context.Context, eid []byte) (err error) {
	d.Logger.Warningf("deleting event %0x", eid)

	// Get the serial number for the event ID
	var ser *types.Uint40
	ser, err = d.GetSerialById(eid)
	if chk.E(err) {
		return
	}
	if ser == nil {
		// Event wasn't found, nothing to delete
		return
	}
	// Fetch the event to get its data
	var ev *event.E
	ev, err = d.FetchEventBySerial(ser)
	if chk.E(err) {
		return
	}
	if ev == nil {
		// Event wasn't found, nothing to delete. this shouldn't happen.
		return
	}
	if err = d.DeleteEventBySerial(c, ser, ev); chk.E(err) {
		return
	}
	return
}

func (d *D) DeleteEventBySerial(
	c context.Context, ser *types.Uint40, ev *event.E,
) (err error) {
	d.Logger.Infof("DeleteEventBySerial: deleting event %0x (serial %d)", ev.ID, ser.Get())

	// Get all indexes for the event
	var idxs [][]byte
	idxs, err = GetIndexesForEvent(ev, ser.Get())
	if chk.E(err) {
		d.Logger.Errorf("DeleteEventBySerial: failed to get indexes for event %0x: %v", ev.ID, err)
		return
	}
	d.Logger.Infof("DeleteEventBySerial: found %d indexes for event %0x", len(idxs), ev.ID)

	// Get the event key
	eventKey := new(bytes.Buffer)
	if err = indexes.EventEnc(ser).MarshalWrite(eventKey); chk.E(err) {
		d.Logger.Errorf("DeleteEventBySerial: failed to create event key for %0x: %v", ev.ID, err)
		return
	}

	// Resolve author serial for ppg/gpp cleanup
	var authorSerial *types.Uint40
	authorSerial, _ = d.GetPubkeySerial(ev.Pubkey)

	// Delete the event and all its indexes in a transaction
	err = d.Update(
		func(txn *badger.Txn) (err error) {
			// Delete the event
			if err = txn.Delete(eventKey.Bytes()); chk.E(err) {
				d.Logger.Errorf("DeleteEventBySerial: failed to delete event %0x: %v", ev.ID, err)
				return
			}
			d.Logger.Infof("DeleteEventBySerial: deleted event %0x", ev.ID)

			// Delete all standard indexes
			for i, key := range idxs {
				if err = txn.Delete(key); chk.E(err) {
					d.Logger.Errorf("DeleteEventBySerial: failed to delete index %d for event %0x: %v", i, ev.ID, err)
					return
				}
			}
			d.Logger.Infof("DeleteEventBySerial: deleted %d indexes for event %0x", len(idxs), ev.ID)

			// Delete graph indexes (epg/peg, ppg/gpp, eeg/gee)
			graphDeleted := deleteGraphIndexes(txn, ser, ev, authorSerial)
			if graphDeleted > 0 {
				log.D.F("DeleteEventBySerial: deleted %d graph index entries for event %0x", graphDeleted, ev.ID)
			}

			return
		},
	)
	if chk.E(err) {
		d.Logger.Errorf("DeleteEventBySerial: transaction failed for event %0x: %v", ev.ID, err)
		return
	}

	d.Logger.Infof("DeleteEventBySerial: successfully deleted event %0x and all indexes", ev.ID)
	return
}

// deleteGraphIndexes removes all graph index entries (epg/peg, ppg/gpp, eeg/gee)
// associated with the given event serial. These indexes are written directly in
// save-event.go and not covered by GetIndexesForEvent.
//
// Returns the number of index entries deleted.
func deleteGraphIndexes(txn *badger.Txn, ser *types.Uint40, ev *event.E, authorSerial *types.Uint40) int {
	deleted := 0

	// Serialize the event serial once for byte comparisons
	serBuf := new(bytes.Buffer)
	if err := ser.MarshalWrite(serBuf); err != nil {
		return 0
	}
	serBytes := serBuf.Bytes() // 5 bytes

	// --- epg: event→pubkey graph ---
	// Key: epg(3)|event_serial(5)|pubkey_serial(5)|kind(2)|direction(1) = 16 bytes
	// Prefix scan on epg|eventSerial finds all pubkey edges for this event.
	// For each, construct and delete the reverse peg key.
	epgPrefix := append([]byte(indexes.EventPubkeyGraphPrefix), serBytes...)
	deleted += deleteByPrefixWithReverse(txn, epgPrefix, 16, func(key []byte) []byte {
		// Extract fields from epg key to construct peg reverse key
		// epg: [0:3]=prefix [3:8]=event_serial [8:13]=pubkey_serial [13:15]=kind [15:16]=direction
		pubkeySerial := key[8:13]
		kind := key[13:15]
		direction := key[15:16]
		// peg: peg(3)|pubkey_serial(5)|kind(2)|direction(1)|event_serial(5) = 16 bytes
		rev := make([]byte, 16)
		copy(rev[0:3], indexes.PubkeyEventGraphPrefix)
		copy(rev[3:8], pubkeySerial)
		copy(rev[8:10], kind)
		copy(rev[10:11], direction)
		copy(rev[11:16], serBytes)
		return rev
	})

	// --- eeg: event→event graph (outbound e-tags) ---
	// Key: eeg(3)|source_event(5)|target_event(5)|kind(2)|direction(1) = 16 bytes
	// Prefix scan on eeg|eventSerial finds all outbound e-tag edges.
	eegPrefix := append([]byte(indexes.EventEventGraphPrefix), serBytes...)
	deleted += deleteByPrefixWithReverse(txn, eegPrefix, 16, func(key []byte) []byte {
		// eeg: [0:3]=prefix [3:8]=source_serial [8:13]=target_serial [13:15]=kind [15:16]=direction
		targetSerial := key[8:13]
		kind := key[13:15]
		// Reverse direction: outbound becomes inbound
		dirIn := []byte{types.EdgeDirectionETagIn}
		// gee: gee(3)|target_event(5)|kind(2)|direction(1)|source_event(5) = 16 bytes
		rev := make([]byte, 16)
		copy(rev[0:3], indexes.GraphEventEventPrefix)
		copy(rev[3:8], targetSerial)
		copy(rev[8:10], kind)
		copy(rev[10:11], dirIn)
		copy(rev[11:16], serBytes)
		return rev
	})

	// --- gee: reverse event→event (inbound e-tags referencing this event) ---
	// Key: gee(3)|target_event(5)|kind(2)|direction(1)|source_event(5) = 16 bytes
	// Prefix scan on gee|eventSerial finds all events that reference this event.
	geePrefix := append([]byte(indexes.GraphEventEventPrefix), serBytes...)
	deleted += deleteByPrefixWithReverse(txn, geePrefix, 16, func(key []byte) []byte {
		// gee: [0:3]=prefix [3:8]=target_serial [8:10]=kind [10:11]=direction [11:16]=source_serial
		kind := key[8:10]
		sourceSerial := key[11:16]
		// Reverse: eeg(3)|source_serial(5)|target_serial(5)|kind(2)|direction(1)
		dirOut := []byte{types.EdgeDirectionETagOut}
		rev := make([]byte, 16)
		copy(rev[0:3], indexes.EventEventGraphPrefix)
		copy(rev[3:8], sourceSerial)
		copy(rev[8:13], serBytes) // this event is the target
		copy(rev[13:15], kind)
		copy(rev[15:16], dirOut)
		return rev
	})

	// --- ppg: pubkey→pubkey graph (outbound, keyed by author) ---
	// Key: ppg(3)|source_pk(5)|target_pk(5)|kind(2)|direction(1)|event_serial(5) = 21 bytes
	// We scan ppg|authorSerial and filter by trailing event_serial matching ser.
	if authorSerial != nil {
		authorBuf := new(bytes.Buffer)
		if err := authorSerial.MarshalWrite(authorBuf); err == nil {
			ppgPrefix := append([]byte(indexes.PubkeyPubkeyGraphPrefix), authorBuf.Bytes()...)
			deleted += deleteByPrefixFilterSerial(txn, ppgPrefix, 21, serBytes, func(key []byte) []byte {
				// ppg: [0:3]=prefix [3:8]=source_pk [8:13]=target_pk [13:15]=kind [15:16]=direction [16:21]=event_serial
				targetPk := key[8:13]
				kind := key[13:15]
				// Reverse direction for gpp
				dirIn := []byte{types.EdgeDirectionPubkeyIn}
				// gpp: gpp(3)|target_pk(5)|kind(2)|direction(1)|source_pk(5)|event_serial(5) = 21 bytes
				rev := make([]byte, 21)
				copy(rev[0:3], indexes.GraphPubkeyPubkeyPrefix)
				copy(rev[3:8], targetPk)
				copy(rev[8:10], kind)
				copy(rev[10:11], dirIn)
				copy(rev[11:16], authorBuf.Bytes()) // source = author
				copy(rev[16:21], serBytes)
				return rev
			})
		}
	}

	// --- peg entries where this event appears as target ---
	// peg keys that reference our event serial are at the end: peg(3)|pk(5)|kind(2)|dir(1)|event_serial(5)
	// We can't efficiently prefix-scan for these. However, we already found and deleted all
	// epg entries above, which gave us the pubkey serials. The reverse peg entries were
	// constructed and deleted from those. For any peg entries where this event is referenced
	// by OTHER events' pubkey graphs, those would be cleaned up when those events are deleted.

	// --- sei: serial→eventID ---
	seiBuf := new(bytes.Buffer)
	seiBuf.Write([]byte(indexes.SerialEventIdPrefix))
	seiBuf.Write(serBytes)
	if err := txn.Delete(seiBuf.Bytes()); err == nil {
		deleted++
	}

	// --- cmp: compact event storage ---
	cmpBuf := new(bytes.Buffer)
	cmpBuf.Write([]byte(indexes.CompactEventPrefix))
	cmpBuf.Write(serBytes)
	if err := txn.Delete(cmpBuf.Bytes()); err == nil {
		deleted++
	}

	// --- aev: addressable event index (kinds 30000-39999) ---
	if ev.Kind >= 30000 && ev.Kind < 40000 {
		dTag := ev.Tags.GetFirst([]byte("d"))
		if dTag != nil && dTag.Len() >= 2 {
			pubHash := new(types.PubHash)
			if err := pubHash.FromPubkey(ev.Pubkey); err == nil {
				kind := new(types.Uint16)
				kind.Set(ev.Kind)
				ident := new(types.Ident)
				ident.FromIdent(dTag.Value())
				aevKey := new(bytes.Buffer)
				if err := indexes.AddressableEventEnc(pubHash, kind, ident).MarshalWrite(aevKey); err == nil {
					if err := txn.Delete(aevKey.Bytes()); err == nil {
						deleted++
					}
				}
			}
		}
	}

	if deleted > 0 {
		log.D.F("deleteGraphIndexes: cleaned %d graph/compact/addressable entries for event %s",
			deleted, hex.Enc(ev.ID))
	}

	return deleted
}

// deleteByPrefixWithReverse scans all keys matching prefix, deletes each, and
// constructs+deletes a reverse key using the provided function.
func deleteByPrefixWithReverse(txn *badger.Txn, prefix []byte, expectedLen int, reverseKeyFn func([]byte) []byte) int {
	deleted := 0
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = prefix

	it := txn.NewIterator(opts)
	defer it.Close()

	// Collect keys first (can't delete while iterating)
	var keys [][]byte
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		key := it.Item().KeyCopy(nil)
		if len(key) != expectedLen {
			continue
		}
		keys = append(keys, key)
	}

	for _, key := range keys {
		if err := txn.Delete(key); err == nil {
			deleted++
		}
		rev := reverseKeyFn(key)
		if rev != nil {
			if err := txn.Delete(rev); err == nil {
				deleted++
			}
		}
	}
	return deleted
}

// deleteByPrefixFilterSerial scans keys matching prefix, filters by trailing
// event serial (last 5 bytes), deletes matches, and constructs+deletes reverse keys.
func deleteByPrefixFilterSerial(txn *badger.Txn, prefix []byte, expectedLen int, serialBytes []byte, reverseKeyFn func([]byte) []byte) int {
	deleted := 0
	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Prefix = prefix

	it := txn.NewIterator(opts)
	defer it.Close()

	var keys [][]byte
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		key := it.Item().KeyCopy(nil)
		if len(key) != expectedLen {
			continue
		}
		// Check if trailing 5 bytes match the event serial
		if !bytes.Equal(key[expectedLen-5:], serialBytes) {
			continue
		}
		keys = append(keys, key)
	}

	for _, key := range keys {
		if err := txn.Delete(key); err == nil {
			deleted++
		}
		rev := reverseKeyFn(key)
		if rev != nil {
			if err := txn.Delete(rev); err == nil {
				deleted++
			}
		}
	}
	return deleted
}
