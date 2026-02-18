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

// WordToken represents a normalized word with its truncated hash for indexing.
// Used by TokenWords() for NIP-50 word search across database backends.
type WordToken struct {
	Word string // normalized lowercase word (e.g., "bitcoin")
	Hash []byte // 8-byte truncated SHA-256
}

// BlobMetadata stores metadata about a blob in the database
type BlobMetadata struct {
	Pubkey    []byte `json:"pubkey"`
	MimeType  string `json:"mime_type"`
	Uploaded  int64  `json:"uploaded"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"` // File extension (e.g., ".png", ".pdf")
}

// BlobDescriptor represents a blob descriptor as defined in BUD-02
type BlobDescriptor struct {
	URL      string     `json:"url"`
	SHA256   string     `json:"sha256"`
	Size     int64      `json:"size"`
	Type     string     `json:"type"`
	Uploaded int64      `json:"uploaded"`
	NIP94    [][]string `json:"nip94,omitempty"`
}

// UserBlobStats represents storage statistics for a single user
type UserBlobStats struct {
	PubkeyHex      string
	BlobCount      int64
	TotalSizeBytes int64
}
