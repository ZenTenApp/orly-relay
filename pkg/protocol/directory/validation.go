package directory

import (
	"github.com/minio/sha256-simd"
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/nostr/crypto/ec/schnorr"
	"git.smesh.lol/orly/pkg/nostr/crypto/ec/secp256k1"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

// Validation constants
const (
	MaxKeyDelegations = 512
	KeyExpirationDays = 30
	MinNonceSize      = 16    // bytes
	MaxContentLength  = 65536 // bytes
)

// Regular expressions for validation
var (
	hexKeyRegex       = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	npubRegex         = regexp.MustCompile(`^npub1[0-9a-z]+$`)
	wsURLRegex        = regexp.MustCompile(`^wss?://[a-zA-Z0-9.-]+(?::[0-9]+)?(?:/.*)?$`)
	groupTagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._~-]+$`) // RFC 3986 URL-safe characters
)

// ValidateGroupTagName validates that a group tag name is URL-safe (RFC 3986).
func ValidateGroupTagName(name string) (err error) {
	if len(name) < 1 {
		return errorf.E("group tag name cannot be empty")
	}
	if len(name) > 255 {
		return errorf.E("group tag name cannot exceed 255 characters")
	}

	// Check for reserved prefixes
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return errorf.E("group tag names starting with '.' or '_' are reserved for system use")
	}

	// Validate URL-safe character set
	if !groupTagNameRegex.MatchString(name) {
		return errorf.E("group tag name must contain only URL-safe characters (a-z, A-Z, 0-9, -, ., _, ~)")
	}

	return nil
}

// ValidateHexKey validates that a string is a valid 64-character hex key.
func ValidateHexKey(key string) (err error) {
	if !hexKeyRegex.MatchString(key) {
		return errorf.E("invalid hex key format: must be 64 hex characters")
	}
	return nil
}

// ValidateNPub validates that a string is a valid npub-encoded public key.
func ValidateNPub(npub string) (err error) {
	if !npubRegex.MatchString(npub) {
		return errorf.E("invalid npub format")
	}

	// Try to decode to verify it's valid
	if _, err = bech32encoding.NpubToBytes(npub); chk.E(err) {
		return errorf.E("invalid npub encoding: %w", err)
	}

	return nil
}

// ValidateWebSocketURL validates that a string is a valid WebSocket URL.
func ValidateWebSocketURL(wsURL string) (err error) {
	if !wsURLRegex.MatchString(wsURL) {
		return errorf.E("invalid WebSocket URL format")
	}

	// Parse URL for additional validation
	var u *url.URL
	if u, err = url.Parse(wsURL); chk.E(err) {
		return errorf.E("invalid URL: %w", err)
	}

	if u.Scheme != "ws" && u.Scheme != "wss" {
		return errorf.E("URL must use ws:// or wss:// scheme")
	}

	if u.Host == "" {
		return errorf.E("URL must have a host")
	}

	return nil
}

// ValidateNonce validates that a nonce meets minimum security requirements.
func ValidateNonce(nonce string) (err error) {
	if len(nonce) < MinNonceSize*2 { // hex-encoded, so double the byte length
		return errorf.E("nonce must be at least %d bytes (%d hex characters)",
			MinNonceSize, MinNonceSize*2)
	}

	// Verify it's valid hex
	if _, err = hex.DecodeString(nonce); chk.E(err) {
		return errorf.E("nonce must be valid hex: %w", err)
	}

	return nil
}

// ValidateSignature validates that a signature is properly formatted.
func ValidateSignature(sig string) (err error) {
	if len(sig) != 128 { // 64 bytes hex-encoded
		return errorf.E("signature must be 64 bytes (128 hex characters)")
	}

	// Verify it's valid hex
	if _, err = hex.DecodeString(sig); chk.E(err) {
		return errorf.E("signature must be valid hex: %w", err)
	}

	return nil
}

// ValidateDerivationPath validates a BIP32 derivation path for this protocol.
func ValidateDerivationPath(path string) (err error) {
	// Expected format: m/39103'/1237'/identity'/usage/index
	if !strings.HasPrefix(path, "m/39103'/1237'/") {
		return errorf.E("derivation path must start with m/39103'/1237'/")
	}

	parts := strings.Split(path, "/")
	if len(parts) != 6 {
		return errorf.E("derivation path must have 6 components")
	}

	// Validate hardened components
	if parts[1] != "39103'" || parts[2] != "1237'" {
		return errorf.E("invalid hardened components in derivation path")
	}

	// Identity component should be hardened (end with ')
	if !strings.HasSuffix(parts[3], "'") {
		return errorf.E("identity component must be hardened")
	}

	return nil
}

// ValidateEventContent validates that event content is within size limits.
func ValidateEventContent(content []byte) (err error) {
	if len(content) > MaxContentLength {
		return errorf.E("content exceeds maximum length of %d bytes", MaxContentLength)
	}
	return nil
}

// ValidateTimestamp validates that a timestamp is reasonable (not too far in past/future).
func ValidateTimestamp(ts int64) (err error) {
	now := time.Now().Unix()

	// Allow up to 1 hour in the future
	if ts > now+3600 {
		return errorf.E("timestamp too far in the future")
	}

	// Allow up to 1 year in the past
	if ts < now-31536000 {
		return errorf.E("timestamp too far in the past")
	}

	return nil
}

// VerifyIdentityTagSignature verifies the signature in an identity tag.
func VerifyIdentityTagSignature(
	identityTag *IdentityTag,
	delegatePubkey []byte,
) (valid bool, err error) {
	if identityTag == nil {
		return false, errorf.E("identity tag cannot be nil")
	}

	// Decode npub to get identity public key
	var identityPubkey []byte
	if identityPubkey, err = bech32encoding.NpubToBytes(identityTag.NPubIdentity); chk.E(err) {
		return false, errorf.E("failed to decode npub: %w", err)
	}

	// Decode nonce and signature
	var nonce, signature []byte
	if nonce, err = hex.DecodeString(identityTag.Nonce); chk.E(err) {
		return false, errorf.E("invalid nonce hex: %w", err)
	}
	if signature, err = hex.DecodeString(identityTag.Signature); chk.E(err) {
		return false, errorf.E("invalid signature hex: %w", err)
	}

	// Create message to verify: nonce + delegate_pubkey_hex + identity_pubkey_hex
	message := make([]byte, 0, len(nonce)+64+64)
	message = append(message, nonce...)
	message = append(message, []byte(hex.EncodeToString(delegatePubkey))...)
	message = append(message, []byte(hex.EncodeToString(identityPubkey))...)

	// Hash the message
	hash := sha256.Sum256(message)

	// Parse signature and verify
	var sig *schnorr.Signature
	if sig, err = schnorr.ParseSignature(signature); chk.E(err) {
		return false, errorf.E("failed to parse signature: %w", err)
	}

	// Parse public key
	var pubKey *secp256k1.PublicKey
	if pubKey, err = schnorr.ParsePubKey(identityPubkey); chk.E(err) {
		return false, errorf.E("failed to parse public key: %w", err)
	}

	return sig.Verify(hash[:], pubKey), nil
}

// ValidateEventKindForReplication validates that an event kind is appropriate
// for replication in the directory consensus protocol.
func ValidateEventKindForReplication(kind uint16) (err error) {
	// Directory events are always valid
	if IsDirectoryEventKind(kind) {
		return nil
	}

	// Protocol events (39100-39105) should not be replicated as regular events
	if kind >= 39100 && kind <= 39105 {
		return errorf.E("protocol events should not be replicated as directory events")
	}

	// Ephemeral events (20000-29999) should not be stored
	if kind >= 20000 && kind <= 29999 {
		return errorf.E("ephemeral events should not be replicated")
	}

	return nil
}

// ValidateRelayIdentityBinding verifies that a relay identity announcement
// is properly bound to its network address through NIP-11 signature verification.
func ValidateRelayIdentityBinding(
	announcement *RelayIdentityAnnouncement,
	nip11Pubkey, nip11Nonce, nip11Sig, relayAddress string,
) (valid bool, err error) {
	if announcement == nil {
		return false, errorf.E("announcement cannot be nil")
	}

	// Verify the announcement event pubkey matches the NIP-11 pubkey
	announcementPubkeyHex := hex.EncodeToString(announcement.Event.Pubkey)
	if announcementPubkeyHex != nip11Pubkey {
		return false, errorf.E("announcement pubkey does not match NIP-11 pubkey")
	}

	// Verify NIP-11 signature format
	if err = ValidateHexKey(nip11Pubkey); chk.E(err) {
		return false, errorf.E("invalid NIP-11 pubkey: %w", err)
	}
	if err = ValidateNonce(nip11Nonce); chk.E(err) {
		return false, errorf.E("invalid NIP-11 nonce: %w", err)
	}
	if err = ValidateSignature(nip11Sig); chk.E(err) {
		return false, errorf.E("invalid NIP-11 signature: %w", err)
	}

	// Decode components
	var pubkey, signature []byte
	if pubkey, err = hex.DecodeString(nip11Pubkey); chk.E(err) {
		return false, errorf.E("failed to decode NIP-11 pubkey: %w", err)
	}
	if signature, err = hex.DecodeString(nip11Sig); chk.E(err) {
		return false, errorf.E("failed to decode NIP-11 signature: %w", err)
	}

	// Create message: pubkey + nonce + relay_address
	message := nip11Pubkey + nip11Nonce + relayAddress
	hash := sha256.Sum256([]byte(message))

	// Parse signature and verify
	var sig *schnorr.Signature
	if sig, err = schnorr.ParseSignature(signature); chk.E(err) {
		return false, errorf.E("failed to parse signature: %w", err)
	}

	// Parse public key
	var pubKey *secp256k1.PublicKey
	if pubKey, err = schnorr.ParsePubKey(pubkey); chk.E(err) {
		return false, errorf.E("failed to parse public key: %w", err)
	}

	return sig.Verify(hash[:], pubKey), nil
}

// ValidateConsortiumEvent performs comprehensive validation of any consortium
// protocol event, including signature verification and protocol-specific checks.
func ValidateConsortiumEvent(ev *event.E) (err error) {
	if ev == nil {
		return errorf.E("event cannot be nil")
	}

	// Verify basic event signature
	if _, err = ev.Verify(); chk.E(err) {
		return errorf.E("invalid event signature: %w", err)
	}

	// Validate timestamp
	if err = ValidateTimestamp(ev.CreatedAt); chk.E(err) {
		return errorf.E("invalid timestamp: %w", err)
	}

	// Validate content size
	if err = ValidateEventContent(ev.Content); chk.E(err) {
		return errorf.E("invalid content: %w", err)
	}

	// Protocol-specific validation based on event kind
	switch ev.Kind {
	case RelayIdentityAnnouncementKind.K:
		var ria *RelayIdentityAnnouncement
		if ria, err = ParseRelayIdentityAnnouncement(ev); chk.E(err) {
			return errorf.E("failed to parse relay identity announcement: %w", err)
		}
		return ria.Validate()

	case TrustActKind.K:
		var ta *TrustAct
		if ta, err = ParseTrustAct(ev); chk.E(err) {
			return errorf.E("failed to parse trust act: %w", err)
		}
		return ta.Validate()

	case GroupTagActKind.K:
		var gta *GroupTagAct
		if gta, err = ParseGroupTagAct(ev); chk.E(err) {
			return errorf.E("failed to parse group tag act: %w", err)
		}
		return gta.Validate()

	case PublicKeyAdvertisementKind.K:
		var pka *PublicKeyAdvertisement
		if pka, err = ParsePublicKeyAdvertisement(ev); chk.E(err) {
			return errorf.E("failed to parse public key advertisement: %w", err)
		}
		return pka.Validate()

	case DirectoryEventReplicationRequestKind.K:
		var derr *DirectoryEventReplicationRequest
		if derr, err = ParseDirectoryEventReplicationRequest(ev); chk.E(err) {
			return errorf.E("failed to parse replication request: %w", err)
		}
		return derr.Validate()

	case DirectoryEventReplicationResponseKind.K:
		var derr *DirectoryEventReplicationResponse
		if derr, err = ParseDirectoryEventReplicationResponse(ev); chk.E(err) {
			return errorf.E("failed to parse replication response: %w", err)
		}
		return derr.Validate()

	default:
		return errorf.E("unknown consortium event kind: %d", ev.Kind)
	}
}

// IsConsortiumEvent returns true if the event is a consortium protocol event.
func IsConsortiumEvent(ev *event.E) bool {
	if ev == nil {
		return false
	}
	return ev.Kind >= 39100 && ev.Kind <= 39105
}
