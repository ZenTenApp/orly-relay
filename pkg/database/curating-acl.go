//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/minio/sha256-simd"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// CuratingConfig represents the configuration for curating ACL mode
// This is parsed from a kind 30078 event with d-tag "curating-config"
type CuratingConfig struct {
	DailyLimit       int      `json:"daily_limit"`        // Max events per day for unclassified users
	IPDailyLimit     int      `json:"ip_daily_limit"`     // Max events per day from a single IP (flood protection)
	FirstBanHours    int      `json:"first_ban_hours"`    // IP ban duration for first offense
	SecondBanHours   int      `json:"second_ban_hours"`   // IP ban duration for second+ offense
	AllowedKinds     []int    `json:"allowed_kinds"`      // Explicit kind numbers
	AllowedRanges    []string `json:"allowed_ranges"`     // Kind ranges like "1000-1999"
	KindCategories   []string `json:"kind_categories"`    // Category IDs like "social", "dm"
	ConfigEventID    string   `json:"config_event_id"`    // ID of the config event
	ConfigPubkey     string   `json:"config_pubkey"`      // Pubkey that published config
	ConfiguredAt     int64    `json:"configured_at"`      // Timestamp of config event
}

// TrustedPubkey represents an explicitly trusted publisher
type TrustedPubkey struct {
	Pubkey string    `json:"pubkey"`
	Note   string    `json:"note,omitempty"`
	Added  time.Time `json:"added"`
}

// BlacklistedPubkey represents a blacklisted publisher
type BlacklistedPubkey struct {
	Pubkey string    `json:"pubkey"`
	Reason string    `json:"reason,omitempty"`
	Added  time.Time `json:"added"`
}

// PubkeyEventCount tracks daily event counts for rate limiting
type PubkeyEventCount struct {
	Pubkey    string    `json:"pubkey"`
	Date      string    `json:"date"` // YYYY-MM-DD format
	Count     int       `json:"count"`
	LastEvent time.Time `json:"last_event"`
}

// IPOffense tracks rate limit violations from IPs
type IPOffense struct {
	IP           string    `json:"ip"`
	OffenseCount int       `json:"offense_count"`
	PubkeysHit   []string  `json:"pubkeys_hit"` // Pubkeys that hit rate limit from this IP
	LastOffense  time.Time `json:"last_offense"`
}

// CuratingBlockedIP represents a temporarily blocked IP with expiration
type CuratingBlockedIP struct {
	IP        string    `json:"ip"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expires_at"`
	Added     time.Time `json:"added"`
}

// SpamEvent represents an event flagged as spam
type SpamEvent struct {
	EventID string    `json:"event_id"`
	Pubkey  string    `json:"pubkey"`
	Reason  string    `json:"reason,omitempty"`
	Added   time.Time `json:"added"`
}

// UnclassifiedUser represents a user who hasn't been trusted or blacklisted
type UnclassifiedUser struct {
	Pubkey     string    `json:"pubkey"`
	EventCount int       `json:"event_count"`
	LastEvent  time.Time `json:"last_event"`
}

// CuratingACL database operations
type CuratingACL struct {
	*D
}

// NewCuratingACL creates a new CuratingACL instance
func NewCuratingACL(db *D) *CuratingACL {
	return &CuratingACL{D: db}
}

// ==================== Configuration ====================

// SaveConfig saves the curating configuration
func (c *CuratingACL) SaveConfig(config CuratingConfig) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getConfigKey()
		data, err := json.Marshal(config)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// GetConfig returns the curating configuration
func (c *CuratingACL) GetConfig() (CuratingConfig, error) {
	var config CuratingConfig
	err := c.View(func(txn *badger.Txn) error {
		key := c.getConfigKey()
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // Return empty config
			}
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &config)
	})
	return config, err
}

// IsConfigured returns true if a configuration event has been set
func (c *CuratingACL) IsConfigured() (bool, error) {
	config, err := c.GetConfig()
	if err != nil {
		return false, err
	}
	return config.ConfigEventID != "", nil
}

// ==================== Trusted Pubkeys ====================

