package main

import "common/jsbridge/sw"

const cacheName = "sm3sh"

var appFiles = []string{
	// Main app.
	"./",
	"./index.html",
	"./$entry.mjs",
	"./smesh3.mjs",
	"./common_crypto_sha256.mjs",
	"./common_helpers.mjs",
	"./common_jsbridge_crypto.mjs",
	"./common_jsbridge_dom.mjs",
	"./common_jsbridge_localstorage.mjs",
	"./common_jsbridge_ws.mjs",
	"./common_nostr.mjs",
	"./common_relay.mjs",
	"./$runtime/index.mjs",
	"./$runtime/runtime.mjs",
	"./$runtime/goroutine.mjs",
	"./$runtime/channel.mjs",
	"./$runtime/builtin.mjs",
	"./$runtime/types.mjs",
	"./$runtime/sync.mjs",
	"./$runtime/dom.mjs",
	"./$runtime/ws.mjs",
	"./$runtime/localstorage.mjs",
	"./$runtime/crypto.mjs",
	"./$wasm/secp256k1.mjs",
	"./$wasm/secp256k1.wasm",
	"./smesh-loader.svg",
	// Service worker.
	"./$sw/$entry.mjs",
	"./$sw/sw.mjs",
	"./$sw/common_jsbridge_sw.mjs",
	"./$sw/$runtime/index.mjs",
	"./$sw/$runtime/runtime.mjs",
	"./$sw/$runtime/goroutine.mjs",
	"./$sw/$runtime/channel.mjs",
	"./$sw/$runtime/builtin.mjs",
	"./$sw/$runtime/types.mjs",
	"./$sw/$runtime/sync.mjs",
	"./$sw/$runtime/sw.mjs",
}

var currentVersion string

func main() {
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
}

func onInstall(event sw.Event) {
	sw.WaitUntil(event, func(done func()) {
		sw.Fetch("/__version", func(resp sw.Response, ok bool) {
			if ok {
				// TODO: read response body — for now skip version fetch
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
		return // let network handle SSE and version endpoints
	}
	sw.RespondWithCacheFirst(event)
}

func onMessage(event sw.Event) {
	data := sw.GetMessageData(event)
	if data == "activate-update" {
		refreshAndReload()
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

// refreshFiles caches each app file sequentially with a cache-bust param.
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
