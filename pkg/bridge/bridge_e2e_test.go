package bridge

import (
	"context"
	"strings"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/protocol/marmot"
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

// TestE2E_RouterDispatch verifies that the router correctly dispatches
// subscribe, status, outbound email, and help messages.
func TestE2E_RouterDispatch(t *testing.T) {
	sink := newMockDMSink()
	store := NewMemorySubscriptionStore()
	subHandler := NewSubscriptionHandler(store, nil, sink.send, 2100, nil, 0, "test.example.com")
	router := NewRouter(subHandler, nil, sink.send)

	ctx := context.Background()
	alice := "aaaa1111"

	// Subscribe command - free subscriptions are instant
	router.RouteDM(ctx, alice, "subscribe")
	waitForMessages(t, sink, alice, 1)
	msgs := sink.get(alice)
	if !strings.Contains(msgs[0], "Subscription active") {
		t.Errorf("subscribe response: want 'Subscription active', got: %s", msgs[0])
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

// TestE2E_MLSKeyPackageEvent verifies that the bridge can produce a valid kind
// 443 key package event and a kind 10051 relay list event for broadcasting.
func TestE2E_MLSKeyPackageEvent(t *testing.T) {
	bridgeSign := newTestSigner(t)
	store, err := marmot.NewFileGroupStore(t.TempDir())
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	client, err := marmot.NewClient(&marmot.LocalCrypto{Sign: bridgeSign}, store, &nullRelay{})
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
// and unwrapped through the marmot SDK.
func TestE2E_MLSWelcomeRoundTrip(t *testing.T) {
	alice := newTestSigner(t)
	bob := newTestSigner(t)

	aliceKPP, err := marmot.GenerateKeyPackage(&marmot.LocalCrypto{Sign: alice})
	if err != nil {
		t.Fatalf("alice key package: %v", err)
	}
	bobKPP, err := marmot.GenerateKeyPackage(&marmot.LocalCrypto{Sign: bob})
	if err != nil {
		t.Fatalf("bob key package: %v", err)
	}

	_, welcome, _, err := marmot.CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	if err != nil {
		t.Fatalf("create DM group: %v", err)
	}

	wrapEv, err := marmot.WelcomeToGiftWrap(welcome, bob.Pub(), &marmot.LocalCrypto{Sign: alice}, nil, nil)
	if err != nil {
		t.Fatalf("WelcomeToGiftWrap: %v", err)
	}

	if wrapEv.Kind != 1059 {
		t.Errorf("wrap kind: want 1059, got %d", wrapEv.Kind)
	}
	pTag := findTag(wrapEv, "p")
	if pTag != hex.Enc(bob.Pub()) {
		t.Errorf("p tag: want bob's pubkey, got %s", pTag)
	}

	unwrapped, err := marmot.UnwrapWelcome(wrapEv, &marmot.LocalCrypto{Sign: bob})
	if err != nil {
		t.Fatalf("UnwrapWelcome: %v", err)
	}

	if hex.Enc(unwrapped.SenderPub) != hex.Enc(alice.Pub()) {
		t.Errorf("sender: want alice, got %s", hex.Enc(unwrapped.SenderPub))
	}

	gs, err := marmot.JoinDMGroup(unwrapped.Welcome, bobKPP, alice.Pub())
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
	if gs == nil {
		t.Fatal("group state is nil after join")
	}
}

// TestE2E_MLSGroupMessageRoundTrip tests that a message encrypted through MLS
// can be wrapped in a kind 445 event and decrypted by the recipient.
func TestE2E_MLSGroupMessageRoundTrip(t *testing.T) {
	alice := newTestSigner(t)
	bob := newTestSigner(t)

	aliceKPP, err := marmot.GenerateKeyPackage(&marmot.LocalCrypto{Sign: alice})
	if err != nil {
		t.Fatalf("alice KPP: %v", err)
	}
	bobKPP, err := marmot.GenerateKeyPackage(&marmot.LocalCrypto{Sign: bob})
	if err != nil {
		t.Fatalf("bob KPP: %v", err)
	}

	aliceGS, welcome, _, err := marmot.CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	if err != nil {
		t.Fatalf("create group: %v", err)
	}

	bobGS, err := marmot.JoinDMGroup(welcome, bobKPP, alice.Pub())
	if err != nil {
		t.Fatalf("join group: %v", err)
	}
	bobGS.GroupID = aliceGS.GroupID

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

	bobExporter, err := bobGS.DeriveExporterSecret()
	if err != nil {
		t.Fatalf("bob exporter secret: %v", err)
	}

	_, mlsCiphertext, err := marmot.EventToMessage(ev, bobExporter)
	if err != nil {
		t.Fatalf("EventToMessage: %v", err)
	}

	decrypted, _, err := bobGS.Decrypt(mlsCiphertext)
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

func waitForMessages(t *testing.T, sink *mockDMSink, pubkey string, n int) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if len(sink.get(pubkey)) >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
}
