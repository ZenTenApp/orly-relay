//go:build !(js && wasm)

package bbolt

import (
	"bytes"
	"errors"

	bolt "go.etcd.io/bbolt"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// GetSerialById gets the serial for an event ID.
func (b *B) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	if len(id) < 8 {
		return nil, errors.New("bbolt: invalid event ID length")
	}

	idHash := hashEventId(id)

	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketEid)
		if bucket == nil {
			return errors.New("id not found in database")
		}

		// Scan for matching ID hash prefix
		c := bucket.Cursor()
		for k, _ := c.Seek(idHash); k != nil && bytes.HasPrefix(k, idHash); k, _ = c.Next() {
			// Key format: id_hash(8) | serial(5)
			if len(k) >= 13 {
				ser = new(types.Uint40)
				ser.Set(decodeUint40(k[8:13]))
				return nil
			}
		}
		return errors.New("id not found in database")
	})

	return
}

// GetSerialsByIds gets serials for multiple event IDs.
func (b *B) GetSerialsByIds(ids *tag.T) (serials map[string]*types.Uint40, err error) {
	serials = make(map[string]*types.Uint40, ids.Len())

	if ids == nil || ids.Len() == 0 {
		return
	}

	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketEid)
		if bucket == nil {
			return nil
		}

		// Iterate over the tag entries using the .T field
		for _, id := range ids.T {
			if len(id) < 8 {
				continue
			}

			idHash := hashEventId(id)
			c := bucket.Cursor()
			for k, _ := c.Seek(idHash); k != nil && bytes.HasPrefix(k, idHash); k, _ = c.Next() {
				if len(k) >= 13 {
					ser := new(types.Uint40)
					ser.Set(decodeUint40(k[8:13]))
					serials[string(id)] = ser
					break
				}
			}
		}
		return nil
	})

	return
}

// GetSerialsByIdsWithFilter gets serials with a filter function.
func (b *B) GetSerialsByIdsWithFilter(ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool) (serials map[string]*types.Uint40, err error) {
	// For now, just call GetSerialsByIds - full implementation would apply filter
	return b.GetSerialsByIds(ids)
}

// GetSerialsByRange gets serials within a key range.
func (b *B) GetSerialsByRange(idx database.Range) (serials types.Uint40s, err error) {
	if len(idx.Start) < 3 {
		return nil, errors.New("bbolt: invalid range start")
	}

	// Extract bucket name from prefix
	bucketName := idx.Start[:3]
	startKey := idx.Start[3:]
	endKey := idx.End[3:]

	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		if bucket == nil {
			return nil
		}

		c := bucket.Cursor()
		for k, _ := c.Seek(startKey); k != nil; k, _ = c.Next() {
			// Check if we've passed the end
			if len(endKey) > 0 && bytes.Compare(k, endKey) >= 0 {
				break
			}

			// Extract serial from end of key (last 5 bytes)
			if len(k) >= 5 {
				ser := new(types.Uint40)
				ser.Set(decodeUint40(k[len(k)-5:]))
				serials = append(serials, ser)
			}
		}
		return nil
	})

	return
}

// GetFullIdPubkeyBySerial gets full event ID and pubkey by serial.
func (b *B) GetFullIdPubkeyBySerial(ser *types.Uint40) (fidpk *store.IdPkTs, err error) {
	if ser == nil {
		return nil, errors.New("bbolt: nil serial")
	}

	serial := ser.Get()
	key := makeSerialKey(serial)

	err = b.db.View(func(tx *bolt.Tx) error {
		// Get full ID/pubkey from fpc bucket
		fpcBucket := tx.Bucket(bucketFpc)
		if fpcBucket == nil {
			return errors.New("bbolt: fpc bucket not found")
		}

		// Scan for matching serial prefix
		c := fpcBucket.Cursor()
		for k, _ := c.Seek(key); k != nil && bytes.HasPrefix(k, key); k, _ = c.Next() {
			// Key format: serial(5) | id(32) | pubkey_hash(8) | created_at(8)
			if len(k) >= 53 {
				fidpk = &store.IdPkTs{
					Ser: serial,
				}
				fidpk.Id = make([]byte, 32)
				copy(fidpk.Id, k[5:37])
				// Pubkey is only hash here, need to look up full pubkey
				// For now return what we have
				fidpk.Pub = make([]byte, 8)
				copy(fidpk.Pub, k[37:45])
				fidpk.Ts = int64(decodeUint64(k[45:53]))
				return nil
			}
		}
		return errors.New("bbolt: serial not found in fpc index")
	})

	return
}

// GetFullIdPubkeyBySerials gets full event IDs and pubkeys for multiple serials.
func (b *B) GetFullIdPubkeyBySerials(sers []*types.Uint40) (fidpks []*store.IdPkTs, err error) {
	fidpks = make([]*store.IdPkTs, 0, len(sers))

	for _, ser := range sers {
		fidpk, e := b.GetFullIdPubkeyBySerial(ser)
		if e == nil && fidpk != nil {
			fidpks = append(fidpks, fidpk)
		}
	}

	return
}
