package main

import (
	"common/crypto/secp256k1"
	"common/helpers"
)

// Identity domain — key management.
// Shell SW stores keys for pubkey derivation. Signing is done by relay SW.

var (
	seckey   [32]byte
	hasKey   bool
	myPubkey string
)

func identitySetKey(hexKey string) {
	seckey = hexTo32(hexKey)
	hasKey = true
	pk, ok := secp256k1.PubKeyFromSecKey(seckey)
	if ok {
		myPubkey = helpers.HexEncode(pk[:])
	}
}

func identitySetPubkey(hex string) {
	myPubkey = hex
}

func identityClearKey() {
	seckey = [32]byte{}
	hasKey = false
	myPubkey = ""
}
