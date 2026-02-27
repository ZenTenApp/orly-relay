//go:build !(js && wasm)

package database

import (
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/database/indexes/types"
)

// AddInboundRefsToResult collects inbound references (events that reference discovered items)
// for events at a specific depth in the result.
//
// For example, if you have a follows graph result and want to find all kind-7 reactions
// to posts by users at depth 1, this collects those reactions and adds them to result.InboundRefs.
//
// Parameters:
// - result: The graph result to augment with ref data
// - depth: The depth at which to collect refs (0 = all depths)
// - kinds: Event kinds to collect (e.g., [7] for reactions, [6] for reposts)
func (d *D) AddInboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error {
	// Determine which events to find refs for
	var targetEventIDs []string

	if depth == 0 {
		// Collect for all depths
		targetEventIDs = result.GetAllEvents()
	} else {
		targetEventIDs = result.GetEventsAtDepth(depth)
	}

	// Also collect refs for events authored by pubkeys in the result
	// This is common for "find reactions to posts by my follows" queries
	pubkeys := result.GetAllPubkeys()
	for _, pubkeyHex := range pubkeys {
		pubkeySerial, err := d.PubkeyHexToSerial(pubkeyHex)
		if err != nil {
			continue
		}

		// Get events authored by this pubkey
		// For efficiency, limit to relevant event kinds that might have reactions
		authoredEvents, err := d.GetEventsByAuthor(pubkeySerial, []uint16{1, 30023}) // notes and articles
		if err != nil {
			continue
		}

		for _, eventSerial := range authoredEvents {
			eventIDHex, err := d.GetEventIDFromSerial(eventSerial)
			if err != nil {
				continue
			}
			// Add to target list if not already tracking
			if !result.HasEvent(eventIDHex) {
				targetEventIDs = append(targetEventIDs, eventIDHex)
			}
		}
	}

	// For each target event, find referencing events
	for _, eventIDHex := range targetEventIDs {
		eventSerial, err := d.EventIDHexToSerial(eventIDHex)
		if err != nil {
			continue
		}

		refSerials, err := d.GetReferencingEvents(eventSerial, kinds)
		if err != nil {
			continue
		}

		for _, refSerial := range refSerials {
			refEventIDHex, err := d.GetEventIDFromSerial(refSerial)
			if err != nil {
				continue
			}

			// Get the kind of the referencing event
			// For now, use the first kind in the filter (assumes single kind queries)
			// TODO: Look up actual event kind from index if needed
			if len(kinds) > 0 {
				result.AddInboundRef(kinds[0], eventIDHex, refEventIDHex)
			}
		}
	}

	log.D.F("AddInboundRefsToResult: collected refs for %d target events", len(targetEventIDs))

	return nil
}

// AddOutboundRefsToResult collects outbound references (events referenced by discovered items).
//
// For example, find all events that posts by users at depth 1 reference (quoted posts, replied-to posts).
func (d *D) AddOutboundRefsToResult(result *GraphResult, depth int, kinds []uint16) error {
	// Determine source events
	var sourceEventIDs []string

	if depth == 0 {
		sourceEventIDs = result.GetAllEvents()
	} else {
		sourceEventIDs = result.GetEventsAtDepth(depth)
	}

	// Also include events authored by pubkeys in result
	pubkeys := result.GetAllPubkeys()
	for _, pubkeyHex := range pubkeys {
		pubkeySerial, err := d.PubkeyHexToSerial(pubkeyHex)
		if err != nil {
			continue
		}

		authoredEvents, err := d.GetEventsByAuthor(pubkeySerial, kinds)
		if err != nil {
			continue
		}

		for _, eventSerial := range authoredEvents {
			eventIDHex, err := d.GetEventIDFromSerial(eventSerial)
			if err != nil {
				continue
			}
			if !result.HasEvent(eventIDHex) {
				sourceEventIDs = append(sourceEventIDs, eventIDHex)
			}
		}
	}

	// For each source event, find referenced events
	for _, eventIDHex := range sourceEventIDs {
		eventSerial, err := d.EventIDHexToSerial(eventIDHex)
		if err != nil {
			continue
		}

		refSerials, err := d.GetETagsFromEventSerial(eventSerial)
		if err != nil {
			continue
		}

		for _, refSerial := range refSerials {
			refEventIDHex, err := d.GetEventIDFromSerial(refSerial)
			if err != nil {
				continue
			}

			// Use first kind for categorization
			if len(kinds) > 0 {
				result.AddOutboundRef(kinds[0], eventIDHex, refEventIDHex)
			}
		}
	}

	log.D.F("AddOutboundRefsToResult: collected refs from %d source events", len(sourceEventIDs))

	return nil
}

// CollectRefsForPubkeys collects inbound references to events by specific pubkeys.
// This is useful for "find all reactions to posts by these users" queries.
//
// Parameters:
// - pubkeySerials: The pubkeys whose events should be checked for refs
// - refKinds: Event kinds to collect (e.g., [7] for reactions)
// - eventKinds: Event kinds to check for refs (e.g., [1] for notes)
func (d *D) CollectRefsForPubkeys(
	pubkeySerials []*types.Uint40,
	refKinds []uint16,
	eventKinds []uint16,
) (*GraphResult, error) {
	result := NewGraphResult()

	for _, pubkeySerial := range pubkeySerials {
		// Get events by this author
		authoredEvents, err := d.GetEventsByAuthor(pubkeySerial, eventKinds)
		if err != nil {
			continue
		}

		for _, eventSerial := range authoredEvents {
			eventIDHex, err := d.GetEventIDFromSerial(eventSerial)
			if err != nil {
				continue
			}

			// Find refs to this event
			refSerials, err := d.GetReferencingEvents(eventSerial, refKinds)
			if err != nil {
				continue
			}

			for _, refSerial := range refSerials {
				refEventIDHex, err := d.GetEventIDFromSerial(refSerial)
				if err != nil {
					continue
				}

				// Add to result
				if len(refKinds) > 0 {
					result.AddInboundRef(refKinds[0], eventIDHex, refEventIDHex)
				}
			}
		}
	}

	return result, nil
}
