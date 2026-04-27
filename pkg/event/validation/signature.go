package validation

import (
	"fmt"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/utils"
)

// ValidateEventID checks that the event ID matches the computed hash.
func ValidateEventID(ev *event.E) Result {
	calculatedID := ev.GetIDBytes()
	if !utils.FastEqual(calculatedID, ev.ID) {
		canonical := ev.ToCanonical(nil)
		log.E.F("ValidateEventID FAIL: event ID %0x computed %0x, canonical: %s",
			ev.ID, calculatedID, string(canonical))
		return Invalid(fmt.Sprintf(
			"event id is computed incorrectly, event has ID %0x, but when computed it is %0x",
			ev.ID, calculatedID,
		))
	}
	return OK()
}

// ValidateSignature verifies the event signature.
func ValidateSignature(ev *event.E) Result {
	// NIP-59 gift wraps (1059) and MLS group messages (445) use ephemeral
	// throwaway keys that provide no identity binding. The outer signature
	// is architecturally meaningless — skip validation for these kinds.
	if ev.Kind == 1059 || ev.Kind == 445 {
		return OK()
	}

	ok, err := ev.Verify()
	if err != nil {
		log.E.F("ValidateSignature ERROR: kind=%d pubkey=%s id=%0x err=%v",
			ev.Kind, hex.Enc(ev.Pubkey), ev.ID, err)
		return Error(fmt.Sprintf("failed to verify signature: %s", err.Error()))
	}
	if !ok {
		canonical := ev.ToCanonical(nil)
		log.E.F("ValidateSignature FAIL: kind=%d pubkey=%s id=%0x sig=%0x canonical=%s",
			ev.Kind, hex.Enc(ev.Pubkey), ev.ID, ev.Sig, string(canonical))
		return Invalid("signature is invalid")
	}
	return OK()
}
