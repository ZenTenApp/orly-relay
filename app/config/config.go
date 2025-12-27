// Package config provides a go-simpler.org/env configuration table and helpers
// for working with the list of key/value lists stored in .env files.
//
// IMPORTANT: This file is the SINGLE SOURCE OF TRUTH for all environment variables.
// All configuration options MUST be defined here with proper `env` struct tags.
// Never use os.Getenv() directly in other packages - pass configuration via structs.
// This ensures all options appear in `./orly help` output and are documented.
//
// For database backends, use GetDatabaseConfigValues() to extract database-specific
// settings, then construct a database.DatabaseConfig in the caller (e.g., main.go).
package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"go-simpler.org/env"
	lol "lol.mleku.dev"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/logbuffer"
	"next.orly.dev/pkg/version"
)

// C holds application configuration settings loaded from environment variables
// and default values. It defines parameters for app behaviour, storage
// locations, logging, and network settings used across the relay service.
type C struct {
	AppName             string        `env:"ORLY_APP_NAME" usage:"set a name to display on information about the relay" default:"ORLY"`
	DataDir             string        `env:"ORLY_DATA_DIR" usage:"storage location for the event store" default:"~/.local/share/ORLY"`
	Listen              string        `env:"ORLY_LISTEN" default:"0.0.0.0" usage:"network listen address"`
	Port                int           `env:"ORLY_PORT" default:"3334" usage:"port to listen on"`
	HealthPort          int           `env:"ORLY_HEALTH_PORT" default:"0" usage:"optional health check HTTP port; 0 disables"`
	EnableShutdown      bool          `env:"ORLY_ENABLE_SHUTDOWN" default:"false" usage:"if true, expose /shutdown on the health port to gracefully stop the process (for profiling)"`
	LogLevel            string        `env:"ORLY_LOG_LEVEL" default:"info" usage:"relay log level: fatal error warn info debug trace"`
	DBLogLevel          string        `env:"ORLY_DB_LOG_LEVEL" default:"info" usage:"database log level: fatal error warn info debug trace"`
	DBBlockCacheMB      int           `env:"ORLY_DB_BLOCK_CACHE_MB" default:"512" usage:"Badger block cache size in MB (higher improves read hit ratio)"`
	DBIndexCacheMB      int           `env:"ORLY_DB_INDEX_CACHE_MB" default:"256" usage:"Badger index cache size in MB (improves index lookup performance)"`
	DBZSTDLevel         int           `env:"ORLY_DB_ZSTD_LEVEL" default:"1" usage:"Badger ZSTD compression level (1=fast/500MB/s, 3=default, 9=best ratio, 0=disable)"`
	LogToStdout         bool          `env:"ORLY_LOG_TO_STDOUT" default:"false" usage:"log to stdout instead of stderr"`
	LogBufferSize       int           `env:"ORLY_LOG_BUFFER_SIZE" default:"10000" usage:"number of log entries to keep in memory for web UI viewing (0 disables)"`
	Pprof               string        `env:"ORLY_PPROF" usage:"enable pprof in modes: cpu,memory,allocation,heap,block,goroutine,threadcreate,mutex"`
	PprofPath           string        `env:"ORLY_PPROF_PATH" usage:"optional directory to write pprof profiles into (inside container); default is temporary dir"`
	PprofHTTP           bool          `env:"ORLY_PPROF_HTTP" default:"false" usage:"if true, expose net/http/pprof on port 6060"`
	IPWhitelist         []string      `env:"ORLY_IP_WHITELIST" usage:"comma-separated list of IP addresses to allow access from, matches on prefixes to allow private subnets, eg 10.0.0 = 10.0.0.0/8"`
	IPBlacklist         []string      `env:"ORLY_IP_BLACKLIST" usage:"comma-separated list of IP addresses to block; matches on prefixes to allow subnets, e.g. 192.168 = 192.168.0.0/16"`
	Admins              []string      `env:"ORLY_ADMINS" usage:"comma-separated list of admin npubs"`
	Owners              []string      `env:"ORLY_OWNERS" usage:"comma-separated list of owner npubs, who have full control of the relay for wipe and restart and other functions"`
	ACLMode             string        `env:"ORLY_ACL_MODE" usage:"ACL mode: follows, managed (nip-86), none" default:"none"`
	AuthRequired        bool          `env:"ORLY_AUTH_REQUIRED" usage:"require authentication for all requests (works with managed ACL)" default:"false"`
	AuthToWrite         bool          `env:"ORLY_AUTH_TO_WRITE" usage:"require authentication only for write operations (EVENT), allow REQ/COUNT without auth" default:"false"`
	BootstrapRelays     []string      `env:"ORLY_BOOTSTRAP_RELAYS" usage:"comma-separated list of bootstrap relay URLs for initial sync"`
	NWCUri              string        `env:"ORLY_NWC_URI" usage:"NWC (Nostr Wallet Connect) connection string for Lightning payments"`
	SubscriptionEnabled bool          `env:"ORLY_SUBSCRIPTION_ENABLED" default:"false" usage:"enable subscription-based access control requiring payment for non-directory events"`
	MonthlyPriceSats    int64         `env:"ORLY_MONTHLY_PRICE_SATS" default:"6000" usage:"price in satoshis for one month subscription (default ~$2 USD)"`
	RelayURL            string        `env:"ORLY_RELAY_URL" usage:"base URL for the relay dashboard (e.g., https://relay.example.com)"`
	RelayAddresses      []string      `env:"ORLY_RELAY_ADDRESSES" usage:"comma-separated list of websocket addresses for this relay (e.g., wss://relay.example.com,wss://backup.example.com)"`
	RelayPeers          []string      `env:"ORLY_RELAY_PEERS" usage:"comma-separated list of peer relay URLs for distributed synchronization (e.g., https://peer1.example.com,https://peer2.example.com)"`
	RelayGroupAdmins    []string      `env:"ORLY_RELAY_GROUP_ADMINS" usage:"comma-separated list of npubs authorized to publish relay group configuration events"`
	ClusterAdmins       []string      `env:"ORLY_CLUSTER_ADMINS" usage:"comma-separated list of npubs authorized to manage cluster membership"`
	FollowListFrequency time.Duration `env:"ORLY_FOLLOW_LIST_FREQUENCY" usage:"how often to fetch admin follow lists (default: 1h)" default:"1h"`

	// Blossom blob storage service level settings
	BlossomServiceLevels string `env:"ORLY_BLOSSOM_SERVICE_LEVELS" usage:"comma-separated list of service levels in format: name:storage_mb_per_sat_per_month (e.g., basic:1,premium:10)"`

	// Web UI and dev mode settings
	WebDisableEmbedded bool   `env:"ORLY_WEB_DISABLE" default:"false" usage:"disable serving the embedded web UI; useful for hot-reload during development"`
	WebDevProxyURL     string `env:"ORLY_WEB_DEV_PROXY_URL" usage:"when ORLY_WEB_DISABLE is true, reverse-proxy non-API paths to this dev server URL (e.g. http://localhost:5173)"`

	// Sprocket settings
	SprocketEnabled bool `env:"ORLY_SPROCKET_ENABLED" default:"false" usage:"enable sprocket event processing plugin system"`

	// Spider settings
	SpiderMode string `env:"ORLY_SPIDER_MODE" default:"none" usage:"spider mode for syncing events: none, follows"`

	// Directory Spider settings
	DirectorySpiderEnabled  bool          `env:"ORLY_DIRECTORY_SPIDER" default:"false" usage:"enable directory spider for metadata sync (kinds 0, 3, 10000, 10002)"`
	DirectorySpiderInterval time.Duration `env:"ORLY_DIRECTORY_SPIDER_INTERVAL" default:"24h" usage:"how often to run directory spider"`
	DirectorySpiderMaxHops  int           `env:"ORLY_DIRECTORY_SPIDER_HOPS" default:"3" usage:"maximum hops for relay discovery from seed users"`

	PolicyEnabled bool   `env:"ORLY_POLICY_ENABLED" default:"false" usage:"enable policy-based event processing (default config: $HOME/.config/ORLY/policy.json)"`
	PolicyPath    string `env:"ORLY_POLICY_PATH" usage:"ABSOLUTE path to policy configuration file (MUST start with /); overrides default location; relative paths are rejected"`

	// NIP-43 Relay Access Metadata and Requests
	NIP43Enabled          bool          `env:"ORLY_NIP43_ENABLED" default:"false" usage:"enable NIP-43 relay access metadata and invite system"`
	NIP43PublishEvents    bool          `env:"ORLY_NIP43_PUBLISH_EVENTS" default:"true" usage:"publish kind 8000/8001 events when members are added/removed"`
	NIP43PublishMemberList bool         `env:"ORLY_NIP43_PUBLISH_MEMBER_LIST" default:"true" usage:"publish kind 13534 membership list events"`
	NIP43InviteExpiry     time.Duration `env:"ORLY_NIP43_INVITE_EXPIRY" default:"24h" usage:"how long invite codes remain valid"`

	// Database configuration
	DBType              string `env:"ORLY_DB_TYPE" default:"badger" usage:"database backend to use: badger or neo4j"`
	QueryCacheDisabled  bool   `env:"ORLY_QUERY_CACHE_DISABLED" default:"true" usage:"disable query cache to reduce memory usage (trades memory for query performance)"`
	QueryCacheSizeMB    int    `env:"ORLY_QUERY_CACHE_SIZE_MB" default:"512" usage:"query cache size in MB (caches database query results for faster REQ responses)"`
	QueryCacheMaxAge    string `env:"ORLY_QUERY_CACHE_MAX_AGE" default:"5m" usage:"maximum age for cached query results (e.g., 5m, 10m, 1h)"`

	// Neo4j configuration (only used when ORLY_DB_TYPE=neo4j)
	Neo4jURI      string `env:"ORLY_NEO4J_URI" default:"bolt://localhost:7687" usage:"Neo4j bolt URI (only used when ORLY_DB_TYPE=neo4j)"`
	Neo4jUser     string `env:"ORLY_NEO4J_USER" default:"neo4j" usage:"Neo4j authentication username (only used when ORLY_DB_TYPE=neo4j)"`
	Neo4jPassword string `env:"ORLY_NEO4J_PASSWORD" default:"password" usage:"Neo4j authentication password (only used when ORLY_DB_TYPE=neo4j)"`

	// Advanced database tuning
	SerialCachePubkeys  int `env:"ORLY_SERIAL_CACHE_PUBKEYS" default:"100000" usage:"max pubkeys to cache for compact event storage (default: 100000, ~3.2MB memory)"`
	SerialCacheEventIds int `env:"ORLY_SERIAL_CACHE_EVENT_IDS" default:"500000" usage:"max event IDs to cache for compact event storage (default: 500000, ~16MB memory)"`

	// Connection concurrency control
	MaxHandlersPerConnection int `env:"ORLY_MAX_HANDLERS_PER_CONN" default:"100" usage:"max concurrent message handlers per WebSocket connection (limits goroutine growth under load)"`

	// Adaptive rate limiting (PID-controlled)
	RateLimitEnabled            bool    `env:"ORLY_RATE_LIMIT_ENABLED" default:"true" usage:"enable adaptive PID-controlled rate limiting for database operations"`
	RateLimitTargetMB           int     `env:"ORLY_RATE_LIMIT_TARGET_MB" default:"0" usage:"target memory limit in MB (0=auto-detect: 66% of available, min 500MB)"`
	RateLimitWriteKp            float64 `env:"ORLY_RATE_LIMIT_WRITE_KP" default:"0.5" usage:"PID proportional gain for write operations"`
	RateLimitWriteKi            float64 `env:"ORLY_RATE_LIMIT_WRITE_KI" default:"0.1" usage:"PID integral gain for write operations"`
	RateLimitWriteKd            float64 `env:"ORLY_RATE_LIMIT_WRITE_KD" default:"0.05" usage:"PID derivative gain for write operations (filtered)"`
	RateLimitReadKp             float64 `env:"ORLY_RATE_LIMIT_READ_KP" default:"0.3" usage:"PID proportional gain for read operations"`
	RateLimitReadKi             float64 `env:"ORLY_RATE_LIMIT_READ_KI" default:"0.05" usage:"PID integral gain for read operations"`
	RateLimitReadKd             float64 `env:"ORLY_RATE_LIMIT_READ_KD" default:"0.02" usage:"PID derivative gain for read operations (filtered)"`
	RateLimitMaxWriteMs         int     `env:"ORLY_RATE_LIMIT_MAX_WRITE_MS" default:"1000" usage:"maximum delay for write operations in milliseconds"`
	RateLimitMaxReadMs          int     `env:"ORLY_RATE_LIMIT_MAX_READ_MS" default:"500" usage:"maximum delay for read operations in milliseconds"`
	RateLimitWriteTarget        float64 `env:"ORLY_RATE_LIMIT_WRITE_TARGET" default:"0.85" usage:"PID setpoint for writes (throttle when load exceeds this, 0.0-1.0)"`
	RateLimitReadTarget         float64 `env:"ORLY_RATE_LIMIT_READ_TARGET" default:"0.90" usage:"PID setpoint for reads (throttle when load exceeds this, 0.0-1.0)"`
	RateLimitEmergencyThreshold float64 `env:"ORLY_RATE_LIMIT_EMERGENCY_THRESHOLD" default:"1.167" usage:"memory pressure ratio (target+1/6) to trigger emergency mode with aggressive throttling"`
	RateLimitRecoveryThreshold  float64 `env:"ORLY_RATE_LIMIT_RECOVERY_THRESHOLD" default:"0.833" usage:"memory pressure ratio (target-1/6) below which emergency mode exits (hysteresis)"`
	RateLimitEmergencyMaxMs     int     `env:"ORLY_RATE_LIMIT_EMERGENCY_MAX_MS" default:"5000" usage:"maximum delay for writes in emergency mode (milliseconds)"`

	// TLS configuration
	TLSDomains []string `env:"ORLY_TLS_DOMAINS" usage:"comma-separated list of domains to respond to for TLS"`
	Certs      []string `env:"ORLY_CERTS" usage:"comma-separated list of paths to certificate root names (e.g., /path/to/cert will load /path/to/cert.pem and /path/to/cert.key)"`

	// WireGuard VPN configuration (for secure bunker access)
	WGEnabled  bool   `env:"ORLY_WG_ENABLED" default:"false" usage:"enable embedded WireGuard VPN server for private bunker access"`
	WGPort     int    `env:"ORLY_WG_PORT" default:"51820" usage:"UDP port for WireGuard VPN server"`
	WGEndpoint string `env:"ORLY_WG_ENDPOINT" usage:"public IP/domain for WireGuard endpoint (required if WG enabled)"`
	WGNetwork  string `env:"ORLY_WG_NETWORK" default:"10.73.0.0/16" usage:"WireGuard internal network CIDR"`

	// NIP-46 Bunker configuration (remote signing service)
	BunkerEnabled bool `env:"ORLY_BUNKER_ENABLED" default:"false" usage:"enable NIP-46 bunker signing service (requires WireGuard)"`
	BunkerPort    int  `env:"ORLY_BUNKER_PORT" default:"3335" usage:"internal port for bunker WebSocket (only accessible via WireGuard)"`

	// Cluster replication configuration
	ClusterPropagatePrivilegedEvents bool `env:"ORLY_CLUSTER_PROPAGATE_PRIVILEGED_EVENTS" default:"true" usage:"propagate privileged events (DMs, gift wraps, etc.) to relay peers for replication"`

	// ServeMode is set programmatically by the 'serve' subcommand to grant full owner
	// access to all users (no env tag - internal use only)
	ServeMode bool
}

