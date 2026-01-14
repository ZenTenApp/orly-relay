//go:build !(js && wasm)

package database

import (
	"bytes"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"

	// "git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

func (d *D) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	// log.T.F("GetSerialById: input id=%s", hex.Enc(id))
	if len(id) == 0 {
		err = errorf.E("GetSerialById: called with empty ID")
		return
	}
	var idxs []Range
	if idxs, err = GetIndexesFromFilter(&filter.F{Ids: tag.NewFromBytesSlice(id)}); chk.E(err) {
		return
	}
	// for i, idx := range idxs {
	// 	log.T.F(
	// 		"GetSerialById: searching range %d: start=%x, end=%x", i, idx.Start,
	// 		idx.End,
	// 	)
	// }
	if len(idxs) == 0 {
		err = errorf.E("no indexes found for id %0x", id)
		return
	}
	idFound := false
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			var key []byte
			defer it.Close()
			it.Seek(idxs[0].Start)
			if it.ValidForPrefix(idxs[0].Start) {
				item := it.Item()
				key = item.Key()
				ser = new(types.Uint40)
				buf := bytes.NewBuffer(key[len(key)-5:])
				if err = ser.UnmarshalRead(buf); chk.E(err) {
					return
				}
				idFound = true
			} else {
				// Item not found in database
				// log.T.F(
				// 	"GetSerialById: ID not found in database: %s", hex.Enc(id),
				// )
			}
			return
		},
	); chk.E(err) {
		return
	}
	if !idFound {
		err = fmt.Errorf("id not found in database")
		return
	}

	return
}

// GetSerialsByIds takes a tag.T containing multiple IDs and returns a map of IDs to their
// corresponding serial numbers. It directly queries the IdPrefix index for matching IDs,
// which is more efficient than using GetIndexesFromFilter.
func (d *D) GetSerialsByIds(ids *tag.T) (
	serials map[string]*types.Uint40, err error,
) {
	return d.GetSerialsByIdsWithFilter(ids, nil)
}

// GetSerialsByIdsWithFilter takes a tag.T containing multiple IDs and returns a
// map of IDs to their corresponding serial numbers, applying a filter function
// to each event. The function directly creates ID index prefixes for efficient querying.
func (d *D) GetSerialsByIdsWithFilter(
	ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool,
) (serials map[string]*types.Uint40, err error) {
	// log.T.F("GetSerialsByIdsWithFilter: input ids count=%d", ids.Len())

	// Initialize the result map with estimated capacity to reduce reallocations
	serials = make(map[string]*types.Uint40, ids.Len())

	// Return early if no IDs are provided
	if ids.Len() == 0 {
		return
	}

	// Process all IDs in a single transaction
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			// Process each ID sequentially
			for _, id := range ids.T {
				// Skip empty IDs
				if len(id) == 0 {
					continue
				}
				// idHex := hex.Enc(id)

				// Get the index prefix for this ID
				var idxs []Range
				if idxs, err = GetIndexesFromFilter(&filter.F{Ids: tag.NewFromBytesSlice(id)}); chk.E(err) {
					// Skip this ID if we can't create its index
					continue
				}

				// Skip if no index was created
				if len(idxs) == 0 {
					continue
				}

				// Seek to the start of this ID's range in the database
				it.Seek(idxs[0].Start)
				if it.ValidForPrefix(idxs[0].Start) {
					// Found an entry for this ID
					item := it.Item()
					key := item.Key()

					// Extract the serial number from the key
					ser := new(types.Uint40)
					buf := bytes.NewBuffer(key[len(key)-5:])
					if err = ser.UnmarshalRead(buf); chk.E(err) {
						continue
					}

					// If a filter function is provided, fetch the event and apply the filter
					if fn != nil {
						var ev *event.E
						if ev, err = d.FetchEventBySerial(ser); err != nil {
							// Skip this event if we can't fetch it
							continue
						}

						// Apply the filter
						if !fn(ev, ser) {
							// Skip this event if it doesn't pass the filter
							continue
						}
					}

					// Store the serial in the result map using the hex-encoded ID as the key
					serials[string(id)] = ser
				}
			}
			return
		},
	); chk.E(err) {
		return
	}

	log.T.F(
		"GetSerialsByIdsWithFilter: found %d serials out of %d requested ids",
		len(serials), ids.Len(),
	)
	return
}

// func (d *D) GetSerialBytesById(id []byte) (ser []byte, err error) {
// 	var idxs []Range
// 	if idxs, err = GetIndexesFromFilter(&filter.F{Ids: tag.New(id)}); chk.E(err) {
// 		return
// 	}
// 	if len(idxs) == 0 {
// 		err = errorf.E("no indexes found for id %0x", id)
// 	}
// 	if err = d.View(
// 		func(txn *badger.Txn) (err error) {
// 			it := txn.NewIterator(badger.DefaultIteratorOptions)
// 			var key []byte
// 			defer it.Close()
// 			it.Seek(idxs[0].Start)
// 			if it.ValidForPrefix(idxs[0].Start) {
// 				item := it.Item()
// 				key = item.Key()
// 				ser = key[len(key)-5:]
// 			} else {
// 				// just don't return what we don't have? others may be
// 				// found tho.
// 			}
// 			return
// 		},
// 	); chk.E(err) {
// 		return
// 	}
// 	return
// }
