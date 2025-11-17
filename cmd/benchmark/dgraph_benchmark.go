package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"next.orly.dev/pkg/database"
	_ "next.orly.dev/pkg/dgraph" // Import to register dgraph factory
)

// DgraphBenchmark wraps a Benchmark with dgraph-specific setup
type DgraphBenchmark struct {
	config   *BenchmarkConfig
	docker   *DgraphDocker
	database database.Database
	bench    *BenchmarkAdapter
}

// NewDgraphBenchmark creates a new dgraph benchmark instance
func NewDgraphBenchmark(config *BenchmarkConfig) (*DgraphBenchmark, error) {
	// Create Docker manager
	docker := NewDgraphDocker()

	// Start dgraph containers
	ctx := context.Background()
	if err := docker.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start dgraph: %w", err)
	}

	// Set environment variable for dgraph connection
	os.Setenv("ORLY_DGRAPH_URL", docker.GetGRPCEndpoint())

	// Create database instance using dgraph backend
	cancel := func() {}
	db, err := database.NewDatabase(ctx, cancel, "dgraph", config.DataDir, "warn")
	if err != nil {
		docker.Stop()
		return nil, fmt.Errorf("failed to create dgraph database: %w", err)
	}

	// Wait for database to be ready
	fmt.Println("Waiting for dgraph database to be ready...")
	select {
	case <-db.Ready():
		fmt.Println("Dgraph database is ready")
	case <-time.After(30 * time.Second):
		db.Close()
		docker.Stop()
		return nil, fmt.Errorf("dgraph database failed to become ready")
	}

	// Create adapter to use Database interface with Benchmark
	adapter := NewBenchmarkAdapter(config, db)

	dgraphBench := &DgraphBenchmark{
		config:   config,
		docker:   docker,
		database: db,
		bench:    adapter,
	}

	return dgraphBench, nil
}

// Close closes the dgraph benchmark and stops Docker containers
func (dgb *DgraphBenchmark) Close() {
	fmt.Println("Closing dgraph benchmark...")

	if dgb.database != nil {
		dgb.database.Close()
	}

	if dgb.docker != nil {
		if err := dgb.docker.Stop(); err != nil {
			log.Printf("Error stopping dgraph Docker: %v", err)
		}
	}
}

// RunSuite runs the benchmark suite on dgraph
func (dgb *DgraphBenchmark) RunSuite() {
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         DGRAPH BACKEND BENCHMARK SUITE                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	// Run only one round for dgraph to keep benchmark time reasonable
	fmt.Printf("\n=== Starting dgraph benchmark ===\n")

	fmt.Printf("RunPeakThroughputTest (dgraph)..\n")
	dgb.bench.RunPeakThroughputTest()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunBurstPatternTest (dgraph)..\n")
	dgb.bench.RunBurstPatternTest()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunMixedReadWriteTest (dgraph)..\n")
	dgb.bench.RunMixedReadWriteTest()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunQueryTest (dgraph)..\n")
	dgb.bench.RunQueryTest()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunConcurrentQueryStoreTest (dgraph)..\n")
	dgb.bench.RunConcurrentQueryStoreTest()

	fmt.Printf("\n=== Dgraph benchmark completed ===\n\n")
}

// GenerateReport generates the benchmark report
func (dgb *DgraphBenchmark) GenerateReport() {
	dgb.bench.GenerateReport()
}

// GenerateAsciidocReport generates asciidoc format report
func (dgb *DgraphBenchmark) GenerateAsciidocReport() {
	dgb.bench.GenerateAsciidocReport()
}
