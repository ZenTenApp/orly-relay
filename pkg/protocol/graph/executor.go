//go:build !(js && wasm)

// Package graph implements NIP-XX Graph Query protocol support.
// This file contains the executor that runs graph traversal queries.
package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

// Response kind for graph queries (ephemeral, relay-signed)
const KindGraphResult = 39000

// GraphResultI is the interface that database.GraphResult implements.
type GraphResultI interface {
	ToDepthArrays() [][]string
	ToEventDepthArrays() [][]string
	GetAllPubkeys() []string
	GetAllEvents() []string
	GetPubkeysByDepth() map[int][]string
	GetEventsByDepth() map[int][]string
	GetTotalPubkeys() int
	GetTotalEvents() int
	GetInboundRefs() map[uint16]map[string][]string
	GetOutboundRefs() map[uint16]map[string][]string
}

// GraphDatabase defines the interface for graph traversal operations.
// Each method corresponds to one cell in the edge × direction matrix.
type GraphDatabase interface {
	// Pubkey↔Pubkey (pp) — noun-noun edges via ppg/gpp index
	TraversePubkeyPubkey(seedPubkey []byte, maxDepth int, direction string) (GraphResultI, error)

	// Pubkey↔Event (pe) — adverb edges via peg/epg index
	TraversePubkeyEvent(seedPubkey []byte, maxDepth int, direction string) (GraphResultI, error)

	// Event↔Event (ee) — adjective edges via eeg/gee index
	TraverseEventEvent(seedEventID []byte, maxDepth int, direction string) (GraphResultI, error)

	// Baseline (no graph indexes) — for benchmark comparison.
	// Same semantics as TraversePubkeyPubkey but uses multi-hop NIP-01 queries.
	TraversePubkeyPubkeyBaseline(seedPubkey []byte, maxDepth int, direction string) (GraphResultI, error)
}

// Executor handles graph query execution and response generation.
type Executor struct {
	db          GraphDatabase
	relaySigner signer.I
	relayPubkey []byte
	baseline    bool // when true, use baseline (no ppg/gpp) for pp queries
}

// NewExecutor creates a new graph query executor.
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

// SetBaseline enables or disables baseline mode for pp queries.
// In baseline mode, pubkey↔pubkey traversal uses multi-hop NIP-01 queries
// instead of the ppg/gpp materialized index, for benchmark comparison.
func (e *Executor) SetBaseline(enabled bool) {
	e.baseline = enabled
}

// Execute runs a graph query and returns a relay-signed event with results.
func (e *Executor) Execute(q *Query) (*event.E, error) {
	var result GraphResultI
	var err error

	start := time.Now()

	switch q.Edge {
	case "pp":
		// Pubkey↔pubkey: decode seed as pubkey hex
		seedBytes, decErr := decodeHex(q.Pubkey)
		if decErr != nil {
			return nil, decErr
		}
		if e.baseline {
			result, err = e.db.TraversePubkeyPubkeyBaseline(seedBytes, q.Depth, q.Direction)
		} else {
			result, err = e.db.TraversePubkeyPubkey(seedBytes, q.Depth, q.Direction)
		}

	case "pe":
		// Pubkey↔event: decode seed as pubkey hex
		seedBytes, decErr := decodeHex(q.Pubkey)
		if decErr != nil {
			return nil, decErr
		}
		result, err = e.db.TraversePubkeyEvent(seedBytes, q.Depth, q.Direction)

	case "ee":
		// Event↔event: seed is still a pubkey in our spec, but the traversal
		// seeds from events authored by that pubkey. For direct event→event
		// traversal, the adapter can resolve this.
		seedBytes, decErr := decodeHex(q.Pubkey)
		if decErr != nil {
			return nil, decErr
		}
		result, err = e.db.TraverseEventEvent(seedBytes, q.Depth, q.Direction)

	default:
		return nil, ErrInvalidEdge
	}

	if err != nil {
		return nil, err
	}

	elapsed := time.Since(start)
	log.D.F("graph query: edge=%s dir=%s depth=%d elapsed=%s pubkeys=%d events=%d baseline=%v",
		q.Edge, q.Direction, q.Depth, elapsed, result.GetTotalPubkeys(), result.GetTotalEvents(), e.baseline)

	return e.generateResponse(q, result, elapsed)
}

// generateResponse creates a relay-signed event containing the query results.
func (e *Executor) generateResponse(q *Query, result GraphResultI, elapsed time.Duration) (*event.E, error) {
	var content ResponseContent
	content.Elapsed = elapsed.String()

	if q.IsPubkeyPubkey() || (q.IsPubkeyEvent() && q.IsOutbound()) {
		content.PubkeysByDepth = result.ToDepthArrays()
		content.TotalPubkeys = result.GetTotalPubkeys()
	}
	if q.IsEventEvent() || (q.IsPubkeyEvent() && q.IsInbound()) {
		content.EventsByDepth = result.ToEventDepthArrays()
		content.TotalEvents = result.GetTotalEvents()
	}

	contentBytes, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}

	tags := tag.NewS(
		tag.NewFromAny("edge", q.Edge),
		tag.NewFromAny("direction", q.Direction),
		tag.NewFromAny("seed", q.Pubkey),
		tag.NewFromAny("depth", strconv.Itoa(q.Depth)),
		tag.NewFromAny("elapsed", elapsed.String()),
	)
	if e.baseline {
		tags.Append(tag.NewFromAny("baseline", "true"))
	}

	ev := &event.E{
		Kind:      KindGraphResult,
		CreatedAt: time.Now().Unix(),
		Tags:      tags,
		Content:   contentBytes,
	}

	if err = ev.Sign(e.relaySigner); chk.E(err) {
		return nil, err
	}

	return ev, nil
}

// ResponseContent is the JSON structure for graph query responses.
type ResponseContent struct {
	PubkeysByDepth [][]string `json:"pubkeys_by_depth,omitempty"`
	EventsByDepth  [][]string `json:"events_by_depth,omitempty"`
	TotalPubkeys   int        `json:"total_pubkeys,omitempty"`
	TotalEvents    int        `json:"total_events,omitempty"`
	Elapsed        string     `json:"elapsed,omitempty"`
}

// RefSummary represents aggregated reference data.
type RefSummary struct {
	Kind   uint16   `json:"kind"`
	Target string   `json:"target"`
	Count  int      `json:"count"`
	Refs   []string `json:"refs,omitempty"`
}

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
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Count != summaries[j].Count {
			return summaries[i].Count > summaries[j].Count
		}
		return summaries[i].Kind < summaries[j].Kind
	})
	return summaries
}

// decodeHex decodes a hex string to bytes, with validation.
func decodeHex(hexStr string) ([]byte, error) {
	if len(hexStr) != 64 {
		return nil, fmt.Errorf("expected 64-char hex, got %d chars", len(hexStr))
	}
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hi := unhex(hexStr[i*2])
		lo := unhex(hexStr[i*2+1])
		if hi == 0xFF || lo == 0xFF {
			return nil, fmt.Errorf("invalid hex char at position %d", i*2)
		}
		b[i] = hi<<4 | lo
	}
	return b, nil
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0xFF
	}
}
