package main

import (
	"common/jsbridge/bc"
	"common/jsbridge/sw"
)

// Relay SW — relay pool, subscriptions, event storage, DM caching.
// Communicates with shell/marmot SWs via BroadcastChannel (client-local).

var bus bc.BC

func main() {
	initSharedState()
	initRouter()
	initRelayProxy()
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
	connectBus()
	// Send READY on every start — not just onActivate, which doesn't re-fire
	// when the browser stops and restarts the SW thread.
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
			sw.Log("relay-sw: starting")
			busSend("shell", "[\"READY\"]")
			done()
		})
	})
}

func onFetch(event sw.Event) {
	// Relay SW does not intercept fetches.
}

func onMessage(event sw.Event) {
	// Keepalive from loader iframe — just receiving the event keeps us alive.
}

func connectBus() {
	bus = bc.Open("smesh-bus", func(raw string) { onBusMessage(raw) })
}

func busSend(to, msg string) {
	bc.Send(bus, "{\"from\":\"relay\",\"to\":"+jstr(to)+",\"msg\":"+msg+"}")
}

func onBusMessage(raw string) {
	from := jsonField(raw, "from")
	if from == "relay" {
		return
	}
	to := jsonField(raw, "to")
	if to != "relay" && to != "*" {
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

	// Identity propagation.
	case "SET_KEY":
		k := w.str()
		identitySetKey(k)
	case "SET_PUBKEY":
		identitySetPubkey(w.str())
	case "CLEAR_KEY":
		identityClearKey()
		writeRelays = nil

	// Relay operations.
	case "SET_WRITE_RELAYS":
		writeRelays = w.strs()
	case "REQ":
		clientID := w.str()
		subID := w.str()
		filterRaw := w.raw()
		routerReq(clientID, subID, filterRaw)
	case "CLOSE":
		subID := w.str()
		routerClose(subID)
	case "EVENT":
		clientID := w.str()
		eventRaw := w.raw()
		routerPublish(clientID, eventRaw)
	case "PROXY":
		clientID := w.str()
		subID := w.str()
		filterRaw := w.raw()
		relayURLs := w.strs()
		routerProxy(clientID, subID, filterRaw, relayURLs)
	case "RELAY_INFO":
		clientID := w.str()
		relayURL := w.str()
		handleRelayInfo(clientID, relayURL)
	case "SIGN":
		clientID := w.str()
		requestID := w.str()
		eventRaw := w.raw()
		routerSign(clientID, requestID, eventRaw)
	case "BROADCAST":
		clientID := w.str()
		pubkey := w.str()
		relayURLs := w.strs()
		routerBroadcast(clientID, pubkey, relayURLs)

	// DM storage.
	case "DM_LIST":
		clientID := w.str()
		routerDMList(clientID)
	case "DM_HISTORY":
		clientID := w.str()
		peer := w.str()
		limit := int(w.num())
		until := w.num()
		routerDMHistory(clientID, peer, limit, until)
	case "SAVE_DM":
		dmJSON := w.raw()
		cacheSaveDM(dmJSON, func(result string) {
			if result != "duplicate" {
				fwdAll("[\"DM_RECEIVED\"," + dmJSON + "]")
			}
		})
	case "SAVE_DM_QUIET":
		dmJSON := w.raw()
		cacheSaveDM(dmJSON, func(result string) {})

	// Crypto proxy result from shell.
	case "CRYPTO_RESULT":
		id := int(w.num())
		result := w.str()
		errMsg := w.str()
		if fn, ok := cryptoCBs[id]; ok {
			delete(cryptoCBs, id)
			fn(result, errMsg)
		}
	}

}
