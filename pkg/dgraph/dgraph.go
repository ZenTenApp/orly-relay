// Package dgraph provides a Dgraph-based implementation of the database interface.
// This is a simplified implementation for testing - full dgraph integration to be completed later.
package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/dgo/v230"
	"github.com/dgraph-io/dgo/v230/protos/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"lol.mleku.dev"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/utils/apputil"
)

// D implements the database.Database interface using Dgraph as the storage backend
type D struct {
	ctx     context.Context
	cancel  context.CancelFunc
	dataDir string
	Logger  *logger

	// Dgraph client connection
	client *dgo.Dgraph
	conn   *grpc.ClientConn

	// Configuration
	dgraphURL           string
	enableGraphQL       bool
	enableIntrospection bool

	ready chan struct{} // Closed when database is ready to serve requests
}

// Ensure D implements database.Database interface at compile time
var _ database.Database = (*D)(nil)

// init registers the dgraph database factory
func init() {
	database.RegisterDgraphFactory(func(
		ctx context.Context,
		cancel context.CancelFunc,
		dataDir string,
		logLevel string,
	) (database.Database, error) {
		return New(ctx, cancel, dataDir, logLevel)
	})
}

// Config holds configuration options for the Dgraph database
type Config struct {
	DataDir             string
	LogLevel            string
	DgraphURL           string // Dgraph gRPC endpoint (e.g., "localhost:9080")
	EnableGraphQL       bool
	EnableIntrospection bool
}

// New creates a new Dgraph-based database instance
func New(
	ctx context.Context, cancel context.CancelFunc, dataDir, logLevel string,
) (
	d *D, err error,
) {
	// Get dgraph URL from environment, default to localhost
	dgraphURL := os.Getenv("ORLY_DGRAPH_URL")
	if dgraphURL == "" {
		dgraphURL = "localhost:9080"
	}

	d = &D{
		ctx:                 ctx,
		cancel:              cancel,
		dataDir:             dataDir,
		Logger:              NewLogger(lol.GetLogLevel(logLevel), dataDir),
		dgraphURL:           dgraphURL,
		enableGraphQL:       false,
		enableIntrospection: false,
		ready:               make(chan struct{}),
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

	// Initialize dgraph client connection
	if err = d.initDgraphClient(); chk.E(err) {
		return
	}

	// Apply Nostr schema to dgraph
	if err = d.applySchema(ctx); chk.E(err) {
		return
	}

	// Initialize serial counter
	if err = d.initSerialCounter(); chk.E(err) {
		return
	}

	// Start warmup goroutine to signal when database is ready
	go d.warmup()

	// Setup shutdown handler
	go func() {
		<-d.ctx.Done()
		d.cancel()
		if d.conn != nil {
			d.conn.Close()
		}
	}()

	return
}

// initDgraphClient establishes connection to dgraph server
func (d *D) initDgraphClient() error {
	d.Logger.Infof("connecting to dgraph at %s", d.dgraphURL)

	// Establish gRPC connection
	conn, err := grpc.Dial(d.dgraphURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to dgraph at %s: %w", d.dgraphURL, err)
	}

	d.conn = conn
	d.client = dgo.NewDgraphClient(api.NewDgraphClient(conn))

	d.Logger.Infof("successfully connected to dgraph")
	return nil
}


// Query executes a DQL query against dgraph
func (d *D) Query(ctx context.Context, query string) (*api.Response, error) {
	txn := d.client.NewReadOnlyTxn()
	defer txn.Discard(ctx)

	resp, err := txn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("dgraph query failed: %w", err)
	}

	return resp, nil
}

// Mutate executes a mutation against dgraph
func (d *D) Mutate(ctx context.Context, mutation *api.Mutation) (*api.Response, error) {
	txn := d.client.NewTxn()
	defer txn.Discard(ctx)

	resp, err := txn.Mutate(ctx, mutation)
	if err != nil {
		return nil, fmt.Errorf("dgraph mutation failed: %w", err)
	}

	// Only commit if CommitNow is false (mutation didn't auto-commit)
	if !mutation.CommitNow {
		if err := txn.Commit(ctx); err != nil {
			return nil, fmt.Errorf("dgraph commit failed: %w", err)
		}
	}

	return resp, nil
}

// Path returns the data directory path
func (d *D) Path() string { return d.dataDir }

// Init initializes the database with a given path (no-op, path set in New)
func (d *D) Init(path string) (err error) {
	// Path already set in New()
	return nil
}

// Sync flushes pending writes (DGraph handles persistence automatically)
func (d *D) Sync() (err error) {
	return nil
}

// Close closes the database
func (d *D) Close() (err error) {
	d.cancel()
	if d.conn != nil {
		if e := d.conn.Close(); e != nil {
			err = e
		}
	}
	return
}

// Wipe removes all data
func (d *D) Wipe() (err error) {
	// Drop all data in DGraph using Alter
	op := &api.Operation{
		DropOp: api.Operation_DATA,
	}

	if err = d.client.Alter(context.Background(), op); err != nil {
		return fmt.Errorf("failed to drop dgraph data: %w", err)
	}

	// Remove data directory
	if err = os.RemoveAll(d.dataDir); chk.E(err) {
		return
	}

	return nil
}

// SetLogLevel sets the logging level
func (d *D) SetLogLevel(level string) {
	// d.Logger.SetLevel(lol.GetLogLevel(level))
}

// EventIdsBySerial retrieves event IDs by serial range
func (d *D) EventIdsBySerial(start uint64, count int) (
	evs []uint64, err error,
) {
	// Query for events in the specified serial range
	query := fmt.Sprintf(`{
		events(func: ge(event.serial, %d), orderdesc: event.serial, first: %d) {
			event.serial
		}
	}`, start, count)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query event IDs by serial: %w", err)
	}

	var result struct {
		Events []struct {
			Serial int64 `json:"event.serial"`
		} `json:"events"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	evs = make([]uint64, 0, len(result.Events))
	for _, ev := range result.Events {
		evs = append(evs, uint64(ev.Serial))
	}

	return evs, nil
}

// RunMigrations runs database migrations (no-op for dgraph)
func (d *D) RunMigrations() {
	// No-op for dgraph
}

// Ready returns a channel that closes when the database is ready to serve requests.
// This allows callers to wait for database warmup to complete.
func (d *D) Ready() <-chan struct{} {
	return d.ready
}

// warmup performs database warmup operations and closes the ready channel when complete.
// For Dgraph, warmup ensures the connection is healthy and schema is applied.
func (d *D) warmup() {
	defer close(d.ready)

	// Dgraph connection and schema are already verified during initialization
	// Just give a brief moment for any background processes to settle
	d.Logger.Infof("dgraph database warmup complete, ready to serve requests")
}
func (d *D) GetCachedJSON(f *filter.F) ([][]byte, bool)              { return nil, false }
func (d *D) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte)  {}
func (d *D) GetCachedEvents(f *filter.F) (event.S, bool)             { return nil, false }
func (d *D) CacheEvents(f *filter.F, events event.S)                 {}
func (d *D) InvalidateQueryCache()                                    {}
