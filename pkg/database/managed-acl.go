//go:build !(js && wasm)

package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// ManagedACLConfig represents the configuration for managed ACL mode
type ManagedACLConfig struct {
	RelayName        string `json:"relay_name"`
	RelayDescription string `json:"relay_description"`
	RelayIcon        string `json:"relay_icon"`
}

// BannedPubkey represents a banned public key entry
type BannedPubkey struct {
	Pubkey string    `json:"pubkey"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// AllowedPubkey represents an allowed public key entry
type AllowedPubkey struct {
	Pubkey string    `json:"pubkey"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// BannedEvent represents a banned event entry
type BannedEvent struct {
	ID     string    `json:"id"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// AllowedEvent represents an allowed event entry
type AllowedEvent struct {
	ID     string    `json:"id"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// BlockedIP represents a blocked IP address entry
type BlockedIP struct {
	IP     string    `json:"ip"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// AllowedKind represents an allowed event kind
type AllowedKind struct {
	Kind  int       `json:"kind"`
	Added time.Time `json:"added"`
}

// EventNeedingModeration represents an event that needs moderation
type EventNeedingModeration struct {
	ID     string    `json:"id"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// ManagedACL database operations
type ManagedACL struct {
	*D
}

// NewManagedACL creates a new ManagedACL instance
func NewManagedACL(db *D) *ManagedACL {
	return &ManagedACL{D: db}
}

// SaveBannedPubkey saves a banned pubkey to the database
func (m *ManagedACL) SaveBannedPubkey(pubkey string, reason string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getBannedPubkeyKey(pubkey)
		banned := BannedPubkey{
			Pubkey: pubkey,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(banned)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveBannedPubkey removes a banned pubkey from the database
func (m *ManagedACL) RemoveBannedPubkey(pubkey string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getBannedPubkeyKey(pubkey)
		return txn.Delete(key)
	})
}

// ListBannedPubkeys returns all banned pubkeys
func (m *ManagedACL) ListBannedPubkeys() ([]BannedPubkey, error) {
	var banned []BannedPubkey
	return banned, m.View(func(txn *badger.Txn) error {
		prefix := m.getBannedPubkeyPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var bannedPubkey BannedPubkey
			if err := json.Unmarshal(val, &bannedPubkey); err != nil {
				continue
			}
			banned = append(banned, bannedPubkey)
		}
		return nil
	})
}

// SaveAllowedPubkey saves an allowed pubkey to the database
func (m *ManagedACL) SaveAllowedPubkey(pubkey string, reason string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getAllowedPubkeyKey(pubkey)
		allowed := AllowedPubkey{
			Pubkey: pubkey,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(allowed)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveAllowedPubkey removes an allowed pubkey from the database
func (m *ManagedACL) RemoveAllowedPubkey(pubkey string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getAllowedPubkeyKey(pubkey)
		return txn.Delete(key)
	})
}

// ListAllowedPubkeys returns all allowed pubkeys
func (m *ManagedACL) ListAllowedPubkeys() ([]AllowedPubkey, error) {
	var allowed []AllowedPubkey
	return allowed, m.View(func(txn *badger.Txn) error {
		prefix := m.getAllowedPubkeyPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var allowedPubkey AllowedPubkey
			if err := json.Unmarshal(val, &allowedPubkey); err != nil {
				continue
			}
			allowed = append(allowed, allowedPubkey)
		}
		return nil
	})
}

// SaveBannedEvent saves a banned event to the database
func (m *ManagedACL) SaveBannedEvent(eventID string, reason string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getBannedEventKey(eventID)
		banned := BannedEvent{
			ID:     eventID,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(banned)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveBannedEvent removes a banned event from the database
func (m *ManagedACL) RemoveBannedEvent(eventID string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getBannedEventKey(eventID)
		return txn.Delete(key)
	})
}

// ListBannedEvents returns all banned events
func (m *ManagedACL) ListBannedEvents() ([]BannedEvent, error) {
	var banned []BannedEvent
	return banned, m.View(func(txn *badger.Txn) error {
		prefix := m.getBannedEventPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var bannedEvent BannedEvent
			if err := json.Unmarshal(val, &bannedEvent); err != nil {
				continue
			}
			banned = append(banned, bannedEvent)
		}
		return nil
	})
}

