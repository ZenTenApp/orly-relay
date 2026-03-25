package main

import (
	"common/helpers"
	"common/jsbridge/sw"
	"common/jsbridge/ws"
)

// Marmot SW — runs on marmot.* subdomain for thread isolation.
// Handles MLS DM proxy via /__marmot backend WS.
// Communicates with shell SW via /__bus backend WS.

var (
	busConn    ws.Conn
	busReady   bool
	busPending []busMsgPending
)

type busMsgPending struct{ to, msg string }

func main() {
	initSharedState()
	initCryptoProxy()
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
	// Marmot SW does not intercept fetches.
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
	sw.Log("marmot-sw: connecting to bus")

	busConn = ws.Dial(wsURL,
		func(connID int, msg string) { onBusMessage(msg) },
		func(connID int) { onBusOpen() },
		func(connID int, code int, reason string) {
			sw.Log("marmot-sw: bus closed code=" + helpers.Itoa(int64(code)) + " reason=" + reason)
			busReady = false
			sw.SetTimeout(2000, func() { connectBus() })
		},
		func(connID int) {
			sw.Log("marmot-sw: bus error")
			busReady = false
			sw.SetTimeout(2000, func() { connectBus() })
		},
	)
}

func onBusOpen() {
	busReady = true
	sw.Log("marmot-sw: bus connected")
	ws.Send(busConn, "{\"role\":\"marmot\"}")
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
		if fn, ok := cryptoCBs[id]; ok {
			delete(cryptoCBs, id)
			fn(result, errMsg)
		}
	}
}
