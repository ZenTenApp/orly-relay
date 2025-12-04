//go:build !(js && wasm)

package database

import (
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// FindMentions finds events that mention a pubkey via p-tags.
// This returns events grouped by depth, where depth represents how the events relate:
// - Depth 1: Events that directly mention the seed pubkey
// - Depth 2+: Not typically used for mentions (reserved for future expansion)
//
// The kinds parameter filters which event kinds to include (e.g., [1] for notes only,
// [1,7] for notes and reactions, etc.)
func (d *D) FindMentions(pubkey []byte, kinds []uint16) (*GraphResult, error) {
	result := NewGraphResult()

	if len(pubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	// Get pubkey serial
	pubkeySerial, err := d.GetPubkeySerial(pubkey)
	if err != nil {
		log.D.F("FindMentions: pubkey not in database: %s", hex.Enc(pubkey))
		return result, nil
	}

	// Find all events that reference this pubkey
	eventSerials, err := d.GetEventsReferencingPubkey(pubkeySerial, kinds)
	if err != nil {
		return nil, err
	}

	// Add each event at depth 1
	for _, eventSerial := range eventSerials {
		eventIDHex, err := d.GetEventIDFromSerial(eventSerial)
		if err != nil {
			log.D.F("FindMentions: error getting event ID for serial %d: %v", eventSerial.Get(), err)
			continue
		}
		result.AddEventAtDepth(eventIDHex, 1)
	}

	log.D.F("FindMentions: found %d events mentioning pubkey %s", result.TotalEvents, hex.Enc(pubkey))

	return result, nil
}

// FindMentionsFromHex is a convenience wrapper that accepts hex-encoded pubkey.
func (d *D) FindMentionsFromHex(pubkeyHex string, kinds []uint16) (*GraphResult, error) {
	pubkey, err := hex.Dec(pubkeyHex)
	if err != nil {
		return nil, err
	}
	return d.FindMentions(pubkey, kinds)
}

// FindMentionsByPubkeys returns events that mention any of the given pubkeys.
// Useful for finding mentions across a set of followed accounts.
func (d *D) FindMentionsByPubkeys(pubkeySerials []*types.Uint40, kinds []uint16) (*GraphResult, error) {
	result := NewGraphResult()

	seen := make(map[uint64]bool)

	for _, pubkeySerial := range pubkeySerials {
		eventSerials, err := d.GetEventsReferencingPubkey(pubkeySerial, kinds)
		if err != nil {
			log.D.F("FindMentionsByPubkeys: error for serial %d: %v", pubkeySerial.Get(), err)
			continue
		}

		for _, eventSerial := range eventSerials {
			if seen[eventSerial.Get()] {
				continue
			}
			seen[eventSerial.Get()] = true

			eventIDHex, err := d.GetEventIDFromSerial(eventSerial)
			if err != nil {
				continue
			}
			result.AddEventAtDepth(eventIDHex, 1)
		}
	}

	return result, nil
}
