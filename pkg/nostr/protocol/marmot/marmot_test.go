package marmot

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-mls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// mockRelay is a test relay that routes events between connected clients.
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
	s.once.Do(func() {})
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

	id1 := DMGroupID(pubA, pubB)
	id2 := DMGroupID(pubB, pubA)
	assert.Equal(t, id1, id2, "group ID should be order-independent")
	assert.Len(t, id1, 32, "group ID should be 32 bytes (SHA256)")
}

func TestKeyPackageRoundTrip(t *testing.T) {
	sign := generateSigner(t)

	kpp, err := GenerateKeyPackage(&LocalCrypto{Sign: sign})
	require.NoError(t, err)
	require.NotNil(t, kpp)

	ev, err := KeyPackageToEvent(kpp, &LocalCrypto{Sign: sign}, []string{"wss://relay.example.com"})
	require.NoError(t, err)
	assert.Equal(t, KindKeyPackage, ev.Kind)

	kp, err := EventToKeyPackage(ev)
	require.NoError(t, err)
	require.NotNil(t, kp)

	assert.Equal(t, kpp.Public.RawBytes(), kp.RawBytes())
}

func TestGroupMarshalRoundTrip(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: alice})
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: bob})
	require.NoError(t, err)

	// Alice creates group
	gs, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)
	require.NotEmpty(t, gs.mlsBytes, "mlsBytes must be set after CreateDMGroup")

	// Bob joins
	bobGS, err := JoinDMGroup(welcome, bobKPP, alice.Pub())
	require.NoError(t, err)
	require.NotEmpty(t, bobGS.mlsBytes, "mlsBytes must be set after JoinDMGroup")

	// Send a message to advance epoch, then persist+restore Alice's group
	plaintext := []byte("test persistence")
	ct, err := gs.Encrypt(plaintext)
	require.NoError(t, err)

	// Re-marshal after encrypt (epoch may have advanced)
	aliceBytes, err := gs.group.Marshal()
	require.NoError(t, err)

	// Restore from bytes and verify decryption still works from Bob's side
	_, _, err = bobGS.Decrypt(ct)
	require.NoError(t, err)

	// Re-marshal Bob's state after decrypt
	bobBytes, err := bobGS.group.Marshal()
	require.NoError(t, err)

	// Restore both sides from serialized state
	restoredAlice, err := mls.UnmarshalGroup(aliceBytes)
	require.NoError(t, err)
	restoredBob, err := mls.UnmarshalGroup(bobBytes)
	require.NoError(t, err)

	// Verify restored groups can still encrypt/decrypt
	aliceGS2 := &GroupState{group: restoredAlice}
	bobGS2 := &GroupState{group: restoredBob}

	msg2 := []byte("after restore")
	ct2, err := aliceGS2.Encrypt(msg2)
	require.NoError(t, err)

	dec2, _, err := bobGS2.Decrypt(ct2)
	require.NoError(t, err)
	assert.Equal(t, msg2, dec2)

	// And the reverse direction
	msg3 := []byte("bob replies after restore")
	ct3, err := bobGS2.Encrypt(msg3)
	require.NoError(t, err)

	dec3, _, err := aliceGS2.Decrypt(ct3)
	require.NoError(t, err)
	assert.Equal(t, msg3, dec3)
}

func TestGroupCreateJoinEncryptDecrypt(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: alice})
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: bob})
	require.NoError(t, err)

	gs, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)
	require.NotNil(t, gs)
	require.NotNil(t, welcome)

	bobGS, err := JoinDMGroup(welcome, bobKPP, alice.Pub())
	require.NoError(t, err)
	require.NotNil(t, bobGS)

	plaintext := []byte("hello from alice")
	ciphertext, err := gs.Encrypt(plaintext)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	decrypted, _, err := bobGS.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	reply := []byte("hello from bob")
	replyCT, err := bobGS.Encrypt(reply)
	require.NoError(t, err)

	decryptedReply, _, err := gs.Decrypt(replyCT)
	require.NoError(t, err)
	assert.Equal(t, reply, decryptedReply)
}

