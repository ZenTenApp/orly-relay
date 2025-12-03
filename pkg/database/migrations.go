//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/ints"
	"git.mleku.dev/mleku/nostr/encoders/kind"
)

const (
	currentVersion uint32 = 5
)

func (d *D) RunMigrations() {
	var err error
	var dbVersion uint32
	// first find the current version tag if any
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			buf := new(bytes.Buffer)
			if err = indexes.VersionEnc(nil).MarshalWrite(buf); chk.E(err) {
				return
			}
			verPrf := new(bytes.Buffer)
			if _, err = indexes.VersionPrefix.Write(verPrf); chk.E(err) {
				return
			}
			it := txn.NewIterator(
				badger.IteratorOptions{
					Prefix: verPrf.Bytes(),
				},
			)
			defer it.Close()
			ver := indexes.VersionVars()
			for it.Rewind(); it.Valid(); it.Next() {
				// there should only be one
				item := it.Item()
				key := item.Key()
				if err = indexes.VersionDec(ver).UnmarshalRead(
					bytes.NewBuffer(key),
				); chk.E(err) {
					return
				}
				log.I.F("found version tag: %d", ver.Get())
				dbVersion = ver.Get()
			}
			return
		},
	); chk.E(err) {
	}
	if dbVersion == 0 {
		// write the version tag now (ensure any old tags are removed first)
		if err = d.writeVersionTag(currentVersion); chk.E(err) {
			return
		}
	}
	if dbVersion < 1 {
		log.I.F("migrating to version 1...")
		// the first migration is expiration tags
		d.UpdateExpirationTags()
		// bump to version 1
		_ = d.writeVersionTag(1)
	}
	if dbVersion < 2 {
		log.I.F("migrating to version 2...")
		// backfill word indexes
		d.UpdateWordIndexes()
		// bump to version 2
		_ = d.writeVersionTag(2)
	}
	if dbVersion < 3 {
		log.I.F("migrating to version 3...")
		// clean up ephemeral events that should never have been stored
		d.CleanupEphemeralEvents()
		// bump to version 3
		_ = d.writeVersionTag(3)
	}
	if dbVersion < 4 {
		log.I.F("migrating to version 4...")
		// convert small events to inline storage (Reiser4 optimization)
		d.ConvertSmallEventsToInline()
		// bump to version 4
		_ = d.writeVersionTag(4)
	}
	if dbVersion < 5 {
		log.I.F("migrating to version 5...")
		// re-encode events with optimized tag binary format (e/p tags)
		d.ReencodeEventsWithOptimizedTags()
		// bump to version 5
		_ = d.writeVersionTag(5)
	}
}

// writeVersionTag writes a new version tag key to the database (no value)
func (d *D) writeVersionTag(ver uint32) (err error) {
	return d.Update(
		func(txn *badger.Txn) (err error) {
			// delete any existing version keys first (there should only be one, but be safe)
			verPrf := new(bytes.Buffer)
			if _, err = indexes.VersionPrefix.Write(verPrf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: verPrf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.KeyCopy(nil)
				if err = txn.Delete(key); chk.E(err) {
					return
				}
			}

			// now write the new version key
			buf := new(bytes.Buffer)
			vv := new(types.Uint32)
			vv.Set(ver)
			if err = indexes.VersionEnc(vv).MarshalWrite(buf); chk.E(err) {
				return
			}
			return txn.Set(buf.Bytes(), nil)
		},
	)
}

func (d *D) UpdateWordIndexes() {
	var err error
	var wordIndexes [][]byte
	// iterate all events and generate word index keys from content and tags
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			prf := new(bytes.Buffer)
			if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: prf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				var val []byte
				if val, err = item.ValueCopy(nil); chk.E(err) {
					continue
				}
				// decode the event
				ev := new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
					continue
				}
				// log.I.F("updating word indexes for event: %s", ev.Serialize())
				// read serial from key
				key := item.Key()
				ser := indexes.EventVars()
				if err = indexes.EventDec(ser).UnmarshalRead(bytes.NewBuffer(key)); chk.E(err) {
					continue
				}
				// collect unique word hashes for this event
				seen := make(map[string]struct{})
				// from content
				if len(ev.Content) > 0 {
					for _, h := range TokenHashes(ev.Content) {
						seen[string(h)] = struct{}{}
					}
				}
				// from all tag fields (key and values)
				if ev.Tags != nil && ev.Tags.Len() > 0 {
					for _, t := range *ev.Tags {
						for _, field := range t.T {
							if len(field) == 0 {
								continue
							}
							for _, h := range TokenHashes(field) {
								seen[string(h)] = struct{}{}
							}
						}
					}
				}
				// build keys
				for k := range seen {
					w := new(types.Word)
					w.FromWord([]byte(k))
					buf := new(bytes.Buffer)
					if err = indexes.WordEnc(
						w, ser,
					).MarshalWrite(buf); chk.E(err) {
						continue
					}
					wordIndexes = append(wordIndexes, buf.Bytes())
				}
			}
			return
		},
	); chk.E(err) {
		return
	}
	// sort the indexes for ordered writes
	sort.Slice(
		wordIndexes, func(i, j int) bool {
			return bytes.Compare(
				wordIndexes[i], wordIndexes[j],
			) < 0
		},
	)
	// write in a batch
	batch := d.NewWriteBatch()
	for _, v := range wordIndexes {
		if err = batch.Set(v, nil); chk.E(err) {
			continue
		}
	}
	_ = batch.Flush()
}

