package find

import (
	"fmt"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

// TrustGraph manages trust relationships between registry services
type TrustGraph struct {
	entries      map[string][]TrustEntry // pubkey -> trust entries
	selfPubkey   []byte                  // This registry service's pubkey
	lastUpdated  map[string]time.Time    // pubkey -> last update time
	decayFactors map[int]float64         // hop distance -> decay factor

	// Actor channels
	addGraphCh       chan addGraphReq
	addEntryCh       chan addEntryReq
	getTrustLevelCh  chan getTrustLevelReq
	getDirectCh      chan getDirectReq
	removeEntryCh    chan removeEntryReq
	updateEntryCh    chan updateEntryReq
	getAllEntriesCh   chan getAllEntriesReq
	getTrustedCh     chan getTrustedReq
	getInheritedCh   chan getInheritedReq
	exportCh         chan exportReq
	calcMetricsCh    chan calcMetricsReq
	stop             chan struct{}
	done             chan struct{}
}

type addGraphReq struct {
	graph *TrustGraph
	resp  chan error
}

type addEntryReq struct {
	entry TrustEntry
	resp  chan error
}

type getTrustLevelReq struct {
	pubkey []byte
	resp   chan float64
}

type getDirectReq struct {
	resp chan []TrustEntry
}

type removeEntryReq struct {
	pubkey string
}

type updateEntryReq struct {
	pubkey   string
	newScore float64
	resp     chan error
}

type getAllEntriesReq struct {
	resp chan map[string][]TrustEntry
}

type getTrustedReq struct {
	resp chan []string
}

type getInheritedReq struct {
	fromPubkey string
	toPubkey   string
	resp       chan getInheritedResp
}

type getInheritedResp struct {
	trust float64
	path  []string
}

type exportReq struct {
	resp chan *TrustGraphEvent
}

type calcMetricsReq struct {
	resp chan *TrustMetrics
}

// NewTrustGraph creates a new trust graph
func NewTrustGraph(selfPubkey []byte) *TrustGraph {
	tg := &TrustGraph{
		entries:     make(map[string][]TrustEntry),
		selfPubkey:  selfPubkey,
		lastUpdated: make(map[string]time.Time),
		decayFactors: map[int]float64{
			0: 1.0,  // Direct trust (0-hop)
			1: 0.8,  // 1-hop trust
			2: 0.6,  // 2-hop trust
			3: 0.4,  // 3-hop trust
			4: 0.0,  // 4+ hops not counted
		},
		addGraphCh:      make(chan addGraphReq),
		addEntryCh:      make(chan addEntryReq),
		getTrustLevelCh: make(chan getTrustLevelReq),
		getDirectCh:     make(chan getDirectReq),
		removeEntryCh:   make(chan removeEntryReq, 16),
		updateEntryCh:   make(chan updateEntryReq),
		getAllEntriesCh:  make(chan getAllEntriesReq),
		getTrustedCh:    make(chan getTrustedReq),
		getInheritedCh:  make(chan getInheritedReq),
		exportCh:        make(chan exportReq),
		calcMetricsCh:   make(chan calcMetricsReq),
		stop:            make(chan struct{}),
		done:            make(chan struct{}),
	}
	go tg.actor()
	return tg
}

// Stop shuts down the trust graph actor.
func (tg *TrustGraph) Stop() {
	close(tg.stop)
	<-tg.done
}

func (tg *TrustGraph) actor() {
	defer close(tg.done)
	for {
		select {
		case <-tg.stop:
			return
		case req := <-tg.addGraphCh:
			req.resp <- tg.doAddTrustGraph(req.graph)
		case req := <-tg.addEntryCh:
			req.resp <- tg.doAddEntry(req.entry)
		case req := <-tg.getTrustLevelCh:
			req.resp <- tg.doGetTrustLevel(req.pubkey)
		case req := <-tg.getDirectCh:
			req.resp <- tg.doGetDirectTrust()
		case req := <-tg.removeEntryCh:
			tg.doRemoveEntry(req.pubkey)
		case req := <-tg.updateEntryCh:
			req.resp <- tg.doUpdateEntry(req.pubkey, req.newScore)
		case req := <-tg.getAllEntriesCh:
			req.resp <- tg.doGetAllEntries()
		case req := <-tg.getTrustedCh:
			req.resp <- tg.doGetTrustedServices()
		case req := <-tg.getInheritedCh:
			trust, path := tg.doGetInheritedTrust(req.fromPubkey, req.toPubkey)
			req.resp <- getInheritedResp{trust: trust, path: path}
		case req := <-tg.exportCh:
			req.resp <- tg.doExportTrustGraph()
		case req := <-tg.calcMetricsCh:
			req.resp <- tg.doCalculateTrustMetrics()
		}
	}
}

// AddTrustGraph adds a trust graph from another registry service
func (tg *TrustGraph) AddTrustGraph(graph *TrustGraph) error {
	resp := make(chan error, 1)
	tg.addGraphCh <- addGraphReq{graph: graph, resp: resp}
	return <-resp
}

func (tg *TrustGraph) doAddTrustGraph(graph *TrustGraph) error {
	sourcePubkey := hex.Enc(graph.selfPubkey)

	// Copy entries from the source graph
	for pubkey, entries := range graph.entries {
		// Store the trust entries
		tg.entries[pubkey] = make([]TrustEntry, len(entries))
		copy(tg.entries[pubkey], entries)
	}

	// Update last modified time
	tg.lastUpdated[sourcePubkey] = time.Now()

	return nil
}

// AddEntry adds a trust entry to the graph
func (tg *TrustGraph) AddEntry(entry TrustEntry) error {
	if err := ValidateTrustScore(entry.TrustScore); err != nil {
		return err
	}
	resp := make(chan error, 1)
	tg.addEntryCh <- addEntryReq{entry: entry, resp: resp}
	return <-resp
}

func (tg *TrustGraph) doAddEntry(entry TrustEntry) error {
	selfPubkey := hex.Enc(tg.selfPubkey)
	tg.entries[selfPubkey] = append(tg.entries[selfPubkey], entry)
	tg.lastUpdated[selfPubkey] = time.Now()
	return nil
}

// GetTrustLevel returns the trust level for a given pubkey (0.0 to 1.0)
// This computes both direct trust and inherited trust through the web of trust
func (tg *TrustGraph) GetTrustLevel(pubkey []byte) float64 {
	resp := make(chan float64, 1)
	tg.getTrustLevelCh <- getTrustLevelReq{pubkey: pubkey, resp: resp}
	return <-resp
}

func (tg *TrustGraph) doGetTrustLevel(pubkey []byte) float64 {
	pubkeyStr := hex.Enc(pubkey)
	selfPubkeyStr := hex.Enc(tg.selfPubkey)

	// Check for direct trust first (0-hop)
	if entries, ok := tg.entries[selfPubkeyStr]; ok {
		for _, entry := range entries {
			if entry.Pubkey == pubkeyStr {
				return entry.TrustScore
			}
		}
	}

	// Compute inherited trust through web of trust
	// Use breadth-first search to find shortest trust path
	maxHops := 3 // Maximum path length (configurable)
	visited := make(map[string]bool)
	queue := []trustPath{{pubkey: selfPubkeyStr, trust: 1.0, hops: 0}}
	visited[selfPubkeyStr] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Stop if we've exceeded max hops
		if current.hops > maxHops {
			continue
		}

		// Check if we found the target
		if current.pubkey == pubkeyStr {
			// Apply hop-based decay
			decayFactor := tg.decayFactors[current.hops]
			return current.trust * decayFactor
		}

		// Expand to neighbors
		if entries, ok := tg.entries[current.pubkey]; ok {
			for _, entry := range entries {
				if !visited[entry.Pubkey] {
					visited[entry.Pubkey] = true
					queue = append(queue, trustPath{
						pubkey: entry.Pubkey,
						trust:  current.trust * entry.TrustScore,
						hops:   current.hops + 1,
					})
				}
			}
		}
	}

	// No trust path found - return default minimal trust for unknown services
	return 0.0
}

