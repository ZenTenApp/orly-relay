package marmot

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/emersion/go-mls"
	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer"
)

const (
	kindSeal uint16 = 13
)

// UnwrappedWelcome holds the result of unwrapping a NIP-EE gift-wrapped Welcome.
type UnwrappedWelcome struct {
	Welcome   *mls.Welcome
	SenderPub []byte // real sender pubkey from the seal layer
}

// WelcomeToGiftWrap creates a NIP-59 gift-wrapped event containing an MLS
// Welcome message per MIP-02.
//
// Structure: kind 444 (unsigned rumor) -> kind 13 (seal) -> kind 1059 (gift wrap).
//
// The kind 444 inner event is unsigned (no sig field), with base64-encoded
// Welcome content and tags: ["e", keypackage_event_id], ["relays", ...],
// ["encoding", "base64"].
func WelcomeToGiftWrap(welcome *mls.Welcome, recipientPub []byte, sign signer.I, kpEvent *event.E, relays []string) (*event.E, error) {
	welcomeBytes := welcome.Bytes()
	recipientPubHex := hex.Enc(recipientPub)
	now := time.Now().Unix()

	// Layer 1: Kind 444 unsigned rumor with base64 Welcome content
	innerTags := []*tag.T{
		tag.NewFromAny("encoding", "base64"),
	}

	// Add key package event reference
	if kpEvent != nil {
		innerTags = append(innerTags, tag.NewFromAny("e", hex.Enc(kpEvent.ID)))
	}

	// Add relays tag
	if len(relays) > 0 {
		relayArgs := make([]any, 0, len(relays)+1)
		relayArgs = append(relayArgs, "relays")
		for _, r := range relays {
			relayArgs = append(relayArgs, r)
		}
		innerTags = append(innerTags, tag.NewFromAny(relayArgs...))
	}

	inner := &event.E{
		Content:   []byte(base64.StdEncoding.EncodeToString(welcomeBytes)),
		CreatedAt: now,
		Kind:      KindWelcome,
		Tags:      tag.NewS(innerTags...),
		Pubkey:    sign.Pub(),
	}
	// Kind 444 is unsigned per MIP-02 (no sig field)
	// Compute event ID from canonical form
	inner.ID = inner.GetIDBytes()

	innerJSON, err := inner.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal welcome event: %w", err)
	}

	// Layer 2: Kind 13 seal encrypted with (sender_secret, recipient_pubkey)
	sealConvKey, err := encryption.GenerateConversationKey(sign.Sec(), recipientPub)
	if err != nil {
		return nil, fmt.Errorf("seal ECDH: %w", err)
	}
	sealCiphertext, err := encryption.Encrypt(sealConvKey, innerJSON, nil)
	if err != nil {
		return nil, fmt.Errorf("seal encrypt: %w", err)
	}

	seal := &event.E{
		Content:   []byte(sealCiphertext),
		CreatedAt: randomizeTimestamp(now),
		Kind:      kindSeal,
		Tags:      tag.NewS(),
	}
	if err := seal.Sign(sign); err != nil {
		return nil, fmt.Errorf("sign seal: %w", err)
	}

	sealJSON, err := seal.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal seal: %w", err)
	}

	// Layer 3: Kind 1059 gift wrap with ephemeral key
	ephSecret, err := keys.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephSigner, err := keys.SecretBytesToSigner(ephSecret)
	if err != nil {
		return nil, fmt.Errorf("create ephemeral signer: %w", err)
	}
	defer ephSigner.Zero()

	wrapConvKey, err := encryption.GenerateConversationKey(ephSecret, recipientPub)
	if err != nil {
		return nil, fmt.Errorf("gift wrap ECDH: %w", err)
	}
	wrapCiphertext, err := encryption.Encrypt(wrapConvKey, sealJSON, nil)
	if err != nil {
		return nil, fmt.Errorf("gift wrap encrypt: %w", err)
	}

	gw := &event.E{
		Content:   []byte(wrapCiphertext),
		CreatedAt: randomizeTimestamp(now),
		Kind:      KindGiftWrap,
		Tags: tag.NewS(
			tag.NewFromAny("p", recipientPubHex),
		),
	}
	if err := gw.Sign(ephSigner); err != nil {
		return nil, fmt.Errorf("sign gift wrap: %w", err)
	}
	return gw, nil
}

// UnwrapWelcome decrypts a NIP-59 gift-wrapped event and extracts the MLS
// Welcome message. Returns the Welcome and the sender's real pubkey.
func UnwrapWelcome(ev *event.E, sign signer.I) (*UnwrappedWelcome, error) {
	if ev.Kind != KindGiftWrap {
		return nil, fmt.Errorf("expected kind %d, got %d", KindGiftWrap, ev.Kind)
	}

	// Layer 3 -> 2: Decrypt gift wrap
	convKey, err := encryption.GenerateConversationKey(sign.Sec(), ev.Pubkey)
	if err != nil {
		return nil, fmt.Errorf("gift wrap ECDH: %w", err)
	}
	sealJSON, err := encryption.Decrypt(convKey, string(ev.Content))
	if err != nil {
		return nil, fmt.Errorf("gift wrap decrypt: %w", err)
	}

	// Parse the seal event (kind 13)
	var seal event.E
	if err := seal.UnmarshalJSON([]byte(sealJSON)); err != nil {
		return nil, fmt.Errorf("parse seal: %w", err)
	}
	if seal.Kind != kindSeal {
		return nil, fmt.Errorf("expected seal kind %d, got %d", kindSeal, seal.Kind)
	}

	// Layer 2 -> 1: Decrypt seal
	sealConvKey, err := encryption.GenerateConversationKey(sign.Sec(), seal.Pubkey)
	if err != nil {
		return nil, fmt.Errorf("seal ECDH: %w", err)
	}
	innerJSON, err := encryption.Decrypt(sealConvKey, string(seal.Content))
	if err != nil {
		return nil, fmt.Errorf("seal decrypt: %w", err)
	}

	// Parse the inner Welcome event (kind 444)
	var inner event.E
	if err := inner.UnmarshalJSON([]byte(innerJSON)); err != nil {
		return nil, fmt.Errorf("parse inner welcome: %w", err)
	}
	if inner.Kind != KindWelcome {
		return nil, fmt.Errorf("expected welcome kind %d, got %d", KindWelcome, inner.Kind)
	}

	// Decode content: base64 or raw binary (legacy)
	content := inner.Content
	encodingTag := inner.Tags.GetFirst([]byte("encoding"))
	if encodingTag != nil && string(encodingTag.Value()) == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(string(content))
		if err != nil {
			return nil, fmt.Errorf("base64 decode welcome: %w", err)
		}
		content = decoded
	}

	welcome, err := mls.UnmarshalWelcome(content)
	if err != nil {
		return nil, fmt.Errorf("unmarshal welcome: %w", err)
	}

	return &UnwrappedWelcome{
		Welcome:   welcome,
		SenderPub: seal.Pubkey,
	}, nil
}

// randomizeTimestamp subtracts a random offset of 0-2 days for NIP-59 privacy.
func randomizeTimestamp(base int64) int64 {
	const twoDays = 2 * 24 * 60 * 60
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return base
	}
	n := int64(binary.LittleEndian.Uint64(buf[:]))
	if n < 0 {
		n = -n
	}
	return base - (n % twoDays)
}
