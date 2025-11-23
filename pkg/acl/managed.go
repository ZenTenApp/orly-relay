package acl

import (
	"context"
	"encoding/hex"
	"net"
	"reflect"
	"sync"

	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"next.orly.dev/pkg/utils"
)

type Managed struct {
	Ctx context.Context
	cfg *config.C
	*database.D
	managedACL *database.ManagedACL
	owners     [][]byte
	admins     [][]byte
	peerAdmins [][]byte // peer relay identity pubkeys with admin access
	mx         sync.RWMutex
}

func (m *Managed) Configure(cfg ...any) (err error) {
	log.I.F("configuring managed ACL")
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			m.cfg = c
		case *database.D:
			m.D = c
			m.managedACL = database.NewManagedACL(c)
		case context.Context:
			m.Ctx = c
		default:
			err = errorf.E("invalid type: %T", reflect.TypeOf(ca))
		}
	}
	if m.cfg == nil || m.D == nil {
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

	return
}

// UpdatePeerAdmins updates the list of peer relay identity pubkeys that have admin access
func (m *Managed) UpdatePeerAdmins(peerPubkeys [][]byte) {
	m.mx.Lock()
	defer m.mx.Unlock()
	m.peerAdmins = make([][]byte, len(peerPubkeys))
	copy(m.peerAdmins, peerPubkeys)
	log.I.F("updated peer admin list with %d pubkeys", len(peerPubkeys))
}

func (m *Managed) GetAccessLevel(pub []byte, address string) (level string) {
	m.mx.RLock()
	defer m.mx.RUnlock()

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
	for _, v := range m.peerAdmins {
		if utils.FastEqual(v, pub) {
			return "admin"
		}
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

func (m *Managed) CheckPolicy(ev *event.E) (allowed bool, err error) {
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
