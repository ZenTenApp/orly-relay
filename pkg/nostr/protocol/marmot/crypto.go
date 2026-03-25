package marmot

import (
	"errors"
	"sync"
	"time"

	"next.orly.dev/pkg/nostr/crypto/encryption"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer"
)

// CryptoProvider abstracts the crypto operations marmot needs.
// LocalCrypto wraps signer.I for direct-key mode.
// ProxyCrypto delegates to the browser extension for NIP-07 mode.
type CryptoProvider interface {
	Pub() []byte
	SignEvent(ev *event.E) error
	Nip44Encrypt(peerPub []byte, plaintext []byte) (string, error)
	Nip44Decrypt(peerPub []byte, ciphertext string) (string, error)
}

// LocalCrypto wraps a signer.I for use as CryptoProvider.
// Used by bridge, bridgebot, and nsec-authenticated sessions.
type LocalCrypto struct{ Sign signer.I }

func (c *LocalCrypto) Pub() []byte { return c.Sign.Pub() }

func (c *LocalCrypto) SignEvent(ev *event.E) error { return ev.Sign(c.Sign) }

func (c *LocalCrypto) Nip44Encrypt(peerPub []byte, plaintext []byte) (string, error) {
	convKey, err := encryption.GenerateConversationKey(c.Sign.Sec(), peerPub)
	if err != nil {
		return "", err
	}
	return encryption.Encrypt(convKey, plaintext, nil)
}

func (c *LocalCrypto) Nip44Decrypt(peerPub []byte, ciphertext string) (string, error) {
	convKey, err := encryption.GenerateConversationKey(c.Sign.Sec(), peerPub)
	if err != nil {
		return "", err
	}
	return encryption.Decrypt(convKey, ciphertext)
}

// ProxyCrypto delegates crypto operations to the browser extension via WebSocket.
// Used for NIP-07 and pubkey+sig authenticated sessions.
type ProxyCrypto struct {
	pubkey  []byte
	mu      sync.Mutex
	nextID  int
	pending map[int]chan proxyResult
	sendFn  func(op, peerHex, data string, id int)
}

type proxyResult struct {
	Result string
	Err    string
}

func NewProxyCrypto(pubkey []byte, sendFn func(op, peerHex, data string, id int)) *ProxyCrypto {
	return &ProxyCrypto{
		pubkey:  pubkey,
		pending: make(map[int]chan proxyResult),
		sendFn:  sendFn,
	}
}

func (c *ProxyCrypto) Pub() []byte { return c.pubkey }

func (c *ProxyCrypto) SignEvent(ev *event.E) error {
	// Build unsigned event JSON for the extension to sign.
	ev.Pubkey = make([]byte, len(c.pubkey))
	copy(ev.Pubkey, c.pubkey)
	ev.ID = ev.GetIDBytes()
	unsigned, err := ev.MarshalJSON()
	if err != nil {
		return err
	}
	signed, err := c.request("signEvent", "", string(unsigned))
	if err != nil {
		return err
	}
	// Parse the signed event to extract sig and verified ID.
	return ev.UnmarshalJSON([]byte(signed))
}

func (c *ProxyCrypto) Nip44Encrypt(peerPub []byte, plaintext []byte) (string, error) {
	return c.request("nip44Encrypt", hex.Enc(peerPub), string(plaintext))
}

func (c *ProxyCrypto) Nip44Decrypt(peerPub []byte, ciphertext string) (string, error) {
	return c.request("nip44Decrypt", hex.Enc(peerPub), ciphertext)
}

func (c *ProxyCrypto) request(op, peerHex, data string) (string, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan proxyResult, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	c.sendFn(op, peerHex, data, id)

	select {
	case res := <-ch:
		if res.Err != "" {
			return "", errors.New(res.Err)
		}
		return res.Result, nil
	case <-time.After(15 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return "", errors.New("crypto proxy timeout")
	}
}

// Resolve routes a crypto_resp from the browser to the waiting goroutine.
func (c *ProxyCrypto) Resolve(id int, result, errMsg string) {
	c.mu.Lock()
	ch, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if ok {
		ch <- proxyResult{result, errMsg}
	}
}

// Close unblocks all pending requests with an error (called on WS disconnect).
func (c *ProxyCrypto) Close() {
	c.mu.Lock()
	for id, ch := range c.pending {
		ch <- proxyResult{Err: "connection closed"}
		delete(c.pending, id)
	}
	c.mu.Unlock()
}
