// Package server provides a shared gRPC database server implementation.
package server

import "time"

// Config holds configuration for the database gRPC server.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string

	// LogLevel is the logging level
	LogLevel string

	// StreamBatchSize is the number of events per stream batch
	StreamBatchSize int

	// MaxConcurrentQueries is the max concurrent queries
	MaxConcurrentQueries int
}

// DatabaseConfig holds Badger-specific configuration.
type DatabaseConfig struct {
	DataDir             string
	LogLevel            string
	BlockCacheMB        int
	IndexCacheMB        int
	ZSTDLevel           int
	QueryCacheSizeMB    int
	QueryCacheMaxAge    time.Duration
	QueryCacheDisabled  bool
	SerialCachePubkeys  int
	SerialCacheEventIds int
}
