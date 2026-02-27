package marmot

import (
	"context"
	"sync"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRelay is a test relay that routes events between connected clients.
// It buffers all published events and replays matching ones to new subscribers.
type mockRelay struct {
	mu          sync.Mutex
	events      []*event.E
	subscribers []chan *event.E
}

func newMockRelay() *mockRelay {
	return &mockRelay{}
}

func (r *mockRelay) Publish(ctx context.Context, ev *event.E) error {
	r.mu.Lock()
	r.events = append(r.events, ev.Clone())
	subs := make([]chan *event.E, len(r.subscribers))
	copy(subs, r.subscribers)
	r.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- ev.Clone():
		default:
		}
	}
	return nil
}

func (r *mockRelay) Subscribe(ctx context.Context, ff *filter.S) (EventStream, error) {
	ch := make(chan *event.E, 64)

	r.mu.Lock()
	// Replay existing events that match the filter
	for _, ev := range r.events {
		if ff.Match(ev) {
			select {
			case ch <- ev.Clone():
			default:
			}
		}
	}
	r.subscribers = append(r.subscribers, ch)
	r.mu.Unlock()

	return &mockStream{ch: ch}, nil
}

type mockStream struct {
	ch   chan *event.E
	once sync.Once
}

func (s *mockStream) Events() <-chan *event.E { return s.ch }
func (s *mockStream) Close() {
	s.once.Do(func() {
		// Don't close — just drain. Closing a channel that's being
		// written to by Publish would panic.
	})
}

func generateSigner(t *testing.T) *p8k.Signer {
	t.Helper()
	s, err := p8k.New()
	require.NoError(t, err)
	require.NoError(t, s.Generate())
	return s
}

func TestDMGroupID(t *testing.T) {
	pubA := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa1")
	pubB := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb1")

	// Same result regardless of order
	id1 := DMGroupID(pubA, pubB)
	id2 := DMGroupID(pubB, pubA)
	assert.Equal(t, id1, id2, "group ID should be order-independent")
	assert.Len(t, id1, 32, "group ID should be 32 bytes (SHA256)")
}

func TestKeyPackageRoundTrip(t *testing.T) {
	sign := generateSigner(t)

	kpp, err := GenerateKeyPackage(sign)
	require.NoError(t, err)
	require.NotNil(t, kpp)

	ev, err := KeyPackageToEvent(kpp, sign)
	require.NoError(t, err)
	assert.Equal(t, KindKeyPackage, ev.Kind)

	kp, err := EventToKeyPackage(ev)
	require.NoError(t, err)
	require.NotNil(t, kp)

	// The key package bytes should round-trip
	assert.Equal(t, kpp.Public.Bytes(), kp.Bytes())
}

func TestGroupCreateJoinEncryptDecrypt(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(alice)
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(bob)
	require.NoError(t, err)

	// Alice creates a group and invites Bob
	gs, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub())
	require.NoError(t, err)
	require.NotNil(t, gs)
	require.NotNil(t, welcome)

	// Bob joins the group from the welcome
	bobGS, err := JoinDMGroup(welcome, bobKPP, alice.Pub())
	require.NoError(t, err)
	require.NotNil(t, bobGS)

	// Alice sends a message
	plaintext := []byte("hello from alice")
	ciphertext, err := gs.Encrypt(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	// Bob decrypts
	decrypted, err := bobGS.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Bob sends a reply
	reply := []byte("hello from bob")
	replyCT, err := bobGS.Encrypt(reply)
	require.NoError(t, err)

	// Alice decrypts
	decryptedReply, err := gs.Decrypt(replyCT)
	require.NoError(t, err)
	assert.Equal(t, reply, decryptedReply)
}

func TestWelcomeGiftWrapRoundTrip(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(alice)
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(bob)
	require.NoError(t, err)

	_, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub())
	require.NoError(t, err)

	// Alice wraps the welcome for Bob
	wrapEv, err := WelcomeToGiftWrap(welcome, bob.Pub(), alice)
	require.NoError(t, err)
	assert.Equal(t, KindGiftWrap, wrapEv.Kind)

	// Bob unwraps
	unwrapped, err := UnwrapWelcome(wrapEv, bob)
	require.NoError(t, err)
	require.NotNil(t, unwrapped)

	// Bob should be able to join the group from the unwrapped welcome
	bobGS, err := JoinDMGroup(unwrapped, bobKPP, alice.Pub())
	require.NoError(t, err)
	require.NotNil(t, bobGS)
}

func TestMessageEventRoundTrip(t *testing.T) {
	sign := generateSigner(t)
	groupID := []byte("test-group-id-32-bytes-long!!!!!")
	ciphertext := []byte("encrypted-payload-here")

	ev, err := MessageToEvent(groupID, ciphertext, sign)
	require.NoError(t, err)
	assert.Equal(t, KindGroupMessage, ev.Kind)

	gid, ct, err := EventToMessage(ev)
	require.NoError(t, err)
	assert.Equal(t, groupID, gid)
	assert.Equal(t, ciphertext, ct)
}

func TestGroupStore(t *testing.T) {
	store := NewMemoryGroupStore()

	groupID := []byte("test-group-id")
	state := []byte(`{"test": true}`)

	// Save
	require.NoError(t, store.SaveGroup(groupID, state))

	// Load
	loaded, err := store.LoadGroup(groupID)
	require.NoError(t, err)
	assert.Equal(t, state, loaded)

	// List
	ids, err := store.ListGroups()
	require.NoError(t, err)
	assert.Len(t, ids, 1)

	// Delete
	require.NoError(t, store.DeleteGroup(groupID))
	_, err = store.LoadGroup(groupID)
	assert.Error(t, err)
}

func TestFileGroupStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileGroupStore(dir)
	require.NoError(t, err)

	groupID := []byte{0xde, 0xad, 0xbe, 0xef}
	state := []byte(`{"group": "data"}`)

	require.NoError(t, store.SaveGroup(groupID, state))

	loaded, err := store.LoadGroup(groupID)
	require.NoError(t, err)
	assert.Equal(t, state, loaded)

	ids, err := store.ListGroups()
	require.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, groupID, ids[0])
}

func TestClientSendReceiveDM(t *testing.T) {
	relay := newMockRelay()
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceStore := NewMemoryGroupStore()
	bobStore := NewMemoryGroupStore()

	aliceClient, err := NewClient(alice, aliceStore, relay)
	require.NoError(t, err)
	bobClient, err := NewClient(bob, bobStore, relay)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Bob publishes his key package so Alice can find it
	require.NoError(t, bobClient.PublishKeyPackage(ctx))

	// Set up Bob's DM handler
	received := make(chan []byte, 1)
	bobClient.OnDM(func(senderPub []byte, plaintext []byte) {
		received <- plaintext
	})

	// Start Bob listening for events
	go func() {
		stream, err := relay.Subscribe(ctx, bobClient.SubscriptionFilter())
		if err != nil {
			return
		}
		defer stream.Close()
		for ev := range stream.Events() {
			_ = bobClient.HandleEvent(ctx, ev)
		}
	}()

	// Give Bob a moment to subscribe
	time.Sleep(50 * time.Millisecond)

	// Alice sends a DM to Bob
	err = aliceClient.SendDM(ctx, bob.Pub(), []byte("hello bob"))
	require.NoError(t, err)

	// Wait for Bob to receive and decrypt
	select {
	case msg := <-received:
		assert.Equal(t, []byte("hello bob"), msg)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for DM")
	}
}
