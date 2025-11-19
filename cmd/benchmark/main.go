package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/encoders/event"
	examples "next.orly.dev/pkg/encoders/event/examples"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/interfaces/signer/p8k"
	"next.orly.dev/pkg/protocol/ws"
)

type BenchmarkConfig struct {
	DataDir           string
	NumEvents         int
	ConcurrentWorkers int
	TestDuration      time.Duration
	BurstPattern      bool
	ReportInterval    time.Duration

	// Network load options
	RelayURL   string
	NetWorkers int
	NetRate    int // events/sec per worker

	// Backend selection
	UseDgraph bool
	UseNeo4j  bool
}

type BenchmarkResult struct {
	TestName          string
	Duration          time.Duration
	TotalEvents       int
	EventsPerSecond   float64
	AvgLatency        time.Duration
	P90Latency        time.Duration
	P95Latency        time.Duration
	P99Latency        time.Duration
	Bottom10Avg       time.Duration
	SuccessRate       float64
	ConcurrentWorkers int
	MemoryUsed        uint64
	Errors            []string
}

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	rate       float64       // events per second
	interval   time.Duration // time between events
	lastEvent  time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a rate limiter for the specified events per second
func NewRateLimiter(eventsPerSecond float64) *RateLimiter {
	return &RateLimiter{
		rate:      eventsPerSecond,
		interval:  time.Duration(float64(time.Second) / eventsPerSecond),
		lastEvent: time.Now(),
	}
}

// Wait blocks until the next event is allowed based on the rate limit
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	nextAllowed := rl.lastEvent.Add(rl.interval)

	if now.Before(nextAllowed) {
		time.Sleep(nextAllowed.Sub(now))
		rl.lastEvent = nextAllowed
	} else {
		rl.lastEvent = now
	}
}

type Benchmark struct {
	config       *BenchmarkConfig
	db           *database.D
	results      []*BenchmarkResult
	mu           sync.RWMutex
	cachedEvents []*event.E // Real-world events from examples.Cache
	eventCacheMu sync.Mutex
}

func main() {
	// lol.SetLogLevel("trace")
	config := parseFlags()

	if config.RelayURL != "" {
		// Network mode: connect to relay and generate traffic
		runNetworkLoad(config)
		return
	}

	if config.UseDgraph {
		// Run dgraph benchmark
		runDgraphBenchmark(config)
		return
	}

	if config.UseNeo4j {
		// Run Neo4j benchmark
		runNeo4jBenchmark(config)
		return
	}

	// Run standard Badger benchmark
	fmt.Printf("Starting Nostr Relay Benchmark (Badger Backend)\n")
	fmt.Printf("Data Directory: %s\n", config.DataDir)
	fmt.Printf(
		"Events: %d, Workers: %d, Duration: %v\n",
		config.NumEvents, config.ConcurrentWorkers, config.TestDuration,
	)

	benchmark := NewBenchmark(config)
	defer benchmark.Close()

	// Run benchmark suite twice with pauses
	benchmark.RunSuite()

	// Generate reports
	benchmark.GenerateReport()
	benchmark.GenerateAsciidocReport()
}

func runDgraphBenchmark(config *BenchmarkConfig) {
	fmt.Printf("Starting Nostr Relay Benchmark (Dgraph Backend)\n")
	fmt.Printf("Data Directory: %s\n", config.DataDir)
	fmt.Printf(
		"Events: %d, Workers: %d\n",
		config.NumEvents, config.ConcurrentWorkers,
	)

	dgraphBench, err := NewDgraphBenchmark(config)
	if err != nil {
		log.Fatalf("Failed to create dgraph benchmark: %v", err)
	}
	defer dgraphBench.Close()

	// Run dgraph benchmark suite
	dgraphBench.RunSuite()

	// Generate reports
	dgraphBench.GenerateReport()
	dgraphBench.GenerateAsciidocReport()
}

func runNeo4jBenchmark(config *BenchmarkConfig) {
	fmt.Printf("Starting Nostr Relay Benchmark (Neo4j Backend)\n")
	fmt.Printf("Data Directory: %s\n", config.DataDir)
	fmt.Printf(
		"Events: %d, Workers: %d\n",
		config.NumEvents, config.ConcurrentWorkers,
	)

	neo4jBench, err := NewNeo4jBenchmark(config)
	if err != nil {
		log.Fatalf("Failed to create Neo4j benchmark: %v", err)
	}
	defer neo4jBench.Close()

	// Run Neo4j benchmark suite
	neo4jBench.RunSuite()

	// Generate reports
	neo4jBench.GenerateReport()
	neo4jBench.GenerateAsciidocReport()
}

func parseFlags() *BenchmarkConfig {
	config := &BenchmarkConfig{}

	flag.StringVar(
		&config.DataDir, "datadir", "/tmp/benchmark_db", "Database directory",
	)
	flag.IntVar(
		&config.NumEvents, "events", 10000, "Number of events to generate",
	)
	flag.IntVar(
		&config.ConcurrentWorkers, "workers", max(2, runtime.NumCPU()/4),
		"Number of concurrent workers (default: CPU cores / 4 for low CPU usage)",
	)
	flag.DurationVar(
		&config.TestDuration, "duration", 60*time.Second, "Test duration",
	)
	flag.BoolVar(
		&config.BurstPattern, "burst", true, "Enable burst pattern testing",
	)
	flag.DurationVar(
		&config.ReportInterval, "report-interval", 10*time.Second,
		"Report interval",
	)

	// Network mode flags
	flag.StringVar(
		&config.RelayURL, "relay-url", "",
		"Relay WebSocket URL (enables network mode if set)",
	)
	flag.IntVar(
		&config.NetWorkers, "net-workers", runtime.NumCPU(),
		"Network workers (connections)",
	)
	flag.IntVar(&config.NetRate, "net-rate", 20, "Events per second per worker")

	// Backend selection
	flag.BoolVar(
		&config.UseDgraph, "dgraph", false,
		"Use dgraph backend (requires Docker)",
	)
	flag.BoolVar(
		&config.UseNeo4j, "neo4j", false,
		"Use Neo4j backend (requires Docker)",
	)

	flag.Parse()
	return config
}

