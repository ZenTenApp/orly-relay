package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

// Config holds the launcher configuration.
type Config struct {
	// DBBinary is the path to the orly-db binary
	DBBinary string

	// RelayBinary is the path to the orly binary
	RelayBinary string

	// DBListen is the address the database server listens on
	DBListen string

	// DBReadyTimeout is how long to wait for the database to be ready
	DBReadyTimeout time.Duration

	// StopTimeout is how long to wait for processes to stop gracefully
	StopTimeout time.Duration

	// DataDir is the data directory to pass to orly-db
	DataDir string

	// LogLevel is the log level to use for both processes
	LogLevel string
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		DBBinary:       getEnvOrDefault("ORLY_LAUNCHER_DB_BINARY", "orly-db"),
		RelayBinary:    getEnvOrDefault("ORLY_LAUNCHER_RELAY_BINARY", "orly"),
		DBListen:       getEnvOrDefault("ORLY_LAUNCHER_DB_LISTEN", "127.0.0.1:50051"),
		DBReadyTimeout: parseDuration("ORLY_LAUNCHER_DB_READY_TIMEOUT", 30*time.Second),
		StopTimeout:    parseDuration("ORLY_LAUNCHER_STOP_TIMEOUT", 30*time.Second), // Increased for DB flush
		DataDir:        getEnvOrDefault("ORLY_DATA_DIR", filepath.Join(xdg.DataHome, "ORLY")),
		LogLevel:       getEnvOrDefault("ORLY_LOG_LEVEL", "info"),
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func parseDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}
