//go:build !(js && wasm)

// Package bufpool provides buffer pools for reducing GC pressure in hot paths.
//
// Two pool sizes are provided:
//   - SmallPool (64 bytes): For index keys, serial encoding, short buffers
//   - MediumPool (1KB): For event encoding, larger serialization buffers
//
// Usage:
//
//	buf := bufpool.GetSmall()
//	defer bufpool.PutSmall(buf)
//	// Use buf...
//	// IMPORTANT: Copy buf.Bytes() before Put if data is needed after
package bufpool

import (
	"bytes"
	"sync"
)

const (
	// SmallBufferSize for index keys (8-64 bytes typical)
	SmallBufferSize = 64

	// MediumBufferSize for event encoding (300-1000 bytes typical)
	MediumBufferSize = 1024
)

var (
	// smallPool for index keys and short encodings
	smallPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, SmallBufferSize))
		},
	}

	// mediumPool for event encoding and larger buffers
	mediumPool = sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, MediumBufferSize))
		},
	}
)

// GetSmall returns a small buffer (64 bytes) from the pool.
// Call PutSmall when done to return it to the pool.
//
// WARNING: Copy buf.Bytes() before calling PutSmall if the data
// is needed after the buffer is returned to the pool.
func GetSmall() *bytes.Buffer {
	return smallPool.Get().(*bytes.Buffer)
}

// PutSmall returns a small buffer to the pool.
// The buffer is reset before being returned.
func PutSmall(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	smallPool.Put(buf)
}

// GetMedium returns a medium buffer (1KB) from the pool.
// Call PutMedium when done to return it to the pool.
//
// WARNING: Copy buf.Bytes() before calling PutMedium if the data
// is needed after the buffer is returned to the pool.
func GetMedium() *bytes.Buffer {
	return mediumPool.Get().(*bytes.Buffer)
}

// PutMedium returns a medium buffer to the pool.
// The buffer is reset before being returned.
func PutMedium(buf *bytes.Buffer) {
	if buf == nil {
		return
	}
	buf.Reset()
	mediumPool.Put(buf)
}

// CopyBytes copies the buffer contents to a new slice.
// Use this before returning the buffer to the pool if the
// data needs to persist.
func CopyBytes(buf *bytes.Buffer) []byte {
	if buf == nil || buf.Len() == 0 {
		return nil
	}
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result
}
