// Package bunker provides a NIP-46 remote signing service that listens
// only on the WireGuard VPN network for secure access.
package bunker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"github.com/gorilla/websocket"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type bunkerRegisterArgs struct {
	ID      string
	Session *Session
}

// Server is the NIP-46 bunker server.
type Server struct {
	relaySigner signer.I      // Relay's signer for signing events
	relayPubkey []byte        // Relay's public key
	netstack    *netstack.Net // WireGuard netstack for listening
	listenAddr  string        // e.g., "10.73.0.1:3335"

	register   actor.Inbox[bunkerRegisterArgs]
	unregister actor.Inbox[string]
	count      actor.Query[int]
	lc         actor.Lifecycle

	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// Config holds bunker server configuration.
type Config struct {
	RelaySigner signer.I
	RelayPubkey []byte
	Netstack    *netstack.Net
	ListenAddr  string // IP:port on WireGuard network
}

// New creates a new bunker server.
func New(cfg *Config) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		relaySigner: cfg.RelaySigner,
		relayPubkey: cfg.RelayPubkey,
		netstack:    cfg.Netstack,
		listenAddr:  cfg.ListenAddr,
		register:    actor.NewInbox[bunkerRegisterArgs](16),
		unregister:  actor.NewInbox[string](16),
		count:       actor.NewQuery[int](),
		lc:          actor.NewLifecycle(),
		ctx:         ctx,
		cancel:      cancel,
	}

	actor.Go(s.lc, s.actorLoop)

	return s
}

func (s *Server) actorLoop() {
	sessions := make(map[string]*Session)
	for {
		select {
		case <-s.lc.Stopping():
			return
		case args := <-s.register.Recv():
			sessions[args.ID] = args.Session
		case id := <-s.unregister.Recv():
			delete(sessions, id)
		case msg := <-s.count.Recv():
			msg.Reply(len(sessions))
		}
	}
}

// Start begins listening for bunker connections on the WireGuard network.
func (s *Server) Start() error {
	// Parse listen address
	host, port, err := net.SplitHostPort(s.listenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen address: %w", err)
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", host)
	}

	portNum := 0
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		return fmt.Errorf("invalid port: %s", port)
	}

	// Create TCP listener on netstack (WireGuard network only)
	listener, err := s.netstack.ListenTCP(&net.TCPAddr{
		IP:   ip,
		Port: portNum,
	})
	if err != nil {
		return fmt.Errorf("failed to listen on netstack: %w", err)
	}

	// Create HTTP server with WebSocket handler
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleWebSocket)

	s.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.E.F("bunker server error: %v", err)
		}
	}()

	log.I.F("NIP-46 bunker server started on %s (WireGuard only)", s.listenAddr)
	return nil
}

// Stop shuts down the bunker server.
func (s *Server) Stop() error {
	s.cancel()

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); chk.E(err) {
			return err
		}
	}

	s.lc.Stop()
	log.I.F("NIP-46 bunker server stopped")
	return nil
}

// handleWebSocket handles WebSocket connections for NIP-46.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.E.F("bunker websocket upgrade failed: %v", err)
		return
	}

	session := NewSession(s.ctx, conn, s.relaySigner, s.relayPubkey)

	// Register session
	s.register.TrySend(bunkerRegisterArgs{ID: session.ID, Session: session})

	// Handle session
	session.Handle()

	// Unregister session
	s.unregister.TrySend(session.ID)
}

// SessionCount returns the number of active sessions.
func (s *Server) SessionCount() int {
	return s.count.Call()
}

// RelayPubkeyHex returns the relay's public key as hex.
func (s *Server) RelayPubkeyHex() string {
	return fmt.Sprintf("%x", s.relayPubkey)
}

// NIP46Request represents a NIP-46 request from a client.
type NIP46Request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// NIP46Response represents a NIP-46 response to a client.
type NIP46Response struct {
	ID     string `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}
