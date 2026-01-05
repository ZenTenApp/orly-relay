//go:build !(js && wasm)

// Package graph implements NIP-XX Graph Query protocol support.
// This file contains the executor that runs graph traversal queries.
package graph

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// Response kinds for graph queries (ephemeral range, relay-signed)
const (
	KindGraphFollows   = 39000 // Response for follows/followers queries
	KindGraphMentions  = 39001 // Response for mentions queries
	KindGraphThread    = 39002 // Response for thread traversal queries
)

// GraphResultI is the interface that database.GraphResult implements.
// This allows the executor to work with the database result without importing it.
type GraphResultI interface {
	ToDepthArrays() [][]string
	ToEventDepthArrays() [][]string
	GetAllPubkeys() []string
	GetAllEvents() []string
	GetPubkeysByDepth() map[int][]string
	GetEventsByDepth() map[int][]string
	GetTotalPubkeys() int
	GetTotalEvents() int
	// Ref aggregation methods
	GetInboundRefs() map[uint16]map[string][]string
	GetOutboundRefs() map[uint16]map[string][]string
}

// GraphDatabase defines the interface for graph traversal operations.
// This is implemented by the database package.
type GraphDatabase interface {
	// TraverseFollows performs BFS traversal of follow graph
	TraverseFollows(seedPubkey []byte, maxDepth int) (GraphResultI, error)
	// TraverseFollowers performs BFS traversal to find followers
	TraverseFollowers(seedPubkey []byte, maxDepth int) (GraphResultI, error)
	// FindMentions finds events mentioning a pubkey
	FindMentions(pubkey []byte, kinds []uint16) (GraphResultI, error)
	// TraverseThread performs BFS traversal of thread structure
	TraverseThread(seedEventID []byte, maxDepth int, direction string) (GraphResultI, error)
	// CollectInboundRefs finds events that reference items in the result
	CollectInboundRefs(result GraphResultI, depth int, kinds []uint16) error
	// CollectOutboundRefs finds events referenced by items in the result
	CollectOutboundRefs(result GraphResultI, depth int, kinds []uint16) error
}

// Executor handles graph query execution and response generation.
type Executor struct {
	db           GraphDatabase
	relaySigner  signer.I
	relayPubkey  []byte
}

// NewExecutor creates a new graph query executor.
// The secretKey should be the 32-byte relay identity secret key.
func NewExecutor(db GraphDatabase, secretKey []byte) (*Executor, error) {
	s, err := p8k.New()
	if err != nil {
		return nil, err
	}
	if err = s.InitSec(secretKey); err != nil {
		return nil, err
	}
	return &Executor{
		db:          db,
		relaySigner: s,
		relayPubkey: s.Pub(),
	}, nil
}

// Execute runs a graph query and returns a relay-signed event with results.
func (e *Executor) Execute(q *Query) (*event.E, error) {
	var result GraphResultI
	var err error
	var responseKind uint16

	// Decode seed (hex string to bytes)
	seedBytes, err := hex.Dec(q.Seed)
	if err != nil {
		return nil, err
	}

	// Execute the appropriate traversal
	switch q.Method {
	case "follows":
		responseKind = KindGraphFollows
		result, err = e.db.TraverseFollows(seedBytes, q.Depth)
		if err != nil {
			return nil, err
		}
		log.D.F("graph executor: follows traversal returned %d pubkeys", result.GetTotalPubkeys())

	case "followers":
		responseKind = KindGraphFollows
		result, err = e.db.TraverseFollowers(seedBytes, q.Depth)
		if err != nil {
			return nil, err
		}
		log.D.F("graph executor: followers traversal returned %d pubkeys", result.GetTotalPubkeys())

	case "mentions":
		responseKind = KindGraphMentions
		// Mentions don't use depth traversal, just find direct mentions
		// Convert RefSpec kinds to uint16 for the database call
		var kinds []uint16
		if len(q.InboundRefs) > 0 {
			for _, rs := range q.InboundRefs {
				for _, k := range rs.Kinds {
					kinds = append(kinds, uint16(k))
				}
			}
		} else {
			kinds = []uint16{1} // Default to kind 1 (notes)
		}
		result, err = e.db.FindMentions(seedBytes, kinds)
		if err != nil {
			return nil, err
		}
		log.D.F("graph executor: mentions query returned %d events", result.GetTotalEvents())

	case "thread":
		responseKind = KindGraphThread
		result, err = e.db.TraverseThread(seedBytes, q.Depth, "both")
		if err != nil {
			return nil, err
		}
		log.D.F("graph executor: thread traversal returned %d events", result.GetTotalEvents())

	default:
		return nil, ErrInvalidMethod
	}

	// Collect inbound refs if specified
	if q.HasInboundRefs() {
		for _, refSpec := range q.InboundRefs {
			kinds := make([]uint16, len(refSpec.Kinds))
			for i, k := range refSpec.Kinds {
				kinds[i] = uint16(k)
			}
			// Collect refs at the specified from_depth (0 = all depths)
			if err = e.db.CollectInboundRefs(result, refSpec.FromDepth, kinds); err != nil {
				log.W.F("graph executor: failed to collect inbound refs: %v", err)
				// Continue without refs rather than failing the query
			}
		}
		log.D.F("graph executor: collected inbound refs")
	}

	// Collect outbound refs if specified
	if q.HasOutboundRefs() {
		for _, refSpec := range q.OutboundRefs {
			kinds := make([]uint16, len(refSpec.Kinds))
			for i, k := range refSpec.Kinds {
				kinds[i] = uint16(k)
			}
			if err = e.db.CollectOutboundRefs(result, refSpec.FromDepth, kinds); err != nil {
				log.W.F("graph executor: failed to collect outbound refs: %v", err)
			}
		}
		log.D.F("graph executor: collected outbound refs")
	}

	// Generate response event
	return e.generateResponse(q, result, responseKind)
}

