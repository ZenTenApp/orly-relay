package database

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

func (d *D) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			// Helper function to extract inline event data from key
			extractInlineData := func(key []byte, prefixLen int) (*event.E, error) {
				if len(key) > prefixLen+2 {
					sizeIdx := prefixLen
					size := int(key[sizeIdx])<<8 | int(key[sizeIdx+1])
					dataStart := sizeIdx + 2

					if len(key) >= dataStart+size {
						eventData := key[dataStart : dataStart+size]
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

			// Try sev (small event inline) prefix first - Reiser4 optimization
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
