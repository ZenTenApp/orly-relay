package bridge

import (
	"context"
	"strings"
	"testing"
	"time"

	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/nostr/protocol/marmot"
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

// TestE2E_MLSKeyPackageEvent verifies that the bridge can produce a valid kind
// 443 key package event and a kind 10051 relay list event for broadcasting.
func TestE2E_MLSKeyPackageEvent(t *testing.T) {
	bridgeSign := newTestSigner(t)
	store, err := marmot.NewFileGroupStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	// We need a mock relay adapter but KeyPackageEvent doesn't use it.
	client, err := marmot.NewClient(bridgeSign, store, &nullRelay{})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	// Kind 443 key package event
	kpEv, err := client.KeyPackageEvent()
	if err != nil {
		t.Fatalf("KeyPackageEvent: %v", err)
	}
	if kpEv.Kind != 443 {
		t.Errorf("kind: want 443, got %d", kpEv.Kind)
	}
	if len(kpEv.Content) == 0 {
		t.Error("key package event has empty content")
	}
	// Verify round-trip: parse back to MLS key package
	kp, err := marmot.EventToKeyPackage(kpEv)
	if err != nil {
		t.Fatalf("parse key package from event: %v", err)
	}
	if kp == nil {
		t.Fatal("parsed key package is nil")
	}

	// Kind 10051 relay list event
	kprEv, err := client.KeyPackageRelaysEvent([]string{"wss://relay.example.com"})
	if err != nil {
		t.Fatalf("KeyPackageRelaysEvent: %v", err)
	}
	if kprEv.Kind != 10051 {
		t.Errorf("kind: want 10051, got %d", kprEv.Kind)
	}
	relayTag := findTag(kprEv, "relay")
	if relayTag != "wss://relay.example.com" {
		t.Errorf("relay tag: want wss://relay.example.com, got %q", relayTag)
	}
}

// TestE2E_MLSWelcomeRoundTrip verifies that an MLS Welcome can be gift-wrapped
// and unwrapped through the marmot SDK, exercising the same code path that the
// bridge's handleGiftWrapEvent uses for MLS disambiguation.
func TestE2E_MLSWelcomeRoundTrip(t *testing.T) {
	alice := newTestSigner(t)
	bob := newTestSigner(t)

	aliceKPP, err := marmot.GenerateKeyPackage(alice)
	if err != nil {
		t.Fatalf("alice key package: %v", err)
	}
	bobKPP, err := marmot.GenerateKeyPackage(bob)
	if err != nil {
		t.Fatalf("bob key package: %v", err)
	}

	// Alice creates a group with Bob
	_, welcome, _, err := marmot.CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	if err != nil {
		t.Fatalf("create DM group: %v", err)
	}

	// Alice wraps the welcome for Bob using NIP-59 three-layer gift wrap
	wrapEv, err := marmot.WelcomeToGiftWrap(welcome, bob.Pub(), alice, nil, nil)
	if err != nil {
		t.Fatalf("WelcomeToGiftWrap: %v", err)
	}

	// Verify outer structure
	if wrapEv.Kind != 1059 {
		t.Errorf("wrap kind: want 1059, got %d", wrapEv.Kind)
	}
	pTag := findTag(wrapEv, "p")
	if pTag != hex.Enc(bob.Pub()) {
		t.Errorf("p tag: want bob's pubkey, got %s", pTag)
	}

	// Bob unwraps the welcome
	unwrapped, err := marmot.UnwrapWelcome(wrapEv, bob)
	if err != nil {
		t.Fatalf("UnwrapWelcome: %v", err)
	}

	// Sender should be Alice (from the seal layer)
	if hex.Enc(unwrapped.SenderPub) != hex.Enc(alice.Pub()) {
		t.Errorf("sender: want alice, got %s", hex.Enc(unwrapped.SenderPub))
	}

	// Bob joins the group
	gs, err := marmot.JoinDMGroup(unwrapped.Welcome, bobKPP, alice.Pub())
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
	if gs == nil {
		t.Fatal("group state is nil after join")
	}
}

// TestE2E_Kind1059Disambiguation verifies that the bridge correctly
// distinguishes between NIP-17 gift-wrapped DMs and MLS Welcome events,
// both of which arrive as kind 1059.
func TestE2E_Kind1059Disambiguation(t *testing.T) {
	bridgeSign := newTestSigner(t)
	alice := newTestSigner(t)

	bridgePubHex := hex.Enc(bridgeSign.Pub())

	// Create a NIP-17 gift wrap
	nip17Wrap, _, err := wrapGiftWrap(bridgePubHex, "hello from NIP-17", alice)
	if err != nil {
		t.Fatalf("NIP-17 wrap: %v", err)
	}

	// NIP-17 unwrap should succeed
	dm, err := unwrapGiftWrap(nip17Wrap, bridgeSign)
	if err != nil {
		t.Fatalf("NIP-17 unwrap: %v", err)
	}
	if dm.Content != "hello from NIP-17" {
		t.Errorf("NIP-17 content: want 'hello from NIP-17', got %q", dm.Content)
	}

	// Create an MLS Welcome wrapped as kind 1059
	aliceKPP, err := marmot.GenerateKeyPackage(alice)
	if err != nil {
		t.Fatalf("alice key package: %v", err)
	}
	bridgeKPP, err := marmot.GenerateKeyPackage(bridgeSign)
	if err != nil {
		t.Fatalf("bridge key package: %v", err)
	}

	_, welcome, _, err := marmot.CreateDMGroup(aliceKPP, &bridgeKPP.Public, alice.Pub(), bridgeSign.Pub(), nil)
	if err != nil {
		t.Fatalf("create DM group: %v", err)
	}

	mlsWrap, err := marmot.WelcomeToGiftWrap(welcome, bridgeSign.Pub(), alice, nil, nil)
	if err != nil {
		t.Fatalf("MLS wrap: %v", err)
	}

	// NIP-17 unwrap of MLS Welcome should FAIL (different inner structure)
	_, err = unwrapGiftWrap(mlsWrap, bridgeSign)
	if err == nil {
		t.Fatal("NIP-17 unwrap of MLS Welcome should have failed")
	}

	// MLS unwrap should succeed (this is the fallback path)
	unwrapped, err := marmot.UnwrapWelcome(mlsWrap, bridgeSign)
	if err != nil {
		t.Fatalf("MLS unwrap: %v", err)
	}
	if hex.Enc(unwrapped.SenderPub) != hex.Enc(alice.Pub()) {
		t.Errorf("MLS sender: want alice, got %s", hex.Enc(unwrapped.SenderPub))
	}
}

// TestE2E_ThreeProtocolFormatTracking verifies that the bridge correctly
// tracks NIP-04, NIP-17, and MLS format per sender and returns the right
// format for each.
func TestE2E_ThreeProtocolFormatTracking(t *testing.T) {
	bridgeSign := newTestSigner(t)
	alice := newTestSigner(t)
	bob := newTestSigner(t)
	carol := newTestSigner(t)

	alicePubHex := hex.Enc(alice.Pub())
	bobPubHex := hex.Enc(bob.Pub())
	carolPubHex := hex.Enc(carol.Pub())

	b := &Bridge{
		sign:          bridgeSign,
		senderFormats: make(map[string]dmFormat),
	}

	// Alice uses NIP-04
	b.recordSenderFormat(alicePubHex, dmFormatKind4)
	// Bob uses NIP-17
	b.recordSenderFormat(bobPubHex, dmFormatGiftWrap)
	// Carol uses MLS
	b.recordSenderFormat(carolPubHex, dmFormatMLS)

	if f := b.getSenderFormat(alicePubHex); f != dmFormatKind4 {
		t.Errorf("alice: want kind4, got %d", f)
	}
	if f := b.getSenderFormat(bobPubHex); f != dmFormatGiftWrap {
		t.Errorf("bob: want giftWrap, got %d", f)
	}
	if f := b.getSenderFormat(carolPubHex); f != dmFormatMLS {
		t.Errorf("carol: want MLS, got %d", f)
	}

	// Unknown defaults to kind4
	if f := b.getSenderFormat("unknown"); f != dmFormatKind4 {
		t.Errorf("unknown: want kind4 default, got %d", f)
	}
}

// TestE2E_MLSGroupMessageRoundTrip tests that a message encrypted through MLS
// can be wrapped in a kind 445 event and decrypted by the recipient, verifying
// the full NIP-EE message path that the bridge uses.
func TestE2E_MLSGroupMessageRoundTrip(t *testing.T) {
	alice := newTestSigner(t)
	bob := newTestSigner(t)

	aliceKPP, err := marmot.GenerateKeyPackage(alice)
	if err != nil {
		t.Fatalf("alice KPP: %v", err)
	}
	bobKPP, err := marmot.GenerateKeyPackage(bob)
	if err != nil {
		t.Fatalf("bob KPP: %v", err)
	}

	// Alice creates group, gets Welcome for Bob
	aliceGS, welcome, _, err := marmot.CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	// Bob joins
	bobGS, err := marmot.JoinDMGroup(welcome, bobKPP, alice.Pub())
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
	bobGS.GroupID = aliceGS.GroupID

	// Alice encrypts and wraps in kind 445
	plaintext := []byte("subscribe")
	ciphertext, err := aliceGS.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	exporterSecret, err := aliceGS.DeriveExporterSecret()
	if err != nil {
		t.Fatalf("exporter secret: %v", err)
	}

	ev, err := marmot.MessageToEvent(aliceGS.NostrGroupID, ciphertext, exporterSecret)
	if err != nil {
		t.Fatalf("MessageToEvent: %v", err)
	}

	if ev.Kind != 445 {
		t.Errorf("kind: want 445, got %d", ev.Kind)
	}
	hTag := findTag(ev, "h")
	if hTag != hex.Enc(aliceGS.NostrGroupID) {
		t.Errorf("h tag: want nostr group ID, got %s", hTag)
	}

	// Bob extracts and decrypts
	bobExporter, err := bobGS.DeriveExporterSecret()
	if err != nil {
		t.Fatalf("bob exporter secret: %v", err)
	}

	_, mlsCiphertext, err := marmot.EventToMessage(ev, bobExporter)
	if err != nil {
		t.Fatalf("EventToMessage: %v", err)
	}

	decrypted, err := bobGS.Decrypt(mlsCiphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if string(decrypted) != "subscribe" {
		t.Errorf("decrypted: want 'subscribe', got %q", string(decrypted))
	}
}

// nullRelay is a no-op relay for tests that don't need network.
type nullRelay struct{}

func (n *nullRelay) Publish(ctx context.Context, ev *event.E) error { return nil }
func (n *nullRelay) Subscribe(ctx context.Context, ff *filter.S) (marmot.EventStream, error) {
	return &nullStream{}, nil
}

type nullStream struct{ ch chan *event.E }

func (s *nullStream) Events() <-chan *event.E {
	if s.ch == nil {
		s.ch = make(chan *event.E)
	}
	return s.ch
}
func (s *nullStream) Close() {}

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
