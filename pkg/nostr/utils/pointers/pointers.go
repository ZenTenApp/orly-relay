package pointers

import (
	"time"

	"next.orly.dev/pkg/nostr/encoders/timestamp"
)

// PointerToValue is a generic interface (type constraint) to refer to any
// pointer to almost any kind of common type of value.
//
// see the utils/values package for a set of methods to accept these values and
// return the correct type pointer to them.
type PointerToValue interface {
	~*uint | ~*int | ~*uint8 | ~*uint16 | ~*uint32 | ~*uint64 | ~*int8 | ~*int16 | ~*int32 |
		~*int64 | ~*float32 | ~*float64 | ~*string | ~*[]string | ~*time.Time | ~*time.Duration |
		~*[]byte | ~*[][]byte | ~*timestamp.T
}

// Present determines whether there is a value for a PointerToValue type.
func Present[V PointerToValue](i V) bool {
	return i != nil
}