// New creates and initializes a new configuration object for the relay
// application
//
// # Return Values
//
//   - cfg: A pointer to the initialized configuration struct containing default
//     or environment-provided values
//
//   - err: An error object that is non-nil if any operation during
//     initialization fails
//
// # Expected Behaviour:
//
// Initializes a new configuration instance by loading environment variables and
// checking for a .env file in the default configuration directory. Sets logging
// levels based on configuration values and returns the populated configuration
// or an error if any step fails
func New() (cfg *C, err error) {
	cfg = &C{}
	if err = env.Load(cfg, &env.Options{SliceSep: ","}); chk.T(err) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n\n", err)
		}
		PrintHelp(cfg, os.Stderr)
		os.Exit(0)
	}
	if cfg.DataDir == "" || strings.Contains(cfg.DataDir, "~") {
		cfg.DataDir = filepath.Join(xdg.DataHome, cfg.AppName)
	}
	if GetEnv() {
		PrintEnv(cfg, os.Stdout)
		os.Exit(0)
	}
	if HelpRequested() {
		PrintHelp(cfg, os.Stderr)
		os.Exit(0)
	}
	if cfg.LogToStdout {
		lol.Writer = os.Stdout
	}
	// Initialize log buffer for web UI viewing
	if cfg.LogBufferSize > 0 {
		logbuffer.Init(cfg.LogBufferSize)
		logbuffer.SetCurrentLevel(cfg.LogLevel)
		lol.Writer = logbuffer.NewBufferedWriter(lol.Writer, logbuffer.GlobalBuffer)
		// Reinitialize the loggers to use the new wrapped Writer
		// The lol.Main logger is initialized in init() with os.Stderr directly,
		// so we need to recreate it with the new Writer
		l, c, e := lol.New(lol.Writer, 2)
		lol.Main.Log = l
		lol.Main.Check = c
		lol.Main.Errorf = e
		// Also update the log package convenience variables
		log.F, log.E, log.W, log.I, log.D, log.T = l.F, l.E, l.W, l.I, l.D, l.T
	}
	lol.SetLogLevel(cfg.LogLevel)
	return
}

