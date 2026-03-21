package main

import (
	"common/crypto/secp256k1"
	"common/helpers"
	"common/jsbridge/idb"
	"common/jsbridge/sw"
	"common/nostr"
)

const cacheName = "sm3sh"

var appFiles = []string{
	// Main app.
	"./",
	"./index.html",
	"./$entry.mjs",
	"./smesh3.mjs",
	"./common_crypto_secp256k1.mjs",
	"./common_crypto_sha256.mjs",
	"./common_helpers.mjs",
	"./common_jsbridge_dom.mjs",
	"./common_jsbridge_localstorage.mjs",
	"./common_jsbridge_signer.mjs",
	"./common_nostr.mjs",
	"./$runtime/index.mjs",
	"./$runtime/runtime.mjs",
	"./$runtime/goroutine.mjs",
	"./$runtime/channel.mjs",
	"./$runtime/builtin.mjs",
	"./$runtime/types.mjs",
	"./$runtime/sync.mjs",
	"./$runtime/dom.mjs",
	"./$runtime/localstorage.mjs",
	"./$runtime/signer.mjs",
	"./$wasm/secp256k1.mjs",
	"./$wasm/secp256k1.wasm",
	"./smesh-loader.svg",
	// Service worker.
	"./$sw/$entry.mjs",
	"./$sw/sw.mjs",
	"./$sw/common_jsbridge_sw.mjs",
	"./$sw/common_jsbridge_idb.mjs",
	"./$sw/common_jsbridge_ws.mjs",
	"./$sw/common_jsbridge_subtle.mjs",
	"./$sw/common_crypto_secp256k1.mjs",
	"./$sw/common_crypto_sha256.mjs",
	"./$sw/common_crypto_chacha20.mjs",
	"./$sw/common_crypto_hmac.mjs",
	"./$sw/common_crypto_hkdf.mjs",
	"./$sw/common_crypto_nip44.mjs",
	"./$sw/common_crypto_nip04.mjs",
	"./$sw/common_helpers.mjs",
	"./$sw/common_nostr.mjs",
	"./$sw/common_relay.mjs",
	"./$sw/$runtime/index.mjs",
	"./$sw/$runtime/runtime.mjs",
	"./$sw/$runtime/goroutine.mjs",
	"./$sw/$runtime/channel.mjs",
	"./$sw/$runtime/builtin.mjs",
	"./$sw/$runtime/types.mjs",
	"./$sw/$runtime/sync.mjs",
	"./$sw/$runtime/sw.mjs",
	"./$sw/$runtime/idb.mjs",
	"./$sw/$runtime/subtle.mjs",
	"./$sw/$runtime/crypto.mjs",
	"./$sw/$runtime/dom.mjs",
	"./$sw/$runtime/localstorage.mjs",
	"./$sw/$runtime/ws.mjs",
}

var currentVersion string

func main() {
	initState()
	idb.Open(func() {
		sw.Log("IDB ready")
	})
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
}

func onInstall(event sw.Event) {
	sw.WaitUntil(event, func(done func()) {
		sw.Fetch("/__version", func(resp sw.Response, ok bool) {
			if ok {
				// TODO: read response body
			}
			sw.CacheOpen(cacheName, func(cache sw.Cache) {
				sw.CacheAddAll(cache, appFiles, func() {
					sw.SkipWaiting()
					done()
				})
			})
		})
	})
}

func onActivate(event sw.Event) {
	sw.WaitUntil(event, func(done func()) {
		sw.ClaimClients(func() {
			connectSSE()
			done()
		})
	})
}

func onFetch(event sw.Event) {
	path := sw.GetRequestPath(event)
	if path == "/__sse" || path == "/__version" {
		return
	}
	sw.RespondWithCacheFirst(event)
}

