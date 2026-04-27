package marmot

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/emersion/go-mls"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
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
func WelcomeToGiftWrap(welcome *mls.Welcome, recipientPub []byte, crypto CryptoProvider, kpEvent *event.E, relays []string) (*event.E, error) {
	welcomeBytes := welcome.Bytes()
	recipientPubHex := hex.Enc(recipientPub)
	now := time.Now().Unix()

	// Layer 1: Kind 444 unsigned rumor with base64 Welcome content
	innerTags := []*tag.T{
		tag.NewFromAny("encoding", "base64"),
	}

	if kpEvent != nil {
		innerTags = append(innerTags, tag.NewFromAny("e", hex.Enc(kpEvent.ID)))
	}

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
		Pubkey:    crypto.Pub(),
	}
	inner.ID = inner.GetIDBytes()

	innerJSON, err := inner.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal welcome event: %w", err)
	}

	// Layer 2: Kind 13 seal — NIP-44 encrypt with sender identity
	sealCiphertext, err := crypto.Nip44Encrypt(recipientPub, innerJSON)
	if err != nil {
		return nil, fmt.Errorf("seal encrypt: %w", err)
	}

	seal := &event.E{
		Content:   []byte(sealCiphertext),
		CreatedAt: randomizeTimestamp(now),
		Kind:      kindSeal,
		Tags:      tag.NewS(),
	}
	if err := crypto.SignEvent(seal); err != nil {
		return nil, fmt.Errorf("sign seal: %w", err)
	}

	sealJSON, err := seal.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal seal: %w", err)
	}

	// Layer 3: Kind 1059 gift wrap — ephemeral key (always local)
	ephSecret, err := keys.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephSigner, err := keys.SecretBytesToSigner(ephSecret)
	if err != nil {
		return nil, fmt.Errorf("create ephemeral signer: %w", err)
	}
	defer ephSigner.Zero()

	ephCrypto := &LocalCrypto{Sign: ephSigner}
	wrapCiphertext, err := ephCrypto.Nip44Encrypt(recipientPub, sealJSON)
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

// UnwrappedGiftWrap holds the result of unwrapping any NIP-59 gift-wrapped event.
// The inner event may be kind 444 (MLS welcome) or kind 14 (NIP-17 DM) etc.
type UnwrappedGiftWrap struct {
	Inner     *event.E // the decrypted inner event (rumor)
	SenderPub []byte   // real sender pubkey from the seal layer
}

// UnwrapGiftWrap decrypts a NIP-59 gift-wrapped event through both layers
// (gift wrap → seal → inner) and returns the inner event regardless of its kind.
func UnwrapGiftWrap(ev *event.E, crypto CryptoProvider) (*UnwrappedGiftWrap, error) {
	if ev.Kind != KindGiftWrap {
		return nil, fmt.Errorf("expected kind %d, got %d", KindGiftWrap, ev.Kind)
	}

	// Layer 3 -> 2: Decrypt gift wrap via NIP-44
	sealJSON, err := crypto.Nip44Decrypt(ev.Pubkey, string(ev.Content))
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

	// Layer 2 -> 1: Decrypt seal via NIP-44
	innerJSON, err := crypto.Nip44Decrypt(seal.Pubkey, string(seal.Content))
	if err != nil {
		return nil, fmt.Errorf("seal decrypt: %w", err)
	}

	// Parse the inner event (kind varies: 444 welcome, 14 NIP-17 DM, etc.)
	var inner event.E
	if err := inner.UnmarshalJSON([]byte(innerJSON)); err != nil {
		return nil, fmt.Errorf("parse inner event: %w", err)
	}

	return &UnwrappedGiftWrap{
		Inner:     &inner,
		SenderPub: seal.Pubkey,
	}, nil
}

// UnwrapWelcome decrypts a NIP-59 gift-wrapped event and extracts the MLS
// Welcome message. Returns the Welcome and the sender's real pubkey.
func UnwrapWelcome(ev *event.E, crypto CryptoProvider) (*UnwrappedWelcome, error) {
	uw, err := UnwrapGiftWrap(ev, crypto)
	if err != nil {
		return nil, err
	}
	if uw.Inner.Kind != KindWelcome {
		return nil, fmt.Errorf("expected welcome kind %d, got %d", KindWelcome, uw.Inner.Kind)
	}

	// Decode content: base64 or raw binary (legacy)
	content := uw.Inner.Content
	encodingTag := uw.Inner.Tags.GetFirst([]byte("encoding"))
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
		SenderPub: uw.SenderPub,
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
