// Package neo4j provides a Neo4j-based implementation of the database interface.
// Neo4j is a native graph database optimized for relationship-heavy queries,
// making it ideal for Nostr's social graph and event reference patterns.
package neo4j

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"next.orly.dev/pkg/utils/apputil"
)

// N implements the database.Database interface using Neo4j as the storage backend
type N struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dataDir string
	Logger  *logger

	// Neo4j client connection
	driver neo4j.DriverWithContext

	// Configuration
	neo4jURI      string
	neo4jUser     string
	neo4jPassword string

	ready chan struct{} // Closed when database is ready to serve requests
}

// Ensure N implements database.Database interface at compile time
var _ database.Database = (*N)(nil)

// CollectedResult wraps pre-fetched Neo4j records for iteration after session close
// This is necessary because Neo4j results are lazy and need an open session for iteration
type CollectedResult struct {
	records []*neo4j.Record
	index   int
}

// Next advances to the next record, returning true if there is one
func (r *CollectedResult) Next(ctx context.Context) bool {
	r.index++
	return r.index < len(r.records)
}

// Record returns the current record
func (r *CollectedResult) Record() *neo4j.Record {
	if r.index < 0 || r.index >= len(r.records) {
		return nil
	}
	return r.records[r.index]
}

// Len returns the number of records
func (r *CollectedResult) Len() int {
	return len(r.records)
}

// Err returns any error from iteration (always nil for pre-collected results)
// This method satisfies the resultiter.Neo4jResultIterator interface
func (r *CollectedResult) Err() error {
	return nil
}

// init registers the neo4j database factory
func init() {
	database.RegisterNeo4jFactory(func(
		ctx context.Context,
		cancel context.CancelFunc,
		cfg *database.DatabaseConfig,
	) (database.Database, error) {
		return NewWithConfig(ctx, cancel, cfg)
	})
}

// NewWithConfig creates a new Neo4j-based database instance with full configuration.
// Configuration is passed from the centralized app config via DatabaseConfig.
func NewWithConfig(
	ctx context.Context, cancel context.CancelFunc, cfg *database.DatabaseConfig,
) (
	n *N, err error,
) {
	// Apply defaults for empty values
	neo4jURI := cfg.Neo4jURI
	if neo4jURI == "" {
		neo4jURI = "bolt://localhost:7687"
	}
	neo4jUser := cfg.Neo4jUser
	if neo4jUser == "" {
		neo4jUser = "neo4j"
	}
	neo4jPassword := cfg.Neo4jPassword
	if neo4jPassword == "" {
		neo4jPassword = "password"
	}

	n = &N{
		ctx:           ctx,
		cancel:        cancel,
		dataDir:       cfg.DataDir,
		Logger:        NewLogger(lol.GetLogLevel(cfg.LogLevel), cfg.DataDir),
		neo4jURI:      neo4jURI,
		neo4jUser:     neo4jUser,
		neo4jPassword: neo4jPassword,
		ready:         make(chan struct{}),
	}

	// Ensure the data directory exists
	if err = os.MkdirAll(cfg.DataDir, 0755); chk.E(err) {
		return
	}

	// Ensure directory structure
	dummyFile := filepath.Join(cfg.DataDir, "dummy.sst")
	if err = apputil.EnsureDir(dummyFile); chk.E(err) {
		return
	}

	// Initialize neo4j client connection
	if err = n.initNeo4jClient(); chk.E(err) {
		return
	}

	// Apply Nostr schema to neo4j (create constraints and indexes)
	if err = n.applySchema(ctx); chk.E(err) {
		return
	}

	// Run database migrations (e.g., Author -> NostrUser consolidation)
	n.RunMigrations()

	// Initialize serial counter
	if err = n.initSerialCounter(); chk.E(err) {
		return
	}

	// Start warmup goroutine to signal when database is ready
	go n.warmup()

	// Setup shutdown handler
	go func() {
		<-n.ctx.Done()
		n.cancel()
		if n.driver != nil {
			n.driver.Close(context.Background())
		}
	}()

	return
}

// New creates a new Neo4j-based database instance with default configuration.
// This is provided for backward compatibility with existing callers (tests, etc.).
// For full configuration control, use NewWithConfig instead.
func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (
	n *N, err error,
) {
	cfg := &database.DatabaseConfig{
		DataDir:  dataDir,
		LogLevel: logLevel,
	}
	return NewWithConfig(ctx, cancel, cfg)
}

