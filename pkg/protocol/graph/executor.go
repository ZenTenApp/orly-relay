//go:build !(js && wasm)

// Package graph implements NIP-XX Graph Query protocol support.
// This file contains the executor that runs graph traversal queries.
package graph

import (
	"encoding/json"
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
}