func (d *D) UpdateExpirationTags() {
	var err error
	var expIndexes [][]byte
	// iterate all event records and decode and look for version tags
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			prf := new(bytes.Buffer)
			if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: prf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				var val []byte
				if val, err = item.ValueCopy(nil); chk.E(err) {
					continue
				}
				// decode the event
				ev := new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
					continue
				}
				expTag := ev.Tags.GetFirst([]byte("expiration"))
				if expTag == nil {
					continue
				}
				expTS := ints.New(0)
				if _, err = expTS.Unmarshal(expTag.Value()); chk.E(err) {
					continue
				}
				key := item.Key()
				ser := indexes.EventVars()
				if err = indexes.EventDec(ser).UnmarshalRead(
					bytes.NewBuffer(key),
				); chk.E(err) {
					continue
				}
				// create the expiration tag
				exp, _ := indexes.ExpirationVars()
				exp.Set(expTS.N)
				expBuf := new(bytes.Buffer)
				if err = indexes.ExpirationEnc(
					exp, ser,
				).MarshalWrite(expBuf); chk.E(err) {
					continue
				}
				expIndexes = append(expIndexes, expBuf.Bytes())
			}
			return
		},
	); chk.E(err) {
		return
	}
	// sort the indexes first so they're written in order, improving compaction
	// and iteration.
	sort.Slice(
		expIndexes, func(i, j int) bool {
			return bytes.Compare(expIndexes[i], expIndexes[j]) < 0
		},
	)
	// write the collected indexes
	batch := d.NewWriteBatch()
	for _, v := range expIndexes {
		if err = batch.Set(v, nil); chk.E(err) {
			continue
		}
	}
	if err = batch.Flush(); chk.E(err) {
		return
	}
}

func (d *D) CleanupEphemeralEvents() {
	log.I.F("cleaning up ephemeral events (kinds 20000-29999)...")
	var err error
	var ephemeralEvents [][]byte
	
	// iterate all event records and find ephemeral events
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			prf := new(bytes.Buffer)
			if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: prf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				var val []byte
				if val, err = item.ValueCopy(nil); chk.E(err) {
					continue
				}
				// decode the event
				ev := new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
					continue
				}
				// check if it's an ephemeral event (kinds 20000-29999)
				if ev.Kind >= 20000 && ev.Kind <= 29999 {
					ephemeralEvents = append(ephemeralEvents, ev.ID)
				}
			}
			return
		},
	); chk.E(err) {
		return
	}
	
	// delete all found ephemeral events
	deletedCount := 0
	for _, eventId := range ephemeralEvents {
		if err = d.DeleteEvent(context.Background(), eventId); chk.E(err) {
			log.W.F("failed to delete ephemeral event %x: %v", eventId, err)
			continue
		}
		deletedCount++
	}
	
	log.I.F("cleaned up %d ephemeral events from database", deletedCount)
}