// trustPath represents a path in the trust graph during BFS
type trustPath struct {
	pubkey string
	trust  float64
	hops   int
}

// GetDirectTrust returns direct trust relationships (0-hop only)
func (tg *TrustGraph) GetDirectTrust() []TrustEntry {
	resp := make(chan []TrustEntry, 1)
	tg.getDirectCh <- getDirectReq{resp: resp}
	return <-resp
}

func (tg *TrustGraph) doGetDirectTrust() []TrustEntry {
	selfPubkeyStr := hex.Enc(tg.selfPubkey)
	if entries, ok := tg.entries[selfPubkeyStr]; ok {
		result := make([]TrustEntry, len(entries))
		copy(result, entries)
		return result
	}
	return []TrustEntry{}
}

// RemoveEntry removes a trust entry for a given pubkey
func (tg *TrustGraph) RemoveEntry(pubkey string) {
	tg.removeEntryCh <- removeEntryReq{pubkey: pubkey}
}

func (tg *TrustGraph) doRemoveEntry(pubkey string) {
	selfPubkeyStr := hex.Enc(tg.selfPubkey)
	if entries, ok := tg.entries[selfPubkeyStr]; ok {
		filtered := make([]TrustEntry, 0, len(entries))
		for _, entry := range entries {
			if entry.Pubkey != pubkey {
				filtered = append(filtered, entry)
			}
		}
		tg.entries[selfPubkeyStr] = filtered
		tg.lastUpdated[selfPubkeyStr] = time.Now()
	}
}

