package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/kind"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/encoders/timestamp"
	"next.orly.dev/pkg/interfaces/signer/p8k"
)

// BenchmarkAdapter adapts a database.Database interface to work with benchmark tests
type BenchmarkAdapter struct {
	config  *BenchmarkConfig
	db      database.Database
	results []*BenchmarkResult
	mu      sync.RWMutex
}

// NewBenchmarkAdapter creates a new benchmark adapter
func NewBenchmarkAdapter(config *BenchmarkConfig, db database.Database) *BenchmarkAdapter {
	return &BenchmarkAdapter{
		config:  config,
		db:      db,
		results: make([]*BenchmarkResult, 0),
	}
}

// RunPeakThroughputTest runs the peak throughput benchmark
func (ba *BenchmarkAdapter) RunPeakThroughputTest() {
	fmt.Println("\n=== Peak Throughput Test ===")

	start := time.Now()
	var wg sync.WaitGroup
	var totalEvents int64
	var errors []error
	var latencies []time.Duration
	var mu sync.Mutex

	events := ba.generateEvents(ba.config.NumEvents)
	eventChan := make(chan *event.E, len(events))

	// Fill event channel
	for _, ev := range events {
		eventChan <- ev
	}
	close(eventChan)

	// Start workers
	for i := 0; i < ba.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			ctx := context.Background()
			for ev := range eventChan {
				eventStart := time.Now()

				_, err := ba.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				if err != nil {
					errors = append(errors, err)
				} else {
					totalEvents++
					latencies = append(latencies, latency)
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// Calculate metrics
	result := &BenchmarkResult{
		TestName:          "Peak Throughput",
		Duration:          duration,
		TotalEvents:       int(totalEvents),
		EventsPerSecond:   float64(totalEvents) / duration.Seconds(),
		ConcurrentWorkers: ba.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		result.AvgLatency = calculateAverage(latencies)
		result.P90Latency = latencies[int(float64(len(latencies))*0.90)]
		result.P95Latency = latencies[int(float64(len(latencies))*0.95)]
		result.P99Latency = latencies[int(float64(len(latencies))*0.99)]

		bottom10 := latencies[:int(float64(len(latencies))*0.10)]
		result.Bottom10Avg = calculateAverage(bottom10)
	}

	result.SuccessRate = float64(totalEvents) / float64(ba.config.NumEvents) * 100
	if len(errors) > 0 {
		result.Errors = make([]string, 0, len(errors))
		for _, err := range errors {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	ba.mu.Lock()
	ba.results = append(ba.results, result)
	ba.mu.Unlock()

	ba.printResult(result)
}

// RunBurstPatternTest runs burst pattern test
func (ba *BenchmarkAdapter) RunBurstPatternTest() {
	fmt.Println("\n=== Burst Pattern Test ===")

	start := time.Now()
	var totalEvents int64
	var latencies []time.Duration
	var mu sync.Mutex

	ctx := context.Background()
	burstSize := 100
	bursts := ba.config.NumEvents / burstSize

	for i := 0; i < bursts; i++ {
		// Generate a burst of events
		events := ba.generateEvents(burstSize)

		var wg sync.WaitGroup
		for _, ev := range events {
			wg.Add(1)
			go func(e *event.E) {
				defer wg.Done()

				eventStart := time.Now()
				_, err := ba.db.SaveEvent(ctx, e)
				latency := time.Since(eventStart)

				mu.Lock()
				if err == nil {
					totalEvents++
					latencies = append(latencies, latency)
				}
				mu.Unlock()
			}(ev)
		}

		wg.Wait()

		// Short pause between bursts
		time.Sleep(10 * time.Millisecond)
	}

	duration := time.Since(start)

	result := &BenchmarkResult{
		TestName:          "Burst Pattern",
		Duration:          duration,
		TotalEvents:       int(totalEvents),
		EventsPerSecond:   float64(totalEvents) / duration.Seconds(),
		ConcurrentWorkers: burstSize,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       float64(totalEvents) / float64(ba.config.NumEvents) * 100,
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		result.AvgLatency = calculateAverage(latencies)
		result.P90Latency = latencies[int(float64(len(latencies))*0.90)]
		result.P95Latency = latencies[int(float64(len(latencies))*0.95)]
		result.P99Latency = latencies[int(float64(len(latencies))*0.99)]

		bottom10 := latencies[:int(float64(len(latencies))*0.10)]
		result.Bottom10Avg = calculateAverage(bottom10)
	}

	ba.mu.Lock()
	ba.results = append(ba.results, result)
	ba.mu.Unlock()

	ba.printResult(result)
}

// RunMixedReadWriteTest runs mixed read/write test
func (ba *BenchmarkAdapter) RunMixedReadWriteTest() {
	fmt.Println("\n=== Mixed Read/Write Test ===")

	// First, populate some events
	fmt.Println("Populating database with initial events...")
	populateEvents := ba.generateEvents(1000)
	ctx := context.Background()

	for _, ev := range populateEvents {
		ba.db.SaveEvent(ctx, ev)
	}

	start := time.Now()
	var writeCount, readCount int64
	var latencies []time.Duration
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Start workers doing mixed read/write
	for i := 0; i < ba.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			events := ba.generateEvents(ba.config.NumEvents / ba.config.ConcurrentWorkers)

			for idx, ev := range events {
				eventStart := time.Now()

				if idx%3 == 0 {
					// Read operation
					f := filter.New()
					f.Kinds = kind.NewS(kind.TextNote)
					limit := uint(10)
					f.Limit = &limit
					_, _ = ba.db.QueryEvents(ctx, f)

					mu.Lock()
					readCount++
					mu.Unlock()
				} else {
					// Write operation
					_, _ = ba.db.SaveEvent(ctx, ev)

					mu.Lock()
					writeCount++
					mu.Unlock()
				}

				latency := time.Since(eventStart)
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	result := &BenchmarkResult{
		TestName:          fmt.Sprintf("Mixed R/W (R:%d W:%d)", readCount, writeCount),
		Duration:          duration,
		TotalEvents:       int(writeCount + readCount),
		EventsPerSecond:   float64(writeCount+readCount) / duration.Seconds(),
		ConcurrentWorkers: ba.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		result.AvgLatency = calculateAverage(latencies)
		result.P90Latency = latencies[int(float64(len(latencies))*0.90)]
		result.P95Latency = latencies[int(float64(len(latencies))*0.95)]
		result.P99Latency = latencies[int(float64(len(latencies))*0.99)]

		bottom10 := latencies[:int(float64(len(latencies))*0.10)]
		result.Bottom10Avg = calculateAverage(bottom10)
	}

	ba.mu.Lock()
	ba.results = append(ba.results, result)
	ba.mu.Unlock()

	ba.printResult(result)
}

// RunQueryTest runs query performance test
func (ba *BenchmarkAdapter) RunQueryTest() {
	fmt.Println("\n=== Query Performance Test ===")

	// Populate with test data
	fmt.Println("Populating database for query tests...")
	events := ba.generateEvents(5000)
	ctx := context.Background()

	for _, ev := range events {
		ba.db.SaveEvent(ctx, ev)
	}

	start := time.Now()
	var queryCount int64
	var latencies []time.Duration
	var mu sync.Mutex
	var wg sync.WaitGroup

	queryTypes := []func() *filter.F{
		func() *filter.F {
			f := filter.New()
			f.Kinds = kind.NewS(kind.TextNote)
			limit := uint(100)
			f.Limit = &limit
			return f
		},
		func() *filter.F {
			f := filter.New()
			f.Kinds = kind.NewS(kind.TextNote, kind.Repost)
			limit := uint(50)
			f.Limit = &limit
			return f
		},
		func() *filter.F {
			f := filter.New()
			limit := uint(10)
			f.Limit = &limit
			since := time.Now().Add(-1 * time.Hour).Unix()
			f.Since = timestamp.FromUnix(since)
			return f
		},
	}

	// Run concurrent queries
	iterations := 1000
	for i := 0; i < ba.config.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < iterations/ba.config.ConcurrentWorkers; j++ {
				f := queryTypes[j%len(queryTypes)]()

				queryStart := time.Now()
				_, _ = ba.db.QueryEvents(ctx, f)
				latency := time.Since(queryStart)

				mu.Lock()
				queryCount++
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	result := &BenchmarkResult{
		TestName:          fmt.Sprintf("Query Performance (%d queries)", queryCount),
		Duration:          duration,
		TotalEvents:       int(queryCount),
		EventsPerSecond:   float64(queryCount) / duration.Seconds(),
		ConcurrentWorkers: ba.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		result.AvgLatency = calculateAverage(latencies)
		result.P90Latency = latencies[int(float64(len(latencies))*0.90)]
		result.P95Latency = latencies[int(float64(len(latencies))*0.95)]
		result.P99Latency = latencies[int(float64(len(latencies))*0.99)]

		bottom10 := latencies[:int(float64(len(latencies))*0.10)]
		result.Bottom10Avg = calculateAverage(bottom10)
	}

	ba.mu.Lock()
	ba.results = append(ba.results, result)
	ba.mu.Unlock()

	ba.printResult(result)
}

// RunConcurrentQueryStoreTest runs concurrent query and store test
func (ba *BenchmarkAdapter) RunConcurrentQueryStoreTest() {
	fmt.Println("\n=== Concurrent Query+Store Test ===")

	start := time.Now()
	var storeCount, queryCount int64
	var latencies []time.Duration
	var mu sync.Mutex
	var wg sync.WaitGroup

	ctx := context.Background()

	// Half workers write, half query
	halfWorkers := ba.config.ConcurrentWorkers / 2
	if halfWorkers < 1 {
		halfWorkers = 1
	}

	// Writers
	for i := 0; i < halfWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			events := ba.generateEvents(ba.config.NumEvents / halfWorkers)
			for _, ev := range events {
				eventStart := time.Now()
				ba.db.SaveEvent(ctx, ev)
				latency := time.Since(eventStart)

				mu.Lock()
				storeCount++
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}()
	}

	// Readers
	for i := 0; i < halfWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < ba.config.NumEvents/halfWorkers; j++ {
				f := filter.New()
				f.Kinds = kind.NewS(kind.TextNote)
				limit := uint(10)
				f.Limit = &limit

				queryStart := time.Now()
				ba.db.QueryEvents(ctx, f)
				latency := time.Since(queryStart)

				mu.Lock()
				queryCount++
				latencies = append(latencies, latency)
				mu.Unlock()

				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	result := &BenchmarkResult{
		TestName:          fmt.Sprintf("Concurrent Q+S (Q:%d S:%d)", queryCount, storeCount),
		Duration:          duration,
		TotalEvents:       int(storeCount + queryCount),
		EventsPerSecond:   float64(storeCount+queryCount) / duration.Seconds(),
		ConcurrentWorkers: ba.config.ConcurrentWorkers,
		MemoryUsed:        getMemUsage(),
		SuccessRate:       100.0,
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		result.AvgLatency = calculateAverage(latencies)
		result.P90Latency = latencies[int(float64(len(latencies))*0.90)]
		result.P95Latency = latencies[int(float64(len(latencies))*0.95)]
		result.P99Latency = latencies[int(float64(len(latencies))*0.99)]

		bottom10 := latencies[:int(float64(len(latencies))*0.10)]
		result.Bottom10Avg = calculateAverage(bottom10)
	}

	ba.mu.Lock()
	ba.results = append(ba.results, result)
	ba.mu.Unlock()

	ba.printResult(result)
}

// generateEvents generates test events with proper signatures
func (ba *BenchmarkAdapter) generateEvents(count int) []*event.E {
	events := make([]*event.E, count)

	// Create a test signer
	signer := p8k.MustNew()
	if err := signer.Generate(); err != nil {
		panic(fmt.Sprintf("failed to generate test key: %v", err))
	}

	for i := 0; i < count; i++ {
		ev := event.New()
		ev.Kind = kind.TextNote.ToU16()
		ev.CreatedAt = time.Now().Unix()
		ev.Content = []byte(fmt.Sprintf("Benchmark event #%d - Testing Nostr relay performance with automated load generation", i))
		ev.Tags = tag.NewS()

		// Add some tags for variety
		if i%10 == 0 {
			benchmarkTag := tag.NewFromBytesSlice([]byte("t"), []byte("benchmark"))
			ev.Tags.Append(benchmarkTag)
		}

		// Sign the event (sets Pubkey, ID, and Sig)
		if err := ev.Sign(signer); err != nil {
			panic(fmt.Sprintf("failed to sign event: %v", err))
		}

		events[i] = ev
	}

	return events
}

func (ba *BenchmarkAdapter) printResult(r *BenchmarkResult) {
	fmt.Printf("\nResults for %s:\n", r.TestName)
	fmt.Printf("  Duration: %v\n", r.Duration)
	fmt.Printf("  Total Events: %d\n", r.TotalEvents)
	fmt.Printf("  Events/sec: %.2f\n", r.EventsPerSecond)
	fmt.Printf("  Success Rate: %.2f%%\n", r.SuccessRate)
	fmt.Printf("  Workers: %d\n", r.ConcurrentWorkers)
	fmt.Printf("  Memory Used: %.2f MB\n", float64(r.MemoryUsed)/1024/1024)

	if r.AvgLatency > 0 {
		fmt.Printf("  Avg Latency: %v\n", r.AvgLatency)
		fmt.Printf("  P90 Latency: %v\n", r.P90Latency)
		fmt.Printf("  P95 Latency: %v\n", r.P95Latency)
		fmt.Printf("  P99 Latency: %v\n", r.P99Latency)
		fmt.Printf("  Bottom 10%% Avg: %v\n", r.Bottom10Avg)
	}

	if len(r.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(r.Errors))
		// Print first few errors as samples
		sampleCount := 3
		if len(r.Errors) < sampleCount {
			sampleCount = len(r.Errors)
		}
		for i := 0; i < sampleCount; i++ {
			fmt.Printf("    Sample %d: %s\n", i+1, r.Errors[i])
		}
	}
}

func (ba *BenchmarkAdapter) GenerateReport() {
	// Delegate to main benchmark report generator
	// We'll add the results to a file
	fmt.Println("\n=== Benchmark Results Summary ===")
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	for _, result := range ba.results {
		ba.printResult(result)
	}
}

func (ba *BenchmarkAdapter) GenerateAsciidocReport() {
	// TODO: Implement asciidoc report generation
	fmt.Println("Asciidoc report generation not yet implemented for adapter")
}

func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}
