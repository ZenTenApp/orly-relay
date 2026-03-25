package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/nostr/protocol/marmot"
	"next.orly.dev/pkg/nostr/ws"
)

var marmotUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// marmotSession holds state for one authenticated Marmot WebSocket client.
type marmotSession struct {
	client    *marmot.Client
	adapter   *marmot.WSRelayAdapter
	sign      signer
	conn      *websocket.Conn
	mu        sync.Mutex
	cancel    context.CancelFunc
	relays    []*ws.Client
	pubkeyHex string
	relayURL  string
	subscribed bool
	subCancel  context.CancelFunc // cancels the subscription goroutine
}

// signer wraps p8k.Signer to keep the import local.
type signer = *p8k.Signer

// marmotReq is the JSON-RPC request from the client.
type marmotReq struct {
	Method    string          `json:"method"`
	Pubkey    string          `json:"pubkey,omitempty"`
	Sig       string          `json:"sig,omitempty"`
	Nsec      string          `json:"nsec,omitempty"` // hex secret key — self-hosted / trusted backend auth
	Event     json.RawMessage `json:"event,omitempty"`
	Recipient string          `json:"recipient,omitempty"`
	Content   string          `json:"content,omitempty"`
	Relays    []string        `json:"relays,omitempty"`
	Alias     string          `json:"alias,omitempty"` // NIP-05 address (user@domain)
}

// marmotResp is the JSON-RPC response to the client.
type marmotResp struct {
	Method     string   `json:"method"`
	OK         bool     `json:"ok,omitempty"`
	Error      string   `json:"error,omitempty"`
	Peer       string   `json:"peer,omitempty"`
	Content    string   `json:"content,omitempty"`
	Ts         int64    `json:"ts,omitempty"`
	Groups     []string `json:"groups,omitempty"`
	Pubkey     string   `json:"pubkey,omitempty"`     // for resolve_alias
	Subscribed bool     `json:"subscribed,omitempty"` // for status
	Relay      string   `json:"relay,omitempty"`      // for status
	NumGroups  int      `json:"num_groups,omitempty"` // for status
}

func (s *Smesh3Server) handleMarmot(w http.ResponseWriter, r *http.Request) {
	log.I.F("marmot: WS connection from %s", r.RemoteAddr)
	conn, err := marmotUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.W.F("marmot: upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Time{})
	log.I.F("marmot: WS upgraded successfully")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	sess := &marmotSession{conn: conn, cancel: cancel}
	defer sess.cleanup()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.W.F("marmot: read error: %v", err)
			}
			log.I.F("marmot: client disconnected")
			return
		}

		log.I.F("marmot: recv: %s", string(msg))

		var req marmotReq
		if err := json.Unmarshal(msg, &req); err != nil {
			sess.writeResp(marmotResp{Method: "error", Error: "invalid json"})
			continue
		}

		switch req.Method {
		case "auth":
			sess.handleAuth(ctx, s.dataDir, req)
		case "send_dm":
			log.I.F("marmot: send_dm to=%s content=%q", req.Recipient, req.Content)
			sess.handleSendDM(ctx, req)
		case "subscribe":
			sess.handleSubscribe(ctx)
		case "publish_kp":
			sess.handlePublishKP(ctx, req)
		case "list_groups":
			sess.handleListGroups()
		case "status":
			sess.handleStatus()
		case "reset":
			sess.handleReset(ctx, s.dataDir)
		case "resolve_alias":
			sess.handleResolveAlias(req)
		default:
			sess.writeResp(marmotResp{Method: "error", Error: "unknown method: " + req.Method})
		}
	}
}

