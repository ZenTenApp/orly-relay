package validation

import (
	"fmt"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"next.orly.dev/pkg/utils"
)

// ValidateEventID checks that the event ID matches the computed hash.
func ValidateEventID(ev *event.E) Result {
	calculatedID := ev.GetIDBytes()
	if !utils.FastEqual(calculatedID, ev.ID) {
		return Invalid(fmt.Sprintf(
			"event id is computed incorrectly, event has ID %0x, but when computed it is %0x",
			ev.ID, calculatedID,
		))
	}
	return OK()
}

// ValidateSignature verifies the event signature.
func ValidateSignature(ev *event.E) Result {
	ok, err := ev.Verify()
	if err != nil {
		return Error(fmt.Sprintf("failed to verify signature: %s", err.Error()))
	}
	if !ok {
		return Invalid("signature is invalid")
	}
	return OK()
}
