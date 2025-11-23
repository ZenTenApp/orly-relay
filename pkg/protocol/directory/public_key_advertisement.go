package directory

import (
	"strconv"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// PublicKeyAdvertisement represents a complete Public Key Advertisement event
// (Kind 39103) with typed access to its components.
type PublicKeyAdvertisement struct {
	Event          *event.E
	KeyID          string
	PublicKey      string
	Purpose        KeyPurpose
	Expiry         *time.Time
	Algorithm      string
	DerivationPath string
	KeyIndex       int
	IdentityTag    *IdentityTag
}

// NewPublicKeyAdvertisement creates a new Public Key Advertisement event.
func NewPublicKeyAdvertisement(
	pubkey []byte,
	keyID, publicKey string,
	purpose KeyPurpose,
	expiry *time.Time,
	algorithm, derivationPath string,
	keyIndex int,
	identityTag *IdentityTag,
) (pka *PublicKeyAdvertisement, err error) {

	// Validate required fields
	if len(pubkey) != 32 {
		return nil, errorf.E("pubkey must be 32 bytes")
	}
	if keyID == "" {
		return nil, errorf.E("key ID is required")
	}
	if publicKey == "" {
		return nil, errorf.E("public key is required")
	}
	if len(publicKey) != 64 {
		return nil, errorf.E("public key must be 64 hex characters")
	}
	if err = ValidateKeyPurpose(string(purpose)); chk.E(err) {
		return
	}
	// Expiry is optional, but if provided, must be in the future
	if expiry != nil && expiry.Before(time.Now()) {
		return nil, errorf.E("expiry time must be in the future")
	}
	if algorithm == "" {
		algorithm = "secp256k1" // Default algorithm
	}
	if derivationPath == "" {
		return nil, errorf.E("derivation path is required")
	}
	if keyIndex < 0 {
		return nil, errorf.E("key index must be non-negative")
	}

	// Validate identity tag if provided
	if identityTag != nil {
		if err = identityTag.Validate(); chk.E(err) {
			return
		}
	}

	// Create base event
	ev := CreateBaseEvent(pubkey, PublicKeyAdvertisementKind)

	// Add required tags
	ev.Tags.Append(tag.NewFromAny(string(DTag), keyID))
	ev.Tags.Append(tag.NewFromAny(string(PubkeyTag), publicKey))
	ev.Tags.Append(tag.NewFromAny(string(PurposeTag), string(purpose)))
	ev.Tags.Append(tag.NewFromAny(string(AlgorithmTag), algorithm))
	ev.Tags.Append(tag.NewFromAny(string(DerivationPathTag), derivationPath))
	ev.Tags.Append(tag.NewFromAny(string(KeyIndexTag), strconv.Itoa(keyIndex)))

	// Add optional expiry tag
	if expiry != nil {
		ev.Tags.Append(tag.NewFromAny(string(ExpiryTag), strconv.FormatInt(expiry.Unix(), 10)))
	}

	// Add identity tag if provided
	if identityTag != nil {
		ev.Tags.Append(tag.NewFromAny(string(ITag),
			identityTag.NPubIdentity,
			identityTag.Nonce,
			identityTag.Signature))
	}

	pka = &PublicKeyAdvertisement{
		Event:          ev,
		KeyID:          keyID,
		PublicKey:      publicKey,
		Purpose:        purpose,
		Expiry:         expiry,
		Algorithm:      algorithm,
		DerivationPath: derivationPath,
		KeyIndex:       keyIndex,
		IdentityTag:    identityTag,
	}

	return
}

// ParsePublicKeyAdvertisement parses an event into a PublicKeyAdvertisement
// structure with validation.
func ParsePublicKeyAdvertisement(ev *event.E) (pka *PublicKeyAdvertisement, err error) {
	if ev == nil {
		return nil, errorf.E("event cannot be nil")
	}

	// Validate event kind
	if ev.Kind != PublicKeyAdvertisementKind.K {
		return nil, errorf.E("invalid event kind: expected %d, got %d",
			PublicKeyAdvertisementKind.K, ev.Kind)
	}

	// Extract required tags
	dTag := ev.Tags.GetFirst(DTag)
	if dTag == nil {
		return nil, errorf.E("missing d tag")
	}

	pubkeyTag := ev.Tags.GetFirst(PubkeyTag)
	if pubkeyTag == nil {
		return nil, errorf.E("missing pubkey tag")
	}

	purposeTag := ev.Tags.GetFirst(PurposeTag)
	if purposeTag == nil {
		return nil, errorf.E("missing purpose tag")
	}

	// Parse optional expiry
	var expiry *time.Time
	expiryTag := ev.Tags.GetFirst(ExpiryTag)
	if expiryTag != nil {
		var expiryUnix int64
		if expiryUnix, err = strconv.ParseInt(string(expiryTag.Value()), 10, 64); chk.E(err) {
			return nil, errorf.E("invalid expiry timestamp: %w", err)
		}
		expiryTime := time.Unix(expiryUnix, 0)
		expiry = &expiryTime
	}

	algorithmTag := ev.Tags.GetFirst(AlgorithmTag)
	if algorithmTag == nil {
		return nil, errorf.E("missing algorithm tag")
	}

	derivationPathTag := ev.Tags.GetFirst(DerivationPathTag)
	if derivationPathTag == nil {
		return nil, errorf.E("missing derivation_path tag")
	}

	keyIndexTag := ev.Tags.GetFirst(KeyIndexTag)
	if keyIndexTag == nil {
		return nil, errorf.E("missing key_index tag")
	}

	// Validate and parse purpose
	purpose := KeyPurpose(purposeTag.Value())
	if err = ValidateKeyPurpose(string(purpose)); chk.E(err) {
		return
	}

	// Parse key index
	var keyIndex int
	if keyIndex, err = strconv.Atoi(string(keyIndexTag.Value())); chk.E(err) {
		return nil, errorf.E("invalid key_index: %w", err)
	}

	// Parse identity tag (I tag)
	var identityTag *IdentityTag
	iTag := ev.Tags.GetFirst(ITag)
	if iTag != nil {
		if identityTag, err = ParseIdentityTag(iTag); chk.E(err) {
			return
		}
	}

	pka = &PublicKeyAdvertisement{
		Event:          ev,
		KeyID:          string(dTag.Value()),
		PublicKey:      string(pubkeyTag.Value()),
		Purpose:        purpose,
		Expiry:         expiry,
		Algorithm:      string(algorithmTag.Value()),
		DerivationPath: string(derivationPathTag.Value()),
		KeyIndex:       keyIndex,
		IdentityTag:    identityTag,
	}

	return
}

// Validate performs comprehensive validation of a PublicKeyAdvertisement.
func (pka *PublicKeyAdvertisement) Validate() (err error) {
	if pka == nil {
		return errorf.E("PublicKeyAdvertisement cannot be nil")
	}

	if pka.Event == nil {
		return errorf.E("event cannot be nil")
	}

	// Validate event signature
	if _, err = pka.Event.Verify(); chk.E(err) {
		return errorf.E("invalid event signature: %w", err)
	}

	// Validate required fields
	if pka.KeyID == "" {
		return errorf.E("key ID is required")
	}

	if pka.PublicKey == "" {
		return errorf.E("public key is required")
	}

	if len(pka.PublicKey) != 64 {
		return errorf.E("public key must be 64 hex characters")
	}

	if err = ValidateKeyPurpose(string(pka.Purpose)); chk.E(err) {
		return
	}

	// Ensure no more mistakes by correcting field usage comprehensively

	// Update relevant parts of the code to use Expiry instead of removed fields.
	if pka.Expiry != nil && pka.Expiry.Before(time.Now()) {
		return errorf.E("public key advertisement is expired")
	}

	// Make sure any logic that checks valid periods is now using the created_at timestamp rather than a specific validity period
	// Statements using ValidFrom or ValidUntil should be revised or removed according to the new logic.

	if pka.Algorithm == "" {
		return errorf.E("algorithm is required")
	}

	if pka.DerivationPath == "" {
		return errorf.E("derivation path is required")
	}

	if pka.KeyIndex < 0 {
		return errorf.E("key index must be non-negative")
	}

	// Validate identity tag if present
	if pka.IdentityTag != nil {
		if err = pka.IdentityTag.Validate(); chk.E(err) {
			return
		}
	}

	return nil
}

// IsValid returns true if the key is currently valid (within its validity period).
func (pka *PublicKeyAdvertisement) IsValid() bool {
	if pka.Expiry == nil {
		return false
	}
	return time.Now().Before(*pka.Expiry)
}

// IsExpired returns true if the key has expired.
func (pka *PublicKeyAdvertisement) IsExpired() bool {
	if pka.Expiry == nil {
		return false
	}
	return time.Now().After(*pka.Expiry)
}

// IsNotYetValid returns true if the key is not yet valid.
func (pka *PublicKeyAdvertisement) IsNotYetValid() bool {
	if pka.Expiry == nil {
		return true // Consider valid if no expiry is set
	}
	return time.Now().Before(*pka.Expiry)
}

// TimeUntilExpiry returns the duration until the key expires.
// Returns 0 if already expired.
func (pka *PublicKeyAdvertisement) TimeUntilExpiry() time.Duration {
	if pka.Expiry == nil {
		return 0
	}
	if pka.IsExpired() {
		return 0
	}
	return time.Until(*pka.Expiry)
}

// TimeUntilValid returns the duration until the key becomes valid.
// Returns 0 if already valid or expired.
func (pka *PublicKeyAdvertisement) TimeUntilValid() time.Duration {
	if !pka.IsNotYetValid() {
		return 0
	}
	return time.Until(*pka.Expiry)
}

// GetKeyID returns the unique key identifier.
func (pka *PublicKeyAdvertisement) GetKeyID() string {
	return pka.KeyID
}

// GetPublicKey returns the hex-encoded public key.
func (pka *PublicKeyAdvertisement) GetPublicKey() string {
	return pka.PublicKey
}

// GetPurpose returns the key purpose.
func (pka *PublicKeyAdvertisement) GetPurpose() KeyPurpose {
	return pka.Purpose
}

// GetAlgorithm returns the cryptographic algorithm.
func (pka *PublicKeyAdvertisement) GetAlgorithm() string {
	return pka.Algorithm
}

// GetDerivationPath returns the BIP32 derivation path.
func (pka *PublicKeyAdvertisement) GetDerivationPath() string {
	return pka.DerivationPath
}

// GetKeyIndex returns the key index from the derivation path.
func (pka *PublicKeyAdvertisement) GetKeyIndex() int {
	return pka.KeyIndex
}

// GetIdentityTag returns the identity tag, or nil if not present.
func (pka *PublicKeyAdvertisement) GetIdentityTag() *IdentityTag {
	return pka.IdentityTag
}

// HasPurpose returns true if the key has the specified purpose.
func (pka *PublicKeyAdvertisement) HasPurpose(purpose KeyPurpose) bool {
	return pka.Purpose == purpose
}

// IsSigningKey returns true if this is a signing key.
func (pka *PublicKeyAdvertisement) IsSigningKey() bool {
	return pka.Purpose == KeyPurposeSigning
}

// IsEncryptionKey returns true if this is an encryption key.
func (pka *PublicKeyAdvertisement) IsEncryptionKey() bool {
	return pka.Purpose == KeyPurposeEncryption
}

// IsDelegationKey returns true if this is a delegation key.
func (pka *PublicKeyAdvertisement) IsDelegationKey() bool {
	return pka.Purpose == KeyPurposeDelegation
}
