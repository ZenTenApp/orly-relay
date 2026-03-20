package main

import (
	"smesh3/crypto/secp256k1"
	"smesh3/crypto/sha256"
	"smesh3/helpers"
)

func main() {
	seckey, _ := helpers.HexDecode32("0000000000000000000000000000000000000000000000000000000000000003")
	pubkey, _ := secp256k1.PubKeyFromSecKey(seckey)
	println("pubkey: " + helpers.HexEncode(pubkey[:]))

	msg := sha256.Sum([]byte("hello nostr"))
	aux := sha256.Sum([]byte("aux randomness"))

	sig, ok := secp256k1.SignSchnorr(seckey, msg, aux)
	if !ok {
		println("sign: FAIL")
		return
	}
	if secp256k1.VerifySchnorr(pubkey, msg, sig) {
		println("verify: pass")
	} else {
		println("verify: FAIL")
	}

	println("signing 100x...")
	for i := 0; i < 100; i++ {
		msg[0] = byte(i)
		secp256k1.SignSchnorr(seckey, msg, aux)
	}
	println("sign done")

	msg[0] = 42
	sig, _ = secp256k1.SignSchnorr(seckey, msg, aux)
	println("verifying 100x...")
	for i := 0; i < 100; i++ {
		secp256k1.VerifySchnorr(pubkey, msg, sig)
	}
	println("verify done")
}
