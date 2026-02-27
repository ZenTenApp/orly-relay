package directory

import (
	"encoding/json"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

// RelayIdentityContent represents the JSON content of a Relay Identity
// Announcement event (Kind 39100).
type RelayIdentityContent struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Contact     string `json:"contact,omitempty"`
}

// RelayIdentityAnnouncement represents a complete Relay Identity Announcement
// event with typed access to its components.
type RelayIdentityAnnouncement struct {
	Event         *event.E
	Content       *RelayIdentityContent
	RelayURL      string
	SigningKey    string
	EncryptionKey string
	Version       string
}

// NewRelayIdentityAnnouncement creates a new Relay Identity Announcement event.
func NewRelayIdentityAnnouncement(
	pubkey []byte,
	name, description, contact string,
	relayURL, signingKey, encryptionKey, version string,
) (ria *RelayIdentityAnnouncement, err error) {

	// Validate required fields
	if len(pubkey) != 32 {
		return nil, errorf.E("pubkey must be 32 bytes")
	}
	if name == "" {
		return nil, errorf.E("name is required")
	}
	if relayURL == "" {
		return nil, errorf.E("relay URL is required")
	}
	if signingKey == "" {
		return nil, errorf.E("signing key is required")
	}
	if encryptionKey == "" {
		return nil, errorf.E("encryption key is required")
	}
	if version == "" {
		version = "1" // Default version
	}

	// Create content
	content := &RelayIdentityContent{
		Name:        name,
		Description: description,
		Contact:     contact,
	}

	// Marshal content to JSON
	var contentBytes []byte
	if contentBytes, err = json.Marshal(content); chk.E(err) {
		return
	}

	// Create base event
	ev := CreateBaseEvent(pubkey, RelayIdentityAnnouncementKind)
	ev.Content = contentBytes

	// Add required tags
	ev.Tags.Append(tag.NewFromAny(string(DTag), "relay-identity"))
	ev.Tags.Append(tag.NewFromAny(string(RelayTag), relayURL))
	ev.Tags.Append(tag.NewFromAny(string(SigningKeyTag), signingKey))
	ev.Tags.Append(tag.NewFromAny(string(EncryptionKeyTag), encryptionKey))
	ev.Tags.Append(tag.NewFromAny(string(VersionTag), version))

	ria = &RelayIdentityAnnouncement{
		Event:         ev,
		Content:       content,
		RelayURL:      relayURL,
		SigningKey:    signingKey,
		EncryptionKey: encryptionKey,
		Version:       version,
	}

	return
}

// ParseRelayIdentityAnnouncement parses an event into a RelayIdentityAnnouncement
// structure with validation.
func ParseRelayIdentityAnnouncement(ev *event.E) (ria *RelayIdentityAnnouncement, err error) {
	if ev == nil {
		return nil, errorf.E("event cannot be nil")
	}

	// Validate event kind
	if ev.Kind != RelayIdentityAnnouncementKind.K {
		return nil, errorf.E("invalid event kind: expected %d, got %d",
			RelayIdentityAnnouncementKind.K, ev.Kind)
	}

	// Parse content
	var content RelayIdentityContent
	if len(ev.Content) > 0 {
		if err = json.Unmarshal(ev.Content, &content); chk.E(err) {
			return nil, errorf.E("failed to parse content: %w", err)
		}
	}

	// Extract required tags
	dTag := ev.Tags.GetFirst(DTag)
	if dTag == nil || string(dTag.Value()) != "relay-identity" {
		return nil, errorf.E("missing or invalid d tag")
	}

	relayTag := ev.Tags.GetFirst(RelayTag)
	if relayTag == nil {
		return nil, errorf.E("missing relay tag")
	}

	signingKeyTag := ev.Tags.GetFirst(SigningKeyTag)
	if signingKeyTag == nil {
		return nil, errorf.E("missing signing_key tag")
	}

	encryptionKeyTag := ev.Tags.GetFirst(EncryptionKeyTag)
	if encryptionKeyTag == nil {
		return nil, errorf.E("missing encryption_key tag")
	}

	versionTag := ev.Tags.GetFirst(VersionTag)
	if versionTag == nil {
		return nil, errorf.E("missing version tag")
	}

	ria = &RelayIdentityAnnouncement{
		Event:         ev,
		Content:       &content,
		RelayURL:      string(relayTag.Value()),
		SigningKey:    string(signingKeyTag.Value()),
		EncryptionKey: string(encryptionKeyTag.Value()),
		Version:       string(versionTag.Value()),
	}

	return
}

// Validate performs comprehensive validation of a RelayIdentityAnnouncement.
func (ria *RelayIdentityAnnouncement) Validate() (err error) {
	if ria == nil {
		return errorf.E("RelayIdentityAnnouncement cannot be nil")
	}

	if ria.Event == nil {
		return errorf.E("event cannot be nil")
	}

	// Validate event signature
	if _, err = ria.Event.Verify(); chk.E(err) {
		return errorf.E("invalid event signature: %w", err)
	}

	// Validate required fields
	if ria.Content.Name == "" {
		return errorf.E("name is required")
	}

	if ria.RelayURL == "" {
		return errorf.E("relay URL is required")
	}

	if ria.SigningKey == "" {
		return errorf.E("signing key is required")
	}

	if ria.EncryptionKey == "" {
		return errorf.E("encryption key is required")
	}

	if ria.Version == "" {
		return errorf.E("version is required")
	}

	// Validate hex-encoded keys (should be 64 characters for 32-byte keys)
	if len(ria.SigningKey) != 64 {
		return errorf.E("signing key must be 64 hex characters")
	}

	if len(ria.EncryptionKey) != 64 {
		return errorf.E("encryption key must be 64 hex characters")
	}

	return nil
}

// GetRelayURL returns the relay WebSocket URL.
func (ria *RelayIdentityAnnouncement) GetRelayURL() string {
	return ria.RelayURL
}

// GetSigningKey returns the hex-encoded signing public key.
func (ria *RelayIdentityAnnouncement) GetSigningKey() string {
	return ria.SigningKey
}

// GetEncryptionKey returns the hex-encoded encryption public key.
func (ria *RelayIdentityAnnouncement) GetEncryptionKey() string {
	return ria.EncryptionKey
}

// GetVersion returns the protocol version.
func (ria *RelayIdentityAnnouncement) GetVersion() string {
	return ria.Version
}

// GetName returns the relay name from the content.
func (ria *RelayIdentityAnnouncement) GetName() string {
	if ria.Content == nil {
		return ""
	}
	return ria.Content.Name
}

// GetDescription returns the relay description from the content.
func (ria *RelayIdentityAnnouncement) GetDescription() string {
	if ria.Content == nil {
		return ""
	}
	return ria.Content.Description
}

// GetContact returns the relay contact information from the content.
func (ria *RelayIdentityAnnouncement) GetContact() string {
	if ria.Content == nil {
		return ""
	}
	return ria.Content.Contact
}