func (sess *marmotSession) handleAuth(ctx context.Context, dataDir string, req marmotReq) {
	var pubkeyHex string

	if req.Nsec != "" {
		// Direct secret key auth — self-hosted / trusted backend.
		// The client sends the hex secret key; we derive the pubkey and
		// use this key for all MLS operations so that key packages are
		// discoverable by the user's actual Nostr pubkey.
		sec, err := hex.Dec(req.Nsec)
		if err != nil || len(sec) != 32 {
			sess.writeResp(marmotResp{Method: "auth", Error: "invalid nsec"})
			return
		}
		sign, err := p8k.New()
		if err != nil {
			sess.writeResp(marmotResp{Method: "auth", Error: "signer creation failed"})
			return
		}
		if err := sign.InitSec(sec); err != nil {
			sess.writeResp(marmotResp{Method: "auth", Error: "signer init failed: " + err.Error()})
			return
		}
		pubkeyHex = hex.Enc(sign.Pub())
		sess.sign = sign
	} else if len(req.Event) > 0 {
		// Event-based auth (NIP-07 extension mode).
		pubkeyHex = sess.handleEventAuth(req)
		if pubkeyHex == "" {
			return
		}
	} else if req.Pubkey != "" && req.Sig != "" {
		// Direct Schnorr signature auth.
		pubkeyHex = req.Pubkey
		pub, err := hex.Dec(req.Pubkey)
		if err != nil || len(pub) != 32 {
			sess.writeResp(marmotResp{Method: "auth", Error: "invalid pubkey"})
			return
		}
		sig, err := hex.Dec(req.Sig)
		if err != nil || len(sig) != 64 {
			sess.writeResp(marmotResp{Method: "auth", Error: "invalid signature"})
			return
		}
		verifier, err := p8k.New()
		if err != nil {
			sess.writeResp(marmotResp{Method: "auth", Error: "signer init failed"})
			return
		}
		if err := verifier.InitPub(pub); err != nil {
			sess.writeResp(marmotResp{Method: "auth", Error: "pubkey init failed"})
			return
		}
		msgHash := sha256.Sum256(pub)
		ok, err := verifier.Verify(msgHash[:], sig)
		if err != nil || !ok {
			sess.writeResp(marmotResp{Method: "auth", Error: "signature verification failed"})
			return
		}
	} else {
		sess.writeResp(marmotResp{Method: "auth", Error: "nsec, pubkey+sig, or event required"})
		return
	}

	// If signer not set yet (pubkey+sig or event auth), create one.
	// For these modes, the user's actual secret key is unavailable, so
	// MLS uses an ephemeral key. KP discovery won't work until crypto
	// proxying is wired through the SW bus.
	sign := sess.sign
	if sign == nil {
		var err error
		sign, err = p8k.New()
		if err != nil {
			sess.writeResp(marmotResp{Method: "auth", Error: "signer creation failed"})
			return
		}
		if err := sign.Generate(); err != nil {
			sess.writeResp(marmotResp{Method: "auth", Error: "key generation failed"})
			return
		}
	}

	// Create group store.
	storeDir := filepath.Join(dataDir, "marmot", pubkeyHex)
	store, err := marmot.NewFileGroupStore(storeDir)
	if err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "store creation failed: " + err.Error()})
		return
	}

	// Connect to default relay for outbound Marmot operations.
	relayURL := "wss://relay.orly.dev"
	relay, err := ws.RelayConnect(ctx, relayURL)
	if err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "relay connection failed: " + err.Error()})
		return
	}
	sess.relays = append(sess.relays, relay)

	sess.adapter = marmot.NewWSRelayAdapter(relay)
	client, err := marmot.NewClient(sign, store, sess.adapter, relayURL)
	if err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "marmot client creation failed: " + err.Error()})
		return
	}

	// Register DM handler — push incoming DMs to the WebSocket.
	client.OnDM(func(senderPub []byte, plaintext []byte) {
		sess.writeResp(marmotResp{
			Method:  "dm_received",
			Peer:    hex.Enc(senderPub),
			Content: string(plaintext),
			Ts:      time.Now().Unix(),
		})
	})

	sess.client = client
	sess.sign = sign
	sess.pubkeyHex = pubkeyHex
	sess.relayURL = relayURL
	sess.writeResp(marmotResp{Method: "auth", OK: true})
	log.I.F("marmot: authenticated session for %s", pubkeyHex)
}

