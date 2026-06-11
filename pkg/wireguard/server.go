package wireguard

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/tun/netstack"
	"git.smesh.lol/orly/pkg/lol/log"
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

// -- Actor request types --

type wgStartReq struct {
	resp chan error
}

type wgStopReq struct {
	resp chan error
}

type wgIsRunningReq struct {
	resp chan bool
}

type wgGetNetstackReq struct {
	resp chan *netstack.Net
}

type wgAddPeerReq struct {
	nostrPubkey []byte
	wgPublicKey []byte
	assignedIP  string
	resp        chan error
}

type wgRemovePeerReq struct {
	wgPublicKey []byte
	resp        chan error
}

type wgGetPeerReq struct {
	wgPublicKey []byte
	resp        chan wgGetPeerResp
}

type wgGetPeerResp struct {
	peer *Peer
	ok   bool
}

type wgPeerCountReq struct {
	resp chan int
}

// Server manages the embedded WireGuard VPN server.
type Server struct {
	cfg       *Config
	publicKey []byte

	// Single actor channel for all operations
	reqCh chan interface{}

	stop chan struct{}
	done chan struct{}
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

	s := &Server{
		cfg:       cfg,
		publicKey: publicKey,
		reqCh:     make(chan interface{}),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}

	go s.run()

	return s, nil
}

// run is the single actor goroutine that owns all mutable state: device, tun, peers, running.
func (s *Server) run() {
	defer close(s.done)

	var (
		dev     *device.Device
		tunNet  *netstack.Net
		tunDev  tun.Device
		running bool
		cancel  context.CancelFunc
		peers   = make(map[string]*Peer) // WG pubkey (base64) -> Peer
	)

	for {
		select {
		case <-s.stop:
			if running && dev != nil {
				if cancel != nil {
					cancel()
				}
				dev.Close()
			}
			return

		case raw := <-s.reqCh:
			switch req := raw.(type) {

			case wgIsRunningReq:
				req.resp <- running

			case wgGetNetstackReq:
				req.resp <- tunNet

			case wgStartReq:
				if running {
					req.resp <- nil
					continue
				}

				_, cancel = context.WithCancel(context.Background())

				// Parse server IP
				serverAddr, err := netip.ParseAddr(s.cfg.ServerIP)
				if err != nil {
					req.resp <- fmt.Errorf("invalid server IP: %w", err)
					continue
				}

				// Create netstack TUN device (userspace, no root required)
				tunDev, tunNet, err = netstack.CreateNetTUN(
					[]netip.Addr{serverAddr},
					[]netip.Addr{}, // No DNS servers
					1420,           // MTU
				)
				if err != nil {
					req.resp <- fmt.Errorf("failed to create netstack TUN: %w", err)
					continue
				}

				// Create WireGuard device
				dev = device.NewDevice(
					tunDev,
					conn.NewDefaultBind(),
					device.NewLogger(device.LogLevelSilent, "wg"),
				)

				// Configure device with server private key and listen port
				privateKeyHex := hex.EncodeToString(s.cfg.PrivateKey)
				ipcConfig := fmt.Sprintf("private_key=%s\nlisten_port=%d\n",
					privateKeyHex,
					s.cfg.Port,
				)

				if err = dev.IpcSet(ipcConfig); err != nil {
					dev.Close()
					dev = nil
					tunNet = nil
					req.resp <- fmt.Errorf("failed to configure WireGuard device: %w", err)
					continue
				}

				// Bring up the device
				if err = dev.Up(); err != nil {
					dev.Close()
					dev = nil
					tunNet = nil
					req.resp <- fmt.Errorf("failed to bring up WireGuard device: %w", err)
					continue
				}

				running = true
				log.I.F("WireGuard server started on UDP port %d", s.cfg.Port)
				log.I.F("WireGuard server public key: %s", base64.StdEncoding.EncodeToString(s.publicKey))
				log.I.F("WireGuard internal network: %s (server: %s)", s.cfg.Network, s.cfg.ServerIP)
				req.resp <- nil

			case wgStopReq:
				if !running {
					req.resp <- nil
					continue
				}
				if cancel != nil {
					cancel()
				}
				if dev != nil {
					dev.Close()
					dev = nil
					tunNet = nil
				}
				running = false
				log.I.F("WireGuard server stopped")
				req.resp <- nil

			case wgAddPeerReq:
				if !running {
					req.resp <- ErrServerNotRunning
					continue
				}

				wgPubkeyHex := hex.EncodeToString(req.wgPublicKey)
				wgPubkeyBase64 := base64.StdEncoding.EncodeToString(req.wgPublicKey)

				ipcConfig := fmt.Sprintf(
					"public_key=%s\nallowed_ip=%s/32\n",
					wgPubkeyHex,
					req.assignedIP,
				)

				if err := dev.IpcSet(ipcConfig); err != nil {
					req.resp <- fmt.Errorf("failed to add peer: %w", err)
					continue
				}

				peers[wgPubkeyBase64] = &Peer{
					NostrPubkey: req.nostrPubkey,
					WGPublicKey: req.wgPublicKey,
					AssignedIP:  req.assignedIP,
				}
				log.D.F("WireGuard peer added: %s -> %s", wgPubkeyBase64[:16]+"...", req.assignedIP)
				req.resp <- nil

			case wgRemovePeerReq:
				if !running {
					req.resp <- ErrServerNotRunning
					continue
				}

				wgPubkeyHex := hex.EncodeToString(req.wgPublicKey)
				wgPubkeyBase64 := base64.StdEncoding.EncodeToString(req.wgPublicKey)

				ipcConfig := fmt.Sprintf(
					"public_key=%s\nremove=true\n",
					wgPubkeyHex,
				)

				if err := dev.IpcSet(ipcConfig); err != nil {
					req.resp <- fmt.Errorf("failed to remove peer: %w", err)
					continue
				}

				delete(peers, wgPubkeyBase64)
				log.D.F("WireGuard peer removed: %s", wgPubkeyBase64[:16]+"...")
				req.resp <- nil

			case wgGetPeerReq:
				wgPubkeyBase64 := base64.StdEncoding.EncodeToString(req.wgPublicKey)
				peer, ok := peers[wgPubkeyBase64]
				req.resp <- wgGetPeerResp{peer: peer, ok: ok}

			case wgPeerCountReq:
				req.resp <- len(peers)
			}
		}
	}
}

