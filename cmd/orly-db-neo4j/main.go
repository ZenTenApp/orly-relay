// orly-db-neo4j is a standalone gRPC database server using the Neo4j backend.
package main

import (
	"context"
	"os"
	"time"

	"go-simpler.org/env"
	"git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/database/server"

	// Import neo4j to register the factory
	_ "git.smesh.lol/orly/pkg/neo4j"
)

// Config holds the database server configuration.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string `env:"ORLY_DB_LISTEN" default:"127.0.0.1:50051" usage:"gRPC server listen address"`

	// LogLevel is the logging level
	LogLevel string `env:"ORLY_DB_LOG_LEVEL" default:"info" usage:"log level (trace, debug, info, warn, error)"`

	// DataDir is required by the database layer for metadata storage
	DataDir string `env:"ORLY_DATA_DIR" default:"/tmp/orly-neo4j" usage:"data directory for metadata"`

	// Neo4j configuration
	Neo4jURI      string `env:"ORLY_NEO4J_URI" default:"bolt://localhost:7687" usage:"Neo4j connection URI"`
	Neo4jUser     string `env:"ORLY_NEO4J_USER" default:"neo4j" usage:"Neo4j username"`
	Neo4jPassword string `env:"ORLY_NEO4J_PASSWORD" usage:"Neo4j password"`

	// Neo4j driver tuning
	Neo4jMaxConnPoolSize   int `env:"ORLY_NEO4J_MAX_CONN_POOL" default:"25" usage:"max connection pool size"`
	Neo4jFetchSize         int `env:"ORLY_NEO4J_FETCH_SIZE" default:"1000" usage:"max records per fetch batch"`
	Neo4jMaxTxRetrySeconds int `env:"ORLY_NEO4J_MAX_TX_RETRY_SEC" default:"30" usage:"max transaction retry time"`
	Neo4jQueryResultLimit  int `env:"ORLY_NEO4J_QUERY_RESULT_LIMIT" default:"10000" usage:"max results per query (0=unlimited)"`

	// Query cache configuration (for the gRPC server)
	QueryCacheSizeMB   int           `env:"ORLY_DB_QUERY_CACHE_SIZE_MB" default:"256" usage:"query cache size in MB"`
	QueryCacheMaxAge   time.Duration `env:"ORLY_DB_QUERY_CACHE_MAX_AGE" default:"5m" usage:"query cache max age"`
	QueryCacheDisabled bool          `env:"ORLY_DB_QUERY_CACHE_DISABLED" default:"false" usage:"disable query cache"`

	// gRPC server configuration
	StreamBatchSize int `env:"ORLY_DB_STREAM_BATCH_SIZE" default:"100" usage:"events per stream batch"`
}

func main() {
	cfg := loadConfig()

	// Set log level
	lol.SetLogLevel(cfg.LogLevel)
	log.I.F("orly-db-neo4j starting with log level: %s", cfg.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create database configuration
	dbCfg := &database.DatabaseConfig{
		DataDir:              cfg.DataDir,
		LogLevel:             cfg.LogLevel,
		Neo4jURI:             cfg.Neo4jURI,
		Neo4jUser:            cfg.Neo4jUser,
		Neo4jPassword:        cfg.Neo4jPassword,
		Neo4jMaxConnPoolSize: cfg.Neo4jMaxConnPoolSize,
		Neo4jFetchSize:       cfg.Neo4jFetchSize,
		Neo4jMaxTxRetrySeconds: cfg.Neo4jMaxTxRetrySeconds,
		Neo4jQueryResultLimit:  cfg.Neo4jQueryResultLimit,
		QueryCacheSizeMB:     cfg.QueryCacheSizeMB,
		QueryCacheMaxAge:     cfg.QueryCacheMaxAge,
		QueryCacheDisabled:   cfg.QueryCacheDisabled,
	}

	// Initialize Neo4j database via factory
	log.I.F("connecting to Neo4j at %s", cfg.Neo4jURI)
	db, err := database.NewDatabaseWithConfig(ctx, cancel, "neo4j", dbCfg)
	if chk.E(err) {
		log.E.F("failed to initialize Neo4j database: %v", err)
		os.Exit(1)
	}

	// Wait for database to be ready
	log.I.F("waiting for database to be ready...")
	<-db.Ready()
	log.I.F("database ready")

	// Create and start gRPC server
	serverCfg := &server.Config{
		Listen:          cfg.Listen,
		LogLevel:        cfg.LogLevel,
		StreamBatchSize: cfg.StreamBatchSize,
	}

	srv := server.New(db, serverCfg)
	if err := srv.ListenAndServe(ctx, cancel); err != nil {
		log.E.F("gRPC server error: %v", err)
	}
}

func loadConfig() *Config {
	cfg := &Config{}
	if err := env.Load(cfg, nil); chk.E(err) {
		log.E.F("failed to load config: %v", err)
		os.Exit(1)
	}

	return cfg
}
