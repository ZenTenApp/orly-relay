package neo4j

import (
	"sort"
)

// GraphResult contains depth-organized traversal results for graph queries.
// It tracks pubkeys and events discovered at each depth level, ensuring
// each entity appears only at the depth where it was first discovered.
//
// This is the Neo4j implementation that mirrors the Badger implementation
// in pkg/database/graph-result.go, implementing the graph.GraphResultI interface.
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
}

// NewGraphResult creates a new initialized GraphResult.
func NewGraphResult() *GraphResult {
	return &GraphResult{
		PubkeysByDepth:  make(map[int][]string),
		EventsByDepth:   make(map[int][]string),
		FirstSeenPubkey: make(map[string]int),
		FirstSeenEvent:  make(map[string]int),
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
