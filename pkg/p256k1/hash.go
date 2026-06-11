package p256k1

import (
	"crypto/sha256"
	"errors"
	"hash"
	"sync"
	"unsafe"

	sha256simd "github.com/minio/sha256-simd"
)

// Precomputed TaggedHash prefixes for common BIP-340 tags
// Computed once at init time to avoid repeated hash operations.
var (
	bip340AuxTagHash       [32]byte
	bip340NonceTagHash     [32]byte
	bip340ChallengeTagHash [32]byte
)

func init() {
	initTaggedHashPrefixes()
}

// sha256Pool provides a pool of SHA256 hash contexts to reduce allocations
var sha256Pool = sync.Pool{
	New: func() interface{} {
		return sha256.New()
	},
}

// getSHA256 gets a SHA256 hash context from the pool
func getSHA256() hash.Hash {
	h := sha256Pool.Get().(hash.Hash)
	h.Reset()
	return h
}

// putSHA256 returns a SHA256 hash context to the pool
func putSHA256(h hash.Hash) {
	sha256Pool.Put(h)
}

// hmacPool provides a pool of HMACSHA256 structs to reduce allocations
var hmacPool = sync.Pool{
	New: func() interface{} {
		return &HMACSHA256{}
	},
}

// getHMAC gets an HMACSHA256 from the pool
func getHMAC() *HMACSHA256 {
	return hmacPool.Get().(*HMACSHA256)
}

// putHMAC returns an HMACSHA256 to the pool
func putHMAC(h *HMACSHA256) {
	if h.inner != nil {
		putSHA256(h.inner)
		h.inner = nil
	}
	if h.outer != nil {
		putSHA256(h.outer)
		h.outer = nil
	}
	hmacPool.Put(h)
}

func initTaggedHashPrefixes() {
	bip340AuxTagHash = sha256.Sum256([]byte("BIP0340/aux"))
	bip340NonceTagHash = sha256.Sum256([]byte("BIP0340/nonce"))
	bip340ChallengeTagHash = sha256.Sum256([]byte("BIP0340/challenge"))
}

// getTaggedHashPrefix returns the precomputed SHA256(tag) for common tags
func getTaggedHashPrefix(tag []byte) [32]byte {
	// Fast path for common BIP-340 tags
	if len(tag) == 13 {
		switch string(tag) {
		case "BIP0340/aux":
			return bip340AuxTagHash
		case "BIP0340/nonce":
			return bip340NonceTagHash
		case "BIP0340/challenge":
			return bip340ChallengeTagHash
		}
	}

	// Fallback for unknown tags
	return sha256.Sum256(tag)
}

// SHA256 represents a SHA-256 hash context
type SHA256 struct {
	hasher hash.Hash
}

// NewSHA256 creates a new SHA-256 hash context
func NewSHA256() *SHA256 {
	h := &SHA256{}
	h.hasher = sha256simd.New()
	return h
}

// Write writes data to the hash
func (h *SHA256) Write(data []byte) {
	h.hasher.Write(data)
}

// Sum finalizes the hash and returns the 32-byte result
func (h *SHA256) Sum(out []byte) []byte {
	if out == nil {
		out = make([]byte, 32)
	}
	copy(out, h.hasher.Sum(nil))
	return out
}

// Finalize finalizes the hash and writes the result to out32 (must be 32 bytes)
func (h *SHA256) Finalize(out32 []byte) {
	if len(out32) != 32 {
		panic("output buffer must be 32 bytes")
	}
	sum := h.hasher.Sum(nil)
	copy(out32, sum)
}

// Clear clears the hash context to prevent leaking sensitive information
func (h *SHA256) Clear() {
	memclear(unsafe.Pointer(h), unsafe.Sizeof(*h))
}

// HMACSHA256 represents an HMAC-SHA256 context
// Uses pooled hash contexts to minimize allocations
type HMACSHA256 struct {
	inner, outer hash.Hash
}

