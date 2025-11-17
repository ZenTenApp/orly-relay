// Package neo4j provides a Neo4j-based implementation of the database interface.
// Neo4j is a native graph database optimized for relationship-heavy queries,
// making it ideal for Nostr's social graph and event reference patterns.
package neo4j

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/filter"
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

	// Fallback badger storage for metadata (markers, identity, etc.)
	pstore *badger.DB

	// Configuration
	neo4jURI      string
	neo4jUser     string
	neo4jPassword string

	ready chan struct{} // Closed when database is ready to serve requests
}

// Ensure N implements database.Database interface at compile time
var _ database.Database = (*N)(nil)

// init registers the neo4j database factory
func init() {
	database.RegisterNeo4jFactory(func(
		ctx context.Context,
		cancel context.CancelFunc,
		dataDir string,
		logLevel string,
	) (database.Database, error) {
		return New(ctx, cancel, dataDir, logLevel)
	})
}

// Config holds configuration options for the Neo4j database
type Config struct {
	DataDir       string
	LogLevel      string
	Neo4jURI      string // Neo4j bolt URI (e.g., "bolt://localhost:7687")
	Neo4jUser     string // Authentication username
	Neo4jPassword string // Authentication password
}

// New creates a new Neo4j-based database instance
func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (
	n *N, err error,
) {
	// Get Neo4j connection details from environment
	neo4jURI := os.Getenv("ORLY_NEO4J_URI")
	if neo4jURI == "" {
		neo4jURI = "bolt://localhost:7687"
	}
	neo4jUser := os.Getenv("ORLY_NEO4J_USER")
	if neo4jUser == "" {
		neo4jUser = "neo4j"
	}
	neo4jPassword := os.Getenv("ORLY_NEO4J_PASSWORD")
	if neo4jPassword == "" {
		neo4jPassword = "password"
	}

	n = &N{
		ctx:           ctx,
		cancel:        cancel,
		dataDir:       dataDir,
		Logger:        NewLogger(lol.GetLogLevel(logLevel), dataDir),
		neo4jURI:      neo4jURI,
		neo4jUser:     neo4jUser,
		neo4jPassword: neo4jPassword,
		ready:         make(chan struct{}),
	}

	// Ensure the data directory exists
	if err = os.MkdirAll(dataDir, 0755); chk.E(err) {
		return
	}

	// Ensure directory structure
	dummyFile := filepath.Join(dataDir, "dummy.sst")
	if err = apputil.EnsureDir(dummyFile); chk.E(err) {
		return
	}

	// Initialize neo4j client connection
	if err = n.initNeo4jClient(); chk.E(err) {
		return
	}

	// Initialize badger for metadata storage
	if err = n.initStorage(); chk.E(err) {
		return
	}

	// Apply Nostr schema to neo4j (create constraints and indexes)
	if err = n.applySchema(ctx); chk.E(err) {
		return
	}

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
		if n.pstore != nil {
			n.pstore.Close()
		}
	}()

	return
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

// initStorage opens Badger database for metadata storage
func (n *N) initStorage() error {
	metadataDir := filepath.Join(n.dataDir, "metadata")

	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	opts := badger.DefaultOptions(metadataDir)

	var err error
	n.pstore, err = badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open badger metadata store: %w", err)
	}

	n.Logger.Infof("metadata storage initialized")
	return nil
}

// ExecuteRead executes a read query against Neo4j
func (n *N) ExecuteRead(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.Run(ctx, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("neo4j read query failed: %w", err)
	}

	return result, nil
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

// Sync flushes pending writes
func (n *N) Sync() (err error) {
	if n.pstore != nil {
		return n.pstore.Sync()
	}
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
	if n.pstore != nil {
		if e := n.pstore.Close(); e != nil && err == nil {
			err = e
		}
	}
	return
}

// Wipe removes all data
func (n *N) Wipe() (err error) {
	// Close and remove badger metadata
	if n.pstore != nil {
		if err = n.pstore.Close(); chk.E(err) {
			return
		}
	}
	if err = os.RemoveAll(n.dataDir); chk.E(err) {
		return
	}

	// Delete all nodes and relationships in Neo4j
	ctx := context.Background()
	_, err = n.ExecuteWrite(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("failed to wipe neo4j database: %w", err)
	}

	return n.initStorage()
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

// RunMigrations runs database migrations (no-op for neo4j)
func (n *N) RunMigrations() {
	// No-op for neo4j
}

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

// InvalidateQueryCache invalidates the query cache (not implemented for Neo4j)
func (n *N) InvalidateQueryCache() {}
