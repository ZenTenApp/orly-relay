package main

import (
	"common/helpers"
	"common/jsbridge/sw"
	"common/jsbridge/wasm"
)

// Marmot WASM bridge — loads marmot.wasm and bridges relay/crypto
// operations through BroadcastChannel to the relay and shell SWs.

var wasmReady bool

func marmotInit(relayURLs []string) {
	if wasmReady {
		// Already loaded, just re-init with new relays
		args := myPubkey
		for _, u := range relayURLs {
			args += "\x00" + u
		}
		sw.CallGlobal("_marmot_init", myPubkey)
		return
	}
	sw.Log("marmot-sw: loading WASM module")
	wasm.LoadGoWASM("/$sw-marmot/wasm_exec.mjs", "/$sw-marmot/marmot.wasm", func() {
		sw.Log("marmot-sw: WASM loaded")
		wasmReady = true
		wasmInitClient(relayURLs)
	})
}

func wasmInitClient(relayURLs []string) {
	if myPubkey == "" {
		sw.Log("marmot-sw: no pubkey, deferring WASM init")
		return
	}
	// _marmot_init(pubkey, relay1, relay2, ...) is handled by wasm_bridge.mjs
	// which creates the JS callback functions and calls _marmot.init()
	args := []string{myPubkey}
	args = append(args, relayURLs...)
	sw.CallGlobal("_marmot_init", args...)
	sw.Log("marmot-sw: WASM client initialized")
}

func marmotSend(recipient, content string) {
	sw.CallGlobal("_marmot_call", "sendDM", "["+jstr(recipient)+","+jstr(content)+"]")
}

func marmotSubscribe() {
	sw.CallGlobal("_marmot_call", "subscribe", "[]")
}

func marmotPublishKP(relays []string) {
	sw.CallGlobal("_marmot_call", "publishKP", "[]")
}

func marmotListGroups() {
	result := sw.CallGlobalResult("_marmot_call", "listGroups", "[]")
	busSend("shell", "[\"MLS_GROUPS\","+result+"]")
}

// handleCryptoResult routes a crypto response from the shell SW back to the WASM module.
func handleCryptoResult(id int, result, errMsg string) {
	sw.CallGlobal("_marmot_crypto_result", helpers.Itoa(int64(id)), result, errMsg)
}

func initCryptoProxy() {
	// No more pending crypto IDs tracking — the WASM module handles its own.
	// Crypto results come in via bus and get forwarded to _marmot_crypto_result.
}

// jsonField and parseInt kept for bus envelope parsing.

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
		start := idx
		for idx < len(json) && json[idx] != '"' {
			if json[idx] == '\\' && idx+1 < len(json) {
				idx++
			}
			idx++
		}
		return json[start:idx]
	}
	end := idx
	for end < len(json) && json[end] != ',' && json[end] != '}' && json[end] != ' ' {
		end++
	}
	return json[idx:end]
}