// HelpRequested determines if the command line arguments indicate a request for help
//
// # Return Values
//
//   - help: A boolean value indicating true if a help flag was detected in the
//     command line arguments, false otherwise
//
// # Expected Behaviour
//
// The function checks the first command line argument for common help flags and
// returns true if any of them are present. Returns false if no help flag is found
func HelpRequested() (help bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "help", "-h", "--h", "-help", "--help", "?":
			help = true
		}
	}
	return
}

// GetEnv checks if the first command line argument is "env" and returns
// whether the environment configuration should be printed.
//
// # Return Values
//
//   - requested: A boolean indicating true if the 'env' argument was
//     provided, false otherwise.
//
// # Expected Behaviour
//
// The function returns true when the first command line argument is "env"
// (case-insensitive), signalling that the environment configuration should be
// printed. Otherwise, it returns false.
func GetEnv() (requested bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "env":
			requested = true
		}
	}
	return
}

// IdentityRequested checks if the first command line argument is "identity" and returns
// whether the relay identity should be printed and the program should exit.
//
// Return Values
//   - requested: true if the 'identity' subcommand was provided, false otherwise.
func IdentityRequested() (requested bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "identity":
			requested = true
		}
	}
	return
}

// ServeRequested checks if the first command line argument is "serve" and returns
// whether the relay should start in ephemeral serve mode with RAM-based storage.
//
// Return Values
//   - requested: true if the 'serve' subcommand was provided, false otherwise.
func ServeRequested() (requested bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "serve":
			requested = true
		}
	}
	return
}

