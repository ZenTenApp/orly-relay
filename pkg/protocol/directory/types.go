// Package directory provides data structures and validation for the distributed
// directory consensus protocol as defined in NIP-XX.
//
// This package implements message encoding and validation for the following
// event kinds:
//   - 39100: Relay Identity Announcement - Announces relay participation in consortium
//   - 39101: Trust Act - Establishes trust relationships with numeric levels (0-100%)
//   - 39102: Group Tag Act - Creates/manages group tags with ownership specs
//   - 39103: Public Key Advertisement - Advertises delegate keys with expiration
//   - 39104: Directory Event Replication Request - Requests event synchronization
//   - 39105: Directory Event Replication Response - Responds to sync requests
//   - 39106: Group Tag Transfer - Transfers group tag ownership (planned)
//   - 39107: Escrow Witness Completion Act - Completes escrow transfers (planned)
//
// # Numeric Trust Levels
//
// Trust levels are represented as numeric values from 0-100, indicating the
// percentage probability that any given event will be replicated. This implements
// partial replication via cryptographic random selection (dice-throw mechanism):
//
//   - 100: Full replication - ALL events replicated (100% probability)
//   - 75:  High partial replication - 75% of events replicated on average
//   - 50:  Medium partial replication - 50% of events replicated on average
//   - 25:  Low partial replication - 25% of events replicated on average
//   - 10:  Minimal sampling - 10% of events replicated on average
//   - 0:   No replication - effectively disables replication
//
// For each event, a cryptographically secure random number (0-100) is generated
// and compared to the trust level threshold. If the random number ≤ trust level,
// the event is replicated; otherwise it is discarded. This provides:
//
//   - Proportional bandwidth/storage reduction
//   - Probabilistic network coverage (events propagate via multiple paths)
//   - Network resilience (different relays replicate different random subsets)
//   - Tunable trade-offs between resources and completeness
//
// # Group Tag Ownership
//
// Group Tag Acts (Kind 39102) establish ownership and control over arbitrary
// string tags, functioning as a first-come-first-served registration system
// akin to domain name registration. Features include:
//
//   - Single signature or multisig ownership (2-of-3, 3-of-5)
//   - URL-safe character validation (RFC 3986 compliance)
//   - Ownership transfer capability (via Kind 39106)
//   - Escrow-based transfer protocol (via Kind 39107)
//   - Foundation for permissioned structured groups across multiple relays
//
// # Legal Concept of Acts
//
// The term "act" in this protocol draws from legal terminology, where an act
// represents a formal declaration or testimony that has legal significance.
// Similar to legal instruments such as:
//
//   - Deed Poll: A legal document binding one party to a particular course of action
//   - Witness Testimony: A formal statement given under oath as evidence
//   - Affidavit: A written statement confirmed by oath for use as evidence
//
// In the context of this protocol, acts serve as cryptographically signed
// declarations that establish trust relationships, group memberships, or other
// formal statements within the relay consortium. Like their legal counterparts,
// these acts:
//
//   - Are formally structured with specific required elements
//   - Carry the authority and responsibility of the signing party
//   - Create binding relationships or obligations within the consortium
//   - Can be verified for authenticity through cryptographic signatures
//   - May have expiration dates or other temporal constraints
//
// This legal framework provides a conceptual foundation for understanding the
// formal nature and binding character of consortium declarations.
package directory

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// Event kinds for the distributed directory consensus protocol
var (
	RelayIdentityAnnouncementKind         = kind.New(39100)
	TrustActKind                          = kind.New(39101)
	GroupTagActKind                       = kind.New(39102)
	PublicKeyAdvertisementKind            = kind.New(39103)
	DirectoryEventReplicationRequestKind  = kind.New(39104)
	DirectoryEventReplicationResponseKind = kind.New(39105)
	GroupTagTransferKind                  = kind.New(39106)
	EscrowWitnessCompletionActKind        = kind.New(39107)
)

