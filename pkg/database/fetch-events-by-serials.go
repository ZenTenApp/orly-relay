package database

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

// FetchEventsBySerials fetches multiple events by their serials in a single database transaction.
// Returns a map of serial uint64 value to event, only including successfully fetched events.
func (d *D) FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error) {
	// Pre-allocate map with estimated capacity to reduce reallocations
	events = make(map[uint64]*event.E, len(serials))

	if len(serials) == 0 {
		return events, nil
	}

	if err = d.View(
		func(txn *badger.Txn) (err error) {
			for _, ser := range serials {
				var ev *event.E

				// Try sev (small event inline) prefix first - Reiser4 optimization
				smallBuf := new(bytes.Buffer)
				if err = indexes.SmallEventEnc(ser).MarshalWrite(smallBuf); chk.E(err) {
					// Skip this serial on error but continue with others
					err = nil
					continue
				}

				// Iterate with prefix to find the small event key
				opts := badger.DefaultIteratorOptions
				opts.Prefix = smallBuf.Bytes()
				opts.PrefetchValues = true
				opts.PrefetchSize = 1
				it := txn.NewIterator(opts)

				it.Rewind()
				if it.Valid() {
					// Found in sev table - extract inline data
					key := it.Item().Key()
					// Key format: sev|serial|size_uint16|event_data
					if len(key) > 8+2 { // prefix(3) + serial(5) + size(2) = 10 bytes minimum
						sizeIdx := 8 // After sev(3) + serial(5)
						// Read uint16 big-endian size
						size := int(key[sizeIdx])<<8 | int(key[sizeIdx+1])
						dataStart := sizeIdx + 2

						if len(key) >= dataStart+size {
							eventData := key[dataStart : dataStart+size]
							ev = new(event.E)
							if err = ev.UnmarshalBinary(bytes.NewBuffer(eventData)); err == nil {
								events[ser.Get()] = ev
							}
							// Clean up and continue
							it.Close()
							err = nil
							continue
						}
					}
				}
				it.Close()

				// Not found in sev table, try evt (traditional) prefix
				buf := new(bytes.Buffer)
				if err = indexes.EventEnc(ser).MarshalWrite(buf); chk.E(err) {
					// Skip this serial on error but continue with others
					err = nil
					continue
				}

				var item *badger.Item
				if item, err = txn.Get(buf.Bytes()); err != nil {
					// Skip this serial if not found but continue with others
					err = nil
					continue
				}

				var v []byte
				if v, err = item.ValueCopy(nil); chk.E(err) {
					// Skip this serial on error but continue with others
					err = nil
					continue
				}

				// Check if we have valid data before attempting to unmarshal
				if len(v) < 32+32+1+2+1+1+64 { // ID + Pubkey + min varint fields + Sig
					// Skip this serial - incomplete data
					continue
				}

				ev = new(event.E)
				if err = ev.UnmarshalBinary(bytes.NewBuffer(v)); err != nil {
					// Skip this serial on unmarshal error but continue with others
					err = nil
					continue
				}

				// Successfully unmarshaled event, add to results
				events[ser.Get()] = ev
			}
			return nil
		},
	); err != nil {
		return
	}

	return events, nil
}