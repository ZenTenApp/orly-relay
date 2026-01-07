//go:build js && wasm

// Package bufpool provides buffer pools for reducing GC pressure in hot paths.
// This is the WASM version which uses simple allocations since sync.Pool
// behavior differs in WASM environments.
package bufpool

import (
	"bytes"
)

const (
	// SmallBufferSize for index keys (8-64 bytes typical)
	SmallBufferSize = 64

	// MediumBufferSize for event encoding (300-1000 bytes typical)
	MediumBufferSize = 1024
)

// GetSmall returns a small buffer (64 bytes).
// In WASM, we simply allocate new buffers as sync.Pool is less effective.
func GetSmall() *bytes.Buffer {
	return bytes.NewBuffer(make([]byte, 0, SmallBufferSize))
}

// PutSmall is a no-op in WASM; the buffer will be garbage collected.
func PutSmall(buf *bytes.Buffer) {
	// No-op in WASM
}

// GetMedium returns a medium buffer (1KB).
func GetMedium() *bytes.Buffer {
	return bytes.NewBuffer(make([]byte, 0, MediumBufferSize))
}

// PutMedium is a no-op in WASM; the buffer will be garbage collected.
func PutMedium(buf *bytes.Buffer) {
	// No-op in WASM
}

// CopyBytes copies the buffer contents to a new slice.
func CopyBytes(buf *bytes.Buffer) []byte {
	if buf == nil || buf.Len() == 0 {
		return nil
	}
	result := make([]byte, buf.Len())
	copy(result, buf.Bytes())
	return result
}
