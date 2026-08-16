//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"time"

	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database/indexes"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
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
				// ser is a single reused codec instance; the slice stores pointers,
				// so copy the value before appending — otherwise every entry aliases
				// the same object and ends up holding only the last decoded serial.
				s := new(types.Uint40)
				s.Set(ser.Get())
				expiredSerials = append(expiredSerials, s)
			}
			return
		},
	); chk.E(err) {
	}
	// delete the events and their indexes, capped per cycle and rate-limited
	// to avoid overwhelming Badger under concurrent REQ load (see pkg/storage/gc.go
	// notes on race conditions with inline deletion). Remaining expired events are
	// picked up by the next sweep cycle.
	deleted := 0
	for i, ser := range expiredSerials {
		if i >= expiredCleanupBatch {
			log.I.F(
				"DeleteExpired: reached batch limit (%d), %d deleted; %d remaining deferred to next cycle",
				expiredCleanupBatch, deleted, len(expiredSerials)-i,
			)
			break
		}
		var ev *event.E
		if ev, err = d.FetchEventBySerial(ser); chk.E(err) {
			continue
		}
		if err = d.DeleteEventBySerial(
			context.Background(), ser, ev,
		); chk.E(err) {
			continue
		}
		deleted++
		if deleted%expiredCleanupRateLimit == 0 {
			time.Sleep(expiredCleanupDelay)
		}
	}
	if deleted > 0 {
		// Invalidate the query cache: it may still hold these now-deleted expired
		// events and would keep serving them on read otherwise.
		d.InvalidateQueryCache()
		log.I.F("DeleteExpired: deleted %d expired events", deleted)
	}
}

// Tuning constants for the expired-event sweep. The batch cap bounds each cycle
// so the sweeper never monopolizes the database, and the rate limit inserts small
// pauses between bulk deletions to avoid Badger race conditions under load.
const (
	expiredCleanupBatch      = 5000
	expiredCleanupRateLimit  = 50
	expiredCleanupDelay      = 5 * time.Millisecond
)
