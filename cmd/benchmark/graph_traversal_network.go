package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"git.mleku.dev/mleku/nostr/ws"
	"lukechampine.com/frand"
)

// NetworkGraphTraversalBenchmark benchmarks graph traversal using NIP-01 queries over WebSocket
type NetworkGraphTraversalBenchmark struct {
	relayURL string
	relay    *ws.Client
	results  []*BenchmarkResult
	mu       sync.RWMutex
	workers  int

	// Cached data for the benchmark
	pubkeys [][]byte      // 100k pubkeys as 32-byte arrays
	signers []*p8k.Signer // signers for each pubkey
	follows [][]int       // follows[i] = list of indices that pubkey[i] follows
	rng     *frand.RNG    // deterministic PRNG
}

// NewNetworkGraphTraversalBenchmark creates a new network graph traversal benchmark
func NewNetworkGraphTraversalBenchmark(relayURL string, workers int) *NetworkGraphTraversalBenchmark {
	return &NetworkGraphTraversalBenchmark{
		relayURL: relayURL,
		workers:  workers,
		results:  make([]*BenchmarkResult, 0),
		rng:      frand.NewCustom(make([]byte, 32), 1024, 12), // ChaCha12 with seed buffer
	}
}

// Connect establishes WebSocket connection to the relay
func (n *NetworkGraphTraversalBenchmark) Connect(ctx context.Context) error {
	var err error
	n.relay, err = ws.RelayConnect(ctx, n.relayURL)
	if err != nil {
		return fmt.Errorf("failed to connect to relay %s: %w", n.relayURL, err)
	}
	return nil
}

// Close closes the relay connection
func (n *NetworkGraphTraversalBenchmark) Close() {
	if n.relay != nil {
		n.relay.Close()
	}
}

// initializeDeterministicRNG initializes the PRNG with deterministic seed
func (n *NetworkGraphTraversalBenchmark) initializeDeterministicRNG() {
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
	n.rng = frand.NewCustom(seedBuf, 1024, 12)
}

// generatePubkeys generates deterministic pubkeys using frand
func (n *NetworkGraphTraversalBenchmark) generatePubkeys() {
	fmt.Printf("Generating %d deterministic pubkeys...\n", GraphBenchNumPubkeys)
	start := time.Now()

	n.initializeDeterministicRNG()
	n.pubkeys = make([][]byte, GraphBenchNumPubkeys)
	n.signers = make([]*p8k.Signer, GraphBenchNumPubkeys)

	for i := 0; i < GraphBenchNumPubkeys; i++ {
		// Generate deterministic 32-byte secret key from PRNG
		secretKey := make([]byte, 32)
		n.rng.Read(secretKey)

		// Create signer from secret key
		signer := p8k.MustNew()
		if err := signer.InitSec(secretKey); err != nil {
			panic(fmt.Sprintf("failed to init signer %d: %v", i, err))
		}

		n.signers[i] = signer
		n.pubkeys[i] = make([]byte, 32)
		copy(n.pubkeys[i], signer.Pub())

		if (i+1)%10000 == 0 {
			fmt.Printf("  Generated %d/%d pubkeys...\n", i+1, GraphBenchNumPubkeys)
		}
	}

	fmt.Printf("Generated %d pubkeys in %v\n", GraphBenchNumPubkeys, time.Since(start))
}

