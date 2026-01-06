//go:build !(js && wasm)

package bbolt

import (
	bolt "go.etcd.io/bbolt"
)

const markerPrefix = "marker:"

// SetMarker sets a metadata marker.
func (b *B) SetMarker(key string, value []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}
		return bucket.Put([]byte(markerPrefix+key), value)
	})
}

// GetMarker gets a metadata marker.
func (b *B) GetMarker(key string) (value []byte, err error) {
	err = b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}
		data := bucket.Get([]byte(markerPrefix + key))
		if data != nil {
			value = make([]byte, len(data))
			copy(value, data)
		}
		return nil
	})
	return
}

// HasMarker checks if a marker exists.
func (b *B) HasMarker(key string) bool {
	var exists bool
	b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}
		exists = bucket.Get([]byte(markerPrefix+key)) != nil
		return nil
	})
	return exists
}

// DeleteMarker deletes a marker.
func (b *B) DeleteMarker(key string) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(markerPrefix + key))
	})
}
