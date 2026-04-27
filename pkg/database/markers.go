//go:build !(js && wasm)

package database

import (
	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
)

const (
	markerPrefix = "MARKER:"
)

// SetMarker stores an arbitrary marker in the database
func (d *D) SetMarker(key string, value []byte) (err error) {
	markerKey := []byte(markerPrefix + key)
	
	err = d.Update(func(txn *badger.Txn) error {
		return txn.Set(markerKey, value)
	})
	
	return
}

// GetMarker retrieves an arbitrary marker from the database
func (d *D) GetMarker(key string) (value []byte, err error) {
	markerKey := []byte(markerPrefix + key)
	
	err = d.View(func(txn *badger.Txn) error {
		item, err := txn.Get(markerKey)
		if err != nil {
			return err
		}
		
		value, err = item.ValueCopy(nil)
		return err
	})
	
	return
}

// HasMarker checks if a marker exists in the database
func (d *D) HasMarker(key string) (exists bool) {
	markerKey := []byte(markerPrefix + key)
	
	err := d.View(func(txn *badger.Txn) error {
		_, err := txn.Get(markerKey)
		return err
	})
	
	exists = !chk.E(err)
	return
}

// DeleteMarker removes a marker from the database
func (d *D) DeleteMarker(key string) (err error) {
	markerKey := []byte(markerPrefix + key)
	
	err = d.Update(func(txn *badger.Txn) error {
		return txn.Delete(markerKey)
	})
	
	return
}