func runNetworkLoad(cfg *BenchmarkConfig) {
	fmt.Printf(
		"Network mode: relay=%s workers=%d rate=%d ev/s per worker duration=%s\n",
		cfg.RelayURL, cfg.NetWorkers, cfg.NetRate, cfg.TestDuration,
	)
	// Create a timeout context for benchmark control only, not for connections
	timeoutCtx, cancel := context.WithTimeout(
		context.Background(), cfg.TestDuration,
	)
	defer cancel()

	// Use a separate background context for relay connections to avoid
	// cancelling the server when the benchmark timeout expires
	connCtx := context.Background()

	var wg sync.WaitGroup
	if cfg.NetWorkers <= 0 {
		cfg.NetWorkers = 1
	}
	if cfg.NetRate <= 0 {
		cfg.NetRate = 1
	}
	for i := 0; i < cfg.NetWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// Connect to relay using non-cancellable context
			rl, err := ws.RelayConnect(connCtx, cfg.RelayURL)
			if err != nil {
				fmt.Printf(
					"worker %d: failed to connect to %s: %v\n", workerID,
					cfg.RelayURL, err,
				)
				return
			}
			defer rl.Close()
			fmt.Printf("worker %d: connected to %s\n", workerID, cfg.RelayURL)

			// Signer for this worker
			var keys *p8k.Signer
			if keys, err = p8k.New(); err != nil {
				fmt.Printf("worker %d: signer create failed: %v\n", workerID, err)
				return
			}
			if err := keys.Generate(); err != nil {
				fmt.Printf("worker %d: keygen failed: %v\n", workerID, err)
				return
			}

			// Start a concurrent subscriber that listens for events published by this worker
			// Build a filter that matches this worker's pubkey and kind=1, since now
			since := time.Now().Unix()
			go func() {
				f := filter.New()
				f.Kinds = kind.NewS(kind.TextNote)
				f.Authors = tag.NewWithCap(1)
				f.Authors.T = append(f.Authors.T, keys.Pub())
				f.Since = timestamp.FromUnix(since)
				sub, err := rl.Subscribe(connCtx, filter.NewS(f))
				if err != nil {
					fmt.Printf(
						"worker %d: subscribe error: %v\n", workerID, err,
					)
					return
				}
				defer sub.Unsub()
				recv := 0
				for {
					select {
					case <-timeoutCtx.Done():
						fmt.Printf(
							"worker %d: subscriber exiting after %d events (benchmark timeout: %v)\n",
							workerID, recv, timeoutCtx.Err(),
						)
						return
					case <-rl.Context().Done():
						fmt.Printf(
							"worker %d: relay connection closed; cause=%v lastErr=%v\n",
							workerID, rl.ConnectionCause(), rl.LastError(),
						)
						return
					case <-sub.EndOfStoredEvents:
						// continue streaming live events
					case ev := <-sub.Events:
						if ev == nil {
							continue
						}
						recv++
						if recv%100 == 0 {
							fmt.Printf(
								"worker %d: received %d matching events\n",
								workerID, recv,
							)
						}
						ev.Free()
					}
				}
			}()

			interval := time.Second / time.Duration(cfg.NetRate)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			count := 0
			for {
				select {
				case <-timeoutCtx.Done():
					fmt.Printf(
						"worker %d: stopping after %d publishes\n", workerID,
						count,
					)
					return
				case <-ticker.C:
					// Build and sign a simple text note event
					ev := event.New()
					ev.Kind = uint16(1)
					ev.CreatedAt = time.Now().Unix()
					ev.Tags = tag.NewS()
					ev.Content = []byte(fmt.Sprintf(
						"bench worker=%d n=%d", workerID, count,
					))
					if err := ev.Sign(keys); err != nil {
						fmt.Printf("worker %d: sign error: %v\n", workerID, err)
						ev.Free()
						continue
					}
					// Async publish: don't wait for OK; this greatly increases throughput
					ch := rl.Write(eventenvelope.NewSubmissionWith(ev).Marshal(nil))
					// Non-blocking error check
					select {
					case err := <-ch:
						if err != nil {
							fmt.Printf(
								"worker %d: write error: %v\n", workerID, err,
							)
						}
					default:
					}
					if count%100 == 0 {
						fmt.Printf(
							"worker %d: sent %d events\n", workerID, count,
						)
					}
					ev.Free()
					count++
				}
			}
		}(i)
	}
	wg.Wait()
}

func NewBenchmark(config *BenchmarkConfig) *Benchmark {
	// Clean up existing data directory
	os.RemoveAll(config.DataDir)

	ctx := context.Background()
	cancel := func() {}

	db, err := database.New(ctx, cancel, config.DataDir, "warn")
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	b := &Benchmark{
		config:  config,
		db:      db,
		results: make([]*BenchmarkResult, 0),
	}

	// Trigger compaction/GC before starting tests
	b.compactDatabase()

	return b
}

