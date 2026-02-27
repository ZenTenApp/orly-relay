package bridge

import (
	"fmt"
	"math/rand"
	"time"

	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"next.orly.dev/pkg/lol/log"
)

const (
	kindSeal      uint16 = 13
	kindDM        uint16 = 14
	kindGiftWrap  uint16 = 1059
)

// UnwrappedDM holds the result of unwrapping a NIP-17 gift-wrapped DM.
type UnwrappedDM struct {
	SenderPubHex string
	Content      string
}

// unwrapGiftWrap decrypts a kind 1059 gift-wrapped event to extract the
// inner kind 14 DM. NIP-17 structure: 1059 (gift wrap) → 13 (seal) → 14 (DM).
//
// Layer 1: Gift wrap is NIP-44 encrypted with ephemeral_key + recipient_key.
// Layer 2: Seal is NIP-44 encrypted with sender_key + recipient_key.
// The sender's real pubkey is on the seal (kind 13), not the gift wrap.
func unwrapGiftWrap(ev *event.E, sign signer.I) (*UnwrappedDM, error) {
	if ev.Kind != kindGiftWrap {
		return nil, fmt.Errorf("expected kind %d, got %d", kindGiftWrap, ev.Kind)
	}

	// Layer 1: Decrypt gift wrap using bridge secret + gift wrap pubkey (ephemeral)
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

	// Layer 2: Decrypt seal using bridge secret + seal pubkey (real sender)
	sealConvKey, err := encryption.GenerateConversationKey(sign.Sec(), seal.Pubkey)
	if err != nil {
		return nil, fmt.Errorf("seal ECDH: %w", err)
	}
	innerJSON, err := encryption.Decrypt(sealConvKey, string(seal.Content))
	if err != nil {
		return nil, fmt.Errorf("seal decrypt: %w", err)
	}

	// Parse the inner DM event (kind 14)
	var inner event.E
	if err := inner.UnmarshalJSON([]byte(innerJSON)); err != nil {
		return nil, fmt.Errorf("parse inner DM: %w", err)
	}
	if inner.Kind != kindDM {
		return nil, fmt.Errorf("expected DM kind %d, got %d", kindDM, inner.Kind)
	}

	return &UnwrappedDM{
		SenderPubHex: hex.Enc(seal.Pubkey),
		Content:      string(inner.Content),
	}, nil
}

// wrapGiftWrap creates a NIP-17 gift-wrapped DM (kind 14 → 13 → 1059).
// The outer gift wrap uses an ephemeral key for sender privacy.
func wrapGiftWrap(
	recipientPubHex string, content string, sign signer.I,
) (*event.E, error) {
	recipientPub, err := hex.Dec(recipientPubHex)
	if err != nil {
		return nil, fmt.Errorf("decode recipient pubkey: %w", err)
	}

	now := time.Now().Unix()

	// Step 1: Create inner DM (kind 14)
	inner := &event.E{
		Content:   []byte(content),
		CreatedAt: now,
		Kind:      kindDM,
		Tags: tag.NewS(
			tag.NewFromAny("p", recipientPubHex),
		),
	}
	if err := inner.Sign(sign); err != nil {
		return nil, fmt.Errorf("sign inner DM: %w", err)
	}

	innerJSON, err := inner.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal inner DM: %w", err)
	}

	// Step 2: Create seal (kind 13) — encrypt inner with sender + recipient
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
		Tags:      tag.NewS(), // no tags on seal
	}
	if err := seal.Sign(sign); err != nil {
		return nil, fmt.Errorf("sign seal: %w", err)
	}

	sealJSON, err := seal.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal seal: %w", err)
	}

	// Step 3: Create gift wrap (kind 1059) — encrypt seal with ephemeral + recipient
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

	giftWrap := &event.E{
		Content:   []byte(wrapCiphertext),
		CreatedAt: randomizeTimestamp(now),
		Kind:      kindGiftWrap,
		Tags: tag.NewS(
			tag.NewFromAny("p", recipientPubHex),
		),
	}
	if err := giftWrap.Sign(ephSigner); err != nil {
		return nil, fmt.Errorf("sign gift wrap: %w", err)
	}

	log.D.F("created NIP-17 gift wrap for %s (ephemeral pubkey %s)",
		recipientPubHex, hex.Enc(ephSigner.Pub()))

	return giftWrap, nil
}

// randomizeTimestamp adds a random offset of +/- 2 days for NIP-59 privacy.
func randomizeTimestamp(base int64) int64 {
	const twoDays = 2 * 24 * 60 * 60
	offset := rand.Int63n(4*24*60*60) - twoDays
	return base + offset
}
