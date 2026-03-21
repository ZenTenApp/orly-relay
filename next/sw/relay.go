package main

import (
	"common/jsbridge/idb"
	"common/jsbridge/sw"
	"common/nostr"
	"common/relay"
)

// getConn gets or creates a relay connection with SW event routing.
func getConn(url string) *relay.Conn {
	c := rpool.Connect(url)
	wireConn(c, url)
	return c
}

func wireConn(c *relay.Conn, url string) {
	c.SetOnEvent(func(_ string, ev *nostr.Event) {
		onRelayEvent(url, ev)
	})
	c.SetOnEOSE(func(subID string) {
		onRelayEOSE(subID)
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

func onRelayEvent(relayURL string, ev *nostr.Event) {
	evJSON := ev.ToJSON()

	idb.SaveEvent(evJSON, func(saved bool) {
		if saved {
			pushToMatchingSubs(ev)
			// Propagate to write relays (skip source, skip DMs).
			if ev.Kind != 4 && ev.Kind != 1059 {
				for _, wr := range writeRelays {
					if wr != relayURL {
						c := getConn(wr)
						c.Publish(ev)
					}
				}
			}
		}
		// Process DMs regardless of saved.
		if ev.Kind == 4 || ev.Kind == 1059 {
			processIncomingDM(ev)
		}
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
	aux := random32()
	if !authEv.Sign(seckey, aux) {
		sw.Warn("AUTH sign failed for " + relayURL)
		return
	}
	c := rpool.Get(relayURL)
	if c != nil && c.IsOpen() {
		c.Send("[\"AUTH\"," + authEv.ToJSON() + "]")
	}
}