func (b *Benchmark) Close() {
	if b.db != nil {
		b.db.Close()
	}
}

// RunSuite runs the full benchmark test suite
func (b *Benchmark) RunSuite() {
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         BADGER BACKEND BENCHMARK SUITE                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	fmt.Printf("\n=== Starting Badger benchmark ===\n")

	fmt.Printf("RunPeakThroughputTest (Badger)..\n")
	b.RunPeakThroughputTest()
	fmt.Println("Wiping database between tests...")
	b.db.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunBurstPatternTest (Badger)..\n")
	b.RunBurstPatternTest()
	fmt.Println("Wiping database between tests...")
	b.db.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunMixedReadWriteTest (Badger)..\n")
	b.RunMixedReadWriteTest()
	fmt.Println("Wiping database between tests...")
	b.db.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunQueryTest (Badger)..\n")
	b.RunQueryTest()
	fmt.Println("Wiping database between tests...")
	b.db.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunConcurrentQueryStoreTest (Badger)..\n")
	b.RunConcurrentQueryStoreTest()

	fmt.Printf("\n=== Badger benchmark completed ===\n\n")
}

// compactDatabase triggers a Badger value log GC before starting tests.
func (b *Benchmark) compactDatabase() {
	if b.db == nil || b.db.DB == nil {
		return
	}
	// Attempt value log GC. Ignore errors; this is best-effort.
	_ = b.db.DB.RunValueLogGC(0.5)
}

func (b *Benchmark) RunPeakThroughputTest() {
	fmt.Println("\n=== Peak Throughput Test ===")

	// Create latency recorder (writes to disk, not memory)
	latencyRecorder, err := NewLatencyRecorder(b.config.DataDir, "peak_throughput")
	if err != nil {
		log.Fatalf("Failed to create latency recorder: %v", err)
	}

	start := time.Now()
	var wg sync.WaitGroup
	var totalEvents int64
	var errorCount int64
	var mu sync.Mutex

	// Stream events from memory (real-world sample events)
	eventChan, errChan := b.getEventChannel(b.config.NumEvents, 1000)

	// Calculate per-worker rate: 20k events/sec total divided by worker count
	// This prevents all workers from synchronizing and hitting DB simultaneously
	perWorkerRate := 20000.0 / float64(b.config.ConcurrentWorkers)

	// Start workers with rate limiting
	ctx := context.Background()

	for i := 0; i < b.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Each worker gets its own rate limiter to avoid mutex contention
			workerLimiter := NewRateLimiter(perWorkerRate)

			for ev := range eventChan {
				// Wait for rate limiter to allow this event
				workerLimiter.Wait()

				eventStart := time.Now()
				_, err := b.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					totalEvents++
					if err := latencyRecorder.Record(latency); err != nil {
						log.Printf("Failed to record latency: %v", err)
					}
				}
				mu.Unlock()
			}
		}(i)
	}

	// Check for streaming errors
	go func() {
		for err := range errChan {
			if err != nil {
				log.Printf("Event stream error: %v", err)
			}
		}
	}()

	wg.Wait()
	duration := time.Since(start)

	// Flush latency data to disk before calculating stats
	if err := latencyRecorder.Close(); err != nil {
		log.Printf("Failed to close latency recorder: %v", err)
	}

	// Calculate statistics from disk
	latencyStats, err := latencyRecorder.CalculateStats()
	if err != nil {
		log.Printf("Failed to calculate latency stats: %v", err)
		latencyStats = &LatencyStats{}
	}

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Peak Throughput",
		Duration:          duration,
		TotalEvents:       int(totalEvents),
		EventsPerSecond:   float64(totalEvents) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
		AvgLatency:        latencyStats.Avg,
		P90Latency:        latencyStats.P90,
		P95Latency:        latencyStats.P95,
		P99Latency:        latencyStats.P99,
		Bottom10Avg:       latencyStats.Bottom10,
	}

	result.SuccessRate = float64(totalEvents) / float64(b.config.NumEvents) * 100

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf(
		"Events saved: %d/%d (%.1f%%), errors: %d\n",
		totalEvents, b.config.NumEvents, result.SuccessRate, errorCount,
	)
	fmt.Printf("Duration: %v\n", duration)
	fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
	fmt.Printf("Avg latency: %v\n", result.AvgLatency)
	fmt.Printf("P90 latency: %v\n", result.P90Latency)
	fmt.Printf("P95 latency: %v\n", result.P95Latency)
	fmt.Printf("P99 latency: %v\n", result.P99Latency)
	fmt.Printf("Bottom 10%% Avg latency: %v\n", result.Bottom10Avg)
}

