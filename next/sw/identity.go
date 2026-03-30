package main

// Identity — pubkey only. No secret key stored.
// All signing/crypto proxied through signer extension.

var myPubkey string

func identitySetPubkey(hex string) {
	if myPubkey != "" && hex != myPubkey {
		// Identity change: close old marmot sub, reset relay, notify page.
		if currentMarmotSub != "" {
			busSend("relay", "[\"CLOSE\","+jstr(currentMarmotSub)+"]")
			currentMarmotSub = ""
		}
		busSend("relay", "[\"CLEAR_KEY\"]")
		broadcastToClients("[\"RESUB\"]")
	}
	myPubkey = hex
}

func identityClearKey() {
	if currentMarmotSub != "" {
		busSend("relay", "[\"CLOSE\","+jstr(currentMarmotSub)+"]")
		currentMarmotSub = ""
	}
	myPubkey = ""
}