// SaveAllowedEvent saves an allowed event to the database
func (m *ManagedACL) SaveAllowedEvent(eventID string, reason string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getAllowedEventKey(eventID)
		allowed := AllowedEvent{
			ID:     eventID,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(allowed)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveAllowedEvent removes an allowed event from the database
func (m *ManagedACL) RemoveAllowedEvent(eventID string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getAllowedEventKey(eventID)
		return txn.Delete(key)
	})
}

// ListAllowedEvents returns all allowed events
func (m *ManagedACL) ListAllowedEvents() ([]AllowedEvent, error) {
	var allowed []AllowedEvent
	return allowed, m.View(func(txn *badger.Txn) error {
		prefix := m.getAllowedEventPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var allowedEvent AllowedEvent
			if err := json.Unmarshal(val, &allowedEvent); err != nil {
				continue
			}
			allowed = append(allowed, allowedEvent)
		}
		return nil
	})
}

// SaveBlockedIP saves a blocked IP to the database
func (m *ManagedACL) SaveBlockedIP(ip string, reason string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getBlockedIPKey(ip)
		blocked := BlockedIP{
			IP:     ip,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(blocked)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveBlockedIP removes a blocked IP from the database
func (m *ManagedACL) RemoveBlockedIP(ip string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getBlockedIPKey(ip)
		return txn.Delete(key)
	})
}

// ListBlockedIPs returns all blocked IPs
func (m *ManagedACL) ListBlockedIPs() ([]BlockedIP, error) {
	var blocked []BlockedIP
	return blocked, m.View(func(txn *badger.Txn) error {
		prefix := m.getBlockedIPPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var blockedIP BlockedIP
			if err := json.Unmarshal(val, &blockedIP); err != nil {
				continue
			}
			blocked = append(blocked, blockedIP)
		}
		return nil
	})
}

// SaveAllowedKind saves an allowed kind to the database
func (m *ManagedACL) SaveAllowedKind(kind int) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getAllowedKindKey(kind)
		allowed := AllowedKind{
			Kind:  kind,
			Added: time.Now(),
		}
		data, err := json.Marshal(allowed)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveAllowedKind removes an allowed kind from the database
func (m *ManagedACL) RemoveAllowedKind(kind int) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getAllowedKindKey(kind)
		return txn.Delete(key)
	})
}

// ListAllowedKinds returns all allowed kinds
func (m *ManagedACL) ListAllowedKinds() ([]int, error) {
	var kinds []int
	return kinds, m.View(func(txn *badger.Txn) error {
		prefix := m.getAllowedKindPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var allowedKind AllowedKind
			if err := json.Unmarshal(val, &allowedKind); err != nil {
				continue
			}
			kinds = append(kinds, allowedKind.Kind)
		}
		return nil
	})
}

// SaveEventNeedingModeration saves an event that needs moderation
func (m *ManagedACL) SaveEventNeedingModeration(eventID string, reason string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getEventNeedingModerationKey(eventID)
		event := EventNeedingModeration{
			ID:     eventID,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveEventNeedingModeration removes an event from moderation queue
func (m *ManagedACL) RemoveEventNeedingModeration(eventID string) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getEventNeedingModerationKey(eventID)
		return txn.Delete(key)
	})
}

// ListEventsNeedingModeration returns all events needing moderation
func (m *ManagedACL) ListEventsNeedingModeration() ([]EventNeedingModeration, error) {
	var events []EventNeedingModeration
	return events, m.View(func(txn *badger.Txn) error {
		prefix := m.getEventNeedingModerationPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var event EventNeedingModeration
			if err := json.Unmarshal(val, &event); err != nil {
				continue
			}
			events = append(events, event)
		}
		return nil
	})
}

// SaveRelayConfig saves relay configuration
func (m *ManagedACL) SaveRelayConfig(config ManagedACLConfig) error {
	return m.Update(func(txn *badger.Txn) error {
		key := m.getRelayConfigKey()
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// GetRelayConfig returns relay configuration
func (m *ManagedACL) GetRelayConfig() (ManagedACLConfig, error) {
	var config ManagedACLConfig
	return config, m.View(func(txn *badger.Txn) error {
		key := m.getRelayConfigKey()
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				// Return default config
				config = ManagedACLConfig{
					RelayName:        "Managed Relay",
					RelayDescription: "A managed Nostr relay",
					RelayIcon:        "",
				}
				return nil
			}
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &config)
	})
}

// Check if a pubkey is banned
func (m *ManagedACL) IsPubkeyBanned(pubkey string) (bool, error) {
	var banned bool
	return banned, m.View(func(txn *badger.Txn) error {
		key := m.getBannedPubkeyKey(pubkey)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			banned = false
			return nil
		}
		if err != nil {
			return err
		}
		banned = true
		return nil
	})
}

