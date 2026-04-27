//go:build !(js && wasm)

package database

import (
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// TraversePubkeyPubkey performs BFS traversal of the pubkey↔pubkey graph using
// the ppg/gpp materialized index. This collapses the two-hop
// pubkey→event→pubkey traversal into a single prefix scan per frontier node.
//
// Direction selects which index to scan:
//   - "out":  ppg index (who does seed follow/reference?)
//   - "in":   gpp index (who references seed?)
//   - "both": union of both directions at each depth
func (d *D) TraversePubkeyPubkey(seedPubkey []byte, maxDepth int, direction string) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	seedSerial, err := d.GetPubkeySerial(seedPubkey)
	if err != nil {
		log.D.F("TraversePubkeyPubkey: seed not in database: %s", hex.Enc(seedPubkey))
		return result, nil
	}

	visited := make(map[uint64]bool)
	visited[seedSerial.Get()] = true

	currentFrontier := []*types.Uint40{seedSerial}
	consecutiveEmpty := 0

	for depth := 1; depth <= maxDepth; depth++ {
		var nextFrontier []*types.Uint40
		newCount := 0

		for _, serial := range currentFrontier {
			var neighbors []*types.Uint40

			if direction == "out" || direction == "both" {
				outNeighbors, err := d.GetFollowsByKindViaPPG(serial, 3)
				if err != nil {
					log.D.F("TraversePubkeyPubkey: ppg scan error for serial %d: %v", serial.Get(), err)
					continue
				}
				neighbors = append(neighbors, outNeighbors...)
			}

			if direction == "in" || direction == "both" {
				inNeighbors, err := d.GetFollowersByKindViaGPP(serial, 3)
				if err != nil {
					log.D.F("TraversePubkeyPubkey: gpp scan error for serial %d: %v", serial.Get(), err)
					continue
				}
				neighbors = append(neighbors, inNeighbors...)
			}

			for _, neighborSerial := range neighbors {
				if visited[neighborSerial.Get()] {
					continue
				}
				visited[neighborSerial.Get()] = true

				pubkeyHex, err := d.GetPubkeyHexFromSerial(neighborSerial)
				if err != nil {
					continue
				}

				result.AddPubkeyAtDepth(pubkeyHex, depth)
				newCount++
				nextFrontier = append(nextFrontier, neighborSerial)
			}
		}

		log.T.F("TraversePubkeyPubkey: depth %d found %d new pubkeys (ppg/gpp)", depth, newCount)

		if newCount == 0 {
			consecutiveEmpty++
			if consecutiveEmpty >= 2 {
				break
			}
		} else {
			consecutiveEmpty = 0
		}

		currentFrontier = nextFrontier
	}

	log.D.F("TraversePubkeyPubkey: completed with %d pubkeys across %d depths",
		result.TotalPubkeys, len(result.PubkeysByDepth))

	return result, nil
}

// TraversePubkeyPubkeyBaseline performs the same BFS as TraversePubkeyPubkey
// but uses the old multi-hop approach: find kind-3 event → extract p-tags.
// This provides the benchmark baseline for comparison against the ppg/gpp index.
func (d *D) TraversePubkeyPubkeyBaseline(seedPubkey []byte, maxDepth int, direction string) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	seedSerial, err := d.GetPubkeySerial(seedPubkey)
	if err != nil {
		log.D.F("TraversePubkeyPubkeyBaseline: seed not in database: %s", hex.Enc(seedPubkey))
		return result, nil
	}

	visited := make(map[uint64]bool)
	visited[seedSerial.Get()] = true

	currentFrontier := []*types.Uint40{seedSerial}
	consecutiveEmpty := 0

	for depth := 1; depth <= maxDepth; depth++ {
		var nextFrontier []*types.Uint40
		newCount := 0

		for _, serial := range currentFrontier {
			var neighbors []*types.Uint40

			if direction == "out" || direction == "both" {
				// Old path: find kind-3 event, extract p-tags (two index lookups)
				follows, err := d.GetFollowsFromPubkeySerial(serial)
				if err != nil {
					continue
				}
				neighbors = append(neighbors, follows...)
			}

			if direction == "in" || direction == "both" {
				// Old path: find kind-3 events referencing target, get authors (O(N) lookups)
				followers, err := d.GetFollowersOfPubkeySerial(serial)
				if err != nil {
					continue
				}
				neighbors = append(neighbors, followers...)
			}

			for _, neighborSerial := range neighbors {
				if visited[neighborSerial.Get()] {
					continue
				}
				visited[neighborSerial.Get()] = true

				pubkeyHex, err := d.GetPubkeyHexFromSerial(neighborSerial)
				if err != nil {
					continue
				}

				result.AddPubkeyAtDepth(pubkeyHex, depth)
				newCount++
				nextFrontier = append(nextFrontier, neighborSerial)
			}
		}

		log.T.F("TraversePubkeyPubkeyBaseline: depth %d found %d new pubkeys (multi-hop)", depth, newCount)

		if newCount == 0 {
			consecutiveEmpty++
			if consecutiveEmpty >= 2 {
				break
			}
		} else {
			consecutiveEmpty = 0
		}

		currentFrontier = nextFrontier
	}

	log.D.F("TraversePubkeyPubkeyBaseline: completed with %d pubkeys across %d depths",
		result.TotalPubkeys, len(result.PubkeysByDepth))

	return result, nil
}