// generateFollowGraph generates the random follow graph with deterministic PRNG
func (n *NetworkGraphTraversalBenchmark) generateFollowGraph() {
	fmt.Printf("Generating follow graph (1-%d follows per pubkey)...\n", GraphBenchMaxFollows)
	start := time.Now()

	// Reset RNG to ensure deterministic follow graph
	n.initializeDeterministicRNG()
	// Skip the bytes used for pubkey generation
	skipBuf := make([]byte, 32*GraphBenchNumPubkeys)
	n.rng.Read(skipBuf)

	n.follows = make([][]int, GraphBenchNumPubkeys)

	totalFollows := 0
	for i := 0; i < GraphBenchNumPubkeys; i++ {
		// Determine number of follows for this pubkey (1 to 1000)
		numFollows := int(n.rng.Uint64n(uint64(GraphBenchMaxFollows-GraphBenchMinFollows+1))) + GraphBenchMinFollows

		// Generate random follow indices (excluding self)
		followSet := make(map[int]struct{})
		for len(followSet) < numFollows {
			followIdx := int(n.rng.Uint64n(uint64(GraphBenchNumPubkeys)))
			if followIdx != i {
				followSet[followIdx] = struct{}{}
			}
		}

		// Convert to slice
		n.follows[i] = make([]int, 0, numFollows)
		for idx := range followSet {
			n.follows[i] = append(n.follows[i], idx)
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

// createFollowListEvents creates kind 3 follow list events via WebSocket
func (n *NetworkGraphTraversalBenchmark) createFollowListEvents(ctx context.Context) {
	fmt.Println("Creating follow list events via WebSocket...")
	start := time.Now()

	baseTime := time.Now().Unix()

	var mu sync.Mutex
	var wg sync.WaitGroup
	var successCount, errorCount int64
	latencies := make([]time.Duration, 0, GraphBenchNumPubkeys)

	// Use worker pool for parallel event creation
	numWorkers := n.workers
	if numWorkers < 1 {
		numWorkers = 4
	}

	workChan := make(chan int, numWorkers*2)

	// Rate limiter: cap at 1000 events/second per relay (to avoid overwhelming)
	perWorkerRate := 1000.0 / float64(numWorkers)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			workerLimiter := NewRateLimiter(perWorkerRate)

			for i := range workChan {
				workerLimiter.Wait()

				ev := event.New()
				ev.Kind = kind.FollowList.K
				ev.CreatedAt = baseTime + int64(i)
				ev.Content = []byte("")
				ev.Tags = tag.NewS()

				// Add p tags for all follows
				for _, followIdx := range n.follows[i] {
					pubkeyHex := hex.Enc(n.pubkeys[followIdx])
					ev.Tags.Append(tag.NewFromAny("p", pubkeyHex))
				}

				// Sign the event
				if err := ev.Sign(n.signers[i]); err != nil {
					mu.Lock()
					errorCount++
					mu.Unlock()
					ev.Free()
					continue
				}

				// Publish via WebSocket
				eventStart := time.Now()
				errCh := n.relay.Write(eventenvelope.NewSubmissionWith(ev).Marshal(nil))

				// Wait for write to complete
				select {
				case err := <-errCh:
					latency := time.Since(eventStart)
					mu.Lock()
					if err != nil {
						errorCount++
					} else {
						successCount++
						latencies = append(latencies, latency)
					}
					mu.Unlock()
				case <-ctx.Done():
					mu.Lock()
					errorCount++
					mu.Unlock()
				}

				ev.Free()
			}
		}()
	}

	// Send work
	for i := 0; i < GraphBenchNumPubkeys; i++ {
		workChan <- i
		if (i+1)%10000 == 0 {
			fmt.Printf("  Queued %d/%d follow list events...\n", i+1, GraphBenchNumPubkeys)
		}
	}
	close(workChan)
	wg.Wait()

	duration := time.Since(start)
	eventsPerSec := float64(successCount) / duration.Seconds()

	// Calculate latency stats
	var avgLatency, p90Latency, p95Latency, p99Latency time.Duration
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		avgLatency = calculateAvgLatency(latencies)
		p90Latency = calculatePercentileLatency(latencies, 0.90)
		p95Latency = calculatePercentileLatency(latencies, 0.95)
		p99Latency = calculatePercentileLatency(latencies, 0.99)
	}

	fmt.Printf("Created %d follow list events in %v (%.2f events/sec, errors: %d)\n",
		successCount, duration, eventsPerSec, errorCount)
	fmt.Printf("  Avg latency: %v, P95: %v, P99: %v\n", avgLatency, p95Latency, p99Latency)

	// Record result for event creation phase
	result := &BenchmarkResult{
		TestName:          "Graph Setup (Follow Lists)",
		Duration:          duration,
		TotalEvents:       int(successCount),
		EventsPerSecond:   eventsPerSec,
		AvgLatency:        avgLatency,
		P90Latency:        p90Latency,
		P95Latency:        p95Latency,
		P99Latency:        p99Latency,
		Bottom10Avg:       calculateBottom10Avg(latencies),
		ConcurrentWorkers: numWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       float64(successCount) / float64(GraphBenchNumPubkeys) * 100,
	}

	n.mu.Lock()
	n.results = append(n.results, result)
	n.mu.Unlock()
}

