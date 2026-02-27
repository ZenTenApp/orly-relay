// Package acl is an interface for implementing arbitrary access control lists.
package acl

import (
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/interfaces/typer"
)

const (
	None = "none"
	// Read means read only
	Read = "read"
	// Write means read and write
	Write = "write"
	// Admin means read, write, import/export and arbitrary delete
	Admin = "admin"
	// Owner means read, write, import/export, arbitrary delete and wipe
	Owner = "owner"
)

type I interface {
	Configure(cfg ...any) (err error)
	// GetAccessLevel returns the access level string for a given pubkey.
	GetAccessLevel(pub []byte, address string) (level string)
	// GetACLInfo returns the name and a description of the ACL, which should
	// explain briefly how it works, and then a long text of documentation of
	// the ACL's rules and configuration (in asciidoc or markdown).
	GetACLInfo() (name, description, documentation string)
	// Syncer is a worker thread that does things in the background like syncing
	// with other relays on admin relay lists using subscriptions for all events
	// that arrive elsewhere relevant to the ACL scheme.
	Syncer()
	typer.T
}

// PolicyChecker is an optional interface that ACL implementations can implement
// to provide custom event policy checking beyond basic access level checks.
type PolicyChecker interface {
	CheckPolicy(ev *event.E) (allowed bool, err error)
}

// Registry is the interface for the ACL registry that manages ACL implementations.
// This interface enables dependency injection instead of relying on a global singleton.
type Registry interface {
	// GetMode returns the currently active ACL mode name.
	GetMode() string

	// SetMode sets the active ACL mode.
	SetMode(mode string)

	// GetActiveACL returns the currently active ACL implementation.
	GetActiveACL() I

	// GetACLByType returns the ACL implementation with the given type name.
	GetACLByType(typ string) I

	// ACLs returns all registered ACL implementations.
	ACLs() []I

	// ListRegisteredACLs returns the type names of all registered ACLs.
	ListRegisteredACLs() []string

	// Register adds an ACL implementation to the registry.
	Register(i I)

	// Configure configures the active ACL.
	Configure(cfg ...any) error

	// GetAccessLevel returns the access level for a pubkey using the active ACL.
	GetAccessLevel(pub []byte, address string) string

	// CheckPolicy checks if an event is allowed by the active ACL.
	CheckPolicy(ev *event.E) (bool, error)

	// Type returns the type of the active ACL.
	Type() string
}
