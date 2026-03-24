package main

import (
	"common/crypto/secp256k1"
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/registry"
	"common/jsbridge/subtle"
	"common/jsbridge/sw"
	"common/jsbridge/ws"
)

func hexTo32(s string) [32]byte {
	out, _ := helpers.HexDecode32(s)
	return out
}

func random32() [32]byte {
	var b [32]byte
	subtle.RandomBytes(b[:])
	return b
}

var (
	conn    ws.Conn
	ready   bool
	pending []string
)

func init() {
	registry.OnMarmotInit(func(_ string) {
		marmotInit()
	})
	registry.OnMarmotSend(func(recipient, content string) {
		marmotSend(recipient, content)
	})
	registry.OnMarmotSubscribe(func() {
		marmotSubscribe()
	})
	registry.OnMarmotPublishKP(func(relaysJSON string) {
		marmotPublishKP(relaysJSON)
	})
	registry.OnMarmotListGroups(func(_ string) {
		marmotListGroups()
	})
}

func main() {}

func marmotInit() {
	if ready {
		return
	}
	wsURL := sw.Origin()
	if len(wsURL) > 5 && wsURL[:5] == "https" {
		wsURL = "wss" + wsURL[5:]
	} else if len(wsURL) > 4 && wsURL[:4] == "http" {
		wsURL = "ws" + wsURL[4:]
	}
	wsURL += "/__marmot"

	pending = nil
	conn = ws.Dial(wsURL,
		func(connID int, msg string) { onMessage(msg) },
		func(connID int) { onOpen() },
		func(connID int, code int, reason string) {
			sw.Log("marmot: ws closed code=" + helpers.Itoa(int64(code)))
			onClose()
		},
		func(connID int) { onClose() },
	)
}

func onOpen() {
	ready = true
	sw.Log("marmot: connected")
	if registry.Pubkey() != "" && registry.HasKey() {
		auth()
	}
	for _, msg := range pending {
		ws.Send(conn, msg)
	}
	pending = nil
}

func onClose() {
	ready = false
	sw.Log("marmot: disconnected")
}

func auth() {
	pub := registry.Pubkey()
	if pub == "" || !registry.HasKey() {
		return
	}
	pubBytes := helpers.HexDecode(pub)
	if pubBytes == nil {
		return
	}
	msgHash := sha256.Sum(pubBytes)
	aux := random32()
	sig, ok := secp256k1.SignSchnorr(hexTo32(registry.Seckey()), msgHash, aux)
	if !ok {
		sw.Log("marmot: auth sign failed")
		return
	}
	sendJSON("{\"method\":\"auth\",\"pubkey\":" + helpers.JsonString(pub) + ",\"sig\":" + helpers.JsonString(helpers.HexEncode(sig[:])) + "}")
}

func marmotSend(recipient, content string) {
	sendJSON("{\"method\":\"send_dm\",\"recipient\":" + helpers.JsonString(recipient) + ",\"content\":" + helpers.JsonString(content) + "}")
}

func marmotSubscribe() {
	sendJSON("{\"method\":\"subscribe\"}")
}

func marmotPublishKP(relaysJSON string) {
	msg := "{\"method\":\"publish_kp\""
	if relaysJSON != "[]" && relaysJSON != "" {
		msg += ",\"relays\":" + relaysJSON
	}
	msg += "}"
	sendJSON(msg)
}

func marmotListGroups() {
	sendJSON("{\"method\":\"list_groups\"}")
}

func sendJSON(msg string) {
	if !ready {
		pending = append(pending, msg)
		return
	}
	ws.Send(conn, msg)
}

func onMessage(msg string) {
	method := jsonField(msg, "method")
	switch method {
	case "auth":
		errMsg := jsonField(msg, "error")
		if errMsg != "" {
			sw.Log("marmot: auth failed: " + errMsg)
		} else {
			sw.Log("marmot: authenticated")
			marmotSubscribe()
		}

	case "dm_received":
		peer := jsonField(msg, "peer")
		content := jsonField(msg, "content")
		tsStr := jsonField(msg, "ts")
		ts := parseInt(tsStr)
		if peer != "" && content != "" {
			recJSON := registry.MakeDMRecord(peer, peer, content, ts, "marmot", "")
			if recJSON != "" {
				registry.SaveDMRecord(recJSON)
			}
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
		registry.BroadcastToClients("[\"MLS_GROUPS\"," + msg + "]")

	case "error":
		errMsg := jsonField(msg, "error")
		sw.Log("marmot: server error: " + errMsg)
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
