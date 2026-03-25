package main

import (
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/crypto"
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
	// marmotReady stays false until auth succeeds — queues sends in marmotPending.
	sw.Log("marmot-sw: marmot connected, pubkey=" + myPubkey[:min(len(myPubkey), 16)] + " hasKey=" + boolStr(hasKey))
	if myPubkey != "" {
		marmotAuth()
	} else {
		sw.Log("marmot-sw: no pubkey, skipping auth")
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func marmotOnClose() {
	marmotReady = false
	sw.Log("marmot-sw: marmot ws closed, reconnecting in 2s")
	sw.SetTimeout(2000, func() { marmotReconnect() })
}

func marmotReconnect() {
	if marmotReady {
		return
	}
	wsURL := sw.Origin()
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += "/__marmot"
	sw.Log("marmot-sw: reconnecting to " + wsURL)
	// Don't clear marmotPending — queued messages will drain after auth.
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

func marmotAuth() {
	if myPubkey == "" {
		sw.Log("marmot-sw: auth: pubkey empty")
		return
	}
	if !hasKey {
		sw.Log("marmot-sw: auth: NIP-07 mode")
		// NIP-07 extension mode: proxy signing through shell SW via bus.
		now := sw.NowSeconds()
		evJSON := "{\"kind\":22242,\"content\":\"marmot-auth\",\"tags\":[]," +
			"\"created_at\":" + helpers.Itoa(now) + ",\"pubkey\":" + jstr(myPubkey) + "}"
		cryptoProxy("signEvent", "", evJSON, func(signedJSON, errMsg string) {
			if errMsg != "" || signedJSON == "" {
				sw.Log("marmot-sw: extension sign failed: " + errMsg)
				return
			}
			sw.Log("marmot-sw: auth: sending NIP-07 signed auth")
			ws.Send(marmotConn, "{\"method\":\"auth\",\"event\":"+signedJSON+"}")
		})
		return
	}
	// Direct Schnorr auth.
	pubBytes := helpers.HexDecode(myPubkey)
	if pubBytes == nil {
		sw.Log("marmot-sw: auth: pubkey decode failed")
		return
	}
	msgHash := sha256.Sum(pubBytes)
	aux := random32()
	sig := crypto.SignSchnorr(seckey[:], msgHash[:], aux[:])
	if sig == nil {
		sw.Log("marmot-sw: auth sign failed")
		return
	}
	authMsg := "{\"method\":\"auth\",\"pubkey\":" + jstr(myPubkey) + ",\"sig\":" + jstr(helpers.HexEncode(sig)) + "}"
	ws.Send(marmotConn, authMsg)
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
	sw.Log("marmot-sw: recv: " + msg[:min(len(msg), 120)])
	method := jsonField(msg, "method")
	switch method {
	case "auth":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot-sw: auth failed: " + errMsg)
		} else {
			sw.Log("marmot-sw: authenticated")
			marmotReady = true
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
		} else {
			tsStr := jsonField(msg, "ts")
			busSend("shell", "[\"DM_SENT\","+tsStr+"]")
		}

	case "subscribe":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			busSend("shell", "[\"MLS_STATUS\","+jstr("subscribe error: "+errMsg)+"]")
		} else {
			busSend("shell", "[\"MLS_STATUS\","+jstr("subscribed")+"]")
		}

	case "publish_kp":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot-sw: publish_kp error: " + errMsg)
		}

	case "list_groups":
		busSend("shell", "[\"MLS_GROUPS\","+msg+"]")

	case "status":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			busSend("shell", "[\"MLS_STATUS\","+jstr("error: "+errMsg)+"]")
		} else {
			info := "pubkey: " + jsonField(msg, "pubkey") +
				"\nrelay: " + jsonField(msg, "relay") +
				"\nsubscribed: " + jsonField(msg, "subscribed") +
				"\ngroups: " + jsonField(msg, "num_groups")
			busSend("shell", "[\"MLS_STATUS\","+jstr(info)+"]")
		}

	case "reset":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			busSend("shell", "[\"MLS_STATUS\","+jstr("reset error: "+errMsg)+"]")
		} else {
			busSend("shell", "[\"MLS_STATUS\","+jstr("reset OK")+"]")
		}

	case "resolve_alias":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			busSend("shell", "[\"MLS_STATUS\","+jstr("alias error: "+errMsg)+"]")
		} else {
			pk := jsonField(msg, "pubkey")
			busSend("shell", "[\"MLS_STATUS\","+jstr("alias → "+pk)+"]")
		}

	case "crypto_req":
		// Backend needs crypto op proxied through the browser extension.
		handleCryptoReq(msg)

	case "error":
		errMsg := jsonField(msg, "error")
		sw.Log("marmot-sw: server error: " + errMsg)
	}
}

var pendingCryptoIDs map[int]string

func initCryptoProxy() {
	pendingCryptoIDs = make(map[int]string)
}

func handleCryptoReq(msg string) {
	backendID := jsonField(msg, "id")
	op := jsonField(msg, "op")
	peer := jsonField(msg, "peer")
	data := jsonField(msg, "content")
	pageMethod := op
	if op == "nip44Encrypt" {
		pageMethod = "nip44.encrypt"
	} else if op == "nip44Decrypt" {
		pageMethod = "nip44.decrypt"
	}
	// Store mapping from local crypto ID → backend ID before issuing request.
	localID := nextCryptoID
	pendingCryptoIDs[localID] = backendID
	cryptoProxy(pageMethod, peer, data, func(result, errMsg string) {
		bid, ok := pendingCryptoIDs[localID]
		if ok {
			delete(pendingCryptoIDs, localID)
		} else {
			bid = backendID
		}
		ws.Send(marmotConn, "{\"method\":\"crypto_resp\",\"id\":"+bid+
			",\"result\":"+jstr(result)+",\"error\":"+jstr(errMsg)+"}")
	})
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
		var buf []byte
		hasEsc := false
		start := idx
		for idx < len(json) && json[idx] != '"' {
			if json[idx] == '\\' && idx+1 < len(json) {
				if !hasEsc {
					hasEsc = true
					buf = append(buf, json[start:idx]...)
				} else {
					buf = append(buf, json[start:idx]...)
				}
				idx++
				switch json[idx] {
				case '"', '\\', '/':
					buf = append(buf, json[idx])
				case 'n':
					buf = append(buf, '\n')
				case 't':
					buf = append(buf, '\t')
				case 'r':
					buf = append(buf, '\r')
				default:
					buf = append(buf, '\\', json[idx])
				}
				idx++
				start = idx
				continue
			}
			idx++
		}
		if hasEsc {
			buf = append(buf, json[start:idx]...)
			return string(buf)
		}
		return json[start:idx]
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
