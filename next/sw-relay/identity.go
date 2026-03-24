package main

import (
	"common/crypto/secp256k1"
	"common/helpers"
	"common/nostr"
)

// Identity — local copy of key, received via bus broadcast.
// Relay SW needs signing for NIP-42 relay auth.

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

func identitySignEvent(ev *nostr.Event) bool {
	if !hasKey {
		return false
	}
	aux := random32()
	return ev.Sign(seckey, aux)
}
