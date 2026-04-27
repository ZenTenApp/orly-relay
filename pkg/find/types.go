package find

import (
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

// Event kind constants as defined in the NIP
const (
	KindRegistrationProposal = 30100 // Parameterized replaceable
	KindAttestation          = 20100 // Ephemeral
	KindTrustGraph           = 30101 // Parameterized replaceable
	KindNameState            = 30102 // Parameterized replaceable
	KindNameRecords          = 30103 // Parameterized replaceable
	KindCertificate          = 30104 // Parameterized replaceable
	KindWitnessService       = 30105 // Parameterized replaceable
)

// Action types for registration proposals
const (
	ActionRegister = "register"
	ActionTransfer = "transfer"
)

// Decision types for attestations
const (
	DecisionApprove = "approve"
	DecisionReject  = "reject"
	DecisionAbstain = "abstain"
)

// DNS record types
const (
	RecordTypeA     = "A"
	RecordTypeAAAA  = "AAAA"
	RecordTypeCNAME = "CNAME"
	RecordTypeMX    = "MX"
	RecordTypeTXT   = "TXT"
	RecordTypeNS    = "NS"
	RecordTypeSRV   = "SRV"
)

// Time constants
const (
	ProposalExpiry          = 5 * time.Minute      // Proposals expire after 5 minutes
	AttestationExpiry       = 3 * time.Minute      // Attestations expire after 3 minutes
	TrustGraphExpiry        = 30 * 24 * time.Hour  // Trust graphs expire after 30 days
	NameRegistrationPeriod  = 365 * 24 * time.Hour // Names expire after 1 year
	PreferentialRenewalDays = 30                   // Final 30 days before expiration
	CertificateValidity     = 90 * 24 * time.Hour  // Recommended certificate validity
	WitnessServiceExpiry    = 180 * 24 * time.Hour // Witness service info expires after 180 days
)

// RegistrationProposal represents a kind 30100 event
type RegistrationProposal struct {
	Event      *event.E
	Name       string
	Action     string // "register" or "transfer"
	PrevOwner  string // Previous owner pubkey (for transfers)
	PrevSig    string // Signature from previous owner (for transfers)
	Expiration time.Time
}

// Attestation represents a kind 20100 event
type Attestation struct {
	Event      *event.E
	ProposalID string // Event ID of the proposal being attested
	Decision   string // "approve", "reject", or "abstain"
	Weight     int    // Stake/confidence weight (default 100)
	Reason     string // Human-readable justification
	ServiceURL string // Registry service endpoint
	Expiration time.Time
}

// TrustEntry represents a single trust relationship
type TrustEntry struct {
	Pubkey     string
	ServiceURL string
	TrustScore float64 // 0.0 to 1.0
}

// TrustGraphEvent represents a kind 30101 event (renamed to avoid conflict with TrustGraph manager in trust.go)
type TrustGraphEvent struct {
	Event      *event.E
	Entries    []TrustEntry
	Expiration time.Time
}

// NameState represents a kind 30102 event
type NameState struct {
	Event        *event.E
	Name         string
	Owner        string // Current owner pubkey
	RegisteredAt time.Time
	ProposalID   string  // Event ID of the registration proposal
	Attestations int     // Number of attestations
	Confidence   float64 // Consensus confidence score (0.0 to 1.0)
	Expiration   time.Time
}

// NameRecord represents a kind 30103 event
type NameRecord struct {
	Event    *event.E
	Name     string
	Type     string // A, AAAA, CNAME, MX, TXT, NS, SRV
	Value    string
	TTL      int // Cache TTL in seconds
	Priority int // For MX and SRV records
	Weight   int // For SRV records
	Port     int // For SRV records
}

// RecordLimits defines per-type record limits
var RecordLimits = map[string]int{
	RecordTypeA:     5,
	RecordTypeAAAA:  5,
	RecordTypeCNAME: 1,
	RecordTypeMX:    5,
	RecordTypeTXT:   10,
	RecordTypeNS:    5,
	RecordTypeSRV:   10,
}

// Certificate represents a kind 30104 event
type Certificate struct {
	Event          *event.E
	Name           string
	CertPubkey     string // Public key for the service
	ValidFrom      time.Time
	ValidUntil     time.Time
	Challenge      string // Challenge token for ownership proof
	ChallengeProof string // Signature over challenge
	Witnesses      []WitnessSignature
	Algorithm      string // e.g., "secp256k1-schnorr"
	Usage          string // e.g., "tls-replacement"
}

// WitnessSignature represents a witness attestation on a certificate
type WitnessSignature struct {
	Pubkey    string
	Signature string
}

// WitnessService represents a kind 30105 event
type WitnessService struct {
	Event        *event.E
	Endpoint     string
	Challenges   []string // Supported challenge types: "txt", "http", "event"
	MaxValidity  int      // Maximum certificate validity in seconds
	Fee          int      // Fee in sats per certificate
	ReputationID string   // Event ID of reputation event
	Description  string
	Contact      string
	Expiration   time.Time
}

// TransferAuthorization represents the message signed for transfer authorization
type TransferAuthorization struct {
	Name      string
	NewOwner  string
	Timestamp time.Time
}

// ChallengeProofMessage represents the message signed for certificate challenge proof
type ChallengeProofMessage struct {
	Challenge  string
	Name       string
	CertPubkey string
	ValidUntil time.Time
}

// WitnessMessage represents the message signed by witnesses
type WitnessMessage struct {
	CertPubkey string
	Name       string
	ValidFrom  time.Time
	ValidUntil time.Time
	Challenge  string
}
