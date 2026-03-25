//go:build !(js && wasm)

package launcher

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
	DBDriver   string   `json:"db_driver,omitempty"`
	ACLDriver  string   `json:"acl_driver,omitempty"`
	DBListen   string   `json:"db_listen,omitempty"`
	ACLListen  string   `json:"acl_listen,omitempty"`
	ACLEnabled *bool    `json:"acl_enabled,omitempty"`
	DataDir    string   `json:"data_dir,omitempty"`
	LogLevel   string   `json:"log_level,omitempty"`

	// Admin UI configuration
	AdminPort   *int     `json:"admin_port,omitempty"`
	AdminOwners []string `json:"admin_owners,omitempty"`

	// Relay configuration
	RelayPort    *int   `json:"relay_port,omitempty"`
	RelayHost    string `json:"relay_host,omitempty"`
	TLSDomains   string `json:"tls_domains,omitempty"`
	AuthToWrite  *bool  `json:"auth_to_write,omitempty"`
	AuthRequired *bool  `json:"auth_required,omitempty"`

	// Sync services
	DistributedSyncEnabled *bool  `json:"distributed_sync_enabled,omitempty"`
	ClusterSyncEnabled     *bool  `json:"cluster_sync_enabled,omitempty"`
	RelayGroupEnabled      *bool  `json:"relay_group_enabled,omitempty"`
	NegentropyEnabled      *bool  `json:"negentropy_enabled,omitempty"`
	NegentropyListen       string `json:"negentropy_listen,omitempty"`

	// Certificate service
	CertsEnabled *bool `json:"certs_enabled,omitempty"`
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

// Config holds the launcher configuration for the unified binary.
type Config struct {
	// DBDriver is the database driver: badger or neo4j
	DBDriver string

	// ACLDriver is the ACL driver: follows, managed, curating
	ACLDriver string

	// DBListen is the address the database server listens on
	DBListen string

	// ACLListen is the address the ACL server listens on
	ACLListen string

	// ACLEnabled controls whether to run the ACL server as a separate process
	// When false, the relay runs in open mode (no ACL restrictions)
	ACLEnabled bool

	// DBReadyTimeout is how long to wait for the database to be ready
	DBReadyTimeout time.Duration

	// ACLReadyTimeout is how long to wait for the ACL server to be ready
	ACLReadyTimeout time.Duration

	// StopTimeout is how long to wait for processes to stop gracefully
	StopTimeout time.Duration

	// DataDir is the data directory to pass to the db subprocess
	DataDir string

	// LogLevel is the log level to use for all processes
	LogLevel string

	// Sync service configuration
	DistributedSyncEnabled bool
	DistributedSyncListen  string

	ClusterSyncEnabled bool
	ClusterSyncListen  string

	RelayGroupEnabled bool
	RelayGroupListen  string

	NegentropyEnabled bool
	NegentropyListen  string

	SyncReadyTimeout time.Duration

	// Certificate service configuration
	CertsEnabled bool

	// Bridge configuration
	BridgeEnabled bool
	BridgeDomain  string

	// Bridge bot configuration
	BridgeBotEnabled bool
	BridgeBotRelay   string
	BridgeBotFree    bool

	// ServicesEnabled controls whether to start the DB, relay, and other services
	// When false, only the admin UI runs (useful for initial setup/updates)
	ServicesEnabled bool

	// Admin UI configuration
	AdminEnabled bool
	AdminPort    int
	AdminOwners  []string
}

