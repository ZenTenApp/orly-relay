package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

// ConfigFile is the JSON structure for persistent configuration.
type ConfigFile struct {
	DBBackend    string   `json:"db_backend,omitempty"`
	DBBinary     string   `json:"db_binary,omitempty"`
	RelayBinary  string   `json:"relay_binary,omitempty"`
	ACLBinary    string   `json:"acl_binary,omitempty"`
	DBListen     string   `json:"db_listen,omitempty"`
	ACLListen    string   `json:"acl_listen,omitempty"`
	ACLEnabled   *bool    `json:"acl_enabled,omitempty"`
	ACLMode      string   `json:"acl_mode,omitempty"`
	DataDir      string   `json:"data_dir,omitempty"`
	LogLevel     string   `json:"log_level,omitempty"`
	AdminPort    *int     `json:"admin_port,omitempty"`
	AdminOwners  []string `json:"admin_owners,omitempty"`
	BinDir       string   `json:"bin_dir,omitempty"`
	RelayPort    *int     `json:"relay_port,omitempty"`
	RelayHost    string   `json:"relay_host,omitempty"`
	TLSDomains   string   `json:"tls_domains,omitempty"`
	AuthToWrite  *bool    `json:"auth_to_write,omitempty"`
	AuthRequired *bool    `json:"auth_required,omitempty"`

	// Sync services
	DistributedSyncEnabled *bool  `json:"distributed_sync_enabled,omitempty"`
	ClusterSyncEnabled     *bool  `json:"cluster_sync_enabled,omitempty"`
	RelayGroupEnabled      *bool  `json:"relay_group_enabled,omitempty"`
	NegentropyEnabled      *bool  `json:"negentropy_enabled,omitempty"`
	NegentropyBinary       string `json:"negentropy_binary,omitempty"`
	NegentropyListen       string `json:"negentropy_listen,omitempty"`
}

// configFilePath returns the path to the config file.
func configFilePath() string {
	return filepath.Join(xdg.ConfigHome, "orly", "launcher.json")
}

// loadConfigFile loads configuration from the JSON file if it exists.
func loadConfigFile() (*ConfigFile, error) {
	path := configFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConfigFile{}, nil
		}
		return nil, err
	}

	var cf ConfigFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, err
	}
	return &cf, nil
}

