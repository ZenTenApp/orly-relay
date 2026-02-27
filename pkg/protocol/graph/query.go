// Package graph implements NIP-XX Graph Query protocol support.
// It provides types and functions for parsing and validating graph traversal queries.
//
// The _graph filter extension uses a 4-element array:
//
//	["_graph", ["<pubkey_hex>", <depth>, "<edge>", "<direction>"]]
//
// Parameters:
//   - pubkey:    64-char hex seed pubkey — the starting node for traversal
//   - depth:     integer 1-16 — maximum BFS depth from seed
//   - edge:      "pp" | "pe" | "ee" — which graph edge family to traverse
//   - direction: "out" | "in" | "both" — traversal direction
//
// Edge types map to the relational grammar:
//   - "pp" (pubkey↔pubkey): noun-noun — direct social graph (materialized ppg/gpp index)
//   - "pe" (pubkey↔event):  adverb   — authorship and p-tag references (epg/peg index)
//   - "ee" (event↔event):   adjective — e-tag thread structure (eeg/gee index)
package graph

import (
	"encoding/json"
	"errors"
	"fmt"

	"next.orly.dev/pkg/nostr/encoders/filter"
)

// Query represents a graph traversal query from a _graph filter extension.
type Query struct {
	// Pubkey is the 64-char hex seed pubkey for traversal.
	Pubkey string `json:"pubkey"`

	// Depth is the maximum BFS traversal depth (1-16, default: 1).
	Depth int `json:"depth"`

	// Edge selects which graph edge family to traverse:
	// "pp" = pubkey↔pubkey, "pe" = pubkey↔event, "ee" = event↔event
	Edge string `json:"edge"`

	// Direction selects traversal direction: "out", "in", or "both".
	Direction string `json:"direction"`
}

// Validation errors
var (
	ErrMissingPubkey    = errors.New("_graph[0] (pubkey) is required")
	ErrInvalidPubkey    = errors.New("_graph[0] (pubkey) must be a 64-character hex string")
	ErrInvalidDepth     = errors.New("_graph[1] (depth) must be between 1 and 16")
	ErrMissingEdge      = errors.New("_graph[2] (edge) is required")
	ErrInvalidEdge      = errors.New("_graph[2] (edge) must be one of: pp, pe, ee")
	ErrMissingDirection = errors.New("_graph[3] (direction) is required")
	ErrInvalidDirection = errors.New("_graph[3] (direction) must be one of: out, in, both")
	ErrInvalidArray     = errors.New("_graph must be a 4-element array: [pubkey, depth, edge, direction]")
)

// Valid edge types
var validEdges = map[string]bool{
	"pp": true, // pubkey↔pubkey (noun-noun)
	"pe": true, // pubkey↔event (adverb)
	"ee": true, // event↔event (adjective)
}

// Valid direction values
var validDirections = map[string]bool{
	"out":  true,
	"in":   true,
	"both": true,
}

// Validate checks the query for correctness and applies defaults.
func (q *Query) Validate() error {
	// Pubkey is required
	if q.Pubkey == "" {
		return ErrMissingPubkey
	}
	if len(q.Pubkey) != 64 {
		return ErrInvalidPubkey
	}
	for _, c := range q.Pubkey {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return ErrInvalidPubkey
		}
	}

	// Apply depth defaults and limits
	if q.Depth < 1 {
		q.Depth = 1
	}
	if q.Depth > 16 {
		return ErrInvalidDepth
	}

	// Edge is required
	if q.Edge == "" {
		return ErrMissingEdge
	}
	if !validEdges[q.Edge] {
		return ErrInvalidEdge
	}

	// Direction is required
	if q.Direction == "" {
		return ErrMissingDirection
	}
	if !validDirections[q.Direction] {
		return ErrInvalidDirection
	}

	return nil
}

// ExtractFromFilter checks if a filter has a _graph extension and parses it.
// The _graph field is a 4-element JSON array: [pubkey, depth, edge, direction].
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

	// Parse as a JSON array of 4 elements
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		// Try legacy object format for backward compatibility
		var q Query
		if err2 := json.Unmarshal(raw, &q); err2 != nil {
			return nil, fmt.Errorf("_graph: expected 4-element array, got: %w", err)
		}
		if err2 := q.Validate(); err2 != nil {
			return nil, err2
		}
		return &q, nil
	}

	if len(arr) != 4 {
		return nil, ErrInvalidArray
	}

	var q Query

	// [0] pubkey: string
	if err := json.Unmarshal(arr[0], &q.Pubkey); err != nil {
		return nil, fmt.Errorf("_graph[0] (pubkey): %w", err)
	}

	// [1] depth: number
	if err := json.Unmarshal(arr[1], &q.Depth); err != nil {
		return nil, fmt.Errorf("_graph[1] (depth): %w", err)
	}

	// [2] edge: string
	if err := json.Unmarshal(arr[2], &q.Edge); err != nil {
		return nil, fmt.Errorf("_graph[2] (edge): %w", err)
	}

	// [3] direction: string
	if err := json.Unmarshal(arr[3], &q.Direction); err != nil {
		return nil, fmt.Errorf("_graph[3] (direction): %w", err)
	}

	if err := q.Validate(); err != nil {
		return nil, err
	}

	return &q, nil
}

// IsGraphQuery returns true if the filter contains a _graph extension.
func IsGraphQuery(f *filter.F) bool {
	if f == nil || f.Extra == nil {
		return false
	}
	_, ok := f.Extra["_graph"]
	return ok
}

// IsPubkeyPubkey returns true if this query traverses pubkey↔pubkey edges.
func (q *Query) IsPubkeyPubkey() bool { return q.Edge == "pp" }

// IsPubkeyEvent returns true if this query traverses pubkey↔event edges.
func (q *Query) IsPubkeyEvent() bool { return q.Edge == "pe" }

// IsEventEvent returns true if this query traverses event↔event edges.
func (q *Query) IsEventEvent() bool { return q.Edge == "ee" }

// IsOutbound returns true if traversal follows outbound edges.
func (q *Query) IsOutbound() bool { return q.Direction == "out" || q.Direction == "both" }

// IsInbound returns true if traversal follows inbound edges.
func (q *Query) IsInbound() bool { return q.Direction == "in" || q.Direction == "both" }

// IsBidirectional returns true if traversal follows both directions.
func (q *Query) IsBidirectional() bool { return q.Direction == "both" }