func onMessage(event sw.Event) {
	data := sw.GetMessageData(event)
	clientID := sw.GetMessageClientID(event)

	// Simple string messages.
	if data == "activate-update" {
		refreshAndReload()
		return
	}

	// JSON array messages.
	w := newMW(data)
	msgType := w.str()

	switch msgType {
	case "REQ":
		subID := w.str()
		filterRaw := w.raw()
		handleReq(clientID, subID, filterRaw)

	case "CLOSE":
		subID := w.str()
		handleClose(subID)

	case "EVENT":
		eventRaw := w.raw()
		handlePublish(clientID, eventRaw)

	case "PROXY":
		subID := w.str()
		filterRaw := w.raw()
		relayURLs := w.strs()
		handleProxy(clientID, subID, filterRaw, relayURLs)

	case "RELAY_INFO":
		relayURL := w.str()
		handleRelayInfo(clientID, relayURL)

	case "SKIP_WAITING":
		sw.SkipWaiting()

	case "SET_KEY":
		hexKey := w.str()
		seckey = hexTo32(hexKey)
		hasKey = true
		pk, ok := secp256k1.PubKeyFromSecKey(seckey)
		if ok {
			myPubkey = helpers.HexEncode(pk[:])
		}
		sendToClient(clientID, "[\"KEY_SET\"]")

	case "SET_PUBKEY":
		myPubkey = w.str()

	case "CLEAR_KEY":
		seckey = [32]byte{}
		hasKey = false
		myPubkey = ""
		writeRelays = nil

	case "SET_WRITE_RELAYS":
		writeRelays = w.strs()

	case "SIGN":
		requestID := w.str()
		eventRaw := w.raw()
		if !hasKey {
			sendToClient(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"no key\"]")
			break
		}
		ev := nostr.ParseEvent(eventRaw)
		if ev == nil {
			sendToClient(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"parse error\"]")
			break
		}
		aux := random32()
		if ev.Sign(seckey, aux) {
			sendToClient(clientID, "[\"SIGNED\","+jstr(requestID)+","+ev.ToJSON()+"]")
		} else {
			sendToClient(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"sign failed\"]")
		}

	case "BROADCAST":
		pubkey := w.str()
		relayURLs := w.strs()
		broadcastIdentity(clientID, pubkey, relayURLs)

	case "SEND_DM":
		recipientPubkey := w.str()
		content := w.str()
		relayURLs := w.strs()
		sendDM(clientID, recipientPubkey, content, relayURLs)

	case "DM_SUB":
		relayURLs := w.strs()
		handleDMSub(clientID, relayURLs)

	case "DM_LIST":
		handleDMList(clientID)

	case "DM_HISTORY":
		peer := w.str()
		limit := int(w.num())
		until := w.num()
		handleDMHistory(clientID, peer, limit, until)

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

func connectSSE() {
	sw.SSEConnect("/__sse", func(data string) {
		v := data
		if currentVersion == "" {
			currentVersion = v
			return
		}
		if v != currentVersion {
			currentVersion = v
			notifyUpdate()
		}
	})
}

func notifyUpdate() {
	sw.MatchClients(func(client sw.Client) {
		sw.PostMessage(client, "update-available")
	})
}

func refreshAndReload() {
	sw.CacheOpen(cacheName, func(cache sw.Cache) {
		refreshFiles(cache, 0, func() {
			sw.MatchClients(func(client sw.Client) {
				sw.Navigate(client, "")
			})
		})
	})
}

func refreshFiles(cache sw.Cache, idx int, done func()) {
	if idx >= len(appFiles) {
		done()
		return
	}
	file := appFiles[idx]
	bust := file + "?v=" + currentVersion
	sw.Fetch(bust, func(resp sw.Response, ok bool) {
		if ok && sw.ResponseOK(resp) {
			sw.CachePut(cache, file, resp, func() {
				refreshFiles(cache, idx+1, done)
			})
		} else {
			refreshFiles(cache, idx+1, done)
		}
	})
}
