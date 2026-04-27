//go:build js && wasm

package wasmdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/aperturerobotics/go-indexeddb/idb"
	"github.com/hack-pad/safejs"

	"git.smesh.lol/orly/pkg/database"
)

const (
	// NIP43StoreName is the object store for NIP-43 membership
	NIP43StoreName = "nip43"

	// InvitesStoreName is the object store for invite codes
	InvitesStoreName = "invites"
)

// AddNIP43Member adds a pubkey as a NIP-43 member with the given invite code
func (w *W) AddNIP43Member(pubkey []byte, inviteCode string) error {
	if len(pubkey) != 32 {
		return errors.New("invalid pubkey length")
	}

	// Create membership record
	membership := &database.NIP43Membership{
		Pubkey:     make([]byte, 32),
		InviteCode: inviteCode,
		AddedAt:    time.Now(),
	}
	copy(membership.Pubkey, pubkey)

	// Serialize membership
	data := w.serializeNIP43Membership(membership)

	// Store using pubkey as key
	return w.setStoreValue(NIP43StoreName, string(pubkey), data)
}

// RemoveNIP43Member removes a pubkey from NIP-43 membership
func (w *W) RemoveNIP43Member(pubkey []byte) error {
	return w.deleteStoreValue(NIP43StoreName, string(pubkey))
}

// IsNIP43Member checks if a pubkey is a NIP-43 member
func (w *W) IsNIP43Member(pubkey []byte) (isMember bool, err error) {
	data, err := w.getStoreValue(NIP43StoreName, string(pubkey))
	if err != nil {
		return false, nil // Not found is not an error, just not a member
	}
	return data != nil, nil
}

// GetNIP43Membership returns the full membership details for a pubkey
func (w *W) GetNIP43Membership(pubkey []byte) (*database.NIP43Membership, error) {
	data, err := w.getStoreValue(NIP43StoreName, string(pubkey))
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errors.New("membership not found")
	}

	return w.deserializeNIP43Membership(data)
}

// GetAllNIP43Members returns all NIP-43 member pubkeys
func (w *W) GetAllNIP43Members() ([][]byte, error) {
	tx, err := w.db.Transaction(idb.TransactionReadOnly, NIP43StoreName)
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(NIP43StoreName)
	if err != nil {
		return nil, err
	}

	var members [][]byte

	cursorReq, err := store.OpenCursor(idb.CursorNext)
	if err != nil {
		return nil, err
	}

	err = cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		// Key is the pubkey stored as string
		keyBytes := safeValueToBytes(keyVal)
		if len(keyBytes) == 32 {
			pubkey := make([]byte, 32)
			copy(pubkey, keyBytes)
			members = append(members, pubkey)
		}

		return cursor.Continue()
	})

	if err != nil && err.Error() != "found" {
		return nil, err
	}

	return members, nil
}

// StoreInviteCode stores an invite code with expiration time
func (w *W) StoreInviteCode(code string, expiresAt time.Time) error {
	// Serialize expiration time as unix timestamp
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(expiresAt.Unix()))

	return w.setStoreValue(InvitesStoreName, code, data)
}

// ValidateInviteCode checks if an invite code is valid (exists and not expired)
func (w *W) ValidateInviteCode(code string) (valid bool, err error) {
	data, err := w.getStoreValue(InvitesStoreName, code)
	if err != nil {
		return false, nil
	}
	if data == nil || len(data) < 8 {
		return false, nil
	}

	// Check expiration
	expiresAt := time.Unix(int64(binary.BigEndian.Uint64(data)), 0)
	if time.Now().After(expiresAt) {
		return false, nil
	}

	return true, nil
}

// DeleteInviteCode removes an invite code
func (w *W) DeleteInviteCode(code string) error {
	return w.deleteStoreValue(InvitesStoreName, code)
}

// PublishNIP43MembershipEvent is a no-op in WASM (events are handled by the relay)
func (w *W) PublishNIP43MembershipEvent(kind int, pubkey []byte) error {
	// In WASM context, this would typically be handled by the client
	// This is a no-op implementation
	return nil
}

// serializeNIP43Membership converts a membership to bytes for storage
func (w *W) serializeNIP43Membership(m *database.NIP43Membership) []byte {
	buf := new(bytes.Buffer)

	// Write pubkey (32 bytes)
	buf.Write(m.Pubkey)

	// Write AddedAt as unix timestamp (8 bytes)
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, uint64(m.AddedAt.Unix()))
	buf.Write(ts)

	// Write invite code length (4 bytes) + invite code
	codeBytes := []byte(m.InviteCode)
	codeLen := make([]byte, 4)
	binary.BigEndian.PutUint32(codeLen, uint32(len(codeBytes)))
	buf.Write(codeLen)
	buf.Write(codeBytes)

	return buf.Bytes()
}

// deserializeNIP43Membership converts bytes back to a membership
func (w *W) deserializeNIP43Membership(data []byte) (*database.NIP43Membership, error) {
	if len(data) < 44 { // 32 + 8 + 4 minimum
		return nil, errors.New("invalid membership data")
	}

	m := &database.NIP43Membership{}

	// Read pubkey
	m.Pubkey = make([]byte, 32)
	copy(m.Pubkey, data[:32])

	// Read AddedAt
	m.AddedAt = time.Unix(int64(binary.BigEndian.Uint64(data[32:40])), 0)

	// Read invite code
	codeLen := binary.BigEndian.Uint32(data[40:44])
	if len(data) < int(44+codeLen) {
		return nil, errors.New("invalid invite code length")
	}
	m.InviteCode = string(data[44 : 44+codeLen])

	return m, nil
}

// Helper to convert safejs.Value to string for keys
func safeValueToString(v safejs.Value) string {
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	str, err := v.String()
	if err != nil {
		return ""
	}
	return str
}
