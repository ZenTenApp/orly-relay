package p256k1

import (
	"errors"
	"unsafe"
)

// ECDSASignature represents an ECDSA signature
type ECDSASignature struct {
	r, s Scalar
}

// ECDSASign creates an ECDSA signature for a message hash using a private key
func ECDSASign(sig *ECDSASignature, msghash32 []byte, seckey []byte) error {
	if len(msghash32) != 32 {
		return errors.New("message hash must be 32 bytes")
	}
	if len(seckey) != 32 {
		return errors.New("private key must be 32 bytes")
	}
	
	// Parse secret key
	var sec Scalar
	if !sec.setB32Seckey(seckey) {
		return errors.New("invalid private key")
	}
	
	// Parse message hash
	var msg Scalar
	msg.setB32(msghash32)
	
	// Generate nonce using RFC6979
	var nonceKey [64]byte
	copy(nonceKey[:32], msghash32)
	copy(nonceKey[32:], seckey)

	rng := NewRFC6979HMACSHA256(nonceKey[:])
	memclear(unsafe.Pointer(&nonceKey[0]), 64)
	
	var nonceBytes [32]byte
	rng.Generate(nonceBytes[:])
	
	// Parse nonce
	var nonce Scalar
	if !nonce.setB32Seckey(nonceBytes[:]) {
		// Retry with new nonce
		rng.Generate(nonceBytes[:])
		if !nonce.setB32Seckey(nonceBytes[:]) {
			rng.Finalize()
			rng.Clear()
			return errors.New("nonce generation failed")
		}
	}
	memclear(unsafe.Pointer(&nonceBytes[0]), 32)
	rng.Finalize()
	rng.Clear()
	
	// Compute R = nonce * G
	var rp GroupElementJacobian
	EcmultGen(&rp, &nonce)
	
	// Convert to affine
	var r GroupElementAffine
	r.setGEJ(&rp)
	r.x.normalize()
	r.y.normalize()
	
	// Extract r = X(R) mod n
	var rBytes [32]byte
	r.x.getB32(rBytes[:])
	
	sig.r.setB32(rBytes[:])
	if sig.r.isZero() {
		return errors.New("signature r is zero")
	}
	
	// Compute s = nonce^-1 * (msg + r * sec) mod n
	var n Scalar
	n.mul(&sig.r, &sec)
	n.add(&n, &msg)
	
	var nonceInv Scalar
	nonceInv.inverse(&nonce)
	sig.s.mul(&nonceInv, &n)
	
	// Normalize to low-S
	if sig.s.isHigh() {
		sig.s.condNegate(1)
	}
	
	if sig.s.isZero() {
		return errors.New("signature s is zero")
	}
	
	// Clear sensitive data
	sec.clear()
	msg.clear()
	nonce.clear()
	n.clear()
	nonceInv.clear()
	rp.clear()
	r.clear()
	
	return nil
}

// ECDSAVerify verifies an ECDSA signature against a message hash and public key
func ECDSAVerify(sig *ECDSASignature, msghash32 []byte, pubkey *PublicKey) bool {
	if len(msghash32) != 32 {
		return false
	}

	// Check signature components are non-zero
	if sig.r.isZero() || sig.s.isZero() {
		return false
	}

	// Reject high-S signatures (Bitcoin consensus rule BIP-146)
	if sig.s.isHigh() {
		return false
	}

	// Parse message hash
	var msg Scalar
	msg.setB32(msghash32)

	// Load public key
	var pubkeyPoint GroupElementAffine
	pubkeyPoint.fromBytes(pubkey.data[:])
	if pubkeyPoint.isInfinity() {
		return false
	}

	// Compute s^-1 mod n
	var sInv Scalar
	sInv.inverse(&sig.s)

	// Compute u1 = msg * s^-1 mod n
	var u1 Scalar
	u1.mul(&msg, &sInv)

	// Compute u2 = r * s^-1 mod n
	var u2 Scalar
	u2.mul(&sig.r, &sInv)

	// Compute R = u1*G + u2*P using combined Strauss algorithm
	// This shares doublings between both multiplications for better performance
	var pubkeyJac GroupElementJacobian
	pubkeyJac.setGE(&pubkeyPoint)

	var R GroupElementJacobian
	EcmultCombined(&R, &pubkeyJac, &u2, &u1)

	if R.isInfinity() {
		return false
	}

	// Convert R to affine
	var RAff GroupElementAffine
	RAff.setGEJ(&R)
	RAff.x.normalize()

	// Extract X(R) mod n
	var rBytes [32]byte
	RAff.x.getB32(rBytes[:])

	var computedR Scalar
	computedR.setB32(rBytes[:])

	// Compare r with X(R) mod n
	return sig.r.equal(&computedR)
}

