package dgraph

import (
	"encoding/json"
	"fmt"
	"time"

	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// Subscription and payment methods
// Simplified implementation using marker-based storage
// For production, these should use proper graph nodes with relationships

// GetSubscription retrieves subscription information for a pubkey
func (d *D) GetSubscription(pubkey []byte) (*database.Subscription, error) {
	key := "sub_" + hex.Enc(pubkey)
	data, err := d.GetMarker(key)
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
func (d *D) IsSubscriptionActive(pubkey []byte) (bool, error) {
	sub, err := d.GetSubscription(pubkey)
	if err != nil {
		return false, nil // No subscription = not active
	}

	return sub.PaidUntil.After(time.Now()), nil
}

// ExtendSubscription extends a subscription by the specified number of days
func (d *D) ExtendSubscription(pubkey []byte, days int) error {
	key := "sub_" + hex.Enc(pubkey)

	// Get existing subscription or create new
	var sub database.Subscription
	data, err := d.GetMarker(key)
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

	return d.SetMarker(key, data)
}

// RecordPayment records a payment for subscription extension
func (d *D) RecordPayment(
	pubkey []byte, amount int64, invoice, preimage string,
) error {
	// Store payment in payments list
	key := "payments_" + hex.Enc(pubkey)

	var payments []database.Payment
	data, err := d.GetMarker(key)
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

	return d.SetMarker(key, data)
}

// GetPaymentHistory retrieves payment history for a pubkey
func (d *D) GetPaymentHistory(pubkey []byte) ([]database.Payment, error) {
	key := "payments_" + hex.Enc(pubkey)

	data, err := d.GetMarker(key)
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
func (d *D) ExtendBlossomSubscription(
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

	return d.SetMarker(key, jsonData)
}

// GetBlossomStorageQuota retrieves the storage quota for a pubkey
func (d *D) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	key := "blossom_" + hex.Enc(pubkey)

	data, err := d.GetMarker(key)
	if err != nil {
		return 0, nil // No subscription = 0 quota
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return 0, fmt.Errorf("failed to unmarshal blossom data: %w", err)
	}

	// Default quota based on tier - simplified
	if tier, ok := result["tier"].(string); ok {
		switch tier {
		case "basic":
			return 100, nil
		case "premium":
			return 1000, nil
		default:
			return 10, nil
		}
	}

	return 0, nil
}

// IsFirstTimeUser checks if a pubkey is a first-time user
func (d *D) IsFirstTimeUser(pubkey []byte) (bool, error) {
	// Check if they have any subscription or payment history
	sub, _ := d.GetSubscription(pubkey)
	if sub != nil {
		return false, nil
	}

	payments, _ := d.GetPaymentHistory(pubkey)
	if len(payments) > 0 {
		return false, nil
	}

	return true, nil
}