func loadConfig() (*Config, error) {
	// Load config file first (provides defaults)
	cf, err := loadConfigFile()
	if err != nil {
		// Log but don't fail - env vars are still valid
		cf = &ConfigFile{}
	}

	// Get driver settings - file first, then env
	dbDriver := stringOr(cf.DBDriver, getEnvOrDefault("ORLY_LAUNCHER_DB_DRIVER", "badger"))
	aclDriver := stringOr(cf.ACLDriver, getEnvOrDefault("ORLY_ACL_MODE", "follows"))

	// Parse admin owners - env takes precedence, then file
	envOwners := getEnvOrDefault("ORLY_LAUNCHER_OWNERS", "")
	var adminOwners []string
	if envOwners != "" {
		adminOwners = parseOwnersList(envOwners)
	} else if len(cf.AdminOwners) > 0 {
		adminOwners = cf.AdminOwners
	}

	cfg := &Config{
		DBDriver:        dbDriver,
		ACLDriver:       aclDriver,
		DBListen:        envOrFileOrDefault("ORLY_LAUNCHER_DB_LISTEN", cf.DBListen, "127.0.0.1:50051"),
		ACLListen:       envOrFileOrDefault("ORLY_LAUNCHER_ACL_LISTEN", cf.ACLListen, "127.0.0.1:50052"),
		ACLEnabled:      boolEnvOrFile("ORLY_LAUNCHER_ACL_ENABLED", cf.ACLEnabled, false),
		DBReadyTimeout:  parseDuration("ORLY_LAUNCHER_DB_READY_TIMEOUT", 30*time.Second),
		ACLReadyTimeout: parseDuration("ORLY_LAUNCHER_ACL_READY_TIMEOUT", 120*time.Second),
		StopTimeout:     parseDuration("ORLY_LAUNCHER_STOP_TIMEOUT", 30*time.Second),
		DataDir:         envOrFileOrDefault("ORLY_DATA_DIR", cf.DataDir, filepath.Join(xdg.DataHome, "ORLY")),
		LogLevel:        envOrFileOrDefault("ORLY_LOG_LEVEL", cf.LogLevel, "info"),

		// Sync services configuration
		DistributedSyncEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_DISTRIBUTED_ENABLED", cf.DistributedSyncEnabled, false),
		DistributedSyncListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_DISTRIBUTED_LISTEN", "127.0.0.1:50061"),

		ClusterSyncEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_CLUSTER_ENABLED", cf.ClusterSyncEnabled, false),
		ClusterSyncListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_CLUSTER_LISTEN", "127.0.0.1:50062"),

		RelayGroupEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_RELAYGROUP_ENABLED", cf.RelayGroupEnabled, false),
		RelayGroupListen:  getEnvOrDefault("ORLY_LAUNCHER_SYNC_RELAYGROUP_LISTEN", "127.0.0.1:50063"),

		NegentropyEnabled: boolEnvOrFile("ORLY_LAUNCHER_SYNC_NEGENTROPY_ENABLED", cf.NegentropyEnabled, false),
		NegentropyListen:  envOrFileOrDefault("ORLY_LAUNCHER_SYNC_NEGENTROPY_LISTEN", cf.NegentropyListen, "127.0.0.1:50064"),

		SyncReadyTimeout: parseDuration("ORLY_LAUNCHER_SYNC_READY_TIMEOUT", 30*time.Second),

		// Certificate service configuration
		CertsEnabled: boolEnvOrFile("ORLY_LAUNCHER_CERTS_ENABLED", cf.CertsEnabled, false),

		// Bridge configuration
		BridgeEnabled: getEnvOrDefault("ORLY_BRIDGE_ENABLED", "false") == "true",
		BridgeDomain:  getEnvOrDefault("ORLY_BRIDGE_DOMAIN", ""),

		// Bridge bot configuration
		BridgeBotEnabled: getEnvOrDefault("ORLY_BRIDGE_BOT_ENABLED", "false") == "true",
		BridgeBotRelay:   getEnvOrDefault("ORLY_BRIDGE_BOT_RELAY", ""),
		BridgeBotFree:    getEnvOrDefault("ORLY_BRIDGE_BOT_FREE", "false") == "true",

		// Services enabled (default true for backwards compatibility)
		ServicesEnabled: getEnvOrDefault("ORLY_LAUNCHER_SERVICES_ENABLED", "true") == "true",

		// Admin UI configuration
		AdminEnabled: getEnvOrDefault("ORLY_LAUNCHER_ADMIN_ENABLED", "true") == "true",
		AdminPort:    intEnvOrFile("ORLY_LAUNCHER_ADMIN_PORT", cf.AdminPort, 8080),
		AdminOwners:  adminOwners,
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
