package bunker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"lukechampine.com/frand"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
)

// NIP-46 method names
const (
	MethodConnect      = "connect"
	MethodGetPublicKey = "get_public_key"
	MethodSignEvent    = "sign_event"
	MethodNIP04Encrypt = "nip04_encrypt"
	MethodNIP04Decrypt = "nip04_decrypt"
	MethodNIP44Encrypt = "nip44_encrypt"
	MethodNIP44Decrypt = "nip44_decrypt"
	MethodPing         = "ping"
)

// Session represents a NIP-46 client session.
type Session struct {
	ID            string
	conn          *websocket.Conn
	ctx           context.Context
	cancel        context.CancelFunc
	relaySigner   signer.I
	relayPubkey   []byte
	authenticated bool
	clientPubkey  []byte // Client's pubkey after connect
}

// NewSession creates a new bunker session.
func NewSession(parentCtx context.Context, conn *websocket.Conn, relaySigner signer.I, relayPubkey []byte) *Session {
	ctx, cancel := context.WithCancel(parentCtx)

	// Generate random session ID
	idBytes := make([]byte, 16)
	frand.Read(idBytes)

	return &Session{
		ID:          hex.Enc(idBytes),
		conn:        conn,
		ctx:         ctx,
		cancel:      cancel,
		relaySigner: relaySigner,
		relayPubkey: relayPubkey,
	}
}

// Handle processes messages from the client.
func (s *Session) Handle() {
	defer s.conn.Close()
	defer s.cancel()

	log.D.F("bunker session started: %s", s.ID[:8])

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Set read deadline
		s.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Read message
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.D.F("bunker session closed normally: %s", s.ID[:8])
			} else {
				log.D.F("bunker session read error: %v", err)
			}
			return
		}

		// Parse request
		var req NIP46Request
		if err := json.Unmarshal(msg, &req); err != nil {
			s.sendError("", "invalid request format")
			continue
		}

		// Handle request
		resp := s.handleRequest(&req)
		s.sendResponse(resp)
	}
}

// handleRequest processes a NIP-46 request.
func (s *Session) handleRequest(req *NIP46Request) *NIP46Response {
	switch req.Method {
	case MethodConnect:
		return s.handleConnect(req)
	case MethodGetPublicKey:
		return s.handleGetPublicKey(req)
	case MethodSignEvent:
		return s.handleSignEvent(req)
	case MethodPing:
		return s.handlePing(req)
	case MethodNIP44Encrypt, MethodNIP44Decrypt, MethodNIP04Encrypt, MethodNIP04Decrypt:
		// Encryption/decryption not supported in this bunker implementation
		return &NIP46Response{
			ID:    req.ID,
			Error: "encryption methods not supported",
		}
	default:
		return &NIP46Response{
			ID:    req.ID,
			Error: fmt.Sprintf("unsupported method: %s", req.Method),
		}
	}
}

// handleConnect handles the connect method.
func (s *Session) handleConnect(req *NIP46Request) *NIP46Response {
	// Parse params: [pubkey, secret?]
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &NIP46Response{ID: req.ID, Error: "invalid params"}
	}

	if len(params) < 1 {
		return &NIP46Response{ID: req.ID, Error: "missing pubkey"}
	}

	pubkeyHex := params[0]
	clientPubkey, err := hex.Dec(pubkeyHex)
	if err != nil || len(clientPubkey) != 32 {
		return &NIP46Response{ID: req.ID, Error: "invalid pubkey"}
	}

	s.clientPubkey = clientPubkey
	s.authenticated = true

	log.D.F("bunker session authenticated: %s (client=%s...)",
		s.ID[:8], pubkeyHex[:16])

	return &NIP46Response{
		ID:     req.ID,
		Result: "ack",
	}
}

// handleGetPublicKey returns the relay's public key.
func (s *Session) handleGetPublicKey(req *NIP46Request) *NIP46Response {
	return &NIP46Response{
		ID:     req.ID,
		Result: hex.Enc(s.relayPubkey),
	}
}

// handleSignEvent signs an event with the relay's key.
func (s *Session) handleSignEvent(req *NIP46Request) *NIP46Response {
	if !s.authenticated {
		return &NIP46Response{ID: req.ID, Error: "not authenticated"}
	}

	// Parse event from params
	var params []json.RawMessage
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &NIP46Response{ID: req.ID, Error: "invalid params"}
	}

	if len(params) < 1 {
		return &NIP46Response{ID: req.ID, Error: "missing event"}
	}

	// Parse the event
	ev := &event.E{}
	if err := json.Unmarshal(params[0], ev); err != nil {
		return &NIP46Response{ID: req.ID, Error: "invalid event"}
	}

	// Set pubkey to relay's pubkey
	copy(ev.Pubkey[:], s.relayPubkey)

	// Set created_at if not set
	if ev.CreatedAt == 0 {
		ev.CreatedAt = timestamp.Now().V
	}

	// Sign the event
	if err := ev.Sign(s.relaySigner); err != nil {
		return &NIP46Response{ID: req.ID, Error: fmt.Sprintf("signing failed: %v", err)}
	}

	// Return signed event as JSON
	signedJSON, err := json.Marshal(ev)
	if err != nil {
		return &NIP46Response{ID: req.ID, Error: "marshal failed"}
	}

	return &NIP46Response{
		ID:     req.ID,
		Result: string(signedJSON),
	}
}

// handlePing responds to ping requests.
func (s *Session) handlePing(req *NIP46Request) *NIP46Response {
	return &NIP46Response{
		ID:     req.ID,
		Result: "pong",
	}
}

// sendResponse sends a response to the client.
func (s *Session) sendResponse(resp *NIP46Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.E.F("bunker marshal error: %v", err)
		return
	}

	s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := s.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.E.F("bunker write error: %v", err)
	}
}

// sendError sends an error response.
func (s *Session) sendError(id, msg string) {
	s.sendResponse(&NIP46Response{
		ID:    id,
		Error: msg,
	})
}
