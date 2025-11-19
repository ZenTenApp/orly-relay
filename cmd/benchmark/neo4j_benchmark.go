package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"next.orly.dev/pkg/database"
	_ "next.orly.dev/pkg/neo4j" // Import to register neo4j factory
)

// Neo4jBenchmark wraps a Benchmark with Neo4j-specific setup
type Neo4jBenchmark struct {
	config   *BenchmarkConfig
	docker   *Neo4jDocker
	database database.Database
	bench    *BenchmarkAdapter
}

// NewNeo4jBenchmark creates a new Neo4j benchmark instance
func NewNeo4jBenchmark(config *BenchmarkConfig) (*Neo4jBenchmark, error) {
	// Create Docker manager
	docker, err := NewNeo4jDocker()
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j docker manager: %w", err)
	}

	// Start Neo4j container
	if err := docker.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Neo4j: %w", err)
	}

	// Set environment variables for Neo4j connection
	os.Setenv("ORLY_NEO4J_URI", "bolt://localhost:7687")
	os.Setenv("ORLY_NEO4J_USER", "neo4j")
	os.Setenv("ORLY_NEO4J_PASSWORD", "benchmark123")

	// Create database instance using Neo4j backend
	ctx := context.Background()
	cancel := func() {}
	db, err := database.NewDatabase(ctx, cancel, "neo4j", config.DataDir, "warn")
	if err != nil {
		docker.Stop()
		return nil, fmt.Errorf("failed to create Neo4j database: %w", err)
	}

	// Wait for database to be ready
	fmt.Println("Waiting for Neo4j database to be ready...")
	select {
	case <-db.Ready():
		fmt.Println("Neo4j database is ready")
	case <-time.After(30 * time.Second):
		db.Close()
		docker.Stop()
		return nil, fmt.Errorf("Neo4j database failed to become ready")
	}

	// Create adapter to use Database interface with Benchmark
	adapter := NewBenchmarkAdapter(config, db)

	neo4jBench := &Neo4jBenchmark{
		config:   config,
		docker:   docker,
		database: db,
		bench:    adapter,
	}

	return neo4jBench, nil
}

// Close closes the Neo4j benchmark and stops Docker container
func (ngb *Neo4jBenchmark) Close() {
	fmt.Println("Closing Neo4j benchmark...")

	if ngb.database != nil {
		ngb.database.Close()
	}

	if ngb.docker != nil {
		if err := ngb.docker.Stop(); err != nil {
			log.Printf("Error stopping Neo4j Docker: %v", err)
		}
	}
}

// RunSuite runs the benchmark suite on Neo4j
func (ngb *Neo4jBenchmark) RunSuite() {
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         NEO4J BACKEND BENCHMARK SUITE                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	// Run benchmark tests
	fmt.Printf("\n=== Starting Neo4j benchmark ===\n")

	fmt.Printf("RunPeakThroughputTest (Neo4j)..\n")
	ngb.bench.RunPeakThroughputTest()
	fmt.Println("Wiping database between tests...")
	ngb.database.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunBurstPatternTest (Neo4j)..\n")
	ngb.bench.RunBurstPatternTest()
	fmt.Println("Wiping database between tests...")
	ngb.database.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunMixedReadWriteTest (Neo4j)..\n")
	ngb.bench.RunMixedReadWriteTest()
	fmt.Println("Wiping database between tests...")
	ngb.database.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunQueryTest (Neo4j)..\n")
	ngb.bench.RunQueryTest()
	fmt.Println("Wiping database between tests...")
	ngb.database.Wipe()
	time.Sleep(10 * time.Second)

	fmt.Printf("RunConcurrentQueryStoreTest (Neo4j)..\n")
	ngb.bench.RunConcurrentQueryStoreTest()

	fmt.Printf("\n=== Neo4j benchmark completed ===\n\n")
}

// GenerateReport generates the benchmark report
func (ngb *Neo4jBenchmark) GenerateReport() {
	ngb.bench.GenerateReport()
}

// GenerateAsciidocReport generates asciidoc format report
func (ngb *Neo4jBenchmark) GenerateAsciidocReport() {
	ngb.bench.GenerateAsciidocReport()
}
