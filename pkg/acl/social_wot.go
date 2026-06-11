//go:build !(js && wasm)

package acl

import (
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// WoTDepthMap maintains a mapping from pubkey serial to WoT depth (1-N).
// It is computed via BFS from anchor pubkeys using the ppg/gpp materialized
// indexes and is shared between the social ACL driver and the GC.
//
// All state is owned by a single actor goroutine.
type WoTDepthMap struct {
	anchors  [][]byte // anchor pubkeys (32-byte raw)
	maxDepth int

	recomputeCh    chan wotRecomputeReq
	getDepthCh     chan wotGetDepthReq
	getDepthHexCh  chan wotGetDepthHexReq
	sizeCh         chan wotSizeReq
	stop           chan struct{}
	done           chan struct{}
}

type wotRecomputeReq struct {
	db   *database.D
	resp chan error
}

type wotGetDepthReq struct {
	pubkeySerial uint64
	resp         chan wotGetDepthResp
}

type wotGetDepthResp struct {
	depth int
	ok    bool
}

type wotGetDepthHexReq struct {
	pubkeyHex string
	resp      chan int
}

type wotSizeReq struct {
	resp chan int
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
	w := &WoTDepthMap{
		anchors:       anchors,
		maxDepth:      maxDepth,
		recomputeCh:   make(chan wotRecomputeReq),
		getDepthCh:    make(chan wotGetDepthReq),
		getDepthHexCh: make(chan wotGetDepthHexReq),
		sizeCh:        make(chan wotSizeReq),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
	}
	go w.actor()
	return w
}

func (w *WoTDepthMap) actor() {
	defer close(w.done)

	depths := make(map[uint64]int)
	hexIndex := make(map[string]int)

	for {
		select {
		case <-w.stop:
			return

		case req := <-w.recomputeCh:
			newDepths := make(map[uint64]int)
			newHex := make(map[string]int)

			for _, anchor := range w.anchors {
				result, err := req.db.TraversePubkeyPubkey(anchor, w.maxDepth, "out")
				if err != nil {
					log.W.F("WoTDepthMap: BFS failed for anchor %s: %v", hex.Enc(anchor), err)
					continue
				}

				for depth, pubkeys := range result.PubkeysByDepth {
					for _, pkHex := range pubkeys {
						pkBytes, err := hex.Dec(pkHex)
						if err != nil || len(pkBytes) != 32 {
							continue
						}
						serial, err := req.db.GetPubkeySerial(pkBytes)
						if err != nil {
							continue
						}
						serialVal := serial.Get()

						if existing, ok := newDepths[serialVal]; !ok || depth < existing {
							newDepths[serialVal] = depth
							newHex[pkHex] = depth
						}
					}
				}
			}

			for _, anchor := range w.anchors {
				serial, err := req.db.GetPubkeySerial(anchor)
				if err != nil {
					continue
				}
				newDepths[serial.Get()] = 0
				newHex[hex.Enc(anchor)] = 0
			}

			depths = newDepths
			hexIndex = newHex

			log.I.F("WoTDepthMap: recomputed with %d pubkeys across %d anchors (max depth %d)",
				len(newDepths), len(w.anchors), w.maxDepth)

			req.resp <- nil

		case req := <-w.getDepthCh:
			d, ok := depths[req.pubkeySerial]
			req.resp <- wotGetDepthResp{depth: d, ok: ok}

		case req := <-w.getDepthHexCh:
			d, ok := hexIndex[req.pubkeyHex]
			if !ok {
				req.resp <- -1
			} else {
				req.resp <- d
			}

		case req := <-w.sizeCh:
			req.resp <- len(depths)
		}
	}
}

// Recompute runs BFS from each anchor pubkey using the existing
// TraversePubkeyPubkey with direction="out" and repopulates the depth map.
// For multiple anchors, the minimum depth across all anchors is kept.
func (w *WoTDepthMap) Recompute(db *database.D) error {
	resp := make(chan error, 1)
	w.recomputeCh <- wotRecomputeReq{db: db, resp: resp}
	return <-resp
}

// GetDepth returns the WoT depth for a pubkey serial.
// Returns 0 if unknown (not in WoT) - callers must distinguish
// "depth 0 = anchor" from "not found" using the ok return value.
func (w *WoTDepthMap) GetDepth(pubkeySerial uint64) (depth int, ok bool) {
	resp := make(chan wotGetDepthResp, 1)
	w.getDepthCh <- wotGetDepthReq{pubkeySerial: pubkeySerial, resp: resp}
	r := <-resp
	return r.depth, r.ok
}

// GetDepthByHex returns the WoT depth for a hex-encoded pubkey.
// Returns -1 if not in WoT.
func (w *WoTDepthMap) GetDepthByHex(pubkeyHex string) int {
	resp := make(chan int, 1)
	w.getDepthHexCh <- wotGetDepthHexReq{pubkeyHex: pubkeyHex, resp: resp}
	return <-resp
}

// Size returns the number of pubkeys in the depth map.
func (w *WoTDepthMap) Size() int {
	resp := make(chan int, 1)
	w.sizeCh <- wotSizeReq{resp: resp}
	return <-resp
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

// Stop shuts down the actor goroutine and waits for it to exit.
func (w *WoTDepthMap) Stop() {
	close(w.stop)
	<-w.done
}
