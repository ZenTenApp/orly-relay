//go:build !(js && wasm)

package bbolt

import (
	"bytes"
	"context"
	"errors"

	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
)

// FetchEventBySerial fetches an event by its serial number.
func (b *B) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	if ser == nil {
		return nil, errors.New("bbolt: nil serial")
	}

	serial := ser.Get()
	key := makeSerialKey(serial)

	err = b.db.View(func(tx *bolt.Tx) error {
		// Get event ID first
		seiBucket := tx.Bucket(bucketSei)
		var eventId []byte
		if seiBucket != nil {
			eventId = seiBucket.Get(key)
		}

		// Try compact event storage first
		cmpBucket := tx.Bucket(bucketCmp)
		if cmpBucket != nil {
			data := cmpBucket.Get(key)
			if data != nil && eventId != nil && len(eventId) == 32 {
				// Unmarshal compact event
				resolver := &bboltSerialResolver{b: b}
				ev, err = database.UnmarshalCompactEvent(data, eventId, resolver)
				if err == nil {
					return nil
				}
				// Fall through to try legacy format
			}
		}

		// Try legacy event storage
		evtBucket := tx.Bucket(bucketEvt)
		if evtBucket != nil {
			data := evtBucket.Get(key)
			if data != nil {
				ev = new(event.E)
				reader := bytes.NewReader(data)
				if err = ev.UnmarshalBinary(reader); err == nil {
					return nil
				}
			}
		}

		return errors.New("bbolt: event not found")
	})

	return
}

// FetchEventsBySerials fetches multiple events by their serial numbers.
func (b *B) FetchEventsBySerials(serials []*types.Uint40) (events map[uint64]*event.E, err error) {
	events = make(map[uint64]*event.E, len(serials))

	err = b.db.View(func(tx *bolt.Tx) error {
		cmpBucket := tx.Bucket(bucketCmp)
		evtBucket := tx.Bucket(bucketEvt)
		seiBucket := tx.Bucket(bucketSei)
		resolver := &bboltSerialResolver{b: b}

		for _, ser := range serials {
			if ser == nil {
				continue
			}

			serial := ser.Get()
			key := makeSerialKey(serial)

			// Get event ID
			var eventId []byte
			if seiBucket != nil {
				eventId = seiBucket.Get(key)
			}

			// Try compact event storage first
			if cmpBucket != nil {
				data := cmpBucket.Get(key)
				if data != nil && eventId != nil && len(eventId) == 32 {
					ev, e := database.UnmarshalCompactEvent(data, eventId, resolver)
					if e == nil {
						events[serial] = ev
						continue
					}
				}
			}

			// Try legacy event storage
			if evtBucket != nil {
				data := evtBucket.Get(key)
				if data != nil {
					ev := new(event.E)
					reader := bytes.NewReader(data)
					if e := ev.UnmarshalBinary(reader); e == nil {
						events[serial] = ev
					}
				}
			}
		}
		return nil
	})

	return
}

// CountEvents counts events matching a filter.
func (b *B) CountEvents(c context.Context, f *filter.F) (count int, approximate bool, err error) {
	// Get serials matching filter
	var serials types.Uint40s
	if serials, err = b.GetSerialsFromFilter(f); chk.E(err) {
		return
	}

	count = len(serials)
	approximate = false
	return
}
