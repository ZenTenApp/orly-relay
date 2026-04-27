package marmot

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"git.smesh.lol/orly/pkg/nostr/crypto/keys"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
)

// MLS exporter parameters for kind 445 encryption per MIP-03.
var (
	exporterLabel   = "marmot"
	exporterContext = []byte("group-event")
)

const exporterLength uint16 = 32

// MessageToEvent creates a kind 445 event per MIP-03.
//
// The mlsCiphertext (output of group.CreateApplicationMessage) is encrypted
// with ChaCha20-Poly1305 using a key derived from MLS-Exporter("marmot",
// "group-event", 32). Content is base64(nonce[12] || ciphertext).
// The event is signed by a fresh ephemeral keypair.
func MessageToEvent(nostrGroupID, mlsCiphertext, exporterSecret []byte) (*event.E, error) {
	// ChaCha20-Poly1305 encrypt the MLS ciphertext
	aead, err := chacha20poly1305.New(exporterSecret)
	if err != nil {
		return nil, fmt.Errorf("create chacha20poly1305: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, mlsCiphertext, nil)

	// base64(nonce || ciphertext)
	raw := make([]byte, len(nonce)+len(ciphertext))
	copy(raw, nonce)
	copy(raw[len(nonce):], ciphertext)
	content := base64.StdEncoding.EncodeToString(raw)

	// Fresh ephemeral keypair per MIP-03
	ephSecret, err := keys.GenerateSecretKey()
	if err != nil {
		return nil, fmt.Errorf("generate ephemeral key: %w", err)
	}
	ephSigner, err := keys.SecretBytesToSigner(ephSecret)
	if err != nil {
		return nil, fmt.Errorf("create ephemeral signer: %w", err)
	}
	defer ephSigner.Zero()

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindGroupMessage
	ev.Content = []byte(content)
	ev.Tags = tag.NewS(
		tag.NewFromAny("h", hex.Enc(nostrGroupID)),
		tag.NewFromAny("encoding", "base64"),
	)
	if err := ev.Sign(ephSigner); err != nil {
		return nil, fmt.Errorf("sign group message: %w", err)
	}
	return ev, nil
}

// EventToMessage extracts the nostr_group_id and decrypts the MLS ciphertext
// from a kind 445 event using the exporter secret.
func EventToMessage(ev *event.E, exporterSecret []byte) (nostrGroupID, mlsCiphertext []byte, err error) {
	if ev.Kind != KindGroupMessage {
		return nil, nil, fmt.Errorf("expected kind %d, got %d", KindGroupMessage, ev.Kind)
	}

	hTag := ev.Tags.GetFirst([]byte("h"))
	if hTag == nil {
		return nil, nil, fmt.Errorf("missing 'h' tag (group ID)")
	}
	nostrGroupID, err = hex.Dec(string(hTag.Value()))
	if err != nil {
		return nil, nil, fmt.Errorf("decode group ID: %w", err)
	}

	// Decode base64 content
	raw, err := base64.StdEncoding.DecodeString(string(ev.Content))
	if err != nil {
		return nil, nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(raw) < chacha20poly1305.NonceSize {
		return nil, nil, fmt.Errorf("content too short: %d bytes", len(raw))
	}

	nonce := raw[:chacha20poly1305.NonceSize]
	ciphertext := raw[chacha20poly1305.NonceSize:]

	aead, err := chacha20poly1305.New(exporterSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("create chacha20poly1305: %w", err)
	}

	mlsCiphertext, err = aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("chacha20poly1305 decrypt: %w", err)
	}

	return nostrGroupID, mlsCiphertext, nil
}

// DeriveExporterSecret derives the ChaCha20-Poly1305 key for kind 445 events
// from an MLS group using MLS-Exporter("marmot", "group-event", 32).
func DeriveExporterSecret(group interface {
	DeriveExporter(label, context []byte, length uint16) ([]byte, error)
}) ([]byte, error) {
	return group.DeriveExporter([]byte(exporterLabel), exporterContext, exporterLength)
}

// ensure cipher.AEAD is satisfied at compile time
var _ cipher.AEAD = (cipher.AEAD)(nil)
