package p256k1

// =============================================================================
// PUBLIC API REFERENCE
// =============================================================================
//
// This file documents the intended public API surface of the p256k1 package.
// While Go exports all capitalized identifiers, users should prefer the types
// and functions documented here for stability guarantees.
//
// # Domain-Focused Packages (Recommended)
//
// For most use cases, prefer the domain-focused wrapper packages that provide
// cleaner, more focused APIs:
//
//   - p256k1.mleku.dev/ecdsa    - ECDSA signature operations
//   - p256k1.mleku.dev/schnorr  - BIP-340 Schnorr signatures
//   - p256k1.mleku.dev/keys     - Key generation and management
//   - p256k1.mleku.dev/exchange - ECDH key exchange
//
// These packages re-export the core types and provide domain-specific functions
// with better ergonomics and documentation.
//
// For a high-level signer abstraction, see the signer.P256K1Signer type.
// =============================================================================

// =============================================================================
// KEY TYPES (Aggregate Root: KeyPair)
// =============================================================================

// PublicKey represents a secp256k1 public key.
// Use ECPubkeyCreate to generate from a private key, or ECPubkeyParse to load
// from serialized form.
//
// The internal representation stores the uncompressed point coordinates.
// Use ECPubkeySerialize to export in compressed (33 bytes) or uncompressed
// (65 bytes) format.

// XOnlyPubkey represents a BIP-340 x-only public key (32 bytes).
// X-only keys are used with Schnorr signatures and implicitly have even Y.
// Use XOnlyPubkeyParse to load, XOnlyPubkeySerialize to export.

// KeyPair represents a secp256k1 key pair (secret key + public key).
// This is the Aggregate Root for key management operations.
// Use KeyPairCreate to generate from a secret key.

// =============================================================================
// SIGNATURE TYPES (Value Objects)
// =============================================================================

// ECDSASignature represents an ECDSA signature with r and s components.
// Use ECDSASign to create, ECDSAVerify to verify.

// ECDSASignatureCompact is a 64-byte compact signature format (r || s).
// Use ToCompact/FromCompact methods on ECDSASignature.

// ECDSARecoverableSignature includes a recovery ID for public key recovery.
// Use ECDSASignRecoverable to create, ECDSARecover to recover public key.

// SchnorrSignature is a 64-byte BIP-340 Schnorr signature (r || s).
// Use SchnorrSign to create, SchnorrVerify to verify.

// =============================================================================
// KEY MANAGEMENT FUNCTIONS
// =============================================================================

// Key Generation and Parsing:
//   - ECPubkeyCreate(pubkey *PublicKey, seckey []byte) error
//   - ECPubkeyParse(pubkey *PublicKey, input []byte) error
//   - ECPubkeySerialize(output []byte, pubkey *PublicKey, flags uint) int
//   - ECSecKeyVerify(seckey []byte) error
//
// X-Only Keys (BIP-340):
//   - XOnlyPubkeyParse(pubkey *XOnlyPubkey, input []byte) error
//   - XOnlyPubkeySerialize(output []byte, pubkey *XOnlyPubkey) int
//   - XOnlyPubkeyFromPubkey(xonly *XOnlyPubkey, pk *PublicKey) bool
//
// Key Pairs:
//   - KeyPairCreate(keypair *KeyPair, seckey []byte) error
//   - KeyPairXOnlyPub(xonly *XOnlyPubkey, keypair *KeyPair) error

// =============================================================================
// ECDSA SIGNATURE FUNCTIONS
// =============================================================================

// Signing:
//   - ECDSASign(sig *ECDSASignature, msghash32 []byte, seckey []byte) error
//   - ECDSASignCompact(compact *ECDSASignatureCompact, msghash32, seckey []byte) error
//   - ECDSASignDER(msghash32, seckey []byte) ([]byte, error)
//   - ECDSASignRecoverable(sig *ECDSARecoverableSignature, msghash32, seckey []byte) error
//
// Verification:
//   - ECDSAVerify(sig *ECDSASignature, msghash32 []byte, pubkey *PublicKey) bool
//   - ECDSAVerifyCompact(compact *ECDSASignatureCompact, msghash32 []byte, pubkey *PublicKey) bool
//   - ECDSAVerifyDER(sigDER, msghash32 []byte, pubkey *PublicKey) bool
//
// Recovery:
//   - ECDSARecover(pubkey *PublicKey, sig *ECDSARecoverableSignature, msghash32 []byte) error
//   - ECDSARecoverCompact(pubkey *PublicKey, sig64 []byte, recid int, msghash32 []byte) error

// =============================================================================
// SCHNORR SIGNATURE FUNCTIONS (BIP-340)
// =============================================================================

// Signing:
//   - SchnorrSign(sig64, msg32 []byte, keypair *KeyPair, auxRand32 []byte) error
//
// Verification:
//   - SchnorrVerify(sig64, msg32 []byte, xonlyPubkey *XOnlyPubkey) bool
//   - SchnorrVerifyBatch(items []SchnorrVerifyItem) bool

// =============================================================================
// ECDH KEY EXCHANGE FUNCTIONS
// =============================================================================

// Key Exchange:
//   - ECDH(output []byte, pubkey *PublicKey, seckey []byte, hashfp ECDHHashFunction) error
//   - ECDHXOnly(output []byte, pubkey *PublicKey, seckey []byte) error
//   - ECDHWithHKDF(output []byte, pubkey *PublicKey, seckey []byte, salt, info []byte) error

// =============================================================================
// SCALAR MULTIPLICATION FUNCTIONS
// =============================================================================

// Generator Multiplication (computes scalar * G):
//   - EcmultGen(r *GroupElementJacobian, q *Scalar)
//
// Point Multiplication (computes scalar * point):
//   - Ecmult(r *GroupElementJacobian, a *GroupElementJacobian, q *Scalar)
//   - EcmultConst(r *GroupElementJacobian, a *GroupElementAffine, q *Scalar)
//
// Combined Multiplication (computes na*a + ng*G):
//   - EcmultCombined(r *GroupElementJacobian, a *GroupElementJacobian, na, ng *Scalar)

// =============================================================================
// HASHING FUNCTIONS
// =============================================================================

// SHA-256:
//   - NewSHA256() *SHA256
//   - (h *SHA256) Write(data []byte)
//   - (h *SHA256) Finalize(out32 []byte)
//
// HMAC-SHA256:
//   - NewHMACSHA256(key []byte) *HMACSHA256
//   - (h *HMACSHA256) Write(data []byte)
//   - (h *HMACSHA256) Finalize(out32 []byte)
//
// Tagged Hash (BIP-340):
//   - TaggedHash(tag, data []byte) [32]byte
//
// Key Derivation:
//   - HKDF(output, ikm, salt, info []byte) error

// =============================================================================
// CONSTANTS
// =============================================================================

// Compression flags for ECPubkeySerialize:
//   - ECCompressed   = 0x02  (33-byte output)
//   - ECUncompressed = 0x04  (65-byte output)

// =============================================================================
// INTERNAL TYPES (Use with caution - not part of stable API)
// =============================================================================

// The following types are exported but considered internal implementation details:
//
// Primitives:
//   - FieldElement, FieldElementStorage
//   - Scalar, ScalarZero, ScalarOne
//   - GroupElementAffine, GroupElementJacobian, GroupElementStorage
//
// These are exposed for advanced users who need low-level access to the
// cryptographic primitives, but their API may change between versions.
