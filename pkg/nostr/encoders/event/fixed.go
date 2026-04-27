package event

import (
	"git.smesh.lol/orly/pkg/nostr/types"
)

// Fixed-type accessor methods for Event.
// These methods provide type-safe access to event fields using fixed-size
// array types that are stack-allocated and copied on assignment.
//
// Migration guide:
//   - Old: ev.ID ([]byte, mutable, may escape to heap)
//   - New: ev.IDFixed() (types.EventID, immutable copy, stays on stack)
//
// The slice fields (ID, Pubkey, Sig) are retained for backward compatibility.
// New code should prefer the Fixed methods for type safety and performance.

// IDFixed returns the event ID as a fixed-size array.
// The returned value is a copy - modifications do not affect the event.
func (ev *E) IDFixed() types.EventID {
	return types.EventIDFromBytes(ev.ID)
}

// SetIDFixed sets the event ID from a fixed-size array.
// This updates the underlying slice field for backward compatibility.
func (ev *E) SetIDFixed(id types.EventID) {
	if ev.ID == nil {
		ev.ID = make([]byte, types.EventIDSize)
	}
	copy(ev.ID, id[:])
}

// PubkeyFixed returns the event pubkey as a fixed-size array.
// The returned value is a copy - modifications do not affect the event.
func (ev *E) PubkeyFixed() types.Pubkey {
	return types.PubkeyFromBytes(ev.Pubkey)
}

// SetPubkeyFixed sets the event pubkey from a fixed-size array.
// This updates the underlying slice field for backward compatibility.
func (ev *E) SetPubkeyFixed(pk types.Pubkey) {
	if ev.Pubkey == nil {
		ev.Pubkey = make([]byte, types.PubkeySize)
	}
	copy(ev.Pubkey, pk[:])
}

// SigFixed returns the event signature as a fixed-size array.
// The returned value is a copy - modifications do not affect the event.
func (ev *E) SigFixed() types.Signature {
	return types.SignatureFromBytes(ev.Sig)
}

// SetSigFixed sets the event signature from a fixed-size array.
// This updates the underlying slice field for backward compatibility.
func (ev *E) SetSigFixed(sig types.Signature) {
	if ev.Sig == nil {
		ev.Sig = make([]byte, types.SignatureSize)
	}
	copy(ev.Sig, sig[:])
}

// IDHex returns the event ID as a lowercase hex string.
// Convenience method equivalent to hex.EncodeToString(ev.ID).
func (ev *E) IDHex() string {
	return types.EventIDFromBytes(ev.ID).Hex()
}

// PubkeyHex returns the event pubkey as a lowercase hex string.
// Convenience method equivalent to hex.EncodeToString(ev.Pubkey).
func (ev *E) PubkeyHex() string {
	return types.PubkeyFromBytes(ev.Pubkey).Hex()
}

// SigHex returns the event signature as a lowercase hex string.
// Convenience method equivalent to hex.EncodeToString(ev.Sig).
func (ev *E) SigHex() string {
	return types.SignatureFromBytes(ev.Sig).Hex()
}
