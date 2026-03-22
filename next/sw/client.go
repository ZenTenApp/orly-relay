package main

import (
	"common/helpers"
	"common/jsbridge/sw"
	"common/nostr"
	"common/relay"
)

// Relay Proxy domain — WebSocket connection management.
// Receives events from relays, routes them to the Subscription Router.
// Never touches cache or DM crypto directly.

var (
	rpool       *relay.Pool
	writeRelays []string
)

func initRelayProxy() {
	rpool = relay.NewPool(16)
}

// getConn gets or creates a relay connection with event routing.
func getConn(url string) *relay.Conn {
	c := rpool.Connect(url)
	wireConn(c, url)
	return c
}

func wireConn(c *relay.Conn, url string) {
	c.SetOnEvent(func(_ string, ev *nostr.Event) {
		routerOnRelayEvent(url, ev)
	})
	c.SetOnEOSE(func(subID string) {
		routerOnRelayEOSE(subID)
	})
	c.SetOnOK(func(eventID string, ok bool, msg string) {
		okStr := "true"
		if !ok {
			okStr = "false"
		}
		broadcastToClients("[\"OK\"," + jstr(eventID) + "," + okStr + "," + jstr(msg) + "]")
	})
	c.SetOnAuth(func(challenge string) {
		onRelayAuth(url, challenge)
	})
}

func onRelayAuth(relayURL, challenge string) {
	if !hasKey || myPubkey == "" {
		return
	}
	authEv := &nostr.Event{
		Kind:      22242,
		Content:   "",
		Tags:      nostr.Tags{{"relay", relayURL}, {"challenge", challenge}},
		CreatedAt: sw.NowSeconds(),
	}
	if !identitySignEvent(authEv) {
		return
	}
	c := rpool.Get(relayURL)
	if c != nil && c.IsOpen() {
		c.Send("[\"AUTH\"," + authEv.ToJSON() + "]")
	}
}

// relayPublish sends an event to all write relays.
func relayPublish(ev *nostr.Event) {
	for _, url := range writeRelays {
		getConn(url).Publish(ev)
	}
}

// relayPublishExcept sends an event to all write relays except the specified one.
func relayPublishExcept(ev *nostr.Event, exceptURL string) {
	for _, wr := range writeRelays {
		if wr != exceptURL {
			getConn(wr).Publish(ev)
		}
	}
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
		_ = helpers.Itoa(0)
		sendToClient(clientID, "[\"RELAY_INFO\","+jstr(relayURL)+",null]")
	})
}
