package main

import (
	"common/helpers"
	"common/jsbridge/sw"
	"common/jsbridge/ws"
)

// Bus client — connects to /__bus on own origin to route messages
// between the shell SW and satellite SWs (marmot, relay).

var (
	busConn    ws.Conn
	busReady   bool
	busPending []busMsgPending
)

type busMsgPending struct{ to, msg string }

func connectBus() {
	wsURL := sw.Origin()
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += "/__bus"
	sw.Log("shell-sw: connecting to bus")

	busConn = ws.Dial(wsURL,
		func(connID int, msg string) { onBusMessage(msg) },
		func(connID int) { onBusOpen() },
		func(connID int, code int, reason string) {
			sw.Log("shell-sw: bus closed")
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
	sw.Log("shell-sw: bus connected")
	ws.Send(busConn, "{\"role\":\"shell\"}")
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
	case "FWD":
		// Relay/marmot SW wants to send to a specific browser client.
		clientID := w.str()
		innerMsg := w.raw()
		if clientID == "" {
			broadcastToClients(innerMsg)
		} else {
			sendToClient(clientID, innerMsg)
		}
	case "FWD_ALL":
		// Relay/marmot SW wants to broadcast to all browser clients.
		innerMsg := w.raw()
		broadcastToClients(innerMsg)
	case "DM_RECEIVED":
		// Marmot SW forwarded a DM — route to relay for IDB storage.
		dmJSON := w.raw()
		busSend("relay", "[\"SAVE_DM\","+dmJSON+"]")
	case "MLS_GROUPS":
		// Marmot SW forwarded groups list — broadcast to clients.
		raw := w.raw()
		broadcastToClients("[\"MLS_GROUPS\"," + raw + "]")
	case "DM_SENT":
		tsStr := w.raw()
		broadcastToClients("[\"DM_SENT\"," + tsStr + "]")
	case "MLS_STATUS":
		text := w.raw()
		broadcastToClients("[\"MLS_STATUS\"," + text + "]")
	case "CRYPTO_REQ":
		// Satellite SW needs crypto proxy through the main page.
		from := w.str()
		id := int(w.num())
		method := w.str()
		peerPubkey := w.str()
		data := w.str()
		cryptoProxy(method, peerPubkey, data, func(result, errMsg string) {
			busSend(from, "[\"CRYPTO_RESULT\","+helpers.Itoa(int64(id))+","+jstr(result)+","+jstr(errMsg)+"]")
		})
	}
}

// strsJSON serializes a string slice to a JSON array.
func strsJSON(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	out := "["
	for i, s := range ss {
		if i > 0 {
			out += ","
		}
		out += jstr(s)
	}
	return out + "]"
}
