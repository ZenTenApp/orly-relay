package wireguard

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"sync"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"next.orly.dev/pkg/lol/log"
)

// Config holds the WireGuard server configuration.
type Config struct {
	Port       int    // UDP port for WireGuard (default 51820)
	Endpoint   string // Public IP/domain for clients to connect to
	PrivateKey []byte // Server's 32-byte Curve25519 private key
	Network    string // CIDR for internal network (e.g., "10.73.0.0/16")
	ServerIP   string // Server's internal IP (e.g., "10.73.0.1")
}

// Peer represents a WireGuard peer (client).
type Peer struct {
	NostrPubkey []byte // User's Nostr pubkey (32 bytes)
	WGPublicKey []byte // WireGuard public key (32 bytes)
	AssignedIP  string // Assigned internal IP
}

// Server manages the embedded WireGuard VPN server.
type Server struct {
	cfg       *Config
	device    *device.Device
	tun       *netstack.Net
	tunDev    tun.Device
	publicKey []byte

	peers   map[string]*Peer // WG pubkey (base64) -> Peer
	peersMu sync.RWMutex

	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	mu      sync.RWMutex
}

// New creates a new WireGuard server with the given configuration.
func New(cfg *Config) (*Server, error) {
	if cfg.Endpoint == "" {
		return nil, ErrEndpointRequired
	}

	// Parse network CIDR to validate it
	_, _, err := net.ParseCIDR(cfg.Network)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidNetwork, err)
	}

	// Default server IP if not set
	if cfg.ServerIP == "" {
		cfg.ServerIP = "10.73.0.1"
	}

	// Derive public key from private key
	publicKey, err := DerivePublicKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	return &Server{
		cfg:       cfg,
		publicKey: publicKey,
		peers:     make(map[string]*Peer),
	}, nil
}

// Start initializes and starts the WireGuard server.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Parse server IP
	serverAddr, err := netip.ParseAddr(s.cfg.ServerIP)
	if err != nil {
		return fmt.Errorf("invalid server IP: %w", err)
	}

	// Create netstack TUN device (userspace, no root required)
	s.tunDev, s.tun, err = netstack.CreateNetTUN(
		[]netip.Addr{serverAddr},
		[]netip.Addr{}, // No DNS servers
		1420,           // MTU
	)
	if err != nil {
		return fmt.Errorf("failed to create netstack TUN: %w", err)
	}

	// Create WireGuard device
	s.device = device.NewDevice(
		s.tunDev,
		conn.NewDefaultBind(),
		device.NewLogger(device.LogLevelSilent, "wg"),
	)

	// Configure device with server private key and listen port
	privateKeyHex := hex.EncodeToString(s.cfg.PrivateKey)
	ipcConfig := fmt.Sprintf("private_key=%s\nlisten_port=%d\n",
		privateKeyHex,
		s.cfg.Port,
	)

	if err = s.device.IpcSet(ipcConfig); err != nil {
		s.device.Close()
		return fmt.Errorf("failed to configure WireGuard device: %w", err)
	}

	// Bring up the device
	if err = s.device.Up(); err != nil {
		s.device.Close()
		return fmt.Errorf("failed to bring up WireGuard device: %w", err)
	}

	s.running = true
	log.I.F("WireGuard server started on UDP port %d", s.cfg.Port)
	log.I.F("WireGuard server public key: %s", base64.StdEncoding.EncodeToString(s.publicKey))
	log.I.F("WireGuard internal network: %s (server: %s)", s.cfg.Network, s.cfg.ServerIP)

	return nil
}

// Stop shuts down the WireGuard server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}

	if s.device != nil {
		s.device.Close()
	}

	s.running = false
	log.I.F("WireGuard server stopped")

	return nil
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ServerPublicKey returns the server's WireGuard public key.
func (s *Server) ServerPublicKey() []byte {
	return s.publicKey
}

// Endpoint returns the configured endpoint address.
func (s *Server) Endpoint() string {
	return fmt.Sprintf("%s:%d", s.cfg.Endpoint, s.cfg.Port)
}

// GetNetstack returns the netstack networking interface.
// This is used by the bunker to listen on the WireGuard network.
func (s *Server) GetNetstack() *netstack.Net {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tun
}

// ServerIP returns the server's internal IP address.
func (s *Server) ServerIP() string {
	return s.cfg.ServerIP
}

// AddPeer adds a new peer to the WireGuard server.
func (s *Server) AddPeer(nostrPubkey, wgPublicKey []byte, assignedIP string) error {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return ErrServerNotRunning
	}
	s.mu.RUnlock()

	// Encode WG public key as hex for IPC
	wgPubkeyHex := hex.EncodeToString(wgPublicKey)
	wgPubkeyBase64 := base64.StdEncoding.EncodeToString(wgPublicKey)

	// Configure peer in WireGuard device
	ipcConfig := fmt.Sprintf(
		"public_key=%s\nallowed_ip=%s/32\n",
		wgPubkeyHex,
		assignedIP,
	)

	if err := s.device.IpcSet(ipcConfig); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	// Track peer
	s.peersMu.Lock()
	s.peers[wgPubkeyBase64] = &Peer{
		NostrPubkey: nostrPubkey,
		WGPublicKey: wgPublicKey,
		AssignedIP:  assignedIP,
	}
	s.peersMu.Unlock()

	log.I.F("WireGuard peer added: %s -> %s", wgPubkeyBase64[:16]+"...", assignedIP)

	return nil
}

// RemovePeer removes a peer from the WireGuard server.
func (s *Server) RemovePeer(wgPublicKey []byte) error {
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return ErrServerNotRunning
	}
	s.mu.RUnlock()

	wgPubkeyHex := hex.EncodeToString(wgPublicKey)
	wgPubkeyBase64 := base64.StdEncoding.EncodeToString(wgPublicKey)

	// Remove peer from WireGuard device
	ipcConfig := fmt.Sprintf(
		"public_key=%s\nremove=true\n",
		wgPubkeyHex,
	)

	if err := s.device.IpcSet(ipcConfig); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	// Remove from tracking
	s.peersMu.Lock()
	delete(s.peers, wgPubkeyBase64)
	s.peersMu.Unlock()

	log.I.F("WireGuard peer removed: %s", wgPubkeyBase64[:16]+"...")

	return nil
}

// GetPeer returns a peer by their WireGuard public key.
func (s *Server) GetPeer(wgPublicKey []byte) (*Peer, bool) {
	wgPubkeyBase64 := base64.StdEncoding.EncodeToString(wgPublicKey)

	s.peersMu.RLock()
	defer s.peersMu.RUnlock()

	peer, ok := s.peers[wgPubkeyBase64]
	return peer, ok
}

// PeerCount returns the number of active peers.
func (s *Server) PeerCount() int {
	s.peersMu.RLock()
	defer s.peersMu.RUnlock()
	return len(s.peers)
}
