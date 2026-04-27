package neo4j

import (
	"encoding/json"
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// Subscription and payment methods
// Simplified implementation using marker-based storage via Badger
// For production graph-based storage, these could use Neo4j nodes with relationships

// GetSubscription retrieves subscription information for a pubkey
func (n *N) GetSubscription(pubkey []byte) (*database.Subscription, error) {
	key := "sub_" + hex.Enc(pubkey)
	data, err := n.GetMarker(key)
	if err != nil {
		return nil, err
	}

	var sub database.Subscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscription: %w", err)
	}

	return &sub, nil
}

// IsSubscriptionActive checks if a pubkey has an active subscription
func (n *N) IsSubscriptionActive(pubkey []byte) (bool, error) {
	sub, err := n.GetSubscription(pubkey)
	if err != nil {
		return false, nil // No subscription = not active
	}

	return sub.PaidUntil.After(time.Now()), nil
}

// ExtendSubscription extends a subscription by the specified number of days
func (n *N) ExtendSubscription(pubkey []byte, days int) error {
	key := "sub_" + hex.Enc(pubkey)

	// Get existing subscription or create new
	var sub database.Subscription
	data, err := n.GetMarker(key)
	if err == nil {
		if err := json.Unmarshal(data, &sub); err != nil {
			return fmt.Errorf("failed to unmarshal subscription: %w", err)
		}
	} else {
		// New subscription - set trial period
		sub.TrialEnd = time.Now()
		sub.PaidUntil = time.Now()
	}

	// Extend expiration
	if sub.PaidUntil.Before(time.Now()) {
		sub.PaidUntil = time.Now()
	}
	sub.PaidUntil = sub.PaidUntil.Add(time.Duration(days) * 24 * time.Hour)

	// Save
	data, err = json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	return n.SetMarker(key, data)
}

// RecordPayment records a payment for subscription extension
func (n *N) RecordPayment(
	pubkey []byte, amount int64, invoice, preimage string,
) error {
	// Store payment in payments list
	key := "payments_" + hex.Enc(pubkey)

	var payments []database.Payment
	data, err := n.GetMarker(key)
	if err == nil {
		if err := json.Unmarshal(data, &payments); err != nil {
			return fmt.Errorf("failed to unmarshal payments: %w", err)
		}
	}

	payment := database.Payment{
		Amount:    amount,
		Timestamp: time.Now(),
		Invoice:   invoice,
		Preimage:  preimage,
	}

	payments = append(payments, payment)

	data, err = json.Marshal(payments)
	if err != nil {
		return fmt.Errorf("failed to marshal payments: %w", err)
	}

	return n.SetMarker(key, data)
}

// GetPaymentHistory retrieves payment history for a pubkey
func (n *N) GetPaymentHistory(pubkey []byte) ([]database.Payment, error) {
	key := "payments_" + hex.Enc(pubkey)

	data, err := n.GetMarker(key)
	if err != nil {
		return nil, nil // No payments = empty list
	}

	var payments []database.Payment
	if err := json.Unmarshal(data, &payments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payments: %w", err)
	}

	return payments, nil
}

// ExtendBlossomSubscription extends a Blossom storage subscription
func (n *N) ExtendBlossomSubscription(
	pubkey []byte, tier string, storageMB int64, daysExtended int,
) error {
	key := "blossom_" + hex.Enc(pubkey)

	// Simple implementation - just store tier and expiry
	data := map[string]interface{}{
		"tier":      tier,
		"storageMB": storageMB,
		"extended":  daysExtended,
		"updated":   time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal blossom subscription: %w", err)
	}

	return n.SetMarker(key, jsonData)
}

// GetBlossomStorageQuota retrieves the storage quota for a pubkey
func (n *N) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	key := "blossom_" + hex.Enc(pubkey)

	data, err := n.GetMarker(key)
	if err != nil {
		return 0, nil // No subscription = 0 quota
	}

	var subData map[string]interface{}
	if err := json.Unmarshal(data, &subData); err != nil {
		return 0, fmt.Errorf("failed to unmarshal blossom data: %w", err)
	}

	if storageMB, ok := subData["storageMB"].(float64); ok {
		return int64(storageMB), nil
	}

	return 0, nil
}

// IsFirstTimeUser checks if this is the first time a user is accessing the relay
func (n *N) IsFirstTimeUser(pubkey []byte) (bool, error) {
	key := "first_seen_" + hex.Enc(pubkey)

	// If marker exists, not first time
	if n.HasMarker(key) {
		return false, nil
	}

	// Mark as seen
	if err := n.SetMarker(key, []byte{1}); err != nil {
		return true, err
	}

	return true, nil
}