// ConvertSmallEventsToInline migrates small events (<=384 bytes) to inline storage.
// This is a Reiser4-inspired optimization that stores small event data in the key itself,
// avoiding a second database lookup and improving query performance.
// Also handles replaceable and addressable events with specialized storage.
func (d *D) ConvertSmallEventsToInline() {
	log.I.F("converting events to optimized inline storage (Reiser4 optimization)...")
	var err error
	const smallEventThreshold = 384

	type EventData struct {
		Serial          uint64
		EventData       []byte
		OldKey          []byte
		IsReplaceable   bool
		IsAddressable   bool
		Pubkey          []byte
		Kind            uint16
		DTag            []byte
	}

	var events []EventData
	var convertedCount int
	var deletedCount int

	// Helper function for counting by predicate
	countBy := func(events []EventData, predicate func(EventData) bool) int {
		count := 0
		for _, e := range events {
			if predicate(e) {
				count++
			}
		}
		return count
	}

	// First pass: identify events in evt table that can benefit from inline storage
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			prf := new(bytes.Buffer)
			if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: prf.Bytes()})
			defer it.Close()

			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				var val []byte
				if val, err = item.ValueCopy(nil); chk.E(err) {
					continue
				}

				// Check if event data is small enough for inline storage
				if len(val) <= smallEventThreshold {
					// Decode event to check if it's replaceable or addressable
					ev := new(event.E)
					if err = ev.UnmarshalBinary(bytes.NewBuffer(val)); chk.E(err) {
						continue
					}

					// Extract serial from key
					key := item.KeyCopy(nil)
					ser := indexes.EventVars()
					if err = indexes.EventDec(ser).UnmarshalRead(bytes.NewBuffer(key)); chk.E(err) {
						continue
					}

					eventData := EventData{
						Serial:        ser.Get(),
						EventData:     val,
						OldKey:        key,
						IsReplaceable: kind.IsReplaceable(ev.Kind),
						IsAddressable: kind.IsParameterizedReplaceable(ev.Kind),
						Pubkey:        ev.Pubkey,
						Kind:          ev.Kind,
					}

					// Extract d-tag for addressable events
					if eventData.IsAddressable {
						dTag := ev.Tags.GetFirst([]byte("d"))
						if dTag != nil {
							eventData.DTag = dTag.Value()
						}
					}

					events = append(events, eventData)
				}
			}
			return nil
		},
	); chk.E(err) {
		return
	}

	log.I.F("found %d events to convert (%d regular, %d replaceable, %d addressable)",
		len(events),
		countBy(events, func(e EventData) bool { return !e.IsReplaceable && !e.IsAddressable }),
		countBy(events, func(e EventData) bool { return e.IsReplaceable }),
		countBy(events, func(e EventData) bool { return e.IsAddressable }),
	)

	// Second pass: convert in batches to avoid large transactions
	const batchSize = 1000
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		batch := events[i:end]

		// Write new inline keys and delete old keys
		if err = d.Update(
			func(txn *badger.Txn) (err error) {
				for _, e := range batch {
					// First, write the sev key for serial-based access (all small events)
					sevKeyBuf := new(bytes.Buffer)
					ser := new(types.Uint40)
					if err = ser.Set(e.Serial); chk.E(err) {
						continue
					}

					if err = indexes.SmallEventEnc(ser).MarshalWrite(sevKeyBuf); chk.E(err) {
						continue
					}

					// Append size as uint16 big-endian (2 bytes)
					sizeBytes := []byte{byte(len(e.EventData) >> 8), byte(len(e.EventData))}
					sevKeyBuf.Write(sizeBytes)

					// Append event data
					sevKeyBuf.Write(e.EventData)

					// Write sev key (no value needed)
					if err = txn.Set(sevKeyBuf.Bytes(), nil); chk.E(err) {
						log.W.F("failed to write sev key for serial %d: %v", e.Serial, err)
						continue
					}
					convertedCount++

					// Additionally, for replaceable/addressable events, write specialized keys
					if e.IsAddressable && len(e.DTag) > 0 {
						// Addressable event: aev|pubkey_hash|kind|dtag_hash|size|data
						aevKeyBuf := new(bytes.Buffer)
						pubHash := new(types.PubHash)
						pubHash.FromPubkey(e.Pubkey)
						kindVal := new(types.Uint16)
						kindVal.Set(e.Kind)
						dTagHash := new(types.Ident)
						dTagHash.FromIdent(e.DTag)

						if err = indexes.AddressableEventEnc(pubHash, kindVal, dTagHash).MarshalWrite(aevKeyBuf); chk.E(err) {
							continue
						}

						// Append size and data
						aevKeyBuf.Write(sizeBytes)
						aevKeyBuf.Write(e.EventData)

						if err = txn.Set(aevKeyBuf.Bytes(), nil); chk.E(err) {
							log.W.F("failed to write aev key for serial %d: %v", e.Serial, err)
							continue
						}
					} else if e.IsReplaceable {
						// Replaceable event: rev|pubkey_hash|kind|size|data
						revKeyBuf := new(bytes.Buffer)
						pubHash := new(types.PubHash)
						pubHash.FromPubkey(e.Pubkey)
						kindVal := new(types.Uint16)
						kindVal.Set(e.Kind)

						if err = indexes.ReplaceableEventEnc(pubHash, kindVal).MarshalWrite(revKeyBuf); chk.E(err) {
							continue
						}

						// Append size and data
						revKeyBuf.Write(sizeBytes)
						revKeyBuf.Write(e.EventData)

						if err = txn.Set(revKeyBuf.Bytes(), nil); chk.E(err) {
							log.W.F("failed to write rev key for serial %d: %v", e.Serial, err)
							continue
						}
					}

					// Delete old evt key
					if err = txn.Delete(e.OldKey); chk.E(err) {
						log.W.F("failed to delete old event key for serial %d: %v", e.Serial, err)
						continue
					}
					deletedCount++
				}
				return nil
			},
		); chk.E(err) {
			log.W.F("batch update failed: %v", err)
			continue
		}

		if (i/batchSize)%10 == 0 && i > 0 {
			log.I.F("progress: %d/%d events converted", i, len(events))
		}
	}

	log.I.F("migration complete: converted %d events to optimized inline storage, deleted %d old keys", convertedCount, deletedCount)
}

