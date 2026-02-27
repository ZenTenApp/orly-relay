// Package kinds is a set of helpers for dealing with lists of kind numbers
// including comparisons and encoding.
package kind

import (
	"git.mleku.dev/mleku/nostr/encoders/ints"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
)

// S is an array of kind.K, used in filter.K and filter.S for searches.
type S struct {
	K []*K
}

// NewS creates a new kinds.S, if no parameter is given it just creates an empty zero kinds.S.
func NewS(k ...*K) *S { return &S{k} }

// NewWithCap creates a new empty kinds.S with a given slice capacity.
func NewWithCap(c int) *S { return &S{make([]*K, 0, c)} }

// FromIntSlice converts a []int into a kinds.S.
func FromIntSlice(is []int) (k *S) {
	k = &S{}
	for i := range is {
		k.K = append(k.K, New(uint16(is[i])))
	}
	return
}

// Len returns the number of elements in a kinds.S.
func (k *S) Len() (l int) {
	if k == nil {
		return
	}
	return len(k.K)
}

// Less returns which of two elements of a kinds.S is lower.
func (k *S) Less(i, j int) bool { return k.K[i].K < k.K[j].K }

// Swap switches the position of two kinds.S elements.
func (k *S) Swap(i, j int) {
	k.K[i].K, k.K[j].K = k.K[j].K, k.K[i].K
}

// ToUint16 returns a []uint16 version of the kinds.S.
func (k *S) ToUint16() (o []uint16) {
	for i := range k.K {
		o = append(o, k.K[i].ToU16())
	}
	return
}

// Clone makes a new kind.K with the same members.
func (k *S) Clone() (c *S) {
	c = &S{K: make([]*K, len(k.K))}
	for i := range k.K {
		c.K[i] = k.K[i]
	}
	return
}

// Contains returns true if the provided element is found in the kinds.S.
//
// Note that the request must use the typed kind.K or convert the number thus.
// Even if a custom number is found, this codebase does not have the logic to
// deal with the kind so such a search is pointless and for which reason static
// typing always wins. No mistakes possible with known quantities.
func (k *S) Contains(s uint16) bool {
	for i := range k.K {
		if k.K[i].Equal(s) {
			return true
		}
	}
	return false
}

// Equals checks that the provided kind.K matches.
func (k *S) Equals(t1 *S) bool {
	if len(k.K) != len(t1.K) {
		return false
	}
	for i := range k.K {
		if k.K[i] != t1.K[i] {
			return false
		}
	}
	return true
}

// Marshal renders the kinds.S into a JSON array of integers.
func (k *S) Marshal(dst []byte) (b []byte) {
	b = dst
	b = append(b, '[')
	for i := range k.K {
		b = k.K[i].Marshal(b)
		if i != len(k.K)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

// Unmarshal decodes a provided JSON array of integers into a kinds.S.
func (k *S) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	var openedBracket bool
	for ; len(r) > 0; r = r[1:] {
		if !openedBracket && r[0] == '[' {
			openedBracket = true
			continue
		} else if openedBracket {
			if r[0] == ']' {
				// done
				return
			} else if r[0] == ',' {
				continue
			}
			kk := ints.New(0)
			if r, err = kk.Unmarshal(r); chk.E(err) {
				return
			}
			k.K = append(k.K, New(kk.Uint16()))
			if r[0] == ']' {
				r = r[1:]
				return
			}
		}
	}
	if !openedBracket {
		return nil, errorf.E(
			"kinds: failed to unmarshal\n%s\n%s\n%s", k,
			b, r,
		)
	}
	return
}

// IsPrivileged returns true if any of the elements of a kinds.S are privileged (ie, they should
// be privacy protected).
func (k *S) IsPrivileged() (priv bool) {
	for i := range k.K {
		if IsPrivileged(k.K[i].K) {
			return true
		}
	}
	return
}
