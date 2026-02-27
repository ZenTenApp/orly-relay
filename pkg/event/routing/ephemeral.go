package routing

import (
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/lol/log"
)

// Publisher abstracts event delivery to subscribers.
type Publisher interface {
	// Deliver sends an event to all matching subscribers.
	Deliver(ev *event.E)
}

// IsEphemeral checks if a kind is ephemeral (20000-29999).
func IsEphemeral(k uint16) bool {
	return kind.IsEphemeral(k)
}

// MakeEphemeralHandler creates a handler for ephemeral events.
// Ephemeral events (kinds 20000-29999):
// - Are NOT persisted to the database
// - Are immediately delivered to subscribers
func MakeEphemeralHandler(publisher Publisher) Handler {
	return func(ev *event.E, authedPubkey []byte) Result {
		log.D.F("ephemeral handler received event kind %d, id %0x", ev.Kind, ev.ID[:8])
		// Clone and deliver immediately without persistence
		cloned := ev.Clone()
		go publisher.Deliver(cloned)
		return HandledResult("")
	}
}
