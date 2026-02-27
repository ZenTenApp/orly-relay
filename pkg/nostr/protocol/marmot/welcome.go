package marmot

import (
	"fmt"
	"time"

	"git.mleku.dev/mleku/nostr/crypto/encryption"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
	"github.com/emersion/go-mls"
)

// WelcomeToGiftWrap creates a kind 1059 gift-wrapped event containing the MLS
// Welcome message. The welcome is NIP-44 encrypted to the recipient so only
// they can decrypt and join the group.
func WelcomeToGiftWrap(welcome *mls.Welcome, recipientPub []byte, sign signer.I) (*event.E, error) {
	welcomeBytes := welcome.Bytes()

	// NIP-44 encrypt the welcome to the recipient
	convKey, err := encryption.GenerateConversationKey(sign.Sec(), recipientPub)
	if err != nil {
		return nil, fmt.Errorf("generate conversation key: %w", err)
	}
	ciphertext, err := encryption.Encrypt(convKey, welcomeBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("encrypt welcome: %w", err)
	}

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = KindGiftWrap
	ev.Content = []byte(ciphertext)
	ev.Tags = tag.NewS(
		tag.NewFromAny("p", hex.Enc(recipientPub)),
	)
	if err := ev.Sign(sign); err != nil {
		return nil, fmt.Errorf("sign gift wrap: %w", err)
	}
	return ev, nil
}

// UnwrapWelcome decrypts a kind 1059 gift-wrapped event and extracts the MLS
// Welcome message.
func UnwrapWelcome(ev *event.E, sign signer.I) (*mls.Welcome, error) {
	if ev.Kind != KindGiftWrap {
		return nil, fmt.Errorf("expected kind %d, got %d", KindGiftWrap, ev.Kind)
	}

	// The sender's pubkey is in the event
	senderPub := ev.Pubkey

	// NIP-44 decrypt using our secret key and the sender's pubkey
	convKey, err := encryption.GenerateConversationKey(sign.Sec(), senderPub)
	if err != nil {
		return nil, fmt.Errorf("generate conversation key: %w", err)
	}
	plaintext, err := encryption.Decrypt(convKey, string(ev.Content))
	if err != nil {
		return nil, fmt.Errorf("decrypt welcome: %w", err)
	}

	welcome, err := mls.UnmarshalWelcome([]byte(plaintext))
	if err != nil {
		return nil, fmt.Errorf("unmarshal welcome: %w", err)
	}
	return welcome, nil
}
