package main

// Identity — pubkey only. No secret key stored.
// All signing/crypto proxied through signer extension.

var myPubkey string

func identitySetPubkey(hex string) {
	if myPubkey != "" && hex != myPubkey {
		// Identity change: close all marmot subs, reset relay, notify page.
		closeMarmotSubs()
		busSend("relay", "[\"CLEAR_KEY\"]")
		broadcastToClients("[\"RESUB\"]")
	}
	myPubkey = hex
}

func identityClearKey() {
	closeMarmotSubs()
	myPubkey = ""
}

func closeMarmotSubs() {
	for subID := range marmotSubs {
		busSend("relay", "[\"CLOSE\","+jstr(subID)+"]")
	}
	marmotSubs = nil
}
