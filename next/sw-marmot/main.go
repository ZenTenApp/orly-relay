package main

import (
	"common/jsbridge/bc"
	"common/jsbridge/sw"
)

// Marmot SW — MLS DM proxy, crypto proxy chain.
// Communicates with shell/relay SWs via BroadcastChannel (client-local).

var bus bc.BC

func main() {
	initSharedState()
	initCryptoProxy()
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
	connectBus()
	busSend("shell", "[\"READY\"]")
}

func onInstall(event sw.Event) {
	sw.WaitUntil(event, func(done func()) {
		sw.SkipWaiting()
		done()
	})
}

func onActivate(event sw.Event) {
	sw.WaitUntil(event, func(done func()) {
		sw.ClaimClients(func() {
			sw.Log("marmot-sw: starting")
			busSend("shell", "[\"READY\"]")
			done()
		})
	})
}

func onFetch(event sw.Event) {
	// Marmot SW does not intercept fetches.
}

func onMessage(event sw.Event) {
	// Keepalive from loader iframe — just receiving the event keeps us alive.
}

func connectBus() {
	bus = bc.Open("smesh-bus", func(raw string) { onBusMessage(raw) })
}

func busSend(to, msg string) {
	bc.Send(bus, "{\"from\":\"marmot\",\"to\":"+jstr(to)+",\"msg\":"+msg+"}")
}

func onBusMessage(raw string) {
	from := jsonField(raw, "from")
	if from == "marmot" {
		return
	}
	to := jsonField(raw, "to")
	if to != "marmot" && to != "*" {
		return
	}
	msg := jsonFieldRaw(raw, "msg")
	if msg == "" {
		return
	}
	w := newMW(msg)
	msgType := w.str()

	switch msgType {
	case "PING":
		busSend("shell", "[\"READY\"]")
		return
	case "SET_KEY":
		identitySetKey(w.str())
	case "SET_PUBKEY":
		identitySetPubkey(w.str())
	case "CLEAR_KEY":
		identityClearKey()
	case "MLS_INIT":
		marmotInit(w.strs())
	case "MLS_SEND":
		recipient := w.str()
		content := w.str()
		marmotSend(recipient, content)
	case "MLS_SUB":
		marmotSubscribe()
	case "MLS_PUBLISH_KP":
		marmotPublishKP(w.strs())
	case "MLS_LIST_GROUPS":
		marmotListGroups()
	case "CRYPTO_RESULT":
		id := int(w.num())
		result := w.str()
		errMsg := w.str()
		// Forward to WASM module's crypto resolver
		handleCryptoResult(id, result, errMsg)
	}
}
