package validation

import (
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

// ValidateTimestamp checks that the event timestamp is not too far in the future.
// maxFutureSeconds is the maximum allowed seconds ahead of current time.
func ValidateTimestamp(ev *event.E, maxFutureSeconds int64) Result {
	now := time.Now().Unix()
	if ev.CreatedAt > now+maxFutureSeconds {
		return Invalid("timestamp too far in the future")
	}
	return OK()
}
