package main

import (
	"common/helpers"
	"common/jsbridge/crypto"
)

// Identity — local copy of key, received via bus broadcast.

var (
	seckey   [32]byte
	hasKey   bool
	myPubkey string
)

func identitySetKey(hexKey string) {
	seckey = hexTo32(hexKey)
	hasKey = true
	pk := crypto.PubKeyFromSecKey(seckey[:])
	if pk != nil {
		myPubkey = helpers.HexEncode(pk)
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
