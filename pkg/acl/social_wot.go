//go:build !(js && wasm)

package acl

import (
	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

type wotGetDepthResp struct {
	depth int
	ok    bool
}

// WoTDepthMap maintains a mapping from pubkey serial to WoT depth (1-N).
// It is computed via BFS from anchor pubkeys using the ppg/gpp materialized
// indexes and is shared between the social ACL driver and the GC.
//
// All state is owned by a single actor goroutine.
type WoTDepthMap struct {
	anchors  [][]byte // anchor pubkeys (32-byte raw)
	maxDepth int

	recompute    actor.Func[*database.D, error]
	getDepth     actor.Func[uint64, wotGetDepthResp]
	getDepthHex  actor.Func[string, int]
	size         actor.Query[int]
	actor.Lifecycle
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
		anchors:     anchors,
		maxDepth:    maxDepth,
		recompute:   actor.NewFunc[*database.D, error](),
		getDepth:    actor.NewFunc[uint64, wotGetDepthResp](),
		getDepthHex: actor.NewFunc[string, int](),
		size:        actor.NewQuery[int](),
		Lifecycle:   actor.NewLifecycle(),
	}
	actor.Go(w.Lifecycle, w.actorLoop)
	return w
}

func (w *WoTDepthMap) actorLoop() {
	depths := make(map[uint64]int)
	hexIndex := make(map[string]int)

	for {
		select {
		case <-w.Stopping():
			return

		case msg := <-w.recompute.Recv():
			newDepths := make(map[uint64]int)
			newHex := make(map[string]int)

			for _, anchor := range w.anchors {
				result, err := msg.Req.TraversePubkeyPubkey(anchor, w.maxDepth, "out")
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
						serial, err := msg.Req.GetPubkeySerial(pkBytes)
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
				serial, err := msg.Req.GetPubkeySerial(anchor)
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

			msg.Reply(nil)

		case msg := <-w.getDepth.Recv():
			d, ok := depths[msg.Req]
			msg.Reply(wotGetDepthResp{depth: d, ok: ok})

		case msg := <-w.getDepthHex.Recv():
			d, ok := hexIndex[msg.Req]
			if !ok {
				msg.Reply(-1)
			} else {
				msg.Reply(d)
			}

		case msg := <-w.size.Recv():
			msg.Reply(len(depths))
		}
	}
}

// Recompute runs BFS from each anchor pubkey using the existing
// TraversePubkeyPubkey with direction="out" and repopulates the depth map.
// For multiple anchors, the minimum depth across all anchors is kept.
func (w *WoTDepthMap) Recompute(db *database.D) error {
	return w.recompute.Call(db)
}

// GetDepth returns the WoT depth for a pubkey serial.
// Returns 0 if unknown (not in WoT) - callers must distinguish
// "depth 0 = anchor" from "not found" using the ok return value.
func (w *WoTDepthMap) GetDepth(pubkeySerial uint64) (depth int, ok bool) {
	r := w.getDepth.Call(pubkeySerial)
	return r.depth, r.ok
}

// GetDepthByHex returns the WoT depth for a hex-encoded pubkey.
// Returns -1 if not in WoT.
func (w *WoTDepthMap) GetDepthByHex(pubkeyHex string) int {
	return w.getDepthHex.Call(pubkeyHex)
}

// Size returns the number of pubkeys in the depth map.
func (w *WoTDepthMap) Size() int {
	return w.size.Call()
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

// Shutdown stops the actor goroutine and waits for it to exit.
func (w *WoTDepthMap) Shutdown() {
	w.Stop()
}
