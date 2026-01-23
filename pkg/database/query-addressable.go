//go:build !(js && wasm)

package database

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/bufpool"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
)

// IsAddressableEventQuery checks if a filter matches the NIP-33 addressable event
// query pattern: exactly one kind (30000-39999), one author, and one d-tag.
// This pattern uniquely identifies a single parameterized replaceable event.
func IsAddressableEventQuery(f *filter.F) bool {
	// Must have exactly one kind
	if f.Kinds == nil || f.Kinds.Len() != 1 {
		return false
	}
	// Kind must be parameterized replaceable (30000-39999)
	kindVal := f.Kinds.K[0].K
	if !kind.IsParameterizedReplaceable(kindVal) {
		return false
	}
	// Must have exactly one author
	if f.Authors == nil || f.Authors.Len() != 1 {
		return false
	}
	// Must have a d-tag filter
	if f.Tags == nil {
		return false
	}
	dTagFilter := f.Tags.GetFirst([]byte("#d"))
	if dTagFilter == nil || dTagFilter.Len() != 2 {
		return false
	}
	// Must not have IDs filter (would bypass this optimization)
	if f.Ids != nil && f.Ids.Len() > 0 {
		return false
	}
	return true
}

// QueryForAddressableEvent performs a direct O(1) lookup for a NIP-33 parameterized
// replaceable event using the AddressableEvent index.
// Returns the serial if found, nil if not found, or an error.
func (d *D) QueryForAddressableEvent(f *filter.F) (serial *types.Uint40, err error) {
	if !IsAddressableEventQuery(f) {
		return nil, nil
	}

	// Extract components from filter
	kindVal := f.Kinds.K[0].K
	author := f.Authors.T[0]
	dTagFilter := f.Tags.GetFirst([]byte("#d"))
	dTagValue := dTagFilter.T[1]

	// Build pubkey hash
	pubHash := new(types.PubHash)
	if err = pubHash.FromPubkey(author); chk.E(err) {
		return nil, err
	}

	// Build kind type
	kindType := new(types.Uint16)
	kindType.Set(kindVal)

	// Build d-tag hash
	dTagHash := new(types.Ident)
	dTagHash.FromIdent(dTagValue)

	// Build the AddressableEvent index key
	aevKey := indexes.AddressableEventEnc(pubHash, kindType, dTagHash)
	keyBuf := bufpool.GetSmall()
	defer bufpool.PutSmall(keyBuf)
	if err = aevKey.MarshalWrite(keyBuf); chk.E(err) {
		return nil, err
	}
	keyBytes := bufpool.CopyBytes(keyBuf)

	// Direct key lookup - O(1)
	err = d.View(func(txn *badger.Txn) error {
		item, err := txn.Get(keyBytes)
		if err == badger.ErrKeyNotFound {
			// Not found - this is not an error
			return nil
		}
		if err != nil {
			return err
		}

		// Read the serial from the value
		return item.Value(func(val []byte) error {
			if len(val) < 5 {
				log.W.F("QueryForAddressableEvent: invalid value length %d", len(val))
				return nil
			}
			serial = new(types.Uint40)
			rdr := bytes.NewReader(val)
			if err := serial.UnmarshalRead(rdr); err != nil {
				log.W.F("QueryForAddressableEvent: failed to read serial: %v", err)
				serial = nil
				return nil
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if serial != nil {
		log.T.F("QueryForAddressableEvent: found serial %d for kind=%d author=%x d=%s",
			serial.Get(), kindVal, author[:8], string(dTagValue))
	}

	return serial, nil
}

// BuildAddressableEventKey builds the key for an AddressableEvent index entry.
// This is used by both save-event.go (for writing) and deletion (for cleanup).
func BuildAddressableEventKey(pubkey []byte, eventKind uint16, dTagValue []byte) ([]byte, error) {
	pubHash := new(types.PubHash)
	if err := pubHash.FromPubkey(pubkey); err != nil {
		return nil, err
	}

	kindType := new(types.Uint16)
	kindType.Set(eventKind)

	dTagHash := new(types.Ident)
	dTagHash.FromIdent(dTagValue)

	aevKey := indexes.AddressableEventEnc(pubHash, kindType, dTagHash)
	keyBuf := bufpool.GetSmall()
	defer bufpool.PutSmall(keyBuf)
	if err := aevKey.MarshalWrite(keyBuf); err != nil {
		return nil, err
	}

	return bufpool.CopyBytes(keyBuf), nil
}
