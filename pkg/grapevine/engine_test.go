package grapevine

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"lukechampine.com/frand"
)

// mockGraphSource provides a controllable follow graph for testing.
type mockGraphSource struct {
	follows   map[string][]string // source -> targets
	followers map[string][]string // target -> sources (reverse)
}

func newMockGraph() *mockGraphSource {
	return &mockGraphSource{
		follows:   make(map[string][]string),
		followers: make(map[string][]string),
	}
}

func (m *mockGraphSource) addFollow(from, to string) {
	m.follows[from] = append(m.follows[from], to)
	m.followers[to] = append(m.followers[to], from)
}

func (m *mockGraphSource) TraverseFollowsOutbound(seedPubkey []byte, maxDepth int) (
	map[int][]string, map[string]int, error,
) {
	seedHex := bytesToHex(seedPubkey)
	visited := map[string]int{seedHex: 0}
	byDepth := map[int][]string{0: {seedHex}}

	for depth := 1; depth <= maxDepth; depth++ {
		prevDepth := byDepth[depth-1]
		for _, pk := range prevDepth {
			for _, followed := range m.follows[pk] {
				if _, seen := visited[followed]; !seen {
					visited[followed] = depth
					byDepth[depth] = append(byDepth[depth], followed)
				}
			}
		}
		if len(byDepth[depth]) == 0 {
			break
		}
	}
	return byDepth, visited, nil
}

func (m *mockGraphSource) GetFollowerPubkeys(targetHex string) ([]string, error) {
	return m.followers[targetHex], nil
}

func (m *mockGraphSource) GetFollowsPubkeys(sourceHex string) ([]string, error) {
	return m.follows[sourceHex], nil
}

func bytesToHex(b []byte) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexChars[v>>4]
		out[i*2+1] = hexChars[v&0x0f]
	}
	return string(out)
}

// mockStore stores score data in memory.
type mockStore struct {
	sets    map[string][]byte
	entries map[string][]byte
}

func newMockStore() *mockStore {
	return &mockStore{
		sets:    make(map[string][]byte),
		entries: make(map[string][]byte),
	}
}

func (s *mockStore) SaveScoreSet(observerHex string, setData []byte, entries map[string][]byte) error {
	s.sets[observerHex] = setData
	for target, data := range entries {
		s.entries[observerHex+":"+target] = data
	}
	return nil
}

func (s *mockStore) GetScoreSet(observerHex string) ([]byte, error) {
	return s.sets[observerHex], nil
}

func (s *mockStore) GetScore(observerHex, targetHex string) ([]byte, error) {
	return s.entries[observerHex+":"+targetHex], nil
}

// generateDeterministicGraph builds a realistic follow graph using frand with a fixed seed.
// Parameters:
//   - numUsers: total number of pubkeys
//   - avgFollows: average number of follows per user
//   - observerFollows: number of follows for the observer specifically
//
// Returns the graph, all pubkey hexes (index 0 is observer), and the observer hex.
func generateDeterministicGraph(numUsers, avgFollows, observerFollows int) (*mockGraphSource, []string, string) {
	rng := frand.NewCustom(make([]byte, 32), 1024, 12) // fixed zero seed

	graph := newMockGraph()
	pubkeys := make([]string, numUsers)
	for i := 0; i < numUsers; i++ {
		var buf [32]byte
		rng.Read(buf[:])
		pubkeys[i] = bytesToHex(buf[:])
	}
	observerHex := pubkeys[0]

	// Observer follows first N users (deterministic, always the same set)
	for i := 1; i <= observerFollows && i < numUsers; i++ {
		graph.addFollow(observerHex, pubkeys[i])
	}

	// Each other user follows a random subset
	for i := 1; i < numUsers; i++ {
		nFollows := avgFollows/2 + int(rng.Uint64n(uint64(avgFollows)))
		if nFollows > numUsers-1 {
			nFollows = numUsers - 1
		}
		for j := 0; j < nFollows; j++ {
			target := int(rng.Uint64n(uint64(numUsers)))
			if target != i {
				graph.addFollow(pubkeys[i], pubkeys[target])
			}
		}
	}

	return graph, pubkeys, observerHex
}

