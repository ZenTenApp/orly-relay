package main

import (
	"common/helpers"
	"common/jsbridge/bc"
	"common/jsbridge/sw"
)

// Bus — BroadcastChannel connecting shell, marmot, and relay SWs.
// All client-local, no server involvement.

var bus bc.BC

func connectBus() {
	bus = bc.Open("smesh-bus", func(msg string) { onBusMessage(msg) })
	sw.Log("shell-sw: bus connected (BroadcastChannel)")
	if myPubkey == "" {
		broadcastToClients("[\"NEED_IDENTITY\"]")
	}
}

func busSend(to, msg string) {
	bc.Send(bus, "{\"from\":\"shell\",\"to\":"+jstr(to)+",\"msg\":"+msg+"}")
}

func onBusMessage(raw string) {
	from := jsonField(raw, "from")
	if from == "shell" {
		return
	}
	to := jsonField(raw, "to")
	if to != "shell" && to != "*" {
		return
	}
	msg := jsonFieldRaw(raw, "msg")
	if msg == "" {
		return
	}
	w := newMW(msg)
	msgType := w.str()

	switch msgType {
	case "FWD":
		clientID := w.str()
		innerMsg := w.raw()
		if clientID == "" {
			broadcastToClients(innerMsg)
		} else {
			sendToClient(clientID, innerMsg)
		}
	case "FWD_ALL":
		innerMsg := w.raw()
		broadcastToClients(innerMsg)
	case "DM_RECEIVED":
		dmJSON := w.raw()
		busSend("relay", "[\"SAVE_DM\","+dmJSON+"]")
	case "MLS_GROUPS":
		raw := w.raw()
		broadcastToClients("[\"MLS_GROUPS\"," + raw + "]")
	case "DM_SENT":
		tsStr := w.raw()
		broadcastToClients("[\"DM_SENT\"," + tsStr + "]")
	case "MLS_STATUS":
		text := w.raw()
		broadcastToClients("[\"MLS_STATUS\"," + text + "]")
	case "CRYPTO_REQ":
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

// jsonField extracts a string value (unquoted) for a key from a JSON object string.
func jsonField(json, key string) string {
	v := jsonFieldRaw(json, key)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// jsonFieldRaw extracts a raw JSON value for a key from a JSON object string.
func jsonFieldRaw(json, key string) string {
	needle := "\"" + key + "\":"
	idx := -1
	for i := 0; i <= len(json)-len(needle); i++ {
		if json[i:i+len(needle)] == needle {
			idx = i + len(needle)
			break
		}
	}
	if idx < 0 {
		return ""
	}
	for idx < len(json) && (json[idx] == ' ' || json[idx] == '\t') {
		idx++
	}
	if idx >= len(json) {
		return ""
	}
	end := skipval(json, idx)
	if end < 0 {
		return ""
	}
	return json[idx:end]
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