func (b *Benchmark) RunBurstPatternTest() {
	fmt.Println("\n=== Burst Pattern Test ===")

	// Create latency recorder (writes to disk, not memory)
	latencyRecorder, err := NewLatencyRecorder(b.config.DataDir, "burst_pattern")
	if err != nil {
		log.Fatalf("Failed to create latency recorder: %v", err)
	}

	start := time.Now()
	var totalEvents int64
	var errorCount int64
	var mu sync.Mutex

	// Stream events from memory (real-world sample events)
	eventChan, errChan := b.getEventChannel(b.config.NumEvents, 500)

	// Check for streaming errors
	go func() {
		for err := range errChan {
			if err != nil {
				log.Printf("Event stream error: %v", err)
			}
		}
	}()

	// Simulate burst pattern: high activity periods followed by quiet periods
	burstSize := b.config.NumEvents / 10 // 10% of events in each burst
	quietPeriod := 500 * time.Millisecond
	burstPeriod := 100 * time.Millisecond

	ctx := context.Background()
	var eventIndex int64

	// Start persistent worker pool (prevents goroutine explosion)
	numWorkers := b.config.ConcurrentWorkers
	eventQueue := make(chan *event.E, numWorkers*4)
	var wg sync.WaitGroup

	// Calculate per-worker rate to avoid mutex contention
	perWorkerRate := 20000.0 / float64(numWorkers)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each worker gets its own rate limiter
			workerLimiter := NewRateLimiter(perWorkerRate)

			for ev := range eventQueue {
				// Wait for rate limiter to allow this event
				workerLimiter.Wait()

				eventStart := time.Now()
				_, err := b.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					totalEvents++
					// Record latency to disk instead of keeping in memory
					if err := latencyRecorder.Record(latency); err != nil {
						log.Printf("Failed to record latency: %v", err)
					}
				}
				mu.Unlock()
			}
		}()
	}

	for int(eventIndex) < b.config.NumEvents && time.Since(start) < b.config.TestDuration {
		// Burst period - send events rapidly
		burstStart := time.Now()

		for i := 0; i < burstSize && int(eventIndex) < b.config.NumEvents; i++ {
			ev, ok := <-eventChan
			if !ok {
				break
			}
			eventQueue <- ev
			eventIndex++
			time.Sleep(burstPeriod / time.Duration(burstSize))
		}

		fmt.Printf(
			"Burst completed: %d events in %v\n", burstSize,
			time.Since(burstStart),
		)

		// Quiet period
		time.Sleep(quietPeriod)
	}

	close(eventQueue)
	wg.Wait()

	duration := time.Since(start)

	// Flush latency data to disk before calculating stats
	if err := latencyRecorder.Close(); err != nil {
		log.Printf("Failed to close latency recorder: %v", err)
	}

	// Calculate statistics from disk
	latencyStats, err := latencyRecorder.CalculateStats()
	if err != nil {
		log.Printf("Failed to calculate latency stats: %v", err)
		latencyStats = &LatencyStats{}
	}

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Burst Pattern",
		Duration:          duration,
		TotalEvents:       int(totalEvents),
		EventsPerSecond:   float64(totalEvents) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
		AvgLatency:        latencyStats.Avg,
		P90Latency:        latencyStats.P90,
		P95Latency:        latencyStats.P95,
		P99Latency:        latencyStats.P99,
		Bottom10Avg:       latencyStats.Bottom10,
	}

	result.SuccessRate = float64(totalEvents) / float64(eventIndex) * 100

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf(
		"Burst test completed: %d events in %v, errors: %d\n",
		totalEvents, duration, errorCount,
	)
	fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
}

