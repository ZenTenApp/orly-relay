package main

// Identity — pubkey only. No secret key stored.
// All signing/crypto proxied through signer extension.

var myPubkey string

func identitySetPubkey(hex string) {
	myPubkey = hex
}

func identityClearKey() {
	myPubkey = ""
}
