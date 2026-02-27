package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"lukechampine.com/frand"
	"next.orly.dev/pkg/database"
)

// GraphBenchNumPubkeys is the number of pubkeys to generate for graph benchmark.
// Set via -graph-pubkeys flag; default 10000.
var GraphBenchNumPubkeys = 10000

const (
	// GraphBenchMinFollows is the minimum number of follows per pubkey
	GraphBenchMinFollows = 1
	// GraphBenchMaxFollows is the maximum number of follows per pubkey
	GraphBenchMaxFollows = 1000
	// GraphBenchSeed is the deterministic seed for frand PRNG (fits in uint64)
	GraphBenchSeed uint64 = 0x4E6F737472 // "Nostr" in hex
	// GraphBenchTraversalDepth is the depth of graph traversal (3 = third degree)
	GraphBenchTraversalDepth = 3
)

// GraphTraversalBenchmark benchmarks graph traversal using NIP-01 style queries
type GraphTraversalBenchmark struct {
	config  *BenchmarkConfig
	db      *database.D
	results []*BenchmarkResult
	mu      sync.RWMutex

	// Cached data for the benchmark
	pubkeys  [][]byte          // 100k pubkeys as 32-byte arrays
	signers  []*p8k.Signer     // signers for each pubkey
	follows  [][]int           // follows[i] = list of indices that pubkey[i] follows
	rng      *frand.RNG        // deterministic PRNG
}

// NewGraphTraversalBenchmark creates a new graph traversal benchmark
func NewGraphTraversalBenchmark(config *BenchmarkConfig, db *database.D) *GraphTraversalBenchmark {
	return &GraphTraversalBenchmark{
		config:  config,
		db:      db,
		results: make([]*BenchmarkResult, 0),
		rng:     frand.NewCustom(make([]byte, 32), 1024, 12), // ChaCha12 with seed buffer
	}
}

// initializeDeterministicRNG initializes the PRNG with deterministic seed
func (g *GraphTraversalBenchmark) initializeDeterministicRNG() {
	// Create seed buffer from GraphBenchSeed (uint64 spread across 8 bytes)
	seedBuf := make([]byte, 32)
	seed := GraphBenchSeed
	seedBuf[0] = byte(seed >> 56)
	seedBuf[1] = byte(seed >> 48)
	seedBuf[2] = byte(seed >> 40)
	seedBuf[3] = byte(seed >> 32)
	seedBuf[4] = byte(seed >> 24)
	seedBuf[5] = byte(seed >> 16)
	seedBuf[6] = byte(seed >> 8)
	seedBuf[7] = byte(seed)
	g.rng = frand.NewCustom(seedBuf, 1024, 12)
}

// generatePubkeys generates deterministic pubkeys using frand
func (g *GraphTraversalBenchmark) generatePubkeys() {
	fmt.Printf("Generating %d deterministic pubkeys...\n", GraphBenchNumPubkeys)
	start := time.Now()

	g.initializeDeterministicRNG()
	g.pubkeys = make([][]byte, GraphBenchNumPubkeys)
	g.signers = make([]*p8k.Signer, GraphBenchNumPubkeys)

	for i := 0; i < GraphBenchNumPubkeys; i++ {
		// Generate deterministic 32-byte secret key from PRNG
		secretKey := make([]byte, 32)
		g.rng.Read(secretKey)

		// Create signer from secret key
		signer := p8k.MustNew()
		if err := signer.InitSec(secretKey); err != nil {
			panic(fmt.Sprintf("failed to init signer %d: %v", i, err))
		}

		g.signers[i] = signer
		g.pubkeys[i] = make([]byte, 32)
		copy(g.pubkeys[i], signer.Pub())

		if (i+1)%10000 == 0 {
			fmt.Printf("  Generated %d/%d pubkeys...\n", i+1, GraphBenchNumPubkeys)
		}
	}

	fmt.Printf("Generated %d pubkeys in %v\n", GraphBenchNumPubkeys, time.Since(start))
}

