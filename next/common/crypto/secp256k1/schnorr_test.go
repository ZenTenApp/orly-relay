package secp256k1

import (
	"common/helpers"
	"testing"
)

func TestPubKeyFromSecKey(t *testing.T) {
	// BIP-340 test vector 0.
	seckey, _ := helpers.HexDecode32("0000000000000000000000000000000000000000000000000000000000000003")
	expected := "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"

	pubkey, ok := PubKeyFromSecKey(seckey)
	if !ok {
		t.Fatal("PubKeyFromSecKey failed")
	}
	got := helpers.HexEncode(pubkey[:])

	if got != expected {
		t.Errorf("PubKeyFromSecKey:\ngot  %s\nwant %s", got, expected)
	}
}

func TestSignVerify(t *testing.T) {
	seckey, _ := helpers.HexDecode32("0000000000000000000000000000000000000000000000000000000000000003")
	pubkey, _ := PubKeyFromSecKey(seckey)
	msg, _ := helpers.HexDecode32("0000000000000000000000000000000000000000000000000000000000000000")
	aux, _ := helpers.HexDecode32("0000000000000000000000000000000000000000000000000000000000000000")

	sig, ok := SignSchnorr(seckey, msg, aux)
	if !ok {
		t.Fatal("SignSchnorr failed")
	}

	if !VerifySchnorr(pubkey, msg, sig) {
		t.Error("VerifySchnorr failed on own signature")
	}

	// Tamper with message.
	msg[0] ^= 1
	if VerifySchnorr(pubkey, msg, sig) {
		t.Error("VerifySchnorr should fail on tampered message")
	}
}

func TestBIP340Vector1(t *testing.T) {
	// Test vector 1 from BIP-340.
	seckey, _ := helpers.HexDecode32("0000000000000000000000000000000000000000000000000000000000000001")
	pubkey, ok := PubKeyFromSecKey(seckey)
	if !ok {
		t.Fatal("PubKeyFromSecKey failed")
	}
	expectedPubkey := "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"

	got := helpers.HexEncode(pubkey[:])
	if got != expectedPubkey {
		t.Errorf("pubkey mismatch:\ngot  %s\nwant %s", got, expectedPubkey)
	}
}
