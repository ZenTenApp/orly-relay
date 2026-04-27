//go:build !(js && wasm)

package database

import (
	"bytes"
	"errors"

	"github.com/dgraph-io/badger/v4"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/database/indexes"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// GetOrCreatePubkeySerial returns the serial for a pubkey, creating one if it doesn't exist.
// The pubkey parameter should be 32 bytes (schnorr public key).
// This function is thread-safe and uses transactions to ensure atomicity.
func (d *D) GetOrCreatePubkeySerial(pubkey []byte) (ser *types.Uint40, err error) {
	if len(pubkey) != 32 {
		err = errors.New("pubkey must be 32 bytes")
		return
	}

	// Create pubkey hash
	pubHash := new(types.PubHash)
	if err = pubHash.FromPubkey(pubkey); chk.E(err) {
		return
	}

	// First, try to get existing serial (separate transaction for read)
	var existingSer *types.Uint40
	existingSer, err = d.GetPubkeySerial(pubkey)
	if err == nil && existingSer != nil {
		// Serial already exists
		ser = existingSer
		return ser, nil
	}

	// Serial doesn't exist, create a new one
	var serial uint64
	if serial, err = d.pubkeySeq.Next(); chk.E(err) {
		return
	}

	ser = new(types.Uint40)
	if err = ser.Set(serial); chk.E(err) {
		return
	}

	// Store both mappings in a transaction
	err = d.Update(func(txn *badger.Txn) error {
		// Double-check that the serial wasn't created by another goroutine
		// while we were getting the sequence number
		prefixBuf := new(bytes.Buffer)
		prefixBuf.Write([]byte(indexes.PubkeySerialPrefix))
		if terr := pubHash.MarshalWrite(prefixBuf); chk.E(terr) {
			return terr
		}
		searchPrefix := prefixBuf.Bytes()

		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = searchPrefix
		it := txn.NewIterator(opts)
		it.Seek(searchPrefix)
		if it.Valid() {
			// Another goroutine created it, extract and return that serial
			key := it.Item().KeyCopy(nil)
			it.Close()
			if len(key) == 16 {
				serialBytes := key[11:16]
				serialBuf := bytes.NewReader(serialBytes)
				existSer := new(types.Uint40)
				if terr := existSer.UnmarshalRead(serialBuf); terr == nil {
					ser = existSer
					return nil // Don't write, just return the existing serial
				}
			}
		}
		it.Close()

		// Store pubkey hash -> serial mapping
		keyBuf := new(bytes.Buffer)
		if terr := indexes.PubkeySerialEnc(pubHash, ser).MarshalWrite(keyBuf); chk.E(terr) {
			return terr
		}
		fullKey := make([]byte, len(keyBuf.Bytes()))
		copy(fullKey, keyBuf.Bytes())
		// DEBUG: log the key being written
		if len(fullKey) > 0 {
			// log.T.F("Writing PubkeySerial: key=%s (len=%d), prefix=%s", hex.Enc(fullKey), len(fullKey), string(fullKey[:3]))
		}
		if terr := txn.Set(fullKey, nil); chk.E(terr) {
			return terr
		}

		// Store serial -> full pubkey mapping (pubkey stored as value)
		keyBuf.Reset()
		if terr := indexes.SerialPubkeyEnc(ser).MarshalWrite(keyBuf); chk.E(terr) {
			return terr
		}
		if terr := txn.Set(keyBuf.Bytes(), pubkey); chk.E(terr) {
			return terr
		}

		return nil
	})

	return
}

// GetPubkeySerial returns the serial for a pubkey if it exists.
// Returns an error if the pubkey doesn't have a serial yet.
func (d *D) GetPubkeySerial(pubkey []byte) (ser *types.Uint40, err error) {
	if len(pubkey) != 32 {
		err = errors.New("pubkey must be 32 bytes")
		return
	}

	// Create pubkey hash
	pubHash := new(types.PubHash)
	if err = pubHash.FromPubkey(pubkey); chk.E(err) {
		return
	}

	// Build search key with just prefix + pubkey hash (no serial)
	prefixBuf := new(bytes.Buffer)
	prefixBuf.Write([]byte(indexes.PubkeySerialPrefix)) // 3 bytes
	if err = pubHash.MarshalWrite(prefixBuf); chk.E(err) {
		return
	}
	searchPrefix := prefixBuf.Bytes() // Should be 11 bytes: 3 (prefix) + 8 (pubkey hash)

	ser = new(types.Uint40)
	err = d.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // We only need the key
		it := txn.NewIterator(opts)
		defer it.Close()

		// Seek to the prefix and check if we found a matching key
		it.Seek(searchPrefix)
		if !it.ValidForPrefix(searchPrefix) {
			return errors.New("pubkey serial not found")
		}

		// Extract serial from key (last 5 bytes)
		// Key format: prefix(3) + pubkey_hash(8) + serial(5) = 16 bytes
		key := it.Item().KeyCopy(nil)
		if len(key) != 16 {
			return errors.New("invalid key length for pubkey serial")
		}

		// Verify the prefix matches
		if !bytes.HasPrefix(key, searchPrefix) {
			return errors.New("key prefix mismatch")
		}

		serialBytes := key[11:16] // Extract last 5 bytes (the serial)

		// Decode serial
		serialBuf := bytes.NewReader(serialBytes)
		if err := ser.UnmarshalRead(serialBuf); chk.E(err) {
			return err
		}

		return nil
	})

	return
}

// GetPubkeyBySerial returns the full 32-byte pubkey for a given serial.
func (d *D) GetPubkeyBySerial(ser *types.Uint40) (pubkey []byte, err error) {
	keyBuf := new(bytes.Buffer)
	if err = indexes.SerialPubkeyEnc(ser).MarshalWrite(keyBuf); chk.E(err) {
		return
	}

	err = d.View(func(txn *badger.Txn) error {
		item, gerr := txn.Get(keyBuf.Bytes())
		if chk.E(gerr) {
			return gerr
		}

		return item.Value(func(val []byte) error {
			pubkey = make([]byte, len(val))
			copy(pubkey, val)
			return nil
		})
	})

	if err != nil {
		err = errors.New("pubkey not found for serial: " + hex.Enc([]byte{byte(ser.Get())}))
	}

	return
}