// generateFollowGraph generates the random follow graph with deterministic PRNG
func (g *GraphTraversalBenchmark) generateFollowGraph() {
	fmt.Printf("Generating follow graph (1-%d follows per pubkey)...\n", GraphBenchMaxFollows)
	start := time.Now()

	// Reset RNG to ensure deterministic follow graph
	g.initializeDeterministicRNG()
	// Skip the bytes used for pubkey generation
	skipBuf := make([]byte, 32*GraphBenchNumPubkeys)
	g.rng.Read(skipBuf)

	g.follows = make([][]int, GraphBenchNumPubkeys)

	totalFollows := 0
	for i := 0; i < GraphBenchNumPubkeys; i++ {
		// Determine number of follows for this pubkey (1 to 1000)
		numFollows := int(g.rng.Uint64n(uint64(GraphBenchMaxFollows-GraphBenchMinFollows+1))) + GraphBenchMinFollows

		// Generate random follow indices (excluding self)
		followSet := make(map[int]struct{})
		for len(followSet) < numFollows {
			followIdx := int(g.rng.Uint64n(uint64(GraphBenchNumPubkeys)))
			if followIdx != i {
				followSet[followIdx] = struct{}{}
			}
		}

		// Convert to slice
		g.follows[i] = make([]int, 0, numFollows)
		for idx := range followSet {
			g.follows[i] = append(g.follows[i], idx)
		}
		totalFollows += numFollows

		if (i+1)%10000 == 0 {
			fmt.Printf("  Generated follow lists for %d/%d pubkeys...\n", i+1, GraphBenchNumPubkeys)
		}
	}

	avgFollows := float64(totalFollows) / float64(GraphBenchNumPubkeys)
	fmt.Printf("Generated follow graph in %v (avg %.1f follows/pubkey, total %d follows)\n",
		time.Since(start), avgFollows, totalFollows)
}

// createFollowListEvents creates kind 3 follow list events in the database.
// Signing is done sequentially (p256k1 uses a global SHA-256 context that is
// not goroutine-safe), then saving is parallelized across workers.
func (g *GraphTraversalBenchmark) createFollowListEvents() {
	fmt.Println("Creating follow list events in database...")
	start := time.Now()

	ctx := context.Background()
	baseTime := time.Now().Unix()

	var mu sync.Mutex
	var wg sync.WaitGroup
	var successCount, errorCount int64
	latencies := make([]time.Duration, 0, GraphBenchNumPubkeys)

	// Use worker pool for parallel database saves
	numWorkers := g.config.ConcurrentWorkers
	if numWorkers < 1 {
		numWorkers = 4
	}

	// Channel carries pre-signed events for parallel saving
	eventChan := make(chan *event.E, numWorkers*4)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for ev := range eventChan {
				// Save to database
				eventStart := time.Now()
				_, err := g.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					successCount++
					latencies = append(latencies, latency)
				}
				mu.Unlock()

				ev.Free()
			}
		}()
	}

	// Sign events sequentially (p256k1.TaggedHash uses a global hash.Hash
	// that panics under concurrent access) and feed to save workers.
	signErrors := 0
	for i := 0; i < GraphBenchNumPubkeys; i++ {
		ev := event.New()
		ev.Kind = kind.FollowList.K
		ev.CreatedAt = baseTime + int64(i)
		ev.Content = []byte("")
		ev.Tags = tag.NewS()

		// Add p tags for all follows
		for _, followIdx := range g.follows[i] {
			pubkeyHex := hex.Enc(g.pubkeys[followIdx])
			ev.Tags.Append(tag.NewFromAny("p", pubkeyHex))
		}

		// Sign the event (must be sequential — p256k1 global state)
		if err := ev.Sign(g.signers[i]); err != nil {
			signErrors++
			ev.Free()
			continue
		}

		eventChan <- ev

		if (i+1)%10000 == 0 {
			fmt.Printf("  Signed and queued %d/%d follow list events...\n", i+1, GraphBenchNumPubkeys)
		}
	}
	close(eventChan)
	wg.Wait()

	duration := time.Since(start)
	eventsPerSec := float64(successCount) / duration.Seconds()

	// Calculate latency stats
	var avgLatency, p95Latency, p99Latency time.Duration
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		avgLatency = calculateAvgLatency(latencies)
		p95Latency = calculatePercentileLatency(latencies, 0.95)
		p99Latency = calculatePercentileLatency(latencies, 0.99)
	}

	fmt.Printf("Created %d follow list events in %v (%.2f events/sec, save errors: %d, sign errors: %d)\n",
		successCount, duration, eventsPerSec, errorCount, signErrors)
	fmt.Printf("  Avg latency: %v, P95: %v, P99: %v\n", avgLatency, p95Latency, p99Latency)

	// Record result for event creation phase
	result := &BenchmarkResult{
		TestName:          "Graph Setup (Follow Lists)",
		Duration:          duration,
		TotalEvents:       int(successCount),
		EventsPerSecond:   eventsPerSec,
		AvgLatency:        avgLatency,
		P95Latency:        p95Latency,
		P99Latency:        p99Latency,
		ConcurrentWorkers: numWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       float64(successCount) / float64(GraphBenchNumPubkeys) * 100,
	}

	g.mu.Lock()
	g.results = append(g.results, result)
	g.mu.Unlock()
}