// VersionRequested checks if the first command line argument is "version" and returns
// whether the version should be printed and the program should exit.
//
// Return Values
//   - requested: true if the 'version' subcommand was provided, false otherwise.
func VersionRequested() (requested bool) {
	if len(os.Args) > 1 {
		switch strings.ToLower(os.Args[1]) {
		case "version", "-v", "--v", "-version", "--version":
			requested = true
		}
	}
	return
}

// KV is a key/value pair.
type KV struct{ Key, Value string }

// KVSlice is a sortable slice of key/value pairs, designed for managing
// configuration data and enabling operations like merging and sorting based on
// keys.
type KVSlice []KV

func (kv KVSlice) Len() int           { return len(kv) }
func (kv KVSlice) Less(i, j int) bool { return kv[i].Key < kv[j].Key }
func (kv KVSlice) Swap(i, j int)      { kv[i], kv[j] = kv[j], kv[i] }

// Compose merges two KVSlice instances into a new slice where key-value pairs
// from the second slice override any duplicate keys from the first slice.
//
// # Parameters
//
//   - kv2: The second KVSlice whose entries will be merged with the receiver.
//
// # Return Values
//
//   - out: A new KVSlice containing all entries from both slices, with keys
//     from kv2 taking precedence over keys from the receiver.
//
// # Expected Behaviour
//
// The method returns a new KVSlice that combines the contents of the receiver
// and kv2. If any key exists in both slices, the value from kv2 is used. The
// resulting slice remains sorted by keys as per the KVSlice implementation.
func (kv KVSlice) Compose(kv2 KVSlice) (out KVSlice) {
	// duplicate the initial KVSlice
	out = append(out, kv...)
out:
	for i, p := range kv2 {
		for j, q := range out {
			// if the key is repeated, replace the value
			if p.Key == q.Key {
				out[j].Value = kv2[i].Value
				continue out
			}
		}
		out = append(out, p)
	}
	return
}