// handleEventAuth verifies a signed kind 22242 auth event.
// Returns the pubkey hex on success, empty string on failure.
func (sess *marmotSession) handleEventAuth(req marmotReq) string {
	var ev struct {
		ID        string     `json:"id"`
		PubKey    string     `json:"pubkey"`
		Kind      int        `json:"kind"`
		Content   string     `json:"content"`
		Sig       string     `json:"sig"`
		CreatedAt int64      `json:"created_at"`
		Tags      [][]string `json:"tags"`
	}
	if err := json.Unmarshal(req.Event, &ev); err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "invalid event json"})
		return ""
	}
	if ev.Kind != 22242 {
		sess.writeResp(marmotResp{Method: "auth", Error: "wrong event kind"})
		return ""
	}
	if ev.PubKey == "" || ev.Sig == "" || ev.ID == "" {
		sess.writeResp(marmotResp{Method: "auth", Error: "incomplete event"})
		return ""
	}

	// Verify event signature.
	pub, err := hex.Dec(ev.PubKey)
	if err != nil || len(pub) != 32 {
		sess.writeResp(marmotResp{Method: "auth", Error: "invalid event pubkey"})
		return ""
	}
	sig, err := hex.Dec(ev.Sig)
	if err != nil || len(sig) != 64 {
		sess.writeResp(marmotResp{Method: "auth", Error: "invalid event signature"})
		return ""
	}

	// Verify the event ID matches the serialized content.
	serialized := serializeEvent(ev.Kind, ev.PubKey, ev.CreatedAt, ev.Tags, ev.Content)
	idHash := sha256.Sum256([]byte(serialized))
	computedID := fmt.Sprintf("%x", idHash)
	if computedID != ev.ID {
		sess.writeResp(marmotResp{Method: "auth", Error: "event id mismatch"})
		return ""
	}

	// Verify BIP-340 signature over the event ID hash.
	verifier, err := p8k.New()
	if err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "signer init failed"})
		return ""
	}
	if err := verifier.InitPub(pub); err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "pubkey init failed"})
		return ""
	}
	ok, err := verifier.Verify(idHash[:], sig)
	if err != nil || !ok {
		sess.writeResp(marmotResp{Method: "auth", Error: "event signature verification failed"})
		return ""
	}

	// Check freshness — reject events older than 5 minutes.
	now := time.Now().Unix()
	if ev.CreatedAt < now-300 || ev.CreatedAt > now+60 {
		sess.writeResp(marmotResp{Method: "auth", Error: "event too old or too far in future"})
		return ""
	}

	return ev.PubKey
}

// serializeEvent produces the NIP-01 serialization for event ID computation.
func serializeEvent(kind int, pubkey string, createdAt int64, tags [][]string, content string) string {
	s := fmt.Sprintf("[0,%q,%d,%d,", pubkey, createdAt, kind)
	// Tags.
	s += "["
	for i, tag := range tags {
		if i > 0 {
			s += ","
		}
		s += "["
		for j, v := range tag {
			if j > 0 {
				s += ","
			}
			s += fmt.Sprintf("%q", v)
		}
		s += "]"
	}
	s += "],"
	s += fmt.Sprintf("%q]", content)
	return s
}

