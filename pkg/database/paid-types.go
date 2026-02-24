package database

import "time"

// PaidSubscription represents an active paid subscription.
type PaidSubscription struct {
	PubkeyHex   string    `json:"pubkey"`
	Alias       string    `json:"alias,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	InvoiceHash string    `json:"invoice_hash,omitempty"`
}

// IsActive returns true if the subscription has not expired.
func (s *PaidSubscription) IsActive() bool {
	return time.Now().Before(s.ExpiresAt)
}

// AliasClaim represents a claimed email alias.
type AliasClaim struct {
	Alias     string    `json:"alias"`
	PubkeyHex string    `json:"pubkey"`
	ClaimedAt time.Time `json:"claimed_at"`
}

// ErrAliasTaken is returned when attempting to claim an alias already taken by another pubkey.
var ErrAliasTaken = &aliasTakenError{}

type aliasTakenError struct{}

func (e *aliasTakenError) Error() string { return "alias is already taken" }
