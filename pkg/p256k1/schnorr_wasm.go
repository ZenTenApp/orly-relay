//go:build js || wasm || tinygo || wasm32

package p256k1

import (
	"errors"
)

// BIP-340 nonce tag
var bip340NonceTag = []byte("BIP0340/nonce")

// BIP-340 aux tag
var bip340AuxTag = []byte("BIP0340/aux")

// BIP-340 challenge tag
var bip340ChallengeTag = []byte("BIP0340/challenge")

// Zero mask for BIP-340 nonce generation (precomputed TaggedHash("BIP0340/aux", 0x0000...00))
var zeroMask = [32]byte{
	84, 241, 105, 207, 201, 226, 229, 114,
	116, 128, 68, 31, 144, 186, 37, 196,
	136, 244, 97, 199, 11, 94, 165, 220,
	170, 247, 175, 105, 39, 10, 165, 20,
}

// NonceFunctionBIP340 implements BIP-340 nonce generation
func NonceFunctionBIP340(nonce32 []byte, msg []byte, key32 []byte, xonlyPk32 []byte, auxRand32 []byte) error {
	if len(nonce32) != 32 {
		return errors.New("nonce32 must be 32 bytes")
	}
	if len(key32) != 32 {
		return errors.New("key32 must be 32 bytes")
	}
	if len(xonlyPk32) != 32 {
		return errors.New("xonlyPk32 must be 32 bytes")
	}

	// Mask key with aux random data
	var maskedKey [32]byte
	if auxRand32 != nil && len(auxRand32) == 32 {
		// TaggedHash("BIP0340/aux", aux_rand32)
		auxHash := TaggedHash(bip340AuxTag, auxRand32)
		for i := 0; i < 32; i++ {
			maskedKey[i] = key32[i] ^ auxHash[i]
		}
	} else {
		// Use zero mask
		for i := 0; i < 32; i++ {
			maskedKey[i] = key32[i] ^ zeroMask[i]
		}
	}

	// TaggedHash("BIP0340/nonce", masked_key || xonly_pk || msg)
	// Use fixed-size buffer to avoid heap allocation (32 + 32 + 32 = 96 bytes)
	var nonceInput [96]byte
	copy(nonceInput[0:32], maskedKey[:])
	copy(nonceInput[32:64], xonlyPk32)
	copy(nonceInput[64:96], msg)

	nonceHash := TaggedHash(bip340NonceTag, nonceInput[:])
	copy(nonce32, nonceHash[:])

	// Clear sensitive data
	for i := range maskedKey {
		maskedKey[i] = 0
	}

	return nil
}

// BatchSchnorrItem holds one item for batch verification (stub for WASM).
type BatchSchnorrItem struct {
	Signature []byte
	Message   []byte
	Pubkey    *XOnlyPubkey
}

// SchnorrBatchVerify verifies multiple signatures (WASM fallback: individual verification).
func SchnorrBatchVerify(items []BatchSchnorrItem) bool {
	for _, item := range items {
		if !SchnorrVerify(item.Signature, item.Message, item.Pubkey) {
			return false
		}
	}
	return true
}

// SchnorrSignature represents a 64-byte Schnorr signature (r || s)
type SchnorrSignature [64]byte

// SchnorrSign creates a Schnorr signature following BIP-340
func SchnorrSign(sig64 []byte, msg32 []byte, keypair *KeyPair, auxRand32 []byte) error {
	if len(sig64) != 64 {
		return errors.New("signature must be 64 bytes")
	}
	if len(msg32) != 32 {
		return errors.New("message must be 32 bytes")
	}
	if keypair == nil {
		return errors.New("keypair cannot be nil")
	}

	// Load secret key
	var sk Scalar
	if !sk.setB32Seckey(keypair.seckey[:]) {
		return errors.New("invalid secret key")
	}

	// Load public key
	var pk GroupElementAffine
	pk.fromBytes(keypair.pubkey.data[:])
	if pk.isInfinity() {
		return errors.New("invalid public key")
	}

	// Negate secret key if Y coordinate is odd (BIP-340 requires even Y)
	pk.y.normalize()
	var skBytes [32]byte
	sk.getB32(skBytes[:])

	if pk.y.isOdd() {
		sk.negate(&sk)
		sk.getB32(skBytes[:]) // Update skBytes with negated key
		// Update pk to have even Y
		pk.negate(&pk)
	}

	// Get x-only public key (X coordinate)
	var pkX [32]byte
	pk.x.normalize()
	pk.x.getB32(pkX[:])

	// Generate nonce (use the possibly-negated secret key)
	var nonce32 [32]byte
	if err := NonceFunctionBIP340(nonce32[:], msg32, skBytes[:], pkX[:], auxRand32); err != nil {
		return err
	}

	// Parse nonce scalar
	var k Scalar
	if !k.setB32Seckey(nonce32[:]) {
		return errors.New("nonce generation failed")
	}

	if k.isZero() {
		return errors.New("nonce is zero")
	}

	// Compute R = k * G
	var rj GroupElementJacobian
	EcmultGen(&rj, &k)

	// Convert to affine
	var r GroupElementAffine
	r.setGEJ(&rj)
	r.y.normalize()

	// If R.y is odd, negate k
	if r.y.isOdd() {
		k.negate(&k)
		// Recompute R with negated k
		EcmultGen(&rj, &k)
		r.setGEJ(&rj)
	}

	// Extract r = X(R)
	r.x.normalize()
	var r32 [32]byte
	r.x.getB32(r32[:])
	copy(sig64[:32], r32[:])

	// Compute challenge e = TaggedHash("BIP0340/challenge", r || pk || msg)
	// Use fixed-size buffer to avoid heap allocation (32 + 32 + 32 = 96 bytes)
	var challengeInput [96]byte
	copy(challengeInput[0:32], r32[:])
	copy(challengeInput[32:64], pkX[:])
	copy(challengeInput[64:96], msg32)

	challengeHash := TaggedHash(bip340ChallengeTag, challengeInput[:])
	var e Scalar
	e.setB32(challengeHash[:])

	// Compute s = k + e * sk
	var s Scalar
	s.mul(&e, &sk)
	s.add(&s, &k)

	// Serialize s
	var s32 [32]byte
	s.getB32(s32[:])
	copy(sig64[32:], s32[:])

	// Clear sensitive data
	sk.clear()
	k.clear()
	e.clear()
	s.clear()
	for i := range nonce32 {
		nonce32[i] = 0
	}
	for i := range pkX {
		pkX[i] = 0
	}
	for i := range skBytes {
		skBytes[i] = 0
	}
	rj.clear()
	r.clear()

	return nil
}

// SchnorrVerify verifies a BIP-340 Schnorr signature
func SchnorrVerify(sig64 []byte, msg32 []byte, xonlyPubkey *XOnlyPubkey) bool {
	if len(sig64) != 64 {
		return false
	}
	if len(msg32) != 32 {
		return false
	}
	if xonlyPubkey == nil {
		return false
	}

	ctx := getSchnorrVerifyContext()

	// Convert x-only pubkey to secp256k1_xonly_pubkey format
	var secp_xonly secp256k1_xonly_pubkey
	copy(secp_xonly.data[:], xonlyPubkey.data[:])

	result := secp256k1_schnorrsig_verify(ctx, sig64, msg32, len(msg32), &secp_xonly)
	return result == 1
}
