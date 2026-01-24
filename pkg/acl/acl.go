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
	s.Active.Store(m)
	mode.ACLMode.Store(m)
}

type S struct {
	// ACL holds registered ACL implementations.
	// Deprecated: Use GetACLByType() or ListRegisteredACLs() instead of accessing directly.
	ACL []acliface.I
	// Active holds the name of the currently active ACL mode.
	// Deprecated: Use GetMode() instead of Active.Load().
	Active atomic.String
}

// GetMode returns the currently active ACL mode name.
func (s *S) GetMode() string {
	return s.Active.Load()
}

// GetACLByType returns the ACL implementation with the given type name, or nil if not found.
func (s *S) GetACLByType(typ string) acliface.I {
	for _, i := range s.ACL {
		if i.Type() == typ {
			return i
		}
	}
	return nil
}

// GetActiveACL returns the currently active ACL implementation, or nil if none is active.
func (s *S) GetActiveACL() acliface.I {
	return s.GetACLByType(s.Active.Load())
}

// ListRegisteredACLs returns the type names of all registered ACL implementations.
func (s *S) ListRegisteredACLs() []string {
	types := make([]string, 0, len(s.ACL))
	for _, i := range s.ACL {
		types = append(types, i.Type())
	}
	return types
}

// IsRegistered returns true if an ACL with the given type is registered.
func (s *S) IsRegistered(typ string) bool {
	return s.GetACLByType(typ) != nil
}

type A struct{ S }

func (s *S) Register(i acliface.I) {
	(*s).ACL = append((*s).ACL, i)
}

// RegisterAndActivate registers an ACL implementation and sets it as the active one.
// This is used for gRPC clients where the mode is determined by the remote server.
func (s *S) RegisterAndActivate(i acliface.I) {
	s.ACL = []acliface.I{i}
	s.SetMode(i.Type())
}

func (s *S) Configure(cfg ...any) (err error) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			err = i.Configure(cfg...)
			return
		}
	}
	return err
}

func (s *S) GetAccessLevel(pub []byte, address string) (level string) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			level = i.GetAccessLevel(pub, address)
			break
		}
	}
	return
}

func (s *S) GetACLInfo() (name, description, documentation string) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			name, description, documentation = i.GetACLInfo()
			break
		}
	}
	return
}

func (s *S) Syncer() {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			i.Syncer()
			break
		}
	}
}

func (s *S) Type() (typ string) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			typ = i.Type()
			break
		}
	}
	return
}

// AddFollow forwards a pubkey to the active ACL if it supports dynamic follows
func (s *S) AddFollow(pub []byte) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
			if f, ok := i.(*Follows); ok {
				f.AddFollow(pub)
			}
			break
		}
	}
}

// CheckPolicy checks if an event is allowed by the active ACL policy
func (s *S) CheckPolicy(ev *event.E) (allowed bool, err error) {
	for _, i := range s.ACL {
		if i.Type() == s.Active.Load() {
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