// Common tag names used across directory protocol messages.
// These tags are used to structure and encode protocol messages within Nostr events.
var (
	DTag                = []byte("d")                 // Unique identifier for replaceable events
	RelayTag            = []byte("relay")             // WebSocket URL of a relay
	SigningKeyTag       = []byte("signing_key")       // Public key for signature verification
	EncryptionKeyTag    = []byte("encryption_key")    // Public key for ECDH encryption
	VersionTag          = []byte("version")           // Protocol version number
	NIP11URLTag         = []byte("nip11_url")         // Deprecated: Use HTTP GET with Accept header instead
	PubkeyTag           = []byte("p")                 // Public key reference
	TrustLevelTag       = []byte("trust_level")       // Numeric trust level (0-100)
	ExpiryTag           = []byte("expiry")            // Unix timestamp for expiration
	ReasonTag           = []byte("reason")            // Reason for trust establishment
	KTag                = []byte("K")                 // Comma-separated event kinds for replication
	ITag                = []byte("I")                 // Identity tag with proof-of-control
	GroupTagTag         = []byte("group_tag")         // Group identifier and metadata
	ActorTag            = []byte("actor")             // Public key of the actor
	ConfidenceTag       = []byte("confidence")        // Confidence score (0-100)
	OwnersTag           = []byte("owners")            // Ownership specification (scheme + pubkeys)
	CreatedTag          = []byte("created")           // Creation/registration timestamp
	FromOwnersTag       = []byte("from_owners")       // Previous owners in transfer
	ToOwnersTag         = []byte("to_owners")         // New owners in transfer
	TransferDateTag     = []byte("transfer_date")     // Transfer effective date
	SignaturesTag       = []byte("signatures")        // Multisig signatures for transfers
	EscrowIDTag         = []byte("escrow_id")         // Unique escrow identifier
	SellerWitnessTag    = []byte("seller_witness")    // Seller's witness public key
	BuyerWitnessTag     = []byte("buyer_witness")     // Buyer's witness public key
	ConditionsTag       = []byte("conditions")        // Escrow conditions/requirements
	WitnessRoleTag      = []byte("witness_role")      // Role of witness (seller/buyer)
	CompletionStatusTag = []byte("completion_status") // Escrow completion status
	VerificationHashTag = []byte("verification_hash") // Hash for verification
	TimestampTag        = []byte("timestamp")         // General purpose timestamp
	PurposeTag          = []byte("purpose")           // Key purpose (signing/encryption/delegation)
	AlgorithmTag        = []byte("algorithm")         // Cryptographic algorithm identifier
	DerivationPathTag   = []byte("derivation_path")   // BIP32 derivation path
	KeyIndexTag         = []byte("key_index")         // Key index in derivation chain
	RequestIDTag        = []byte("request_id")        // Unique request identifier
	EventIDTag          = []byte("event_id")          // Event ID reference
	StatusTag           = []byte("status")            // Status code (success/error/pending)
	ErrorTag            = []byte("error")             // Error message
)

// TrustLevel represents the replication percentage (0-100) indicating
// the probability that any given event will be replicated.
//
// This implements partial replication via cryptographic random selection:
// For each event, a secure random number (0-100) is generated and compared
// to the trust level. If random ≤ trust_level, the event is replicated.
//
// Benefits of numeric trust levels:
//   - Precise resource control (bandwidth, storage, CPU)
//   - Probabilistic network coverage via multiple propagation paths
//   - Network resilience through random subset selection
//   - Graceful degradation under resource constraints
//   - Trust inheritance via percentage multiplication (e.g., 75% * 50% = 37.5%)
type TrustLevel uint8

// Suggested trust level constants for common use cases.
// These are guidelines; any value 0-100 is valid.
const (
	TrustLevelNone    TrustLevel = 0   // No replication (0%)
	TrustLevelMinimal TrustLevel = 10  // Minimal sampling (10%) - network discovery
	TrustLevelLow     TrustLevel = 25  // Low partial replication (25%) - exploratory
	TrustLevelMedium  TrustLevel = 50  // Medium partial replication (50%) - standard
	TrustLevelHigh    TrustLevel = 75  // High partial replication (75%) - important
	TrustLevelFull    TrustLevel = 100 // Full replication (100%) - critical partners
)

// TrustReason indicates how a trust relationship was established.
// This helps operators understand and audit their trust networks.
type TrustReason string

const (
	TrustReasonManual    TrustReason = "manual"    // Manually configured by operator
	TrustReasonAutomatic TrustReason = "automatic" // Automatically established by policy
	TrustReasonInherited TrustReason = "inherited" // Inherited through web of trust
)

// KeyPurpose indicates the intended use of an advertised public key.
// This allows proper key separation and reduces security risks.
type KeyPurpose string

