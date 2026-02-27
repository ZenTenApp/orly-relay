package acl

import (
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/utils"
)

type None struct {
	cfg    *config.C
	owners [][]byte
	admins [][]byte
}

func (n *None) Configure(cfg ...any) (err error) {
	for _, ca := range cfg {
		switch c := ca.(type) {
		case *config.C:
			n.cfg = c
		}
	}
	if n.cfg == nil {
		return
	}

	// Load owners
	for _, owner := range n.cfg.Owners {
		if len(owner) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(owner); err != nil {
			continue
		}
		n.owners = append(n.owners, pk)
	}

	// Load admins
	for _, admin := range n.cfg.Admins {
		if len(admin) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(admin); err != nil {
			continue
		}
		n.admins = append(n.admins, pk)
	}

	return
}

func (n *None) GetAccessLevel(pub []byte, address string) (level string) {
	// In serve mode, grant full owner access to everyone
	if n.cfg != nil && n.cfg.ServeMode {
		return "owner"
	}

	// Check owners first
	for _, v := range n.owners {
		if utils.FastEqual(v, pub) {
			return "owner"
		}
	}

	// Check admins
	for _, v := range n.admins {
		if utils.FastEqual(v, pub) {
			return "admin"
		}
	}

	// Default to write for everyone else
	return "write"
}

func (n None) GetACLInfo() (name, description, documentation string) {
	return "none", "no ACL", "blanket write access for all clients"
}

func (n None) Type() string {
	return "none"
}

func (n None) CheckPolicy(ev *event.E) (allowed bool, err error) {
	return true, nil
}

func (n None) Syncer() {}

func init() {
	Registry.Register(new(None))
}
