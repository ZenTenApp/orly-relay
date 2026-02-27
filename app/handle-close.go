package app

import (
	"errors"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/envelopes/closeenvelope"
)

// HandleClose processes a CLOSE envelope by unmarshalling the request,
// validates the presence of an <id> field, and signals cancellation for
// the associated listener through the server's publisher mechanism.
func (l *Listener) HandleClose(req []byte) (err error) {
	var rem []byte
	env := closeenvelope.New()
	if rem, err = env.Unmarshal(req); chk.E(err) {
		return
	}
	if len(rem) > 0 {
		log.T.F("extra '%s'", rem)
	}
	if len(env.ID) == 0 {
		return errors.New("CLOSE has no <id>")
	}

	subID := string(env.ID)

	// Cancel the subscription goroutine by calling its cancel function
	l.subscriptionsMu.Lock()
	if cancelFunc, exists := l.subscriptions[subID]; exists {
		log.D.F("cancelling subscription %s for %s", subID, l.remote)
		cancelFunc()
		delete(l.subscriptions, subID)
	} else {
		log.D.F("subscription %s not found for %s (already closed?)", subID, l.remote)
	}
	l.subscriptionsMu.Unlock()

	// Also remove from publisher's tracking
	l.publishers.Receive(
		&W{
			Cancel: true,
			remote: l.remote,
			Conn:   l.conn,
			Id:     subID,
		},
	)

	log.D.F("CLOSE processed for subscription %s @ %s", subID, l.remote)
	return
}
