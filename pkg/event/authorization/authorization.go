// Package authorization provides event authorization services for the ORLY relay.
// It handles ACL checks, policy evaluation, and access level decisions.
package authorization

import (
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/hex"
)

// Decision carries authorization context through the event processing pipeline.
type Decision struct {
	Allowed      bool
	AccessLevel  string // none/read/write/admin/owner/blocked/banned
	IsAdmin      bool
	IsOwner      bool
	IsPeerRelay  bool
	SkipACLCheck bool   // For admin/owner deletes
	DenyReason   string // Human-readable reason for denial
	RequireAuth  bool   // Should send AUTH challenge
}

// Allow returns an allowed decision with the given access level.
func Allow(accessLevel string) Decision {
	return Decision{
		Allowed:     true,
		AccessLevel: accessLevel,
	}
}

// Deny returns a denied decision with the given reason.
func Deny(reason string, requireAuth bool) Decision {
	return Decision{
		Allowed:     false,
		DenyReason:  reason,
		RequireAuth: requireAuth,
	}
}

// Authorizer makes authorization decisions for events.
type Authorizer interface {
	// Authorize checks if event is allowed based on ACL and policy.
	Authorize(ev *event.E, authedPubkey []byte, remote string, eventKind uint16) Decision
}

// ACLRegistry abstracts the ACL registry for authorization checks.
type ACLRegistry interface {
	// GetAccessLevel returns the access level for a pubkey and remote address.
	GetAccessLevel(pub []byte, address string) string
	// CheckPolicy checks if an event passes ACL policy.
	CheckPolicy(ev *event.E) (bool, error)
	// Active returns the active ACL mode name.
	Active() string
}

// PolicyManager abstracts the policy manager for authorization checks.
type PolicyManager interface {
	// IsEnabled returns whether policy is enabled.
	IsEnabled() bool
	// CheckPolicy checks if an action is allowed by policy.
	CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error)
}

// SyncManager abstracts the sync manager for peer relay checking.
type SyncManager interface {
	// GetPeers returns the list of peer relay URLs.
	GetPeers() []string
	// IsAuthorizedPeer checks if a pubkey is an authorized peer.
	IsAuthorizedPeer(url, pubkey string) bool
}

// Config holds configuration for the authorization service.
type Config struct {
	AuthRequired    bool     // Whether auth is required for all operations
	AuthToWrite     bool     // Whether auth is required for write operations
	NIP46BypassAuth bool     // Allow NIP-46 events through without auth
	Admins          [][]byte // Admin pubkeys
	Owners          [][]byte // Owner pubkeys
}

// Service implements the Authorizer interface.
type Service struct {
	cfg         *Config
	acl         ACLRegistry
	policy      PolicyManager
	sync        SyncManager
}

// New creates a new authorization service.
func New(cfg *Config, acl ACLRegistry, policy PolicyManager, sync SyncManager) *Service {
	return &Service{
		cfg:    cfg,
		acl:    acl,
		policy: policy,
		sync:   sync,
	}
}

// Authorize checks if event is allowed based on ACL and policy.
func (s *Service) Authorize(ev *event.E, authedPubkey []byte, remote string, eventKind uint16) Decision {
	// Check if peer relay - they get special treatment
	if s.isPeerRelayPubkey(authedPubkey) {
		return Decision{
			Allowed:     true,
			AccessLevel: "admin",
			IsPeerRelay: true,
		}
	}

	// Check policy if enabled
	if s.policy != nil && s.policy.IsEnabled() {
		allowed, err := s.policy.CheckPolicy("write", ev, authedPubkey, remote)
		if err != nil {
			return Deny("policy check failed", false)
		}
		if !allowed {
			return Deny("event blocked by policy", false)
		}

		// Check ACL policy for managed ACL mode
		if s.acl != nil && s.acl.Active() == "managed" {
			allowed, err := s.acl.CheckPolicy(ev)
			if err != nil {
				return Deny("ACL policy check failed", false)
			}
			if !allowed {
				return Deny("event blocked by ACL policy", false)
			}
		}
	}

	// Determine pubkey for ACL check
	pubkeyForACL := authedPubkey
	if len(authedPubkey) == 0 && s.acl != nil && s.acl.Active() == "none" &&
		!s.cfg.AuthRequired && !s.cfg.AuthToWrite {
		pubkeyForACL = ev.Pubkey
	}

	// Check if auth is required but user not authenticated
	// NIP-46 bunker events (kind 24133) can bypass auth if configured
	const kindNIP46 = 24133
	if (s.cfg.AuthRequired || s.cfg.AuthToWrite) && len(authedPubkey) == 0 {
		if !(s.cfg.NIP46BypassAuth && eventKind == kindNIP46) {
			return Deny("authentication required for write operations", true)
		}
	}

	// Get access level
	accessLevel := "write" // Default for none mode
	if s.acl != nil {
		accessLevel = s.acl.GetAccessLevel(pubkeyForACL, remote)
	}

	// Check if admin/owner for delete events (skip ACL check)
	isAdmin := s.isAdmin(ev.Pubkey)
	isOwner := s.isOwner(ev.Pubkey)
	skipACL := (isAdmin || isOwner) && eventKind == 5 // kind 5 = deletion

	decision := Decision{
		AccessLevel:  accessLevel,
		IsAdmin:      isAdmin,
		IsOwner:      isOwner,
		SkipACLCheck: skipACL,
	}

	// Handle access levels
	if !skipACL {
		switch accessLevel {
		case "none":
			decision.Allowed = false
			decision.DenyReason = "auth required for write access"
			decision.RequireAuth = true
		case "read":
			decision.Allowed = false
			decision.DenyReason = "auth required for write access"
			decision.RequireAuth = true
		case "blocked":
			decision.Allowed = false
			decision.DenyReason = "IP address blocked"
		case "banned":
			decision.Allowed = false
			decision.DenyReason = "pubkey banned"
		default:
			// write/admin/owner - allowed
			decision.Allowed = true
		}
	} else {
		decision.Allowed = true
	}

	return decision
}

// isPeerRelayPubkey checks if the given pubkey belongs to a peer relay.
func (s *Service) isPeerRelayPubkey(pubkey []byte) bool {
	if s.sync == nil || len(pubkey) == 0 {
		return false
	}

	peerPubkeyHex := hex.Enc(pubkey)

	for _, peerURL := range s.sync.GetPeers() {
		if s.sync.IsAuthorizedPeer(peerURL, peerPubkeyHex) {
			return true
		}
	}

	return false
}

// isAdmin checks if a pubkey is an admin.
func (s *Service) isAdmin(pubkey []byte) bool {
	for _, admin := range s.cfg.Admins {
		if fastEqual(admin, pubkey) {
			return true
		}
	}
	return false
}

// isOwner checks if a pubkey is an owner.
func (s *Service) isOwner(pubkey []byte) bool {
	for _, owner := range s.cfg.Owners {
		if fastEqual(owner, pubkey) {
			return true
		}
	}
	return false
}

// fastEqual compares two byte slices for equality.
func fastEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
