//go:build !(js && wasm)

package database

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// PaidACL provides database operations for paid ACL data.
type PaidACL struct {
	*D
}

// NewPaidACL creates a new PaidACL instance.
func NewPaidACL(db *D) *PaidACL {
	return &PaidACL{D: db}
}

// SaveSubscription saves or updates a subscription.
func (p *PaidACL) SaveSubscription(sub *PaidSubscription) error {
	return p.Update(func(txn *badger.Txn) error {
		key := p.getSubKey(sub.PubkeyHex)
		data, err := json.Marshal(sub)
		if err != nil {
			return err
		}
		return txn.Set(key, data)
	})
}

// GetSubscription returns the subscription for a pubkey.
func (p *PaidACL) GetSubscription(pubkeyHex string) (*PaidSubscription, error) {
	var sub PaidSubscription
	err := p.View(func(txn *badger.Txn) error {
		key := p.getSubKey(pubkeyHex)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &sub)
	})
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// DeleteSubscription removes a subscription.
func (p *PaidACL) DeleteSubscription(pubkeyHex string) error {
	return p.Update(func(txn *badger.Txn) error {
		return txn.Delete(p.getSubKey(pubkeyHex))
	})
}

// ListSubscriptions returns all subscriptions.
func (p *PaidACL) ListSubscriptions() ([]*PaidSubscription, error) {
	var subs []*PaidSubscription
	err := p.View(func(txn *badger.Txn) error {
		prefix := p.getSubPrefix()
		it := txn.NewIterator(badger.IteratorOptions{Prefix: prefix})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			val, err := item.ValueCopy(nil)
			if err != nil {
				continue
			}
			var sub PaidSubscription
			if err := json.Unmarshal(val, &sub); err != nil {
				continue
			}
			subs = append(subs, &sub)
		}
		return nil
	})
	return subs, err
}

// ClaimAlias atomically claims an alias for a pubkey.
// Returns an error if the alias is already taken by another pubkey.
// One pubkey can have multiple aliases — each claim appends to the list.
func (p *PaidACL) ClaimAlias(alias, pubkeyHex string) error {
	return p.Update(func(txn *badger.Txn) error {
		// Check if alias is already taken
		aliasKey := p.getAliasKey(alias)
		item, err := txn.Get(aliasKey)
		if err == nil {
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			var existing AliasClaim
			if err := json.Unmarshal(val, &existing); err != nil {
				return err
			}
			if existing.PubkeyHex == pubkeyHex {
				return nil // already claimed by this pubkey
			}
			return ErrAliasTaken
		}
		if err != badger.ErrKeyNotFound {
			return err
		}

		// Write forward mapping: alias → pubkey
		claim := AliasClaim{
			Alias:     alias,
			PubkeyHex: pubkeyHex,
			ClaimedAt: time.Now(),
		}
		data, err := json.Marshal(claim)
		if err != nil {
			return err
		}
		if err := txn.Set(aliasKey, data); err != nil {
			return err
		}

		// Append to reverse mapping: pubkey → JSON array of aliases
		revKey := p.getAliasRevKey(pubkeyHex)
		existing := p.readAliasRev(txn, revKey)
		for _, a := range existing {
			if a == alias {
				return nil // already in list
			}
		}
		existing = append(existing, alias)
		revData, err := json.Marshal(existing)
		if err != nil {
			return err
		}
		return txn.Set(revKey, revData)
	})
}

// GetAliasByPubkey returns the first alias for a pubkey, or "" if none.
func (p *PaidACL) GetAliasByPubkey(pubkeyHex string) (string, error) {
	aliases, err := p.GetAliasesByPubkey(pubkeyHex)
	if err != nil || len(aliases) == 0 {
		return "", err
	}
	return aliases[0], nil
}

// GetAliasesByPubkey returns all aliases for a pubkey.
func (p *PaidACL) GetAliasesByPubkey(pubkeyHex string) ([]string, error) {
	var aliases []string
	err := p.View(func(txn *badger.Txn) error {
		key := p.getAliasRevKey(pubkeyHex)
		aliases = p.readAliasRev(txn, key)
		return nil
	})
	return aliases, err
}

// readAliasRev reads the reverse mapping, handling both old (plain string)
// and new (JSON array) formats.
func (p *PaidACL) readAliasRev(txn *badger.Txn, key []byte) []string {
	item, err := txn.Get(key)
	if err != nil {
		return nil
	}
	val, err := item.ValueCopy(nil)
	if err != nil || len(val) == 0 {
		return nil
	}
	// Try JSON array first (new format)
	var arr []string
	if json.Unmarshal(val, &arr) == nil {
		return arr
	}
	// Fall back to plain string (old format)
	return []string{string(val)}
}

// GetPubkeyByAlias returns the pubkey for an alias, or "" if not found.
func (p *PaidACL) GetPubkeyByAlias(alias string) (string, error) {
	var pubkey string
	err := p.View(func(txn *badger.Txn) error {
		key := p.getAliasKey(alias)
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
		var claim AliasClaim
		if err := json.Unmarshal(val, &claim); err != nil {
			return err
		}
		pubkey = claim.PubkeyHex
		return nil
	})
	return pubkey, err
}

// IsAliasTaken returns true if the alias is claimed by any pubkey.
func (p *PaidACL) IsAliasTaken(alias string) (bool, error) {
	var taken bool
	err := p.View(func(txn *badger.Txn) error {
		key := p.getAliasKey(alias)
		_, err := txn.Get(key)
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		taken = true
		return nil
	})
	return taken, err
}

// Key generation methods

func (p *PaidACL) getSubKey(pubkeyHex string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("PAID_SUB_")
	buf.WriteString(pubkeyHex)
	return buf.Bytes()
}

func (p *PaidACL) getSubPrefix() []byte {
	return []byte("PAID_SUB_")
}

func (p *PaidACL) getAliasKey(alias string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("PAID_ALIAS_")
	buf.WriteString(alias)
	return buf.Bytes()
}

func (p *PaidACL) getAliasRevKey(pubkeyHex string) []byte {
	buf := new(bytes.Buffer)
	buf.WriteString("PAID_ALIAS_REV_")
	buf.WriteString(pubkeyHex)
	return buf.Bytes()
}

