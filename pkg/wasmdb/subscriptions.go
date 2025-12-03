//go:build js && wasm

package wasmdb

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"time"

	"github.com/aperturerobotics/go-indexeddb/idb"

	"next.orly.dev/pkg/database"
)

const (
	// SubscriptionsStoreName is the object store for payment subscriptions
	SubscriptionsStoreName = "subscriptions"

	// PaymentsPrefix is the key prefix for payment records
	PaymentsPrefix = "payment:"
)

// GetSubscription retrieves a subscription for a pubkey
func (w *W) GetSubscription(pubkey []byte) (*database.Subscription, error) {
	key := "sub:" + string(pubkey)
	data, err := w.getStoreValue(SubscriptionsStoreName, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	return w.deserializeSubscription(data)
}

// IsSubscriptionActive checks if a pubkey has an active subscription
// If no subscription exists, creates a 30-day trial
func (w *W) IsSubscriptionActive(pubkey []byte) (bool, error) {
	key := "sub:" + string(pubkey)
	data, err := w.getStoreValue(SubscriptionsStoreName, key)
	if err != nil {
		return false, err
	}

	now := time.Now()

	if data == nil {
		// Create new trial subscription
		sub := &database.Subscription{
			TrialEnd: now.AddDate(0, 0, 30),
		}
		subData := w.serializeSubscription(sub)
		if err := w.setStoreValue(SubscriptionsStoreName, key, subData); err != nil {
			return false, err
		}
		return true, nil
	}

	sub, err := w.deserializeSubscription(data)
	if err != nil {
		return false, err
	}

	// Active if within trial or paid period
	return now.Before(sub.TrialEnd) || (!sub.PaidUntil.IsZero() && now.Before(sub.PaidUntil)), nil
}

// ExtendSubscription extends a subscription by the given number of days
func (w *W) ExtendSubscription(pubkey []byte, days int) error {
	if days <= 0 {
		return errors.New("invalid days")
	}

	key := "sub:" + string(pubkey)
	data, err := w.getStoreValue(SubscriptionsStoreName, key)
	if err != nil {
		return err
	}

	now := time.Now()
	var sub *database.Subscription

	if data == nil {
		// Create new subscription
		sub = &database.Subscription{
			PaidUntil: now.AddDate(0, 0, days),
		}
	} else {
		sub, err = w.deserializeSubscription(data)
		if err != nil {
			return err
		}
		// Extend from current paid date if still active, otherwise from now
		extendFrom := now
		if !sub.PaidUntil.IsZero() && sub.PaidUntil.After(now) {
			extendFrom = sub.PaidUntil
		}
		sub.PaidUntil = extendFrom.AddDate(0, 0, days)
	}

	// Serialize and store
	subData := w.serializeSubscription(sub)
	return w.setStoreValue(SubscriptionsStoreName, key, subData)
}

// RecordPayment records a payment for a pubkey
func (w *W) RecordPayment(pubkey []byte, amount int64, invoice, preimage string) error {
	now := time.Now()
	payment := &database.Payment{
		Amount:    amount,
		Timestamp: now,
		Invoice:   invoice,
		Preimage:  preimage,
	}

	data := w.serializePayment(payment)

	// Create unique key with timestamp
	key := PaymentsPrefix + string(pubkey) + ":" + now.Format(time.RFC3339Nano)
	return w.setStoreValue(SubscriptionsStoreName, key, data)
}

// GetPaymentHistory retrieves all payments for a pubkey
func (w *W) GetPaymentHistory(pubkey []byte) ([]database.Payment, error) {
	prefix := PaymentsPrefix + string(pubkey) + ":"

	tx, err := w.db.Transaction(idb.TransactionReadOnly, SubscriptionsStoreName)
	if err != nil {
		return nil, err
	}

	store, err := tx.ObjectStore(SubscriptionsStoreName)
	if err != nil {
		return nil, err
	}

	var payments []database.Payment

	cursorReq, err := store.OpenCursor(idb.CursorNext)
	if err != nil {
		return nil, err
	}

	prefixBytes := []byte(prefix)

	err = cursorReq.Iter(w.ctx, func(cursor *idb.CursorWithValue) error {
		keyVal, keyErr := cursor.Key()
		if keyErr != nil {
			return keyErr
		}

		keyBytes := safeValueToBytes(keyVal)
		if bytes.HasPrefix(keyBytes, prefixBytes) {
			val, valErr := cursor.Value()
			if valErr != nil {
				return valErr
			}
			valBytes := safeValueToBytes(val)
			if payment, err := w.deserializePayment(valBytes); err == nil {
				payments = append(payments, *payment)
			}
		}

		return cursor.Continue()
	})

	if err != nil {
		return nil, err
	}

	return payments, nil
}

// ExtendBlossomSubscription extends a blossom subscription with storage quota
func (w *W) ExtendBlossomSubscription(pubkey []byte, level string, storageMB int64, days int) error {
	if days <= 0 {
		return errors.New("invalid days")
	}

	key := "sub:" + string(pubkey)
	data, err := w.getStoreValue(SubscriptionsStoreName, key)
	if err != nil {
		return err
	}

	now := time.Now()
	var sub *database.Subscription

	if data == nil {
		sub = &database.Subscription{
			PaidUntil:      now.AddDate(0, 0, days),
			BlossomLevel:   level,
			BlossomStorage: storageMB,
		}
	} else {
		sub, err = w.deserializeSubscription(data)
		if err != nil {
			return err
		}

		// Extend from current paid date if still active
		extendFrom := now
		if !sub.PaidUntil.IsZero() && sub.PaidUntil.After(now) {
			extendFrom = sub.PaidUntil
		}
		sub.PaidUntil = extendFrom.AddDate(0, 0, days)

		// Set level and accumulate storage
		sub.BlossomLevel = level
		if sub.BlossomStorage > 0 && sub.PaidUntil.After(now) {
			sub.BlossomStorage += storageMB
		} else {
			sub.BlossomStorage = storageMB
		}
	}

	subData := w.serializeSubscription(sub)
	return w.setStoreValue(SubscriptionsStoreName, key, subData)
}

// GetBlossomStorageQuota returns the storage quota for a pubkey
func (w *W) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	sub, err := w.GetSubscription(pubkey)
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

// IsFirstTimeUser checks if a pubkey is a first-time user (no subscription history)
func (w *W) IsFirstTimeUser(pubkey []byte) (bool, error) {
	key := "firstlogin:" + string(pubkey)
	data, err := w.getStoreValue(SubscriptionsStoreName, key)
	if err != nil {
		return false, err
	}

	if data == nil {
		// First time - record the login
		now := time.Now()
		loginData, _ := json.Marshal(map[string]interface{}{
			"first_login": now,
		})
		_ = w.setStoreValue(SubscriptionsStoreName, key, loginData)
		return true, nil
	}

	return false, nil
}

// serializeSubscription converts a subscription to bytes using JSON
func (w *W) serializeSubscription(s *database.Subscription) []byte {
	data, _ := json.Marshal(s)
	return data
}

// deserializeSubscription converts bytes to a subscription
func (w *W) deserializeSubscription(data []byte) (*database.Subscription, error) {
	s := &database.Subscription{}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	return s, nil
}

// serializePayment converts a payment to bytes
func (w *W) serializePayment(p *database.Payment) []byte {
	buf := new(bytes.Buffer)

	// Amount (8 bytes)
	amt := make([]byte, 8)
	binary.BigEndian.PutUint64(amt, uint64(p.Amount))
	buf.Write(amt)

	// Timestamp (8 bytes)
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, uint64(p.Timestamp.Unix()))
	buf.Write(ts)

	// Invoice length (4 bytes) + Invoice
	invBytes := []byte(p.Invoice)
	invLen := make([]byte, 4)
	binary.BigEndian.PutUint32(invLen, uint32(len(invBytes)))
	buf.Write(invLen)
	buf.Write(invBytes)

	// Preimage length (4 bytes) + Preimage
	preBytes := []byte(p.Preimage)
	preLen := make([]byte, 4)
	binary.BigEndian.PutUint32(preLen, uint32(len(preBytes)))
	buf.Write(preLen)
	buf.Write(preBytes)

	return buf.Bytes()
}

// deserializePayment converts bytes to a payment
func (w *W) deserializePayment(data []byte) (*database.Payment, error) {
	if len(data) < 24 { // 8 + 8 + 4 + 4 minimum
		return nil, errors.New("invalid payment data")
	}

	p := &database.Payment{}

	p.Amount = int64(binary.BigEndian.Uint64(data[0:8]))
	p.Timestamp = time.Unix(int64(binary.BigEndian.Uint64(data[8:16])), 0)

	invLen := binary.BigEndian.Uint32(data[16:20])
	if len(data) < int(20+invLen+4) {
		return nil, errors.New("invalid invoice length")
	}
	p.Invoice = string(data[20 : 20+invLen])

	offset := 20 + invLen
	preLen := binary.BigEndian.Uint32(data[offset : offset+4])
	if len(data) < int(offset+4+preLen) {
		return nil, errors.New("invalid preimage length")
	}
	p.Preimage = string(data[offset+4 : offset+4+preLen])

	return p, nil
}