func (sess *marmotSession) handleSendDM(ctx context.Context, req marmotReq) {
	if sess.client == nil {
		sess.writeResp(marmotResp{Method: "send_dm", Error: "not authenticated"})
		return
	}
	if req.Recipient == "" || req.Content == "" {
		sess.writeResp(marmotResp{Method: "send_dm", Error: "recipient and content required"})
		return
	}

	recipientPub, err := hex.Dec(req.Recipient)
	if err != nil || len(recipientPub) != 32 {
		sess.writeResp(marmotResp{Method: "send_dm", Error: "invalid recipient pubkey"})
		return
	}

	if err := sess.client.SendDM(ctx, recipientPub, []byte(req.Content)); err != nil {
		sess.writeResp(marmotResp{Method: "send_dm", Error: err.Error()})
		return
	}

	sess.writeResp(marmotResp{Method: "send_dm", OK: true, Ts: time.Now().Unix()})
}

func (sess *marmotSession) handleSubscribe(ctx context.Context) {
	if sess.client == nil {
		sess.writeResp(marmotResp{Method: "subscribe", Error: "not authenticated"})
		return
	}

	// Cancel previous subscription if any.
	if sess.subCancel != nil {
		sess.subCancel()
	}

	subCtx, subCancel := context.WithCancel(ctx)
	sess.subCancel = subCancel
	sess.subscribed = true

	go func() {
		defer func() { sess.subscribed = false }()
		for {
			filters := sess.client.SubscriptionFilters()
			stream, err := sess.adapter.Subscribe(subCtx, filters)
			if err != nil {
				log.W.F("marmot: subscription failed: %v", err)
				return
			}

			// Read events until stream ends or groups change.
			done := make(chan struct{})
			go func() {
				defer close(done)
				for ev := range stream.Events() {
					if err := sess.client.HandleEvent(subCtx, ev); err != nil {
						log.W.F("marmot: handle event error: %v", err)
					}
				}
			}()

			select {
			case <-subCtx.Done():
				stream.Close()
				return
			case <-sess.client.GroupsChanged():
				log.I.F("marmot: groups changed, re-subscribing")
				stream.Close()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
				}
			case <-done:
				select {
				case <-subCtx.Done():
					return
				default:
				}
			}
		}
	}()

	sess.writeResp(marmotResp{Method: "subscribe", OK: true})
}

func (sess *marmotSession) handlePublishKP(ctx context.Context, req marmotReq) {
	if sess.client == nil {
		sess.writeResp(marmotResp{Method: "publish_kp", Error: "not authenticated"})
		return
	}

	if err := sess.client.PublishKeyPackage(ctx); err != nil {
		sess.writeResp(marmotResp{Method: "publish_kp", Error: err.Error()})
		return
	}

	if len(req.Relays) > 0 {
		if err := sess.client.PublishKeyPackageRelays(ctx, req.Relays); err != nil {
			log.W.F("marmot: publish key package relays failed: %v", err)
		}
	}

	sess.writeResp(marmotResp{Method: "publish_kp", OK: true})
}

func (sess *marmotSession) handleListGroups() {
	if sess.client == nil {
		sess.writeResp(marmotResp{Method: "list_groups", Error: "not authenticated"})
		return
	}

	sess.writeResp(marmotResp{
		Method: "list_groups",
		OK:     true,
		Groups: sess.client.ActiveGroupIDs(),
	})
}

func (sess *marmotSession) handleStatus() {
	if sess.client == nil {
		sess.writeResp(marmotResp{Method: "status", Error: "not authenticated"})
		return
	}
	groups := sess.client.ActiveGroupIDs()
	sess.writeResp(marmotResp{
		Method:     "status",
		OK:         true,
		Pubkey:     sess.pubkeyHex,
		Relay:      sess.relayURL,
		Subscribed: sess.subscribed,
		NumGroups:  len(groups),
		Groups:     groups,
	})
}

