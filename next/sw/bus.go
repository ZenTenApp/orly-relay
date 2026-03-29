package main

import (
	"common/helpers"
	"common/jsbridge/bc"
	"common/jsbridge/sw"
)

// Bus — BroadcastChannel connecting shell and relay SWs.
// All client-local, no server involvement.
//
// Satellite SWs may be terminated by the browser at any time.
// BroadcastChannel messages don't wake terminated SWs. We queue messages for
// each destination until it sends a READY announcement, then flush the queue.

var bus bc.BC

// Per-destination readiness tracking and message queue.
var (
	relayReady bool
	relayQueue []string
)

func connectBus() {
	bus = bc.Open("smesh-bus", func(msg string) { onBusMessage(msg) })
	// Ask satellite SWs to re-announce readiness (shell may have restarted).
	bc.Send(bus, "{\"from\":\"shell\",\"to\":\"*\",\"msg\":[\"PING\","+jstr(version)+"]}")
	if myPubkey == "" {
		broadcastToClients("[\"NEED_IDENTITY\"]")
	}
}

func busSend(to, msg string) {
	envelope := "{\"from\":\"shell\",\"to\":" + jstr(to) + ",\"msg\":" + msg + "}"
	if to == "relay" && !relayReady {
		// Fix 1f: cap queue to prevent unbounded growth if relay SW never starts.
		if len(relayQueue) >= 256 {
			relayQueue = relayQueue[1:]
		}
		relayQueue = append(relayQueue, envelope)
		return
	}
	bc.Send(bus, envelope)
}

func flushQueue(to string) {
	if to != "relay" {
		return
	}
	wasReady := relayReady
	relayReady = true
	for _, msg := range relayQueue {
		bc.Send(bus, msg)
	}
	relayQueue = nil
	if wasReady {
		broadcastToClients("[\"RESUB\"]")
	}
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

	// READY handshake — satellite SW just connected to bus.
	if msgType == "READY" {
		satVer := w.str() // may be empty for old satellites
		if satVer != "" && satVer != version {
			sw.Log("shell: version mismatch from " + from + ": " + satVer + " != " + version)
			broadcastToClients("[\"FORCE_UPDATE_SW\"," + jstr(from) + "]")
			return
		}
		flushQueue(from)
		return
	}

	// Any message from relay SW also confirms it's alive.
	if from == "relay" && !relayReady {
		flushQueue("relay")
	}

	if msgType != "LOG" && msgType != "FWD" && msgType != "FWD_ALL" && msgType != "FWD_BATCH" {
		sw.Log("shell: bus " + from + "→" + msgType)
	}

	switch msgType {
	case "LOG":
		origin := w.str()
		logMsg := w.str()
		broadcastToClients("[\"SW_LOG\"," + jstr(origin) + "," + jstr(logMsg) + "]")
		return
	case "FWD":
		dispatchFwd(w.str(), w.raw())
	case "FWD_ALL":
		broadcastToClients(w.raw())
	case "FWD_BATCH":
		dispatchFwdBatch(msg)
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

func dispatchFwd(clientID, innerMsg string) {
	if clientID == "" {
		if hasMarmotSub(innerMsg) {
			deliverMarmotEvent(innerMsg)
		} else {
			broadcastToClients(innerMsg)
		}
	} else {
		sendToClient(clientID, innerMsg)
	}
}

// dispatchFwdBatch unpacks a FWD_BATCH message and dispatches each item.
// Format: ["FWD_BATCH", [ ["FWD",clientID,msg], ["FWD_ALL",msg], ... ] ]
func dispatchFwdBatch(raw string) {
	// Find the outer array (second element of the batch message).
	bw := newMW(raw)
	_ = bw.str() // "FWD_BATCH"
	// Now we're at the inner array. Walk it as raw values.
	bw.sep()
	if bw.i >= len(bw.s) || bw.s[bw.i] != '[' {
		return
	}
	bw.i++ // skip '['
	for {
		bw.sep()
		if bw.i >= len(bw.s) || bw.s[bw.i] == ']' {
			break
		}
		// Each item is a full message array like ["FWD","cid",...] or ["FWD_ALL",...]
		itemStart := bw.i
		bw.i = skipval(bw.s, bw.i)
		if bw.i < 0 {
			break
		}
		item := bw.s[itemStart:bw.i]
		iw := newMW(item)
		t := iw.str()
		switch t {
		case "FWD":
			dispatchFwd(iw.str(), iw.raw())
		case "FWD_ALL":
			broadcastToClients(iw.raw())
		}
	}
}

// hasMarmotSub checks if a FWD inner message is a marmot MLS subscription event.
func hasMarmotSub(msg string) bool {
	const prefix = "[\"EVENT\",\"marmot-sub-"
	return len(msg) > len(prefix) && msg[:len(prefix)] == prefix
}

// deliverMarmotEvent converts a relay EVENT for a marmot subscription
// into an MLS_PROXY deliverEvent message for the page.
func deliverMarmotEvent(msg string) {
	iw := newMW(msg)
	_ = iw.str()            // "EVENT"
	fullSubID := iw.str()   // "marmot-sub-<n>"
	eventJSON := iw.raw()   // the event object
	subID := fullSubID[11:] // strip "marmot-sub-" (11 chars)
	// Event must be a JSON string (not raw object) — Go WASM calls args[1].String().
	broadcastToClients("[\"MLS_PROXY\",\"deliverEvent\"," + subID + "," + jstr(eventJSON) + "]")
}

// jsonField extracts a string value (unquoted) for a key from a JSON object string.
func jsonField(json, key string) string {
	v := jsonFieldRaw(json, key)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return jsonUnescape(v[1 : len(v)-1])
	}
	return v
}

// jsonUnescape handles JSON string escape sequences: \n \t \\ \" \/ \r
func jsonUnescape(s string) string {
	if len(s) == 0 {
		return s
	}
	// Fast path: no escapes.
	hasEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			hasEscape = true
			break
		}
	}
	if !hasEscape {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			case '\\':
				out = append(out, '\\')
			case '"':
				out = append(out, '"')
			case '/':
				out = append(out, '/')
			default:
				out = append(out, s[i], s[i+1])
			}
			i++
		} else {
			out = append(out, s[i])
		}
	}
	return string(out)
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

// parseTS parses a numeric string (from JSON) to int64.
func parseTS(s string) int64 {
	var n int64
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int64(s[i]-'0')
		}
	}
	return n
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