func (b *Benchmark) RunMixedReadWriteTest() {
	fmt.Println("\n=== Mixed Read/Write Test ===")

	start := time.Now()
	var totalWrites, totalReads int64
	var writeLatencies, readLatencies []time.Duration
	var errors []error
	var mu sync.Mutex

	// Pre-populate with some events for reading
	seedEvents := b.generateEvents(1000)
	ctx := context.Background()

	fmt.Println("Pre-populating database for read tests...")
	for _, ev := range seedEvents {
		b.db.SaveEvent(ctx, ev)
	}

	events := b.generateEvents(b.config.NumEvents)
	var wg sync.WaitGroup

	// Calculate per-worker rate to avoid mutex contention
	perWorkerRate := 20000.0 / float64(b.config.ConcurrentWorkers)

	// Start mixed read/write workers
	for i := 0; i < b.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Each worker gets its own rate limiter
			workerLimiter := NewRateLimiter(perWorkerRate)

			eventIndex := workerID
			for time.Since(start) < b.config.TestDuration && eventIndex < len(events) {
				// Alternate between write and read operations
				if eventIndex%2 == 0 {
					// Write operation - apply rate limiting
					workerLimiter.Wait()

					writeStart := time.Now()
					_, err := b.db.SaveEvent(ctx, events[eventIndex])
					writeLatency := time.Since(writeStart)

					mu.Lock()
					if err != nil {
						errors = append(errors, err)
					} else {
						totalWrites++
						writeLatencies = append(writeLatencies, writeLatency)
					}
					mu.Unlock()
				} else {
					// Read operation
					readStart := time.Now()
					f := filter.New()
					f.Kinds = kind.NewS(kind.TextNote)
					limit := uint(10)
					f.Limit = &limit
					_, err := b.db.GetSerialsFromFilter(f)
					readLatency := time.Since(readStart)

					mu.Lock()
					if err != nil {
						errors = append(errors, err)
					} else {
						totalReads++
						readLatencies = append(readLatencies, readLatency)
					}
					mu.Unlock()
				}

				eventIndex += b.config.ConcurrentWorkers
				time.Sleep(10 * time.Millisecond) // Small delay between operations
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Mixed Read/Write",
		Duration:          duration,
		TotalEvents:       int(totalWrites + totalReads),
		EventsPerSecond:   float64(totalWrites+totalReads) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	// Calculate combined latencies for overall metrics
	allLatencies := append(writeLatencies, readLatencies...)
	if len(allLatencies) > 0 {
		result.AvgLatency = calculateAvgLatency(allLatencies)
		result.P90Latency = calculatePercentileLatency(allLatencies, 0.90)
		result.P95Latency = calculatePercentileLatency(allLatencies, 0.95)
		result.P99Latency = calculatePercentileLatency(allLatencies, 0.99)
		result.Bottom10Avg = calculateBottom10Avg(allLatencies)
	}

	result.SuccessRate = float64(totalWrites+totalReads) / float64(len(events)) * 100

	for _, err := range errors {
		result.Errors = append(result.Errors, err.Error())
	}

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf(
		"Mixed test completed: %d writes, %d reads in %v\n", totalWrites,
		totalReads, duration,
	)
	fmt.Printf("Combined ops/sec: %.2f\n", result.EventsPerSecond)
}

// RunQueryTest specifically benchmarks the QueryEvents function performance
func (b *Benchmark) RunQueryTest() {
	fmt.Println("\n=== Query Test ===")

	start := time.Now()
	var totalQueries int64
	var queryLatencies []time.Duration
	var errors []error
	var mu sync.Mutex

	// Pre-populate with events for querying
	numSeedEvents := 10000
	seedEvents := b.generateEvents(numSeedEvents)
	ctx := context.Background()

	fmt.Printf(
		"Pre-populating database with %d events for query tests...\n",
		numSeedEvents,
	)
	for _, ev := range seedEvents {
		b.db.SaveEvent(ctx, ev)
	}

	// Create different types of filters for querying
	filters := []*filter.F{
		func() *filter.F { // Kind filter
			f := filter.New()
			f.Kinds = kind.NewS(kind.TextNote)
			limit := uint(100)
			f.Limit = &limit
			return f
		}(),
		func() *filter.F { // Tag filter
			f := filter.New()
			f.Tags = tag.NewS(
				tag.NewFromBytesSlice(
					[]byte("t"), []byte("benchmark"),
				),
			)
			limit := uint(100)
			f.Limit = &limit
			return f
		}(),
		func() *filter.F { // Mixed filter
			f := filter.New()
			f.Kinds = kind.NewS(kind.TextNote)
			f.Tags = tag.NewS(
				tag.NewFromBytesSlice(
					[]byte("t"), []byte("benchmark"),
				),
			)
			limit := uint(50)
			f.Limit = &limit
			return f
		}(),
	}

	var wg sync.WaitGroup
	// Start query workers
	for i := 0; i < b.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			filterIndex := workerID % len(filters)
			queryCount := 0

			for time.Since(start) < b.config.TestDuration {
				// Rotate through different filters
				f := filters[filterIndex]
				filterIndex = (filterIndex + 1) % len(filters)

				// Execute query
				queryStart := time.Now()
				events, err := b.db.QueryEvents(ctx, f)
				queryLatency := time.Since(queryStart)

				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else {
					totalQueries++
					queryLatencies = append(queryLatencies, queryLatency)

					// Free event memory
					for _, ev := range events {
						ev.Free()
					}
				}
				mu.Unlock()

				queryCount++
				// Always add delay to prevent CPU saturation (queries are CPU-intensive)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Query Performance",
		Duration:          duration,
		TotalEvents:       int(totalQueries),
		EventsPerSecond:   float64(totalQueries) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	if len(queryLatencies) > 0 {
		result.AvgLatency = calculateAvgLatency(queryLatencies)
		result.P90Latency = calculatePercentileLatency(queryLatencies, 0.90)
		result.P95Latency = calculatePercentileLatency(queryLatencies, 0.95)
		result.P99Latency = calculatePercentileLatency(queryLatencies, 0.99)
		result.Bottom10Avg = calculateBottom10Avg(queryLatencies)
	}

	result.SuccessRate = 100.0 // No specific target count for queries

	for _, err := range errors {
		result.Errors = append(result.Errors, err.Error())
	}

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	fmt.Printf(
		"Query test completed: %d queries in %v\n", totalQueries, duration,
	)
	fmt.Printf("Queries/sec: %.2f\n", result.EventsPerSecond)
	fmt.Printf("Avg query latency: %v\n", result.AvgLatency)
	fmt.Printf("P95 query latency: %v\n", result.P95Latency)
	fmt.Printf("P99 query latency: %v\n", result.P99Latency)
}

// RunConcurrentQueryStoreTest benchmarks the performance of concurrent query and store operations
func (b *Benchmark) RunConcurrentQueryStoreTest() {
	fmt.Println("\n=== Concurrent Query/Store Test ===")

	start := time.Now()
	var totalQueries, totalWrites int64
	var queryLatencies, writeLatencies []time.Duration
	var errors []error
	var mu sync.Mutex

	// Pre-populate with some events
	numSeedEvents := 5000
	seedEvents := b.generateEvents(numSeedEvents)
	ctx := context.Background()

	fmt.Printf(
		"Pre-populating database with %d events for concurrent query/store test...\n",
		numSeedEvents,
	)
	for _, ev := range seedEvents {
		b.db.SaveEvent(ctx, ev)
	}

	// Generate events for writing during the test
	writeEvents := b.generateEvents(b.config.NumEvents)

	// Create filters for querying
	filters := []*filter.F{
		func() *filter.F { // Recent events filter
			f := filter.New()
			f.Since = timestamp.FromUnix(time.Now().Add(-10 * time.Minute).Unix())
			limit := uint(100)
			f.Limit = &limit
			return f
		}(),
		func() *filter.F { // Kind and tag filter
			f := filter.New()
			f.Kinds = kind.NewS(kind.TextNote)
			f.Tags = tag.NewS(
				tag.NewFromBytesSlice(
					[]byte("t"), []byte("benchmark"),
				),
			)
			limit := uint(50)
			f.Limit = &limit
			return f
		}(),
	}

	var wg sync.WaitGroup

	// Half of the workers will be readers, half will be writers
	numReaders := b.config.ConcurrentWorkers / 2
	numWriters := b.config.ConcurrentWorkers - numReaders

	// Calculate per-worker write rate to avoid mutex contention
	perWorkerRate := 20000.0 / float64(numWriters)

	// Start query workers (readers)
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			filterIndex := workerID % len(filters)
			queryCount := 0

			for time.Since(start) < b.config.TestDuration {
				// Select a filter
				f := filters[filterIndex]
				filterIndex = (filterIndex + 1) % len(filters)

				// Execute query
				queryStart := time.Now()
				events, err := b.db.QueryEvents(ctx, f)
				queryLatency := time.Since(queryStart)

				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else {
					totalQueries++
					queryLatencies = append(queryLatencies, queryLatency)

					// Free event memory
					for _, ev := range events {
						ev.Free()
					}
				}
				mu.Unlock()

				queryCount++
				// Always add delay to prevent CPU saturation (queries are CPU-intensive)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Start write workers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Each worker gets its own rate limiter
			workerLimiter := NewRateLimiter(perWorkerRate)

			eventIndex := workerID
			writeCount := 0

			for time.Since(start) < b.config.TestDuration && eventIndex < len(writeEvents) {
				// Write operation - apply rate limiting
				workerLimiter.Wait()

				writeStart := time.Now()
				_, err := b.db.SaveEvent(ctx, writeEvents[eventIndex])
				writeLatency := time.Since(writeStart)

				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else {
					totalWrites++
					writeLatencies = append(writeLatencies, writeLatency)
				}
				mu.Unlock()

				eventIndex += numWriters
				writeCount++
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Calculate metrics
	totalOps := totalQueries + totalWrites
	result := &BenchmarkResult{
		TestName:          "Concurrent Query/Store",
		Duration:          duration,
		TotalEvents:       int(totalOps),
		EventsPerSecond:   float64(totalOps) / duration.Seconds(),
		ConcurrentWorkers: b.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	// Calculate combined latencies for overall metrics
	allLatencies := append(queryLatencies, writeLatencies...)
	if len(allLatencies) > 0 {
		result.AvgLatency = calculateAvgLatency(allLatencies)
		result.P90Latency = calculatePercentileLatency(allLatencies, 0.90)
		result.P95Latency = calculatePercentileLatency(allLatencies, 0.95)
		result.P99Latency = calculatePercentileLatency(allLatencies, 0.99)
		result.Bottom10Avg = calculateBottom10Avg(allLatencies)
	}

	result.SuccessRate = 100.0 // No specific target

	for _, err := range errors {
		result.Errors = append(result.Errors, err.Error())
	}

	b.mu.Lock()
	b.results = append(b.results, result)
	b.mu.Unlock()

	// Calculate separate metrics for queries and writes
	var queryAvg, writeAvg time.Duration
	if len(queryLatencies) > 0 {
		queryAvg = calculateAvgLatency(queryLatencies)
	}
	if len(writeLatencies) > 0 {
		writeAvg = calculateAvgLatency(writeLatencies)
	}

	fmt.Printf(
		"Concurrent test completed: %d operations (%d queries, %d writes) in %v\n",
		totalOps, totalQueries, totalWrites, duration,
	)
	fmt.Printf("Operations/sec: %.2f\n", result.EventsPerSecond)
	fmt.Printf("Avg latency: %v\n", result.AvgLatency)
	fmt.Printf("Avg query latency: %v\n", queryAvg)
	fmt.Printf("Avg write latency: %v\n", writeAvg)
	fmt.Printf("P95 latency: %v\n", result.P95Latency)
	fmt.Printf("P99 latency: %v\n", result.P99Latency)
}

func (b *Benchmark) generateEvents(count int) []*event.E {
	fmt.Printf("Generating %d unique synthetic events (minimum 300 bytes each)...\n", count)

	// Create a single signer for all events (reusing key is faster)
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		log.Fatalf("Failed to generate keypair: %v", err)
	}

	// Base timestamp - start from current time and increment
	baseTime := time.Now().Unix()

	// Minimum content size
	const minContentSize = 300

	// Base content template
	baseContent := "This is a benchmark test event with realistic content size. "

	// Pre-calculate how much padding we need
	paddingNeeded := minContentSize - len(baseContent)
	if paddingNeeded < 0 {
		paddingNeeded = 0
	}

	// Create padding string (with varied characters for realistic size)
	padding := make([]byte, paddingNeeded)
	for i := range padding {
		padding[i] = ' ' + byte(i%94) // Printable ASCII characters
	}

	events := make([]*event.E, count)
	for i := 0; i < count; i++ {
		ev := event.New()
		ev.Kind = kind.TextNote.K
		ev.CreatedAt = baseTime + int64(i) // Unique timestamp for each event
		ev.Tags = tag.NewS()

		// Create content with unique identifier and padding
		ev.Content = []byte(fmt.Sprintf("%s Event #%d. %s", baseContent, i, string(padding)))

		// Sign the event (this calculates ID and Sig)
		if err := ev.Sign(signer); err != nil {
			log.Fatalf("Failed to sign event %d: %v", i, err)
		}

		events[i] = ev
	}

	// Print stats
	totalSize := int64(0)
	for _, ev := range events {
		totalSize += int64(len(ev.Content))
	}
	avgSize := totalSize / int64(count)

	fmt.Printf("Generated %d events:\n", count)
	fmt.Printf("  Average content size: %d bytes\n", avgSize)
	fmt.Printf("  All events are unique (incremental timestamps)\n")
	fmt.Printf("  All events are properly signed\n\n")

	return events
}

// printEventStats prints statistics about the loaded real-world events
func (b *Benchmark) printEventStats() {
	if len(b.cachedEvents) == 0 {
		return
	}

	// Analyze event distribution
	kindCounts := make(map[uint16]int)
	var totalSize int64

	for _, ev := range b.cachedEvents {
		kindCounts[ev.Kind]++
		totalSize += int64(len(ev.Content))
	}

	avgSize := totalSize / int64(len(b.cachedEvents))

	fmt.Printf("\nEvent Statistics:\n")
	fmt.Printf("  Total events: %d\n", len(b.cachedEvents))
	fmt.Printf("  Average content size: %d bytes\n", avgSize)
	fmt.Printf("  Event kinds found: %d unique\n", len(kindCounts))
	fmt.Printf("  Most common kinds:\n")

	// Print top 5 kinds
	type kindCount struct {
		kind  uint16
		count int
	}
	var counts []kindCount
	for k, c := range kindCounts {
		counts = append(counts, kindCount{k, c})
	}
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})
	for i := 0; i < min(5, len(counts)); i++ {
		fmt.Printf("    Kind %d: %d events\n", counts[i].kind, counts[i].count)
	}
	fmt.Println()
}

// loadRealEvents loads events from embedded examples.Cache on first call
func (b *Benchmark) loadRealEvents() {
	b.eventCacheMu.Lock()
	defer b.eventCacheMu.Unlock()

	// Only load once
	if len(b.cachedEvents) > 0 {
		return
	}

	fmt.Println("Loading real-world sample events (11,596 events from 6 months of Nostr)...")
	scanner := bufio.NewScanner(bytes.NewReader(examples.Cache))

	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var ev event.E
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			fmt.Printf("Warning: failed to unmarshal event: %v\n", err)
			continue
		}
		b.cachedEvents = append(b.cachedEvents, &ev)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read events: %v", err)
	}

	fmt.Printf("Loaded %d real-world events (already signed, zero crypto overhead)\n", len(b.cachedEvents))
	b.printEventStats()
}