// generateResponse creates a relay-signed event containing the query results.
func (e *Executor) generateResponse(q *Query, result GraphResultI, responseKind uint16) (*event.E, error) {
	// Build content as JSON with depth arrays
	var content ResponseContent

	if q.Method == "follows" || q.Method == "followers" {
		// For pubkey-based queries, use pubkeys_by_depth
		content.PubkeysByDepth = result.ToDepthArrays()
		content.TotalPubkeys = result.GetTotalPubkeys()
	} else {
		// For event-based queries, use events_by_depth
		content.EventsByDepth = result.ToEventDepthArrays()
		content.TotalEvents = result.GetTotalEvents()
	}

	// Add ref summaries if present
	if inboundRefs := result.GetInboundRefs(); len(inboundRefs) > 0 {
		content.InboundRefs = buildRefSummaries(inboundRefs)
	}
	if outboundRefs := result.GetOutboundRefs(); len(outboundRefs) > 0 {
		content.OutboundRefs = buildRefSummaries(outboundRefs)
	}

	contentBytes, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	// Build tags
	tags := tag.NewS(
		tag.NewFromAny("method", q.Method),
		tag.NewFromAny("seed", q.Seed),
		tag.NewFromAny("depth", strconv.Itoa(q.Depth)),
	)

	// Create event
	ev := &event.E{
		Kind:      responseKind,
		CreatedAt: time.Now().Unix(),
		Tags:      tags,
		Content:   contentBytes,
	}

	// Sign with relay identity
	if err = ev.Sign(e.relaySigner); chk.E(err) {
		return nil, err
	}

	return ev, nil
}

// ResponseContent is the JSON structure for graph query responses.
type ResponseContent struct {
	// PubkeysByDepth contains arrays of pubkeys at each depth (1-indexed)
	// Each pubkey appears ONLY at the depth where it was first discovered.
	PubkeysByDepth [][]string `json:"pubkeys_by_depth,omitempty"`

	// EventsByDepth contains arrays of event IDs at each depth (1-indexed)
	EventsByDepth [][]string `json:"events_by_depth,omitempty"`

	// TotalPubkeys is the total count of unique pubkeys discovered
	TotalPubkeys int `json:"total_pubkeys,omitempty"`

	// TotalEvents is the total count of unique events discovered
	TotalEvents int `json:"total_events,omitempty"`

	// InboundRefs contains aggregated inbound references (events referencing discovered items)
	// Structure: array of {kind, target, count, refs[]}
	InboundRefs []RefSummary `json:"inbound_refs,omitempty"`

	// OutboundRefs contains aggregated outbound references (events referenced by discovered items)
	// Structure: array of {kind, source, count, refs[]}
	OutboundRefs []RefSummary `json:"outbound_refs,omitempty"`
}

// RefSummary represents aggregated reference data for a single target/source.
type RefSummary struct {
	// Kind is the kind of the referencing/referenced events
	Kind uint16 `json:"kind"`

	// Target is the event ID being referenced (for inbound) or referencing (for outbound)
	Target string `json:"target"`

	// Count is the number of references
	Count int `json:"count"`

	// Refs is the list of event IDs (optional, may be omitted for large sets)
	Refs []string `json:"refs,omitempty"`
}

// buildRefSummaries converts the ref map structure to a sorted array of RefSummary.
// Results are sorted by count descending (most referenced first).
func buildRefSummaries(refs map[uint16]map[string][]string) []RefSummary {
	var summaries []RefSummary

	for kind, targets := range refs {
		for targetID, refIDs := range targets {
			summaries = append(summaries, RefSummary{
				Kind:   kind,
				Target: targetID,
				Count:  len(refIDs),
				Refs:   refIDs,
			})
		}
	}

	// Sort by count descending
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Count != summaries[j].Count {
			return summaries[i].Count > summaries[j].Count
		}
		// Secondary sort by kind for stability
		return summaries[i].Kind < summaries[j].Kind
	})

	return summaries
}
