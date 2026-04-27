//go:build !(js && wasm)

package database

import (
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database/indexes/types"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// TraverseThread performs BFS traversal of thread structure via e-tags.
// Starting from a seed event, it finds all replies/references at each depth.
//
// The traversal works bidirectionally:
// - Forward: Events that the seed references (parents, quoted posts)
// - Backward: Events that reference the seed (replies, reactions, reposts)
//
// Parameters:
// - seedEventID: The event ID to start traversal from
// - maxDepth: Maximum depth to traverse
// - direction: "both" (default), "inbound" (replies to seed), "outbound" (seed's references)
func (d *D) TraverseThread(seedEventID []byte, maxDepth int, direction string) (*GraphResult, error) {
	result := NewGraphResult()

	if len(seedEventID) != 32 {
		return result, ErrEventNotFound
	}

	// Get seed event serial
	seedSerial, err := d.GetSerialById(seedEventID)
	if err != nil {
		log.D.F("TraverseThread: seed event not in database: %s", hex.Enc(seedEventID))
		return result, nil
	}

	// Normalize direction
	if direction == "" {
		direction = "both"
	}

	// Track visited events
	visited := make(map[uint64]bool)
	visited[seedSerial.Get()] = true

	// Current frontier
	currentFrontier := []*types.Uint40{seedSerial}

	consecutiveEmptyDepths := 0

	for currentDepth := 1; currentDepth <= maxDepth; currentDepth++ {
		var nextFrontier []*types.Uint40
		newEventsAtDepth := 0

		for _, eventSerial := range currentFrontier {
			// Get inbound references (events that reference this event)
			if direction == "both" || direction == "inbound" {
				inboundSerials, err := d.GetReferencingEvents(eventSerial, nil)
				if err != nil {
					log.D.F("TraverseThread: error getting inbound refs for serial %d: %v", eventSerial.Get(), err)
				} else {
					for _, refSerial := range inboundSerials {
						if visited[refSerial.Get()] {
							continue
						}
						visited[refSerial.Get()] = true

						eventIDHex, err := d.GetEventIDFromSerial(refSerial)
						if err != nil {
							continue
						}

						result.AddEventAtDepth(eventIDHex, currentDepth)
						newEventsAtDepth++
						nextFrontier = append(nextFrontier, refSerial)
					}
				}
			}

			// Get outbound references (events this event references)
			if direction == "both" || direction == "outbound" {
				outboundSerials, err := d.GetETagsFromEventSerial(eventSerial)
				if err != nil {
					log.D.F("TraverseThread: error getting outbound refs for serial %d: %v", eventSerial.Get(), err)
				} else {
					for _, refSerial := range outboundSerials {
						if visited[refSerial.Get()] {
							continue
						}
						visited[refSerial.Get()] = true

						eventIDHex, err := d.GetEventIDFromSerial(refSerial)
						if err != nil {
							continue
						}

						result.AddEventAtDepth(eventIDHex, currentDepth)
						newEventsAtDepth++
						nextFrontier = append(nextFrontier, refSerial)
					}
				}
			}
		}

		log.T.F("TraverseThread: depth %d found %d new events", currentDepth, newEventsAtDepth)

		if newEventsAtDepth == 0 {
			consecutiveEmptyDepths++
			if consecutiveEmptyDepths >= 2 {
				break
			}
		} else {
			consecutiveEmptyDepths = 0
		}

		currentFrontier = nextFrontier
	}

	log.D.F("TraverseThread: completed with %d total events", result.TotalEvents)

	return result, nil
}

// TraverseThreadFromHex is a convenience wrapper that accepts hex-encoded event ID.
func (d *D) TraverseThreadFromHex(seedEventIDHex string, maxDepth int, direction string) (*GraphResult, error) {
	seedEventID, err := hex.Dec(seedEventIDHex)
	if err != nil {
		return nil, err
	}
	return d.TraverseThread(seedEventID, maxDepth, direction)
}

// GetThreadReplies finds all direct replies to an event.
// This is a convenience method that returns events at depth 1 with inbound direction.
func (d *D) GetThreadReplies(eventID []byte, kinds []uint16) (*GraphResult, error) {
	result := NewGraphResult()

	if len(eventID) != 32 {
		return result, ErrEventNotFound
	}

	eventSerial, err := d.GetSerialById(eventID)
	if err != nil {
		return result, nil
	}

	// Get events that reference this event
	replySerials, err := d.GetReferencingEvents(eventSerial, kinds)
	if err != nil {
		return nil, err
	}

	for _, replySerial := range replySerials {
		eventIDHex, err := d.GetEventIDFromSerial(replySerial)
		if err != nil {
			continue
		}
		result.AddEventAtDepth(eventIDHex, 1)
	}

	return result, nil
}

// GetThreadParents finds events that a given event references (its parents/quotes).
func (d *D) GetThreadParents(eventID []byte) (*GraphResult, error) {
	result := NewGraphResult()

	if len(eventID) != 32 {
		return result, ErrEventNotFound
	}

	eventSerial, err := d.GetSerialById(eventID)
	if err != nil {
		return result, nil
	}

	// Get events that this event references
	parentSerials, err := d.GetETagsFromEventSerial(eventSerial)
	if err != nil {
		return nil, err
	}

	for _, parentSerial := range parentSerials {
		eventIDHex, err := d.GetEventIDFromSerial(parentSerial)
		if err != nil {
			continue
		}
		result.AddEventAtDepth(eventIDHex, 1)
	}

	return result, nil
}
