package app

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"next.orly.dev/pkg/lol/log"
)

var busUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// busMsg is the envelope sent by SW peers over the bus.
type busMsg struct {
	To  string          `json:"to"`
	Msg json.RawMessage `json:"msg"`
}

// busHub routes messages between SW peers identified by role.
type busHub struct {
	mu    sync.RWMutex
	peers map[string]*websocket.Conn
}

func newBusHub() *busHub {
	return &busHub{peers: make(map[string]*websocket.Conn)}
}

func (h *busHub) register(role string, conn *websocket.Conn) {
	h.mu.Lock()
	if old, ok := h.peers[role]; ok {
		old.Close()
	}
	h.peers[role] = conn
	h.mu.Unlock()
	log.I.F("bus: %s connected", role)
}

func (h *busHub) unregister(role string, conn *websocket.Conn) {
	h.mu.Lock()
	if h.peers[role] == conn {
		delete(h.peers, role)
	}
	h.mu.Unlock()
	log.I.F("bus: %s disconnected", role)
}

func (h *busHub) route(to string, raw json.RawMessage, from string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if to == "*" {
		for role, conn := range h.peers {
			if role != from {
				conn.WriteMessage(websocket.TextMessage, raw)
			}
		}
		return
	}
	if conn, ok := h.peers[to]; ok {
		conn.WriteMessage(websocket.TextMessage, raw)
	}
}

func (s *Smesh3Server) handleBus(w http.ResponseWriter, r *http.Request) {
	conn, err := busUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.W.F("bus: upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// First message identifies the peer's role.
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var ident struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(raw, &ident); err != nil || ident.Role == "" {
		log.W.F("bus: invalid ident: %s", string(raw))
		return
	}

	s.bus.register(ident.Role, conn)
	defer s.bus.unregister(ident.Role, conn)

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.W.F("bus: %s read error: %v", ident.Role, err)
			}
			return
		}
		var msg busMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		s.bus.route(msg.To, msg.Msg, ident.Role)
	}
}
