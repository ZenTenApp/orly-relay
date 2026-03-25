package main

import (
	"common/jsbridge/sw"
	"common/jsbridge/ws"
)

// Relay SW — runs on relay.* subdomain for thread isolation.
// Handles relay pool, subscriptions, event storage, DM caching.
// Communicates with shell SW via /__bus backend WS.

var (
	busConn    ws.Conn
	busReady   bool
	busPending []busMsgPending
)

type busMsgPending struct{ to, msg string }

func main() {
	initSharedState()
	initRouter()
	initRelayProxy()
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
	connectBus()
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
			done()
		})
	})
}

func onFetch(event sw.Event) {
	// Relay SW does not intercept fetches.
}

func onMessage(event sw.Event) {
	// No client messages — all communication via bus.
}

func connectBus() {
	wsURL := sw.Origin()
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += "/__bus"
	sw.Log("relay-sw: connecting to bus")

	busConn = ws.Dial(wsURL,
		func(connID int, msg string) { onBusMessage(msg) },
		func(connID int) { onBusOpen() },
		func(connID int, code int, reason string) {
			sw.Log("relay-sw: bus closed")
			busReady = false
			sw.SetTimeout(2000, func() { connectBus() })
		},
		func(connID int) {
			busReady = false
			sw.SetTimeout(2000, func() { connectBus() })
		},
	)
}

func onBusOpen() {
	busReady = true
	sw.Log("relay-sw: bus connected")
	ws.Send(busConn, "{\"role\":\"relay\"}")
	for _, p := range busPending {
		ws.Send(busConn, "{\"to\":"+jstr(p.to)+",\"msg\":"+p.msg+"}")
	}
	busPending = nil
}

func busSend(to, msg string) {
	if !busReady {
		busPending = append(busPending, busMsgPending{to, msg})
		return
	}
	ws.Send(busConn, "{\"to\":"+jstr(to)+",\"msg\":"+msg+"}")
}

func onBusMessage(msg string) {
	w := newMW(msg)
	msgType := w.str()

	switch msgType {
	// Identity propagation.
	case "SET_KEY":
		identitySetKey(w.str())
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
		cacheSaveDM(dmJSON, func(string) {})

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
