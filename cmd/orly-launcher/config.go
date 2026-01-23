package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

// Config holds the launcher configuration.
type Config struct {
	// DBBackend is the database backend: badger or neo4j
	DBBackend string

	// DBBinary is the path to the database server binary (computed from DBBackend if not set)
	DBBinary string

	// RelayBinary is the path to the orly binary
	RelayBinary string

	// ACLBinary is the path to the ACL server binary (computed from ACLMode if not set)
	ACLBinary string

	// DBListen is the address the database server listens on
	DBListen string

	// ACLListen is the address the ACL server listens on
	ACLListen string

	// ACLEnabled controls whether to run the ACL server as a separate process
	// When false, the relay runs in open mode (no ACL restrictions)
	ACLEnabled bool

	// ACLMode is the ACL mode: follows, managed, curation
	// Determines which ACL binary to use when ACLEnabled is true
	ACLMode string

	// DBReadyTimeout is how long to wait for the database to be ready
	DBReadyTimeout time.Duration

	// ACLReadyTimeout is how long to wait for the ACL server to be ready
	ACLReadyTimeout time.Duration

	// StopTimeout is how long to wait for processes to stop gracefully
	StopTimeout time.Duration

	// DataDir is the data directory to pass to orly-db
	DataDir string

	// LogLevel is the log level to use for all processes
	LogLevel string

	// Sync service configuration
	// DistributedSyncEnabled enables the distributed sync service
	DistributedSyncEnabled bool
	// DistributedSyncBinary is the path to the distributed sync binary
	DistributedSyncBinary string
	// DistributedSyncListen is the gRPC listen address for distributed sync
	DistributedSyncListen string

	// ClusterSyncEnabled enables the cluster sync service
	ClusterSyncEnabled bool
	// ClusterSyncBinary is the path to the cluster sync binary
	ClusterSyncBinary string
	// ClusterSyncListen is the gRPC listen address for cluster sync
	ClusterSyncListen string

	// RelayGroupEnabled enables the relay group service
	RelayGroupEnabled bool
	// RelayGroupBinary is the path to the relay group binary
	RelayGroupBinary string
	// RelayGroupListen is the gRPC listen address for relay group
	RelayGroupListen string

	// NegentropyEnabled enables the negentropy sync service
	NegentropyEnabled bool
	// NegentropyBinary is the path to the negentropy sync binary
	NegentropyBinary string
	// NegentropyListen is the gRPC listen address for negentropy
	NegentropyListen string

	// SyncReadyTimeout is how long to wait for sync services to be ready
	SyncReadyTimeout time.Duration

	// Admin UI configuration
	// AdminEnabled controls whether to run the admin HTTP server
	AdminEnabled bool
	// AdminPort is the port for the admin HTTP server
	AdminPort int
	// AdminOwners is a list of pubkeys (hex) allowed to access the admin UI
	AdminOwners []string
	// BinDir is the directory for versioned binary management
	BinDir string
}

func loadConfig() (*Config, error) {
	// Get backend and mode first to compute default binary names
	dbBackend := getEnvOrDefault("ORLY_LAUNCHER_DB_BACKEND", "badger")
	aclMode := getEnvOrDefault("ORLY_ACL_MODE", "follows")

	// Compute default binary names based on backend/mode
	defaultDBBinary := "orly-db-" + dbBackend
	defaultACLBinary := "orly-acl-" + aclMode

	// Parse admin owners (comma-separated hex pubkeys)
	adminOwners := parseOwnersList(getEnvOrDefault("ORLY_LAUNCHER_OWNERS", ""))

	cfg := &Config{
		DBBackend:       dbBackend,
		DBBinary:        getEnvOrDefault("ORLY_LAUNCHER_DB_BINARY", defaultDBBinary),
		RelayBinary:     getEnvOrDefault("ORLY_LAUNCHER_RELAY_BINARY", "orly"),
		ACLBinary:       getEnvOrDefault("ORLY_LAUNCHER_ACL_BINARY", defaultACLBinary),
		DBListen:        getEnvOrDefault("ORLY_LAUNCHER_DB_LISTEN", "127.0.0.1:50051"),
		ACLListen:       getEnvOrDefault("ORLY_LAUNCHER_ACL_LISTEN", "127.0.0.1:50052"),
		ACLEnabled:      getEnvOrDefault("ORLY_LAUNCHER_ACL_ENABLED", "false") == "true",
		ACLMode:         aclMode,
		DBReadyTimeout:  parseDuration("ORLY_LAUNCHER_DB_READY_TIMEOUT", 30*time.Second),
		ACLReadyTimeout: parseDuration("ORLY_LAUNCHER_ACL_READY_TIMEOUT", 120*time.Second),
		StopTimeout:     parseDuration("ORLY_LAUNCHER_STOP_TIMEOUT", 30*time.Second), // Increased for DB flush
		DataDir:         getEnvOrDefault("ORLY_DATA_DIR", filepath.Join(xdg.DataHome, "ORLY")),
		LogLevel:        getEnvOrDefault("ORLY_LOG_LEVEL", "info"),

		// Sync services configuration
		DistributedSyncEnabled: getEnvOrDefault("ORLY_LAUNCHER_SYNC_DISTRIBUTED_ENABLED", "false") == "true",
		DistributedSyncBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_DISTRIBUTED_BINARY", "orly-sync-distributed"),
		DistributedSyncListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_DISTRIBUTED_LISTEN", "127.0.0.1:50061"),

		ClusterSyncEnabled: getEnvOrDefault("ORLY_LAUNCHER_SYNC_CLUSTER_ENABLED", "false") == "true",
		ClusterSyncBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_CLUSTER_BINARY", "orly-sync-cluster"),
		ClusterSyncListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_CLUSTER_LISTEN", "127.0.0.1:50062"),

		RelayGroupEnabled: getEnvOrDefault("ORLY_LAUNCHER_SYNC_RELAYGROUP_ENABLED", "false") == "true",
		RelayGroupBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_RELAYGROUP_BINARY", "orly-sync-relaygroup"),
		RelayGroupListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_RELAYGROUP_LISTEN", "127.0.0.1:50063"),

		NegentropyEnabled: getEnvOrDefault("ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED", "false") == "true",
		NegentropyBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY", "orly-sync-negentropy"),
		NegentropyListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN", "127.0.0.1:50064"),

		SyncReadyTimeout: parseDuration("ORLY_LAUNCHER_SYNC_READY_TIMEOUT", 30*time.Second),

		// Admin UI configuration
		AdminEnabled: getEnvOrDefault("ORLY_LAUNCHER_ADMIN_ENABLED", "true") == "true",
		AdminPort:    parseInt("ORLY_LAUNCHER_ADMIN_PORT", 8080),
		AdminOwners:  adminOwners,
		BinDir:       getEnvOrDefault("ORLY_LAUNCHER_BIN_DIR", filepath.Join(xdg.DataHome, "orly", "bin")),
	}

	return cfg, nil
}

func parseOwnersList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var owners []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			owners = append(owners, p)
		}
	}
	return owners
}

func parseInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
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
