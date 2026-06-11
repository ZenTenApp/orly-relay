package marmot

import (
	"crypto/sha256"
	"errors"
	"time"

	"golang.org/x/crypto/hkdf"
	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
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

func (c *LocalCrypto) convKey(peerPub []byte) ([]byte, error) {
	shared, err := c.Sign.ECDHRaw(peerPub)
	if err != nil {
		return nil, err
	}
	return hkdf.Extract(sha256.New, shared, []byte("nip44-v2")), nil
}

func (c *LocalCrypto) Nip44Encrypt(peerPub []byte, plaintext []byte) (string, error) {
	convKey, err := c.convKey(peerPub)
	if err != nil {
		return "", err
	}
	return encryption.Encrypt(convKey, plaintext, nil)
}

func (c *LocalCrypto) Nip44Decrypt(peerPub []byte, ciphertext string) (string, error) {
	convKey, err := c.convKey(peerPub)
	if err != nil {
		return "", err
	}
	return encryption.Decrypt(convKey, ciphertext)
}

// --- ProxyCrypto actor request types ---

type proxyResult struct {
	Result string
	Err    string
}

type pcRequestReq struct {
	op      string
	peerHex string
	data    string
	resp    chan pcRequestResp
}

type pcRequestResp struct {
	result string
	err    error
}

type pcResolveReq struct {
	id     int
	result string
	errMsg string
}

type pcCloseReq struct {
	resp chan struct{}
}

// ProxyCrypto delegates crypto operations to the browser extension via WebSocket.
// Used for NIP-07 and pubkey+sig authenticated sessions.
type ProxyCrypto struct {
	pubkey    []byte
	requestCh chan pcRequestReq
	resolveCh chan pcResolveReq
	closeCh   chan pcCloseReq
	stop      chan struct{}
	done      chan struct{}
}

func NewProxyCrypto(pubkey []byte, sendFn func(op, peerHex, data string, id int)) *ProxyCrypto {
	c := &ProxyCrypto{
		pubkey:    pubkey,
		requestCh: make(chan pcRequestReq),
		resolveCh: make(chan pcResolveReq, 16),
		closeCh:   make(chan pcCloseReq),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
	go c.actor(sendFn)
	return c
}

func (c *ProxyCrypto) actor(sendFn func(op, peerHex, data string, id int)) {
	defer close(c.done)

	nextID := 0
	pending := make(map[int]chan proxyResult)

	for {
		select {
		case <-c.stop:
			// Unblock all pending with error
			for id, ch := range pending {
				ch <- proxyResult{Err: "connection closed"}
				delete(pending, id)
			}
			return

		case req := <-c.requestCh:
			id := nextID
			nextID++
			ch := make(chan proxyResult, 1)
			pending[id] = ch

			// Send to browser extension (non-blocking from actor's perspective,
			// sendFn is expected to be fast/non-blocking)
			sendFn(req.op, req.peerHex, req.data, id)

			// Wait for result in a separate goroutine to not block the actor
			go func() {
				select {
				case res := <-ch:
					if res.Err != "" {
						req.resp <- pcRequestResp{"", errors.New(res.Err)}
					} else {
						req.resp <- pcRequestResp{res.Result, nil}
					}
				case <-time.After(15 * time.Second):
					// Timeout - tell actor to clean up
					c.resolveCh <- pcResolveReq{id: id, errMsg: "crypto proxy timeout"}
					req.resp <- pcRequestResp{"", errors.New("crypto proxy timeout")}
				}
			}()

		case req := <-c.resolveCh:
			ch, ok := pending[req.id]
			if ok {
				delete(pending, req.id)
				ch <- proxyResult{req.result, req.errMsg}
			}

		case req := <-c.closeCh:
			for id, ch := range pending {
				ch <- proxyResult{Err: "connection closed"}
				delete(pending, id)
			}
			req.resp <- struct{}{}
		}
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
	req := pcRequestReq{
		op:      op,
		peerHex: peerHex,
		data:    data,
		resp:    make(chan pcRequestResp, 1),
	}
	c.requestCh <- req
	r := <-req.resp
	return r.result, r.err
}

// Resolve routes a crypto_resp from the browser to the waiting goroutine.
func (c *ProxyCrypto) Resolve(id int, result, errMsg string) {
	c.resolveCh <- pcResolveReq{id: id, result: result, errMsg: errMsg}
}

// Close unblocks all pending requests with an error (called on WS disconnect).
func (c *ProxyCrypto) Close() {
	req := pcCloseReq{resp: make(chan struct{}, 1)}
	c.closeCh <- req
	<-req.resp
}
