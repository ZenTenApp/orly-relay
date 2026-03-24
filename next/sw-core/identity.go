package main

import (
	"common/crypto/secp256k1"
	"common/helpers"
	"common/jsbridge/registry"
	"common/nostr"
)

// Identity domain — key management and signing.

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
	// Sync to registry for extension modules.
	registry.SetSeckey(hexKey)
	registry.SetPubkey(myPubkey)
	registry.SetHasKey(true)
}

func identitySetPubkey(hex string) {
	myPubkey = hex
	registry.SetPubkey(hex)
}

func identityClearKey() {
	seckey = [32]byte{}
	hasKey = false
	myPubkey = ""
	registry.SetSeckey("")
	registry.SetPubkey("")
	registry.SetHasKey(false)
}

func identitySignEvent(ev *nostr.Event) bool {
	if !hasKey {
		return false
	}
	aux := random32()
	return ev.Sign(seckey, aux)
}
