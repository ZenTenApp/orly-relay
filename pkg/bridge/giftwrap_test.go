package bridge

import (
	"testing"

	"next.orly.dev/pkg/nostr/crypto/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGiftWrap_RoundTrip(t *testing.T) {
	// Create sender and recipient signers
	senderSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	sender, err := keys.SecretBytesToSigner(senderSK)
	require.NoError(t, err)

	recipientSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	recipient, err := keys.SecretBytesToSigner(recipientSK)
	require.NoError(t, err)

	recipientPubHex := hexEnc(recipient.Pub())
	senderPubHex := hexEnc(sender.Pub())

	// Wrap a message from sender to recipient
	content := "subscribe testuser"
	gw, err := wrapGiftWrap(recipientPubHex, content, sender)
	require.NoError(t, err)

	assert.Equal(t, uint16(1059), gw.Kind)
	assert.NotEmpty(t, gw.Content)
	assert.NotEmpty(t, gw.Sig)

	// The gift wrap pubkey should NOT be the sender's real pubkey (ephemeral key)
	assert.NotEqual(t, sender.Pub(), gw.Pubkey,
		"gift wrap should use ephemeral key, not sender's real key")

	// Unwrap as recipient
	dm, err := unwrapGiftWrap(gw, recipient)
	require.NoError(t, err)

	assert.Equal(t, senderPubHex, dm.SenderPubHex)
	assert.Equal(t, content, dm.Content)
}

func TestGiftWrap_WrongRecipient(t *testing.T) {
	senderSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	sender, err := keys.SecretBytesToSigner(senderSK)
	require.NoError(t, err)

	recipientSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	recipient, err := keys.SecretBytesToSigner(recipientSK)
	require.NoError(t, err)

	// Third party tries to unwrap
	thirdSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	third, err := keys.SecretBytesToSigner(thirdSK)
	require.NoError(t, err)

	recipientPubHex := hexEnc(recipient.Pub())

	gw, err := wrapGiftWrap(recipientPubHex, "secret message", sender)
	require.NoError(t, err)

	// Third party cannot decrypt
	_, err = unwrapGiftWrap(gw, third)
	assert.Error(t, err)
}

func TestGiftWrap_TimestampRandomized(t *testing.T) {
	senderSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	sender, err := keys.SecretBytesToSigner(senderSK)
	require.NoError(t, err)

	recipientSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	recipient, err := keys.SecretBytesToSigner(recipientSK)
	require.NoError(t, err)

	recipientPubHex := hexEnc(recipient.Pub())

	// Create multiple gift wraps and verify timestamps differ
	timestamps := make(map[int64]bool)
	for i := 0; i < 10; i++ {
		gw, err := wrapGiftWrap(recipientPubHex, "test", sender)
		require.NoError(t, err)
		timestamps[gw.CreatedAt] = true
	}

	// With random offsets of +/- 2 days, we should get multiple different timestamps
	assert.Greater(t, len(timestamps), 1,
		"gift wrap timestamps should be randomized")
}

func TestGiftWrap_LongContent(t *testing.T) {
	senderSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	sender, err := keys.SecretBytesToSigner(senderSK)
	require.NoError(t, err)

	recipientSK, err := keys.GenerateSecretKey()
	require.NoError(t, err)
	recipient, err := keys.SecretBytesToSigner(recipientSK)
	require.NoError(t, err)

	recipientPubHex := hexEnc(recipient.Pub())

	// Test with a long email-like content
	content := "To: alice@example.com\nSubject: Important\n\n" +
		"This is a much longer message body that tests whether the " +
		"gift wrap encryption handles larger payloads correctly. " +
		"It includes multiple paragraphs and special characters like " +
		"<html>, &amp;, \"quotes\", and unicode: 日本語テスト"

	gw, err := wrapGiftWrap(recipientPubHex, content, sender)
	require.NoError(t, err)

	dm, err := unwrapGiftWrap(gw, recipient)
	require.NoError(t, err)

	assert.Equal(t, content, dm.Content)
}

func TestSenderFormatTracking(t *testing.T) {
	b := &Bridge{
		senderFormats: make(map[string]dmFormat),
	}

	pubA := "aaaa"
	pubB := "bbbb"

	// Default for unknown sender is kind 4
	assert.Equal(t, dmFormatKind4, b.getSenderFormat(pubA))

	// Record sender A as gift wrap
	b.recordSenderFormat(pubA, dmFormatGiftWrap)
	assert.Equal(t, dmFormatGiftWrap, b.getSenderFormat(pubA))

	// Sender B still defaults to kind 4
	assert.Equal(t, dmFormatKind4, b.getSenderFormat(pubB))

	// Record sender B as kind 4
	b.recordSenderFormat(pubB, dmFormatKind4)
	assert.Equal(t, dmFormatKind4, b.getSenderFormat(pubB))

	// Sender A upgrades to gift wrap doesn't affect B
	assert.Equal(t, dmFormatGiftWrap, b.getSenderFormat(pubA))
}

// hexEnc is a test helper since we can't import the hex encoder easily.
func hexEnc(b []byte) string {
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}
