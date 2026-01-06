//go:build !(js && wasm)

package bbolt

import (
	"bytes"
	"context"
	"errors"
	"runtime/debug"
	"sort"
	"time"

	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/bufpool"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

// SaveEventMinimal stores only the essential event data for fast bulk import.
// It skips all indexes - call BuildIndexes after import completes.
func (b *B) SaveEventMinimal(ev *event.E) error {
	if ev == nil {
		return errors.New("nil event")
	}

	// Reject ephemeral events
	if ev.Kind >= 20000 && ev.Kind <= 29999 {
		return nil
	}

	// Get the next serial number
	serial := b.getNextEventSerial()

	// Serialize event in raw binary format (not compact - preserves full pubkey)
	// This allows index building to work without pubkey serial resolution
	legacyBuf := bufpool.GetMedium()
	defer bufpool.PutMedium(legacyBuf)
	ev.MarshalBinary(legacyBuf)
	eventData := bufpool.CopyBytes(legacyBuf)

	// Create minimal batch - only event data and ID mappings
	batch := &EventBatch{
		Serial:    serial,
		EventData: eventData,
		Indexes: []BatchedWrite{
			// Event ID -> Serial (for lookups)
			{BucketName: bucketEid, Key: ev.ID[:], Value: makeSerialKey(serial)},
			// Serial -> Event ID (for reverse lookups)
			{BucketName: bucketSei, Key: makeSerialKey(serial), Value: ev.ID[:]},
		},
	}

	return b.batcher.Add(batch)
}

// BuildIndexes builds all query indexes from stored events.
// Call this after importing events with SaveEventMinimal.
// Processes events in chunks to avoid OOM on large databases.
func (b *B) BuildIndexes(ctx context.Context) error {
	log.I.F("bbolt: starting index build...")
	startTime := time.Now()

	// Process in chunks to avoid OOM
	// With ~15 indexes per event and ~50 bytes per key, 200k events = ~150MB
	const chunkSize = 200000

	var totalEvents int
	var lastSerial uint64 = 0
	var lastLogTime = time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Collect indexes for this chunk
		indexesByBucket := make(map[string][][]byte)
		var chunkEvents int
		var chunkSerial uint64

		// Read a chunk of events
		err := b.db.View(func(tx *bolt.Tx) error {
			cmpBucket := tx.Bucket(bucketCmp)
			if cmpBucket == nil {
				return errors.New("cmp bucket not found")
			}

			cursor := cmpBucket.Cursor()

			// Seek to start position
			var k, v []byte
			if lastSerial == 0 {
				k, v = cursor.First()
			} else {
				// Seek past the last processed serial
				seekKey := makeSerialKey(lastSerial + 1)
				k, v = cursor.Seek(seekKey)
			}

			for ; k != nil && chunkEvents < chunkSize; k, v = cursor.Next() {
				serial := decodeSerialKey(k)
				chunkSerial = serial

				// Decode event from raw binary format
				ev := event.New()
				if err := ev.UnmarshalBinary(bytes.NewBuffer(v)); err != nil {
					log.W.F("bbolt: failed to unmarshal event at serial %d: %v", serial, err)
					continue
				}

				// Generate indexes for this event
				rawIdxs, err := database.GetIndexesForEvent(ev, serial)
				if chk.E(err) {
					ev.Free()
					continue
				}

				// Group by bucket (first 3 bytes)
				for _, idx := range rawIdxs {
					if len(idx) < 3 {
						continue
					}
					bucketName := string(idx[:3])
					key := idx[3:]

					// Skip eid and sei - already stored during import
					if bucketName == "eid" || bucketName == "sei" {
						continue
					}

					// Make a copy of the key
					keyCopy := make([]byte, len(key))
					copy(keyCopy, key)
					indexesByBucket[bucketName] = append(indexesByBucket[bucketName], keyCopy)
				}

				ev.Free()
				chunkEvents++
			}
			return nil
		})
		if err != nil {
			return err
		}

		// No more events to process
		if chunkEvents == 0 {
			break
		}

		totalEvents += chunkEvents
		lastSerial = chunkSerial

		// Progress logging
		if time.Since(lastLogTime) >= 5*time.Second {
			log.I.F("bbolt: index build progress: %d events processed", totalEvents)
			lastLogTime = time.Now()
		}

		// Count total keys in this chunk
		var totalKeys int
		for _, keys := range indexesByBucket {
			totalKeys += len(keys)
		}
		log.I.F("bbolt: writing %d index keys for chunk (%d events)", totalKeys, chunkEvents)

		// Write this chunk's indexes
		for bucketName, keys := range indexesByBucket {
			if len(keys) == 0 {
				continue
			}

			bucketBytes := []byte(bucketName)

			// Sort keys for this bucket before writing
			sort.Slice(keys, func(i, j int) bool {
				return bytes.Compare(keys[i], keys[j]) < 0
			})

			// Write in batches
			const batchSize = 50000
			for i := 0; i < len(keys); i += batchSize {
				end := i + batchSize
				if end > len(keys) {
					end = len(keys)
				}
				batch := keys[i:end]

				err := b.db.Update(func(tx *bolt.Tx) error {
					bucket := tx.Bucket(bucketBytes)
					if bucket == nil {
						return nil
					}
					for _, key := range batch {
						if err := bucket.Put(key, nil); err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					log.E.F("bbolt: failed to write batch for bucket %s: %v", bucketName, err)
					return err
				}
			}
		}

		// Clear for next chunk and release memory
		indexesByBucket = nil
		debug.FreeOSMemory()
	}

	elapsed := time.Since(startTime)
	log.I.F("bbolt: index build complete in %v (%d events)", elapsed.Round(time.Second), totalEvents)

	return nil
}

// decodeSerialKey decodes a 5-byte serial key to uint64
func decodeSerialKey(b []byte) uint64 {
	if len(b) < 5 {
		return 0
	}
	return uint64(b[0])<<32 | uint64(b[1])<<24 | uint64(b[2])<<16 | uint64(b[3])<<8 | uint64(b[4])
}
