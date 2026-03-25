package marmot

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

// simulatedExtension creates a ProxyCrypto backed by real keys, with a
// goroutine that intercepts sendFn calls and resolves them using LocalCrypto.
// This simulates the browser extension path without needing a browser.
func simulatedExtension(t *testing.T, sign *p8k.Signer) (*ProxyCrypto, func()) {
	t.Helper()
	local := &LocalCrypto{Sign: sign}

	type cryptoReq struct {
		op, peerHex, data string
		id                int
	}
	ch := make(chan cryptoReq, 16)

	proxy := NewProxyCrypto(sign.Pub(), func(op, peerHex, data string, id int) {
		ch <- cryptoReq{op, peerHex, data, id}
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case req := <-ch:
				var result string
				var errMsg string
				switch req.op {
				case "signEvent":
					var ev event.E
					if err := ev.UnmarshalJSON([]byte(req.data)); err != nil {
						errMsg = err.Error()
					} else if err := local.SignEvent(&ev); err != nil {
						errMsg = err.Error()
					} else {
						b, _ := ev.MarshalJSON()
						result = string(b)
					}
				case "nip44Encrypt":
					peer, _ := hex.Dec(req.peerHex)
					ct, err := local.Nip44Encrypt(peer, []byte(req.data))
					if err != nil {
						errMsg = err.Error()
					} else {
						result = ct
					}
				case "nip44Decrypt":
					peer, _ := hex.Dec(req.peerHex)
					pt, err := local.Nip44Decrypt(peer, req.data)
					if err != nil {
						errMsg = err.Error()
					} else {
						result = pt
					}
				default:
					errMsg = "unknown op: " + req.op
				}
				proxy.Resolve(req.id, result, errMsg)
			case <-ctx.Done():
				return
			}
		}
	}()

	return proxy, cancel
}

func TestProxyCrypto_BasicRequestResolve(t *testing.T) {
	var proxy *ProxyCrypto
	proxy = NewProxyCrypto([]byte("testpub"), func(op, peerHex, data string, id int) {
		go func() { proxy.Resolve(id, "result-"+data, "") }()
	})
	// Use the low-level request directly.
	result, err := proxy.request("test", "", "hello")
	require.NoError(t, err)
	assert.Equal(t, "result-hello", result)
}

func TestProxyCrypto_Close(t *testing.T) {
	proxy := NewProxyCrypto([]byte("testpub"), func(op, peerHex, data string, id int) {
		// Don't resolve — let Close() unblock it.
	})
	done := make(chan error, 1)
	go func() {
		_, err := proxy.request("test", "", "blocked")
		done <- err
	}()
	time.Sleep(10 * time.Millisecond) // let goroutine start
	proxy.Close()
	err := <-done
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection closed")
}

func TestProxyCrypto_ConcurrentRequests(t *testing.T) {
	var mu sync.Mutex
	pending := make(map[int]string)

	proxy := NewProxyCrypto([]byte("testpub"), func(op, peerHex, data string, id int) {
		mu.Lock()
		pending[id] = data
		mu.Unlock()
	})

	const n = 5
	results := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = proxy.request("test", "", "req-"+string(rune('A'+idx)))
		}(i)
	}

	// Wait for all requests to register.
	time.Sleep(20 * time.Millisecond)

	// Resolve all pending requests.
	mu.Lock()
	for id, data := range pending {
		proxy.Resolve(id, "ok-"+data, "")
	}
	mu.Unlock()

	wg.Wait()
	for i := 0; i < n; i++ {
		require.NoError(t, errs[i], "request %d", i)
		assert.Contains(t, results[i], "ok-req-", "request %d", i)
	}
}

func TestProxyCrypto_ResolveUnknownID(t *testing.T) {
	proxy := NewProxyCrypto([]byte("testpub"), func(op, peerHex, data string, id int) {})
	// Should not panic.
	proxy.Resolve(999, "whatever", "")
	proxy.Resolve(-1, "", "error")
}

func TestProxyCrypto_CloseIdempotent(t *testing.T) {
	proxy := NewProxyCrypto([]byte("testpub"), func(op, peerHex, data string, id int) {})
	proxy.Close()
	proxy.Close() // should not panic
}

