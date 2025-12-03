//go:build !(js && wasm)

package database

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"encoding/json"

	"github.com/dgraph-io/badger/v4"
)

func (d *D) GetSubscription(pubkey []byte) (*Subscription, error) {
	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	var sub *Subscription

	err := d.DB.View(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			return item.Value(
				func(val []byte) error {
					sub = &Subscription{}
					return json.Unmarshal(val, sub)
				},
			)
		},
	)
	return sub, err
}

func (d *D) IsSubscriptionActive(pubkey []byte) (bool, error) {
	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	now := time.Now()
	active := false

	err := d.DB.Update(
		func(txn *badger.Txn) error {
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				sub := &Subscription{TrialEnd: now.AddDate(0, 0, 30)}
				data, err := json.Marshal(sub)
				if err != nil {
					return err
				}
				active = true
				return txn.Set([]byte(key), data)
			}
			if err != nil {
				return err
			}

			var sub Subscription
			err = item.Value(
				func(val []byte) error {
					return json.Unmarshal(val, &sub)
				},
			)
			if err != nil {
				return err
			}

			active = now.Before(sub.TrialEnd) || (!sub.PaidUntil.IsZero() && now.Before(sub.PaidUntil))
			return nil
		},
	)
	return active, err
}

func (d *D) ExtendSubscription(pubkey []byte, days int) error {
	if days <= 0 {
		return fmt.Errorf("invalid days: %d", days)
	}

	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	now := time.Now()

	return d.DB.Update(
		func(txn *badger.Txn) error {
			var sub Subscription
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				sub.PaidUntil = now.AddDate(0, 0, days)
			} else if err != nil {
				return err
			} else {
				err = item.Value(
					func(val []byte) error {
						return json.Unmarshal(val, &sub)
					},
				)
				if err != nil {
					return err
				}
				extendFrom := now
				if !sub.PaidUntil.IsZero() && sub.PaidUntil.After(now) {
					extendFrom = sub.PaidUntil
				}
				sub.PaidUntil = extendFrom.AddDate(0, 0, days)
			}

			data, err := json.Marshal(&sub)
			if err != nil {
				return err
			}
			return txn.Set([]byte(key), data)
		},
	)
}

func (d *D) RecordPayment(
	pubkey []byte, amount int64, invoice, preimage string,
) error {
	now := time.Now()
	key := fmt.Sprintf("payment:%d:%s", now.Unix(), hex.EncodeToString(pubkey))

	payment := Payment{
		Amount:    amount,
		Timestamp: now,
		Invoice:   invoice,
		Preimage:  preimage,
	}

	data, err := json.Marshal(&payment)
	if err != nil {
		return err
	}

	return d.DB.Update(
		func(txn *badger.Txn) error {
			return txn.Set([]byte(key), data)
		},
	)
}

func (d *D) GetPaymentHistory(pubkey []byte) ([]Payment, error) {
	prefix := fmt.Sprintf("payment:")
	suffix := fmt.Sprintf(":%s", hex.EncodeToString(pubkey))
	var payments []Payment

	err := d.DB.View(
		func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
				key := string(it.Item().Key())
				if !strings.HasSuffix(key, suffix) {
					continue
				}

				err := it.Item().Value(
					func(val []byte) error {
						var payment Payment
						err := json.Unmarshal(val, &payment)
						if err != nil {
							return err
						}
						payments = append(payments, payment)
						return nil
					},
				)
				if err != nil {
					return err
				}
			}
			return nil
		},
	)

	return payments, err
}

// ExtendBlossomSubscription extends or creates a blossom subscription with service level
func (d *D) ExtendBlossomSubscription(
	pubkey []byte, level string, storageMB int64, days int,
) error {
	if days <= 0 {
		return fmt.Errorf("invalid days: %d", days)
	}

	key := fmt.Sprintf("sub:%s", hex.EncodeToString(pubkey))
	now := time.Now()

	return d.DB.Update(
		func(txn *badger.Txn) error {
			var sub Subscription
			item, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				sub.PaidUntil = now.AddDate(0, 0, days)
			} else if err != nil {
				return err
			} else {
				err = item.Value(
					func(val []byte) error {
						return json.Unmarshal(val, &sub)
					},
				)
				if err != nil {
					return err
				}
				extendFrom := now
				if !sub.PaidUntil.IsZero() && sub.PaidUntil.After(now) {
					extendFrom = sub.PaidUntil
				}
				sub.PaidUntil = extendFrom.AddDate(0, 0, days)
			}

			// Set blossom service level and storage
			sub.BlossomLevel = level
			// Add storage quota (accumulate if subscription already exists)
			if sub.BlossomStorage > 0 && sub.PaidUntil.After(now) {
				// Add to existing quota
				sub.BlossomStorage += storageMB
			} else {
				// Set new quota
				sub.BlossomStorage = storageMB
			}

			data, err := json.Marshal(&sub)
			if err != nil {
				return err
			}
			return txn.Set([]byte(key), data)
		},
	)
}

// GetBlossomStorageQuota returns the current blossom storage quota in MB for a pubkey
func (d *D) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	sub, err := d.GetSubscription(pubkey)
	if err != nil {
		return 0, err
	}
	if sub == nil {
		return 0, nil
	}
	// Only return quota if subscription is active
	if sub.PaidUntil.IsZero() || time.Now().After(sub.PaidUntil) {
		return 0, nil
	}
	return sub.BlossomStorage, nil
}

// IsFirstTimeUser checks if a user is logging in for the first time and marks them as seen
func (d *D) IsFirstTimeUser(pubkey []byte) (bool, error) {
	key := fmt.Sprintf("firstlogin:%s", hex.EncodeToString(pubkey))

	isFirstTime := false
	err := d.DB.Update(
		func(txn *badger.Txn) error {
			_, err := txn.Get([]byte(key))
			if errors.Is(err, badger.ErrKeyNotFound) {
				// First time - record the login
				isFirstTime = true
				now := time.Now()
				data, err := json.Marshal(map[string]interface{}{
					"first_login": now,
				})
				if err != nil {
					return err
				}
				return txn.Set([]byte(key), data)
			}
			return err // Return any other error as-is
		},
	)

	return isFirstTime, err
}
