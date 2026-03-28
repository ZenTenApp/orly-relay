package main

import (
	"common/jsbridge/idb"
	"common/jsbridge/registry"
	"common/jsbridge/sw"
)

// App Shell domain — Service Worker lifecycle, static asset caching, SSE version monitoring.

const cacheName = "smesh"

var appFiles = []string{
	// Main app.
	"/",
	"/index.html",
	"/$entry.mjs",
	"/smesh3.mjs",
	"/common_crypto_secp256k1.mjs",
	"/common_crypto_sha256.mjs",
	"/common_helpers.mjs",
	"/common_jsbridge_dom.mjs",
	"/common_jsbridge_localstorage.mjs",
	"/common_jsbridge_signer.mjs",
	"/common_nostr.mjs",
	"/$runtime/index.mjs",
	"/$runtime/runtime.mjs",
	"/$runtime/goroutine.mjs",
	"/$runtime/channel.mjs",
	"/$runtime/builtin.mjs",
	"/$runtime/types.mjs",
	"/$runtime/sync.mjs",
	"/$runtime/dom.mjs",
	"/$runtime/localstorage.mjs",
	"/$runtime/signer.mjs",
	"/$wasm/secp256k1.mjs",
	"/$wasm/secp256k1.wasm",
	"/smesh-loader.svg",
	// Service worker — core.
	"/$sw/$entry.mjs",
	"/$sw/sw.mjs",
	"/$sw/common_jsbridge_sw.mjs",
	"/$sw/common_jsbridge_idb.mjs",
	"/$sw/common_jsbridge_registry.mjs",
	"/$sw/common_crypto_secp256k1.mjs",
	"/$sw/common_crypto_sha256.mjs",
	"/$sw/common_helpers.mjs",
	"/$sw/common_nostr.mjs",
	"/$sw/common_relay.mjs",
	"/$sw/$runtime/index.mjs",
	"/$sw/$runtime/runtime.mjs",
	"/$sw/$runtime/goroutine.mjs",
	"/$sw/$runtime/channel.mjs",
	"/$sw/$runtime/builtin.mjs",
	"/$sw/$runtime/types.mjs",
	"/$sw/$runtime/sync.mjs",
	"/$sw/$runtime/sw.mjs",
	"/$sw/$runtime/idb.mjs",
	"/$sw/$runtime/subtle.mjs",
	"/$sw/$runtime/crypto.mjs",
	"/$sw/$runtime/registry.mjs",
	// Service worker — deferred extensions (cached but loaded on demand).
	"/$sw/ext/swcrypto/$entry.mjs",
	"/$sw/ext/swcrypto/swcrypto.mjs",
	"/$sw/ext/swcrypto/common_crypto_secp256k1.mjs",
	"/$sw/ext/swcrypto/common_crypto_sha256.mjs",
	"/$sw/ext/swcrypto/common_crypto_chacha20.mjs",
	"/$sw/ext/swcrypto/common_crypto_hmac.mjs",
	"/$sw/ext/swcrypto/common_crypto_hkdf.mjs",
	"/$sw/ext/swcrypto/common_crypto_nip44.mjs",
	"/$sw/ext/swcrypto/common_crypto_nip04.mjs",
	"/$sw/ext/swcrypto/common_helpers.mjs",
	"/$sw/ext/swcrypto/common_nostr.mjs",
	"/$sw/ext/swcrypto/common_jsbridge_subtle.mjs",
	"/$sw/ext/swcrypto/common_jsbridge_sw.mjs",
	"/$sw/ext/swcrypto/common_jsbridge_registry.mjs",
	"/$sw/ext/swcrypto/$runtime/index.mjs",
	"/$sw/ext/swcrypto/$runtime/runtime.mjs",
	"/$sw/ext/swcrypto/$runtime/goroutine.mjs",
	"/$sw/ext/swcrypto/$runtime/channel.mjs",
	"/$sw/ext/swcrypto/$runtime/builtin.mjs",
	"/$sw/ext/swcrypto/$runtime/types.mjs",
	"/$sw/ext/swcrypto/$runtime/sync.mjs",
	"/$sw/ext/swcrypto/$runtime/sw.mjs",
	"/$sw/ext/swcrypto/$runtime/subtle.mjs",
	"/$sw/ext/swcrypto/$runtime/crypto.mjs",
	"/$sw/ext/swcrypto/$runtime/registry.mjs",
	"/$sw/ext/swmarmot/$entry.mjs",
	"/$sw/ext/swmarmot/swmarmot.mjs",
	"/$sw/ext/swmarmot/common_crypto_secp256k1.mjs",
	"/$sw/ext/swmarmot/common_crypto_sha256.mjs",
	"/$sw/ext/swmarmot/common_helpers.mjs",
	"/$sw/ext/swmarmot/common_jsbridge_subtle.mjs",
	"/$sw/ext/swmarmot/common_jsbridge_sw.mjs",
	"/$sw/ext/swmarmot/common_jsbridge_ws.mjs",
	"/$sw/ext/swmarmot/common_jsbridge_registry.mjs",
	"/$sw/ext/swmarmot/$runtime/index.mjs",
	"/$sw/ext/swmarmot/$runtime/runtime.mjs",
	"/$sw/ext/swmarmot/$runtime/goroutine.mjs",
	"/$sw/ext/swmarmot/$runtime/channel.mjs",
	"/$sw/ext/swmarmot/$runtime/builtin.mjs",
	"/$sw/ext/swmarmot/$runtime/types.mjs",
	"/$sw/ext/swmarmot/$runtime/sync.mjs",
	"/$sw/ext/swmarmot/$runtime/sw.mjs",
	"/$sw/ext/swmarmot/$runtime/subtle.mjs",
	"/$sw/ext/swmarmot/$runtime/ws.mjs",
	"/$sw/ext/swmarmot/$runtime/crypto.mjs",
	"/$sw/ext/swmarmot/$runtime/registry.mjs",
}

var currentVersion string

func main() {
	registerCoreHooks()
	initRouter()
	initRelayProxy()
	initSharedState()
	idb.Open(func() {})
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
}

func registerCoreHooks() {
	registry.OnSaveDMRecord(routerSaveDMRecordJSON)
	registry.OnBroadcastToClients(broadcastToClients)
	registry.OnSendToClient(sendToClient)
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
	if len(path) > 4 && path[:5] == "/$sw/" {
		return
	}
	sw.RespondWithCacheFirst(event)
}

func onMessage(event sw.Event) {
	data := sw.GetMessageData(event)
	clientID := sw.GetMessageClientID(event)

	if data == "activate-update" {
		refreshAndReload()
		return
	}

	w := newMW(data)
	msgType := w.str()

	switch msgType {
	case "SKIP_WAITING":
		sw.SkipWaiting()
	default:
		routeMessage(clientID, &w, msgType)
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
