package acl

import (
	"context"
	"encoding/hex"
	"net"
	"reflect"

	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/bech32encoding"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/utils"
)

// managedUpdatePeerAdminsReq updates the peer admin list.
type managedUpdatePeerAdminsReq struct {
	peerPubkeys [][]byte
	resp        chan struct{}
}

// managedGetAccessLevelReq queries the access level for a pubkey.
type managedGetAccessLevelReq struct {
	pub     []byte
	address string
	resp    chan string
}

type Managed struct {
	// Ctx holds the context for the ACL.
	// Deprecated: Use Context() method instead of accessing directly.
	Ctx context.Context
	cfg *config.C
	db  database.Database
	managedACL *database.ManagedACL
	owners     [][]byte
	admins     [][]byte

	updatePeerAdminsCh chan managedUpdatePeerAdminsReq
	getAccessLevelCh   chan managedGetAccessLevelReq
	stop               chan struct{}
	done               chan struct{}
}

// Context returns the ACL context.
func (m *Managed) Context() context.Context {
	return m.Ctx
}

func (m *Managed) Configure(cfg ...any) (err error) {
	log.I.F("configuring managed ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			m.cfg = c
		case database.Database:
			m.db = c
			// ManagedACL requires the concrete Badger database type
			// Type assertion to check if it's a Badger database
			if d, ok := c.(*database.D); ok {
				m.managedACL = database.NewManagedACL(d)
			} else {
				log.W.F("managed ACL: database is not Badger, managed ACL features will be limited")
			}
		case context.Context:
			m.Ctx = c
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if m.cfg == nil || m.db == nil {
		err = errorf.E("both config and database must be set")
		return
	}

	// Load owners
	for _, owner := range m.cfg.Owners {
		if len(owner) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(owner); err != nil {
			continue
		}
		m.owners = append(m.owners, pk)
	}

	// Load admins
	for _, admin := range m.cfg.Admins {
		if len(admin) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(admin); err != nil {
			continue
		}
		m.admins = append(m.admins, pk)
	}

	// Start actor goroutine
	m.updatePeerAdminsCh = make(chan managedUpdatePeerAdminsReq)
	m.getAccessLevelCh = make(chan managedGetAccessLevelReq)
	m.stop = make(chan struct{})
	m.done = make(chan struct{})
	go m.actor()

	return
}

func (m *Managed) actor() {
	defer close(m.done)

	var peerAdmins [][]byte

	for {
		select {
		case <-m.stop:
			return

		case req := <-m.updatePeerAdminsCh:
			peerAdmins = make([][]byte, len(req.peerPubkeys))
			copy(peerAdmins, req.peerPubkeys)
			log.I.F("updated peer admin list with %d pubkeys", len(req.peerPubkeys))
			req.resp <- struct{}{}

		case req := <-m.getAccessLevelCh:
			req.resp <- m.getAccessLevel(req.pub, req.address, peerAdmins)
		}
	}
}

func (m *Managed) getAccessLevel(pub []byte, address string, peerAdmins [][]byte) string {
	// If no pubkey provided and auth is required, return "none"
	if len(pub) == 0 && m.cfg.AuthRequired {
		return "none"
	}

	// Check owners first
	for _, v := range m.owners {
		if utils.FastEqual(v, pub) {
			return "owner"
		}
	}

	// Check admins
	for _, v := range m.admins {
		if utils.FastEqual(v, pub) {
			return "admin"
		}
	}

	// Check peer relay identity pubkeys (they get admin access)
	for _, v := range peerAdmins {
		if utils.FastEqual(v, pub) {
			return "admin"
		}
	}

	// managedACL may be nil when database is not Badger (e.g., gRPC proxy).
	// Fall through to default read access in that case.
	if m.managedACL == nil {
		if len(pub) == 0 {
			return "none"
		}
		return "read"
	}

	// Check if pubkey is banned
	pubkeyHex := hex.EncodeToString(pub)
	if banned, err := m.managedACL.IsPubkeyBanned(pubkeyHex); err == nil && banned {
		return "banned"
	}

	// Check if pubkey is explicitly allowed
	if allowed, err := m.managedACL.IsPubkeyAllowed(pubkeyHex); err == nil && allowed {
		return "write"
	}

	// Check if IP is blocked
	if blocked, err := m.managedACL.IsIPBlocked(address); err == nil && blocked {
		return "blocked"
	}

	// Default to read-only for managed mode
	return "read"
}

// UpdatePeerAdmins updates the list of peer relay identity pubkeys that have admin access
func (m *Managed) UpdatePeerAdmins(peerPubkeys [][]byte) {
	resp := make(chan struct{}, 1)
	m.updatePeerAdminsCh <- managedUpdatePeerAdminsReq{peerPubkeys: peerPubkeys, resp: resp}
	<-resp
}

func (m *Managed) GetAccessLevel(pub []byte, address string) (level string) {
	resp := make(chan string, 1)
	m.getAccessLevelCh <- managedGetAccessLevelReq{pub: pub, address: address, resp: resp}
	return <-resp
}

func (m *Managed) CheckPolicy(ev *event.E) (allowed bool, err error) {
	// If managedACL is nil (non-Badger DB), allow everything
	if m.managedACL == nil {
		return true, nil
	}

	// Check if event is banned
	eventID := hex.EncodeToString(ev.ID)
	if banned, err := m.managedACL.IsEventBanned(eventID); err == nil && banned {
		return false, nil
	}

	// Check if event is explicitly allowed
	if allowed, err := m.managedACL.IsEventAllowed(eventID); err == nil && allowed {
		return true, nil
	}

	// Check if event kind is allowed
	if allowed, err := m.managedACL.IsKindAllowed(int(ev.Kind)); err == nil && !allowed {
		// If there are allowed kinds configured and this kind is not in the list, deny
		allowedKinds, err := m.managedACL.ListAllowedKinds()
		if err == nil && len(allowedKinds) > 0 {
			return false, nil
		}
	}

	// Check if author is banned
	authorHex := hex.EncodeToString(ev.Pubkey)
	if banned, err := m.managedACL.IsPubkeyBanned(authorHex); err == nil && banned {
		return false, nil
	}

	// Check if author is explicitly allowed
	if allowed, err := m.managedACL.IsPubkeyAllowed(authorHex); err == nil && allowed {
		return true, nil
	}

	// For managed mode, default to allowing events from owners and admins
	for _, v := range m.owners {
		if utils.FastEqual(v, ev.Pubkey) {
			return true, nil
		}
	}

	for _, v := range m.admins {
		if utils.FastEqual(v, ev.Pubkey) {
			return true, nil
		}
	}

	// Check if we should add this event to moderation queue
	// This could be extended to add events to moderation based on content analysis
	// For now, we'll just allow the event

	// Default to allowing events in managed mode (can be restricted by explicit bans/allows)
	return true, nil
}

func (m *Managed) GetACLInfo() (name, description, documentation string) {
	return "managed", "managed ACL with NIP-86 support",
		`Managed ACL mode provides fine-grained access control through NIP-86 management API.

Features:
- Ban/allow specific pubkeys
- Ban/allow specific events
- Block IP addresses
- Allow/deny specific event kinds
- Relay metadata management
- Event moderation queue

This mode requires explicit management through the NIP-86 API endpoints.
Only relay owners can access the management interface and API.`
}

func (m *Managed) Type() string {
	return "managed"
}

func (m *Managed) Syncer() {
	// Managed ACL doesn't need background syncing
	// All management is done through the API
}

// Helper methods for the management API

// IsIPBlocked checks if an IP address is blocked
func (m *Managed) IsIPBlocked(ip string) bool {
	// Parse IP to handle both IPv4 and IPv6
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	blocked, err := m.managedACL.IsIPBlocked(ip)
	if err != nil {
		log.W.F("error checking if IP is blocked: %v", err)
		return false
	}
	return blocked
}

// GetManagedACL returns the managed ACL database instance
func (m *Managed) GetManagedACL() *database.ManagedACL {
	return m.managedACL
}

func init() {
	Registry.Register(new(Managed))
}