// runThirdDegreeTraversal runs the third-degree graph traversal benchmark
func (g *GraphTraversalBenchmark) runThirdDegreeTraversal() {
	fmt.Printf("\n=== Third-Degree Graph Traversal Benchmark ===\n")
	fmt.Printf("Traversing 3 degrees of follows for each of %d pubkeys...\n", GraphBenchNumPubkeys)

	start := time.Now()
	ctx := context.Background()

	var mu sync.Mutex
	var wg sync.WaitGroup
	var totalQueries int64
	var totalPubkeysFound int64
	queryLatencies := make([]time.Duration, 0, GraphBenchNumPubkeys*3)
	traversalLatencies := make([]time.Duration, 0, GraphBenchNumPubkeys)

	// Sample a subset for detailed traversal (full 100k would take too long)
	sampleSize := 1000
	if sampleSize > GraphBenchNumPubkeys {
		sampleSize = GraphBenchNumPubkeys
	}

	// Deterministic sampling
	g.initializeDeterministicRNG()
	sampleIndices := make([]int, sampleSize)
	for i := 0; i < sampleSize; i++ {
		sampleIndices[i] = int(g.rng.Uint64n(uint64(GraphBenchNumPubkeys)))
	}

	fmt.Printf("Sampling %d pubkeys for traversal...\n", sampleSize)

	numWorkers := g.config.ConcurrentWorkers
	if numWorkers < 1 {
		numWorkers = 4
	}

	workChan := make(chan int, numWorkers*2)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for startIdx := range workChan {
				traversalStart := time.Now()
				foundPubkeys := make(map[string]struct{})

				// Start with the initial pubkey
				currentLevel := [][]byte{g.pubkeys[startIdx]}
				startPubkeyHex := hex.Enc(g.pubkeys[startIdx])
				foundPubkeys[startPubkeyHex] = struct{}{}

				// Traverse 3 degrees
				for depth := 0; depth < GraphBenchTraversalDepth; depth++ {
					if len(currentLevel) == 0 {
						break
					}

					nextLevel := make([][]byte, 0)

					// Query follow lists for all pubkeys at current level
					// Batch queries for efficiency
					batchSize := 100
					for batchStart := 0; batchStart < len(currentLevel); batchStart += batchSize {
						batchEnd := batchStart + batchSize
						if batchEnd > len(currentLevel) {
							batchEnd = len(currentLevel)
						}

						batch := currentLevel[batchStart:batchEnd]

						// Build filter for kind 3 events from these pubkeys
						f := filter.New()
						f.Kinds = kind.NewS(kind.FollowList)
						f.Authors = tag.NewWithCap(len(batch))
						for _, pk := range batch {
							// Authors.T expects raw byte slices (pubkeys)
							f.Authors.T = append(f.Authors.T, pk)
						}

						queryStart := time.Now()
						events, err := g.db.QueryEvents(ctx, f)
						queryLatency := time.Since(queryStart)

						mu.Lock()
						totalQueries++
						queryLatencies = append(queryLatencies, queryLatency)
						mu.Unlock()

						if err != nil {
							continue
						}

						// Extract followed pubkeys from p tags
						for _, ev := range events {
							for _, t := range *ev.Tags {
								if len(t.T) >= 2 && string(t.T[0]) == "p" {
									pubkeyHex := string(t.ValueHex())
									if _, exists := foundPubkeys[pubkeyHex]; !exists {
										foundPubkeys[pubkeyHex] = struct{}{}
										// Decode hex to bytes for next level
										if pkBytes, err := hex.Dec(pubkeyHex); err == nil {
											nextLevel = append(nextLevel, pkBytes)
										}
									}
								}
							}
							ev.Free()
						}
					}

					currentLevel = nextLevel
				}

				traversalLatency := time.Since(traversalStart)

				mu.Lock()
				totalPubkeysFound += int64(len(foundPubkeys))
				traversalLatencies = append(traversalLatencies, traversalLatency)
				mu.Unlock()
			}
		}()
	}

	// Send work
	for _, idx := range sampleIndices {
		workChan <- idx
	}
	close(workChan)
	wg.Wait()

	duration := time.Since(start)

	// Calculate statistics
	var avgQueryLatency, p95QueryLatency, p99QueryLatency time.Duration
	if len(queryLatencies) > 0 {
		sort.Slice(queryLatencies, func(i, j int) bool { return queryLatencies[i] < queryLatencies[j] })
		avgQueryLatency = calculateAvgLatency(queryLatencies)
		p95QueryLatency = calculatePercentileLatency(queryLatencies, 0.95)
		p99QueryLatency = calculatePercentileLatency(queryLatencies, 0.99)
	}

	var avgTraversalLatency, p95TraversalLatency, p99TraversalLatency time.Duration
	if len(traversalLatencies) > 0 {
		sort.Slice(traversalLatencies, func(i, j int) bool { return traversalLatencies[i] < traversalLatencies[j] })
		avgTraversalLatency = calculateAvgLatency(traversalLatencies)
		p95TraversalLatency = calculatePercentileLatency(traversalLatencies, 0.95)
		p99TraversalLatency = calculatePercentileLatency(traversalLatencies, 0.99)
	}

	avgPubkeysPerTraversal := float64(totalPubkeysFound) / float64(sampleSize)
	traversalsPerSec := float64(sampleSize) / duration.Seconds()
	queriesPerSec := float64(totalQueries) / duration.Seconds()

	fmt.Printf("\n=== Graph Traversal Results ===\n")
	fmt.Printf("Traversals completed: %d\n", sampleSize)
	fmt.Printf("Total queries: %d (%.2f queries/sec)\n", totalQueries, queriesPerSec)
	fmt.Printf("Avg pubkeys found per traversal: %.1f\n", avgPubkeysPerTraversal)
	fmt.Printf("Total duration: %v\n", duration)
	fmt.Printf("\nQuery Latencies:\n")
	fmt.Printf("  Avg: %v, P95: %v, P99: %v\n", avgQueryLatency, p95QueryLatency, p99QueryLatency)
	fmt.Printf("\nFull Traversal Latencies (3 degrees):\n")
	fmt.Printf("  Avg: %v, P95: %v, P99: %v\n", avgTraversalLatency, p95TraversalLatency, p99TraversalLatency)
	fmt.Printf("Traversals/sec: %.2f\n", traversalsPerSec)

	// Record result for traversal phase
	result := &BenchmarkResult{
		TestName:          "Graph Traversal (3 Degrees)",
		Duration:          duration,
		TotalEvents:       int(totalQueries),
		EventsPerSecond:   traversalsPerSec,
		AvgLatency:        avgTraversalLatency,
		P90Latency:        calculatePercentileLatency(traversalLatencies, 0.90),
		P95Latency:        p95TraversalLatency,
		P99Latency:        p99TraversalLatency,
		Bottom10Avg:       calculateBottom10Avg(traversalLatencies),
		ConcurrentWorkers: numWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	g.mu.Lock()
	g.results = append(g.results, result)
	g.mu.Unlock()

	// Also record query performance separately
	queryResult := &BenchmarkResult{
		TestName:          "Graph Queries (Follow Lists)",
		Duration:          duration,
		TotalEvents:       int(totalQueries),
		EventsPerSecond:   queriesPerSec,
		AvgLatency:        avgQueryLatency,
		P90Latency:        calculatePercentileLatency(queryLatencies, 0.90),
		P95Latency:        p95QueryLatency,
		P99Latency:        p99QueryLatency,
		Bottom10Avg:       calculateBottom10Avg(queryLatencies),
		ConcurrentWorkers: numWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	g.mu.Lock()
	g.results = append(g.results, queryResult)
	g.mu.Unlock()
}

// SeedDatabase generates pubkeys, follow graph, and creates follow list events.
// Call this before RunTraversal or PPG comparison to populate the database.
func (g *GraphTraversalBenchmark) SeedDatabase() {
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║      SEEDING GRAPH DATABASE                            ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	// Step 1: Generate pubkeys
	g.generatePubkeys()

	// Step 2: Generate follow graph
	g.generateFollowGraph()

	// Step 3: Create follow list events in database
	g.createFollowListEvents()

	fmt.Printf("\n=== Database Seeded ===\n\n")
}

// RegeneratePubkeys regenerates the deterministic pubkey/signer arrays
// without re-seeding the database. Used when reusing an existing DB.
func (g *GraphTraversalBenchmark) RegeneratePubkeys() {
	g.generatePubkeys()
	g.generateFollowGraph()
}

// RunSuite runs the complete graph traversal benchmark suite
func (g *GraphTraversalBenchmark) RunSuite() {
	g.SeedDatabase()
	g.runThirdDegreeTraversal()
	fmt.Printf("\n=== Graph Traversal Benchmark Complete ===\n\n")
}

// GetResults returns the benchmark results
func (g *GraphTraversalBenchmark) GetResults() []*BenchmarkResult {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.results
}

// PrintResults prints the benchmark results
func (g *GraphTraversalBenchmark) PrintResults() {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, result := range g.results {
		fmt.Printf("\nTest: %s\n", result.TestName)
		fmt.Printf("Duration: %v\n", result.Duration)
		fmt.Printf("Total Events/Queries: %d\n", result.TotalEvents)
		fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
		fmt.Printf("Success Rate: %.1f%%\n", result.SuccessRate)
		fmt.Printf("Concurrent Workers: %d\n", result.ConcurrentWorkers)
		fmt.Printf("Memory Used: %d MB\n", result.MemoryUsed/(1024*1024))
		fmt.Printf("Avg Latency: %v\n", result.AvgLatency)
		fmt.Printf("P90 Latency: %v\n", result.P90Latency)
		fmt.Printf("P95 Latency: %v\n", result.P95Latency)
		fmt.Printf("P99 Latency: %v\n", result.P99Latency)
		fmt.Printf("Bottom 10%% Avg Latency: %v\n", result.Bottom10Avg)
	}
}