// TraversePubkeyEvent performs BFS traversal of pubkey↔event edges.
// Direction "out" = events authored by/referencing the seed pubkey (peg index).
// Direction "in" = events that reference the seed pubkey (epg reverse).
func (d *D) TraversePubkeyEvent(seedPubkey []byte, maxDepth int, direction string) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	seedSerial, err := d.GetPubkeySerial(seedPubkey)
	if err != nil {
		log.D.F("TraversePubkeyEvent: seed not in database: %s", hex.Enc(seedPubkey))
		return result, nil
	}

	// For pe queries, depth 1 = direct events for the seed pubkey
	if direction == "out" || direction == "both" {
		// Events authored by seed
		eventSerials, err := d.GetEventsByAuthor(seedSerial, nil)
		if err == nil {
			for _, es := range eventSerials {
				eventHex, err := d.GetEventIDFromSerial(es)
				if err != nil {
					continue
				}
				result.AddEventAtDepth(eventHex, 1)
			}
		}
	}

	if direction == "in" || direction == "both" {
		// Events referencing seed via p-tag
		eventSerials, err := d.GetEventsReferencingPubkey(seedSerial, nil)
		if err == nil {
			for _, es := range eventSerials {
				eventHex, err := d.GetEventIDFromSerial(es)
				if err != nil {
					continue
				}
				result.AddEventAtDepth(eventHex, 1)
			}
		}
	}

	log.D.F("TraversePubkeyEvent: completed with %d events", result.TotalEvents)
	return result, nil
}

// TraverseEventEventFromPubkey performs BFS traversal of event↔event edges,
// seeded from events authored by the given pubkey.
func (d *D) TraverseEventEventFromPubkey(seedPubkey []byte, maxDepth int, direction string) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedPubkey) != 32 {
		return result, ErrPubkeyNotFound
	}

	seedSerial, err := d.GetPubkeySerial(seedPubkey)
	if err != nil {
		log.D.F("TraverseEventEvent: seed not in database: %s", hex.Enc(seedPubkey))
		return result, nil
	}

	// Get seed events (authored by the pubkey)
	seedEvents, err := d.GetEventsByAuthor(seedSerial, nil)
	if err != nil {
		return result, err
	}

	// Use the existing TraverseThread for each seed event
	visited := make(map[uint64]bool)
	for _, eventSerial := range seedEvents {
		eventID, err := d.GetEventIdBySerial(eventSerial)
		if err != nil {
			continue
		}
		subResult, err := d.TraverseThread(eventID, maxDepth, direction)
		if err != nil {
			continue
		}
		// Merge sub-results
		for depth, events := range subResult.EventsByDepth {
			for _, e := range events {
				_ = visited // TraverseThread handles its own dedup
				result.AddEventAtDepth(e, depth)
			}
		}
	}

	log.D.F("TraverseEventEvent: completed with %d events from %d seed events",
		result.TotalEvents, len(seedEvents))
	return result, nil
}
