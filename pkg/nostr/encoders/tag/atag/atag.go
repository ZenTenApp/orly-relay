// Package atag implements a special, optimized handling for keeping a tags
// (address) in a more memory efficient form while working with these tags.
package atag

import (
	"bytes"

	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/ints"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/lol/chk"
)

// T is a data structure for what is found in an `a` tag: kind:pubkey:arbitrary data
type T struct {
	Kind   *kind.K
	Pubkey []byte
	DTag   []byte
}

// Marshal an atag.T into raw bytes.
func (t *T) Marshal(dst []byte) (b []byte) {
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	// Estimate: kind (max 10 chars) + ':' + hex pubkey (64 chars) + ':' + dtag
	if b == nil {
		estimatedSize := 10 + 1 + 64 + 1 + len(t.DTag)
		b = make([]byte, 0, estimatedSize)
	}
	b = t.Kind.Marshal(b)
	b = append(b, ':')
	b = hex.EncAppend(b, t.Pubkey)
	b = append(b, ':')
	b = append(b, t.DTag...)
	return
}

// Unmarshal an atag.T from its ascii encoding.
func (t *T) Unmarshal(b []byte) (r []byte, err error) {
	split := bytes.Split(b, []byte{':'})
	if len(split) != 3 {
		return
	}
	// kind
	kin := ints.New(uint16(0))
	if _, err = kin.Unmarshal(split[0]); chk.E(err) {
		return
	}
	t.Kind = kind.New(kin.Uint16())
	// pubkey
	if t.Pubkey, err = hex.DecAppend(t.Pubkey, split[1]); chk.E(err) {
		return
	}
	// d-tag
	t.DTag = split[2]
	return
}