// EnvKV generates key/value pairs from a configuration object's struct tags
//
// # Parameters
//
//   - cfg: A configuration object whose struct fields are processed for env tags
//
// # Return Values
//
//   - m: A KVSlice containing key/value pairs derived from the config's env tags
//
// # Expected Behaviour
//
// Processes each field of the config object, extracting values tagged with
// "env" and converting them to strings. Skips fields without an "env" tag.
// Handles various value types including strings, integers, booleans, durations,
// and string slices by joining elements with commas.
func EnvKV(cfg any) (m KVSlice) {
	t := reflect.TypeOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		k := t.Field(i).Tag.Get("env")
		v := reflect.ValueOf(cfg).Field(i).Interface()
		var val string
		switch v := v.(type) {
		case string:
			val = v
		case int, bool, time.Duration:
			val = fmt.Sprint(v)
		case []string:
			if len(v) > 0 {
				val = strings.Join(v, ",")
			}
		}
		// this can happen with embedded structs
		if k == "" {
			continue
		}
		m = append(m, KV{k, val})
	}
	return
}

// PrintEnv outputs sorted environment key/value pairs from a configuration object
// to the provided writer
//
// # Parameters
//
//   - cfg: Pointer to the configuration object containing env tags
//
//   - printer: Destination for the output, typically an io.Writer implementation
//
// # Expected Behaviour
//
// Outputs each environment variable derived from the config's struct tags in
// sorted order, formatted as "key=value\n" to the specified writer
func PrintEnv(cfg *C, printer io.Writer) {
	kvs := EnvKV(*cfg)
	sort.Sort(kvs)
	for _, v := range kvs {
		_, _ = fmt.Fprintf(printer, "%s=%s\n", v.Key, v.Value)
	}
}

