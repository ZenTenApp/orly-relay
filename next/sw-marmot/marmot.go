package main

import (
	"common/crypto/secp256k1"
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/sw"
	"common/jsbridge/ws"
)

// Marmot WS proxy — connects to /__marmot on own origin,
// routes DM results back to shell SW via bus.

var (
	marmotConn    ws.Conn
	marmotReady   bool
	marmotPending []string
)

func marmotInit(relayURLs []string) {
	if marmotReady {
		return
	}
	wsURL := sw.Origin()
	sw.Log("marmot-sw: connecting to " + wsURL + "/__marmot")
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
			sw.Log("marmot-sw: marmot ws closed code=" + helpers.Itoa(int64(code)))
			marmotOnClose()
		},
		func(connID int) { marmotOnClose() },
	)
}

func marmotOnOpen() {
	marmotReady = true
	sw.Log("marmot-sw: marmot connected")
	if myPubkey != "" {
		marmotAuth()
	}
}

func marmotOnClose() {
	marmotReady = false
}

func marmotAuth() {
	if myPubkey == "" {
		return
	}
	if !hasKey {
		// NIP-07 extension mode: proxy signing through shell SW via bus.
		now := sw.NowSeconds()
		evJSON := "{\"kind\":22242,\"content\":\"marmot-auth\",\"tags\":[]," +
			"\"created_at\":" + helpers.Itoa(now) + ",\"pubkey\":" + jstr(myPubkey) + "}"
		cryptoProxy("signEvent", "", evJSON, func(signedJSON, errMsg string) {
			if errMsg != "" || signedJSON == "" {
				sw.Log("marmot-sw: extension sign failed: " + errMsg)
				return
			}
			marmotSendJSON("{\"method\":\"auth\",\"event\":" + signedJSON + "}")
		})
		return
	}
	// Direct Schnorr auth.
	pubBytes := helpers.HexDecode(myPubkey)
	if pubBytes == nil {
		return
	}
	msgHash := sha256.Sum(pubBytes)
	aux := random32()
	sig, ok := secp256k1.SignSchnorr(seckey, msgHash, aux)
	if !ok {
		sw.Log("marmot-sw: auth sign failed")
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

func marmotListGroups() {
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
	method := jsonField(msg, "method")
	switch method {
	case "auth":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot-sw: auth failed: " + errMsg)
		} else {
			sw.Log("marmot-sw: authenticated")
			for _, p := range marmotPending {
				ws.Send(marmotConn, p)
			}
			marmotPending = nil
			marmotSubscribe()
		}

	case "dm_received":
		peer := jsonField(msg, "peer")
		content := jsonField(msg, "content")
		tsStr := jsonField(msg, "ts")
		ts := parseInt(tsStr)
		if peer != "" && content != "" {
			rec := makeDMRecord(peer, peer, content, ts, "marmot", "")
			busSend("shell", "[\"DM_RECEIVED\","+rec.ToJSON()+"]")
		}

	case "send_dm":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot-sw: send_dm error: " + errMsg)
		}

	case "subscribe":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot-sw: subscribe error: " + errMsg)
		} else {
			sw.Log("marmot-sw: subscribed")
		}

	case "publish_kp":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot-sw: publish_kp error: " + errMsg)
		}

	case "list_groups":
		busSend("shell", "[\"MLS_GROUPS\","+msg+"]")

	case "error":
		errMsg := jsonField(msg, "error")
		sw.Log("marmot-sw: server error: " + errMsg)
	}
}

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
	for idx < len(json) && (json[idx] == ' ' || json[idx] == '\t') {
		idx++
	}
	if idx >= len(json) {
		return ""
	}
	if json[idx] == '"' {
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
