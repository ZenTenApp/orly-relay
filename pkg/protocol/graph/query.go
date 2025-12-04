// Package graph implements NIP-XX Graph Query protocol support.
// It provides types and functions for parsing and validating graph traversal queries.
package graph

import (
	"encoding/json"
	"errors"

	"git.mleku.dev/mleku/nostr/encoders/filter"
)

// Query represents a graph traversal query from a _graph filter extension.
type Query struct {
	// Method is the traversal method: "follows", "followers", "mentions", "thread"
	Method string `json:"method"`

	// Seed is the starting point for traversal (pubkey hex or event ID hex)
	Seed string `json:"seed"`

	// Depth is the maximum traversal depth (1-16, default: 1)
	Depth int `json:"depth,omitempty"`

	// InboundRefs specifies which inbound references to collect
	// (events that reference discovered events via e-tags)
	InboundRefs []RefSpec `json:"inbound_refs,omitempty"`

	// OutboundRefs specifies which outbound references to collect
	// (events referenced by discovered events via e-tags)
	OutboundRefs []RefSpec `json:"outbound_refs,omitempty"`
}

// RefSpec specifies which event references to include in results.
type RefSpec struct {
	// Kinds is the list of event kinds to match (OR semantics within this spec)
	Kinds []int `json:"kinds"`

	// FromDepth specifies the minimum depth at which to collect refs (default: 0)
	// 0 = include refs from seed itself
	// 1 = start from first-hop connections
	FromDepth int `json:"from_depth,omitempty"`
}

// Validation errors
var (
	ErrMissingMethod    = errors.New("_graph.method is required")
	ErrInvalidMethod    = errors.New("_graph.method must be one of: follows, followers, mentions, thread")
	ErrMissingSeed      = errors.New("_graph.seed is required")
	ErrInvalidSeed      = errors.New("_graph.seed must be a 64-character hex string")
	ErrDepthTooHigh     = errors.New("_graph.depth cannot exceed 16")
	ErrEmptyRefSpecKinds = errors.New("ref spec kinds array cannot be empty")
)

// Valid method names
var validMethods = map[string]bool{
	"follows":   true,
	"followers": true,
	"mentions":  true,
	"thread":    true,
}

// Validate checks the query for correctness and applies defaults.
func (q *Query) Validate() error {
	// Method is required
	if q.Method == "" {
		return ErrMissingMethod
	}
	if !validMethods[q.Method] {
		return ErrInvalidMethod
	}

	// Seed is required
	if q.Seed == "" {
		return ErrMissingSeed
	}
	if len(q.Seed) != 64 {
		return ErrInvalidSeed
	}
	// Validate hex characters
	for _, c := range q.Seed {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ErrInvalidSeed
		}
	}

	// Apply depth defaults and limits
	if q.Depth < 1 {
		q.Depth = 1
	}
	if q.Depth > 16 {
		return ErrDepthTooHigh
	}

	// Validate ref specs
	for _, rs := range q.InboundRefs {
		if len(rs.Kinds) == 0 {
			return ErrEmptyRefSpecKinds
		}
	}
	for _, rs := range q.OutboundRefs {
		if len(rs.Kinds) == 0 {
			return ErrEmptyRefSpecKinds
		}
	}

	return nil
}

// HasInboundRefs returns true if the query includes inbound reference collection.
func (q *Query) HasInboundRefs() bool {
	return len(q.InboundRefs) > 0
}

// HasOutboundRefs returns true if the query includes outbound reference collection.
func (q *Query) HasOutboundRefs() bool {
	return len(q.OutboundRefs) > 0
}

// HasRefs returns true if the query includes any reference collection.
func (q *Query) HasRefs() bool {
	return q.HasInboundRefs() || q.HasOutboundRefs()
}

// InboundKindsAtDepth returns a set of kinds that should be collected at the given depth.
// It aggregates all RefSpecs where from_depth <= depth.
func (q *Query) InboundKindsAtDepth(depth int) map[int]bool {
	kinds := make(map[int]bool)
	for _, rs := range q.InboundRefs {
		if rs.FromDepth <= depth {
			for _, k := range rs.Kinds {
				kinds[k] = true
			}
		}
	}
	return kinds
}

// OutboundKindsAtDepth returns a set of kinds that should be collected at the given depth.
func (q *Query) OutboundKindsAtDepth(depth int) map[int]bool {
	kinds := make(map[int]bool)
	for _, rs := range q.OutboundRefs {
		if rs.FromDepth <= depth {
			for _, k := range rs.Kinds {
				kinds[k] = true
			}
		}
	}
	return kinds
}

// ExtractFromFilter checks if a filter has a _graph extension and parses it.
// Returns nil if no _graph field is present.
// Returns an error if _graph is present but invalid.
func ExtractFromFilter(f *filter.F) (*Query, error) {
	if f == nil || f.Extra == nil {
		return nil, nil
	}

	raw, ok := f.Extra["_graph"]
	if !ok {
		return nil, nil
	}

	var q Query
	if err := json.Unmarshal(raw, &q); err != nil {
		return nil, err
	}

	if err := q.Validate(); err != nil {
		return nil, err
	}

	return &q, nil
}

// IsGraphQuery returns true if the filter contains a _graph extension.
// This is a quick check that doesn't parse the full query.
func IsGraphQuery(f *filter.F) bool {
	if f == nil || f.Extra == nil {
		return false
	}
	_, ok := f.Extra["_graph"]
	return ok
}