const (
	KeyPurposeSigning    KeyPurpose = "signing"    // For signing events (BIP-340 Schnorr)
	KeyPurposeEncryption KeyPurpose = "encryption" // For ECDH encryption (NIP-04, NIP-44)
	KeyPurposeDelegation KeyPurpose = "delegation" // For delegated event signing (NIP-26)
)

// ReplicationStatus indicates the outcome of a replication request.
type ReplicationStatus string

const (
	ReplicationStatusSuccess ReplicationStatus = "success" // Replication completed successfully
	ReplicationStatusError   ReplicationStatus = "error"   // Replication failed with error
	ReplicationStatusPending ReplicationStatus = "pending" // Replication in progress
)

// GenerateNonce creates a cryptographically secure random nonce for use in
// identity tags and other protocol messages.
func GenerateNonce(size int) (nonce []byte, err error) {
	if size <= 0 {
		size = 16 // Default to 16 bytes
	}
	nonce = make([]byte, size)
	if _, err = rand.Read(nonce); chk.E(err) {
		return
	}
	return
}

// GenerateNonceHex creates a hex-encoded nonce of the specified byte size.
func GenerateNonceHex(size int) (nonceHex string, err error) {
	var nonce []byte
	if nonce, err = GenerateNonce(size); chk.E(err) {
		return
	}
	nonceHex = hex.EncodeToString(nonce)
	return
}

// IsDirectoryEventKind returns true if the given kind is a directory event
// that should always be replicated among consortium members, regardless of
// the trust level's partial replication setting.
//
// Standard Nostr directory events (always replicated):
//   - Kind 0: User Metadata (profiles)
//   - Kind 3: Follow Lists (contact lists)
//   - Kind 5: Event Deletion Requests
//   - Kind 1984: Reporting (spam, abuse reports)
//   - Kind 10002: Relay List Metadata
//   - Kind 10000: Mute Lists
//   - Kind 10050: DM Relay Lists
//
// Protocol-specific directory events (always replicated):
//   - Kind 39100: Relay Identity Announcements
//   - Kind 39101: Trust Acts
//   - Kind 39102: Group Tag Acts
//   - Kind 39103: Public Key Advertisements
//   - Kind 39104: Directory Event Replication Requests
//   - Kind 39105: Directory Event Replication Responses
//   - Kind 39106: Group Tag Transfers
//   - Kind 39107: Escrow Witness Completion Acts
func IsDirectoryEventKind(k uint16) (isDirectory bool) {
	switch k {
	case 0, 3, 5, 1984, 10002, 10000, 10050:
		return true
	case 39100, 39101, 39102, 39103, 39104, 39105, 39106, 39107:
		return true
	default:
		return false
	}
}

// ValidateTrustLevel checks if the provided trust level is valid (0-100).
// Returns an error if the level exceeds 100.
func ValidateTrustLevel(level TrustLevel) (err error) {
	if level > 100 {
		return errorf.E("invalid trust level: %d (must be 0-100)", level)
	}
	return nil
}

// ValidateKeyPurpose checks if the provided key purpose is valid.
// Valid purposes are: signing, encryption, delegation.
func ValidateKeyPurpose(purpose string) (err error) {
	switch KeyPurpose(purpose) {
	case KeyPurposeSigning, KeyPurposeEncryption, KeyPurposeDelegation:
		return nil
	default:
		return errorf.E("invalid key purpose: %s", purpose)
	}
}

// ValidateReplicationStatus checks if the provided replication status is valid.
// Valid statuses are: success, error, pending.
func ValidateReplicationStatus(status string) (err error) {
	switch ReplicationStatus(status) {
	case ReplicationStatusSuccess, ReplicationStatusError, ReplicationStatusPending:
		return nil
	default:
		return errorf.E("invalid replication status: %s", status)
	}
}

// CreateBaseEvent creates a basic event structure with common fields set.
// This is used as a starting point for constructing protocol messages.
// The event still needs tags, content, and signature to be complete.
func CreateBaseEvent(pubkey []byte, k *kind.K) (ev *event.E) {
	return &event.E{
		Pubkey:    pubkey,
		CreatedAt: time.Now().Unix(),
		Kind:      k.K,
		Tags:      tag.NewS(),
		Content:   []byte(""),
	}
}
