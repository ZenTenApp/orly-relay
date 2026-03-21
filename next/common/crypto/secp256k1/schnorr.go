package secp256k1

import (
	"common/crypto/sha256"
)

// VerifySchnorr verifies a BIP-340 Schnorr signature.
// pubkey: 32-byte x-only public key
// msg: 32-byte message hash
// sig: 64-byte signature (R.x || s)
func VerifySchnorr(pubkey, msg [32]byte, sig [64]byte) bool {
	// Parse public key.
	px := feFromBytes(pubkey[:])
	P, ok := LiftX(px)
	if !ok {
		return false
	}

	// Parse signature.
	rx := feFromBytes(sig[:32])
	s := scalarFromBytes(sig[32:])

	// e = tagged_hash("BIP0340/challenge", R.x || P.x || msg) mod n
	e := computeChallenge(sig[:32], pubkey[:], msg[:])

	// R = s*G - e*P
	sG := ScalarBaseMult(s)
	eP := ScalarMult(pointFromAffine(P), e).toAffine()

	// s*G - e*P = s*G + (-e)*P
	negEP := AffinePoint{eP.X, feNeg(eP.Y)}
	R := pointAdd(pointFromAffine(sG), pointFromAffine(negEP)).toAffine()

	// Check R.x == rx and R.y is even.
	if feIsZero(R.X) && feIsZero(R.Y) {
		return false
	}
	if R.X != rx {
		return false
	}
	if !feIsEven(R.Y) {
		return false
	}

	return true
}

// SignSchnorr creates a BIP-340 Schnorr signature.
// seckey: 32-byte secret key
// msg: 32-byte message hash
// auxRand: 32-byte auxiliary randomness
// Returns zero [64]byte and false on failure.
func SignSchnorr(seckey, msg, auxRand [32]byte) (sig [64]byte, ok bool) {
	sk := scalarFromBytes(seckey[:])
	if scalarIsZero(sk) {
		return sig, false
	}

	// P = sk * G
	P := ScalarBaseMult(sk)

	// If P.y is odd, negate the secret key.
	d := sk
	if !feIsEven(P.Y) {
		d = scalarNeg(d)
	}

	pkBytes := feToBytes(P.X)

	// t = d XOR tagged_hash("BIP0340/aux", auxRand)
	var t [32]byte
	aux := taggedHash("BIP0340/aux", auxRand[:])
	dBytes := feToBytes(d)
	for i := 0; i < 32; i++ {
		t[i] = dBytes[i] ^ aux[i]
	}

	// k' = tagged_hash("BIP0340/nonce", t || pkBytes || msg) mod n
	var nonceInput [96]byte
	copy(nonceInput[:32], t[:])
	copy(nonceInput[32:64], pkBytes[:])
	copy(nonceInput[64:96], msg[:])
	kHash := taggedHash("BIP0340/nonce", nonceInput[:])
	k := scalarFromBytes(kHash[:])

	// Reduce k mod n.
	if feCmp(k, curveN) >= 0 {
		k = scalarSub(k, curveN)
	}
	if scalarIsZero(k) {
		return sig, false
	}

	// R = k * G
	R := ScalarBaseMult(k)

	// If R.y is odd, negate k.
	if !feIsEven(R.Y) {
		k = scalarNeg(k)
	}

	rxBytes := feToBytes(R.X)

	// e = tagged_hash("BIP0340/challenge", R.x || P.x || msg) mod n
	e := computeChallenge(rxBytes[:], pkBytes[:], msg[:])

	// sig = R.x || (k + e*d) mod n
	s := scalarAdd(k, scalarMul(e, d))
	sBytes := feToBytes(s)

	copy(sig[:32], rxBytes[:])
	copy(sig[32:], sBytes[:])
	return sig, true
}

// PubKeyFromSecKey derives the x-only public key from a secret key.
// Returns zero [32]byte and false on failure.
func PubKeyFromSecKey(seckey [32]byte) (pubkey [32]byte, ok bool) {
	sk := scalarFromBytes(seckey[:])
	if scalarIsZero(sk) {
		return pubkey, false
	}
	P := ScalarBaseMult(sk)
	return feToBytes(P.X), true
}

// ECDH computes the shared secret x-coordinate: seckey * pubkey.
// pubkey is a 32-byte x-only public key.
// Returns the 32-byte x-coordinate and true on success.
func ECDH(seckey, pubkey [32]byte) ([32]byte, bool) {
	sk := scalarFromBytes(seckey[:])
	if scalarIsZero(sk) {
		return [32]byte{}, false
	}
	px := feFromBytes(pubkey[:])
	P, ok := LiftX(px)
	if !ok {
		return [32]byte{}, false
	}
	R := ScalarMult(pointFromAffine(P), sk).toAffine()
	if feIsZero(R.X) && feIsZero(R.Y) {
		return [32]byte{}, false
	}
	return feToBytes(R.X), true
}

// tagged_hash(tag, msg) = SHA256(SHA256(tag) || SHA256(tag) || msg)
func taggedHash(tag string, msg []byte) [32]byte {
	tagHash := sha256.Sum([]byte(tag))
	data := make([]byte, 0, 64+len(msg))
	data = append(data, tagHash[:]...)
	data = append(data, tagHash[:]...)
	data = append(data, msg...)
	return sha256.Sum(data)
}

func computeChallenge(rx, px, msg []byte) Fe {
	input := make([]byte, 0, 96)
	input = append(input, rx...)
	input = append(input, px...)
	input = append(input, msg...)
	hash := taggedHash("BIP0340/challenge", input)
	e := scalarFromBytes(hash[:])
	// Reduce mod n.
	if feCmp(e, curveN) >= 0 {
		e = scalarSub(e, curveN)
	}
	return e
}
