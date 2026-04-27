//go:build !(js && wasm)

package acl

import (
	"sync"

	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// WoTDepthMap maintains a mapping from pubkey serial to WoT depth (1-N).
// It is computed via BFS from anchor pubkeys using the ppg/gpp materialized
// indexes and is shared between the social ACL driver and the GC.
//
// Thread-safe: reads are concurrent, writes (Recompute) take an exclusive lock.
type WoTDepthMap struct {
	mu       sync.RWMutex
	depths   map[uint64]int // pubkey serial → WoT depth (1..maxDepth)
	hexIndex map[string]int // pubkey hex → WoT depth (for ACL layer)
	anchors  [][]byte       // anchor pubkeys (32-byte raw)
	maxDepth int
}

// NewWoTDepthMap creates a new WoT depth map.
// anchors: 32-byte raw pubkeys of relay owners/admins.
// maxDepth: maximum BFS traversal depth (typically 3).
func NewWoTDepthMap(anchors [][]byte, maxDepth int) *WoTDepthMap {
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if maxDepth > 16 {
		maxDepth = 16
	}
	return &WoTDepthMap{
		depths:   make(map[uint64]int),
		hexIndex: make(map[string]int),
		anchors:  anchors,
		maxDepth: maxDepth,
	}
}

// Recompute runs BFS from each anchor pubkey using the existing
// TraversePubkeyPubkey with direction="out" and repopulates the depth map.
// For multiple anchors, the minimum depth across all anchors is kept.
func (w *WoTDepthMap) Recompute(db *database.D) error {
	newDepths := make(map[uint64]int)
	newHex := make(map[string]int)

	for _, anchor := range w.anchors {
		result, err := db.TraversePubkeyPubkey(anchor, w.maxDepth, "out")
		if err != nil {
			log.W.F("WoTDepthMap: BFS failed for anchor %s: %v", hex.Enc(anchor), err)
			continue
		}

		// TraversePubkeyPubkey returns GraphResult with PubkeysByDepth map[int][]string
		for depth, pubkeys := range result.PubkeysByDepth {
			for _, pkHex := range pubkeys {
				// Convert hex to raw bytes then to serial
				pkBytes, err := hex.Dec(pkHex)
				if err != nil || len(pkBytes) != 32 {
					continue
				}
				serial, err := db.GetPubkeySerial(pkBytes)
				if err != nil {
					continue
				}
				serialVal := serial.Get()

				// Keep minimum depth across all anchors
				if existing, ok := newDepths[serialVal]; !ok || depth < existing {
					newDepths[serialVal] = depth
					newHex[pkHex] = depth
				}
			}
		}
	}

	// Also add anchors themselves at depth 0 (owners/admins)
	for _, anchor := range w.anchors {
		serial, err := db.GetPubkeySerial(anchor)
		if err != nil {
			continue
		}
		newDepths[serial.Get()] = 0
		newHex[hex.Enc(anchor)] = 0
	}

	w.mu.Lock()
	w.depths = newDepths
	w.hexIndex = newHex
	w.mu.Unlock()

	log.I.F("WoTDepthMap: recomputed with %d pubkeys across %d anchors (max depth %d)",
		len(newDepths), len(w.anchors), w.maxDepth)

	return nil
}

// GetDepth returns the WoT depth for a pubkey serial.
// Returns 0 if unknown (not in WoT) -- callers must distinguish
// "depth 0 = anchor" from "not found" using the ok return value.
func (w *WoTDepthMap) GetDepth(pubkeySerial uint64) (depth int, ok bool) {
	w.mu.RLock()
	depth, ok = w.depths[pubkeySerial]
	w.mu.RUnlock()
	return
}

// GetDepthByHex returns the WoT depth for a hex-encoded pubkey.
// Returns -1 if not in WoT.
func (w *WoTDepthMap) GetDepthByHex(pubkeyHex string) int {
	w.mu.RLock()
	depth, ok := w.hexIndex[pubkeyHex]
	w.mu.RUnlock()
	if !ok {
		return -1
	}
	return depth
}

// Size returns the number of pubkeys in the depth map.
func (w *WoTDepthMap) Size() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.depths)
}

// GetDepthForGC implements the WoTProvider interface used by the GC.
// Returns the depth bonus tier: 1, 2, 3 for WoT members, 0 for outsiders.
// Anchors (depth 0) are treated as tier 1.
func (w *WoTDepthMap) GetDepthForGC(pubkeySerial uint64) int {
	depth, ok := w.GetDepth(pubkeySerial)
	if !ok {
		return 0 // outsider
	}
	if depth == 0 {
		return 1 // anchor = same as depth 1
	}
	return depth
}