// NewHMACSHA256 creates a new HMAC-SHA256 context with the given key
// Uses pooled HMACSHA256 and SHA256 contexts to minimize allocations
func NewHMACSHA256(key []byte) *HMACSHA256 {
	h := getHMAC()

	// Prepare key: if keylen > 64, hash it first
	var rkey [64]byte
	if len(key) <= 64 {
		copy(rkey[:], key)
		// Zero pad the rest
		for i := len(key); i < 64; i++ {
			rkey[i] = 0
		}
	} else {
		// Hash the key if it's too long
		hasher := getSHA256()
		hasher.Write(key)
		sum := hasher.Sum(nil)
		copy(rkey[:32], sum)
		putSHA256(hasher)
		// Zero pad the rest
		for i := 32; i < 64; i++ {
			rkey[i] = 0
		}
	}

	// Initialize outer hash with key XOR 0x5c
	h.outer = getSHA256()
	for i := 0; i < 64; i++ {
		rkey[i] ^= 0x5c
	}
	h.outer.Write(rkey[:])

	// Initialize inner hash with key XOR 0x36
	h.inner = getSHA256()
	for i := 0; i < 64; i++ {
		rkey[i] ^= 0x5c ^ 0x36
	}
	h.inner.Write(rkey[:])

	// Clear sensitive key material
	memclear(unsafe.Pointer(&rkey), unsafe.Sizeof(rkey))
	return h
}

// Write writes data to the inner hash
func (h *HMACSHA256) Write(data []byte) {
	h.inner.Write(data)
}

// Finalize finalizes the HMAC and writes the result to out32 (must be 32 bytes)
func (h *HMACSHA256) Finalize(out32 []byte) {
	if len(out32) != 32 {
		panic("output buffer must be 32 bytes")
	}

	// Finalize inner hash - use temp[:0] to avoid allocation
	var temp [32]byte
	h.inner.Sum(temp[:0])

	// Feed inner hash result to outer hash
	h.outer.Write(temp[:])

	// Finalize outer hash - append to out32[:0] to avoid allocation
	h.outer.Sum(out32[:0])

	// Clear temp
	memclear(unsafe.Pointer(&temp), unsafe.Sizeof(temp))
}

// Clear clears the HMAC context and returns it to the pool
func (h *HMACSHA256) Clear() {
	putHMAC(h)
}

// RFC6979HMACSHA256 implements RFC 6979 deterministic nonce generation
type RFC6979HMACSHA256 struct {
	v     [32]byte
	k     [32]byte
	retry int
}

// Preallocated single-byte slices to avoid allocation in hot paths
var (
	byte0x00 = []byte{0x00}
	byte0x01 = []byte{0x01}
)

// rfc6979Pool provides a pool of RFC6979HMACSHA256 structs
var rfc6979Pool = sync.Pool{
	New: func() interface{} {
		return &RFC6979HMACSHA256{}
	},
}

// NewRFC6979HMACSHA256 initializes a new RFC6979 HMAC-SHA256 context
func NewRFC6979HMACSHA256(key []byte) *RFC6979HMACSHA256 {
	rng := rfc6979Pool.Get().(*RFC6979HMACSHA256)

	// RFC6979 3.2.b: V = 0x01 0x01 0x01 ... 0x01 (32 bytes)
	for i := 0; i < 32; i++ {
		rng.v[i] = 0x01
	}

	// RFC6979 3.2.c: K = 0x00 0x00 0x00 ... 0x00 (32 bytes)
	for i := 0; i < 32; i++ {
		rng.k[i] = 0x00
	}

	// RFC6979 3.2.d: K = HMAC_K(V || 0x00 || key)
	hmac := NewHMACSHA256(rng.k[:])
	hmac.Write(rng.v[:])
	hmac.Write(byte0x00)
	hmac.Write(key)
	hmac.Finalize(rng.k[:])
	hmac.Clear()

	// V = HMAC_K(V)
	hmac = NewHMACSHA256(rng.k[:])
	hmac.Write(rng.v[:])
	hmac.Finalize(rng.v[:])
	hmac.Clear()

	// RFC6979 3.2.f: K = HMAC_K(V || 0x01 || key)
	hmac = NewHMACSHA256(rng.k[:])
	hmac.Write(rng.v[:])
	hmac.Write(byte0x01)
	hmac.Write(key)
	hmac.Finalize(rng.k[:])
	hmac.Clear()

	// V = HMAC_K(V)
	hmac = NewHMACSHA256(rng.k[:])
	hmac.Write(rng.v[:])
	hmac.Finalize(rng.v[:])
	hmac.Clear()

	rng.retry = 0
	return rng
}

