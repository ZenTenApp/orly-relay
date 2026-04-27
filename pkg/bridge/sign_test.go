package bridge

import (
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/eventenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/utils"
)

func TestSignAndVerifyDM(t *testing.T) {
	// Create a signer (like the bridge identity)
	sign, err := p8k.New()
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	if err := sign.Generate(); err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a recipient signer
	recipient, err := p8k.New()
	if err != nil {
		t.Fatalf("create recipient: %v", err)
	}
	if err := recipient.Generate(); err != nil {
		t.Fatalf("generate recipient key: %v", err)
	}

	recipientPubHex := hex.Enc(recipient.Pub())
	t.Logf("sender pubkey: %s", hex.Enc(sign.Pub()))
	t.Logf("recipient pubkey: %s", recipientPubHex)

	// Encrypt content like the bridge does
	conversationKey, err := encryption.GenerateConversationKey(
		sign.Sec(), recipient.Pub(),
	)
	if err != nil {
		t.Fatalf("generate conversation key: %v", err)
	}

	encrypted, err := encryption.Encrypt(conversationKey, []byte("Hello, this is a test DM!"), nil)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Build event exactly like makeSendDM does
	ev := &event.E{
		Content:   []byte(encrypted),
		CreatedAt: time.Now().Unix(),
		Kind:      4,
		Tags: tag.NewS(
			tag.NewFromAny("p", recipientPubHex),
		),
	}

	// Sign
	if err := ev.Sign(sign); err != nil {
		t.Fatalf("sign: %v", err)
	}

	t.Logf("event ID: %0x", ev.ID)
	t.Logf("event pubkey: %0x", ev.Pubkey)
	t.Logf("event sig len: %d", len(ev.Sig))

	// Verify directly (no round-trip)
	valid, err := ev.Verify()
	if err != nil {
		t.Fatalf("direct verify error: %v", err)
	}
	if !valid {
		t.Fatal("direct verify: signature is invalid!")
	}
	t.Log("direct verify: OK")

	// Now simulate what happens over WebSocket:
	// 1. Marshal to JSON (like ws.Client.Publish does)
	submission := eventenvelope.NewSubmissionWith(ev)
	jsonBytes := submission.Marshal(nil)
	t.Logf("marshaled JSON (%d bytes): %s", len(jsonBytes), string(jsonBytes[:min(200, len(jsonBytes))]))

	// 2. Unmarshal from JSON (like the relay does)
	env2 := eventenvelope.NewSubmission()
	_, err = env2.Unmarshal(jsonBytes)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// 3. Check canonical form matches
	canonical1 := ev.ToCanonical(nil)
	canonical2 := env2.E.ToCanonical(nil)
	t.Logf("canonical1: %s", string(canonical1))
	t.Logf("canonical2: %s", string(canonical2))

	if !utils.FastEqual(canonical1, canonical2) {
		t.Fatalf("canonical forms differ!\n  original:   %s\n  roundtrip:  %s", string(canonical1), string(canonical2))
	}
	t.Log("canonical forms match")

	// 4. Validate ID (like ValidateEventID does)
	calculatedID := env2.E.GetIDBytes()
	if !utils.FastEqual(calculatedID, env2.E.ID) {
		t.Fatalf("ID mismatch: event has %0x, computed %0x", env2.E.ID, calculatedID)
	}
	t.Log("ID validation: OK")

	// 5. Verify signature (like ValidateSignature does)
	valid2, err := env2.E.Verify()
	if err != nil {
		t.Fatalf("roundtrip verify error: %v", err)
	}
	if !valid2 {
		t.Fatal("roundtrip verify: signature is invalid!")
	}
	t.Log("roundtrip verify: OK")
}