// TestNIPEE_WelcomeKind444RoundTrip verifies the NIP-59 three-layer gift-wrap
// structure: kind 444 (Welcome) -> kind 13 (Seal) -> kind 1059 (GiftWrap).
func TestNIPEE_WelcomeKind444RoundTrip(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: alice})
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: bob})
	require.NoError(t, err)

	_, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)

	// Alice wraps the welcome for Bob
	wrapEv, err := WelcomeToGiftWrap(welcome, bob.Pub(), &LocalCrypto{Sign: alice}, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, KindGiftWrap, wrapEv.Kind)

	// Outer event should be signed by ephemeral key (not Alice's real key)
	assert.False(t, bytes.Equal(wrapEv.Pubkey, alice.Pub()),
		"gift wrap pubkey must be ephemeral, not sender's real key")

	// Bob unwraps and gets both the Welcome and Alice's real pubkey
	unwrapped, err := UnwrapWelcome(wrapEv, &LocalCrypto{Sign: bob})
	require.NoError(t, err)
	require.NotNil(t, unwrapped)
	require.NotNil(t, unwrapped.Welcome)

	// Sender pubkey should be Alice's real pubkey (from seal layer)
	assert.True(t, bytes.Equal(unwrapped.SenderPub, alice.Pub()),
		"sender pub from seal should be alice's real pubkey")

	// Bob should be able to join from the unwrapped welcome
	bobGS, err := JoinDMGroup(unwrapped.Welcome, bobKPP, alice.Pub())
	require.NoError(t, err)
	require.NotNil(t, bobGS)
}

// TestNIPEE_HTagRoundTrip verifies kind 445 events use "h" tag for group ID.
func TestNIPEE_HTagRoundTrip(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: alice})
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: bob})
	require.NoError(t, err)

	gs, _, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)

	plaintext := []byte("test message")
	ciphertext, err := gs.Encrypt(plaintext)
	require.NoError(t, err)

	exporterSecret, err := gs.DeriveExporterSecret()
	require.NoError(t, err)

	ev, err := MessageToEvent(gs.NostrGroupID, ciphertext, exporterSecret)
	require.NoError(t, err)
	assert.Equal(t, KindGroupMessage, ev.Kind)

	// Verify "h" tag is present with correct nostr group ID
	hTag := ev.Tags.GetFirst([]byte("h"))
	require.NotNil(t, hTag, "kind 445 must have 'h' tag")
	tagValue := string(hTag.Value())
	assert.Equal(t, hex.Enc(gs.NostrGroupID), tagValue)

	// Round-trip: extract and decrypt
	gid, ct, err := EventToMessage(ev, exporterSecret)
	require.NoError(t, err)
	assert.Equal(t, gs.NostrGroupID, gid)
	assert.Equal(t, ciphertext, ct)
}

// TestNIPEE_EphemeralSigning verifies kind 445 events use ephemeral keys.
func TestNIPEE_EphemeralSigning(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: alice})
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: bob})
	require.NoError(t, err)

	gs, _, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)

	ciphertext, err := gs.Encrypt([]byte("secret"))
	require.NoError(t, err)

	exporterSecret, err := gs.DeriveExporterSecret()
	require.NoError(t, err)

	ev1, err := MessageToEvent(gs.NostrGroupID, ciphertext, exporterSecret)
	require.NoError(t, err)

	ev2, err := MessageToEvent(gs.NostrGroupID, ciphertext, exporterSecret)
	require.NoError(t, err)

	// Neither event should be signed by Alice's real key
	assert.False(t, bytes.Equal(ev1.Pubkey, alice.Pub()),
		"kind 445 must not reveal sender identity")
	assert.False(t, bytes.Equal(ev2.Pubkey, alice.Pub()),
		"kind 445 must not reveal sender identity")

	// Each event should use a different ephemeral key
	assert.False(t, bytes.Equal(ev1.Pubkey, ev2.Pubkey),
		"each kind 445 should use a fresh ephemeral key")
}