func TestComputeEmptyGraph(t *testing.T) {
	graph := newMockGraph()
	store := newMockStore()
	engine := NewEngine(graph, store, DefaultConfig())

	rng := frand.NewCustom(make([]byte, 32), 1024, 12)
	var buf [32]byte
	rng.Read(buf[:])
	obs := bytesToHex(buf[:])

	set, err := engine.Compute(obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Observer alone with no follows — hop set has just the observer (depth 0)
	if set.TotalPubkeys > 1 {
		t.Errorf("expected at most 1 pubkey (observer), got %d", set.TotalPubkeys)
	}
}

func TestComputeDeterministicSmall(t *testing.T) {
	// 20 users, observer follows 5, avg 4 follows each
	graph, pubkeys, obs := generateDeterministicGraph(20, 4, 5)
	store := newMockStore()
	cfg := DefaultConfig()
	cfg.MaxDepth = 3
	cfg.Cycles = 5
	engine := NewEngine(graph, store, cfg)

	set, err := engine.Compute(obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should discover users beyond direct follows
	if set.TotalPubkeys < 6 {
		t.Errorf("expected > 5 pubkeys in hop set (observer + 5 follows + their follows), got %d", set.TotalPubkeys)
	}

	// Observer's score should be 1.0
	scores := scoreMap(set)
	if s, ok := scores[obs]; !ok || s.Influence != 1.0 {
		t.Errorf("observer influence should be 1.0")
	}

	// Direct follows should have positive influence
	for i := 1; i <= 5; i++ {
		if s, ok := scores[pubkeys[i]]; !ok || s.Influence <= 0 {
			t.Errorf("direct follow %d should have positive influence", i)
		}
	}

	// Verify determinism: run again, same results
	store2 := newMockStore()
	engine2 := NewEngine(graph, store2, cfg)
	set2, _ := engine2.Compute(obs)
	if len(set.Scores) != len(set2.Scores) {
		t.Errorf("determinism: score count mismatch %d vs %d", len(set.Scores), len(set2.Scores))
	}
}

func TestComputeDeterministicMedium(t *testing.T) {
	// 200 users, observer follows 20, avg 10 follows each
	// This creates a denser graph that exercises the convergence loop more thoroughly
	graph, pubkeys, obs := generateDeterministicGraph(200, 10, 20)
	store := newMockStore()
	cfg := DefaultConfig()
	cfg.MaxDepth = 4
	cfg.Cycles = 8
	engine := NewEngine(graph, store, cfg)

	set, err := engine.Compute(obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scores := scoreMap(set)
	t.Logf("total pubkeys in hop set: %d, scored: %d, compute time: %dms",
		set.TotalPubkeys, len(set.Scores), set.ComputeMs)

	// Observer should always be 1.0
	if s := scores[obs]; s == nil || s.Influence != 1.0 {
		t.Error("observer influence should be 1.0")
	}

	// All direct follows should have positive influence
	for i := 1; i <= 20 && i < len(pubkeys); i++ {
		if s := scores[pubkeys[i]]; s == nil || s.Influence <= 0 {
			t.Errorf("direct follow pubkeys[%d] should have positive influence", i)
		}
	}

	// Depth distribution: should have pubkeys at multiple depths
	depthCounts := make(map[int]int)
	for _, s := range set.Scores {
		depthCounts[s.Depth]++
	}
	if len(depthCounts) < 2 {
		t.Error("expected pubkeys at multiple depths")
	}
	t.Logf("depth distribution: %v", depthCounts)

	// WoT scores should vary (not all zero)
	nonZeroWoT := 0
	for _, s := range set.Scores {
		if s.WoTScore > 0 {
			nonZeroWoT++
		}
	}
	if nonZeroWoT == 0 {
		t.Error("expected some non-zero WoT intersection scores")
	}
	t.Logf("non-zero WoT scores: %d/%d", nonZeroWoT, len(set.Scores))

	// Influence scores should generally decrease with depth
	var depth1Avg, depth2Avg float64
	var d1Count, d2Count int
	for _, s := range set.Scores {
		if s.Depth == 1 {
			depth1Avg += s.Influence
			d1Count++
		} else if s.Depth == 2 {
			depth2Avg += s.Influence
			d2Count++
		}
	}
	if d1Count > 0 && d2Count > 0 {
		depth1Avg /= float64(d1Count)
		depth2Avg /= float64(d2Count)
		if depth1Avg <= depth2Avg {
			t.Errorf("expected depth-1 avg influence (%f) > depth-2 avg (%f)", depth1Avg, depth2Avg)
		}
		t.Logf("avg influence: depth1=%.4f, depth2=%.4f", depth1Avg, depth2Avg)
	}
}

func TestComputeConvergenceStabilizes(t *testing.T) {
	// Run with increasing cycle counts and verify scores stabilize
	graph, _, obs := generateDeterministicGraph(50, 8, 10)

	var prevScores map[string]float64
	for cycles := 1; cycles <= 10; cycles++ {
		store := newMockStore()
		cfg := DefaultConfig()
		cfg.MaxDepth = 3
		cfg.Cycles = cycles
		engine := NewEngine(graph, store, cfg)

		set, err := engine.Compute(obs)
		if err != nil {
			t.Fatalf("cycle %d: unexpected error: %v", cycles, err)
		}

		currentScores := make(map[string]float64)
		for _, s := range set.Scores {
			currentScores[s.PubkeyHex] = s.Influence
		}

		if prevScores != nil && cycles >= 5 {
			maxDelta := 0.0
			for pk, score := range currentScores {
				if prev, ok := prevScores[pk]; ok {
					delta := math.Abs(score - prev)
					if delta > maxDelta {
						maxDelta = delta
					}
				}
			}
			if maxDelta > 0.01 {
				t.Errorf("scores not stabilized by cycle %d, max delta: %f", cycles, maxDelta)
			}
		}
		prevScores = currentScores
	}
}

func TestObserverScoreImmutable(t *testing.T) {
	// Even with many followers pointing back at observer, score stays 1.0
	graph, pubkeys, obs := generateDeterministicGraph(30, 6, 10)
	// Add reverse follows: everyone follows observer
	for i := 1; i < len(pubkeys); i++ {
		graph.addFollow(pubkeys[i], obs)
	}

	store := newMockStore()
	engine := NewEngine(graph, store, DefaultConfig())

	set, err := engine.Compute(obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	scores := scoreMap(set)
	if s := scores[obs]; s == nil || s.Influence != 1.0 {
		inf := 0.0
		if s != nil {
			inf = s.Influence
		}
		t.Errorf("observer influence should be exactly 1.0, got %f", inf)
	}
}

func TestStorageRoundtrip(t *testing.T) {
	graph, _, obs := generateDeterministicGraph(30, 5, 8)
	store := newMockStore()
	engine := NewEngine(graph, store, DefaultConfig())

	computed, err := engine.Compute(obs)
	if err != nil {
		t.Fatalf("compute error: %v", err)
	}

	// Retrieve full set
	retrieved, err := engine.GetScores(obs)
	if err != nil {
		t.Fatalf("GetScores error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected non-nil score set")
	}
	if retrieved.ObserverHex != obs {
		t.Errorf("observer mismatch")
	}
	if len(retrieved.Scores) != len(computed.Scores) {
		t.Errorf("score count mismatch: %d vs %d", len(retrieved.Scores), len(computed.Scores))
	}

	// Retrieve individual entries
	for _, s := range computed.Scores {
		entry, err := engine.GetScore(obs, s.PubkeyHex)
		if err != nil {
			t.Errorf("GetScore error for %s: %v", s.PubkeyHex[:12], err)
			continue
		}
		if entry == nil {
			t.Errorf("missing entry for %s", s.PubkeyHex[:12])
			continue
		}
		if entry.Influence != s.Influence {
			t.Errorf("influence mismatch for %s: %.6f vs %.6f", s.PubkeyHex[:12], entry.Influence, s.Influence)
		}
	}
}

func TestGetScoresUnknownObserver(t *testing.T) {
	store := newMockStore()
	graph := newMockGraph()
	engine := NewEngine(graph, store, DefaultConfig())

	rng := frand.NewCustom([]byte("unknown-observer-seed-padding!!X"), 1024, 12)
	var buf [32]byte
	rng.Read(buf[:])
	unknown := bytesToHex(buf[:])

	set, err := engine.GetScores(unknown)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if set != nil {
		t.Error("expected nil for unknown observer")
	}
}

func TestCertaintyFormula(t *testing.T) {
	rigor := 0.25
	rigority := -math.Log(rigor)

	// input=0 → certainty=0
	c0 := 1 - math.Exp(-0.0*rigority)
	if math.Abs(c0) > 1e-10 {
		t.Errorf("certainty(0) should be 0, got %f", c0)
	}

	// input=high → certainty≈1
	cHigh := 1 - math.Exp(-10.0*rigority)
	if cHigh < 0.99 {
		t.Errorf("certainty(10) should be near 1.0, got %f", cHigh)
	}

	// input=moderate → 0 < certainty < 1
	cMid := 1 - math.Exp(-1.0*rigority)
	if cMid <= 0 || cMid >= 1 {
		t.Errorf("certainty(1) should be in (0,1), got %f", cMid)
	}
}

func TestScoreSetJSON(t *testing.T) {
	rng := frand.NewCustom([]byte("json-test-seed-padding-bytes!!!!"), 1024, 12)
	var buf [32]byte
	rng.Read(buf[:])
	obs := bytesToHex(buf[:])
	rng.Read(buf[:])
	target := bytesToHex(buf[:])

	set := ScoreSet{
		ObserverHex:  obs,
		TotalPubkeys: 1,
		Scores: []ScoreEntry{
			{PubkeyHex: target, Influence: 0.5, Average: 0.8, Certainty: 0.625, Input: 0.1, WoTScore: 3, Depth: 1},
		},
	}

	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ScoreSet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ObserverHex != obs {
		t.Error("observer mismatch after roundtrip")
	}
	if len(decoded.Scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(decoded.Scores))
	}
	if decoded.Scores[0].Influence != 0.5 {
		t.Errorf("influence mismatch: %f", decoded.Scores[0].Influence)
	}
	if decoded.Scores[0].WoTScore != 3 {
		t.Errorf("wot_score mismatch: %d", decoded.Scores[0].WoTScore)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxDepth != 6 {
		t.Errorf("expected MaxDepth=6, got %d", cfg.MaxDepth)
	}
	if cfg.Cycles != 5 {
		t.Errorf("expected Cycles=5, got %d", cfg.Cycles)
	}
	if cfg.AttenuationFactor != 0.8 {
		t.Errorf("expected AttenuationFactor=0.8, got %f", cfg.AttenuationFactor)
	}
	if cfg.Rigor != 0.25 {
		t.Errorf("expected Rigor=0.25, got %f", cfg.Rigor)
	}
	if cfg.FollowConfidence != 0.05 {
		t.Errorf("expected FollowConfidence=0.05, got %f", cfg.FollowConfidence)
	}
}

func TestComputeLargeGraph(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large graph test in short mode")
	}

	// 1000 users, observer follows 50, avg 15 follows each
	graph, _, obs := generateDeterministicGraph(1000, 15, 50)
	store := newMockStore()
	cfg := DefaultConfig()
	cfg.MaxDepth = 3
	cfg.Cycles = 5
	engine := NewEngine(graph, store, cfg)

	set, err := engine.Compute(obs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("large graph: %d pubkeys in hop set, %d scored, %dms",
		set.TotalPubkeys, len(set.Scores), set.ComputeMs)

	// Should discover most of the network via 3 hops in a connected graph
	if set.TotalPubkeys < 100 {
		t.Errorf("expected > 100 pubkeys in hop set with 1000 users, got %d", set.TotalPubkeys)
	}

	// Scores should be stored and retrievable
	retrieved, err := engine.GetScores(obs)
	if err != nil || retrieved == nil {
		t.Fatal("failed to retrieve scores after large computation")
	}

	// Spot check: pick 5 random scores and verify individual lookup
	rng := frand.NewCustom([]byte("spot-check-seed-padding-bytes!!!"), 1024, 12)
	for i := 0; i < 5 && i < len(set.Scores); i++ {
		idx := int(rng.Uint64n(uint64(len(set.Scores))))
		entry, err := engine.GetScore(obs, set.Scores[idx].PubkeyHex)
		if err != nil || entry == nil {
			t.Errorf("spot check %d: failed to retrieve individual score", i)
			continue
		}
		if entry.PubkeyHex != set.Scores[idx].PubkeyHex {
			t.Errorf("spot check %d: pubkey mismatch", i)
		}
	}
}

// scoreMap converts a ScoreSet to a map for easy lookup.
func scoreMap(set *ScoreSet) map[string]*ScoreEntry {
	m := make(map[string]*ScoreEntry, len(set.Scores))
	for i := range set.Scores {
		m[set.Scores[i].PubkeyHex] = &set.Scores[i]
	}
	return m
}

func BenchmarkCompute(b *testing.B) {
	for _, size := range []int{50, 200, 500} {
		b.Run(fmt.Sprintf("users=%d", size), func(b *testing.B) {
			graph, _, obs := generateDeterministicGraph(size, 10, 15)
			cfg := DefaultConfig()
			cfg.MaxDepth = 3

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				store := newMockStore()
				engine := NewEngine(graph, store, cfg)
				engine.Compute(obs)
			}
		})
	}
}
