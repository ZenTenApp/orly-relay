package bridge

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"next.orly.dev/pkg/lol/log"
)

const defaultDMExpirationSeconds int64 = 30 * 24 * 60 * 60 // 30 days

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

// wrapGiftWrap creates NIP-17 gift-wrapped DMs (kind 14 → 13 → 1059).
// Returns two wraps: one for the recipient and one self-addressed copy for the sender.
// NIP-17 requires both so the sender can recover sent messages from relays.
func wrapGiftWrap(
	recipientPubHex string, content string, sign signer.I,
) (recipientWrap *event.E, senderWrap *event.E, err error) {
	recipientPub, err := hex.Dec(recipientPubHex)
	if err != nil {
		return nil, nil, fmt.Errorf("decode recipient pubkey: %w", err)
	}

	senderPub := sign.Pub()
	senderPubHex := hex.Enc(senderPub)
	now := time.Now().Unix()
	expiration := strconv.FormatInt(now+defaultDMExpirationSeconds, 10)

	// Step 1: Create inner DM (kind 14) — shared by both wraps
	inner := &event.E{
		Content:   []byte(content),
		CreatedAt: now,
		Kind:      kindDM,
		Tags: tag.NewS(
			tag.NewFromAny("p", recipientPubHex),
			tag.NewFromAny("expiration", expiration),
		),
	}
	if err := inner.Sign(sign); err != nil {
		return nil, nil, fmt.Errorf("sign inner DM: %w", err)
	}

	innerJSON, err := inner.MarshalJSON()
	if err != nil {
		return nil, nil, fmt.Errorf("marshal inner DM: %w", err)
	}

	// --- Recipient path ---

	// Seal encrypted with (sender_secret, recipient_pubkey)
	recipientSealKey, err := encryption.GenerateConversationKey(sign.Sec(), recipientPub)
	if err != nil {
		return nil, nil, fmt.Errorf("recipient seal ECDH: %w", err)
	}
	recipientSealCT, err := encryption.Encrypt(recipientSealKey, innerJSON, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("recipient seal encrypt: %w", err)
	}

	recipientSeal := &event.E{
		Content:   []byte(recipientSealCT),
		CreatedAt: randomizeTimestamp(now),
		Kind:      kindSeal,
		Tags:      tag.NewS(),
	}
	if err := recipientSeal.Sign(sign); err != nil {
		return nil, nil, fmt.Errorf("sign recipient seal: %w", err)
	}

	recipientSealJSON, err := recipientSeal.MarshalJSON()
	if err != nil {
		return nil, nil, fmt.Errorf("marshal recipient seal: %w", err)
	}

	// Gift wrap with ephemeral key, p-tag = recipient
	recipientWrap, err = wrapSealForTarget(recipientPub, recipientPubHex, recipientSealJSON, now, expiration)
	if err != nil {
		return nil, nil, fmt.Errorf("recipient wrap: %w", err)
	}

	// --- Sender (self) path ---

	// Seal encrypted with (sender_secret, sender_pubkey)
	senderSealKey, err := encryption.GenerateConversationKey(sign.Sec(), senderPub)
	if err != nil {
		return nil, nil, fmt.Errorf("sender seal ECDH: %w", err)
	}
	senderSealCT, err := encryption.Encrypt(senderSealKey, innerJSON, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("sender seal encrypt: %w", err)
	}

	senderSeal := &event.E{
		Content:   []byte(senderSealCT),
		CreatedAt: randomizeTimestamp(now),
		Kind:      kindSeal,
		Tags:      tag.NewS(),
	}
	if err := senderSeal.Sign(sign); err != nil {
		return nil, nil, fmt.Errorf("sign sender seal: %w", err)
	}

	senderSealJSON, err := senderSeal.MarshalJSON()
	if err != nil {
		return nil, nil, fmt.Errorf("marshal sender seal: %w", err)
	}

	// Gift wrap with different ephemeral key, p-tag = sender
	senderWrap, err = wrapSealForTarget(senderPub, senderPubHex, senderSealJSON, now, expiration)
	if err != nil {
		return nil, nil, fmt.Errorf("sender wrap: %w", err)
	}

	log.D.F("created NIP-17 gift wraps for %s (self-copy for %s)", recipientPubHex, senderPubHex)

	return recipientWrap, senderWrap, nil
}

// wrapSealForTarget creates a gift wrap (kind 1059) encrypting sealJSON for targetPub.
func wrapSealForTarget(targetPub []byte, targetPubHex string, sealJSON []byte, now int64, expiration string) (*event.E, error) {
	ephSecret, err := keys.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephSigner, err := keys.SecretBytesToSigner(ephSecret)
	if err != nil {
		return nil, fmt.Errorf("create ephemeral signer: %w", err)
	}
	defer ephSigner.Zero()

	wrapConvKey, err := encryption.GenerateConversationKey(ephSecret, targetPub)
	if err != nil {
		return nil, fmt.Errorf("gift wrap ECDH: %w", err)
	}
	wrapCiphertext, err := encryption.Encrypt(wrapConvKey, sealJSON, nil)
	if err != nil {
		return nil, fmt.Errorf("gift wrap encrypt: %w", err)
	}

	gwTags := tag.NewS(
		tag.NewFromAny("p", targetPubHex),
	)
	if expiration != "" {
		*gwTags = append(*gwTags, tag.NewFromAny("expiration", expiration))
	}
	gw := &event.E{
		Content:   []byte(wrapCiphertext),
		CreatedAt: randomizeTimestamp(now),
		Kind:      kindGiftWrap,
		Tags:      gwTags,
	}
	if err := gw.Sign(ephSigner); err != nil {
		return nil, fmt.Errorf("sign gift wrap: %w", err)
	}

	return gw, nil
}

// randomizeTimestamp subtracts a random offset of 0–2 days for NIP-59 privacy.
// Never produces future timestamps — relays reject those.
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
	offset := n % twoDays
	return base - offset
}
