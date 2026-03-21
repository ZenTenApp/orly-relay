package main

import (
	"common/helpers"
	"common/jsbridge/idb"
	"common/jsbridge/sw"
	"common/nostr"
)

func handleReq(clientID, subID, filterRaw string) {
	f := nostr.ParseFilter(filterRaw)
	if f == nil {
		return
	}
	clientSubs[subID] = &clientSub{filter: f, filterRaw: filterRaw, clientID: clientID}

	// Query IDB for cached events.
	idb.QueryEvents(filterRaw, func(eventsJSON string) {
		events := nostr.ParseEventsJSON(eventsJSON)
		for _, ev := range events {
			sendToClient(clientID, "[\"EVENT\","+jstr(subID)+","+ev.ToJSON()+"]")
		}
		sendToClient(clientID, "[\"EOSE\","+jstr(subID)+"]")
	})
}

func handleClose(subID string) {
	cleanupProxy(subID)
	delete(clientSubs, subID)
}

func pushToMatchingSubs(ev *nostr.Event) {
	for subID, cs := range clientSubs {
		if cs.filter.Matches(ev) {
			sendToClient(cs.clientID, "[\"EVENT\","+jstr(subID)+","+ev.ToJSON()+"]")
		}
	}
}

func handleProxy(clientID, subID, filterRaw string, relayURLs []string) {
	// Clean up existing proxy with same ID.
	cleanupProxy(subID)

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

func cleanupProxy(proxyID string) {
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

	// Close remote subs on all connections.
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

func onRelayEOSE(subID string) {
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

func handlePublish(clientID string, eventRaw string) {
	ev := nostr.ParseEvent(eventRaw)
	if ev == nil {
		return
	}

	idb.SaveEvent(eventRaw, func(saved bool) {
		if saved {
			pushToMatchingSubs(ev)
		}
	})

	for _, url := range writeRelays {
		c := getConn(url)
		c.Publish(ev)
	}

	sendToClient(clientID, "[\"OK\","+jstr(ev.ID)+",true,\"\"]")
}

func urlSuffix(url string) string {
	n := min(len(url), 8)
	out := make([]byte, 0, n)
	for i := len(url) - n; i < len(url); i++ {
		c := url[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			out = append(out, c)
		}
	}
	return string(out)
}

func handleRelayInfo(clientID, relayURL string) {
	httpURL := relayURL
	if len(httpURL) > 6 && httpURL[:6] == "wss://" {
		httpURL = "https://" + httpURL[6:]
	} else if len(httpURL) > 5 && httpURL[:5] == "ws://" {
		httpURL = "http://" + httpURL[5:]
	}
	sw.Fetch(httpURL, func(resp sw.Response, ok bool) {
		if !ok {
			sendToClient(clientID, "[\"RELAY_INFO\","+jstr(relayURL)+",null]")
			return
		}
		// Can't read response body with current jsbridge. Send null for now.
		_ = helpers.Itoa(0)
		sendToClient(clientID, "[\"RELAY_INFO\","+jstr(relayURL)+",null]")
	})
}
