package acl

import (
	"git.mleku.dev/mleku/nostr/encoders/event"
	acliface "next.orly.dev/pkg/interfaces/acl"
	"next.orly.dev/pkg/mode"
	"next.orly.dev/pkg/utils/atomic"
)

var Registry = &S{}

// SetMode sets the active ACL mode and syncs it to the mode package for
// packages that need to check the mode without importing acl (to avoid cycles).
func (s *S) SetMode(m string) {
	s.active.Store(m)
	mode.ACLMode.Store(m)
}

type S struct {
	// acl holds registered ACL implementations.
	// Use ACLs(), GetACLByType(), or ListRegisteredACLs() to access.
	acl []acliface.I
	// active holds the name of the currently active ACL mode.
	// Use GetMode() to read the current mode.
	active atomic.String
}

// GetMode returns the currently active ACL mode name.
func (s *S) GetMode() string {
	return s.active.Load()
}

// GetACLByType returns the ACL implementation with the given type name, or nil if not found.
func (s *S) GetACLByType(typ string) acliface.I {
	for _, i := range s.acl {
		if i.Type() == typ {
			return i
		}
	}
	return nil
}

// GetActiveACL returns the currently active ACL implementation, or nil if none is active.
func (s *S) GetActiveACL() acliface.I {
	return s.GetACLByType(s.active.Load())
}

// ListRegisteredACLs returns the type names of all registered ACL implementations.
func (s *S) ListRegisteredACLs() []string {
	types := make([]string, 0, len(s.acl))
	for _, i := range s.acl {
		types = append(types, i.Type())
	}
	return types
}

// ACLs returns the registered ACL implementations for iteration.
// Prefer using GetActiveACL() or GetACLByType() when possible.
func (s *S) ACLs() []acliface.I {
	return s.acl
}

// IsRegistered returns true if an ACL with the given type is registered.
func (s *S) IsRegistered(typ string) bool {
	return s.GetACLByType(typ) != nil
}

type A struct{ S }

func (s *S) Register(i acliface.I) {
	(*s).acl = append((*s).acl, i)
}

// RegisterAndActivate registers an ACL implementation and sets it as the active one.
// This is used for gRPC clients where the mode is determined by the remote server.
func (s *S) RegisterAndActivate(i acliface.I) {
	s.acl = []acliface.I{i}
	s.SetMode(i.Type())
}

func (s *S) Configure(cfg ...any) (err error) {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			err = i.Configure(cfg...)
			return
		}
	}
	return err
}

func (s *S) GetAccessLevel(pub []byte, address string) (level string) {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			level = i.GetAccessLevel(pub, address)
			break
		}
	}
	return
}

func (s *S) GetACLInfo() (name, description, documentation string) {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			name, description, documentation = i.GetACLInfo()
			break
		}
	}
	return
}

func (s *S) Syncer() {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			i.Syncer()
			break
		}
	}
}

func (s *S) Type() (typ string) {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			typ = i.Type()
			break
		}
	}
	return
}

// AddFollow forwards a pubkey to the active ACL if it supports dynamic follows
func (s *S) AddFollow(pub []byte) {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			if f, ok := i.(*Follows); ok {
				f.AddFollow(pub)
			}
			break
		}
	}
}

// CheckPolicy checks if an event is allowed by the active ACL policy
func (s *S) CheckPolicy(ev *event.E) (allowed bool, err error) {
	for _, i := range s.acl {
		if i.Type() == s.active.Load() {
			// Check if the ACL implementation has a CheckPolicy method
			if policyChecker, ok := i.(acliface.PolicyChecker); ok {
				return policyChecker.CheckPolicy(ev)
			}
			// If no CheckPolicy method, default to allowing
			return true, nil
		}
	}
	// If no active ACL, default to allowing
	return true, nil
}
