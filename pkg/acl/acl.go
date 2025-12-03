package acl

import (
	"git.mleku.dev/mleku/nostr/encoders/event"
	acliface "next.orly.dev/pkg/interfaces/acl"
	"next.orly.dev/pkg/utils/atomic"
)

var Registry = &S{}

type S struct {
	ACL    []acliface.I
	Active atomic.String
}

type A struct{ S }

func (s *S) Register(i acliface.I) {
	(*s).ACL = append((*s).ACL, i)
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