// ECDSASignatureCompact represents a compact 64-byte signature (r || s)
type ECDSASignatureCompact [64]byte

// ToCompact converts an ECDSA signature to compact format
func (sig *ECDSASignature) ToCompact() *ECDSASignatureCompact {
	var compact ECDSASignatureCompact
	sig.r.getB32(compact[:32])
	sig.s.getB32(compact[32:])
	return &compact
}

// FromCompact converts a compact signature to ECDSA signature format
func (sig *ECDSASignature) FromCompact(compact *ECDSASignatureCompact) error {
	sig.r.setB32(compact[:32])
	sig.s.setB32(compact[32:64])
	
	if sig.r.isZero() || sig.s.isZero() {
		return errors.New("invalid signature: r or s is zero")
	}
	
	return nil
}

// VerifyCompact verifies a compact signature
func ECDSAVerifyCompact(compact *ECDSASignatureCompact, msghash32 []byte, pubkey *PublicKey) bool {
	var sig ECDSASignature
	if err := sig.FromCompact(compact); err != nil {
		return false
	}
	return ECDSAVerify(&sig, msghash32, pubkey)
}

// SignCompact creates a compact signature
func ECDSASignCompact(compact *ECDSASignatureCompact, msghash32 []byte, seckey []byte) error {
	var sig ECDSASignature
	if err := ECDSASign(&sig, msghash32, seckey); err != nil {
		return err
	}
	*compact = *sig.ToCompact()
	return nil
}

// SerializeDER serializes the signature in DER format
func (sig *ECDSASignature) SerializeDER() []byte {
	var rBytes, sBytes [32]byte
	sig.r.getB32(rBytes[:])
	sig.s.getB32(sBytes[:])

	// Remove leading zeros and add 0x00 prefix if high bit set
	rStart := 0
	for rStart < 31 && rBytes[rStart] == 0 {
		rStart++
	}
	sStart := 0
	for sStart < 31 && sBytes[sStart] == 0 {
		sStart++
	}

	rLen := 32 - rStart
	sLen := 32 - sStart

	// Add 0x00 prefix if high bit is set (to keep number positive)
	rPad := 0
	if rBytes[rStart]&0x80 != 0 {
		rPad = 1
	}
	sPad := 0
	if sBytes[sStart]&0x80 != 0 {
		sPad = 1
	}

	// DER format: 0x30 [total-len] 0x02 [r-len] [r] 0x02 [s-len] [s]
	totalLen := 2 + rLen + rPad + 2 + sLen + sPad
	der := make([]byte, 2+totalLen)

	der[0] = 0x30
	der[1] = byte(totalLen)

	pos := 2
	der[pos] = 0x02
	der[pos+1] = byte(rLen + rPad)
	pos += 2
	if rPad == 1 {
		der[pos] = 0x00
		pos++
	}
	copy(der[pos:], rBytes[rStart:])
	pos += rLen

	der[pos] = 0x02
	der[pos+1] = byte(sLen + sPad)
	pos += 2
	if sPad == 1 {
		der[pos] = 0x00
		pos++
	}
	copy(der[pos:], sBytes[sStart:])

	return der
}

// ParseDER parses a DER-encoded signature
func (sig *ECDSASignature) ParseDER(der []byte) error {
	if len(der) < 8 {
		return errors.New("DER signature too short")
	}
	if der[0] != 0x30 {
		return errors.New("invalid DER signature: expected 0x30")
	}

	totalLen := int(der[1])
	if len(der) < 2+totalLen {
		return errors.New("DER signature length mismatch")
	}

	pos := 2

	// Parse R
	if der[pos] != 0x02 {
		return errors.New("invalid DER signature: expected 0x02 for R")
	}
	rLen := int(der[pos+1])
	pos += 2
	if pos+rLen > len(der) {
		return errors.New("DER signature: R length overflow")
	}

	rBytes := der[pos : pos+rLen]
	pos += rLen

	// Skip leading zero if present
	if len(rBytes) > 0 && rBytes[0] == 0x00 {
		rBytes = rBytes[1:]
	}
	if len(rBytes) > 32 {
		return errors.New("DER signature: R too large")
	}

	// Parse S
	if pos >= len(der) || der[pos] != 0x02 {
		return errors.New("invalid DER signature: expected 0x02 for S")
	}
	sLen := int(der[pos+1])
	pos += 2
	if pos+sLen > len(der) {
		return errors.New("DER signature: S length overflow")
	}

	sBytes := der[pos : pos+sLen]

	// Skip leading zero if present
	if len(sBytes) > 0 && sBytes[0] == 0x00 {
		sBytes = sBytes[1:]
	}
	if len(sBytes) > 32 {
		return errors.New("DER signature: S too large")
	}

	// Pad to 32 bytes and set
	var rPadded, sPadded [32]byte
	copy(rPadded[32-len(rBytes):], rBytes)
	copy(sPadded[32-len(sBytes):], sBytes)

	sig.r.setB32(rPadded[:])
	sig.s.setB32(sPadded[:])

	if sig.r.isZero() || sig.s.isZero() {
		return errors.New("invalid signature: r or s is zero")
	}

	return nil
}

