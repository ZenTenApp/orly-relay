package filter

import (
	"git.mleku.dev/mleku/nostr/encoders/event"
	"lol.mleku.dev/errorf"
)

type S []*F

func NewS(ff ...*F) *S {
	s := S(ff)
	return &s
}

// Match checks if a set of filters.T matches on an event.F.
func (s *S) Match(event *event.E) bool {
	for _, f := range *s {
		if f.Matches(event) {
			return true
		}
	}
	return false
}

// Marshal encodes a slice of filters as a JSON array of objects.
// It appends the result to dst and returns the resulting slice.
func (s *S) Marshal(dst []byte) (b []byte) {
	if s == nil {
		panic("nil filter slice")
	}
	b = dst
	b = append(b, '[')
	first := false
	for _, f := range *s {
		if f == nil {
			continue
		}
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = f.Marshal(b)
	}
	b = append(b, ']')
	return
}

// Unmarshal decodes one or more filters from JSON.
// This handles both array-wrapped filters [{},...] and unwrapped filters {},...
func (s *S) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	if len(r) == 0 {
		return
	}

	// Check if filters are wrapped in an array
	isArrayWrapped := r[0] == '['
	if isArrayWrapped {
		r = r[1:]
		// Handle empty array "[]"
		if len(r) > 0 && r[0] == ']' {
			r = r[1:]
			return
		}
	}

	for {
		if len(r) == 0 {
			return
		}
		f := New()
		var rem []byte
		if rem, err = f.Unmarshal(r); err != nil {
			return
		}
		*s = append(*s, f)
		r = rem
		if len(r) == 0 {
			return
		}
		if r[0] == ',' {
			// Next element
			r = r[1:]
			continue
		}
		if r[0] == ']' {
			// End of array or envelope
			if isArrayWrapped {
				// Consume the closing bracket of the filter array
				r = r[1:]
			}
			// Otherwise leave it for the envelope parser
			return
		}
		// Unexpected token
		err = errorf.E(
			"filters.Unmarshal: expected ',' or ']' after filter, got: %q",
			r[0],
		)
		return
	}
}
func (s *S) MatchIgnoringTimestampConstraints(ev *event.E) bool {
	for _, ff := range *s {
		if ff.MatchesIgnoringTimestampConstraints(ev) {
			return true
		}
	}
	return false
}