// getEventChannel returns a channel that streams unique synthetic events
// bufferSize controls memory usage - larger buffers improve throughput but use more memory
func (b *Benchmark) getEventChannel(count int, bufferSize int) (<-chan *event.E, <-chan error) {
	eventChan := make(chan *event.E, bufferSize)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		// Create a single signer for all events
		signer := p8k.MustNew()
		if err := signer.Generate(); err != nil {
			errChan <- fmt.Errorf("failed to generate keypair: %w", err)
			return
		}

		// Base timestamp - start from current time and increment
		baseTime := time.Now().Unix()

		// Minimum content size
		const minContentSize = 300

		// Base content template
		baseContent := "This is a benchmark test event with realistic content size. "

		// Pre-calculate padding
		paddingNeeded := minContentSize - len(baseContent)
		if paddingNeeded < 0 {
			paddingNeeded = 0
		}

		// Create padding string (with varied characters for realistic size)
		padding := make([]byte, paddingNeeded)
		for i := range padding {
			padding[i] = ' ' + byte(i%94) // Printable ASCII characters
		}

		// Stream unique events
		for i := 0; i < count; i++ {
			ev := event.New()
			ev.Kind = kind.TextNote.K
			ev.CreatedAt = baseTime + int64(i) // Unique timestamp for each event
			ev.Tags = tag.NewS()

			// Create content with unique identifier and padding
			ev.Content = []byte(fmt.Sprintf("%s Event #%d. %s", baseContent, i, string(padding)))

			// Sign the event (this calculates ID and Sig)
			if err := ev.Sign(signer); err != nil {
				errChan <- fmt.Errorf("failed to sign event %d: %w", i, err)
				return
			}

			eventChan <- ev
		}
	}()

	return eventChan, errChan
}

