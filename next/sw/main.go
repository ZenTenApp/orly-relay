package main

import (
	"common/jsbridge/sw"
)

// App Shell domain — Service Worker lifecycle, static asset caching, SSE version monitoring.
// Thin outer shell: delegates all app-level messages to the Subscription Router.

const cacheName = "sm3sh"

var appFiles = []string{
	// Main app — absolute paths so cache.addAll resolves correctly from SW location.
	// Note: /index.html omitted — Go FileServer 301-redirects it to /, which
	// causes cache.addAll to fail (Cache API rejects redirect responses).
	"/",
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
	// Service worker (shell SW).
	"/$sw/$entry.mjs",
	"/$sw/sw.mjs",
	"/$sw/common_jsbridge_sw.mjs",
	"/$sw/common_jsbridge_bc.mjs",
	"/$sw/common_jsbridge_subtle.mjs",
	"/$sw/common_crypto_secp256k1.mjs",
	"/$sw/common_crypto_sha256.mjs",
	"/$sw/common_helpers.mjs",
	"/$sw/$runtime/index.mjs",
	"/$sw/$runtime/runtime.mjs",
	"/$sw/$runtime/goroutine.mjs",
	"/$sw/$runtime/channel.mjs",
	"/$sw/$runtime/builtin.mjs",
	"/$sw/$runtime/types.mjs",
	"/$sw/$runtime/sync.mjs",
	"/$sw/$runtime/sw.mjs",
	"/$sw/$runtime/subtle.mjs",
	"/$sw/$runtime/crypto.mjs",
	"/$sw/$runtime/ws.mjs",
	"/$sw/$runtime/bc.mjs",
}

var currentVersion string

func main() {
	initSharedState()
	sw.OnInstall(onInstall)
	sw.OnActivate(onActivate)
	sw.OnFetch(onFetch)
	sw.OnMessage(onMessage)
	// Connect bus+SSE from main() so they survive SW thread eviction.
	// onActivate only fires once per lifecycle; the browser can evict
	// and restart the thread at any time, losing all in-memory state.
	connectSSE()
	connectBus()
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
			done()
		})
	})
}

func onFetch(event sw.Event) {
	url := sw.GetRequestURL(event)
	origin := sw.Origin()
	// Only intercept same-origin requests.
	if len(url) < len(origin) || url[:len(origin)] != origin {
		return
	}
	path := sw.GetRequestPath(event)
	if path == "/__sse" || path == "/__version" {
		return
	}
	// SW module files, satellite SW dirs, and fonts: pass through to network.
	if (len(path) > 4 && path[:4] == "/$sw") || (len(path) > 6 && path[:7] == "/fonts/") {
		return
	}
	sw.RespondWithCacheFirst(event)
}

func onMessage(event sw.Event) {
	data := sw.GetMessageData(event)
	clientID := sw.GetMessageClientID(event)

	// Simple string messages — App Shell handles directly.
	if data == "activate-update" {
		refreshAndReload()
		return
	}

	// JSON array messages — parse and route.
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
	refreshAndReload()
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
