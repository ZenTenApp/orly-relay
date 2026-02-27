package p256k1

import (
	"crypto/rand"
	"errors"
)

// ECSeckeyVerify verifies that a 32-byte array is a valid secret key
func ECSeckeyVerify(seckey []byte) bool {
	if len(seckey) != 32 {
		return false
	}
	
	var scalar Scalar
	return scalar.setB32Seckey(seckey)
}

// ECSeckeyNegate negates a secret key in place
func ECSeckeyNegate(seckey []byte) bool {
	if len(seckey) != 32 {
		return false
	}
	
	var scalar Scalar
	if !scalar.setB32Seckey(seckey) {
		return false
	}
	
	scalar.negate(&scalar)
	scalar.getB32(seckey)
	return true
}

// ECSeckeyGenerate generates a new random secret key
func ECSeckeyGenerate() ([]byte, error) {
	seckey := make([]byte, 32)
	for {
		if _, err := rand.Read(seckey); err != nil {
			return nil, err
		}
		
		if ECSeckeyVerify(seckey) {
			return seckey, nil
		}
	}
}

// ECKeyPairGenerate generates a new key pair (private key and public key)
func ECKeyPairGenerate() (seckey []byte, pubkey *PublicKey, err error) {
	seckey, err = ECSeckeyGenerate()
	if err != nil {
		return nil, nil, err
	}
	
	pubkey = &PublicKey{}
	if err := ECPubkeyCreate(pubkey, seckey); err != nil {
		return nil, nil, err
	}
	
	return seckey, pubkey, nil
}

// ECSeckeyTweakAdd adds a tweak to a secret key: seckey = seckey + tweak mod n
func ECSeckeyTweakAdd(seckey []byte, tweak []byte) error {
	if len(seckey) != 32 {
		return errors.New("secret key must be 32 bytes")
	}
	if len(tweak) != 32 {
		return errors.New("tweak must be 32 bytes")
	}
	
	var sec, tw Scalar
	if !sec.setB32Seckey(seckey) {
		return errors.New("invalid secret key")
	}
	if !tw.setB32Seckey(tweak) {
		return errors.New("invalid tweak")
	}
	
	// Add tweak
	sec.add(&sec, &tw)
	
	// Check if result is valid
	if sec.isZero() {
		return errors.New("resulting secret key is zero")
	}
	
	// Get result
	sec.getB32(seckey)
	return nil
}

// ECSeckeyTweakMul multiplies a secret key by a tweak: seckey = seckey * tweak mod n
func ECSeckeyTweakMul(seckey []byte, tweak []byte) error {
	if len(seckey) != 32 {
		return errors.New("secret key must be 32 bytes")
	}
	if len(tweak) != 32 {
		return errors.New("tweak must be 32 bytes")
	}
	
	var sec, tw Scalar
	if !sec.setB32Seckey(seckey) {
		return errors.New("invalid secret key")
	}
	if !tw.setB32Seckey(tweak) {
		return errors.New("invalid tweak")
	}
	
	// Multiply by tweak
	sec.mul(&sec, &tw)
	
	// Check if result is valid
	if sec.isZero() {
		return errors.New("resulting secret key is zero")
	}
	
	// Get result
	sec.getB32(seckey)
	return nil
}

// ECPubkeyTweakAdd adds a tweak to a public key: pubkey = pubkey + tweak*G
func ECPubkeyTweakAdd(pubkey *PublicKey, tweak []byte) error {
	if len(tweak) != 32 {
		return errors.New("tweak must be 32 bytes")
	}
	
	var tw Scalar
	if !tw.setB32Seckey(tweak) {
		return errors.New("invalid tweak")
	}
	
	// Load public key
	var pubkeyPoint GroupElementAffine
	pubkeyPoint.fromBytes(pubkey.data[:])
	if pubkeyPoint.isInfinity() {
		return errors.New("invalid public key")
	}
	
	// Compute tweak*G
	var tweakG GroupElementJacobian
	EcmultGen(&tweakG, &tw)
	
	// Add to public key
	var pubkeyJac GroupElementJacobian
	pubkeyJac.setGE(&pubkeyPoint)
	
	// result = pubkey + tweak*G
	var result GroupElementJacobian
	result.addVar(&pubkeyJac, &tweakG)
	
	// Check if result is infinity
	if result.isInfinity() {
		return errors.New("resulting public key is infinity")
	}
	
	// Convert back to affine and store
	var resultAff GroupElementAffine
	resultAff.setGEJ(&result)
	resultAff.toBytes(pubkey.data[:])
	
	return nil
}

// ECPubkeyTweakMul multiplies a public key by a tweak: pubkey = pubkey * tweak
func ECPubkeyTweakMul(pubkey *PublicKey, tweak []byte) error {
	if len(tweak) != 32 {
		return errors.New("tweak must be 32 bytes")
	}
	
	var tw Scalar
	if !tw.setB32Seckey(tweak) {
		return errors.New("invalid tweak")
	}
	
	// Load public key
	var pubkeyPoint GroupElementAffine
	pubkeyPoint.fromBytes(pubkey.data[:])
	if pubkeyPoint.isInfinity() {
		return errors.New("invalid public key")
	}
	
	// Multiply by tweak using binary method
	var pubkeyJac GroupElementJacobian
	pubkeyJac.setGE(&pubkeyPoint)
	
	var result GroupElementJacobian
	result.setInfinity()
	var base GroupElementJacobian
	base = pubkeyJac
	
	// Simple binary method
	for i := 0; i < 256; i++ {
		if i > 0 {
			result.double(&result)
		}
		bit := tw.getBits(uint(255-i), 1)
		if bit != 0 {
			if result.isInfinity() {
				result = base
			} else {
				result.addVar(&result, &base)
			}
		}
	}
	
	// Check if result is infinity
	if result.isInfinity() {
		return errors.New("resulting public key is infinity")
	}
	
	// Convert back to affine and store
	var resultAff GroupElementAffine
	resultAff.setGEJ(&result)
	resultAff.toBytes(pubkey.data[:])
	
	return nil
}

