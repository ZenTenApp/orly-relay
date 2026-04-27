//go:build !(js && wasm)

package database

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/database/indexes"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

// FetchEventBySerial fetches a single event by its serial.
// This function tries multiple storage formats in order:
// 1. cmp (compact format with serial references) - newest, most space-efficient
// 2. sev (small event inline) - legacy Reiser4 optimization
// 3. evt (traditional separate storage) - legacy fallback
func (d *D) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	// Create resolver for compact event decoding
	resolver := NewDatabaseSerialResolver(d, d.serialCache)

	if err = d.View(
		func(txn *badger.Txn) (err error) {
			// Try cmp (compact format) first - most efficient
			ev, err = d.fetchCompactEvent(txn, ser, resolver)
			if err == nil && ev != nil {
				return nil
			}
			err = nil // Reset error, try legacy formats

			// Helper function to extract inline event data from key
			extractInlineData := func(key []byte, prefixLen int) (*event.E, error) {
				if len(key) > prefixLen+2 {
					sizeIdx := prefixLen
					size := int(key[sizeIdx])<<8 | int(key[sizeIdx+1])
					dataStart := sizeIdx + 2

					if len(key) >= dataStart+size {
						eventData := key[dataStart : dataStart+size]

						// Check if this is compact format (starts with version byte 1)
						// Note: Legacy events whose ID starts with 0x01 will also match this check,
						// so we fall back to legacy format if the SerialEventId mapping doesn't exist.
						if len(eventData) > 0 && eventData[0] == CompactFormatVersion {
							eventId, idErr := d.GetEventIdBySerial(ser)
							if idErr == nil {
								// SerialEventId mapping exists - this is compact format
								return UnmarshalCompactEvent(eventData, eventId, resolver)
							}
							// No SerialEventId mapping - this is likely a legacy event whose ID starts with 0x01
							// Fall through to legacy unmarshal
						}

						// Legacy binary format
						ev := new(event.E)
						if err := ev.UnmarshalBinary(bytes.NewBuffer(eventData)); err != nil {
							return nil, fmt.Errorf(
								"error unmarshaling inline event (size=%d): %w",
								size, err,
							)
						}
						return ev, nil
					}
				}
				return nil, nil
			}

			// Try sev (small event inline) prefix - Reiser4 optimization
			smallBuf := new(bytes.Buffer)
			if err = indexes.SmallEventEnc(ser).MarshalWrite(smallBuf); chk.E(err) {
				return
			}

			opts := badger.DefaultIteratorOptions
			opts.Prefix = smallBuf.Bytes()
			opts.PrefetchValues = true
			opts.PrefetchSize = 1
			it := txn.NewIterator(opts)
			defer it.Close()

			it.Rewind()
			if it.Valid() {
				// Found in sev table - extract inline data
				key := it.Item().Key()
				// Key format: sev|serial|size_uint16|event_data
				if ev, err = extractInlineData(key, 8); err != nil {
					return err
				}
				if ev != nil {
					return nil
				}
			}

			// Not found in sev table, try evt (traditional) prefix
			buf := new(bytes.Buffer)
			if err = indexes.EventEnc(ser).MarshalWrite(buf); chk.E(err) {
				return
			}
			var item *badger.Item
			if item, err = txn.Get(buf.Bytes()); err != nil {
				return
			}
			var v []byte
			if v, err = item.ValueCopy(nil); chk.E(err) {
				return
			}

			// Check if this is compact format (starts with version byte 1)
			// Note: Legacy events whose ID starts with 0x01 will also match this check,
			// so we fall back to legacy format if the SerialEventId mapping doesn't exist.
			if len(v) > 0 && v[0] == CompactFormatVersion {
				eventId, idErr := d.GetEventIdBySerial(ser)
				if idErr == nil {
					// SerialEventId mapping exists - this is compact format
					ev, err = UnmarshalCompactEvent(v, eventId, resolver)
					return
				}
				// No SerialEventId mapping - this is likely a legacy event whose ID starts with 0x01
				// Fall through to legacy unmarshal
			}

			// Check if we have valid data before attempting to unmarshal
			if len(v) < 32+32+1+2+1+1+64 { // ID + Pubkey + min varint fields + Sig
				err = fmt.Errorf(
					"incomplete event data: got %d bytes, expected at least %d",
					len(v), 32+32+1+2+1+1+64,
				)
				return
			}
			ev = new(event.E)
			if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); err != nil {
				// Add more context to EOF errors for debugging
				if err.Error() == "EOF" {
					err = fmt.Errorf(
						"EOF while unmarshaling event (serial=%v, data_len=%d): %w",
						ser, len(v), err,
					)
				}
				return
			}
			return
		},
	); err != nil {
		return
	}
	return
}