// ECDSASignDER signs and returns a DER-encoded signature
func ECDSASignDER(msghash32 []byte, seckey []byte) ([]byte, error) {
	var sig ECDSASignature
	if err := ECDSASign(&sig, msghash32, seckey); err != nil {
		return nil, err
	}
	return sig.SerializeDER(), nil
}

// ECDSAVerifyDER verifies a DER-encoded signature
func ECDSAVerifyDER(sigDER []byte, msghash32 []byte, pubkey *PublicKey) bool {
	var sig ECDSASignature
	if err := sig.ParseDER(sigDER); err != nil {
		return false
	}
	return ECDSAVerify(&sig, msghash32, pubkey)
}

// GetR returns the R component of the signature
func (sig *ECDSASignature) GetR() []byte {
	var r [32]byte
	sig.r.getB32(r[:])
	return r[:]
}

// GetS returns the S component of the signature
func (sig *ECDSASignature) GetS() []byte {
	var s [32]byte
	sig.s.getB32(s[:])
	return s[:]
}

// IsLowS returns true if the S value is in the lower half of the curve order
func (sig *ECDSASignature) IsLowS() bool {
	return !sig.s.isHigh()
}

// NormalizeLowS ensures the S value is in the lower half of the curve order
// This is required by Bitcoin's consensus rules (BIP-66/BIP-146)
func (sig *ECDSASignature) NormalizeLowS() {
	if sig.s.isHigh() {
		sig.s.condNegate(1)
	}
}

// ECDSARecoverableSignature represents an ECDSA signature with recovery information
type ECDSARecoverableSignature struct {
	r, s  Scalar
	recid int // Recovery ID (0-3)
}

// ECDSASignRecoverable creates an ECDSA signature with recovery ID
func ECDSASignRecoverable(sig *ECDSARecoverableSignature, msghash32 []byte, seckey []byte) error {
	if len(msghash32) != 32 {
		return errors.New("message hash must be 32 bytes")
	}
	if len(seckey) != 32 {
		return errors.New("private key must be 32 bytes")
	}

	// Parse secret key
	var sec Scalar
	if !sec.setB32Seckey(seckey) {
		return errors.New("invalid private key")
	}

	// Parse message hash
	var msg Scalar
	msg.setB32(msghash32)

	// Generate nonce using RFC6979
	var nonceKey [64]byte
	copy(nonceKey[:32], msghash32)
	copy(nonceKey[32:], seckey)

	rng := NewRFC6979HMACSHA256(nonceKey[:])
	memclear(unsafe.Pointer(&nonceKey[0]), 64)

	var nonceBytes [32]byte
	rng.Generate(nonceBytes[:])

	// Parse nonce
	var nonce Scalar
	if !nonce.setB32Seckey(nonceBytes[:]) {
		rng.Generate(nonceBytes[:])
		if !nonce.setB32Seckey(nonceBytes[:]) {
			rng.Finalize()
			rng.Clear()
			return errors.New("nonce generation failed")
		}
	}
	memclear(unsafe.Pointer(&nonceBytes[0]), 32)
	rng.Finalize()
	rng.Clear()

	// Compute R = nonce * G
	var rp GroupElementJacobian
	EcmultGen(&rp, &nonce)

	// Convert to affine
	var r GroupElementAffine
	r.setGEJ(&rp)
	r.x.normalize()
	r.y.normalize()

	// Determine recovery ID based on Y coordinate parity and overflow
	sig.recid = 0
	if r.y.isOdd() {
		sig.recid |= 1
	}

	// Extract r = X(R) mod n
	var rBytes [32]byte
	r.x.getB32(rBytes[:])

	sig.r.setB32(rBytes[:])

	// Note: For secp256k1, the case where X(R) >= n is extremely rare (probability ~2^-128)
	// We don't set bit 1 of recid here since the probability is negligible

	if sig.r.isZero() {
		return errors.New("signature r is zero")
	}

	// Compute s = nonce^-1 * (msg + r * sec) mod n
	var n Scalar
	n.mul(&sig.r, &sec)
	n.add(&n, &msg)

	var nonceInv Scalar
	nonceInv.inverse(&nonce)
	sig.s.mul(&nonceInv, &n)

	// Normalize to low-S (flip recid bit 0 if we negate)
	if sig.s.isHigh() {
		sig.s.condNegate(1)
		sig.recid ^= 1
	}

	if sig.s.isZero() {
		return errors.New("signature s is zero")
	}

	// Clear sensitive data
	sec.clear()
	msg.clear()
	nonce.clear()
	n.clear()
	nonceInv.clear()
	rp.clear()
	r.clear()

	return nil
}

