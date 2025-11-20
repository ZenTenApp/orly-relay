package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"next.orly.dev/pkg/database"
)

// RelySQLiteBenchmark wraps a Benchmark with rely-sqlite-specific setup
type RelySQLiteBenchmark struct {
	config   *BenchmarkConfig
	database database.Database
	bench    *BenchmarkAdapter
	dbPath   string
}

// NewRelySQLiteBenchmark creates a new rely-sqlite benchmark instance
func NewRelySQLiteBenchmark(config *BenchmarkConfig) (*RelySQLiteBenchmark, error) {
	// Create database path
	dbPath := filepath.Join(config.DataDir, "relysqlite.db")

	// Ensure parent directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Remove existing database file if it exists
	if _, err := os.Stat(dbPath); err == nil {
		if err := os.Remove(dbPath); err != nil {
			return nil, fmt.Errorf("failed to remove existing database: %w", err)
		}
	}

	// Create wrapper
	wrapper, err := NewRelySQLiteWrapper(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create rely-sqlite wrapper: %w", err)
	}

	// Wait for database to be ready
	fmt.Println("Waiting for rely-sqlite database to be ready...")
	select {
	case <-wrapper.Ready():
		fmt.Println("Rely-sqlite database is ready")
	case <-time.After(10 * time.Second):
		wrapper.Close()
		return nil, fmt.Errorf("rely-sqlite database failed to become ready")
	}

	// Create adapter to use Database interface with Benchmark
	adapter := NewBenchmarkAdapter(config, wrapper)

	relysqliteBench := &RelySQLiteBenchmark{
		config:   config,
		database: wrapper,
		bench:    adapter,
		dbPath:   dbPath,
	}

	return relysqliteBench, nil
}

// Close closes the rely-sqlite benchmark
func (rsb *RelySQLiteBenchmark) Close() {
	fmt.Println("Closing rely-sqlite benchmark...")

	if rsb.database != nil {
		rsb.database.Close()
	}

	// Clean up database file
	if rsb.dbPath != "" {
		os.Remove(rsb.dbPath)
	}
}

// RunSuite runs the benchmark suite on rely-sqlite
func (rsb *RelySQLiteBenchmark) RunSuite() {
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║       RELY-SQLITE BACKEND BENCHMARK SUITE              ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")

	// Run benchmark tests
	fmt.Printf("\n=== Starting Rely-SQLite benchmark ===\n")

	fmt.Printf("RunPeakThroughputTest (Rely-SQLite)..\n")
	rsb.bench.RunPeakThroughputTest()
	fmt.Println("Wiping database between tests...")
	rsb.wipeDatabase()
	time.Sleep(5 * time.Second)

	fmt.Printf("RunBurstPatternTest (Rely-SQLite)..\n")
	rsb.bench.RunBurstPatternTest()
	fmt.Println("Wiping database between tests...")
	rsb.wipeDatabase()
	time.Sleep(5 * time.Second)

	fmt.Printf("RunMixedReadWriteTest (Rely-SQLite)..\n")
	rsb.bench.RunMixedReadWriteTest()
	fmt.Println("Wiping database between tests...")
	rsb.wipeDatabase()
	time.Sleep(5 * time.Second)

	fmt.Printf("RunQueryTest (Rely-SQLite)..\n")
	rsb.bench.RunQueryTest()
	fmt.Println("Wiping database between tests...")
	rsb.wipeDatabase()
	time.Sleep(5 * time.Second)

	fmt.Printf("RunConcurrentQueryStoreTest (Rely-SQLite)..\n")
	rsb.bench.RunConcurrentQueryStoreTest()

	fmt.Printf("\n=== Rely-SQLite benchmark completed ===\n\n")
}

// wipeDatabase recreates the database for a clean slate
func (rsb *RelySQLiteBenchmark) wipeDatabase() {
	// Close existing database
	if rsb.database != nil {
		rsb.database.Close()
	}

	// Remove database file
	if rsb.dbPath != "" {
		os.Remove(rsb.dbPath)
	}

	// Recreate database
	wrapper, err := NewRelySQLiteWrapper(rsb.dbPath)
	if err != nil {
		log.Printf("Failed to recreate database: %v", err)
		return
	}

	rsb.database = wrapper
	rsb.bench.db = wrapper
}

// GenerateReport generates the benchmark report
func (rsb *RelySQLiteBenchmark) GenerateReport() {
	rsb.bench.GenerateReport()
}

// GenerateAsciidocReport generates asciidoc format report
func (rsb *RelySQLiteBenchmark) GenerateAsciidocReport() {
	rsb.bench.GenerateAsciidocReport()
}
