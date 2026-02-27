// Package p256k1 provides a pure Go implementation of the secp256k1 elliptic curve
// cryptographic operations, including ECDSA signatures, Schnorr signatures (BIP-340),
// and Elliptic Curve Diffie-Hellman (ECDH) key exchange.
//
// # Architecture Overview
//
// This package is organized into several conceptual layers, following Domain-Driven Design
// principles where cryptographic primitives form the core domain:
//
// ## Layer 1: Field Arithmetic (Bounded Context: Finite Field)
//
// Operations in the finite field F_p where p = 2^256 - 2^32 - 977 (the secp256k1 field prime).
//
// Types:
//   - FieldElement: A field element in 5x52-bit limb representation
//   - FieldElementStorage: Compact 4x64-bit storage format
//
// Files: field.go, field_mul.go, field_4x64.go, field_32bit.go, field_amd64.go
//
// ## Layer 2: Scalar Arithmetic (Bounded Context: Group Order)
//
// Operations modulo the group order n = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141.
//
// Types:
//   - Scalar: A 256-bit scalar in 4x64-bit representation
//
// Files: scalar.go, scalar_32bit.go, scalar_amd64.go
//
// ## Layer 3: Group Operations (Bounded Context: Elliptic Curve Points)
//
// Point operations on the secp256k1 curve: y^2 = x^3 + 7.
//
// Types:
//   - GroupElementAffine: Point in affine coordinates (x, y)
//   - GroupElementJacobian: Point in Jacobian coordinates (X, Y, Z) where x = X/Z^2, y = Y/Z^3
//   - GroupElementStorage: Compact storage format
//
// Files: group.go, group_4x64.go
//
// ## Layer 4: Cryptographic Hashing (Supporting Domain)
//
// Hash functions used by signature schemes and key derivation.
//
// Types:
//   - SHA256: SHA-256 hash context
//   - HMACSHA256: HMAC-SHA256 context
//   - RFC6979HMACSHA256: Deterministic nonce generation
//
// Files: hash.go
//
// ## Layer 5: Signature Schemes (Core Domain Services)
//
// ECDSA and Schnorr signature creation and verification.
//
// Types:
//   - ECDSASignature: ECDSA signature (r, s)
//   - ECDSARecoverableSignature: ECDSA with recovery ID
//   - SchnorrSignature: BIP-340 Schnorr signature
//
// Files: ecdsa.go, schnorr.go, schnorr_batch.go, verify.go
//
// ## Layer 6: Key Management (Aggregate Root)
//
// Public keys, key pairs, and key operations.
//
// Types:
//   - PublicKey: Secp256k1 public key
//   - XOnlyPubkey: X-only public key (BIP-340)
//   - KeyPair: Secret/public key pair (Aggregate Root)
//
// Files: pubkey.go, eckey.go, extrakeys.go
//
// ## Layer 7: Key Exchange (Domain Service)
//
// Elliptic Curve Diffie-Hellman key exchange.
//
// Functions: ECDH, ECDHXOnly, ECDHWithHKDF
//
// Files: ecdh.go, ecdh_amd64.go
//
// # Performance Optimizations
//
// This implementation includes several performance optimizations:
//
//   - GLV Endomorphism: Splits 256-bit scalar multiplication into two ~128-bit operations
//   - Strauss Algorithm: Combines multiple scalar multiplications to share doublings
//   - wNAF Representation: Windowed Non-Adjacent Form reduces point additions
//   - Precomputed Tables: Generator point multiples precomputed at init time
//   - Platform-Specific Assembly: AMD64 assembly for field/scalar operations
//   - Batch Operations: Batch normalization and batch Schnorr verification
//   - Zero-Allocation Hot Paths: Critical paths avoid heap allocations
//
// # Platform Support
//
// The package supports multiple platforms through build tags:
//
//   - AMD64: Optimized assembly with AVX2/BMI2 support
//   - 32-bit: Pure Go implementation for 32-bit systems
//   - WASM: WebAssembly-compatible implementation
//   - Generic: Fallback implementation for other platforms
//
// # Thread Safety
//
// All public functions are safe for concurrent use. Internal hash contexts use
// sync.Pool for efficient reuse across goroutines.
//
// # Security Considerations
//
//   - Constant-time operations where required (e.g., scalar multiplication with secrets)
//   - Secure memory clearing for sensitive data
//   - RFC 6979 deterministic nonce generation for ECDSA
//   - BIP-340 compliant Schnorr signatures
//
// # Usage Examples
//
// See the examples/ directory for complete usage examples, or use the higher-level
// signer.P256K1Signer type for a simplified interface.
//
// # Related Packages
//
// Domain-focused wrapper packages (recommended for most use cases):
//
//   - ecdsa: ECDSA signature operations with clean API
//   - schnorr: BIP-340 Schnorr signatures with batch verification
//   - keys: Key generation, parsing, serialization, and tweaking
//   - exchange: ECDH key exchange and key derivation (HKDF)
//
// Internal/advanced packages:
//
//   - signer: High-level signer abstraction with simplified API
//   - wnaf: Windowed Non-Adjacent Form encoding utilities
//   - avx: Experimental AVX2-accelerated operations
package p256k1
