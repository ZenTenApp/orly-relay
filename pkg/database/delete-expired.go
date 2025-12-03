//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

func (d *D) DeleteExpired() {
	var err error
	var expiredSerials types.Uint40s
	// make the operation atomic and save on accesses to the system clock by
	// setting the boundary at the current second
	now := time.Now().Unix()
	// search the expiration indexes for expiry timestamps that are now past
	if err = d.View(
		func(txn *badger.Txn) (err error) {
			exp, ser := indexes.ExpirationVars()
			expPrf := new(bytes.Buffer)
			if _, err = indexes.ExpirationPrefix.Write(expPrf); chk.E(err) {
				return
			}
			it := txn.NewIterator(badger.IteratorOptions{Prefix: expPrf.Bytes()})
			defer it.Close()
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := item.Key()
				buf := bytes.NewBuffer(key)
				if err = indexes.ExpirationDec(
					exp, ser,
				).UnmarshalRead(buf); chk.E(err) {
					continue
				}
				if int64(exp.Get()) > now {
					// not expired yet
					continue
				}
				expiredSerials = append(expiredSerials, ser)
			}
			return
		},
	); chk.E(err) {
	}
	// delete the events and their indexes
	for _, ser := range expiredSerials {
		var ev *event.E
		if ev, err = d.FetchEventBySerial(ser); chk.E(err) {
			continue
		}
		if err = d.DeleteEventBySerial(
			context.Background(), ser, ev,
		); chk.E(err) {
			continue
		}
	}
}
