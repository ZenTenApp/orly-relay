package tag

import (
	"bytes"

	"next.orly.dev/pkg/nostr/utils"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// S is a list of tag.T - which are lists of string elements with ordering and
// no uniqueness constraint (not a set).
type S []*T

func NewS(t ...*T) (s *S) {
	s = new(S)
	*s = append(*s, t...)
	return
}

func NewSWithCap(c int) (s *S) {
	ss := make([]*T, 0, c)
	return (*S)(&ss)
}

func (s *S) Len() int {
	if s == nil {
		return 0
		// panic("tags cannot be used without initialization")
	}
	return len(*s)
}

func (s *S) Less(i, j int) bool {
	// only the first element is compared, this is only used for normalizing
	// filters and the individual tags must be separately sorted.
	return bytes.Compare((*s)[i].T[0], (*s)[j].T[0]) < 0
}

func (s *S) Swap(i, j int) {
	(*s)[i].T, (*s)[j].T = (*s)[j].T, (*s)[i].T
}

func (s *S) Append(t ...*T) {
	*s = append(*s, t...)
}

// ContainsAny returns true if any of the values given in `values` matches any
// of the tag elements. For e/p tags with binary-encoded values, this handles
// comparison correctly by using ValueHex() to ensure consistent comparison.
func (s *S) ContainsAny(tagName []byte, values [][]byte) bool {
	if s == nil {
		return false
	}
	if len(tagName) < 1 {
		return false
	}
	// Check if this is a binary-optimized tag type (e or p)
	isBinaryTag := len(tagName) == 1 && (tagName[0] == 'e' || tagName[0] == 'p')

	for _, v := range *s {
		if v.Len() < 2 {
			continue
		}
		if !utils.FastEqual(v.Key(), tagName) {
			continue
		}

		// For e/p tags, use ValueHex() to get consistent hex representation
		// regardless of whether the value is stored in binary or hex format
		var tagValue []byte
		if isBinaryTag {
			tagValue = v.ValueHex()
		} else {
			tagValue = v.Value()
		}

		for _, candidate := range values {
			if bytes.HasPrefix(tagValue, candidate) {
				return true
			}
		}
	}
	return false
}

// MarshalJSON encodes a tags.T appended to a provided byte slice in JSON form.
func (s *S) MarshalJSON() (b []byte, err error) {
	b = append(b, '[')
	for i, ss := range *s {
		b = ss.Marshal(b)
		if i < len(*s)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

func (s *S) Marshal(dst []byte) (b []byte) {
	if s == nil {
		log.I.F("tags cannot be used without initialization")
		return
	}
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	// Estimate: [ + (tag.Marshal result + comma) * n + ]
	if b == nil && len(*s) > 0 {
		estimatedSize := 2 // brackets
		// Estimate based on first tag size
		if len(*s) > 0 && (*s)[0] != nil {
			firstTagSize := (*s)[0].Marshal(nil)
			estimatedSize += len(*s) * (len(firstTagSize) + 1) // tag + comma
		}
		b = make([]byte, 0, estimatedSize)
	}
	b = append(b, '[')
	for i, ss := range *s {
		b = ss.Marshal(b)
		if i < len(*s)-1 {
			b = append(b, ',')
		}
	}
	b = append(b, ']')
	return
}

// UnmarshalJSON a tags.T from a provided byte slice and return what remains
// after the end of the array.
func (s *S) UnmarshalJSON(b []byte) (err error) {
	_, err = s.Unmarshal(b)
	return
}

// Unmarshal a tags.T from a provided byte slice and return what remains after
// the end of the array.
func (s *S) Unmarshal(b []byte) (r []byte, err error) {
	r = b[:]
	// Pre-allocate slice with estimated capacity to reduce reallocations
	// Estimate based on typical tag counts (can grow if needed)
	*s = make([]*T, 0, 16)
	for len(r) > 0 {
		switch r[0] {
		case '[':
			r = r[1:]
			goto inTags
		case ',':
			r = r[1:]
			// next
		case ']':
			r = r[1:]
			// the end
			return
		default:
			r = r[1:]
		}
	inTags:
		for len(r) > 0 {
			switch r[0] {
			case '[':
				tt := New()
				if r, err = tt.Unmarshal(r); chk.E(err) {
					return
				}
				*s = append(*s, tt)
			case ',':
				r = r[1:]
				// next
			case ']':
				r = r[1:]
				// the end
				return
			default:
				r = r[1:]
			}
		}
	}
	return
}

// GetFirst returns the first tag.T that has the same Key as t.
func (s *S) GetFirst(t []byte) (first *T) {
	if s == nil || len(*s) < 1 {
		return
	}
	for _, tt := range *s {
		if tt.Len() == 0 {
			continue
		}
		if utils.FastEqual(tt.T[0], t) {
			return tt
		}
	}
	return
}

func (s *S) GetAll(t []byte) (all []*T) {
	if s == nil || len(*s) < 1 {
		return
	}
	// Pre-allocate slice with estimated capacity to reduce reallocations
	// Estimate: typically 1-2 tags match, but can be more
	all = make([]*T, 0, 4)
	for _, tt := range *s {
		if len(tt.T) < 1 {
			continue
		}
		if utils.FastEqual(tt.T[0], t) {
			all = append(all, tt)
		}
	}
	return
}

func (s *S) GetTagElement(i int) (t *T) {
	if s == nil || len(*s) < i {
		return
	}
	t = (*s)[i]
	return
}

// ToSliceOfSliceOfStrings converts the tag collection into a two-dimensional
// slice of strings, maintaining the structure of tags and their elements.
//
// # Return Values
//
//   - ss ([][]string): A slice of string slices where each inner slice represents
//     a tag's elements converted from bytes to strings.
//
// - err (error): Currently unused but maintained for interface consistency.
//
// # Expected Behaviour
//
// Iterates through each tag in the collection and converts its byte elements
// to strings, preserving the tag structure in the resulting nested slice.
func (s *S) ToSliceOfSliceOfStrings() (ss [][]string) {
	if s == nil || len(*s) == 0 {
		return
	}
	// Pre-allocate slice with exact capacity to reduce reallocations
	ss = make([][]string, 0, len(*s))
	for _, v := range *s {
		ss = append(ss, v.ToSliceOfStrings())
	}
	return
}
