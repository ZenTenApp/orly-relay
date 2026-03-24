package main

import (
	"common/helpers"
	"common/jsbridge/sw"
)

// Subscription Router — thin forwarder to relay and marmot SWs via bus.
// All relay operations, subscriptions, and caching are handled by the relay SW.

func routeMessage(clientID string, w *mw, msgType string) {
	switch msgType {
	// Identity — handle locally + broadcast to all SWs.
	case "SET_KEY":
		hexKey := w.str()
		identitySetKey(hexKey)
		busSend("*", "[\"SET_KEY\","+jstr(hexKey)+"]")
		sendToClient(clientID, "[\"KEY_SET\"]")
	case "SET_PUBKEY":
		pk := w.str()
		identitySetPubkey(pk)
		busSend("*", "[\"SET_PUBKEY\","+jstr(pk)+"]")
	case "CLEAR_KEY":
		identityClearKey()
		busSend("*", "[\"CLEAR_KEY\"]")

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
		// Save sent DM to relay's IDB.
		if myPubkey != "" {
			now := sw.NowSeconds()
			rec := makeDMRecord(recipient, myPubkey, content, now, "marmot", "")
			busSend("relay", "[\"SAVE_DM\","+rec.ToJSON()+"]")
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
