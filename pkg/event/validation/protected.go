package validation

import (
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/utils"
)

// ValidateProtectedTagMatch checks NIP-70 protected tag requirements.
// Events with the "-" tag can only be published by users authenticated
// with the same pubkey as the event author.
func ValidateProtectedTagMatch(ev *event.E, authedPubkey []byte) Result {
	// Check for protected tag (NIP-70)
	protectedTag := ev.Tags.GetFirst([]byte("-"))
	if protectedTag == nil {
		return OK() // No protected tag, validation passes
	}

	// Event has protected tag - verify pubkey matches
	if !utils.FastEqual(authedPubkey, ev.Pubkey) {
		return Blocked("protected tag may only be published by user authed to the same pubkey")
	}

	return OK()
}

// HasProtectedTag checks if an event has the NIP-70 protected tag.
func HasProtectedTag(ev *event.E) bool {
	return ev.Tags.GetFirst([]byte("-")) != nil
}
