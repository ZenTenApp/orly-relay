package directory

import (
	"crypto/rand"
	"strconv"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// TrustAct represents a complete Trust Act event (Kind 39101)
// with typed access to its components.
type TrustAct struct {
	Event            *event.E
	TargetPubkey     string
	TrustLevel       TrustLevel
	RelayURL         string
	Expiry           *time.Time
	Reason           TrustReason
	ReplicationKinds []uint16
	IdentityTag      *IdentityTag
}

// IdentityTag represents the I tag with npub identity and proof-of-control.
type IdentityTag struct {
	NPubIdentity string
	Nonce        string
	Signature    string
}

// NewTrustAct creates a new Trust Act event.
func NewTrustAct(
	pubkey []byte,
	targetPubkey string,
	trustLevel TrustLevel,
	relayURL string,
	expiry *time.Time,
	reason TrustReason,
	replicationKinds []uint16,
	identityTag *IdentityTag,
) (ta *TrustAct, err error) {

	// Validate required fields
	if len(pubkey) != 32 {
		return nil, errorf.E("pubkey must be 32 bytes")
	}
	if targetPubkey == "" {
		return nil, errorf.E("target pubkey is required")
	}
	if len(targetPubkey) != 64 {
		return nil, errorf.E("target pubkey must be 64 hex characters")
	}
	if err = ValidateTrustLevel(trustLevel); chk.E(err) {
		return
	}
	if relayURL == "" {
		return nil, errorf.E("relay URL is required")
	}

	// Create base event
	ev := CreateBaseEvent(pubkey, TrustActKind)

	// Add required tags
	ev.Tags.Append(tag.NewFromAny(string(PubkeyTag), targetPubkey))
	ev.Tags.Append(tag.NewFromAny(string(TrustLevelTag), strconv.FormatUint(uint64(trustLevel), 10)))
	ev.Tags.Append(tag.NewFromAny(string(RelayTag), relayURL))

	// Add optional expiry
	if expiry != nil {
		ev.Tags.Append(tag.NewFromAny(string(ExpiryTag), strconv.FormatInt(expiry.Unix(), 10)))
	}

	// Add reason
	if reason != "" {
		ev.Tags.Append(tag.NewFromAny(string(ReasonTag), string(reason)))
	}

	// Add replication kinds (K tag)
	if len(replicationKinds) > 0 {
		var kindStrings []string
		for _, k := range replicationKinds {
			kindStrings = append(kindStrings, strconv.FormatUint(uint64(k), 10))
		}
		ev.Tags.Append(tag.NewFromAny(string(KTag), strings.Join(kindStrings, ",")))
	}

	// Add identity tag if provided
	if identityTag != nil {
		if err = identityTag.Validate(); chk.E(err) {
			return
		}
		ev.Tags.Append(tag.NewFromAny(string(ITag),
			identityTag.NPubIdentity,
			identityTag.Nonce,
			identityTag.Signature))
	}

	ta = &TrustAct{
		Event:            ev,
		TargetPubkey:     targetPubkey,
		TrustLevel:       trustLevel,
		RelayURL:         relayURL,
		Expiry:           expiry,
		Reason:           reason,
		ReplicationKinds: replicationKinds,
		IdentityTag:      identityTag,
	}

	return
}

// ParseTrustAct parses an event into a TrustAct structure
// with validation.
func ParseTrustAct(ev *event.E) (ta *TrustAct, err error) {
	if ev == nil {
		return nil, errorf.E("event cannot be nil")
	}

	// Validate event kind
	if ev.Kind != TrustActKind.K {
		return nil, errorf.E("invalid event kind: expected %d, got %d",
			TrustActKind.K, ev.Kind)
	}

	// Extract required tags
	pTag := ev.Tags.GetFirst(PubkeyTag)
	if pTag == nil {
		return nil, errorf.E("missing p tag")
	}

	trustLevelTag := ev.Tags.GetFirst(TrustLevelTag)
	if trustLevelTag == nil {
		return nil, errorf.E("missing trust_level tag")
	}

	relayTag := ev.Tags.GetFirst(RelayTag)
	if relayTag == nil {
		return nil, errorf.E("missing relay tag")
	}

	// Validate trust level
	var trustLevelValue uint64
	if trustLevelValue, err = strconv.ParseUint(string(trustLevelTag.Value()), 10, 8); chk.E(err) {
		return nil, errorf.E("invalid trust level: %w", err)
	}
	trustLevel := TrustLevel(trustLevelValue)
	if err = ValidateTrustLevel(trustLevel); chk.E(err) {
		return
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

	// Parse optional reason
	var reason TrustReason
	reasonTag := ev.Tags.GetFirst(ReasonTag)
	if reasonTag != nil {
		reason = TrustReason(reasonTag.Value())
	}

	// Parse replication kinds (K tag)
	var replicationKinds []uint16
	kTag := ev.Tags.GetFirst(KTag)
	if kTag != nil {
		kindStrings := strings.Split(string(kTag.Value()), ",")
		for _, kindStr := range kindStrings {
			kindStr = strings.TrimSpace(kindStr)
			if kindStr == "" {
				continue
			}
			var kind uint64
			if kind, err = strconv.ParseUint(kindStr, 10, 16); chk.E(err) {
				return nil, errorf.E("invalid kind in K tag: %s", kindStr)
			}
			replicationKinds = append(replicationKinds, uint16(kind))
		}
	}

	// Parse identity tag (I tag)
	var identityTag *IdentityTag
	iTag := ev.Tags.GetFirst(ITag)
	if iTag != nil {
		if identityTag, err = ParseIdentityTag(iTag); chk.E(err) {
			return
		}
	}

	ta = &TrustAct{
		Event:            ev,
		TargetPubkey:     string(pTag.ValueHex()), // ValueHex() handles binary/hex storage
		TrustLevel:       trustLevel,
		RelayURL:         string(relayTag.Value()),
		Expiry:           expiry,
		Reason:           reason,
		ReplicationKinds: replicationKinds,
		IdentityTag:      identityTag,
	}

	return
}

// ParseIdentityTag parses an I tag into an IdentityTag structure.
func ParseIdentityTag(t *tag.T) (it *IdentityTag, err error) {
	if t == nil {
		return nil, errorf.E("tag cannot be nil")
	}

	if t.Len() < 4 {
		return nil, errorf.E("I tag must have at least 4 elements")
	}

	// First element should be "I"
	if string(t.T[0]) != "I" {
		return nil, errorf.E("invalid I tag key")
	}

	it = &IdentityTag{
		NPubIdentity: string(t.T[1]),
		Nonce:        string(t.T[2]),
		Signature:    string(t.T[3]),
	}

	if err = it.Validate(); chk.E(err) {
		return nil, err
	}
	return it, nil
}

// Validate performs validation of an IdentityTag.
func (it *IdentityTag) Validate() (err error) {
	if it == nil {
		return errorf.E("IdentityTag cannot be nil")
	}

	if it.NPubIdentity == "" {
		return errorf.E("npub identity is required")
	}

	if !strings.HasPrefix(it.NPubIdentity, "npub1") {
		return errorf.E("identity must be npub-encoded")
	}

	if it.Nonce == "" {
		return errorf.E("nonce is required")
	}

	if len(it.Nonce) < 32 { // Minimum 16 bytes hex-encoded
		return errorf.E("nonce must be at least 16 bytes (32 hex characters)")
	}

	if it.Signature == "" {
		return errorf.E("signature is required")
	}

	if len(it.Signature) != 128 { // 64 bytes hex-encoded
		return errorf.E("signature must be 64 bytes (128 hex characters)")
	}

	return nil
}

// Validate performs comprehensive validation of a TrustAct.
func (ta *TrustAct) Validate() (err error) {
	if ta == nil {
		return errorf.E("TrustAct cannot be nil")
	}

	if ta.Event == nil {
		return errorf.E("event cannot be nil")
	}

	// Validate event signature
	if _, err = ta.Event.Verify(); chk.E(err) {
		return errorf.E("invalid event signature: %w", err)
	}

	// Validate required fields
	if ta.TargetPubkey == "" {
		return errorf.E("target pubkey is required")
	}

	if len(ta.TargetPubkey) != 64 {
		return errorf.E("target pubkey must be 64 hex characters")
	}

	if err = ValidateTrustLevel(ta.TrustLevel); chk.E(err) {
		return
	}

	if ta.RelayURL == "" {
		return errorf.E("relay URL is required")
	}

	// Validate expiry if present
	if ta.Expiry != nil && ta.Expiry.Before(time.Now()) {
		return errorf.E("trust act has expired")
	}

	// Validate identity tag if present
	if ta.IdentityTag != nil {
		if err = ta.IdentityTag.Validate(); chk.E(err) {
			return
		}
	}

	return nil
}

// IsExpired returns true if the trust act has expired.
func (ta *TrustAct) IsExpired() bool {
	return ta.Expiry != nil && ta.Expiry.Before(time.Now())
}

// HasReplicationKind returns true if the act includes the specified
// kind for replication.
func (ta *TrustAct) HasReplicationKind(kind uint16) bool {
	for _, k := range ta.ReplicationKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// ShouldReplicate returns true if an event of the given kind should be
// replicated based on this trust act.
func (ta *TrustAct) ShouldReplicate(kind uint16) bool {
	// Directory events are always replicated
	if IsDirectoryEventKind(kind) {
		return true
	}

	// Check if kind is in the replication list
	return ta.HasReplicationKind(kind)
}

// ShouldReplicateEvent determines whether a specific event should be replicated
// based on the trust level using partial replication (random dice-throw).
// This function uses crypto/rand for cryptographically secure randomness.
func (ta *TrustAct) ShouldReplicateEvent(kind uint16) (shouldReplicate bool, err error) {
	// Check if kind is eligible for replication
	if !ta.ShouldReplicate(kind) {
		return false, nil
	}

	// Trust level of 100 means always replicate
	if ta.TrustLevel == TrustLevelFull {
		return true, nil
	}

	// Trust level of 0 means never replicate
	if ta.TrustLevel == TrustLevelNone {
		return false, nil
	}

	// Generate cryptographically secure random number 0-100
	var randomBytes [1]byte
	if _, err = rand.Read(randomBytes[:]); chk.E(err) {
		return false, errorf.E("failed to generate random number: %w", err)
	}

	// Scale byte value (0-255) to 0-100 range
	randomValue := uint8((uint16(randomBytes[0]) * 101) / 256)

	// Replicate if random value is less than or equal to trust level
	shouldReplicate = randomValue <= uint8(ta.TrustLevel)
	return
}

// GetTargetPubkey returns the target relay's public key.
func (ta *TrustAct) GetTargetPubkey() string {
	return ta.TargetPubkey
}

// GetTrustLevel returns the trust level.
func (ta *TrustAct) GetTrustLevel() TrustLevel {
	return ta.TrustLevel
}

// GetRelayURL returns the target relay's URL.
func (ta *TrustAct) GetRelayURL() string {
	return ta.RelayURL
}

// GetExpiry returns the expiry time, or nil if no expiry is set.
func (ta *TrustAct) GetExpiry() *time.Time {
	return ta.Expiry
}

// GetReason returns the reason for the trust relationship.
func (ta *TrustAct) GetReason() TrustReason {
	return ta.Reason
}

// GetReplicationKinds returns the list of event kinds to replicate.
func (ta *TrustAct) GetReplicationKinds() []uint16 {
	return ta.ReplicationKinds
}

// GetIdentityTag returns the identity tag, or nil if not present.
func (ta *TrustAct) GetIdentityTag() *IdentityTag {
	return ta.IdentityTag
}
