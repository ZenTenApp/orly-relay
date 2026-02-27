//go:build !(js && wasm)

package database

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

// Database key prefixes for NIP-43
const (
	nip43MemberPrefix = "nip43:member:"
	nip43InvitePrefix = "nip43:invite:"
)

// AddNIP43Member adds a member to the NIP-43 membership list
func (d *D) AddNIP43Member(pubkey []byte, inviteCode string) error {
	if len(pubkey) != 32 {
		return fmt.Errorf("invalid pubkey length: %d", len(pubkey))
	}

	key := append([]byte(nip43MemberPrefix), pubkey...)

	// Create membership record
	membership := NIP43Membership{
		Pubkey:     pubkey,
		AddedAt:    time.Now(),
		InviteCode: inviteCode,
	}

	// Serialize membership data
	val := serializeNIP43Membership(membership)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
	})
}

// RemoveNIP43Member removes a member from the NIP-43 membership list
func (d *D) RemoveNIP43Member(pubkey []byte) error {
	if len(pubkey) != 32 {
		return fmt.Errorf("invalid pubkey length: %d", len(pubkey))
	}

	key := append([]byte(nip43MemberPrefix), pubkey...)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// IsNIP43Member checks if a pubkey is a NIP-43 member
func (d *D) IsNIP43Member(pubkey []byte) (isMember bool, err error) {
	if len(pubkey) != 32 {
		return false, fmt.Errorf("invalid pubkey length: %d", len(pubkey))
	}

	key := append([]byte(nip43MemberPrefix), pubkey...)

	err = d.DB.View(func(txn *badger.Txn) error {
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			isMember = false
			return nil
		}
		if err != nil {
			return err
		}
		isMember = true
		return nil
	})

	return isMember, err
}

// GetNIP43Membership retrieves membership details for a pubkey
func (d *D) GetNIP43Membership(pubkey []byte) (*NIP43Membership, error) {
	if len(pubkey) != 32 {
		return nil, fmt.Errorf("invalid pubkey length: %d", len(pubkey))
	}

	key := append([]byte(nip43MemberPrefix), pubkey...)
	var membership *NIP43Membership

	err := d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			membership = deserializeNIP43Membership(val)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return membership, nil
}

// GetAllNIP43Members returns all NIP-43 members
func (d *D) GetAllNIP43Members() ([][]byte, error) {
	var members [][]byte
	prefix := []byte(nip43MemberPrefix)

	err := d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false // We only need keys

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()
			// Extract pubkey from key (skip prefix)
			pubkey := make([]byte, 32)
			copy(pubkey, key[len(prefix):])
			members = append(members, pubkey)
		}

		return nil
	})

	return members, err
}

// StoreInviteCode stores an invite code with expiry
func (d *D) StoreInviteCode(code string, expiresAt time.Time) error {
	key := append([]byte(nip43InvitePrefix), []byte(code)...)

	// Serialize expiry time as unix timestamp
	val := make([]byte, 8)
	binary.BigEndian.PutUint64(val, uint64(expiresAt.Unix()))

	return d.DB.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, val).WithTTL(time.Until(expiresAt))
		return txn.SetEntry(entry)
	})
}

// ValidateInviteCode checks if an invite code is valid and not expired
func (d *D) ValidateInviteCode(code string) (valid bool, err error) {
	key := append([]byte(nip43InvitePrefix), []byte(code)...)

	err = d.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			valid = false
			return nil
		}
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if len(val) != 8 {
				return fmt.Errorf("invalid invite code value")
			}
			expiresAt := int64(binary.BigEndian.Uint64(val))
			valid = time.Now().Unix() < expiresAt
			return nil
		})
	})

	return valid, err
}

// DeleteInviteCode removes an invite code (after use)
func (d *D) DeleteInviteCode(code string) error {
	key := append([]byte(nip43InvitePrefix), []byte(code)...)

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})
}

// Helper functions for serialization

func serializeNIP43Membership(m NIP43Membership) []byte {
	// Format: [pubkey(32)] [timestamp(8)] [invite_code_len(2)] [invite_code]
	codeBytes := []byte(m.InviteCode)
	codeLen := len(codeBytes)

	buf := make([]byte, 32+8+2+codeLen)

	// Copy pubkey
	copy(buf[0:32], m.Pubkey)

	// Write timestamp
	binary.BigEndian.PutUint64(buf[32:40], uint64(m.AddedAt.Unix()))

	// Write invite code length
	binary.BigEndian.PutUint16(buf[40:42], uint16(codeLen))

	// Write invite code
	copy(buf[42:], codeBytes)

	return buf
}

func deserializeNIP43Membership(data []byte) *NIP43Membership {
	if len(data) < 42 {
		return nil
	}

	m := &NIP43Membership{}

	// Read pubkey
	m.Pubkey = make([]byte, 32)
	copy(m.Pubkey, data[0:32])

	// Read timestamp
	timestamp := binary.BigEndian.Uint64(data[32:40])
	m.AddedAt = time.Unix(int64(timestamp), 0)

	// Read invite code
	codeLen := binary.BigEndian.Uint16(data[40:42])
	if len(data) >= 42+int(codeLen) {
		m.InviteCode = string(data[42 : 42+codeLen])
	}

	return m
}

// PublishNIP43MembershipEvent publishes membership change events
func (d *D) PublishNIP43MembershipEvent(kind int, pubkey []byte) error {
	log.I.F("publishing NIP-43 event kind %d for pubkey %s", kind, hex.Enc(pubkey))

	// Get relay identity
	relaySecret, err := d.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		return err
	}

	// This would integrate with the event publisher
	// For now, just log it
	log.D.F("would publish kind %d event for member %s", kind, hex.Enc(pubkey))

	// The actual publishing will be done by the handler
	_ = relaySecret

	return nil
}
