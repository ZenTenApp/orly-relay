package main

import (
	"common/helpers"
	"common/jsbridge/registry"
	"common/jsbridge/sw"
	"common/nostr"
)

// Subscription Router domain — central dispatcher.
// Extension calls (crypto, marmot) go through the registry.

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

	case "MLS_INIT":
		relayURLs := w.strs()
		json := stringsToJSON(relayURLs)
		if !registry.HasHook("marmotInit") {
			registry.LoadModule("swmarmot", func() { registry.MarmotInit(json) })
		} else {
			registry.MarmotInit(json)
		}

	case "MLS_SEND":
		recipient := w.str()
		content := w.str()
		if !registry.HasHook("marmotSend") {
			registry.LoadModule("swmarmot", func() { registry.MarmotSend(recipient, content) })
		} else {
			registry.MarmotSend(recipient, content)
		}

	case "MLS_SUB":
		if !registry.HasHook("marmotSubscribe") {
			registry.LoadModule("swmarmot", func() { registry.MarmotSubscribe() })
		} else {
			registry.MarmotSubscribe()
		}

	case "MLS_PUBLISH_KP":
		relayURLs := w.strs()
		json := stringsToJSON(relayURLs)
		if !registry.HasHook("marmotPublishKP") {
			registry.LoadModule("swmarmot", func() { registry.MarmotPublishKP(json) })
		} else {
			registry.MarmotPublishKP(json)
		}

	case "MLS_LIST_GROUPS":
		if !registry.HasHook("marmotListGroups") {
			registry.LoadModule("swmarmot", func() { registry.MarmotListGroups(clientID) })
		} else {
			registry.MarmotListGroups(clientID)
		}

	case "CRYPTO_RESULT":
		id := int(w.num())
		result := w.str()
		errMsg := w.str()
		if fn, ok := cryptoCBs[id]; ok {
			delete(cryptoCBs, id)
			fn(result, errMsg)
		}

	case "PAGE":
		page := w.str()
		routerPageHint(page)
	}
}

// --- REQ / CLOSE / EVENT ---

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

func routerClose(subID string) {
	delete(clientSubs, subID)
	routerCleanupProxy(subID)
}

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

	proxyID := subID
	proxySubs[subID].timer = sw.SetTimeout(5000, func() {
		info, ok := proxySubs[proxyID]
		if ok && !info.done {
			info.done = true
			if cs, ok := clientSubs[proxyID]; ok {
				sendToClient(cs.clientID, "[\"EOSE\","+jstr(proxyID)+"]")
			}
		}
	})
}

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

func routerOnRelayEvent(relayURL string, ev *nostr.Event) {
	evJSON := ev.ToJSON()

	pushToMatchingSubs(ev)

	cacheStore(evJSON, func(saved bool) {
		if saved {
			if ev.Kind != 4 && ev.Kind != 1059 {
				relayPublishExcept(ev, relayURL)
			}
		}
		if ev.Kind == 4 || ev.Kind == 1059 {
			decryptFn := func() {
				registry.DecryptDM(evJSON, func(dmRecJSON string) {
					if dmRecJSON != "" {
						routerSaveDMRecordJSON(dmRecJSON)
					}
				})
			}
			if !registry.HasHook("decryptDM") {
				registry.LoadModule("swcrypto", func() { decryptFn() })
			} else {
				decryptFn()
			}
		}
	})
}

func routerOnRelayEOSE(subID string) {
	for proxyID, info := range proxySubs {
		if info.remoteIDs[subID] {
			info.eoseCount++
			if info.eoseCount >= info.relayCount && !info.done {
				info.done = true
				sw.ClearTimeout(info.timer)
				if cs, ok := clientSubs[proxyID]; ok {
					sendToClient(cs.clientID, "[\"EOSE\","+jstr(proxyID)+"]")
				}
			}
		}
	}
}

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

// routerSaveDMRecordJSON stores a DM record (JSON) and notifies clients.
func routerSaveDMRecordJSON(dmJSON string) {
	cacheSaveDM(dmJSON, func(result string) {
		if result != "duplicate" {
			broadcastToClients("[\"DM_RECEIVED\"," + dmJSON + "]")
		}
	})
}

func routerSendDM(clientID, recipientPubkey, content string, relayURLs []string) {
	if myPubkey == "" || !hasKey {
		return
	}
	if !registry.HasHook("encryptNip04") {
		registry.LoadModule("swcrypto", func() {
			routerSendDM(clientID, recipientPubkey, content, relayURLs)
		})
		return
	}

	// NIP-04 via crypto extension.
	registry.EncryptNip04(recipientPubkey, content, func(ev04JSON string) {
		cacheStore(ev04JSON, func(_ bool) {})
		ev04 := nostr.ParseEvent(ev04JSON)
		if ev04 != nil {
			for _, url := range relayURLs {
				getConn(url).Publish(ev04)
			}
		}
	})

	// NIP-17 via crypto extension.
	registry.EncryptNip17(recipientPubkey, content, func(recipientJSON, senderJSON string) {
		if recipientJSON != "" {
			rw := nostr.ParseEvent(recipientJSON)
			if rw != nil {
				for _, url := range relayURLs {
					getConn(url).Publish(rw)
				}
			}
		}
		if senderJSON != "" {
			swEv := nostr.ParseEvent(senderJSON)
			if swEv != nil {
				for _, url := range relayURLs {
					getConn(url).Publish(swEv)
				}
			}
		}
	})

	// Save sent DM record.
	now := sw.NowSeconds()
	recJSON := registry.MakeDMRecord(recipientPubkey, myPubkey, content, now, "nip17", "")
	if recJSON != "" {
		routerSaveDMRecordJSON(recJSON)
	}
	sendToClient(clientID, "[\"DM_SENT\","+jstr(recipientPubkey)+",true,\"\"]")
}

func routerDMSub(_ string, relayURLs []string) {
	if myPubkey == "" || len(relayURLs) == 0 {
		return
	}
	dmRelayURLs = relayURLs

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

// --- Page hints (Phase 4) ---

func routerPageHint(page string) {
	if page == "messaging" {
		if !registry.HasHook("encryptNip04") {
			registry.LoadModule("swcrypto", nil)
		}
		if !registry.HasHook("marmotInit") {
			registry.LoadModule("swmarmot", nil)
		}
	}
}

// --- Helpers ---

func stringsToJSON(ss []string) string {
	if len(ss) == 0 {
		return "[]"
	}
	b := "["
	for i, s := range ss {
		if i > 0 {
			b += ","
		}
		b += jstr(s)
	}
	return b + "]"
}