// Start initializes and starts the WireGuard server.
func (s *Server) Start() error {
	resp := make(chan error, 1)
	s.reqCh <- wgStartReq{resp: resp}
	return <-resp
}

// Stop shuts down the WireGuard server.
func (s *Server) Stop() error {
	resp := make(chan error, 1)
	s.reqCh <- wgStopReq{resp: resp}
	err := <-resp
	close(s.stop)
	<-s.done
	return err
}

// IsRunning returns whether the server is currently running.
func (s *Server) IsRunning() bool {
	resp := make(chan bool, 1)
	s.reqCh <- wgIsRunningReq{resp: resp}
	return <-resp
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
	resp := make(chan *netstack.Net, 1)
	s.reqCh <- wgGetNetstackReq{resp: resp}
	return <-resp
}

// ServerIP returns the server's internal IP address.
func (s *Server) ServerIP() string {
	return s.cfg.ServerIP
}

// AddPeer adds a new peer to the WireGuard server.
func (s *Server) AddPeer(nostrPubkey, wgPublicKey []byte, assignedIP string) error {
	resp := make(chan error, 1)
	s.reqCh <- wgAddPeerReq{
		nostrPubkey: nostrPubkey,
		wgPublicKey: wgPublicKey,
		assignedIP:  assignedIP,
		resp:        resp,
	}
	return <-resp
}

// RemovePeer removes a peer from the WireGuard server.
func (s *Server) RemovePeer(wgPublicKey []byte) error {
	resp := make(chan error, 1)
	s.reqCh <- wgRemovePeerReq{
		wgPublicKey: wgPublicKey,
		resp:        resp,
	}
	return <-resp
}

// GetPeer returns a peer by their WireGuard public key.
func (s *Server) GetPeer(wgPublicKey []byte) (*Peer, bool) {
	resp := make(chan wgGetPeerResp, 1)
	s.reqCh <- wgGetPeerReq{
		wgPublicKey: wgPublicKey,
		resp:        resp,
	}
	r := <-resp
	return r.peer, r.ok
}

// PeerCount returns the number of active peers.
func (s *Server) PeerCount() int {
	resp := make(chan int, 1)
	s.reqCh <- wgPeerCountReq{resp: resp}
	return <-resp
}
