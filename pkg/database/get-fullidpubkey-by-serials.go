//go:build !(js && wasm)

package database

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
)

// GetFullIdPubkeyBySerials seeks directly to each serial's prefix in the
// FullIdPubkey index. The input sers slice is expected to be sorted in
// ascending order, allowing efficient forward-only iteration via a single
// Badger iterator.
func (d *D) GetFullIdPubkeyBySerials(sers []*types.Uint40) (
	fidpks []*store.IdPkTs, err error,
) {
	// Pre-allocate slice with exact capacity to reduce reallocations
	fidpks = make([]*store.IdPkTs, 0, len(sers))
	if len(sers) == 0 {
		return
	}
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			// Scope the iterator to the FullIdPubkey table using its 3-byte prefix.
			buf := new(bytes.Buffer)
			if err = indexes.NewPrefix(indexes.FullIdPubkey).MarshalWrite(buf); chk.E(err) {
				return
			}
			tablePrefix := buf.Bytes()
			it := txn.NewIterator(badger.IteratorOptions{Prefix: tablePrefix})
			defer it.Close()

			for _, s := range sers {
				if s == nil {
					continue
				}
				// Build the serial-specific prefix: 3-byte table prefix + 5-byte serial.
				sbuf := new(bytes.Buffer)
				if err = indexes.FullIdPubkeyEnc(
					s, nil, nil, nil,
				).MarshalWrite(sbuf); chk.E(err) {
					return
				}
				serialPrefix := sbuf.Bytes()

				// Seek to the first key for this serial and verify it matches the prefix.
				it.Seek(serialPrefix)
				if it.ValidForPrefix(serialPrefix) {
					item := it.Item()
					key := item.Key()
					ser, fid, p, ca := indexes.FullIdPubkeyVars()
					if err = indexes.FullIdPubkeyDec(
						ser, fid, p, ca,
					).UnmarshalRead(bytes.NewBuffer(key)); chk.E(err) {
						return
					}
					idpkts := store.NewIdPkTs(
						fid.Bytes(),
						p.Bytes(),
						int64(ca.Get()),
						ser.Get(),
					)
					fidpks = append(fidpks, &idpkts)
				}
			}
			return
		},
	); chk.E(err) {
		return
	}
	return
}
