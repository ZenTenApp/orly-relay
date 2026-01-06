//go:build !(js && wasm)

package bbolt

import (
	"encoding/binary"
)

// encodeUint40 encodes a uint64 as 5 bytes (big-endian, truncated to 40 bits)
func encodeUint40(v uint64, buf []byte) {
	buf[0] = byte(v >> 32)
	buf[1] = byte(v >> 24)
	buf[2] = byte(v >> 16)
	buf[3] = byte(v >> 8)
	buf[4] = byte(v)
}

// decodeUint40 decodes 5 bytes as uint64
func decodeUint40(buf []byte) uint64 {
	return uint64(buf[0])<<32 |
		uint64(buf[1])<<24 |
		uint64(buf[2])<<16 |
		uint64(buf[3])<<8 |
		uint64(buf[4])
}

// encodeUint64 encodes a uint64 as 8 bytes (big-endian)
func encodeUint64(v uint64, buf []byte) {
	binary.BigEndian.PutUint64(buf, v)
}

// decodeUint64 decodes 8 bytes as uint64
func decodeUint64(buf []byte) uint64 {
	return binary.BigEndian.Uint64(buf)
}

// encodeUint32 encodes a uint32 as 4 bytes (big-endian)
func encodeUint32(v uint32, buf []byte) {
	binary.BigEndian.PutUint32(buf, v)
}

// decodeUint32 decodes 4 bytes as uint32
func decodeUint32(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

// encodeUint16 encodes a uint16 as 2 bytes (big-endian)
func encodeUint16(v uint16, buf []byte) {
	binary.BigEndian.PutUint16(buf, v)
}

// decodeUint16 decodes 2 bytes as uint16
func decodeUint16(buf []byte) uint16 {
	return binary.BigEndian.Uint16(buf)
}

// encodeVarint encodes a uint64 as a variable-length integer
// Returns the number of bytes written
func encodeVarint(v uint64, buf []byte) int {
	return binary.PutUvarint(buf, v)
}

// decodeVarint decodes a variable-length integer
// Returns the value and the number of bytes read
func decodeVarint(buf []byte) (uint64, int) {
	return binary.Uvarint(buf)
}

// makeSerialKey creates a 5-byte key from a serial number
func makeSerialKey(serial uint64) []byte {
	key := make([]byte, 5)
	encodeUint40(serial, key)
	return key
}

// makePubkeyHashKey creates an 8-byte key from a pubkey hash
func makePubkeyHashKey(hash []byte) []byte {
	key := make([]byte, 8)
	copy(key, hash[:8])
	return key
}

// makeIdHashKey creates an 8-byte key from an event ID hash
func makeIdHashKey(id []byte) []byte {
	key := make([]byte, 8)
	copy(key, id[:8])
	return key
}

// hashPubkey returns the first 8 bytes of a 32-byte pubkey as a hash
func hashPubkey(pubkey []byte) []byte {
	if len(pubkey) < 8 {
		return pubkey
	}
	return pubkey[:8]
}

// hashEventId returns the first 8 bytes of a 32-byte event ID as a hash
func hashEventId(id []byte) []byte {
	if len(id) < 8 {
		return id
	}
	return id[:8]
}

// concatenate joins multiple byte slices into one
func concatenate(slices ...[]byte) []byte {
	var totalLen int
	for _, s := range slices {
		totalLen += len(s)
	}
	result := make([]byte, totalLen)
	var offset int
	for _, s := range slices {
		copy(result[offset:], s)
		offset += len(s)
	}
	return result
}