// formatSize formats byte size in human-readable format
func formatSize(bytes int) string {
	if bytes == 0 {
		return "Empty (0 bytes)"
	}
	if bytes < 1024 {
		return fmt.Sprintf("%d bytes", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%d KB", bytes/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%d MB", bytes/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (b *Benchmark) GenerateReport() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("BENCHMARK REPORT")
	fmt.Println(strings.Repeat("=", 80))

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, result := range b.results {
		fmt.Printf("\nTest: %s\n", result.TestName)
		fmt.Printf("Duration: %v\n", result.Duration)
		fmt.Printf("Total Events: %d\n", result.TotalEvents)
		fmt.Printf("Events/sec: %.2f\n", result.EventsPerSecond)
		fmt.Printf("Success Rate: %.1f%%\n", result.SuccessRate)
		fmt.Printf("Concurrent Workers: %d\n", result.ConcurrentWorkers)
		fmt.Printf("Memory Used: %d MB\n", result.MemoryUsed/(1024*1024))
		fmt.Printf("Avg Latency: %v\n", result.AvgLatency)
		fmt.Printf("P90 Latency: %v\n", result.P90Latency)
		fmt.Printf("P95 Latency: %v\n", result.P95Latency)
		fmt.Printf("P99 Latency: %v\n", result.P99Latency)
		fmt.Printf("Bottom 10%% Avg Latency: %v\n", result.Bottom10Avg)

		if len(result.Errors) > 0 {
			fmt.Printf("Errors (%d):\n", len(result.Errors))
			for i, err := range result.Errors {
				if i < 5 { // Show first 5 errors
					fmt.Printf("  - %s\n", err)
				}
			}
			if len(result.Errors) > 5 {
				fmt.Printf("  ... and %d more errors\n", len(result.Errors)-5)
			}
		}
		fmt.Println(strings.Repeat("-", 40))
	}

	// Save report to file
	reportPath := filepath.Join(b.config.DataDir, "benchmark_report.txt")
	b.saveReportToFile(reportPath)
	fmt.Printf("\nReport saved to: %s\n", reportPath)
}

func (b *Benchmark) saveReportToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("NOSTR RELAY BENCHMARK REPORT\n")
	file.WriteString("============================\n\n")
	file.WriteString(
		fmt.Sprintf(
			"Generated: %s\n", time.Now().Format(time.RFC3339),
		),
	)
	file.WriteString(fmt.Sprintf("Relay: next.orly.dev\n"))
	file.WriteString(fmt.Sprintf("Database: BadgerDB\n"))
	file.WriteString(fmt.Sprintf("Workers: %d\n", b.config.ConcurrentWorkers))
	file.WriteString(
		fmt.Sprintf(
			"Test Duration: %v\n\n", b.config.TestDuration,
		),
	)

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, result := range b.results {
		file.WriteString(fmt.Sprintf("Test: %s\n", result.TestName))
		file.WriteString(fmt.Sprintf("Duration: %v\n", result.Duration))
		file.WriteString(fmt.Sprintf("Events: %d\n", result.TotalEvents))
		file.WriteString(
			fmt.Sprintf(
				"Events/sec: %.2f\n", result.EventsPerSecond,
			),
		)
		file.WriteString(
			fmt.Sprintf(
				"Success Rate: %.1f%%\n", result.SuccessRate,
			),
		)
		file.WriteString(fmt.Sprintf("Avg Latency: %v\n", result.AvgLatency))
		file.WriteString(fmt.Sprintf("P90 Latency: %v\n", result.P90Latency))
		file.WriteString(fmt.Sprintf("P95 Latency: %v\n", result.P95Latency))
		file.WriteString(fmt.Sprintf("P99 Latency: %v\n", result.P99Latency))
		file.WriteString(
			fmt.Sprintf(
				"Bottom 10%% Avg Latency: %v\n", result.Bottom10Avg,
			),
		)
		file.WriteString(
			fmt.Sprintf(
				"Memory: %d MB\n", result.MemoryUsed/(1024*1024),
			),
		)
		file.WriteString("\n")
	}

	return nil
}

// GenerateAsciidocReport creates a simple AsciiDoc report alongside the text report.
func (b *Benchmark) GenerateAsciidocReport() error {
	path := filepath.Join(b.config.DataDir, "benchmark_report.adoc")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("= NOSTR Relay Benchmark Results\n\n")
	file.WriteString(
		fmt.Sprintf(
			"Generated: %s\n\n", time.Now().Format(time.RFC3339),
		),
	)
	file.WriteString("[cols=\"1,^1,^1,^1,^1,^1\",options=\"header\"]\n")
	file.WriteString("|===\n")
	file.WriteString("| Test | Events/sec | Avg Latency | P90 | P95 | Bottom 10% Avg\n")

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, r := range b.results {
		file.WriteString(fmt.Sprintf("| %s\n", r.TestName))
		file.WriteString(fmt.Sprintf("| %.2f\n", r.EventsPerSecond))
		file.WriteString(fmt.Sprintf("| %v\n", r.AvgLatency))
		file.WriteString(fmt.Sprintf("| %v\n", r.P90Latency))
		file.WriteString(fmt.Sprintf("| %v\n", r.P95Latency))
		file.WriteString(fmt.Sprintf("| %v\n", r.Bottom10Avg))
	}
	file.WriteString("|===\n")

	fmt.Printf("AsciiDoc report saved to: %s\n", path)
	return nil
}

// Helper functions

func calculateAvgLatency(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	var total time.Duration
	for _, l := range latencies {
		total += l
	}
	return total / time.Duration(len(latencies))
}

func calculatePercentileLatency(
	latencies []time.Duration, percentile float64,
) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	// Sort a copy to avoid mutating caller slice
	copySlice := make([]time.Duration, len(latencies))
	copy(copySlice, latencies)
	sort.Slice(
		copySlice, func(i, j int) bool { return copySlice[i] < copySlice[j] },
	)
	index := int(float64(len(copySlice)-1) * percentile)
	if index < 0 {
		index = 0
	}
	if index >= len(copySlice) {
		index = len(copySlice) - 1
	}
	return copySlice[index]
}

// calculateBottom10Avg returns the average latency of the slowest 10% of samples.
func calculateBottom10Avg(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	copySlice := make([]time.Duration, len(latencies))
	copy(copySlice, latencies)
	sort.Slice(
		copySlice, func(i, j int) bool { return copySlice[i] < copySlice[j] },
	)
	start := int(float64(len(copySlice)) * 0.9)
	if start < 0 {
		start = 0
	}
	if start >= len(copySlice) {
		start = len(copySlice) - 1
	}
	var total time.Duration
	for i := start; i < len(copySlice); i++ {
		total += copySlice[i]
	}
	count := len(copySlice) - start
	if count <= 0 {
		return 0
	}
	return total / time.Duration(count)
}

func getMemUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}