// Generate generates output bytes using RFC6979
func (rng *RFC6979HMACSHA256) Generate(out []byte) {
	// RFC6979 3.2.h: If retry, update K and V
	if rng.retry != 0 {
		hmac := NewHMACSHA256(rng.k[:])
		hmac.Write(rng.v[:])
		hmac.Write(byte0x00)
		hmac.Finalize(rng.k[:])
		hmac.Clear()

		hmac = NewHMACSHA256(rng.k[:])
		hmac.Write(rng.v[:])
		hmac.Finalize(rng.v[:])
		hmac.Clear()
	}

	// Generate output bytes
	outlen := len(out)
	for outlen > 0 {
		hmac := NewHMACSHA256(rng.k[:])
		hmac.Write(rng.v[:])
		hmac.Finalize(rng.v[:])
		hmac.Clear()

		now := outlen
		if now > 32 {
			now = 32
		}
		copy(out, rng.v[:now])
		out = out[now:]
		outlen -= now
	}

	rng.retry = 1
}

// Finalize finalizes the RFC6979 context
func (rng *RFC6979HMACSHA256) Finalize() {
	// Nothing to do, but matches C API
}

// Clear clears the RFC6979 context and returns it to the pool
func (rng *RFC6979HMACSHA256) Clear() {
	memclear(unsafe.Pointer(rng), unsafe.Sizeof(*rng))
	rfc6979Pool.Put(rng)
}

// TaggedHash computes SHA256(SHA256(tag) || SHA256(tag) || data)
// This is used in BIP-340 for Schnorr signatures
// Optimized to use precomputed tag hashes for common BIP-340 tags
// Pool of SHA-256 hash contexts for TaggedHash — avoids allocation in steady
// state while remaining safe for concurrent use across goroutines.
var taggedHashPool = sync.Pool{
	New: func() any { return sha256.New() },
}

func TaggedHash(tag []byte, data []byte) [32]byte {
	var result [32]byte

	// Get precomputed SHA256(tag) prefix (or compute if not cached)
	tagHash := getTaggedHashPrefix(tag)

	// Second hash: SHA256(SHA256(tag) || SHA256(tag) || data)
	// Each goroutine gets its own hasher from the pool — safe for concurrent use.
	h := taggedHashPool.Get().(hash.Hash)
	h.Reset()
	h.Write(tagHash[:]) // SHA256(tag)
	h.Write(tagHash[:]) // SHA256(tag) again
	h.Write(data)       // data
	h.Sum(result[:0])   // Sum directly into result without allocation
	taggedHashPool.Put(h)

	return result
}

// HashToScalar converts a 32-byte hash to a scalar value
func HashToScalar(hash []byte) (*Scalar, error) {
	if len(hash) != 32 {
		return nil, errors.New("hash must be 32 bytes")
	}

	var scalar Scalar
	scalar.setB32(hash)
	return &scalar, nil
}

// HashToField converts a 32-byte hash to a field element
func HashToField(hash []byte) (*FieldElement, error) {
	if len(hash) != 32 {
		return nil, errors.New("hash must be 32 bytes")
	}

	var field FieldElement
	if err := field.setB32(hash); err != nil {
		return nil, err
	}
	return &field, nil
}
