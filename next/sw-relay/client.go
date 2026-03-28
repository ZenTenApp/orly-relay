package main

import (
	"common/jsbridge/sw"
	"common/nostr"
	"common/relay"
)

// Relay Proxy — WebSocket connection management.
// Receives events from relays, routes them to the Subscription Router.

var (
	rpool       *relay.Pool
	writeRelays []string
)

func initRelayProxy() {
	rpool = relay.NewPool()
}

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
		fwdAll("[\"OK\"," + jstr(eventID) + "," + okStr + "," + jstr(msg) + "]")
	})
	c.SetOnAuth(func(challenge string) {
		onRelayAuth(url, challenge)
	})
	c.ScheduleReconnect = func(fn func()) {
		sw.SetTimeout(5000, fn)
	}
}

func onRelayAuth(relayURL, challenge string) {
	if myPubkey == "" {
		return
	}
	authEv := &nostr.Event{
		Kind:      22242,
		PubKey:    myPubkey,
		Content:   "",
		Tags:      nostr.Tags{{"relay", relayURL}, {"challenge", challenge}},
		CreatedAt: sw.NowSeconds(),
	}
	// Proxy signing through shell SW -> signer extension.
	cryptoProxy("signEvent", "", authEv.ToJSON(), func(signedJSON, errMsg string) {
		if errMsg != "" || signedJSON == "" {
			return
		}
		c := rpool.Get(relayURL)
		if c != nil && c.IsOpen() {
			c.Send("[\"AUTH\"," + signedJSON + "]")
		}
	})
}

func relayPublish(ev *nostr.Event) {
	for _, url := range writeRelays {
		getConn(url).Publish(ev)
	}
}

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
	fwd(clientID, "[\"RELAY_INFO\","+jstr(relayURL)+",null]")
}
