package main

// Identity — pubkey only. No secret key.
// All signing proxied through crypto SW -> signer extension.

var myPubkey string

func identitySetPubkey(hex string) {
	myPubkey = hex
}

func identityClearKey() {
	myPubkey = ""
}
