//go:build !(js && wasm)

package database

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
)

// GrapeVineStore provides database operations for GrapeVine WoT scores.
// It stores raw JSON blobs without importing the grapevine package to avoid circular deps.
type GrapeVineStore struct {
	*D
}

// NewGrapeVineStore creates a new GrapeVineStore instance.
func NewGrapeVineStore(db *D) *GrapeVineStore {
	return &GrapeVineStore{D: db}
}

// SaveScoreSet persists a complete score set for an observer.
// setData is the JSON-marshaled full score set.
// entries maps target pubkey hex to JSON-marshaled individual score entries.
func (g *GrapeVineStore) SaveScoreSet(observerHex string, setData []byte, entries map[string][]byte) error {
	return g.Update(func(txn *badger.Txn) error {
		if err := txn.Set(g.setKey(observerHex), setData); err != nil {
			return err
		}
		for targetHex, entryData := range entries {
			key := g.entryKey(observerHex, targetHex)
			if err := txn.Set(key, entryData); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetScoreSet returns the raw JSON for a full score set, or nil if not found.
func (g *GrapeVineStore) GetScoreSet(observerHex string) ([]byte, error) {
	var data []byte
	err := g.View(func(txn *badger.Txn) error {
		item, err := txn.Get(g.setKey(observerHex))
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// GetScore returns the raw JSON for a single score entry, or nil if not found.
func (g *GrapeVineStore) GetScore(observerHex, targetHex string) ([]byte, error) {
	var data []byte
	err := g.View(func(txn *badger.Txn) error {
		item, err := txn.Get(g.entryKey(observerHex, targetHex))
		if err != nil {
			return err
		}
		data, err = item.ValueCopy(nil)
		return err
	})
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// DeleteScoreSet removes all stored scores for an observer.
func (g *GrapeVineStore) DeleteScoreSet(observerHex string) error {
	return g.Update(func(txn *badger.Txn) error {
		txn.Delete(g.setKey(observerHex))

		prefix := g.entryPrefix(observerHex)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		var keysToDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysToDelete = append(keysToDelete, key)
		}
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *GrapeVineStore) setKey(observerHex string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("GV_SET_")
	buf.WriteString(observerHex)
	return buf.Bytes()
}

func (g *GrapeVineStore) entryKey(observerHex, targetHex string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("GV_ENT_")
	buf.WriteString(observerHex)
	buf.WriteString("_")
	buf.WriteString(targetHex)
	return buf.Bytes()
}

func (g *GrapeVineStore) entryPrefix(observerHex string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("GV_ENT_")
	buf.WriteString(observerHex)
	buf.WriteString("_")
	return buf.Bytes()
}
