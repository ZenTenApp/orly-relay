//go:build !(js && wasm)

package database

import (
	"sort"
)

// GraphResult contains depth-organized traversal results for graph queries.
// It tracks pubkeys and events discovered at each depth level, ensuring
// each entity appears only at the depth where it was first discovered.
type GraphResult struct {
	// PubkeysByDepth maps depth -> pubkeys first discovered at that depth.
	// Each pubkey appears ONLY in the array for the depth where it was first seen.
	// Depth 1 = direct connections, Depth 2 = connections of connections, etc.
	PubkeysByDepth map[int][]string

	// EventsByDepth maps depth -> event IDs discovered at that depth.
	// Used for thread traversal queries.
	EventsByDepth map[int][]string

	// FirstSeenPubkey tracks which depth each pubkey was first discovered.
	// Key is pubkey hex, value is the depth (1-indexed).
	FirstSeenPubkey map[string]int

	// FirstSeenEvent tracks which depth each event was first discovered.
	// Key is event ID hex, value is the depth (1-indexed).
	FirstSeenEvent map[string]int

	// TotalPubkeys is the count of unique pubkeys discovered across all depths.
	TotalPubkeys int

	// TotalEvents is the count of unique events discovered across all depths.
	TotalEvents int

	// InboundRefs tracks inbound references (events that reference discovered items).
	// Structure: kind -> target_id -> []referencing_event_ids
	InboundRefs map[uint16]map[string][]string

	// OutboundRefs tracks outbound references (events referenced by discovered items).
	// Structure: kind -> source_id -> []referenced_event_ids
	OutboundRefs map[uint16]map[string][]string
}

// NewGraphResult creates a new initialized GraphResult.
func NewGraphResult() *GraphResult {
	return &GraphResult{
		PubkeysByDepth:  make(map[int][]string),
		EventsByDepth:   make(map[int][]string),
		FirstSeenPubkey: make(map[string]int),
		FirstSeenEvent:  make(map[string]int),
		InboundRefs:     make(map[uint16]map[string][]string),
		OutboundRefs:    make(map[uint16]map[string][]string),
	}
}

// AddPubkeyAtDepth adds a pubkey to the result at the specified depth if not already seen.
// Returns true if the pubkey was added (first time seen), false if already exists.
func (r *GraphResult) AddPubkeyAtDepth(pubkeyHex string, depth int) bool {
	if _, exists := r.FirstSeenPubkey[pubkeyHex]; exists {
		return false
	}

	r.FirstSeenPubkey[pubkeyHex] = depth
	r.PubkeysByDepth[depth] = append(r.PubkeysByDepth[depth], pubkeyHex)
	r.TotalPubkeys++
	return true
}

// AddEventAtDepth adds an event ID to the result at the specified depth if not already seen.
// Returns true if the event was added (first time seen), false if already exists.
func (r *GraphResult) AddEventAtDepth(eventIDHex string, depth int) bool {
	if _, exists := r.FirstSeenEvent[eventIDHex]; exists {
		return false
	}

	r.FirstSeenEvent[eventIDHex] = depth
	r.EventsByDepth[depth] = append(r.EventsByDepth[depth], eventIDHex)
	r.TotalEvents++
	return true
}

// HasPubkey returns true if the pubkey has been discovered at any depth.
func (r *GraphResult) HasPubkey(pubkeyHex string) bool {
	_, exists := r.FirstSeenPubkey[pubkeyHex]
	return exists
}

// HasEvent returns true if the event has been discovered at any depth.
func (r *GraphResult) HasEvent(eventIDHex string) bool {
	_, exists := r.FirstSeenEvent[eventIDHex]
	return exists
}

// GetPubkeyDepth returns the depth at which a pubkey was first discovered.
// Returns 0 if the pubkey was not found.
func (r *GraphResult) GetPubkeyDepth(pubkeyHex string) int {
	return r.FirstSeenPubkey[pubkeyHex]
}