// Check if a pubkey is explicitly allowed
func (m *ManagedACL) IsPubkeyAllowed(pubkey string) (bool, error) {
	var allowed bool
	return allowed, m.View(func(txn *badger.Txn) error {
		key := m.getAllowedPubkeyKey(pubkey)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			allowed = false
			return nil
		}
		if err != nil {
			return err
		}
		allowed = true
		return nil
	})
}

// Check if an event is banned
func (m *ManagedACL) IsEventBanned(eventID string) (bool, error) {
	var banned bool
	return banned, m.View(func(txn *badger.Txn) error {
		key := m.getBannedEventKey(eventID)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			banned = false
			return nil
		}
		if err != nil {
			return err
		}
		banned = true
		return nil
	})
}

// Check if an event is explicitly allowed
func (m *ManagedACL) IsEventAllowed(eventID string) (bool, error) {
	var allowed bool
	return allowed, m.View(func(txn *badger.Txn) error {
		key := m.getAllowedEventKey(eventID)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			allowed = false
			return nil
		}
		if err != nil {
			return err
		}
		allowed = true
		return nil
	})
}

// Check if an IP is blocked
func (m *ManagedACL) IsIPBlocked(ip string) (bool, error) {
	var blocked bool
	return blocked, m.View(func(txn *badger.Txn) error {
		key := m.getBlockedIPKey(ip)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			blocked = false
			return nil
		}
		if err != nil {
			return err
		}
		blocked = true
		return nil
	})
}

// Check if a kind is allowed
func (m *ManagedACL) IsKindAllowed(kind int) (bool, error) {
	var allowed bool
	return allowed, m.View(func(txn *badger.Txn) error {
		key := m.getAllowedKindKey(kind)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			allowed = false
			return nil
		}
		if err != nil {
			return err
		}
		allowed = true
		return nil
	})
}

// Key generation methods
func (m *ManagedACL) getBannedPubkeyKey(pubkey string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_BANNED_PUBKEY_")
	buf.WriteString(pubkey)
	return buf.Bytes()
}

func (m *ManagedACL) getBannedPubkeyPrefix() []byte {
	return []byte("MANAGED_ACL_BANNED_PUBKEY_")
}

func (m *ManagedACL) getAllowedPubkeyKey(pubkey string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_ALLOWED_PUBKEY_")
	buf.WriteString(pubkey)
	return buf.Bytes()
}

func (m *ManagedACL) getAllowedPubkeyPrefix() []byte {
	return []byte("MANAGED_ACL_ALLOWED_PUBKEY_")
}

func (m *ManagedACL) getBannedEventKey(eventID string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_BANNED_EVENT_")
	buf.WriteString(eventID)
	return buf.Bytes()
}

func (m *ManagedACL) getBannedEventPrefix() []byte {
	return []byte("MANAGED_ACL_BANNED_EVENT_")
}

func (m *ManagedACL) getAllowedEventKey(eventID string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_ALLOWED_EVENT_")
	buf.WriteString(eventID)
	return buf.Bytes()
}

func (m *ManagedACL) getAllowedEventPrefix() []byte {
	return []byte("MANAGED_ACL_ALLOWED_EVENT_")
}

func (m *ManagedACL) getBlockedIPKey(ip string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_BLOCKED_IP_")
	buf.WriteString(ip)
	return buf.Bytes()
}

func (m *ManagedACL) getBlockedIPPrefix() []byte {
	return []byte("MANAGED_ACL_BLOCKED_IP_")
}

func (m *ManagedACL) getAllowedKindKey(kind int) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_ALLOWED_KIND_")
	buf.WriteString(fmt.Sprintf("%d", kind))
	return buf.Bytes()
}

func (m *ManagedACL) getAllowedKindPrefix() []byte {
	return []byte("MANAGED_ACL_ALLOWED_KIND_")
}

func (m *ManagedACL) getEventNeedingModerationKey(eventID string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("MANAGED_ACL_MODERATION_")
	buf.WriteString(eventID)
	return buf.Bytes()
}

func (m *ManagedACL) getEventNeedingModerationPrefix() []byte {
	return []byte("MANAGED_ACL_MODERATION_")
}

func (m *ManagedACL) getRelayConfigKey() []byte {
	return []byte("MANAGED_ACL_RELAY_CONFIG")
}