func TestProxyCrypto_ResolveAfterClose(t *testing.T) {
	proxy := NewProxyCrypto([]byte("testpub"), func(op, peerHex, data string, id int) {})

	done := make(chan error, 1)
	go func() {
		_, err := proxy.request("test", "", "data")
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	proxy.Close()
	err := <-done
	require.Error(t, err)

	// Late resolve after close — should not panic.
	proxy.Resolve(0, "late", "")
}

func TestProxyCrypto_SignEventRoundTrip(t *testing.T) {
	sign := generateSigner(t)
	proxy, cancel := simulatedExtension(t, sign)
	defer cancel()

	ev := event.New()
	ev.CreatedAt = time.Now().Unix()
	ev.Kind = 1
	ev.Content = []byte("hello proxy")

	err := proxy.SignEvent(ev)
	require.NoError(t, err)

	// Event should have valid ID and sig from the simulated extension.
	assert.True(t, len(ev.ID) == 32, "ID should be 32 bytes")
	assert.True(t, len(ev.Sig) == 64, "Sig should be 64 bytes")
	assert.True(t, bytes.Equal(ev.Pubkey, sign.Pub()), "pubkey should match signer")

	// Verify the signature independently.
	ok, err := sign.Verify(ev.ID, ev.Sig)
	require.NoError(t, err)
	assert.True(t, ok, "signature should verify")
}

func TestProxyCrypto_Nip44EncryptDecryptRoundTrip(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceProxy, cancel := simulatedExtension(t, alice)
	defer cancel()
	bobLocal := &LocalCrypto{Sign: bob}

	// Alice encrypts for Bob via proxy.
	ct, err := aliceProxy.Nip44Encrypt(bob.Pub(), []byte("secret message"))
	require.NoError(t, err)
	assert.NotEmpty(t, ct)

	// Bob decrypts with local crypto.
	pt, err := bobLocal.Nip44Decrypt(alice.Pub(), ct)
	require.NoError(t, err)
	assert.Equal(t, "secret message", pt)
}

func TestProxyCrypto_WelcomeGiftWrapViaProxy(t *testing.T) {
	alice := generateSigner(t)
	bob := generateSigner(t)

	aliceProxy, cancel := simulatedExtension(t, alice)
	defer cancel()
	bobLocal := &LocalCrypto{Sign: bob}

	// Generate key packages.
	aliceKPP, err := GenerateKeyPackage(aliceProxy)
	require.NoError(t, err)
	bobKPP, err := GenerateKeyPackage(bobLocal)
	require.NoError(t, err)

	// Alice creates group with Bob's KP.
	_, welcome, _, err := CreateDMGroup(aliceKPP, &bobKPP.Public, alice.Pub(), bob.Pub(), nil)
	require.NoError(t, err)

	// Alice gift-wraps the welcome via ProxyCrypto.
	wrapEv, err := WelcomeToGiftWrap(welcome, bob.Pub(), aliceProxy, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, KindGiftWrap, wrapEv.Kind)

	// Bob unwraps with LocalCrypto.
	unwrapped, err := UnwrapWelcome(wrapEv, bobLocal)
	require.NoError(t, err)

	// Sender should be Alice's real pubkey from the seal layer.
	assert.True(t, bytes.Equal(unwrapped.SenderPub, alice.Pub()),
		"sender should be Alice, got %s", hex.Enc(unwrapped.SenderPub))
	assert.NotNil(t, unwrapped.Welcome)
}

func TestProxyCrypto_ClientE2E(t *testing.T) {
	relay := newMockRelay()
	alice := generateSigner(t)
	bob := generateSigner(t)

	// Alice uses ProxyCrypto (simulated extension).
	aliceProxy, cancelProxy := simulatedExtension(t, alice)
	defer cancelProxy()

	aliceClient, err := NewClient(aliceProxy, NewMemoryGroupStore(), relay)
	require.NoError(t, err)

	// Bob uses LocalCrypto.
	bobClient, err := NewClient(&LocalCrypto{Sign: bob}, NewMemoryGroupStore(), relay)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Bob publishes key package.
	require.NoError(t, bobClient.PublishKeyPackage(ctx))

	// Bob listens for events.
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

	// Alice sends DM to Bob via ProxyCrypto.
	err = aliceClient.SendDM(ctx, bob.Pub(), []byte("hello from proxy"))
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, []byte("hello from proxy"), msg.plaintext)
		assert.True(t, bytes.Equal(msg.sender, alice.Pub()),
			"sender should be Alice's real pubkey, got %s", hex.Enc(msg.sender))
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for DM")
	}
}