// GetEventDepth returns the depth at which an event was first discovered.
// Returns 0 if the event was not found.
func (r *GraphResult) GetEventDepth(eventIDHex string) int {
	return r.FirstSeenEvent[eventIDHex]
}

// GetDepthsSorted returns all depths that have pubkeys, sorted ascending.
func (r *GraphResult) GetDepthsSorted() []int {
	depths := make([]int, 0, len(r.PubkeysByDepth))
	for d := range r.PubkeysByDepth {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	return depths
}

// GetEventDepthsSorted returns all depths that have events, sorted ascending.
func (r *GraphResult) GetEventDepthsSorted() []int {
	depths := make([]int, 0, len(r.EventsByDepth))
	for d := range r.EventsByDepth {
		depths = append(depths, d)
	}
	sort.Ints(depths)
	return depths
}

// ToDepthArrays converts the result to the response format: array of arrays.
// Index 0 = depth 1 pubkeys, Index 1 = depth 2 pubkeys, etc.
// Empty arrays are included for depths with no pubkeys to maintain index alignment.
func (r *GraphResult) ToDepthArrays() [][]string {
	if len(r.PubkeysByDepth) == 0 {
		return [][]string{}
	}

	// Find the maximum depth
	maxDepth := 0
	for d := range r.PubkeysByDepth {
		if d > maxDepth {
			maxDepth = d
		}
	}

	// Create result array with entries for each depth
	result := make([][]string, maxDepth)
	for i := 0; i < maxDepth; i++ {
		depth := i + 1 // depths are 1-indexed
		if pubkeys, exists := r.PubkeysByDepth[depth]; exists {
			result[i] = pubkeys
		} else {
			result[i] = []string{} // Empty array for depths with no pubkeys
		}
	}
	return result
}

// ToEventDepthArrays converts event results to the response format: array of arrays.
// Index 0 = depth 1 events, Index 1 = depth 2 events, etc.
func (r *GraphResult) ToEventDepthArrays() [][]string {
	if len(r.EventsByDepth) == 0 {
		return [][]string{}
	}

	maxDepth := 0
	for d := range r.EventsByDepth {
		if d > maxDepth {
			maxDepth = d
		}
	}

	result := make([][]string, maxDepth)
	for i := 0; i < maxDepth; i++ {
		depth := i + 1
		if events, exists := r.EventsByDepth[depth]; exists {
			result[i] = events
		} else {
			result[i] = []string{}
		}
	}
	return result
}

// AddInboundRef records an inbound reference from a referencing event to a target.
func (r *GraphResult) AddInboundRef(kind uint16, targetIDHex string, referencingEventIDHex string) {
	if r.InboundRefs[kind] == nil {
		r.InboundRefs[kind] = make(map[string][]string)
	}
	r.InboundRefs[kind][targetIDHex] = append(r.InboundRefs[kind][targetIDHex], referencingEventIDHex)
}

// AddOutboundRef records an outbound reference from a source event to a referenced event.
func (r *GraphResult) AddOutboundRef(kind uint16, sourceIDHex string, referencedEventIDHex string) {
	if r.OutboundRefs[kind] == nil {
		r.OutboundRefs[kind] = make(map[string][]string)
	}
	r.OutboundRefs[kind][sourceIDHex] = append(r.OutboundRefs[kind][sourceIDHex], referencedEventIDHex)
}

// RefAggregation represents aggregated reference data for a single target/source.
type RefAggregation struct {
	// TargetEventID is the event ID being referenced (for inbound) or referencing (for outbound)
	TargetEventID string

	// TargetAuthor is the author pubkey of the target event (if known)
	TargetAuthor string

	// TargetDepth is the depth at which this target was discovered in the graph
	TargetDepth int

	// RefKind is the kind of the referencing events
	RefKind uint16

	// RefCount is the number of references to/from this target
	RefCount int

	// RefEventIDs is the list of event IDs that reference this target
	RefEventIDs []string
}

// GetInboundRefsSorted returns inbound refs for a kind, sorted by count descending.
func (r *GraphResult) GetInboundRefsSorted(kind uint16) []RefAggregation {
	kindRefs := r.InboundRefs[kind]
	if kindRefs == nil {
		return nil
	}

	aggs := make([]RefAggregation, 0, len(kindRefs))
	for targetID, refs := range kindRefs {
		agg := RefAggregation{
			TargetEventID: targetID,
			TargetDepth:   r.GetEventDepth(targetID),
			RefKind:       kind,
			RefCount:      len(refs),
			RefEventIDs:   refs,
		}
		aggs = append(aggs, agg)
	}

	// Sort by count descending
	sort.Slice(aggs, func(i, j int) bool {
		return aggs[i].RefCount > aggs[j].RefCount
	})

	return aggs
}

// GetOutboundRefsSorted returns outbound refs for a kind, sorted by count descending.
func (r *GraphResult) GetOutboundRefsSorted(kind uint16) []RefAggregation {
	kindRefs := r.OutboundRefs[kind]
	if kindRefs == nil {
		return nil
	}

	aggs := make([]RefAggregation, 0, len(kindRefs))
	for sourceID, refs := range kindRefs {
		agg := RefAggregation{
			TargetEventID: sourceID,
			TargetDepth:   r.GetEventDepth(sourceID),
			RefKind:       kind,
			RefCount:      len(refs),
			RefEventIDs:   refs,
		}
		aggs = append(aggs, agg)
	}

	sort.Slice(aggs, func(i, j int) bool {
		return aggs[i].RefCount > aggs[j].RefCount
	})

	return aggs
}

// GetAllPubkeys returns all pubkeys discovered across all depths.
func (r *GraphResult) GetAllPubkeys() []string {
	all := make([]string, 0, r.TotalPubkeys)
	for _, pubkeys := range r.PubkeysByDepth {
		all = append(all, pubkeys...)
	}
	return all
}

// GetAllEvents returns all event IDs discovered across all depths.
func (r *GraphResult) GetAllEvents() []string {
	all := make([]string, 0, r.TotalEvents)
	for _, events := range r.EventsByDepth {
		all = append(all, events...)
	}
	return all
}

// GetPubkeysAtDepth returns pubkeys at a specific depth, or empty slice if none.
func (r *GraphResult) GetPubkeysAtDepth(depth int) []string {
	if pubkeys, exists := r.PubkeysByDepth[depth]; exists {
		return pubkeys
	}
	return []string{}
}

// GetEventsAtDepth returns events at a specific depth, or empty slice if none.
func (r *GraphResult) GetEventsAtDepth(depth int) []string {
	if events, exists := r.EventsByDepth[depth]; exists {
		return events
	}
	return []string{}
}

// Interface methods for external package access (e.g., pkg/protocol/graph)
// These allow the graph executor to extract data without direct struct access.

// GetPubkeysByDepth returns the PubkeysByDepth map for external access.
func (r *GraphResult) GetPubkeysByDepth() map[int][]string {
	return r.PubkeysByDepth
}

// GetEventsByDepth returns the EventsByDepth map for external access.
func (r *GraphResult) GetEventsByDepth() map[int][]string {
	return r.EventsByDepth
}

// GetTotalPubkeys returns the total pubkey count for external access.
func (r *GraphResult) GetTotalPubkeys() int {
	return r.TotalPubkeys
}

// GetTotalEvents returns the total event count for external access.
func (r *GraphResult) GetTotalEvents() int {
	return r.TotalEvents
}

// GetInboundRefs returns the InboundRefs map for external access.
func (r *GraphResult) GetInboundRefs() map[uint16]map[string][]string {
	return r.InboundRefs
}

// GetOutboundRefs returns the OutboundRefs map for external access.
func (r *GraphResult) GetOutboundRefs() map[uint16]map[string][]string {
	return r.OutboundRefs
}