// ReencodeEventsWithOptimizedTags re-encodes all events to use the new binary
// tag format that stores e/p tag values as 33-byte binary (32-byte hash + null)
// instead of 64-byte hex strings. This reduces memory usage by ~48% for these tags.
func (d *D) ReencodeEventsWithOptimizedTags() {
	log.I.F("re-encoding events with optimized tag binary format...")
	var err error

	type EventUpdate struct {
		Key     []byte
		OldData []byte
		NewData []byte
	}

	var updates []EventUpdate
	var processedCount int

	// Helper to collect event updates from iterator
	// Only processes regular events (evt prefix) - inline storage already benefits
	collectUpdates := func(it *badger.Iterator, prefix []byte) error {
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)

			var val []byte
			if val, err = item.ValueCopy(nil); chk.E(err) {
				continue
			}

			// Regular event storage - data is in value
			eventData := val
			if len(eventData) == 0 {
				continue
			}

			// Decode the event
			ev := new(event.E)
			if err = ev.UnmarshalBinary(bytes.NewBuffer(eventData)); chk.E(err) {
				continue
			}

			// Check if this event has e or p tags that could benefit from optimization
			hasOptimizableTags := false
			if ev.Tags != nil && ev.Tags.Len() > 0 {
				for _, t := range *ev.Tags {
					if t.Len() >= 2 {
						key := t.Key()
						if len(key) == 1 && (key[0] == 'e' || key[0] == 'p') {
							hasOptimizableTags = true
							break
						}
					}
				}
			}

			if !hasOptimizableTags {
				continue
			}

			// Re-encode the event (this will apply the new tag optimization)
			newData := ev.MarshalBinaryToBytes(nil)

			// Only update if the data actually changed
			if !bytes.Equal(eventData, newData) {
				updates = append(updates, EventUpdate{
					Key:     key,
					OldData: eventData,
					NewData: newData,
				})
			}
		}
		return nil
	}

	// Only process regular "evt" prefix events (not inline storage)
	// Inline storage (sev, rev, aev) already benefits from the optimization
	// because the binary data is stored directly in the key
	prf := new(bytes.Buffer)
	if err = indexes.EventEnc(nil).MarshalWrite(prf); chk.E(err) {
		return
	}
	evtPrefix := prf.Bytes()

	// Collect updates from regular events only
	if err = d.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{Prefix: evtPrefix})
		defer it.Close()
		return collectUpdates(it, evtPrefix)
	}); chk.E(err) {
		return
	}

	log.I.F("found %d events with e/p tags to re-encode", len(updates))

	if len(updates) == 0 {
		log.I.F("no events need re-encoding")
		return
	}

	// Apply updates in batches
	const batchSize = 1000
	for i := 0; i < len(updates); i += batchSize {
		end := i + batchSize
		if end > len(updates) {
			end = len(updates)
		}
		batch := updates[i:end]

		if err = d.Update(func(txn *badger.Txn) error {
			for _, upd := range batch {
				// Since we're only processing regular events (evt prefix),
				// we just update the value directly
				if err = txn.Set(upd.Key, upd.NewData); chk.E(err) {
					log.W.F("failed to update event: %v", err)
					continue
				}
				processedCount++
			}
			return nil
		}); chk.E(err) {
			log.W.F("batch update failed: %v", err)
			continue
		}

		if (i/batchSize)%10 == 0 && i > 0 {
			log.I.F("progress: %d/%d events re-encoded", i, len(updates))
		}
	}

	savedBytes := 0
	for _, upd := range updates {
		savedBytes += len(upd.OldData) - len(upd.NewData)
	}

	log.I.F("migration complete: re-encoded %d events, saved approximately %d bytes (%.2f KB)",
		processedCount, savedBytes, float64(savedBytes)/1024.0)
}
