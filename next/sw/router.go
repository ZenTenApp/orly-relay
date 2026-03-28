package main

import (
	"common/helpers"
	"common/jsbridge/sw"
)

// Subscription Router — thin forwarder to relay SW via bus.
// All relay operations, subscriptions, and caching are handled by the relay SW.

// pendingSentDMs holds MLS_SEND saves deferred because myPubkey wasn't set yet.
// Replayed when SET_PUBKEY arrives.
type pendingSentDM struct {
	recipient string
	content   string
}

var pendingSentDMs []pendingSentDM
var mlsRelays []string

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
	sw.Log("shell: page→" + msgType)
	switch msgType {
	// Identity — handle locally + forward to relay SW.
	case "SET_PUBKEY":
		pk := w.str()
		identitySetPubkey(pk)
		busSend("relay", "[\"SET_PUBKEY\","+jstr(pk)+"]")
		flushPendingSentDMs()
	case "CLEAR_KEY":
		identityClearKey()
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

	// MLS — proxy through page to signer extension (marmot WASM runs inside signer).
	case "MLS_INIT":
		relays := w.strs()
		mlsRelays = relays
		sendToClient(clientID, "[\"MLS_PROXY\",\"init\","+strsJSON(relays)+"]")
	case "MLS_SEND":
		recipient := w.str()
		content := w.str()
		sendToClient(clientID, "[\"MLS_PROXY\",\"sendDM\","+jstr(recipient)+","+jstr(content)+"]")
		// Save sent DM to relay's IDB (quiet — no DM_RECEIVED broadcast).
		if myPubkey == "" {
			pendingSentDMs = append(pendingSentDMs, pendingSentDM{recipient, content})
		} else {
			now := sw.NowSeconds()
			rec := makeDMRecord(recipient, myPubkey, content, now, "marmot", "")
			busSend("relay", "[\"SAVE_DM_QUIET\","+rec.ToJSON()+"]")
		}
	case "MLS_SUB":
		sendToClient(clientID, "[\"MLS_PROXY\",\"subscribe\"]")
	case "MLS_PUBLISH_KP":
		sendToClient(clientID, "[\"MLS_PROXY\",\"publishKP\"]")
	case "MLS_LIST_GROUPS":
		sendToClient(clientID, "[\"MLS_PROXY\",\"listGroups\"]")

	// MLS results from page (mls-bridge.mjs routes signer extension outputs here).
	// Relay URLs may come from mlsRelays (set by MLS_INIT) or inline in the message
	// (set by mls-bridge.mjs). The inline URLs ensure routing works even before the
	// Go WASM app sends MLS_INIT.
	case "MLS_PUBLISH":
		eventRaw := w.str()
		relays := w.strs()
		if len(relays) == 0 {
			relays = mlsRelays
		}
		if len(relays) > 0 {
			busSend("relay", "[\"MLS_RELAY_PUBLISH\","+eventRaw+","+strsJSON(relays)+"]")
		} else {
			busSend("relay", "[\"EVENT\",\"\","+eventRaw+"]")
		}
	case "MLS_SUBSCRIBE":
		subID := w.str()
		filterRaw := w.raw()
		relays := w.strs()
		if len(relays) == 0 {
			relays = mlsRelays
		}
		// Pass filters as-is (array or single object) — relay SW's parseFilters handles both.
		mSubID := "marmot-sub-" + subID
		if len(relays) > 0 {
			busSend("relay", "[\"PROXY\",\"\","+jstr(mSubID)+","+filterRaw+","+strsJSON(relays)+"]")
		} else {
			busSend("relay", "[\"REQ\",\"\","+jstr(mSubID)+","+filterRaw+"]")
		}
	case "MLS_DM":
		dmJSON := w.raw()
		// mls-bridge sends {peer, sender, content, ts, source, eventId}
		// but IDB expects {id, peer, from, content, created_at, protocol, eventId}.
		peer := jsonField(dmJSON, "peer")
		sender := jsonField(dmJSON, "sender")
		content := jsonField(dmJSON, "content")
		ts := parseTS(jsonFieldRaw(dmJSON, "ts"))
		source := jsonField(dmJSON, "source")
		eventID := jsonField(dmJSON, "eventId")
		rec := makeDMRecord(peer, sender, content, ts, source, eventID)
		recJSON := rec.ToJSON()
		busSend("relay", "[\"SAVE_DM_QUIET\","+recJSON+"]")
		fwdDM(recJSON)
	case "MLS_GROUPS":
		groupsJSON := w.raw()
		broadcastToClients("[\"MLS_GROUPS\"," + groupsJSON + "]")
	case "MLS_STATUS":
		statusMsg := w.str()
		broadcastToClients("[\"MLS_STATUS\"," + jstr(statusMsg) + "]")

	// MLS relay event delivery — when relay events match marmot subscriptions,
	// forward them to the page which routes to the signer extension's WASM.
	case "MLS_DELIVER_EVENT":
		subID := w.str()
		eventJSON := w.raw()
		// Event must be a JSON string (not raw object) — Go WASM calls args[1].String().
		broadcastToClients("[\"MLS_PROXY\",\"deliverEvent\"," + subID + "," + jstr(eventJSON) + "]")

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

// fwdDM broadcasts a DM_RECEIVED message to all page clients.
func fwdDM(dmJSON string) {
	broadcastToClients("[\"DM_RECEIVED\"," + dmJSON + "]")
}