// initNeo4jClient establishes connection to Neo4j server
func (n *N) initNeo4jClient() error {
	n.Logger.Infof("connecting to neo4j at %s", n.neo4jURI)

	// Create Neo4j driver
	driver, err := neo4j.NewDriverWithContext(
		n.neo4jURI,
		neo4j.BasicAuth(n.neo4jUser, n.neo4jPassword, ""),
	)
	if err != nil {
		return fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	n.driver = driver

	// Verify connectivity
	ctx := context.Background()
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("failed to verify neo4j connectivity: %w", err)
	}

	n.Logger.Infof("successfully connected to neo4j")
	return nil
}


// ExecuteRead executes a read query against Neo4j
// Returns a collected result that can be iterated after the session closes
func (n *N) ExecuteRead(ctx context.Context, cypher string, params map[string]any) (*CollectedResult, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("neo4j read query failed: %w", err)
	}

	// Collect all records before the session closes
	// (Neo4j results are lazy and need an open session for iteration)
	records, err := result.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("neo4j result collect failed: %w", err)
	}

	return &CollectedResult{records: records, index: -1}, nil
}

// ExecuteWrite executes a write query against Neo4j
func (n *N) ExecuteWrite(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("neo4j write query failed: %w", err)
	}

	return result, nil
}

// ExecuteWriteTransaction executes a transactional write operation
func (n *N) ExecuteWriteTransaction(ctx context.Context, work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	return session.ExecuteWrite(ctx, work)
}

// Path returns the data directory path
func (n *N) Path() string { return n.dataDir }

// Init initializes the database with a given path (no-op, path set in New)
func (n *N) Init(path string) (err error) {
	// Path already set in New()
	return nil
}

// Sync flushes pending writes (Neo4j handles persistence automatically)
func (n *N) Sync() (err error) {
	return nil
}

// Close closes the database
func (n *N) Close() (err error) {
	n.cancel()
	if n.driver != nil {
		if e := n.driver.Close(context.Background()); e != nil {
			err = e
		}
	}
	return
}

// Wipe removes all data and re-applies schema
func (n *N) Wipe() (err error) {
	// Delete all nodes and relationships in Neo4j
	ctx := context.Background()
	_, err = n.ExecuteWrite(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("failed to wipe neo4j database: %w", err)
	}

	// Re-apply schema (indexes and constraints were deleted with the data)
	if err = n.applySchema(ctx); err != nil {
		return fmt.Errorf("failed to re-apply schema after wipe: %w", err)
	}

	// Re-initialize serial counter (it was deleted with the Marker node)
	if err = n.initSerialCounter(); err != nil {
		return fmt.Errorf("failed to re-init serial counter after wipe: %w", err)
	}

	return nil
}

// SetLogLevel sets the logging level
func (n *N) SetLogLevel(level string) {
	// n.Logger.SetLevel(lol.GetLogLevel(level))
}

// EventIdsBySerial retrieves event IDs by serial range (stub)
func (n *N) EventIdsBySerial(start uint64, count int) (
	evs []uint64, err error,
) {
	err = fmt.Errorf("not implemented")
	return
}

// RunMigrations is implemented in migrations.go
// It handles schema migrations like the Author -> NostrUser consolidation

// Ready returns a channel that closes when the database is ready to serve requests.
// This allows callers to wait for database warmup to complete.
func (n *N) Ready() <-chan struct{} {
	return n.ready
}

// warmup performs database warmup operations and closes the ready channel when complete.
// For Neo4j, warmup ensures the connection is healthy and constraints are applied.
func (n *N) warmup() {
	defer close(n.ready)

	// Neo4j connection and schema are already verified during initialization
	// Just give a brief moment for any background processes to settle
	n.Logger.Infof("neo4j database warmup complete, ready to serve requests")
}

// GetCachedJSON returns cached query results (not implemented for Neo4j)
func (n *N) GetCachedJSON(f *filter.F) ([][]byte, bool) { return nil, false }

// CacheMarshaledJSON caches marshaled JSON results (not implemented for Neo4j)
func (n *N) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte) {}

// GetCachedEvents retrieves cached events (not implemented for Neo4j)
func (n *N) GetCachedEvents(f *filter.F) (event.S, bool) { return nil, false }

// CacheEvents caches events (not implemented for Neo4j)
func (n *N) CacheEvents(f *filter.F, events event.S) {}

// InvalidateQueryCache invalidates the query cache (not implemented for Neo4j)
func (n *N) InvalidateQueryCache() {}
