// Regression test: a live subscription must enforce party-involvement access
// control on privileged kinds (DMs). Subscription structs are registered with
// the publisher via *W; the subscription's AuthRequired flag (computed by
// Listener.subscriptionRequiresAuth) decides whether privileged kinds (DMs)
// are gated by party-involvement.
//
// Reproduces the prod leak: with ORLY_AUTH_REQUIRED=true and default ACL mode
// (none), a live {"kinds":[4]} subscription must NOT receive a kind-4 DM that
// the authenticated user is not a party to (author nor p-tag).
package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"github.com/gorilla/websocket"
)

const schnorrPubKeyLen = 32

// deterministicPubkey builds a 32-byte binary pubkey from a label so tests
// don't need real key material (only identity is important here).
func deterministicPubkey(label byte) []byte {
	pk := make([]byte, schnorrPubKeyLen)
	for i := range pk {
		pk[i] = label
	}
	return pk
}

func TestSubscriptionRequiresAuth(t *testing.T) {
	kind4Filters := filter.S{&filter.F{Kinds: kind.NewS(kind.EncryptedDirectMessage)}}
	kind44Filters := filter.S{&filter.F{Kinds: kind.NewS(kind.New(44))}}

	tests := []struct {
		name    string
		cfg     *config.C
		filters *filter.S
		aclMode string
		want    bool
	}{
		{
			// The production deployment: AUTH_REQUIRED=true, ACL mode defaults to "none".
			name:    "prod config AuthRequired=true ACL none",
			cfg:     &config.C{AuthRequired: true, PrivilegedOpen: false},
			filters: &kind4Filters,
			aclMode: "none",
			want:    true,
		},
		{
			name:    "AuthToWrite=true ACL none still gates privileged delivery",
			cfg:     &config.C{AuthToWrite: true, PrivilegedOpen: false},
			filters: &kind4Filters,
			aclMode: "none",
			want:    true,
		},
		{
			name:    "follows ACL mode gates privileged delivery",
			cfg:     &config.C{PrivilegedOpen: false},
			filters: &kind4Filters,
			aclMode: "follows",
			want:    true,
		},
		{
			// Everything off: no auth, no ACL, no channel kinds => not required.
			name:    "everything off",
			cfg:     &config.C{PrivilegedOpen: false},
			filters: &kind4Filters,
			aclMode: "none",
			want:    false,
		},
		{
			// Non-discoverable channel kind must always be gated regardless of config.
			name:    "non-discoverable channel kind always gated",
			cfg:     &config.C{PrivilegedOpen: false},
			filters: &kind44Filters,
			aclMode: "none",
			want:    true,
		},
		{
			// PrivilegedOpen bypasses the gate.
			name:    "PrivilegedOpen bypasses",
			cfg:     &config.C{AuthRequired: true, PrivilegedOpen: true},
			filters: &kind4Filters,
			aclMode: "none",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			acl.Registry.SetMode(tt.aclMode)
			l := &Listener{Server: &Server{Config: tt.cfg}}
			got := l.subscriptionRequiresAuth(tt.filters)
			if got != tt.want {
				t.Fatalf("subscriptionRequiresAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestLiveKind4NotLeakedToNonParty is the core regression test proving the live
// subscription path enforces party-involvement for kind-4 DMs under the
// production config (AUTH_REQUIRED=true, ACL mode none).
func TestLiveKind4NotLeakedToNonParty(t *testing.T) {
	acl.Registry.SetMode("none")
	cfg := &config.C{AuthRequired: true, PrivilegedOpen: false}
	l := &Listener{Server: &Server{Config: cfg}}

	alice := deterministicPubkey(0xAA)
	bob := deterministicPubkey(0xBB)
	charlie := deterministicPubkey(0xCC)

	kind4 := filter.S{&filter.F{Kinds: kind.NewS(kind.EncryptedDirectMessage)}}
	authRequired := l.subscriptionRequiresAuth(&kind4)
	if !authRequired {
		t.Fatalf("expected subscriptionRequiresAuth=true for prod config, got false; live DMs would leak")
	}

	pub := NewPublisher(context.Background())
	pub.PrivilegedOpen = false
	defer func() { close(pub.stop); <-pub.done }()

	// Two separate websocket connections keyed by *websocket.Conn.
	aliceConn := &websocket.Conn{}
	charlieConn := &websocket.Conn{}
	aliceCh := make(event.C, 8)
	charlieCh := make(event.C, 8)

	// Alice subscribes to ALL kind-4 events live (typical DM client filter).
	pub.Receive(&W{
		Conn:         aliceConn,
		remote:       "wss://alice",
		Id:           "alice-sub",
		Receiver:     aliceCh,
		Filters:      &kind4,
		AuthedPubkey: alice,
		AuthRequired: authRequired,
	})

	// Charlie (the actual recipient) also subscribes to kind-4 events live.
	pub.Receive(&W{
		Conn:         charlieConn,
		remote:       "wss://charlie",
		Id:           "charlie-sub",
		Receiver:     charlieCh,
		Filters:      &kind4,
		AuthedPubkey: charlie,
		AuthRequired: authRequired,
	})

	// pub.Receive enqueues asynchronously; round-trip through the publisher
	// actor so both subscriptions are registered before we deliver. Otherwise
	// the deliver request can be processed by the actor before the second
	// subscription lands, making the test flaky (the real pipeline never
	// delivers this fast: events arrive from other goroutines well after REQ).
	syncResp := make(chan pubGetWriteChanResp, 1)
	pub.getWriteCh <- pubGetWriteChanReq{conn: aliceConn, resp: syncResp}
	<-syncResp

	// Bob sends a kind-4 DM to Charlie. The DM metadata (Bob, p=Charlie) is
	// only readable by Bob and Charlie — NOT by Alice.
	pTag := tag.New()
	pTag.T = [][]byte{[]byte("p"), []byte(hex.Enc(charlie))}
	dm := &event.E{
		ID:        bytes.Repeat([]byte{0x01}, 32),
		Pubkey:    bob,
		Kind:      kind.EncryptedDirectMessage.K,
		Content:   []byte("secret"),
		CreatedAt: time.Now().Unix(),
		Tags:      tag.NewS(pTag),
	}
	if dm.Tags.Len() == 0 {
		t.Fatal("failed to build p tag")
	}

	pub.Deliver(dm)

	// Charlie must receive the DM.
	select {
	case <-charlieCh:
		// expected
	case <-time.After(2 * time.Second):
		t.Fatal("Charlie (the recipient) did not receive the DM")
	}

	// Alice must NOT receive the DM.
	select {
	case ev := <-aliceCh:
		t.Fatalf("LEAK: Alice received a kind-4 DM from Bob to Charlie: %s", hex.Enc(ev.Pubkey))
	case <-time.After(500 * time.Millisecond):
		// expected — no leak
	}
}
