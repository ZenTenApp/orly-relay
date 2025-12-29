// Package neo4j provides a Neo4j-based implementation of the database interface.
// Neo4j is a native graph database optimized for relationship-heavy queries,
// making it ideal for Nostr's social graph and event reference patterns.
package neo4j

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"next.orly.dev/pkg/utils/apputil"
)

// Default configuration values (used when config values are 0 or not set)
const (
	defaultMaxConcurrentQueries = 10
	defaultMaxConnPoolSize      = 25
	defaultFetchSize            = 1000
	defaultMaxTxRetrySeconds    = 30
	defaultQueryResultLimit     = 10000
)

// maxRetryAttempts is the maximum number of times to retry a query on rate limit
const maxRetryAttempts = 3

// retryBaseDelay is the base delay for exponential backoff
const retryBaseDelay = 500 * time.Millisecond

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

	// Driver tuning options
	maxConnPoolSize   int // max connections in pool
	fetchSize         int // records per fetch batch
	maxTxRetryTime    time.Duration
	queryResultLimit  int // max results per query (0=unlimited)

	ready chan struct{} // Closed when database is ready to serve requests

	// querySem limits concurrent queries to prevent rate limiting
	querySem chan struct{}
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

	// Apply defaults for driver tuning options
	maxConnPoolSize := cfg.Neo4jMaxConnPoolSize
	if maxConnPoolSize <= 0 {
		maxConnPoolSize = defaultMaxConnPoolSize
	}
	fetchSize := cfg.Neo4jFetchSize
	if fetchSize == 0 {
		fetchSize = defaultFetchSize
	}
	maxTxRetrySeconds := cfg.Neo4jMaxTxRetrySeconds
	if maxTxRetrySeconds <= 0 {
		maxTxRetrySeconds = defaultMaxTxRetrySeconds
	}
	queryResultLimit := cfg.Neo4jQueryResultLimit
	if queryResultLimit == 0 {
		queryResultLimit = defaultQueryResultLimit
	}

	n = &N{
		ctx:              ctx,
		cancel:           cancel,
		dataDir:          cfg.DataDir,
		Logger:           NewLogger(lol.GetLogLevel(cfg.LogLevel), cfg.DataDir),
		neo4jURI:         neo4jURI,
		neo4jUser:        neo4jUser,
		neo4jPassword:    neo4jPassword,
		maxConnPoolSize:  maxConnPoolSize,
		fetchSize:        fetchSize,
		maxTxRetryTime:   time.Duration(maxTxRetrySeconds) * time.Second,
		queryResultLimit: queryResultLimit,
		ready:            make(chan struct{}),
		querySem:         make(chan struct{}, defaultMaxConcurrentQueries),
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
	n.Logger.Infof("connecting to neo4j at %s (pool=%d, fetch=%d, txRetry=%v)",
		n.neo4jURI, n.maxConnPoolSize, n.fetchSize, n.maxTxRetryTime)

	// Create Neo4j driver with tuned configuration
	driver, err := neo4j.NewDriverWithContext(
		n.neo4jURI,
		neo4j.BasicAuth(n.neo4jUser, n.neo4jPassword, ""),
		func(config *neo4j.Config) {
			// Limit connection pool size to reduce memory usage
			config.MaxConnectionPoolSize = n.maxConnPoolSize

			// Set fetch size to batch records and prevent memory overflow
			// -1 means fetch all (driver default), positive value limits batch size
			config.FetchSize = n.fetchSize

			// Set max transaction retry time
			config.MaxTransactionRetryTime = n.maxTxRetryTime
		},
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


// isRateLimitError checks if an error is due to authentication rate limiting
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "AuthenticationRateLimit") ||
		strings.Contains(errStr, "Too many failed authentication attempts")
}