// PrintHelp prints help information including application version, environment
// variable configuration, and details about .env file handling to the provided
// writer
//
// # Parameters
//
//   - cfg: Configuration object containing app name and config directory path
//
//   - printer: Output destination for the help text
//
// # Expected Behaviour
//
// Prints application name and version followed by environment variable
// configuration details, explains .env file behaviour including automatic
// loading and custom path options, and displays current configuration values
// using PrintEnv. Outputs all information to the specified writer
func PrintHelp(cfg *C, printer io.Writer) {
	_, _ = fmt.Fprintf(
		printer,
		"%s %s\n\n", cfg.AppName, version.V,
	)
	_, _ = fmt.Fprintf(
		printer,
		`Usage: %s [env|help|identity|serve|version]

- env: print environment variables configuring %s
- help: print this help text
- identity: print the relay identity secret and public key
- serve: start ephemeral relay with RAM-based storage at /dev/shm/orlyserve
         listening on 0.0.0.0:10547 with 'none' ACL mode (open relay)
         useful for testing and benchmarking
- version: print version and exit (also: -v, --v, -version, --version)

`,
		cfg.AppName, cfg.AppName,
	)
	_, _ = fmt.Fprintf(
		printer,
		"Environment variables that configure %s:\n\n", cfg.AppName,
	)
	env.Usage(cfg, printer, &env.Options{SliceSep: ","})
	fmt.Fprintf(printer, "\ncurrent configuration:\n\n")
	PrintEnv(cfg, printer)
	fmt.Fprintln(printer)
}