// ToCompact returns the 64-byte compact signature and recovery ID
func (sig *ECDSARecoverableSignature) ToCompact() ([]byte, int) {
	compact := make([]byte, 64)
	sig.r.getB32(compact[:32])
	sig.s.getB32(compact[32:])
	return compact, sig.recid
}

// FromCompact parses a compact signature with recovery ID
func (sig *ECDSARecoverableSignature) FromCompact(compact []byte, recid int) error {
	if len(compact) != 64 {
		return errors.New("compact signature must be 64 bytes")
	}
	if recid < 0 || recid > 3 {
		return errors.New("recovery ID must be 0-3")
	}

	sig.r.setB32(compact[:32])
	sig.s.setB32(compact[32:])
	sig.recid = recid

	if sig.r.isZero() || sig.s.isZero() {
		return errors.New("invalid signature: r or s is zero")
	}

	return nil
}

// ECDSARecover recovers the public key from an ECDSA signature
func ECDSARecover(pubkey *PublicKey, sig *ECDSARecoverableSignature, msghash32 []byte) error {
	if len(msghash32) != 32 {
		return errors.New("message hash must be 32 bytes")
	}

	if sig.r.isZero() || sig.s.isZero() {
		return errors.New("invalid signature")
	}

	if sig.recid < 0 || sig.recid > 3 {
		return errors.New("invalid recovery ID")
	}

	// Parse message hash
	var msg Scalar
	msg.setB32(msghash32)

	// Recover the X coordinate of R
	var rBytes [32]byte
	sig.r.getB32(rBytes[:])

	// If recid bit 1 is set, we need to add n to r to get the X coordinate
	// This is very rare for secp256k1 (probability ~2^-128)
	var rx FieldElement
	if err := rx.setB32(rBytes[:]); err != nil {
		return errors.New("invalid r value")
	}

	if sig.recid&2 != 0 {
		// Add the group order to rx (very rare case)
		// For secp256k1, this means rx + n, but since n < p, we need to handle overflow
		// In practice, this case almost never happens
		return errors.New("recovery with overflow not implemented (extremely rare case)")
	}

	// Recover the Y coordinate from X
	var rPoint GroupElementAffine
	odd := (sig.recid & 1) != 0
	if !rPoint.setXOVar(&rx, odd) {
		return errors.New("failed to recover R point")
	}

	// Compute r^-1 mod n
	var rInv Scalar
	rInv.inverse(&sig.r)

	// Compute u1 = -msg * r^-1 mod n
	var u1 Scalar
	u1.mul(&msg, &rInv)
	u1.negate(&u1)

	// Compute u2 = s * r^-1 mod n
	var u2 Scalar
	u2.mul(&sig.s, &rInv)

	// Compute Q = u1*G + u2*R using combined Strauss algorithm
	var rJac GroupElementJacobian
	rJac.setGE(&rPoint)

	var Q GroupElementJacobian
	EcmultCombined(&Q, &rJac, &u2, &u1)

	if Q.isInfinity() {
		return errors.New("recovered point is infinity")
	}

	// Convert to affine and save to pubkey
	var qAff GroupElementAffine
	qAff.setGEJ(&Q)
	qAff.x.normalize()
	qAff.y.normalize()

	pubkeySave(pubkey, &qAff)

	return nil
}

// ECDSARecoverCompact is a convenience function to recover a public key from a compact signature
func ECDSARecoverCompact(pubkey *PublicKey, sig64 []byte, recid int, msghash32 []byte) error {
	var sig ECDSARecoverableSignature
	if err := sig.FromCompact(sig64, recid); err != nil {
		return err
	}
	return ECDSARecover(pubkey, &sig, msghash32)
}

