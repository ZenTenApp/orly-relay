//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/utils/units"
)

// Flusher interface for HTTP streaming
type flusher interface {
	Flush()
}

// Export the complete database of stored events to an io.Writer in line structured minified
// JSON. Supports both legacy and compact event formats.
func (d *D) Export(c context.Context, w io.Writer, pubkeys ...[]byte) {
	var err error
	evB := make([]byte, 0, units.Mb)
	evBuf := bytes.NewBuffer(evB)

	// Get flusher for HTTP streaming if available
	var f flusher
	if fl, ok := w.(flusher); ok {
		f = fl
	}

	// Performance tracking
	startTime := time.Now()
	var eventCount, bytesWritten int64
	lastLogTime := startTime
	const logInterval = 5 * time.Second
	const flushInterval = 100 // Flush every N events

	log.I.F("export: starting export operation")

	// Create resolver for compact event decoding
	resolver := NewDatabaseSerialResolver(d, d.serialCache)

	// Helper function to unmarshal event data (handles both legacy and compact formats)
	unmarshalEventData := func(val []byte, ser *types.Uint40) (*event.E, error) {
		// Check if this is compact format (starts with version byte 1)
		if len(val) > 0 && val[0] == CompactFormatVersion {
			// Get event ID from SerialEventId table
			eventId, idErr := d.GetEventIdBySerial(ser)
			if idErr != nil {
				// Can't decode without event ID - skip
				return nil, idErr
			}
			return UnmarshalCompactEvent(val, eventId, resolver)
		}

		// Legacy binary format
		ev := event.New()
		evBuf.Reset()
		evBuf.Write(val)
		if err := ev.UnmarshalBinary(evBuf); err != nil {
			return nil, err
		}
		return ev, nil
	}

	if len(pubkeys) == 0 {
		// Export all events - prefer cmp table, fall back to evt
		if err = d.View(
			func(txn *badger.Txn) (err error) {
				// First try cmp (compact format) table
				cmpBuf := new(bytes.Buffer)
				if err = indexes.CompactEventEnc(nil).MarshalWrite(cmpBuf); chk.E(err) {
					return
				}

				it := txn.NewIterator(badger.IteratorOptions{Prefix: cmpBuf.Bytes()})
				defer it.Close()

				seenSerials := make(map[uint64]bool)

				for it.Rewind(); it.Valid(); it.Next() {
					item := it.Item()
					key := item.Key()

					// Extract serial from key
					ser := new(types.Uint40)
					if err = ser.UnmarshalRead(bytes.NewReader(key[3:8])); chk.E(err) {
						continue
					}

					seenSerials[ser.Get()] = true

					var val []byte
					if val, err = item.ValueCopy(nil); chk.E(err) {
						continue
					}

					ev, unmarshalErr := unmarshalEventData(val, ser)
					if unmarshalErr != nil {
						continue
					}

					// Serialize the event to JSON and write it to the output
					data := ev.Serialize()
					if _, err = w.Write(data); chk.E(err) {
						ev.Free()
						return
					}
					if _, err = w.Write([]byte{'\n'}); chk.E(err) {
						ev.Free()
						return
					}
					bytesWritten += int64(len(data) + 1)
					eventCount++
					ev.Free()

					// Flush periodically for HTTP streaming
					if f != nil && eventCount%flushInterval == 0 {
						f.Flush()
					}

					// Progress logging every logInterval
					if time.Since(lastLogTime) >= logInterval {
						elapsed := time.Since(startTime)
						eventsPerSec := float64(eventCount) / elapsed.Seconds()
						mbPerSec := float64(bytesWritten) / elapsed.Seconds() / 1024 / 1024
						log.I.F("export: progress %d events, %.2f MB written, %.0f events/sec, %.2f MB/sec",
							eventCount, float64(bytesWritten)/1024/1024, eventsPerSec, mbPerSec)
						lastLogTime = time.Now()
					}
				}
				it.Close()

				// Then fall back to evt (legacy) table for any events not in cmp
				evtBuf := new(bytes.Buffer)
				if err = indexes.EventEnc(nil).MarshalWrite(evtBuf); chk.E(err) {
					return
				}

				it2 := txn.NewIterator(badger.IteratorOptions{Prefix: evtBuf.Bytes()})
				defer it2.Close()

				for it2.Rewind(); it2.Valid(); it2.Next() {
					item := it2.Item()
					key := item.Key()

					// Extract serial from key
					ser := new(types.Uint40)
					if err = ser.UnmarshalRead(bytes.NewReader(key[3:8])); chk.E(err) {
						continue
					}

					// Skip if already exported from cmp table
					if seenSerials[ser.Get()] {
						continue
					}

					var val []byte
					if val, err = item.ValueCopy(nil); chk.E(err) {
						continue
					}

					ev, unmarshalErr := unmarshalEventData(val, ser)
					if unmarshalErr != nil {
						continue
					}

					// Serialize the event to JSON and write it to the output
					data := ev.Serialize()
					if _, err = w.Write(data); chk.E(err) {
						ev.Free()
						return
					}
					if _, err = w.Write([]byte{'\n'}); chk.E(err) {
						ev.Free()
						return
					}
					bytesWritten += int64(len(data) + 1)
					eventCount++
					ev.Free()

					// Flush periodically for HTTP streaming
					if f != nil && eventCount%flushInterval == 0 {
						f.Flush()
					}

					// Progress logging every logInterval
					if time.Since(lastLogTime) >= logInterval {
						elapsed := time.Since(startTime)
						eventsPerSec := float64(eventCount) / elapsed.Seconds()
						mbPerSec := float64(bytesWritten) / elapsed.Seconds() / 1024 / 1024
						log.I.F("export: progress %d events, %.2f MB written, %.0f events/sec, %.2f MB/sec",
							eventCount, float64(bytesWritten)/1024/1024, eventsPerSec, mbPerSec)
						lastLogTime = time.Now()
					}
				}

				return
			},
		); err != nil {
			return
		}

		// Final flush
		if f != nil {
			f.Flush()
		}

		// Final export summary
		elapsed := time.Since(startTime)
		eventsPerSec := float64(eventCount) / elapsed.Seconds()
		mbPerSec := float64(bytesWritten) / elapsed.Seconds() / 1024 / 1024
		log.I.F("export: completed - %d events, %.2f MB in %v (%.0f events/sec, %.2f MB/sec)",
			eventCount, float64(bytesWritten)/1024/1024, elapsed.Round(time.Millisecond), eventsPerSec, mbPerSec)
	} else {
		// Export events for specific pubkeys
		log.I.F("export: exporting events for %d pubkeys", len(pubkeys))
		for _, pubkey := range pubkeys {
			if err = d.View(
				func(txn *badger.Txn) (err error) {
					pkBuf := new(bytes.Buffer)
					ph := &types.PubHash{}
					if err = ph.FromPubkey(pubkey); chk.E(err) {
						return
					}
					if err = indexes.PubkeyEnc(
						ph, nil, nil,
					).MarshalWrite(pkBuf); chk.E(err) {
						return
					}
					it := txn.NewIterator(badger.IteratorOptions{Prefix: pkBuf.Bytes()})
					defer it.Close()
					for it.Rewind(); it.Valid(); it.Next() {
						item := it.Item()
						key := item.Key()

						// Extract serial from pubkey index key
						// Key format: pc-|pubkey_hash|created_at|serial
						if len(key) < 3+8+8+5 {
							continue
						}
						ser := new(types.Uint40)
						if err = ser.UnmarshalRead(bytes.NewReader(key[len(key)-5:])); chk.E(err) {
							continue
						}

						// Fetch the event using FetchEventBySerial which handles all formats
						ev, fetchErr := d.FetchEventBySerial(ser)
						if fetchErr != nil || ev == nil {
							continue
						}

						// Serialize the event to JSON and write it to the output
						data := ev.Serialize()
						if _, err = w.Write(data); chk.E(err) {
							ev.Free()
							continue
						}
						if _, err = w.Write([]byte{'\n'}); chk.E(err) {
							ev.Free()
							continue
						}
						bytesWritten += int64(len(data) + 1)
						eventCount++
						ev.Free()

						// Flush periodically for HTTP streaming
						if f != nil && eventCount%flushInterval == 0 {
							f.Flush()
						}

						// Progress logging every logInterval
						if time.Since(lastLogTime) >= logInterval {
							elapsed := time.Since(startTime)
							eventsPerSec := float64(eventCount) / elapsed.Seconds()
							mbPerSec := float64(bytesWritten) / elapsed.Seconds() / 1024 / 1024
							log.I.F("export: progress %d events, %.2f MB written, %.0f events/sec, %.2f MB/sec",
								eventCount, float64(bytesWritten)/1024/1024, eventsPerSec, mbPerSec)
							lastLogTime = time.Now()
						}
					}
					return
				},
			); err != nil {
				return
			}
		}

		// Final flush
		if f != nil {
			f.Flush()
		}

		// Final export summary for pubkey export
		elapsed := time.Since(startTime)
		eventsPerSec := float64(eventCount) / elapsed.Seconds()
		mbPerSec := float64(bytesWritten) / elapsed.Seconds() / 1024 / 1024
		log.I.F("export: completed - %d events, %.2f MB in %v (%.0f events/sec, %.2f MB/sec)",
			eventCount, float64(bytesWritten)/1024/1024, elapsed.Round(time.Millisecond), eventsPerSec, mbPerSec)
	}
}