func (sess *marmotSession) handleReset(ctx context.Context, dataDir string) {
	// Cancel active subscription.
	if sess.subCancel != nil {
		sess.subCancel()
		sess.subCancel = nil
	}
	sess.subscribed = false

	// Close relay connections.
	for _, r := range sess.relays {
		r.Close()
	}
	sess.relays = nil
	sess.adapter = nil
	sess.client = nil

	// Re-establish if we have a signer (nsec auth persists across reset).
	if sess.sign != nil && sess.pubkeyHex != "" {
		storeDir := filepath.Join(dataDir, "marmot", sess.pubkeyHex)
		// Clear persisted groups for a clean slate.
		os.RemoveAll(storeDir)
		store, err := marmot.NewFileGroupStore(storeDir)
		if err != nil {
			sess.writeResp(marmotResp{Method: "reset", Error: "store: " + err.Error()})
			return
		}
		relayURL := "wss://relay.orly.dev"
		relay, err := ws.RelayConnect(ctx, relayURL)
		if err != nil {
			sess.writeResp(marmotResp{Method: "reset", Error: "relay: " + err.Error()})
			return
		}
		sess.relays = append(sess.relays, relay)
		sess.adapter = marmot.NewWSRelayAdapter(relay)
		sess.relayURL = relayURL

		client, err := marmot.NewClient(sess.sign, store, sess.adapter, relayURL)
		if err != nil {
			sess.writeResp(marmotResp{Method: "reset", Error: "client: " + err.Error()})
			return
		}
		client.OnDM(func(senderPub []byte, plaintext []byte) {
			sess.writeResp(marmotResp{
				Method:  "dm_received",
				Peer:    hex.Enc(senderPub),
				Content: string(plaintext),
				Ts:      time.Now().Unix(),
			})
		})
		sess.client = client
	}

	sess.writeResp(marmotResp{Method: "reset", OK: true})
	log.I.F("marmot: session reset for %s", sess.pubkeyHex)
}

func (sess *marmotSession) handleResolveAlias(req marmotReq) {
	if req.Alias == "" {
		sess.writeResp(marmotResp{Method: "resolve_alias", Error: "alias required"})
		return
	}

	pubkey, err := resolveNIP05(req.Alias)
	if err != nil {
		sess.writeResp(marmotResp{Method: "resolve_alias", Error: err.Error()})
		return
	}

	sess.writeResp(marmotResp{
		Method: "resolve_alias",
		OK:     true,
		Pubkey: pubkey,
	})
}

// resolveNIP05 resolves a NIP-05 identifier (user@domain) to a hex pubkey.
func resolveNIP05(addr string) (string, error) {
	parts := splitNIP05(addr)
	if parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("invalid NIP-05 address: %s", addr)
	}

	url := "https://" + parts[1] + "/.well-known/nostr.json?name=" + parts[0]
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("NIP-05 fetch failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("NIP-05 returned %d", resp.StatusCode)
	}

	var result struct {
		Names map[string]string `json:"names"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("NIP-05 decode failed: %w", err)
	}

	pubkey, ok := result.Names[parts[0]]
	if !ok {
		return "", fmt.Errorf("NIP-05: name %q not found at %s", parts[0], parts[1])
	}

	if len(pubkey) != 64 {
		return "", fmt.Errorf("NIP-05: invalid pubkey length %d", len(pubkey))
	}

	return pubkey, nil
}

func splitNIP05(addr string) [2]string {
	for i := 0; i < len(addr); i++ {
		if addr[i] == '@' {
			return [2]string{addr[:i], addr[i+1:]}
		}
	}
	// bare domain → "_" user per NIP-05
	return [2]string{"_", addr}
}

func (sess *marmotSession) writeResp(resp marmotResp) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	data, err := json.Marshal(resp)
	if err != nil {
		log.W.F("marmot: marshal response failed: %v", err)
		return
	}
	log.I.F("marmot: send: %s", string(data))
	if err := sess.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.W.F("marmot: write failed: %v", err)
	}
}

func (sess *marmotSession) cleanup() {
	for _, r := range sess.relays {
		r.Close()
	}
}

// nonce generates a random 32-byte hex nonce for auth challenges.
func nonce() string {
	b := make([]byte, 32)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