// SaveConfigFile saves the configuration to the JSON file.
func SaveConfigFile(cf *ConfigFile) error {
	path := configFilePath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ConfigToFile converts a Config to a ConfigFile for persistence.
func ConfigToFile(cfg *Config) *ConfigFile {
	return &ConfigFile{
		DBBackend:              cfg.DBBackend,
		DBBinary:               cfg.DBBinary,
		RelayBinary:            cfg.RelayBinary,
		ACLBinary:              cfg.ACLBinary,
		DBListen:               cfg.DBListen,
		ACLListen:              cfg.ACLListen,
		ACLEnabled:             &cfg.ACLEnabled,
		ACLMode:                cfg.ACLMode,
		DataDir:                cfg.DataDir,
		LogLevel:               cfg.LogLevel,
		AdminPort:              &cfg.AdminPort,
		AdminOwners:            cfg.AdminOwners,
		BinDir:                 cfg.BinDir,
		DistributedSyncEnabled: &cfg.DistributedSyncEnabled,
		ClusterSyncEnabled:     &cfg.ClusterSyncEnabled,
		RelayGroupEnabled:      &cfg.RelayGroupEnabled,
		NegentropyEnabled:      &cfg.NegentropyEnabled,
		NegentropyBinary:       cfg.NegentropyBinary,
		NegentropyListen:       cfg.NegentropyListen,
	}
}

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
	// Load config file first (provides defaults)
	cf, err := loadConfigFile()
	if err != nil {
		// Log but don't fail - env vars are still valid
		cf = &ConfigFile{}
	}

	// Get backend and mode - file first, then env
	dbBackend := stringOr(cf.DBBackend, getEnvOrDefault("ORLY_LAUNCHER_DB_BACKEND", "badger"))
	aclMode := stringOr(cf.ACLMode, getEnvOrDefault("ORLY_ACL_MODE", "follows"))

	// Compute default binary names based on backend/mode
	defaultDBBinary := "orly-db-" + dbBackend
	defaultACLBinary := "orly-acl-" + aclMode

	// Parse admin owners - env takes precedence, then file
	envOwners := getEnvOrDefault("ORLY_LAUNCHER_OWNERS", "")
	var adminOwners []string
	if envOwners != "" {
		adminOwners = parseOwnersList(envOwners)
	} else if len(cf.AdminOwners) > 0 {
		adminOwners = cf.AdminOwners
	}

	cfg := &Config{
		DBBackend:       dbBackend,
		DBBinary:        envOrFileOrDefault("ORLY_LAUNCHER_DB_BINARY", cf.DBBinary, defaultDBBinary),
		RelayBinary:     envOrFileOrDefault("ORLY_LAUNCHER_RELAY_BINARY", cf.RelayBinary, "orly"),
		ACLBinary:       envOrFileOrDefault("ORLY_LAUNCHER_ACL_BINARY", cf.ACLBinary, defaultACLBinary),
		DBListen:        envOrFileOrDefault("ORLY_LAUNCHER_DB_LISTEN", cf.DBListen, "127.0.0.1:50051"),
		ACLListen:       envOrFileOrDefault("ORLY_LAUNCHER_ACL_LISTEN", cf.ACLListen, "127.0.0.1:50052"),
		ACLEnabled:      boolEnvOrFile("ORLY_LAUNCHER_ACL_ENABLED", cf.ACLEnabled, false),
		ACLMode:         aclMode,
		DBReadyTimeout:  parseDuration("ORLY_LAUNCHER_DB_READY_TIMEOUT", 30*time.Second),
		ACLReadyTimeout: parseDuration("ORLY_LAUNCHER_ACL_READY_TIMEOUT", 120*time.Second),
		StopTimeout:     parseDuration("ORLY_LAUNCHER_STOP_TIMEOUT", 30*time.Second),
		DataDir:         envOrFileOrDefault("ORLY_DATA_DIR", cf.DataDir, filepath.Join(xdg.DataHome, "ORLY")),
		LogLevel:        envOrFileOrDefault("ORLY_LOG_LEVEL", cf.LogLevel, "info"),

		// Sync services configuration
		DistributedSyncEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_DISTRIBUTED_ENABLED", cf.DistributedSyncEnabled, false),
		DistributedSyncBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_DISTRIBUTED_BINARY", "orly-sync-distributed"),
		DistributedSyncListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_DISTRIBUTED_LISTEN", "127.0.0.1:50061"),

		ClusterSyncEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_CLUSTER_ENABLED", cf.ClusterSyncEnabled, false),
		ClusterSyncBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_CLUSTER_BINARY", "orly-sync-cluster"),
		ClusterSyncListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_CLUSTER_LISTEN", "127.0.0.1:50062"),

		RelayGroupEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_RELAYGROUP_ENABLED", cf.RelayGroupEnabled, false),
		RelayGroupBinary:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_RELAYGROUP_BINARY", "orly-sync-relaygroup"),
		RelayGroupListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_RELAYGROUP_LISTEN", "127.0.0.1:50063"),

		NegentropyEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED", cf.NegentropyEnabled, false),
		NegentropyBinary:  envOrFileOrDefault("ORLY_LAUNCHER_SYNC_NEGENTROPY_BINARY", cf.NegentropyBinary, "orly-sync-negentropy"),
		NegentropyListen:  envOrFileOrDefault("ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN", cf.NegentropyListen, "127.0.0.1:50064"),

		SyncReadyTimeout: parseDuration("ORLY_LAUNCHER_SYNC_READY_TIMEOUT", 30*time.Second),

		// Admin UI configuration
		AdminEnabled: getEnvOrDefault("ORLY_LAUNCHER_ADMIN_ENABLED", "true") == "true",
		AdminPort:    intEnvOrFile("ORLY_LAUNCHER_ADMIN_PORT", cf.AdminPort, 8080),
		AdminOwners:  adminOwners,
		BinDir:       envOrFileOrDefault("ORLY_LAUNCHER_BIN_DIR", cf.BinDir, filepath.Join(xdg.DataHome, "orly", "bin")),
	}

	return cfg, nil
}

// stringOr returns the first non-empty string.
func stringOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// envOrFileOrDefault returns env var if set, then file value if set, then default.
func envOrFileOrDefault(envKey, fileValue, defaultValue string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if fileValue != "" {
		return fileValue
	}
	return defaultValue
}

// boolEnvOrFile returns env var if set, then file value if set, then default.
func boolEnvOrFile(envKey string, fileValue *bool, defaultValue bool) bool {
	if v := os.Getenv(envKey); v != "" {
		return v == "true"
	}
	if fileValue != nil {
		return *fileValue
	}
	return defaultValue
}

// intEnvOrFile returns env var if set, then file value if set, then default.
func intEnvOrFile(envKey string, fileValue *int, defaultValue int) int {
	if v := os.Getenv(envKey); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	if fileValue != nil {
		return *fileValue
	}
	return defaultValue
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
