//go:build !js

package p256k1

import (
	"encoding/hex"
	"testing"

	dcrd "git.smesh.lol/orly/pkg/nostr/crypto/ec/secp256k1"
)

// TestSxSDcrd compares p256k1 pubkey generation against the vendored Decred
// secp256k1 library for a range of known secret keys.
// This validates the native (64-bit) code path.
func TestSxSDcrd(t *testing.T) {
	secrets := []string{
		"0000000000000000000000000000000000000000000000000000000000000001",
		"0000000000000000000000000000000000000000000000000000000000000002",
		"0000000000000000000000000000000000000000000000000000000000000003",
		"0000000000000000000000000000000000000000000000000000000000000007",
		"e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35",
		"deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef",
		"7fffffffffffffffffffffffffffffff5d576e7357a4501ddfe92f46681b20a0",
		// Near-order edge cases
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd036413e",
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364140",
		// GLV lambda
		"5363ad4cc05c30e0a5261c028812645a122e22ea20816678df02967c1b23bd72",
	}

	for _, secHex := range secrets {
		secBytes, _ := hex.DecodeString(secHex)

		// --- Decred ---
		dcrdKey := dcrd.SecKeyFromBytes(secBytes)
		dcrdPub := dcrdKey.PubKey()
		dcrdSer := dcrdPub.SerializeCompressed()
		dcrdX := hex.EncodeToString(dcrdSer[1:33])

		// --- p256k1 ---
		var pubkey PublicKey
		if err := ECPubkeyCreate(&pubkey, secBytes); err != nil {
			t.Errorf("[%s] p256k1 ECPubkeyCreate failed: %v", secHex[:8], err)
			continue
		}
		var p256Ser [33]byte
		ECPubkeySerialize(p256Ser[:], &pubkey, ECCompressed)
		p256X := hex.EncodeToString(p256Ser[1:33])

		if dcrdX != p256X {
			t.Errorf("[%s] X mismatch:\n  dcrd:  %s\n  p256k1: %s", secHex[:8], dcrdX, p256X)
		}
		if dcrdSer[0] != p256Ser[0] {
			t.Errorf("[%s] parity mismatch: dcrd=0x%02x p256k1=0x%02x", secHex[:8], dcrdSer[0], p256Ser[0])
		}

		// Cross-validate: parse p256k1 pubkey with Decred
		_, err := dcrd.ParsePubKey(p256Ser[:])
		if err != nil {
			t.Errorf("[%s] Decred rejects p256k1 pubkey: %v", secHex[:8], err)
		}

		// Cross-validate: parse Decred pubkey with p256k1
		var p256FromDcrd PublicKey
		if err := ECPubkeyParse(&p256FromDcrd, dcrdSer[:]); err != nil {
			t.Errorf("[%s] p256k1 rejects Decred pubkey: %v", secHex[:8], err)
		}

		t.Logf("[%s] OK  x=%s parity=0x%02x", secHex[:8], dcrdX, dcrdSer[0])
	}
}

// TestSxSDcrdECDH compares ECDH shared secret computation between both libs.
func TestSxSDcrdECDH(t *testing.T) {
	// Alice and Bob each have a secret key
	aliceSec, _ := hex.DecodeString("e8f32e723decf4051aefac8e2c93c9c5b214313817cdb01a1494b917c8436b35")
	bobSec, _ := hex.DecodeString("deadbeefcafebabe1234567890abcdef0123456789abcdef0123456789abcdef")

	// --- Decred ECDH ---
	dcrdAlice := dcrd.SecKeyFromBytes(aliceSec)
	dcrdBob := dcrd.SecKeyFromBytes(bobSec)

	dcrdShared1 := dcrd.GenerateSharedSecret(dcrdAlice, dcrdBob.PubKey())
	dcrdShared2 := dcrd.GenerateSharedSecret(dcrdBob, dcrdAlice.PubKey())

	if hex.EncodeToString(dcrdShared1) != hex.EncodeToString(dcrdShared2) {
		t.Fatal("Decred ECDH not symmetric")
	}

	// --- p256k1 ECDH ---
	// Compute Bob's pubkey with p256k1
	var bobPub PublicKey
	ECPubkeyCreate(&bobPub, bobSec)
	var bobSer [33]byte
	ECPubkeySerialize(bobSer[:], &bobPub, ECCompressed)

	// Compute Alice's pubkey with p256k1
	var alicePub PublicKey
	ECPubkeyCreate(&alicePub, aliceSec)
	var aliceSer [33]byte
	ECPubkeySerialize(aliceSer[:], &alicePub, ECCompressed)

	// Use p256k1's EcmultConst for ECDH: alice_sec * bob_pub
	var aliceScalar Scalar
	aliceScalar.setB32(aliceSec)
	var bobPoint GroupElementAffine
	pubkeyLoad(&bobPoint, &bobPub)

	var sharedJac GroupElementJacobian
	EcmultConst(&sharedJac, &bobPoint, &aliceScalar)
	var sharedAff GroupElementAffine
	sharedAff.setGEJ(&sharedJac)
	sharedAff.x.normalize()
	var p256SharedX [32]byte
	sharedAff.x.getB32(p256SharedX[:])

	// Decred ECDH returns just the X coordinate
	// Compare
	dcrdSharedX := hex.EncodeToString(dcrdShared1)
	p256SharedXHex := hex.EncodeToString(p256SharedX[:])

	t.Logf("Decred ECDH shared X: %s", dcrdSharedX)
	t.Logf("p256k1 ECDH shared X: %s", p256SharedXHex)

	if dcrdSharedX != p256SharedXHex {
		t.Errorf("ECDH X mismatch")
	}
}
