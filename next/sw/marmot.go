package main

import (
	"common/crypto/secp256k1"
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/sw"
	"common/jsbridge/ws"
)

// Marmot domain — MLS-based E2E encrypted DMs via backend WebSocket.
// The SW acts as a thin proxy: auth, send, subscribe commands flow to
// the /__marmot backend endpoint; incoming DMs arrive as JSON-RPC pushes
// and are routed through the existing DM record pipeline.

var (
	marmotConn    ws.Conn
	marmotReady   bool
	marmotPending []string // queued messages before WS opens
)

func marmotInit(relayURLs []string) {
	if marmotReady {
		return
	}

	// Build the WS URL from the SW's origin.
	wsURL := sw.Origin()
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += "/__marmot"

	marmotPending = nil
	marmotConn = ws.Dial(wsURL,
		func(connID int, msg string) { marmotOnMessage(msg) },
		func(connID int) { marmotOnOpen() },
		func(connID int, code int, reason string) {
			sw.Log("marmot: ws closed code=" + helpers.Itoa(int64(code)))
			marmotOnClose()
		},
		func(connID int) { marmotOnClose() },
	)
}

func marmotOnOpen() {
	marmotReady = true
	sw.Log("marmot: connected")

	// Authenticate with the current identity.
	if myPubkey != "" && hasKey {
		marmotAuth()
	}

	// Flush pending messages.
	for _, msg := range marmotPending {
		ws.Send(marmotConn, msg)
	}
	marmotPending = nil
}

func marmotOnClose() {
	marmotReady = false
	sw.Log("marmot: disconnected")
}

func marmotAuth() {
	if myPubkey == "" || !hasKey {
		return
	}
	// Sign the SHA256 of the pubkey bytes as the auth challenge.
	pubBytes := helpers.HexDecode(myPubkey)
	if pubBytes == nil {
		return
	}
	msgHash := sha256.Sum(pubBytes)
	aux := random32()
	sig, ok := secp256k1.SignSchnorr(seckey, msgHash, aux)
	if !ok {
		sw.Log("marmot: auth sign failed")
		return
	}
	marmotSendJSON("{\"method\":\"auth\",\"pubkey\":" + jstr(myPubkey) + ",\"sig\":" + jstr(helpers.HexEncode(sig[:])) + "}")
}

func marmotSend(recipient, content string) {
	marmotSendJSON("{\"method\":\"send_dm\",\"recipient\":" + jstr(recipient) + ",\"content\":" + jstr(content) + "}")
}

func marmotSubscribe() {
	marmotSendJSON("{\"method\":\"subscribe\"}")
}

func marmotPublishKP(relays []string) {
	msg := "{\"method\":\"publish_kp\""
	if len(relays) > 0 {
		msg += ",\"relays\":["
		for i, r := range relays {
			if i > 0 {
				msg += ","
			}
			msg += jstr(r)
		}
		msg += "]"
	}
	msg += "}"
	marmotSendJSON(msg)
}

func marmotListGroups(clientID string) {
	marmotSendJSON("{\"method\":\"list_groups\"}")
}

func marmotSendJSON(msg string) {
	if !marmotReady {
		marmotPending = append(marmotPending, msg)
		return
	}
	ws.Send(marmotConn, msg)
}

func marmotOnMessage(msg string) {
	// Minimal JSON parsing — extract "method" field.
	method := jsonField(msg, "method")
	switch method {
	case "auth":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot: auth failed: " + errMsg)
		} else {
			sw.Log("marmot: authenticated")
			// Auto-subscribe after auth.
			marmotSubscribe()
		}

	case "dm_received":
		peer := jsonField(msg, "peer")
		content := jsonField(msg, "content")
		tsStr := jsonField(msg, "ts")
		ts := parseInt(tsStr)
		if peer != "" && content != "" {
			rec := makeDMRecord(peer, peer, content, ts, "marmot", "")
			routerSaveDMRecord(rec)
		}

	case "send_dm":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot: send_dm error: " + errMsg)
		}

	case "subscribe":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot: subscribe error: " + errMsg)
		} else {
			sw.Log("marmot: subscribed")
		}

	case "publish_kp":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot: publish_kp error: " + errMsg)
		}

	case "list_groups":
		// Forward to all clients.
		broadcastToClients("[\"MLS_GROUPS\"," + msg + "]")

	case "error":
		errMsg := jsonField(msg, "error")
		sw.Log("marmot: server error: " + errMsg)
	}
}

// jsonField extracts a string value for a given key from a flat JSON object.
// Minimal parser — handles unescaped string values only.
func jsonField(json, key string) string {
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
	// Skip whitespace.
	for idx < len(json) && (json[idx] == ' ' || json[idx] == '\t') {
		idx++
	}
	if idx >= len(json) {
		return ""
	}
	if json[idx] == '"' {
		// String value.
		idx++
		end := idx
		for end < len(json) && json[end] != '"' {
			if json[end] == '\\' {
				end++
			}
			end++
		}
		return json[idx:end]
	}
	// Non-string value (number, bool, null).
	end := idx
	for end < len(json) && json[end] != ',' && json[end] != '}' && json[end] != ' ' {
		end++
	}
	return json[idx:end]
}

func parseInt(s string) int64 {
	var n int64
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			n = n*10 + int64(s[i]-'0')
		}
	}
	return n
}