// GetDatabaseConfigValues returns the database configuration values as individual fields.
// This avoids circular imports with pkg/database while allowing main.go to construct
// a database.DatabaseConfig with the correct type.
func (cfg *C) GetDatabaseConfigValues() (
	dataDir, logLevel string,
	blockCacheMB, indexCacheMB, queryCacheSizeMB int,
	queryCacheMaxAge time.Duration,
	queryCacheDisabled bool,
	serialCachePubkeys, serialCacheEventIds int,
	zstdLevel int,
	neo4jURI, neo4jUser, neo4jPassword string,
) {
	// Parse query cache max age from string to duration
	queryCacheMaxAge = 5 * time.Minute // Default
	if cfg.QueryCacheMaxAge != "" {
		if duration, err := time.ParseDuration(cfg.QueryCacheMaxAge); err == nil {
			queryCacheMaxAge = duration
		}
	}

	return cfg.DataDir, cfg.DBLogLevel,
		cfg.DBBlockCacheMB, cfg.DBIndexCacheMB, cfg.QueryCacheSizeMB,
		queryCacheMaxAge,
		cfg.QueryCacheDisabled,
		cfg.SerialCachePubkeys, cfg.SerialCacheEventIds,
		cfg.DBZSTDLevel,
		cfg.Neo4jURI, cfg.Neo4jUser, cfg.Neo4jPassword
}

// GetRateLimitConfigValues returns the rate limiting configuration values.
// This avoids circular imports with pkg/ratelimit while allowing main.go to construct
// a ratelimit.Config with the correct type.
func (cfg *C) GetRateLimitConfigValues() (
	enabled bool,
	targetMB int,
	writeKp, writeKi, writeKd float64,
	readKp, readKi, readKd float64,
	maxWriteMs, maxReadMs int,
	writeTarget, readTarget float64,
	emergencyThreshold, recoveryThreshold float64,
	emergencyMaxMs int,
) {
	return cfg.RateLimitEnabled,
		cfg.RateLimitTargetMB,
		cfg.RateLimitWriteKp, cfg.RateLimitWriteKi, cfg.RateLimitWriteKd,
		cfg.RateLimitReadKp, cfg.RateLimitReadKi, cfg.RateLimitReadKd,
		cfg.RateLimitMaxWriteMs, cfg.RateLimitMaxReadMs,
		cfg.RateLimitWriteTarget, cfg.RateLimitReadTarget,
		cfg.RateLimitEmergencyThreshold, cfg.RateLimitRecoveryThreshold,
		cfg.RateLimitEmergencyMaxMs
}

// GetWireGuardConfigValues returns the WireGuard VPN configuration values.
// This avoids circular imports with pkg/wireguard while allowing main.go to construct
// the WireGuard server configuration.
func (cfg *C) GetWireGuardConfigValues() (
	enabled bool,
	port int,
	endpoint string,
	network string,
	bunkerEnabled bool,
	bunkerPort int,
) {
	return cfg.WGEnabled,
		cfg.WGPort,
		cfg.WGEndpoint,
		cfg.WGNetwork,
		cfg.BunkerEnabled,
		cfg.BunkerPort
}