// SaveTrustedPubkey saves a trusted pubkey to the database
func (c *CuratingACL) SaveTrustedPubkey(pubkey string, note string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getTrustedPubkeyKey(pubkey)
		trusted := TrustedPubkey{
			Pubkey: pubkey,
			Note:   note,
			Added:  time.Now(),
		}
		data, err := json.Marshal(trusted)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveTrustedPubkey removes a trusted pubkey from the database
func (c *CuratingACL) RemoveTrustedPubkey(pubkey string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getTrustedPubkeyKey(pubkey)
		return txn.Delete(key)
	})
}

// ListTrustedPubkeys returns all trusted pubkeys
func (c *CuratingACL) ListTrustedPubkeys() ([]TrustedPubkey, error) {
	var trusted []TrustedPubkey
	err := c.View(func(txn *badger.Txn) error {
		prefix := c.getTrustedPubkeyPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var t TrustedPubkey
			if err := json.Unmarshal(val, &t); err != nil {
				continue
			}
			trusted = append(trusted, t)
		}
		return nil
	})
	return trusted, err
}

// IsPubkeyTrusted checks if a pubkey is trusted
func (c *CuratingACL) IsPubkeyTrusted(pubkey string) (bool, error) {
	var trusted bool
	err := c.View(func(txn *badger.Txn) error {
		key := c.getTrustedPubkeyKey(pubkey)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			trusted = false
			return nil
		}
		if err != nil {
			return err
		}
		trusted = true
		return nil
	})
	return trusted, err
}

// ==================== Blacklisted Pubkeys ====================

// SaveBlacklistedPubkey saves a blacklisted pubkey to the database
func (c *CuratingACL) SaveBlacklistedPubkey(pubkey string, reason string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getBlacklistedPubkeyKey(pubkey)
		blacklisted := BlacklistedPubkey{
			Pubkey: pubkey,
			Reason: reason,
			Added:  time.Now(),
		}
		data, err := json.Marshal(blacklisted)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// RemoveBlacklistedPubkey removes a blacklisted pubkey from the database
func (c *CuratingACL) RemoveBlacklistedPubkey(pubkey string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getBlacklistedPubkeyKey(pubkey)
		return txn.Delete(key)
	})
}

// ListBlacklistedPubkeys returns all blacklisted pubkeys
func (c *CuratingACL) ListBlacklistedPubkeys() ([]BlacklistedPubkey, error) {
	var blacklisted []BlacklistedPubkey
	err := c.View(func(txn *badger.Txn) error {
		prefix := c.getBlacklistedPubkeyPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var b BlacklistedPubkey
			if err := json.Unmarshal(val, &b); err != nil {
				continue
			}
			blacklisted = append(blacklisted, b)
		}
		return nil
	})
	return blacklisted, err
}

// IsPubkeyBlacklisted checks if a pubkey is blacklisted
func (c *CuratingACL) IsPubkeyBlacklisted(pubkey string) (bool, error) {
	var blacklisted bool
	err := c.View(func(txn *badger.Txn) error {
		key := c.getBlacklistedPubkeyKey(pubkey)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			blacklisted = false
			return nil
		}
		if err != nil {
			return err
		}
		blacklisted = true
		return nil
	})
	return blacklisted, err
}

// ==================== Event Counting ====================

// GetEventCount returns the event count for a pubkey on a specific date
func (c *CuratingACL) GetEventCount(pubkey, date string) (int, error) {
	var count int
	err := c.View(func(txn *badger.Txn) error {
		key := c.getEventCountKey(pubkey, date)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			count = 0
			return nil
		}
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		var ec PubkeyEventCount
		if err := json.Unmarshal(val, &ec); err != nil {
			return err
		}
		count = ec.Count
		return nil
	})
	return count, err
}