// TestNIPEE_NIP44EncryptionWithExporter verifies kind 445 content is NIP-44
// encrypted with the exporter secret.
func TestNIPEE_NIP44EncryptionWithExporter(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: alice})
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(&LocalCrypto{Sign: bob})
	require.NoError(t, err)

	gs, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)

	bobGS, err := JoinDMGroup(welcome, bobKPP, alice.Pub())
	require.NoError(t, err)

	plaintext := []byte("encrypted with exporter secret")
	mlsCT, err := gs.Encrypt(plaintext)
	require.NoError(t, err)

	aliceExporter, err := gs.DeriveExporterSecret()
	require.NoError(t, err)

	ev, err := MessageToEvent(gs.NostrGroupID, mlsCT, aliceExporter)
	require.NoError(t, err)

	// Content should NOT be the raw MLS ciphertext (it's ChaCha20-Poly1305 wrapped)
	assert.NotEqual(t, mlsCT, ev.Content,
		"content must be encrypted, not raw MLS ciphertext")

	// Bob derives exporter secret from his side and decrypts
	bobExporter, err := bobGS.DeriveExporterSecret()
	require.NoError(t, err)
	assert.Equal(t, aliceExporter, bobExporter,
		"both sides should derive the same exporter secret")

	_, recoveredCT, err := EventToMessage(ev, bobExporter)
	require.NoError(t, err)
	assert.Equal(t, mlsCT, recoveredCT)

	// MLS decrypt to get original plaintext
	decrypted, _, err := bobGS.Decrypt(recoveredCT)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestGroupStore(t *testing.T) {
	store := NewMemoryGroupStore()

	groupID := []byte("test-group-id")
	state := []byte(`{"test": true}`)

	require.NoError(t, store.SaveGroup(groupID, state))

	loaded, err := store.LoadGroup(groupID)
	require.NoError(t, err)
	assert.Equal(t, state, loaded)

	ids, err := store.ListGroups()
	require.NoError(t, err)
	assert.Len(t, ids, 1)

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

// TestNIPEE_ClientE2E runs a full Alice/Bob exchange through the Client layer
// with NIP-EE compliance: ephemeral signing, NIP-44 encryption, "h" tags,
// and three-layer welcome wrapping.
func TestNIPEE_ClientE2E(t *testing.T) {
	relay := newMockRelay()
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceClient, err := NewClient(&LocalCrypto{Sign: alice}, NewMemoryGroupStore(), relay)
	require.NoError(t, err)
	bobClient, err := NewClient(&LocalCrypto{Sign: bob}, NewMemoryGroupStore(), relay)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Bob publishes his key package
	require.NoError(t, bobClient.PublishKeyPackage(ctx))

	// Set up Bob's DM handler
	received := make(chan struct {
		sender    []byte
		plaintext []byte
	}, 1)
	bobClient.OnDM(func(senderPub []byte, plaintext []byte) {
		received <- struct {
			sender    []byte
			plaintext []byte
		}{senderPub, plaintext}
	})

	// Bob listens for events
	go func() {
		stream, err := relay.Subscribe(ctx, bobClient.SubscriptionFilters())
		if err != nil {
			return
		}
		defer stream.Close()
		for {
			select {
			case ev := <-stream.Events():
				_ = bobClient.HandleEvent(ctx, ev)
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Alice sends a DM to Bob
	err = aliceClient.SendDM(ctx, bob.Pub(), []byte("hello bob via MLS"))
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, []byte("hello bob via MLS"), msg.plaintext)
		// Sender should be Alice's real pubkey (not ephemeral)
		assert.True(t, bytes.Equal(msg.sender, alice.Pub()),
			"sender pubkey should be Alice's real identity, got %s",
			hex.Enc(msg.sender))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for DM")
	}
}

// TestSubscriptionFilters verifies the filter structure.
func TestSubscriptionFilters(t *testing.T) {
	relay := newMockRelay()
	alice := generateSigner(t)

	client, err := NewClient(&LocalCrypto{Sign: alice}, NewMemoryGroupStore(), relay)
	require.NoError(t, err)

	// With no groups, should return one filter (welcome only)
	ff := client.SubscriptionFilters()
	require.NotNil(t, ff)
	assert.Len(t, *ff, 1, "should have welcome filter only when no groups")

	// Manually add a group to simulate an established conversation
	setReq := mcSetGroupReq{
		groupID: "test",
		gs: &GroupState{
			GroupID:      []byte("test-group-id-32-bytes-long!!!!!"),
			NostrGroupID: []byte("nostr-group-id-32-bytes-long!!!!"),
			PeerPub:      []byte("peer-pubkey-32-bytes-long!!!!!!!"),
		},
		resp: make(chan struct{}, 1),
	}
	client.setGroup <- setReq
	<-setReq.resp

	ff = client.SubscriptionFilters()
	require.NotNil(t, ff)
	assert.Len(t, *ff, 2, "should have welcome + group message filters")
}