// UpdateEntry updates an existing trust entry
func (tg *TrustGraph) UpdateEntry(pubkey string, newScore float64) error {
	if err := ValidateTrustScore(newScore); err != nil {
		return err
	}
	resp := make(chan error, 1)
	tg.updateEntryCh <- updateEntryReq{pubkey: pubkey, newScore: newScore, resp: resp}
	return <-resp
}

func (tg *TrustGraph) doUpdateEntry(pubkey string, newScore float64) error {
	selfPubkeyStr := hex.Enc(tg.selfPubkey)
	if entries, ok := tg.entries[selfPubkeyStr]; ok {
		for i, entry := range entries {
			if entry.Pubkey == pubkey {
				tg.entries[selfPubkeyStr][i].TrustScore = newScore
				tg.lastUpdated[selfPubkeyStr] = time.Now()
				return nil
			}
		}
	}

	return fmt.Errorf("trust entry not found for pubkey: %s", pubkey)
}

// GetAllEntries returns all trust entries in the graph (for debugging/export)
func (tg *TrustGraph) GetAllEntries() map[string][]TrustEntry {
	resp := make(chan map[string][]TrustEntry, 1)
	tg.getAllEntriesCh <- getAllEntriesReq{resp: resp}
	return <-resp
}

func (tg *TrustGraph) doGetAllEntries() map[string][]TrustEntry {
	result := make(map[string][]TrustEntry)
	for pubkey, entries := range tg.entries {
		result[pubkey] = make([]TrustEntry, len(entries))
		copy(result[pubkey], entries)
	}
	return result
}

// GetTrustedServices returns a list of all directly trusted service pubkeys
func (tg *TrustGraph) GetTrustedServices() []string {
	resp := make(chan []string, 1)
	tg.getTrustedCh <- getTrustedReq{resp: resp}
	return <-resp
}

func (tg *TrustGraph) doGetTrustedServices() []string {
	selfPubkeyStr := hex.Enc(tg.selfPubkey)
	if entries, ok := tg.entries[selfPubkeyStr]; ok {
		pubkeys := make([]string, 0, len(entries))
		for _, entry := range entries {
			pubkeys = append(pubkeys, entry.Pubkey)
		}
		return pubkeys
	}
	return []string{}
}

// GetInheritedTrust computes inherited trust from one service to another
// This is useful for debugging and understanding trust propagation
func (tg *TrustGraph) GetInheritedTrust(fromPubkey, toPubkey string) (float64, []string) {
	resp := make(chan getInheritedResp, 1)
	tg.getInheritedCh <- getInheritedReq{fromPubkey: fromPubkey, toPubkey: toPubkey, resp: resp}
	r := <-resp
	return r.trust, r.path
}