// IncrementEventCount increments and returns the new event count for a pubkey
func (c *CuratingACL) IncrementEventCount(pubkey, date string) (int, error) {
	var newCount int
	err := c.Update(func(txn *badger.Txn) error {
		key := c.getEventCountKey(pubkey, date)
		var ec PubkeyEventCount

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			ec = PubkeyEventCount{
				Pubkey:    pubkey,
				Date:      date,
				Count:     0,
				LastEvent: time.Now(),
			}
		} else if err != nil {
			return err
		} else {
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(val, &ec); err != nil {
				return err
			}
		}

		ec.Count++
		ec.LastEvent = time.Now()
		newCount = ec.Count

		data, err := json.Marshal(ec)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
	return newCount, err
}

// CleanupOldEventCounts removes event counts older than the specified date
func (c *CuratingACL) CleanupOldEventCounts(beforeDate string) error {
	return c.Update(func(txn *badger.Txn) error {
		prefix := c.getEventCountPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		var keysToDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var ec PubkeyEventCount
			if err := json.Unmarshal(val, &ec); err != nil {
				continue
			}
			if ec.Date < beforeDate {
				keysToDelete = append(keysToDelete, item.KeyCopy(nil))
			}
		}

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// ==================== IP Event Counting ====================

// IPEventCount tracks events from an IP address per day (flood protection)
type IPEventCount struct {
	IP        string    `json:"ip"`
	Date      string    `json:"date"`
	Count     int       `json:"count"`
	LastEvent time.Time `json:"last_event"`
}

// GetIPEventCount returns the total event count for an IP on a specific date
func (c *CuratingACL) GetIPEventCount(ip, date string) (int, error) {
	var count int
	err := c.View(func(txn *badger.Txn) error {
		key := c.getIPEventCountKey(ip, date)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			count = 0
			return nil
		}
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		var ec IPEventCount
		if err := json.Unmarshal(val, &ec); err != nil {
			return err
		}
		count = ec.Count
		return nil
	})
	return count, err
}

// IncrementIPEventCount increments and returns the new event count for an IP
func (c *CuratingACL) IncrementIPEventCount(ip, date string) (int, error) {
	var newCount int
	err := c.Update(func(txn *badger.Txn) error {
		key := c.getIPEventCountKey(ip, date)
		var ec IPEventCount

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			ec = IPEventCount{
				IP:        ip,
				Date:      date,
				Count:     0,
				LastEvent: time.Now(),
			}
		} else if err != nil {
			return err
		} else {
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(val, &ec); err != nil {
				return err
			}
		}

		ec.Count++
		ec.LastEvent = time.Now()
		newCount = ec.Count

		data, err := json.Marshal(ec)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
	return newCount, err
}

// CleanupOldIPEventCounts removes IP event counts older than the specified date
func (c *CuratingACL) CleanupOldIPEventCounts(beforeDate string) error {
	return c.Update(func(txn *badger.Txn) error {
		prefix := c.getIPEventCountPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		var keysToDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var ec IPEventCount
			if err := json.Unmarshal(val, &ec); err != nil {
				continue
			}
			if ec.Date < beforeDate {
				keysToDelete = append(keysToDelete, item.KeyCopy(nil))
			}
		}

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

func (c *CuratingACL) getIPEventCountKey(ip, date string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_IP_EVENT_COUNT_")
	buf.WriteString(ip)
	buf.WriteString("_")
	buf.WriteString(date)
	return buf.Bytes()
}

func (c *CuratingACL) getIPEventCountPrefix() []byte {
	return []byte("CURATING_ACL_IP_EVENT_COUNT_")
}

// ==================== IP Offense Tracking ====================

// GetIPOffense returns the offense record for an IP
func (c *CuratingACL) GetIPOffense(ip string) (*IPOffense, error) {
	var offense *IPOffense
	err := c.View(func(txn *badger.Txn) error {
		key := c.getIPOffenseKey(ip)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		offense = new(IPOffense)
		return json.Unmarshal(val, offense)
	})
	return offense, err
}

// RecordIPOffense records a rate limit violation from an IP for a pubkey
// Returns the new offense count
func (c *CuratingACL) RecordIPOffense(ip, pubkey string) (int, error) {
	var newCount int
	err := c.Update(func(txn *badger.Txn) error {
		key := c.getIPOffenseKey(ip)
		var offense IPOffense

		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			offense = IPOffense{
				IP:           ip,
				OffenseCount: 0,
				PubkeysHit:   []string{},
				LastOffense:  time.Now(),
			}
		} else if err != nil {
			return err
		} else {
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(val, &offense); err != nil {
				return err
			}
		}

		// Add pubkey if not already in list
		found := false
		for _, p := range offense.PubkeysHit {
			if p == pubkey {
				found = true
				break
			}
		}
		if !found {
			offense.PubkeysHit = append(offense.PubkeysHit, pubkey)
			offense.OffenseCount++
		}
		offense.LastOffense = time.Now()
		newCount = offense.OffenseCount

		data, err := json.Marshal(offense)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
	return newCount, err
}

// ==================== IP Blocking ====================

// BlockIP blocks an IP for a specified duration
func (c *CuratingACL) BlockIP(ip string, duration time.Duration, reason string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getBlockedIPKey(ip)
		blocked := CuratingBlockedIP{
			IP:        ip,
			Reason:    reason,
			ExpiresAt: time.Now().Add(duration),
			Added:     time.Now(),
		}
		data, err := json.Marshal(blocked)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// UnblockIP removes an IP from the blocked list
func (c *CuratingACL) UnblockIP(ip string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getBlockedIPKey(ip)
		return txn.Delete(key)
	})
}

// IsIPBlocked checks if an IP is blocked and returns expiration time
func (c *CuratingACL) IsIPBlocked(ip string) (bool, time.Time, error) {
	var blocked bool
	var expiresAt time.Time
	err := c.View(func(txn *badger.Txn) error {
		key := c.getBlockedIPKey(ip)
		item, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			blocked = false
			return nil
		}
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		var b CuratingBlockedIP
		if err := json.Unmarshal(val, &b); err != nil {
			return err
		}
		if time.Now().After(b.ExpiresAt) {
			// Block has expired
			blocked = false
			return nil
		}
		blocked = true
		expiresAt = b.ExpiresAt
		return nil
	})
	return blocked, expiresAt, err
}

// ListBlockedIPs returns all blocked IPs (including expired ones)
func (c *CuratingACL) ListBlockedIPs() ([]CuratingBlockedIP, error) {
	var blocked []CuratingBlockedIP
	err := c.View(func(txn *badger.Txn) error {
		prefix := c.getBlockedIPPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var b CuratingBlockedIP
			if err := json.Unmarshal(val, &b); err != nil {
				continue
			}
			blocked = append(blocked, b)
		}
		return nil
	})
	return blocked, err
}

// CleanupExpiredIPBlocks removes expired IP blocks
func (c *CuratingACL) CleanupExpiredIPBlocks() error {
	return c.Update(func(txn *badger.Txn) error {
		prefix := c.getBlockedIPPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		now := time.Now()
		var keysToDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var b CuratingBlockedIP
			if err := json.Unmarshal(val, &b); err != nil {
				continue
			}
			if now.After(b.ExpiresAt) {
				keysToDelete = append(keysToDelete, item.KeyCopy(nil))
			}
		}

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// ==================== Spam Events ====================

// MarkEventAsSpam marks an event as spam
func (c *CuratingACL) MarkEventAsSpam(eventID, pubkey, reason string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getSpamEventKey(eventID)
		spam := SpamEvent{
			EventID: eventID,
			Pubkey:  pubkey,
			Reason:  reason,
			Added:   time.Now(),
		}
		data, err := json.Marshal(spam)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// UnmarkEventAsSpam removes the spam flag from an event
func (c *CuratingACL) UnmarkEventAsSpam(eventID string) error {
	return c.Update(func(txn *badger.Txn) error {
		key := c.getSpamEventKey(eventID)
		return txn.Delete(key)
	})
}

// IsEventSpam checks if an event is marked as spam
func (c *CuratingACL) IsEventSpam(eventID string) (bool, error) {
	var spam bool
	err := c.View(func(txn *badger.Txn) error {
		key := c.getSpamEventKey(eventID)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			spam = false
			return nil
		}
		if err != nil {
			return err
		}
		spam = true
		return nil
	})
	return spam, err
}

// ListSpamEvents returns all spam events
func (c *CuratingACL) ListSpamEvents() ([]SpamEvent, error) {
	var spam []SpamEvent
	err := c.View(func(txn *badger.Txn) error {
		prefix := c.getSpamEventPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var s SpamEvent
			if err := json.Unmarshal(val, &s); err != nil {
				continue
			}
			spam = append(spam, s)
		}
		return nil
	})
	return spam, err
}

// ==================== Unclassified Users ====================

// ListUnclassifiedUsers returns users who are neither trusted nor blacklisted
// sorted by event count descending
func (c *CuratingACL) ListUnclassifiedUsers(limit int) ([]UnclassifiedUser, error) {
	// First, get all trusted and blacklisted pubkeys to exclude
	trusted, err := c.ListTrustedPubkeys()
	if err != nil {
		return nil, err
	}
	blacklisted, err := c.ListBlacklistedPubkeys()
	if err != nil {
		return nil, err
	}

	excludeSet := make(map[string]struct{})
	for _, t := range trusted {
		excludeSet[t.Pubkey] = struct{}{}
	}
	for _, b := range blacklisted {
		excludeSet[b.Pubkey] = struct{}{}
	}

	// Now iterate through event counts and aggregate by pubkey
	pubkeyCounts := make(map[string]*UnclassifiedUser)

	err = c.View(func(txn *badger.Txn) error {
		prefix := c.getEventCountPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var ec PubkeyEventCount
			if err := json.Unmarshal(val, &ec); err != nil {
				continue
			}

			// Skip if trusted or blacklisted
			if _, excluded := excludeSet[ec.Pubkey]; excluded {
				continue
			}

			if existing, ok := pubkeyCounts[ec.Pubkey]; ok {
				existing.EventCount += ec.Count
				if ec.LastEvent.After(existing.LastEvent) {
					existing.LastEvent = ec.LastEvent
				}
			} else {
				pubkeyCounts[ec.Pubkey] = &UnclassifiedUser{
					Pubkey:     ec.Pubkey,
					EventCount: ec.Count,
					LastEvent:  ec.LastEvent,
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert to slice and sort by event count descending
	var users []UnclassifiedUser
	for _, u := range pubkeyCounts {
		users = append(users, *u)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].EventCount > users[j].EventCount
	})

	// Apply limit
	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}

	return users, nil
}

// ==================== Key Generation ====================

func (c *CuratingACL) getConfigKey() []byte {
	return []byte("CURATING_ACL_CONFIG")
}

func (c *CuratingACL) getTrustedPubkeyKey(pubkey string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_TRUSTED_PUBKEY_")
	buf.WriteString(pubkey)
	return buf.Bytes()
}

func (c *CuratingACL) getTrustedPubkeyPrefix() []byte {
	return []byte("CURATING_ACL_TRUSTED_PUBKEY_")
}

func (c *CuratingACL) getBlacklistedPubkeyKey(pubkey string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_BLACKLISTED_PUBKEY_")
	buf.WriteString(pubkey)
	return buf.Bytes()
}

func (c *CuratingACL) getBlacklistedPubkeyPrefix() []byte {
	return []byte("CURATING_ACL_BLACKLISTED_PUBKEY_")
}

func (c *CuratingACL) getEventCountKey(pubkey, date string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_EVENT_COUNT_")
	buf.WriteString(pubkey)
	buf.WriteString("_")
	buf.WriteString(date)
	return buf.Bytes()
}

func (c *CuratingACL) getEventCountPrefix() []byte {
	return []byte("CURATING_ACL_EVENT_COUNT_")
}

func (c *CuratingACL) getIPOffenseKey(ip string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_IP_OFFENSE_")
	buf.WriteString(ip)
	return buf.Bytes()
}

func (c *CuratingACL) getBlockedIPKey(ip string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_BLOCKED_IP_")
	buf.WriteString(ip)
	return buf.Bytes()
}

func (c *CuratingACL) getBlockedIPPrefix() []byte {
	return []byte("CURATING_ACL_BLOCKED_IP_")
}

func (c *CuratingACL) getSpamEventKey(eventID string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("CURATING_ACL_SPAM_EVENT_")
	buf.WriteString(eventID)
	return buf.Bytes()
}

func (c *CuratingACL) getSpamEventPrefix() []byte {
	return []byte("CURATING_ACL_SPAM_EVENT_")
}

// ==================== Kind Checking Helpers ====================

// IsKindAllowed checks if an event kind is allowed based on config
func (c *CuratingACL) IsKindAllowed(kind int, config *CuratingConfig) bool {
	if config == nil {
		return false
	}

	// Check explicit kinds
	for _, k := range config.AllowedKinds {
		if k == kind {
			return true
		}
	}

	// Check ranges
	for _, rangeStr := range config.AllowedRanges {
		if kindInRange(kind, rangeStr) {
			return true
		}
	}

	// Check categories
	for _, cat := range config.KindCategories {
		if kindInCategory(kind, cat) {
			return true
		}
	}

	return false
}

// kindInRange checks if a kind is within a range string like "1000-1999"
func kindInRange(kind int, rangeStr string) bool {
	var start, end int
	n, err := fmt.Sscanf(rangeStr, "%d-%d", &start, &end)
	if err != nil || n != 2 {
		return false
	}
	return kind >= start && kind <= end
}

// kindInCategory checks if a kind belongs to a predefined category
func kindInCategory(kind int, category string) bool {
	categories := map[string][]int{
		"social":              {0, 1, 3, 6, 7, 10002},
		"dm":                  {4, 14, 1059},
		"longform":            {30023, 30024},
		"media":               {1063, 20, 21, 22},
		"marketplace":         {30017, 30018, 30019, 30020, 1021, 1022}, // Legacy alias
		"marketplace_nip15":   {30017, 30018, 30019, 30020, 1021, 1022},
		"marketplace_nip99":   {30402, 30403, 30405, 30406, 31555}, // NIP-99/Gamma Markets (Plebeian Market)
		"order_communication": {16, 17},                            // Gamma Markets order messages
		"groups_nip29":        {9, 10, 11, 12, 9000, 9001, 9002, 39000, 39001, 39002},
		"groups_nip72":        {34550, 1111, 4550},
		"lists":               {10000, 10001, 10003, 30000, 30001, 30003},
	}

	kinds, ok := categories[category]
	if !ok {
		return false
	}

	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// ==================== Database Scanning ====================

// ScanResult contains the results of scanning all pubkeys in the database
type ScanResult struct {
	TotalPubkeys int `json:"total_pubkeys"`
	TotalEvents  int `json:"total_events"`
	Skipped      int `json:"skipped"` // Trusted/blacklisted users skipped
}

// ScanAllPubkeys scans the database to find all unique pubkeys and count their events.
// This populates the event count data needed for the unclassified users list.
// It uses the SerialPubkey index to find all pubkeys, then counts events for each.
func (c *CuratingACL) ScanAllPubkeys() (*ScanResult, error) {
	result := &ScanResult{}

	// First, get all trusted and blacklisted pubkeys to skip
	trusted, err := c.ListTrustedPubkeys()
	if err != nil {
		return nil, err
	}
	blacklisted, err := c.ListBlacklistedPubkeys()
	if err != nil {
		return nil, err
	}

	excludeSet := make(map[string]struct{})
	for _, t := range trusted {
		excludeSet[t.Pubkey] = struct{}{}
	}
	for _, b := range blacklisted {
		excludeSet[b.Pubkey] = struct{}{}
	}

	// Scan the SerialPubkey index to get all pubkeys
	pubkeys := make(map[string]struct{})

	err = c.View(func(txn *badger.Txn) error {
		// SerialPubkey prefix is "spk"
		prefix := []byte("spk")
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			// The value contains the 32-byte pubkey
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			if len(val) == 32 {
				// Convert to hex
				pubkeyHex := fmt.Sprintf("%x", val)
				pubkeys[pubkeyHex] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result.TotalPubkeys = len(pubkeys)

	// For each pubkey, count events and store the count
	today := time.Now().Format("2006-01-02")

	for pubkeyHex := range pubkeys {
		// Skip if trusted or blacklisted
		if _, excluded := excludeSet[pubkeyHex]; excluded {
			result.Skipped++
			continue
		}

		// Count events for this pubkey using the Pubkey index
		count, err := c.countEventsForPubkey(pubkeyHex)
		if err != nil {
			continue
		}

		if count > 0 {
			result.TotalEvents += count

			// Store the event count
			ec := PubkeyEventCount{
				Pubkey:    pubkeyHex,
				Date:      today,
				Count:     count,
				LastEvent: time.Now(),
			}

			err = c.Update(func(txn *badger.Txn) error {
				key := c.getEventCountKey(pubkeyHex, today)
				data, err := json.Marshal(ec)
				if err != nil {
					return err
				}
				return txn.Set(key, data)
			})
			if err != nil {
				continue
			}
		}
	}

	return result, nil
}

// EventSummary represents a simplified event for display in the UI
type EventSummary struct {
	ID        string `json:"id"`
	Kind      int    `json:"kind"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

// GetEventsForPubkey fetches events for a pubkey, returning simplified event data
// limit specifies max events to return, offset is for pagination
func (c *CuratingACL) GetEventsForPubkey(pubkeyHex string, limit, offset int) ([]EventSummary, int, error) {
	var events []EventSummary

	// First, count total events for this pubkey
	totalCount, err := c.countEventsForPubkey(pubkeyHex)
	if err != nil {
		return nil, 0, err
	}

	// Decode the pubkey hex to bytes
	pubkeyBytes, err := hex.DecAppend(nil, []byte(pubkeyHex))
	if err != nil {
		return nil, 0, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	// Create a filter to query events by author
	// Use a larger limit to account for offset, then slice
	queryLimit := uint(limit + offset)
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(pubkeyBytes),
		Limit:   &queryLimit,
	}

	// Query events using the database's QueryEvents method
	ctx := context.Background()
	evs, err := c.D.QueryEvents(ctx, f)
	if err != nil {
		return nil, 0, err
	}

	// Apply offset and convert to EventSummary
	for i, ev := range evs {
		if i < offset {
			continue
		}
		if len(events) >= limit {
			break
		}
		events = append(events, EventSummary{
			ID:        hex.Enc(ev.ID),
			Kind:      int(ev.Kind),
			Content:   string(ev.Content),
			CreatedAt: ev.CreatedAt,
		})
	}

	return events, totalCount, nil
}

// DeleteEventsForPubkey deletes all events for a given pubkey
// Returns the number of events deleted
func (c *CuratingACL) DeleteEventsForPubkey(pubkeyHex string) (int, error) {
	// Decode the pubkey hex to bytes
	pubkeyBytes, err := hex.DecAppend(nil, []byte(pubkeyHex))
	if err != nil {
		return 0, fmt.Errorf("invalid pubkey hex: %w", err)
	}

	// Create a filter to find all events by this author
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(pubkeyBytes),
	}

	// Query all events for this pubkey
	ctx := context.Background()
	evs, err := c.D.QueryEvents(ctx, f)
	if err != nil {
		return 0, err
	}

	// Delete each event
	deleted := 0
	for _, ev := range evs {
		if err := c.D.DeleteEvent(ctx, ev.ID); err != nil {
			// Log error but continue deleting
			continue
		}
		deleted++
	}

	return deleted, nil
}

// countEventsForPubkey counts events in the database for a given pubkey hex string
func (c *CuratingACL) countEventsForPubkey(pubkeyHex string) (int, error) {
	count := 0

	// Decode the pubkey hex to bytes
	pubkeyBytes := make([]byte, 32)
	for i := 0; i < 32 && i*2+1 < len(pubkeyHex); i++ {
		fmt.Sscanf(pubkeyHex[i*2:i*2+2], "%02x", &pubkeyBytes[i])
	}

	// Compute the pubkey hash (SHA256 of pubkey, first 8 bytes)
	// This matches the PubHash type in indexes/types/pubhash.go
	pkh := sha256.Sum256(pubkeyBytes)

	// Scan the Pubkey index (prefix "pc-") for this pubkey
	err := c.View(func(txn *badger.Txn) error {
		// Build prefix: "pc-" + 8-byte SHA256 hash of pubkey
		prefix := make([]byte, 3+8)
		copy(prefix[:3], []byte("pc-"))
		copy(prefix[3:], pkh[:8])

		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})

	return count, err
}
