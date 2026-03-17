package bridge

import (
	"context"
	"strings"
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

func newTestSigner(t *testing.T) *p8k.Signer {
	t.Helper()
	s, err := p8k.New()
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	if err := s.Generate(); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return s
}

// TestE2E_GiftWrapRoundTrip verifies that a user can send a NIP-17 gift-wrapped
// DM to the bridge, the bridge can unwrap it, and the bridge can reply with
// a properly formed gift wrap that the user can unwrap.
func TestE2E_GiftWrapRoundTrip(t *testing.T) {
	bridgeSign := newTestSigner(t)
	alice := newTestSigner(t)

	bridgePubHex := hex.Enc(bridgeSign.Pub())
	alicePubHex := hex.Enc(alice.Pub())

	t.Logf("bridge pubkey: %s", bridgePubHex)
	t.Logf("alice  pubkey: %s", alicePubHex)

	// === Alice sends a gift-wrapped DM to the bridge ===

	recipientWrap, selfWrap, err := wrapGiftWrap(bridgePubHex, "subscribe", alice)
	if err != nil {
		t.Fatalf("alice wrapGiftWrap: %v", err)
	}

	// Verify gift wrap structure
	if recipientWrap.Kind != kindGiftWrap {
		t.Errorf("recipient wrap kind: want %d, got %d", kindGiftWrap, recipientWrap.Kind)
	}
	if selfWrap.Kind != kindGiftWrap {
		t.Errorf("self wrap kind: want %d, got %d", kindGiftWrap, selfWrap.Kind)
	}

	// Verify p-tags
	recipientPTag := findTag(recipientWrap, "p")
	if recipientPTag != bridgePubHex {
		t.Errorf("recipient wrap p-tag: want bridge pubkey, got %s", recipientPTag)
	}
	selfPTag := findTag(selfWrap, "p")
	if selfPTag != alicePubHex {
		t.Errorf("self wrap p-tag: want alice pubkey, got %s", selfPTag)
	}

	// Verify expiration tags are present
	recipientExp := findTag(recipientWrap, "expiration")
	if recipientExp == "" {
		t.Error("recipient wrap missing expiration tag")
	}
	selfExp := findTag(selfWrap, "expiration")
	if selfExp == "" {
		t.Error("self wrap missing expiration tag")
	}

	// Bridge unwraps Alice's gift wrap
	dm, err := unwrapGiftWrap(recipientWrap, bridgeSign)
	if err != nil {
		t.Fatalf("bridge unwrapGiftWrap: %v", err)
	}
	if dm.SenderPubHex != alicePubHex {
		t.Errorf("sender pubkey: want %s, got %s", alicePubHex, dm.SenderPubHex)
	}
	if dm.Content != "subscribe" {
		t.Errorf("content: want 'subscribe', got %q", dm.Content)
	}

	// Alice unwraps her self-copy
	selfDM, err := unwrapGiftWrap(selfWrap, alice)
	if err != nil {
		t.Fatalf("alice unwrap self-copy: %v", err)
	}
	if selfDM.Content != "subscribe" {
		t.Errorf("self-copy content: want 'subscribe', got %q", selfDM.Content)
	}

	// === Bridge replies to Alice ===

	bridgeReply := "Welcome to Marmot!"
	replyRecipientWrap, replySelfWrap, err := wrapGiftWrap(alicePubHex, bridgeReply, bridgeSign)
	if err != nil {
		t.Fatalf("bridge wrapGiftWrap reply: %v", err)
	}

	// Alice unwraps the bridge's reply
	aliceRecv, err := unwrapGiftWrap(replyRecipientWrap, alice)
	if err != nil {
		t.Fatalf("alice unwrapGiftWrap reply: %v", err)
	}
	if aliceRecv.SenderPubHex != bridgePubHex {
		t.Errorf("reply sender: want bridge, got %s", aliceRecv.SenderPubHex)
	}
	if aliceRecv.Content != bridgeReply {
		t.Errorf("reply content: want %q, got %q", bridgeReply, aliceRecv.Content)
	}

	// Bridge unwraps its own self-copy
	bridgeSelfCopy, err := unwrapGiftWrap(replySelfWrap, bridgeSign)
	if err != nil {
		t.Fatalf("bridge unwrap self-copy: %v", err)
	}
	if bridgeSelfCopy.Content != bridgeReply {
		t.Errorf("bridge self-copy content: want %q, got %q", bridgeReply, bridgeSelfCopy.Content)
	}
}

// TestE2E_Kind4RoundTrip verifies NIP-04 kind 4 encryption round-trip
// between a user and the bridge.
func TestE2E_Kind4RoundTrip(t *testing.T) {
	bridgeSign := newTestSigner(t)
	bob := newTestSigner(t)

	bridgePubHex := hex.Enc(bridgeSign.Pub())
	bobPubHex := hex.Enc(bob.Pub())

	// Bob encrypts a kind 4 DM to the bridge using NIP-04
	sharedKey, err := encryption.GenerateNip4Key(bob.Sec(), bridgeSign.Pub())
	if err != nil {
		t.Fatalf("NIP-04 key: %v", err)
	}

	plaintext := "status"
	encrypted, err := encryption.EncryptNip4([]byte(plaintext), sharedKey)
	if err != nil {
		t.Fatalf("NIP-04 encrypt: %v", err)
	}

	ev := &event.E{
		Content:   encrypted,
		CreatedAt: 1700000000,
		Kind:      4,
		Tags: tag.NewS(
			tag.NewFromAny("p", bridgePubHex),
		),
	}
	if err := ev.Sign(bob); err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Bridge decrypts it (NIP-04 path)
	bridgeSharedKey, err := encryption.GenerateNip4Key(bridgeSign.Sec(), ev.Pubkey[:])
	if err != nil {
		t.Fatalf("bridge NIP-04 key: %v", err)
	}

	decrypted, err := encryption.DecryptNip4(ev.Content, bridgeSharedKey)
	if err != nil {
		t.Fatalf("bridge NIP-04 decrypt: %v", err)
	}

	if string(decrypted) != plaintext {
		t.Errorf("decrypted: want %q, got %q", plaintext, string(decrypted))
	}

	// Bridge encrypts a reply back to Bob using NIP-04
	reply := "You are subscribed."
	replyEncrypted, err := encryption.EncryptNip4([]byte(reply), bridgeSharedKey)
	if err != nil {
		t.Fatalf("bridge encrypt reply: %v", err)
	}

	replyEv := &event.E{
		Content:   replyEncrypted,
		CreatedAt: 1700000001,
		Kind:      4,
		Tags: tag.NewS(
			tag.NewFromAny("p", bobPubHex),
		),
	}
	if err := replyEv.Sign(bridgeSign); err != nil {
		t.Fatalf("sign reply: %v", err)
	}

	// Bob decrypts the reply
	bobDecrypted, err := encryption.DecryptNip4(replyEv.Content, sharedKey)
	if err != nil {
		t.Fatalf("bob decrypt reply: %v", err)
	}
	if string(bobDecrypted) != reply {
		t.Errorf("bob decrypted: want %q, got %q", reply, string(bobDecrypted))
	}
}

// TestE2E_FormatTracking verifies that the bridge tracks sender DM format
// and replies in the same format.
func TestE2E_FormatTracking(t *testing.T) {
	sink := newMockDMSink()
	bridgeSign := newTestSigner(t)
	alice := newTestSigner(t)
	bob := newTestSigner(t)

	alicePubHex := hex.Enc(alice.Pub())
	bobPubHex := hex.Enc(bob.Pub())

	// Simulate format tracking
	b := &Bridge{
		sign:          bridgeSign,
		senderFormats: make(map[string]dmFormat),
	}
	_ = sink // sink used below for router test

	// Alice sends via gift wrap → bridge records gift wrap format
	b.recordSenderFormat(alicePubHex, dmFormatGiftWrap)
	if f := b.getSenderFormat(alicePubHex); f != dmFormatGiftWrap {
		t.Errorf("alice format: want giftWrap, got %d", f)
	}

	// Bob sends via kind 4 → bridge records kind 4 format
	b.recordSenderFormat(bobPubHex, dmFormatKind4)
	if f := b.getSenderFormat(bobPubHex); f != dmFormatKind4 {
		t.Errorf("bob format: want kind4, got %d", f)
	}

	// Unknown user defaults to kind 4
	if f := b.getSenderFormat("unknown_user"); f != dmFormatKind4 {
		t.Errorf("unknown format: want kind4, got %d", f)
	}
}

// TestE2E_RouterDispatch verifies that the router correctly dispatches
// subscribe, status, outbound email, and help messages.
func TestE2E_RouterDispatch(t *testing.T) {
	sink := newMockDMSink()
	store := NewMemorySubscriptionStore()
	subHandler := NewSubscriptionHandler(store, nil, sink.send, 2100, nil, 0)
	router := NewRouter(subHandler, nil, sink.send)

	ctx := context.Background()
	alice := "aaaa1111"

	// Subscribe command
	router.RouteDM(ctx, alice, "subscribe")
	// Wait for goroutine
	waitForMessages(t, sink, alice, 1)
	msgs := sink.get(alice)
	if !strings.Contains(msgs[0], "not available") {
		t.Errorf("subscribe response: want 'not available', got: %s", msgs[0])
	}

	// Status command
	router.RouteDM(ctx, alice, "status")
	waitForMessages(t, sink, alice, 2)
	msgs = sink.get(alice)
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}

	// Outbound email (no processor configured)
	bob := "bbbb2222"
	router.RouteDM(ctx, bob, "To: alice@example.com\nSubject: Test\n\nHello")
	waitForMessages(t, sink, bob, 1)
	msgs = sink.get(bob)
	if !strings.Contains(msgs[0], "not configured") {
		t.Errorf("outbound response: want 'not configured', got: %s", msgs[0])
	}

	// Unrecognized → help
	carol := "cccc3333"
	router.RouteDM(ctx, carol, "hey what's up")
	waitForMessages(t, sink, carol, 1)
	msgs = sink.get(carol)
	if !strings.Contains(msgs[0], "Marmot Email Bridge") {
		t.Errorf("help response missing, got: %s", msgs[0])
	}
}

