package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
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
	client  *marmot.Client
	adapter *marmot.WSRelayAdapter
	sign    signer
	conn    *websocket.Conn
	mu      sync.Mutex
	cancel  context.CancelFunc
	relays  []*ws.Client
}

// signer wraps p8k.Signer to keep the import local.
type signer = *p8k.Signer

// marmotReq is the JSON-RPC request from the client.
type marmotReq struct {
	Method    string          `json:"method"`
	Pubkey    string          `json:"pubkey,omitempty"`
	Sig       string          `json:"sig,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	Recipient string          `json:"recipient,omitempty"`
	Content   string          `json:"content,omitempty"`
	Relays    []string        `json:"relays,omitempty"`
}

// marmotResp is the JSON-RPC response to the client.
type marmotResp struct {
	Method  string   `json:"method"`
	OK      bool     `json:"ok,omitempty"`
	Error   string   `json:"error,omitempty"`
	Peer    string   `json:"peer,omitempty"`
	Content string   `json:"content,omitempty"`
	Ts      int64    `json:"ts,omitempty"`
	Groups  []string `json:"groups,omitempty"`
}

func (s *Smesh3Server) handleMarmot(w http.ResponseWriter, r *http.Request) {
	log.I.F("marmot: WS connection from %s", r.RemoteAddr)
	conn, err := marmotUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.W.F("marmot: upgrade failed: %v", err)
		return
	}
	defer conn.Close()
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
		default:
			sess.writeResp(marmotResp{Method: "error", Error: "unknown method: " + req.Method})
		}
	}
}

func (sess *marmotSession) handleAuth(ctx context.Context, dataDir string, req marmotReq) {
	var pubkeyHex string

	if len(req.Event) > 0 {
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
		sess.writeResp(marmotResp{Method: "auth", Error: "pubkey+sig or event required"})
		return
	}

	// Create signer with a fresh key for this session's MLS operations.
	sign, err := p8k.New()
	if err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "signer creation failed"})
		return
	}

	// For a self-hosted relay, the auth proves the user controls this pubkey.
	// Generate a session key for MLS. In a real deployment the user's
	// actual secret key would be used, but here we generate ephemeral
	// keys for the MLS layer.
	if err := sign.Generate(); err != nil {
		sess.writeResp(marmotResp{Method: "auth", Error: "key generation failed"})
		return
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

	sess.writeResp(marmotResp{Method: "send_dm", OK: true})
}

func (sess *marmotSession) handleSubscribe(ctx context.Context) {
	if sess.client == nil {
		sess.writeResp(marmotResp{Method: "subscribe", Error: "not authenticated"})
		return
	}

	filters := sess.client.SubscriptionFilters()
	go func() {
		for {
			stream, err := sess.adapter.Subscribe(ctx, filters)
			if err != nil {
				log.W.F("marmot: subscription failed: %v", err)
				return
			}

			for ev := range stream.Events() {
				if err := sess.client.HandleEvent(ctx, ev); err != nil {
					log.W.F("marmot: handle event error: %v", err)
				}
			}

			// Check if context is done before retrying.
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	// Watch for group changes and refresh subscriptions.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sess.client.GroupsChanged():
				filters = sess.client.SubscriptionFilters()
				log.I.F("marmot: subscription filters refreshed")
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
