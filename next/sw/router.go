package main

import (
	"common/helpers"
	"common/jsbridge/sw"
)

// Subscription Router — thin forwarder to relay and marmot SWs via bus.
// All relay operations, subscriptions, and caching are handled by the relay SW.

// pendingSentDMs holds MLS_SEND saves deferred because myPubkey wasn't set yet.
// Replayed when SET_KEY or SET_PUBKEY arrives.
type pendingSentDM struct {
	recipient string
	content   string
}

var pendingSentDMs []pendingSentDM

func flushPendingSentDMs() {
	if myPubkey == "" || len(pendingSentDMs) == 0 {
		return
	}
	for _, p := range pendingSentDMs {
		now := sw.NowSeconds()
		rec := makeDMRecord(p.recipient, myPubkey, p.content, now, "marmot", "")
		busSend("relay", "[\"SAVE_DM_QUIET\","+rec.ToJSON()+"]")
	}
	pendingSentDMs = nil
}

func routeMessage(clientID string, w *mw, msgType string) {
	switch msgType {
	// Identity — handle locally + send to each SW (targeted, not broadcast,
	// so the bus queues them if a SW hasn't connected yet).
	case "SET_KEY":
		hexKey := w.str()
		identitySetKey(hexKey)
		msg := "[\"SET_KEY\"," + jstr(hexKey) + "]"
		busSend("marmot", msg)
		busSend("relay", msg)
		sendToClient(clientID, "[\"KEY_SET\"]")
		flushPendingSentDMs()
	case "SET_PUBKEY":
		pk := w.str()
		identitySetPubkey(pk)
		msg := "[\"SET_PUBKEY\"," + jstr(pk) + "]"
		busSend("marmot", msg)
		busSend("relay", msg)
		flushPendingSentDMs()
	case "CLEAR_KEY":
		identityClearKey()
		busSend("marmot", "[\"CLEAR_KEY\"]")
		busSend("relay", "[\"CLEAR_KEY\"]")

	// Relay operations — forward to relay SW.
	case "REQ":
		subID := w.str()
		filterRaw := w.raw()
		busSend("relay", "[\"REQ\","+jstr(clientID)+","+jstr(subID)+","+filterRaw+"]")
	case "CLOSE":
		subID := w.str()
		busSend("relay", "[\"CLOSE\","+jstr(subID)+"]")
	case "EVENT":
		eventRaw := w.raw()
		busSend("relay", "[\"EVENT\","+jstr(clientID)+","+eventRaw+"]")
	case "PROXY":
		subID := w.str()
		filterRaw := w.raw()
		relayURLs := w.strs()
		busSend("relay", "[\"PROXY\","+jstr(clientID)+","+jstr(subID)+","+filterRaw+","+strsJSON(relayURLs)+"]")
	case "RELAY_INFO":
		relayURL := w.str()
		busSend("relay", "[\"RELAY_INFO\","+jstr(clientID)+","+jstr(relayURL)+"]")
	case "SET_WRITE_RELAYS":
		busSend("relay", "[\"SET_WRITE_RELAYS\","+strsJSON(w.strs())+"]")
	case "SIGN":
		requestID := w.str()
		eventRaw := w.raw()
		busSend("relay", "[\"SIGN\","+jstr(clientID)+","+jstr(requestID)+","+eventRaw+"]")
	case "BROADCAST":
		pubkey := w.str()
		relayURLs := w.strs()
		busSend("relay", "[\"BROADCAST\","+jstr(clientID)+","+jstr(pubkey)+","+strsJSON(relayURLs)+"]")

	// DM operations — forward to relay SW for IDB.
	case "DM_LIST":
		busSend("relay", "[\"DM_LIST\","+jstr(clientID)+"]")
	case "DM_HISTORY":
		peer := w.str()
		limit := int(w.num())
		until := w.num()
		busSend("relay", "[\"DM_HISTORY\","+jstr(clientID)+","+jstr(peer)+","+helpers.Itoa(int64(limit))+","+helpers.Itoa(until)+"]")

	// MLS — forward to marmot SW.
	case "MLS_INIT":
		busSend("marmot", "[\"MLS_INIT\","+strsJSON(w.strs())+"]")
	case "MLS_SEND":
		recipient := w.str()
		content := w.str()
		busSend("marmot", "[\"MLS_SEND\","+jstr(recipient)+","+jstr(content)+"]")
		// Save sent DM to relay's IDB (quiet — no DM_RECEIVED broadcast).
		if myPubkey == "" {
			pendingSentDMs = append(pendingSentDMs, pendingSentDM{recipient, content})
		} else {
			now := sw.NowSeconds()
			rec := makeDMRecord(recipient, myPubkey, content, now, "marmot", "")
			busSend("relay", "[\"SAVE_DM_QUIET\","+rec.ToJSON()+"]")
		}
	case "MLS_SUB":
		busSend("marmot", "[\"MLS_SUB\"]")
	case "MLS_PUBLISH_KP":
		busSend("marmot", "[\"MLS_PUBLISH_KP\","+strsJSON(w.strs())+"]")
	case "MLS_LIST_GROUPS":
		busSend("marmot", "[\"MLS_LIST_GROUPS\"]")

	// Crypto result from page — dispatch to waiting callback.
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
