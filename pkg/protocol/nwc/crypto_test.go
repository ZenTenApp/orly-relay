package nwc_test

import (
	"encoding/json"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/protocol/nwc"
	"git.smesh.lol/orly/pkg/utils"
)

func TestNWCConversationKey(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	walletPubkey := "816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b"

	uri := "nostr+walletconnect://" + walletPubkey + "?relay=wss://relay.getalby.com/v1&secret=" + secret

	parts, err := nwc.ParseConnectionURI(uri)
	if err != nil {
		t.Fatal(err)
	}

	// Validate conversation key was generated
	convKey := parts.GetConversationKey()
	if len(convKey) == 0 {
		t.Fatal("conversation key should not be empty")
	}

	// Validate wallet public key
	walletKey := parts.GetWalletPublicKey()
	if len(walletKey) == 0 {
		t.Fatal("wallet public key should not be empty")
	}

	expected, err := hex.Dec(walletPubkey)
	if err != nil {
		t.Fatal(err)
	}

	if len(walletKey) != len(expected) {
		t.Fatal("wallet public key length mismatch")
	}

	for i := range walletKey {
		if walletKey[i] != expected[i] {
			t.Fatal("wallet public key mismatch")
		}
	}

	// Test passed
}

func TestNWCEncryptionDecryption(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	walletPubkey := "816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b"

	uri := "nostr+walletconnect://" + walletPubkey + "?relay=wss://relay.getalby.com/v1&secret=" + secret

	parts, err := nwc.ParseConnectionURI(uri)
	if err != nil {
		t.Fatal(err)
	}

	convKey := parts.GetConversationKey()
	testMessage := `{"method":"get_info","params":null}`

	// Test encryption
	encrypted, err := encryption.Encrypt(convKey, []byte(testMessage), nil)
	if err != nil {
		t.Fatalf("encryption failed: %v", err)
	}

	if len(encrypted) == 0 {
		t.Fatal("encrypted message should not be empty")
	}

	// Test decryption
	decrypted, err := encryption.Decrypt(convKey, encrypted)
	if err != nil {
		t.Fatalf("decryption failed: %v", err)
	}

	if decrypted != testMessage {
		t.Fatalf(
			"decrypted message mismatch: got %s, want %s", decrypted,
			testMessage,
		)
	}

	// Test passed
}

func TestNWCEventCreation(t *testing.T) {
	secretBytes, err := hex.Dec("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}

	clientKey := p8k.MustNew()
	if err := clientKey.InitSec(secretBytes); err != nil {
		t.Fatal(err)
	}

	walletPubkey, err := hex.Dec("816fd7f1d000ae81a3da251c91866fc47f4bcd6ce36921e6d46773c32f1d548b")
	if err != nil {
		t.Fatal(err)
	}

	convKey, err := encryption.GenerateConversationKey(
		clientKey.Sec(), walletPubkey,
	)
	if err != nil {
		t.Fatal(err)
	}

	request := map[string]any{"method": "get_info"}
	reqBytes, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := encryption.Encrypt(convKey, reqBytes, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create NWC event
	ev := &event.E{
		Content:   []byte(encrypted),
		CreatedAt: time.Now().Unix(),
		Kind:      23194,
		Tags: tag.NewS(
			tag.NewFromAny("encryption", "nip44_v2"),
			tag.NewFromAny("p", hex.Enc(walletPubkey)),
		),
	}

	if err := ev.Sign(clientKey); err != nil {
		t.Fatalf("event signing failed: %v", err)
	}

	// Validate event structure
	if len(ev.Content) == 0 {
		t.Fatal("event content should not be empty")
	}

	if len(ev.ID) == 0 {
		t.Fatal("event should have ID after signing")
	}

	if len(ev.Sig) == 0 {
		t.Fatal("event should have signature after signing")
	}

	// Validate tags
	hasEncryption := false
	hasP := false
	for i := 0; i < ev.Tags.Len(); i++ {
		tag := ev.Tags.GetTagElement(i)
		if tag.Len() >= 2 {
			if utils.FastEqual(
				tag.T[0], "encryption",
			) && utils.FastEqual(tag.T[1], "nip44_v2") {
				hasEncryption = true
			}
			if utils.FastEqual(
				tag.T[0], "p",
			) && utils.FastEqual(tag.T[1], hex.Enc(walletPubkey)) {
				hasP = true
			}
		}
	}

	if !hasEncryption {
		t.Fatal("event missing encryption tag")
	}

	if !hasP {
		t.Fatal("event missing p tag")
	}

	// Test passed
}
