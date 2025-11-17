package database

import (
	"bytes"
	"sort"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes/types"
)

func (d *D) GetSerialsByRange(idx Range) (
	sers types.Uint40s, err error,
) {
	// Pre-allocate slice with estimated capacity to reduce reallocations
	sers = make(types.Uint40s, 0, 100) // Estimate based on typical range sizes
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			it := txn.NewIterator(
				badger.IteratorOptions{
					Reverse: true,
				},
			)
			defer it.Close()
			// Start from a position that includes the end boundary (until timestamp)
			// We create an end boundary that's slightly beyond the actual end to ensure inclusivity
			endBoundary := make([]byte, len(idx.End))
			copy(endBoundary, idx.End)
			// Add 0xff bytes to ensure we capture all events at the exact until timestamp
			for i := 0; i < 5; i++ {
				endBoundary = append(endBoundary, 0xff)
			}
			iterCount := 0
			it.Seek(endBoundary)
			// log.T.F("GetSerialsByRange: iterator valid=%v, sought to endBoundary", it.Valid())
			for it.Valid() {
				iterCount++
				if iterCount > 100 {
					// Safety limit to prevent infinite loops in debugging
					log.T.F("GetSerialsByRange: hit safety limit of 100 iterations")
					break
				}
				item := it.Item()
				var key []byte
				key = item.Key()
				keyWithoutSerial := key[:len(key)-5]
				cmp := bytes.Compare(keyWithoutSerial, idx.Start)
				// log.T.F("GetSerialsByRange: iter %d, key prefix matches=%v, cmp=%d", iterCount, bytes.HasPrefix(key, idx.Start[:len(idx.Start)-8]), cmp)
				if cmp < 0 {
					// didn't find it within the timestamp range
					// log.T.F("GetSerialsByRange: key out of range (cmp=%d), stopping iteration", cmp)
					// log.T.F("  keyWithoutSerial len=%d: %x", len(keyWithoutSerial), keyWithoutSerial)
					// log.T.F("  idx.Start len=%d: %x", len(idx.Start), idx.Start)
					return
				}
				ser := new(types.Uint40)
				buf := bytes.NewBuffer(key[len(key)-5:])
				if err = ser.UnmarshalRead(buf); chk.E(err) {
					return
				}
				sers = append(sers, ser)
				it.Next()
			}
			// log.T.F("GetSerialsByRange: iteration complete, found %d serials", len(sers))
			return
		},
	); chk.E(err) {
		return
	}
	sort.Slice(
		sers, func(i, j int) bool {
			return sers[i].Get() < sers[j].Get()
		},
	)
	return
}
