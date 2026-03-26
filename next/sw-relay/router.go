package main

import (
	"common/helpers"
	"common/jsbridge/sw"
	"common/nostr"
)

// Subscription Router — central dispatcher for relay operations.
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

// --- REQ / CLOSE / EVENT ---

func routerReq(clientID, subID, filterRaw string) {
	f := nostr.ParseFilter(filterRaw)
	if f == nil {
		sw.Log("relay-sw: REQ parse filter FAILED")
		return
	}
	clientSubs[subID] = &clientSub{filter: f, filterRaw: filterRaw, clientID: clientID}

	cacheQuery(filterRaw, func(eventsJSON string) {
		events := nostr.ParseEventsJSON(eventsJSON)
		for _, ev := range events {
			fwd(clientID, "[\"EVENT\","+jstr(subID)+","+ev.ToJSON()+"]")
		}
		fwd(clientID, "[\"EOSE\","+jstr(subID)+"]")
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
	fwd(clientID, "[\"OK\","+jstr(ev.ID)+",true,\"\"]")
}

// --- PROXY subscriptions ---

func routerProxy(clientID, subID, filterRaw string, relayURLs []string) {
	routerCleanupProxy(subID)

	f := nostr.ParseFilter(filterRaw)
	if f == nil {
		sw.Log("relay-sw: PROXY parse filter FAILED")
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
	proxySubs[subID].timer = sw.SetTimeout(20000, func() {
		info, ok := proxySubs[proxyID]
		if ok && !info.done {
			info.done = true
			if cs, ok := clientSubs[proxyID]; ok {
				fwd(cs.clientID, "[\"EOSE\","+jstr(proxyID)+"]")
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
			fwd(cs.clientID, "[\"EOSE\","+jstr(proxyID)+"]")
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
			relayPublishExcept(ev, relayURL)
		}
	})
}

func routerOnRelayEOSE(subID string) {
	for proxyID, info := range proxySubs {
		if info.remoteIDs[subID] {
			info.eoseCount++
			if !info.done {
				sw.ClearTimeout(info.timer)
				pid := proxyID
				info.timer = sw.SetTimeout(10000, func() {
					inf, ok := proxySubs[pid]
					if ok && !inf.done {
						inf.done = true
						if cs, ok := clientSubs[pid]; ok {
							fwd(cs.clientID, "[\"EOSE\","+jstr(pid)+"]")
						}
					}
				})
			}
		}
	}
}

func pushToMatchingSubs(ev *nostr.Event) {
	matched := 0
	for subID, cs := range clientSubs {
		if cs.filter.Matches(ev) {
			matched++
			fwd(cs.clientID, "[\"EVENT\","+jstr(subID)+","+ev.ToJSON()+"]")
		}
	}
}

// --- Signing ---

func routerSign(clientID, requestID, eventRaw string) {
	if !hasKey {
		fwd(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"no key\"]")
		return
	}
	ev := nostr.ParseEvent(eventRaw)
	if ev == nil {
		fwd(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"parse error\"]")
		return
	}
	if identitySignEvent(ev) {
		fwd(clientID, "[\"SIGNED\","+jstr(requestID)+","+ev.ToJSON()+"]")
	} else {
		fwd(clientID, "[\"SIGN_ERROR\","+jstr(requestID)+",\"sign failed\"]")
	}
}

// --- DM routing ---

func routerSaveDMRecord(rec *DMRecord) {
	dmJSON := rec.ToJSON()
	cacheSaveDM(dmJSON, func(result string) {
		if result != "duplicate" {
			fwdAll("[\"DM_RECEIVED\"," + dmJSON + "]")
		}
	})
}

func routerDMList(clientID string) {
	cacheGetConversationList(func(listJSON string) {
		fwd(clientID, "[\"DM_LIST\","+listJSON+"]")
	})
}

func routerDMHistory(clientID, peer string, limit int, until int64) {
	if limit <= 0 {
		limit = 50
	}
	cacheQueryDMs(peer, limit, until, func(msgsJSON string) {
		fwd(clientID, "[\"DM_HISTORY\","+jstr(peer)+","+msgsJSON+"]")
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
		fwd(clientID, "[\"BROADCAST_DONE\","+helpers.Itoa(int64(count))+","+helpers.Itoa(int64(len(relayURLs)))+"]")
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