// TestE2E_TwoUserConversation simulates Alice and Bob both sending DMs to the
// bridge and receiving replies. Verifies isolation between conversations.
func TestE2E_TwoUserConversation(t *testing.T) {
	bridgeSign := newTestSigner(t)
	alice := newTestSigner(t)
	bob := newTestSigner(t)

	bridgePubHex := hex.Enc(bridgeSign.Pub())
	alicePubHex := hex.Enc(alice.Pub())
	bobPubHex := hex.Enc(bob.Pub())

	// Alice sends "subscribe" to bridge via NIP-17
	aliceWrap, _, err := wrapGiftWrap(bridgePubHex, "subscribe alice", alice)
	if err != nil {
		t.Fatalf("alice wrap: %v", err)
	}
	aliceDM, err := unwrapGiftWrap(aliceWrap, bridgeSign)
	if err != nil {
		t.Fatalf("bridge unwrap alice: %v", err)
	}
	if aliceDM.SenderPubHex != alicePubHex {
		t.Errorf("alice sender: want %s, got %s", alicePubHex, aliceDM.SenderPubHex)
	}
	if aliceDM.Content != "subscribe alice" {
		t.Errorf("alice content: want 'subscribe alice', got %q", aliceDM.Content)
	}

	// Bob sends "subscribe bob" to bridge via NIP-17
	bobWrap, _, err := wrapGiftWrap(bridgePubHex, "subscribe bob", bob)
	if err != nil {
		t.Fatalf("bob wrap: %v", err)
	}
	bobDM, err := unwrapGiftWrap(bobWrap, bridgeSign)
	if err != nil {
		t.Fatalf("bridge unwrap bob: %v", err)
	}
	if bobDM.SenderPubHex != bobPubHex {
		t.Errorf("bob sender: want %s, got %s", bobPubHex, bobDM.SenderPubHex)
	}

	// Bridge replies to Alice
	replyToAlice, _, err := wrapGiftWrap(alicePubHex, "subscribed as alice", bridgeSign)
	if err != nil {
		t.Fatalf("bridge reply alice: %v", err)
	}
	aliceRecv, err := unwrapGiftWrap(replyToAlice, alice)
	if err != nil {
		t.Fatalf("alice unwrap bridge reply: %v", err)
	}
	if aliceRecv.Content != "subscribed as alice" {
		t.Errorf("alice recv: want 'subscribed as alice', got %q", aliceRecv.Content)
	}

	// Bridge replies to Bob
	replyToBob, _, err := wrapGiftWrap(bobPubHex, "subscribed as bob", bridgeSign)
	if err != nil {
		t.Fatalf("bridge reply bob: %v", err)
	}
	bobRecv, err := unwrapGiftWrap(replyToBob, bob)
	if err != nil {
		t.Fatalf("bob unwrap bridge reply: %v", err)
	}
	if bobRecv.Content != "subscribed as bob" {
		t.Errorf("bob recv: want 'subscribed as bob', got %q", bobRecv.Content)
	}

	// Cross-check: Bob CANNOT unwrap Alice's message
	_, err = unwrapGiftWrap(replyToAlice, bob)
	if err == nil {
		t.Error("bob should NOT be able to unwrap alice's gift wrap")
	}

	// Cross-check: Alice CANNOT unwrap Bob's message
	_, err = unwrapGiftWrap(replyToBob, alice)
	if err == nil {
		t.Error("alice should NOT be able to unwrap bob's gift wrap")
	}
}

// --- helpers ---

// findTag returns the first value of a tag with the given name, or "".
func findTag(ev *event.E, name string) string {
	if ev.Tags == nil {
		return ""
	}
	for _, tg := range *ev.Tags {
		if len(tg.T) >= 2 && string(tg.T[0]) == name {
			return string(tg.T[1])
		}
	}
	return ""
}

// waitForMessages busy-waits until the mock sink has at least n messages for pubkey.
func waitForMessages(t *testing.T, sink *mockDMSink, pubkey string, n int) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if len(sink.get(pubkey)) >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
}