// runThirdDegreeTraversal runs the third-degree graph traversal benchmark via WebSocket
func (n *NetworkGraphTraversalBenchmark) runThirdDegreeTraversal(ctx context.Context) {
	fmt.Printf("\n=== Third-Degree Graph Traversal Benchmark (Network) ===\n")
	fmt.Printf("Traversing 3 degrees of follows via WebSocket...\n")

	start := time.Now()

	var mu sync.Mutex
	var wg sync.WaitGroup
	var totalQueries int64
	var totalPubkeysFound int64
	queryLatencies := make([]time.Duration, 0, 10000)
	traversalLatencies := make([]time.Duration, 0, 1000)

	// Sample a subset for detailed traversal
	sampleSize := 1000
	if sampleSize > GraphBenchNumPubkeys {
		sampleSize = GraphBenchNumPubkeys
	}

	// Deterministic sampling
	n.initializeDeterministicRNG()
	sampleIndices := make([]int, sampleSize)
	for i := 0; i < sampleSize; i++ {
		sampleIndices[i] = int(n.rng.Uint64n(uint64(GraphBenchNumPubkeys)))
	}

	fmt.Printf("Sampling %d pubkeys for traversal...\n", sampleSize)

	numWorkers := n.workers
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
				currentLevel := [][]byte{n.pubkeys[startIdx]}
				startPubkeyHex := hex.Enc(n.pubkeys[startIdx])
				foundPubkeys[startPubkeyHex] = struct{}{}

				// Traverse 3 degrees
				for depth := 0; depth < GraphBenchTraversalDepth; depth++ {
					if len(currentLevel) == 0 {
						break
					}

					nextLevel := make([][]byte, 0)

					// Query follow lists for all pubkeys at current level
					// Batch queries for efficiency
					batchSize := 50
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
							f.Authors.T = append(f.Authors.T, pk)
						}

						queryStart := time.Now()

						// Subscribe and collect results
						sub, err := n.relay.Subscribe(ctx, filter.NewS(f))
						if err != nil {
							continue
						}

						// Collect events with timeout
						timeout := time.After(5 * time.Second)
						events := make([]*event.E, 0)
					collectLoop:
						for {
							select {
							case ev := <-sub.Events:
								if ev != nil {
									events = append(events, ev)
								}
							case <-sub.EndOfStoredEvents:
								break collectLoop
							case <-timeout:
								break collectLoop
							case <-ctx.Done():
								break collectLoop
							}
						}
						sub.Unsub()

						queryLatency := time.Since(queryStart)

						mu.Lock()
						totalQueries++
						queryLatencies = append(queryLatencies, queryLatency)
						mu.Unlock()

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
	var avgQueryLatency, p90QueryLatency, p95QueryLatency, p99QueryLatency time.Duration
	if len(queryLatencies) > 0 {
		sort.Slice(queryLatencies, func(i, j int) bool { return queryLatencies[i] < queryLatencies[j] })
		avgQueryLatency = calculateAvgLatency(queryLatencies)
		p90QueryLatency = calculatePercentileLatency(queryLatencies, 0.90)
		p95QueryLatency = calculatePercentileLatency(queryLatencies, 0.95)
		p99QueryLatency = calculatePercentileLatency(queryLatencies, 0.99)
	}

	var avgTraversalLatency, p90TraversalLatency, p95TraversalLatency, p99TraversalLatency time.Duration
	if len(traversalLatencies) > 0 {
		sort.Slice(traversalLatencies, func(i, j int) bool { return traversalLatencies[i] < traversalLatencies[j] })
		avgTraversalLatency = calculateAvgLatency(traversalLatencies)
		p90TraversalLatency = calculatePercentileLatency(traversalLatencies, 0.90)
		p95TraversalLatency = calculatePercentileLatency(traversalLatencies, 0.95)
		p99TraversalLatency = calculatePercentileLatency(traversalLatencies, 0.99)
	}

	avgPubkeysPerTraversal := float64(totalPubkeysFound) / float64(sampleSize)
	traversalsPerSec := float64(sampleSize) / duration.Seconds()
	queriesPerSec := float64(totalQueries) / duration.Seconds()

	fmt.Printf("\n=== Graph Traversal Results (Network) ===\n")
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
		P90Latency:        p90TraversalLatency,
		P95Latency:        p95TraversalLatency,
		P99Latency:        p99TraversalLatency,
		Bottom10Avg:       calculateBottom10Avg(traversalLatencies),
		ConcurrentWorkers: numWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	n.mu.Lock()
	n.results = append(n.results, result)
	n.mu.Unlock()

	// Also record query performance separately
	queryResult := &BenchmarkResult{
		TestName:          "Graph Queries (Follow Lists)",
		Duration:          duration,
		TotalEvents:       int(totalQueries),
		EventsPerSecond:   queriesPerSec,
		AvgLatency:        avgQueryLatency,
		P90Latency:        p90QueryLatency,
		P95Latency:        p95QueryLatency,
		P99Latency:        p99QueryLatency,
		Bottom10Avg:       calculateBottom10Avg(queryLatencies),
		ConcurrentWorkers: numWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	n.mu.Lock()
	n.results = append(n.results, queryResult)
	n.mu.Unlock()
}

// RunSuite runs the complete network graph traversal benchmark suite
func (n *NetworkGraphTraversalBenchmark) RunSuite(ctx context.Context) error {
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║  NETWORK GRAPH TRAVERSAL BENCHMARK (100k Pubkeys)      ║")
	fmt.Printf("║  Relay: %-46s ║\n", n.relayURL)
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	// Step 1: Generate pubkeys
	n.generatePubkeys()

	// Step 2: Generate follow graph
	n.generateFollowGraph()

	// Step 3: Connect to relay
	fmt.Printf("\nConnecting to relay: %s\n", n.relayURL)
	if err := n.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer n.Close()
	fmt.Println("Connected successfully!")

	// Step 4: Create follow list events via WebSocket
	n.createFollowListEvents(ctx)

	// Small delay to ensure events are processed
	fmt.Println("\nWaiting for events to be processed...")
	time.Sleep(5 * time.Second)

	// Step 5: Run third-degree traversal benchmark
	n.runThirdDegreeTraversal(ctx)

	fmt.Printf("\n=== Network Graph Traversal Benchmark Complete ===\n\n")
	return nil
}

// GetResults returns the benchmark results
func (n *NetworkGraphTraversalBenchmark) GetResults() []*BenchmarkResult {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.results
}

// PrintResults prints the benchmark results
func (n *NetworkGraphTraversalBenchmark) PrintResults() {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, result := range n.results {
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
