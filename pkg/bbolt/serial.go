//go:build !(js && wasm)

package bbolt

import (
	"encoding/binary"

	bolt "go.etcd.io/bbolt"
	"lol.mleku.dev/chk"
)

const (
	serialCounterKey       = "serial_counter"
	pubkeySerialCounterKey = "pubkey_serial_counter"
)

// initSerialCounters initializes or loads the serial counters from _meta bucket
func (b *B) initSerialCounters() error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}

		// Load event serial counter
		val := bucket.Get([]byte(serialCounterKey))
		if val == nil {
			b.nextSerial = 1
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, 1)
			if err := bucket.Put([]byte(serialCounterKey), buf); err != nil {
				return err
			}
		} else {
			b.nextSerial = binary.BigEndian.Uint64(val)
		}

		// Load pubkey serial counter
		val = bucket.Get([]byte(pubkeySerialCounterKey))
		if val == nil {
			b.nextPubkeySeq = 1
			buf := make([]byte, 8)
			binary.BigEndian.PutUint64(buf, 1)
			if err := bucket.Put([]byte(pubkeySerialCounterKey), buf); err != nil {
				return err
			}
		} else {
			b.nextPubkeySeq = binary.BigEndian.Uint64(val)
		}

		return nil
	})
}

// persistSerialCounters saves the current serial counters to disk
func (b *B) persistSerialCounters() error {
	b.serialMu.Lock()
	eventSerial := b.nextSerial
	pubkeySerial := b.nextPubkeySeq
	b.serialMu.Unlock()

	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketMeta)
		if bucket == nil {
			return nil
		}

		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, eventSerial)
		if err := bucket.Put([]byte(serialCounterKey), buf); chk.E(err) {
			return err
		}

		binary.BigEndian.PutUint64(buf, pubkeySerial)
		if err := bucket.Put([]byte(pubkeySerialCounterKey), buf); chk.E(err) {
			return err
		}

		return nil
	})
}

// getNextEventSerial returns the next event serial number (thread-safe)
func (b *B) getNextEventSerial() uint64 {
	b.serialMu.Lock()
	defer b.serialMu.Unlock()

	serial := b.nextSerial
	b.nextSerial++

	// Persist every 1000 to reduce disk writes
	if b.nextSerial%1000 == 0 {
		go func() {
			if err := b.persistSerialCounters(); chk.E(err) {
				b.Logger.Warningf("bbolt: failed to persist serial counters: %v", err)
			}
		}()
	}

	return serial
}

// getNextPubkeySerial returns the next pubkey serial number (thread-safe)
func (b *B) getNextPubkeySerial() uint64 {
	b.serialMu.Lock()
	defer b.serialMu.Unlock()

	serial := b.nextPubkeySeq
	b.nextPubkeySeq++

	// Persist every 1000 to reduce disk writes
	if b.nextPubkeySeq%1000 == 0 {
		go func() {
			if err := b.persistSerialCounters(); chk.E(err) {
				b.Logger.Warningf("bbolt: failed to persist serial counters: %v", err)
			}
		}()
	}

	return serial
}

// getOrCreatePubkeySerial gets or creates a serial for a pubkey
func (b *B) getOrCreatePubkeySerial(tx *bolt.Tx, pubkey []byte) (uint64, error) {
	pksBucket := tx.Bucket(bucketPks)
	spkBucket := tx.Bucket(bucketSpk)

	if pksBucket == nil || spkBucket == nil {
		return 0, nil
	}

	// Check if pubkey already has a serial
	pubkeyHash := hashPubkey(pubkey)
	val := pksBucket.Get(pubkeyHash)
	if val != nil {
		return decodeUint40(val), nil
	}

	// Create new serial
	serial := b.getNextPubkeySerial()
	serialKey := makeSerialKey(serial)

	// Store pubkey_hash -> serial
	serialBuf := make([]byte, 5)
	encodeUint40(serial, serialBuf)
	if err := pksBucket.Put(pubkeyHash, serialBuf); err != nil {
		return 0, err
	}

	// Store serial -> full pubkey
	fullPubkey := make([]byte, 32)
	copy(fullPubkey, pubkey)
	if err := spkBucket.Put(serialKey, fullPubkey); err != nil {
		return 0, err
	}

	return serial, nil
}

// getPubkeyBySerial retrieves the full 32-byte pubkey from a serial
func (b *B) getPubkeyBySerial(tx *bolt.Tx, serial uint64) ([]byte, error) {
	spkBucket := tx.Bucket(bucketSpk)
	if spkBucket == nil {
		return nil, nil
	}

	serialKey := makeSerialKey(serial)
	return spkBucket.Get(serialKey), nil
}
