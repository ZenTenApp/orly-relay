package main

import (
	"common/helpers"
	"common/jsbridge/sw"
	"common/nostr"
)

// Subscription Router domain — the central dispatcher (spine).
// Connects all other domains: Cache, Relay Proxy, DM Crypto, Identity.
// Owns client subscriptions, filter matching, message dispatch.

var (
	clientSubs map[string]*clientSub
	proxySubs  map[string]*proxySub
)

type clientSub struct {
	filter    *nostr.Filter
	filterRaw string
	clientID  string
}

type proxySub struct {
	remoteIDs  map[string]bool
	relayCount int
	eoseCount  int
	timer      sw.Timer
	done       bool
}

func initRouter() {
	clientSubs = make(map[string]*clientSub)
	proxySubs = make(map[string]*proxySub)
}

// routeMessage dispatches an app-level message from a browser client.
func routeMessage(clientID string, w *mw, msgType string) {
	switch msgType {
	case "REQ":
		subID := w.str()
		filterRaw := w.raw()
		routerReq(clientID, subID, filterRaw)

	case "CLOSE":
		subID := w.str()
		routerClose(subID)

	case "EVENT":
		eventRaw := w.raw()
		routerPublish(clientID, eventRaw)

	case "PROXY":
		subID := w.str()
		filterRaw := w.raw()
		relayURLs := w.strs()
		routerProxy(clientID, subID, filterRaw, relayURLs)

	case "RELAY_INFO":
		relayURL := w.str()
		handleRelayInfo(clientID, relayURL)

	case "SET_KEY":
		hexKey := w.str()
		identitySetKey(hexKey)
		sendToClient(clientID, "[\"KEY_SET\"]")

	case "SET_PUBKEY":
		identitySetPubkey(w.str())

	case "CLEAR_KEY":
		identityClearKey()
		writeRelays = nil

	case "SET_WRITE_RELAYS":
		writeRelays = w.strs()

	case "SIGN":
		requestID := w.str()
		eventRaw := w.raw()
		routerSign(clientID, requestID, eventRaw)

	case "BROADCAST":
		pubkey := w.str()
		relayURLs := w.strs()
		routerBroadcast(clientID, pubkey, relayURLs)

	case "SEND_DM":
		recipientPubkey := w.str()
		content := w.str()
		relayURLs := w.strs()
		routerSendDM(clientID, recipientPubkey, content, relayURLs)

	case "DM_SUB":
		relayURLs := w.strs()
		routerDMSub(clientID, relayURLs)

	case "DM_LIST":
		routerDMList(clientID)

	case "DM_HISTORY":
		peer := w.str()
		limit := int(w.num())
		until := w.num()
		routerDMHistory(clientID, peer, limit, until)

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

// --- REQ / CLOSE / EVENT ---

// routerReq handles REQ from UI — registers subscription, queries cache, sends results.
func routerReq(clientID, subID, filterRaw string) {
	f := nostr.ParseFilter(filterRaw)
	if f == nil {
		return
	}
	clientSubs[subID] = &clientSub{filter: f, filterRaw: filterRaw, clientID: clientID}

	cacheQuery(filterRaw, func(eventsJSON string) {
		events := nostr.ParseEventsJSON(eventsJSON)
		for _, ev := range events {
			sendToClient(clientID, "[\"EVENT\","+jstr(subID)+","+ev.ToJSON()+"]")
		}
		sendToClient(clientID, "[\"EOSE\","+jstr(subID)+"]")
	})
}

// routerClose closes a client subscription and any associated proxy.
func routerClose(subID string) {
	delete(clientSubs, subID)
	routerCleanupProxy(subID)
}

// routerPublish handles EVENT from UI — store in cache, push to subs, publish to relays.
func routerPublish(clientID, eventRaw string) {
	ev := nostr.ParseEvent(eventRaw)
	if ev == nil {
		return
	}

	cacheStore(eventRaw, func(saved bool) {
		if saved {
			pushToMatchingSubs(ev)
		}
	})

	relayPublish(ev)
	sendToClient(clientID, "[\"OK\","+jstr(ev.ID)+",true,\"\"]")
}

// --- PROXY subscriptions ---

// routerProxy sets up a multi-relay proxy subscription.
func routerProxy(clientID, subID, filterRaw string, relayURLs []string) {
	routerCleanupProxy(subID)

	f := nostr.ParseFilter(filterRaw)
	if f == nil {
		return
	}
	clientSubs[subID] = &clientSub{filter: f, filterRaw: filterRaw, clientID: clientID}

	remoteIDs := make(map[string]bool)
	base := "p_" + subID + "_"

	proxySubs[subID] = &proxySub{
		remoteIDs:  remoteIDs,
		relayCount: len(relayURLs),
	}

	for _, url := range relayURLs {
		suffix := urlSuffix(url)
		rSubID := base + suffix
		remoteIDs[rSubID] = true
		c := getConn(url)
		c.Subscribe(rSubID, []*nostr.Filter{f})
	}
}

// routerCleanupProxy closes remote subscriptions and removes proxy state.
func routerCleanupProxy(proxyID string) {
	info, ok := proxySubs[proxyID]
	if !ok {
		return
	}
	sw.ClearTimeout(info.timer)

	if !info.done {
		if cs, ok := clientSubs[proxyID]; ok {
			sendToClient(cs.clientID, "[\"EOSE\","+jstr(proxyID)+"]")
		}
	}

	for rSubID := range info.remoteIDs {
		for _, url := range rpool.URLs() {
			c := rpool.Get(url)
			if c != nil && c.IsOpen() {
				c.CloseSubscription(rSubID)
			}
		}
	}
	delete(proxySubs, proxyID)
	delete(clientSubs, proxyID)
}

// --- Relay event callbacks ---

// routerOnRelayEvent is called by the Relay Proxy when an event arrives from a relay.
func routerOnRelayEvent(relayURL string, ev *nostr.Event) {
	evJSON := ev.ToJSON()

	sw.Log("relay event kind=" + helpers.Itoa(int64(ev.Kind)) + " from=" + relayURL)

	// Immediate client notification.
	pushToMatchingSubs(ev)

	// Async IDB storage + relay propagation.
	cacheStore(evJSON, func(saved bool) {
		if saved {
			if ev.Kind != 4 && ev.Kind != 1059 {
				relayPublishExcept(ev, relayURL)
			}
		}
		if ev.Kind == 4 || ev.Kind == 1059 {
			decryptIncomingDM(ev, func(rec *DMRecord) {
				routerSaveDMRecord(rec)
			})
		}
	})
}

// routerOnRelayEOSE handles EOSE from the Relay Proxy.
func routerOnRelayEOSE(subID string) {
	for proxyID, info := range proxySubs {
		if info.remoteIDs[subID] {
			info.eoseCount++
			if info.eoseCount >= info.relayCount && !info.done {
				info.done = true
				if cs, ok := clientSubs[proxyID]; ok {
					sendToClient(cs.clientID, "[\"EOSE\","+jstr(proxyID)+"]")
				}
			}
		}
	}
}

// pushToMatchingSubs sends an event to all browser clients with matching subscriptions.
func pushToMatchingSubs(ev *nostr.Event) {
	for subID, cs := range clientSubs {
		if cs.filter.Matches(ev) {
			sendToClient(cs.clientID, "[\"EVENT\","+jstr(subID)+","+ev.ToJSON()+"]")
		}
	}
}

// --- Signing ---

func routerSign(clientID, requestID, eventRaw string) {
	if !hasKey {
		sendToClient(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"no key\"]")
		return
	}
	ev := nostr.ParseEvent(eventRaw)
	if ev == nil {
		sendToClient(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"parse error\"]")
		return
	}
	if identitySignEvent(ev) {
		sendToClient(clientID, "[\"SIGNED\","+jstr(requestID)+","+ev.ToJSON()+"]")
	} else {
		sendToClient(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"sign failed\"]")
	}
}

// --- DM routing ---

// routerSaveDMRecord stores a DM record and notifies clients.
func routerSaveDMRecord(rec *DMRecord) {
	dmJSON := rec.ToJSON()
	cacheSaveDM(dmJSON, func(result string) {
		if result != "duplicate" {
			broadcastToClients("[\"DM_RECEIVED\"," + dmJSON + "]")
		}
	})
}

// routerSendDM handles SEND_DM from UI.
func routerSendDM(clientID, recipientPubkey, content string, relayURLs []string) {
	if myPubkey == "" || !hasKey {
		return
	}

	// NIP-04.
	encryptNip04DM(recipientPubkey, content, func(ev04 *nostr.Event) {
		cacheStore(ev04.ToJSON(), func(_ bool) {})
		for _, url := range relayURLs {
			getConn(url).Publish(ev04)
		}
	})

	// NIP-17.
	recipientWrap, senderWrap := encryptNip17DM(recipientPubkey, content)
	if recipientWrap != nil {
		for _, url := range relayURLs {
			getConn(url).Publish(recipientWrap)
		}
	}
	if senderWrap != nil {
		for _, url := range relayURLs {
			getConn(url).Publish(senderWrap)
		}
	}

	// Save sent DM record.
	now := sw.NowSeconds()
	rec := makeDMRecord(recipientPubkey, myPubkey, content, now, "nip17", "")
	routerSaveDMRecord(rec)
	sendToClient(clientID, "[\"DM_SENT\","+jstr(recipientPubkey)+",true,\"\"]")
}

// routerDMSub handles DM_SUB — opens DM subscriptions on relays.
func routerDMSub(_ string, relayURLs []string) {
	if myPubkey == "" || len(relayURLs) == 0 {
		return
	}
	dmRelayURLs = relayURLs

	// Close existing DM subs.
	for rSubID := range dmSubIDs {
		for _, url := range rpool.URLs() {
			c := rpool.Get(url)
			if c != nil && c.IsOpen() {
				c.CloseSubscription(rSubID)
			}
		}
	}
	dmSubIDs = make(map[string]bool)

	for _, url := range relayURLs {
		suffix := urlSuffix(url)
		id1 := "dm4in_" + suffix
		id2 := "dm4out_" + suffix
		id3 := "dm17_" + suffix
		dmSubIDs[id1] = true
		dmSubIDs[id2] = true
		dmSubIDs[id3] = true

		c := getConn(url)
		c.Subscribe(id1, []*nostr.Filter{{Kinds: []int{4}, Tags: map[string][]string{"#p": {myPubkey}}, Limit: 100}})
		c.Subscribe(id2, []*nostr.Filter{{Kinds: []int{4}, Authors: []string{myPubkey}, Limit: 100}})
		c.Subscribe(id3, []*nostr.Filter{{Kinds: []int{1059}, Tags: map[string][]string{"#p": {myPubkey}}, Limit: 100}})
	}
}

func routerDMList(clientID string) {
	cacheGetConversationList(func(listJSON string) {
		sendToClient(clientID, "[\"DM_LIST\","+listJSON+"]")
	})
}

func routerDMHistory(clientID, peer string, limit int, until int64) {
	if limit <= 0 {
		limit = 50
	}
	cacheQueryDMs(peer, limit, until, func(msgsJSON string) {
		sendToClient(clientID, "[\"DM_HISTORY\","+jstr(peer)+","+msgsJSON+"]")
	})
}

// --- Broadcast ---

// routerBroadcast handles BROADCAST — publishes identity events to relays.
func routerBroadcast(clientID, pubkey string, relayURLs []string) {
	filterJSON := "{\"authors\":[" + jstr(pubkey) + "],\"kinds\":[0,3,10002,10050,10051]}"
	cacheQuery(filterJSON, func(eventsJSON string) {
		events := nostr.ParseEventsJSON(eventsJSON)
		byKind := make(map[int]*nostr.Event)
		for _, ev := range events {
			if prev, ok := byKind[ev.Kind]; !ok || ev.CreatedAt > prev.CreatedAt {
				byKind[ev.Kind] = ev
			}
		}

		// Auto-create 10050/10051 if missing.
		userRelays := relayURLs
		if relayEv, ok := byKind[10002]; ok {
			userRelays = nil
			for _, t := range relayEv.Tags.GetAll("r") {
				userRelays = append(userRelays, t.Value())
			}
		}
		if len(userRelays) == 0 {
			userRelays = writeRelays
		}

		if _, ok := byKind[10050]; !ok && hasKey && len(userRelays) > 0 {
			ev := createRelayListEvent(10050, pubkey, userRelays)
			if ev != nil {
				cacheStore(ev.ToJSON(), func(_ bool) {})
				byKind[10050] = ev
			}
		}
		if _, ok := byKind[10051]; !ok && hasKey && len(userRelays) > 0 {
			ev := createRelayListEvent(10051, pubkey, userRelays)
			if ev != nil {
				cacheStore(ev.ToJSON(), func(_ bool) {})
				byKind[10051] = ev
			}
		}

		// Publish to relays.
		count := 0
		for _, ev := range byKind {
			for _, url := range relayURLs {
				getConn(url).Publish(ev)
			}
			count++
		}
		sendToClient(clientID, "[\"BROADCAST_DONE\","+helpers.Itoa(int64(count))+","+helpers.Itoa(int64(len(relayURLs)))+"]")
	})
}

func createRelayListEvent(kind int, _ string, relays []string) *nostr.Event {
	tagKey := "relay"
	var tags nostr.Tags
	for _, r := range relays {
		tags = append(tags, nostr.Tag{tagKey, r})
	}
	ev := &nostr.Event{
		Kind:      kind,
		Content:   "",
		Tags:      tags,
		CreatedAt: sw.NowSeconds(),
	}
	if !identitySignEvent(ev) {
		return nil
	}
	return ev
}
