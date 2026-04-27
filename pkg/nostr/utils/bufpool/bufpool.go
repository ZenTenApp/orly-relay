package bufpool

import (
	"sync"

	"git.smesh.lol/orly/pkg/nostr/utils/units"
)

const (
	// BufferSize is the size of each buffer in the pool (1kb)
	BufferSize = units.Kb
)

type B []byte

func (b B) ToBytes() []byte { return b }

var Pool = sync.Pool{
	New: func() interface{} {
		// Create a new buffer when the pool is empty
		b := make([]byte, 0, BufferSize)
		// log.T.C(
		// 	func() string {
		// 		ptr := unsafe.SliceData(b)
		// 		return fmt.Sprintf("creating buffer at: %p", ptr)
		// 	},
		// )
		return B(b)
	},
}

// Get returns a buffer from the pool or creates a new one if the pool is empty.
//
// Example usage:
//
//	buf := bufpool.Get()
//	defer bufpool.Put(buf)
//	// Use buf...
func Get() B {
	b := Pool.Get().(B)
	// log.T.C(
	// 	func() string {
	// 		ptr := unsafe.SliceData(b)
	// 		return fmt.Sprintf("getting buffer at: %p", ptr)
	// 	},
	// )
	return b
}

// Put returns a buffer to the pool.
// Buffers should be returned to the pool when no longer needed to allow reuse.
func Put(b B) {
	for i := range b {
		(b)[i] = 0
	}
	b = b[:0]
	// log.T.C(
	// 	func() string {
	// 		ptr := unsafe.SliceData(b)
	// 		return fmt.Sprintf("returning to buffer: %p", ptr)
	// 	},
	// )
	Pool.Put(b)
}

// PutBytes returns a buffer was not necessarily created by Get().
func PutBytes(b []byte) {
	// log.T.C(
	// 	func() string {
	// 		ptr := unsafe.SliceData(b)
	// 		return fmt.Sprintf("returning bytes to buffer: %p", ptr)
	// 	},
	// )
	b = b[:0]
	Put(b)
}