func (tg *TrustGraph) doGetInheritedTrust(fromPubkey, toPubkey string) (float64, []string) {
	// BFS to find shortest path and trust level
	type pathNode struct {
		pubkey string
		trust  float64
		hops   int
		path   []string
	}

	visited := make(map[string]bool)
	queue := []pathNode{{pubkey: fromPubkey, trust: 1.0, hops: 0, path: []string{fromPubkey}}}
	visited[fromPubkey] = true

	maxHops := 3

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.hops > maxHops {
			continue
		}

		// Found target
		if current.pubkey == toPubkey {
			decayFactor := tg.decayFactors[current.hops]
			return current.trust * decayFactor, current.path
		}

		// Expand neighbors
		if entries, ok := tg.entries[current.pubkey]; ok {
			for _, entry := range entries {
				if !visited[entry.Pubkey] {
					visited[entry.Pubkey] = true
					newPath := make([]string, len(current.path))
					copy(newPath, current.path)
					newPath = append(newPath, entry.Pubkey)

					queue = append(queue, pathNode{
						pubkey: entry.Pubkey,
						trust:  current.trust * entry.TrustScore,
						hops:   current.hops + 1,
						path:   newPath,
					})
				}
			}
		}
	}

	// No path found
	return 0.0, nil
}

// ExportTrustGraph exports the trust graph for this service as a TrustGraphEvent
func (tg *TrustGraph) ExportTrustGraph() *TrustGraphEvent {
	resp := make(chan *TrustGraphEvent, 1)
	tg.exportCh <- exportReq{resp: resp}
	return <-resp
}

func (tg *TrustGraph) doExportTrustGraph() *TrustGraphEvent {
	selfPubkeyStr := hex.Enc(tg.selfPubkey)
	entries := tg.entries[selfPubkeyStr]

	exported := &TrustGraphEvent{
		Event:      nil, // TODO: Create event
		Entries:    make([]TrustEntry, len(entries)),
		Expiration: time.Now().Add(TrustGraphExpiry),
	}

	copy(exported.Entries, entries)

	return exported
}

// CalculateTrustMetrics computes metrics about the trust graph
func (tg *TrustGraph) CalculateTrustMetrics() *TrustMetrics {
	resp := make(chan *TrustMetrics, 1)
	tg.calcMetricsCh <- calcMetricsReq{resp: resp}
	return <-resp
}

func (tg *TrustGraph) doCalculateTrustMetrics() *TrustMetrics {
	metrics := &TrustMetrics{
		TotalServices:  len(tg.entries),
		DirectTrust:    0,
		IndirectTrust:  0,
		AverageTrust:   0.0,
		TrustDistribution: make(map[string]int),
	}

	selfPubkeyStr := hex.Enc(tg.selfPubkey)
	if entries, ok := tg.entries[selfPubkeyStr]; ok {
		metrics.DirectTrust = len(entries)

		var trustSum float64
		for _, entry := range entries {
			trustSum += entry.TrustScore

			// Categorize trust level
			if entry.TrustScore >= 0.8 {
				metrics.TrustDistribution["high"]++
			} else if entry.TrustScore >= 0.5 {
				metrics.TrustDistribution["medium"]++
			} else if entry.TrustScore >= 0.2 {
				metrics.TrustDistribution["low"]++
			} else {
				metrics.TrustDistribution["minimal"]++
			}
		}

		if len(entries) > 0 {
			metrics.AverageTrust = trustSum / float64(len(entries))
		}
	}

	// Calculate indirect trust (services reachable via multi-hop)
	// This is approximate - counts unique services reachable within 3 hops
	reachable := make(map[string]bool)
	queue := []string{selfPubkeyStr}
	visited := make(map[string]int) // pubkey -> hop count
	visited[selfPubkeyStr] = 0

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		currentHops := visited[current]
		if currentHops >= 3 {
			continue
		}

		if entries, ok := tg.entries[current]; ok {
			for _, entry := range entries {
				if _, seen := visited[entry.Pubkey]; !seen {
					visited[entry.Pubkey] = currentHops + 1
					queue = append(queue, entry.Pubkey)
					reachable[entry.Pubkey] = true
				}
			}
		}
	}

	metrics.IndirectTrust = len(reachable) - metrics.DirectTrust
	if metrics.IndirectTrust < 0 {
		metrics.IndirectTrust = 0
	}

	return metrics
}

// TrustMetrics holds metrics about the trust graph
type TrustMetrics struct {
	TotalServices     int
	DirectTrust       int
	IndirectTrust     int
	AverageTrust      float64
	TrustDistribution map[string]int // high/medium/low/minimal counts
}