// acquireQuerySlot acquires a slot from the query semaphore
func (n *N) acquireQuerySlot(ctx context.Context) error {
	select {
	case n.querySem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// releaseQuerySlot releases a slot back to the query semaphore
func (n *N) releaseQuerySlot() {
	<-n.querySem
}

// ExecuteRead executes a read query against Neo4j with rate limiting and retry
// Returns a collected result that can be iterated after the session closes
func (n *N) ExecuteRead(ctx context.Context, cypher string, params map[string]any) (*CollectedResult, error) {
	// Acquire semaphore slot to limit concurrent queries
	if err := n.acquireQuerySlot(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire query slot: %w", err)
	}
	defer n.releaseQuerySlot()

	var lastErr error
	for attempt := 0; attempt < maxRetryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := retryBaseDelay * time.Duration(1<<uint(attempt-1))
			n.Logger.Warningf("retrying read query after %v (attempt %d/%d)", delay, attempt+1, maxRetryAttempts)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		session := n.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
		result, err := session.Run(ctx, cypher, params)
		if err != nil {
			session.Close(ctx)
			lastErr = err
			if isRateLimitError(err) {
				continue // Retry on rate limit
			}
			return nil, fmt.Errorf("neo4j read query failed: %w", err)
		}

		// Collect all records before the session closes
		// (Neo4j results are lazy and need an open session for iteration)
		records, err := result.Collect(ctx)
		session.Close(ctx)
		if err != nil {
			lastErr = err
			if isRateLimitError(err) {
				continue // Retry on rate limit
			}
			return nil, fmt.Errorf("neo4j result collect failed: %w", err)
		}

		return &CollectedResult{records: records, index: -1}, nil
	}

	return nil, fmt.Errorf("neo4j read query failed after %d attempts: %w", maxRetryAttempts, lastErr)
}

// ExecuteWrite executes a write query against Neo4j with rate limiting and retry
func (n *N) ExecuteWrite(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	// Acquire semaphore slot to limit concurrent queries
	if err := n.acquireQuerySlot(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire query slot: %w", err)
	}
	defer n.releaseQuerySlot()

	var lastErr error
	for attempt := 0; attempt < maxRetryAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := retryBaseDelay * time.Duration(1<<uint(attempt-1))
			n.Logger.Warningf("retrying write query after %v (attempt %d/%d)", delay, attempt+1, maxRetryAttempts)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		session := n.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
		result, err := session.Run(ctx, cypher, params)
		if err != nil {
			session.Close(ctx)
			lastErr = err
			if isRateLimitError(err) {
				continue // Retry on rate limit
			}
			return nil, fmt.Errorf("neo4j write query failed: %w", err)
		}

		// Consume the result to ensure the query completes before closing session
		_, err = result.Consume(ctx)
		session.Close(ctx)
		if err != nil {
			lastErr = err
			if isRateLimitError(err) {
				continue // Retry on rate limit
			}
			return nil, fmt.Errorf("neo4j write consume failed: %w", err)
		}

		return result, nil
	}

	return nil, fmt.Errorf("neo4j write query failed after %d attempts: %w", maxRetryAttempts, lastErr)
}

// ExecuteWriteTransaction executes a transactional write operation with rate limiting
func (n *N) ExecuteWriteTransaction(ctx context.Context, work func(tx neo4j.ManagedTransaction) (any, error)) (any, error) {
	// Acquire semaphore slot to limit concurrent queries
	if err := n.acquireQuerySlot(ctx); err != nil {
		return nil, fmt.Errorf("failed to acquire query slot: %w", err)
	}
	defer n.releaseQuerySlot()

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

// Driver returns the Neo4j driver for use in rate limiting.
func (n *N) Driver() neo4j.DriverWithContext {
	return n.driver
}

// QuerySem returns the query semaphore for use in rate limiting.
func (n *N) QuerySem() chan struct{} {
	return n.querySem
}

// MaxConcurrentQueries returns the maximum concurrent query limit.
func (n *N) MaxConcurrentQueries() int {
	return cap(n.querySem)
}

// QueryResultLimit returns the configured maximum results per query.
// Returns 0 if unlimited (no limit applied).
func (n *N) QueryResultLimit() int {
	return n.queryResultLimit
}

// FetchSize returns the configured fetch batch size.
func (n *N) FetchSize() int {
	return n.fetchSize
}

// MaxConnPoolSize returns the configured connection pool size.
func (n *N) MaxConnPoolSize() int {
	return n.maxConnPoolSize
}
