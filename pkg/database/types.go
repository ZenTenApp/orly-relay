package database

import "time"

// Subscription represents a user's subscription status
type Subscription struct {
	TrialEnd       time.Time `json:"trial_end"`
	PaidUntil      time.Time `json:"paid_until"`
	BlossomLevel   string    `json:"blossom_level,omitempty"`   // Service level name (e.g., "basic", "premium")
	BlossomStorage int64     `json:"blossom_storage,omitempty"` // Storage quota in MB
}

// Payment represents a recorded payment
type Payment struct {
	Amount    int64     `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
	Invoice   string    `json:"invoice"`
	Preimage  string    `json:"preimage"`
}

// NIP43Membership represents membership metadata for NIP-43
type NIP43Membership struct {
	Pubkey     []byte
	AddedAt    time.Time
	InviteCode string
}
