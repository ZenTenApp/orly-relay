//go:build !(js && wasm)

// Package relay provides shared startup logic for running the ORLY relay.
// This allows both the root binary and the unified binary to share
// the same initialization code for monolithic deployments.
package relay

import (
	"context"
	"fmt"
	"net/http"
	pp "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/profile"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/app"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	aclgrpc "next.orly.dev/pkg/acl/grpc"
	"next.orly.dev/pkg/database"
	_ "next.orly.dev/pkg/database/grpc" // Import for grpc factory registration
	neo4jdb "next.orly.dev/pkg/neo4j"
	"next.orly.dev/pkg/ratelimit"
	"next.orly.dev/pkg/sync/negentropy"
	negentropygrpc "next.orly.dev/pkg/sync/negentropy/grpc"
	"next.orly.dev/pkg/utils/interrupt"
)

// StartupResult holds the initialized components from Startup.
type StartupResult struct {
	Ctx     context.Context
	Cancel  context.CancelFunc
	DB      database.Database
	Limiter *ratelimit.Limiter
	Quit    chan struct{}
}

// Startup initializes the database, ACL, rate limiter, and starts the relay server.
// It returns a StartupResult containing all initialized components.
func Startup(cfg *config.C) (*StartupResult, error) {
	runtime.GOMAXPROCS(128)
	debug.SetGCPercent(10)

	// Setup profiling
	profileStop := setupProfiling(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize database
	log.I.F("initializing %s database at %s", cfg.DBType, cfg.DataDir)
	db, err := database.NewDatabaseWithConfig(
		ctx, cancel, cfg.DBType, MakeDatabaseConfig(cfg),
	)
	if chk.E(err) {
		cancel()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}
	log.I.F("%s database initialized successfully", cfg.DBType)

	// Initialize ACL
	if err := initializeACL(ctx, cfg, db); err != nil {
		db.Close()
		cancel()
		return nil, fmt.Errorf("failed to initialize ACL: %w", err)
	}

	// Initialize negentropy handler (embedded or gRPC client)
	initializeNegentropy(ctx, cfg, db)

	// Create rate limiter
	limiter := createRateLimiter(cfg, db)

	// Start pprof HTTP server if enabled
	startPprofServer(ctx, cfg)

	// Start health check server if configured
	startHealthServer(ctx, cfg)

	// Start the relay
	quit := app.Run(ctx, cfg, db, limiter)

	// Store profileStop for cleanup
	result := &StartupResult{
		Ctx:     ctx,
		Cancel:  cancel,
		DB:      db,
		Limiter: limiter,
		Quit:    quit,
	}

	// Register profile stop handler
	interrupt.AddHandler(func() {
		log.I.F("interrupt received: stopping profiling")
		profileStop()
	})

	return result, nil
}

// RunWithSignals calls Startup and blocks on signals, handling graceful shutdown.
// This is the main entry point for running the relay as a standalone process.
func RunWithSignals(cfg *config.C) error {
	result, err := Startup(cfg)
	if err != nil {
		return err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigs:
			fmt.Printf("\r")
			log.I.F("received shutdown signal, starting graceful shutdown")
			result.Cancel()
			<-result.Quit
			chk.E(result.DB.Close())
			log.I.F("exiting")
			return nil
		case <-result.Quit:
			log.I.F("application quit signal received")
			result.Cancel()
			chk.E(result.DB.Close())
			log.I.F("exiting")
			return nil
		}
	}
}

// setupProfiling configures profiling based on config and returns a stop function.
func setupProfiling(cfg *config.C) func() {
	var profileStopOnce sync.Once
	profileStop := func() {}

	switch cfg.Pprof {
	case "cpu":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.CPUProfile, profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.CPUProfile)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("cpu profiling stopped and flushed")
			})
		}

	case "memory":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.MemProfile, profile.MemProfileRate(32), profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.MemProfile)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("memory profiling stopped and flushed")
			})
		}

	case "allocation":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.MemProfileAllocs, profile.MemProfileRate(32), profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.MemProfileAllocs)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("allocation profiling stopped and flushed")
			})
		}

	case "heap":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.MemProfileHeap, profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.MemProfileHeap)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("heap profiling stopped and flushed")
			})
		}

	case "mutex":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.MutexProfile, profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.MutexProfile)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("mutex profiling stopped and flushed")
			})
		}

	case "threadcreate":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.ThreadcreationProfile, profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.ThreadcreationProfile)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("threadcreate profiling stopped and flushed")
			})
		}

	case "goroutine":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.GoroutineProfile, profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.GoroutineProfile)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("goroutine profiling stopped and flushed")
			})
		}

	case "block":
		var prof interface{ Stop() }
		if cfg.PprofPath != "" {
			prof = profile.Start(profile.BlockProfile, profile.ProfilePath(cfg.PprofPath))
		} else {
			prof = profile.Start(profile.BlockProfile)
		}
		profileStop = func() {
			profileStopOnce.Do(func() {
				prof.Stop()
				log.I.F("block profiling stopped and flushed")
			})
		}
	}

	return profileStop
}

// initializeACL sets up ACL - either remote gRPC or in-process.
func initializeACL(ctx context.Context, cfg *config.C, db database.Database) error {
	aclType, aclServerAddr, aclConnTimeout := cfg.GetGRPCACLConfigValues()

	if aclType == "grpc" {
		// Use remote ACL server via gRPC
		log.I.F("connecting to gRPC ACL server at %s", aclServerAddr)
		aclClient, err := aclgrpc.New(ctx, &aclgrpc.ClientConfig{
			ServerAddress:  aclServerAddr,
			ConnectTimeout: aclConnTimeout,
		})
		if chk.E(err) {
			return fmt.Errorf("failed to connect to gRPC ACL server: %w", err)
		}

		// Wait for ACL server to be ready
		select {
		case <-aclClient.Ready():
			log.I.F("gRPC ACL client connected, mode: %s", aclClient.Type())
		case <-time.After(30 * time.Second):
			return fmt.Errorf("timeout waiting for gRPC ACL server")
		}

		// Register and activate the gRPC client as the ACL backend
		acl.Registry.RegisterAndActivate(aclClient)
	} else {
		// Use in-process ACL
		acl.Registry.SetMode(cfg.ACLMode)
		if err := acl.Registry.Configure(cfg, db, ctx); chk.E(err) {
			return err
		}
		acl.Registry.Syncer()
	}

	return nil
}

// initializeNegentropy sets up negentropy handling (embedded or gRPC client).
func initializeNegentropy(ctx context.Context, cfg *config.C, db database.Database) {
	syncType, _, _, _, negentropyAddr, syncTimeout, negentropyEnabled := cfg.GetGRPCSyncConfigValues()

	if !negentropyEnabled {
		return
	}

	if syncType == "grpc" && negentropyAddr != "" {
		// Use gRPC client to connect to remote negentropy server
		log.I.F("connecting to gRPC negentropy server at %s", negentropyAddr)
		negClient, err := negentropygrpc.New(ctx, &negentropygrpc.ClientConfig{
			ServerAddress:  negentropyAddr,
			ConnectTimeout: syncTimeout,
		})
		if err != nil {
			log.W.F("failed to connect to gRPC negentropy server: %v (NIP-77 disabled)", err)
			return
		}

		// Wait for negentropy server to be ready
		select {
		case <-negClient.Ready():
			log.I.F("gRPC negentropy client connected")
			app.SetNegentropyHandler(negClient)
		case <-time.After(30 * time.Second):
			log.W.F("timeout waiting for gRPC negentropy server (NIP-77 disabled)")
		}
	} else {
		// Use embedded negentropy handler (monolithic mode)
		log.I.F("initializing embedded negentropy handler")
		negHandler := negentropy.NewEmbeddedHandler(db, &negentropy.Config{
			SyncInterval:         60 * time.Second,
			FrameSize:            128 * 1024,
			IDSize:               16,
			ClientSessionTimeout: 5 * time.Minute,
		})
		negHandler.Start()
		app.SetNegentropyHandler(negHandler)
		log.I.F("embedded negentropy handler initialized (NIP-77 enabled)")
	}
}

// createRateLimiter creates and configures the rate limiter.
func createRateLimiter(cfg *config.C, db database.Database) *ratelimit.Limiter {
	rateLimitEnabled, targetMB,
		writeKp, writeKi, writeKd,
		readKp, readKi, readKd,
		maxWriteMs, maxReadMs,
		writeTarget, readTarget,
		emergencyThreshold, recoveryThreshold,
		emergencyMaxMs := cfg.GetRateLimitConfigValues()

	if !rateLimitEnabled {
		return ratelimit.NewDisabledLimiter()
	}

	// Auto-detect memory target if set to 0
	if targetMB == 0 {
		var err error
		targetMB, err = ratelimit.CalculateTargetMemoryMB(targetMB)
		if err != nil {
			log.F.F("FATAL: %v", err)
			log.F.F("There is not enough memory to run this relay in this environment.")
			log.F.F("Available: %dMB, Required minimum: %dMB",
				ratelimit.DetectAvailableMemoryMB(), ratelimit.MinimumMemoryMB)
			os.Exit(1)
		}
		stats := ratelimit.GetMemoryStats(targetMB)
		calculated66 := int(float64(stats.AvailableMB) * ratelimit.AutoDetectMemoryFraction)
		if calculated66 > ratelimit.DefaultMaxMemoryMB {
			log.I.F("memory auto-detected: total=%dMB, available=%dMB, target=%dMB (capped at default max, 66%% would be %dMB)",
				stats.TotalMB, stats.AvailableMB, targetMB, calculated66)
		} else {
			log.I.F("memory auto-detected: total=%dMB, available=%dMB, target=%dMB (66%% of available)",
				stats.TotalMB, stats.AvailableMB, targetMB)
		}
	} else {
		// Validate explicitly configured target
		_, err := ratelimit.CalculateTargetMemoryMB(targetMB)
		if err != nil {
			log.F.F("FATAL: %v", err)
			log.F.F("Configured target memory %dMB is below minimum required %dMB.",
				targetMB, ratelimit.MinimumMemoryMB)
			os.Exit(1)
		}
	}

	rlConfig := ratelimit.NewConfigFromValues(
		rateLimitEnabled, targetMB,
		writeKp, writeKi, writeKd,
		readKp, readKi, readKd,
		maxWriteMs, maxReadMs,
		writeTarget, readTarget,
		emergencyThreshold, recoveryThreshold,
		emergencyMaxMs,
	)

	// Create appropriate monitor based on database type
	if badgerDB, ok := db.(*database.D); ok {
		limiter := ratelimit.NewBadgerLimiter(rlConfig, badgerDB.DB)
		badgerDB.SetRateLimiter(limiter)
		log.I.F("rate limiter configured for Badger backend (target: %dMB)", targetMB)
		return limiter
	}

	if n4jDB, ok := db.(*neo4jdb.N); ok {
		limiter := ratelimit.NewNeo4jLimiter(
			rlConfig,
			n4jDB.Driver(),
			n4jDB.QuerySem(),
			n4jDB.MaxConcurrentQueries(),
		)
		log.I.F("rate limiter configured for Neo4j backend (target: %dMB)", targetMB)
		return limiter
	}

	// For other backends, create a disabled limiter
	log.I.F("rate limiter disabled for unknown backend")
	return ratelimit.NewDisabledLimiter()
}

// startPprofServer starts the HTTP pprof server if enabled.
func startPprofServer(ctx context.Context, cfg *config.C) {
	if !cfg.PprofHTTP {
		return
	}

	pprofAddr := fmt.Sprintf("%s:%d", cfg.Listen, 6060)
	pprofMux := http.NewServeMux()
	pprofMux.HandleFunc("/debug/pprof/", pp.Index)
	pprofMux.HandleFunc("/debug/pprof/cmdline", pp.Cmdline)
	pprofMux.HandleFunc("/debug/pprof/profile", pp.Profile)
	pprofMux.HandleFunc("/debug/pprof/symbol", pp.Symbol)
	pprofMux.HandleFunc("/debug/pprof/trace", pp.Trace)
	for _, p := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		pprofMux.Handle("/debug/pprof/"+p, pp.Handler(p))
	}

	ppSrv := &http.Server{Addr: pprofAddr, Handler: pprofMux}
	go func() {
		log.I.F("pprof server listening on %s", pprofAddr)
		if err := ppSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.E.F("pprof server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelShutdown()
		_ = ppSrv.Shutdown(shutdownCtx)
	}()
}

// startHealthServer starts the health check HTTP server if configured.
func startHealthServer(ctx context.Context, cfg *config.C) {
	if cfg.HealthPort <= 0 {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		log.I.F("health check ok")
	})

	// Optional shutdown endpoint
	if cfg.EnableShutdown {
		mux.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("shutting down"))
			log.I.F("shutdown requested via /shutdown; sending SIGINT to self")
			go func() {
				p, _ := os.FindProcess(os.Getpid())
				_ = p.Signal(os.Interrupt)
			}()
		})
	}

	healthSrv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Listen, cfg.HealthPort),
		Handler: mux,
	}

	go func() {
		log.I.F("health check server listening on %s", healthSrv.Addr)
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.E.F("health server error: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelShutdown()
		_ = healthSrv.Shutdown(shutdownCtx)
	}()
}

// MakeDatabaseConfig creates a database.DatabaseConfig from the app config.
func MakeDatabaseConfig(cfg *config.C) *database.DatabaseConfig {
	dataDir, logLevel,
		blockCacheMB, indexCacheMB, queryCacheSizeMB,
		queryCacheMaxAge,
		queryCacheDisabled,
		serialCachePubkeys, serialCacheEventIds,
		zstdLevel,
		neo4jURI, neo4jUser, neo4jPassword,
		neo4jMaxConnPoolSize, neo4jFetchSize, neo4jMaxTxRetrySeconds, neo4jQueryResultLimit := cfg.GetDatabaseConfigValues()

	grpcServerAddress, grpcConnectTimeout := cfg.GetGRPCConfigValues()

	return &database.DatabaseConfig{
		DataDir:                dataDir,
		LogLevel:               logLevel,
		BlockCacheMB:           blockCacheMB,
		IndexCacheMB:           indexCacheMB,
		QueryCacheSizeMB:       queryCacheSizeMB,
		QueryCacheMaxAge:       queryCacheMaxAge,
		QueryCacheDisabled:     queryCacheDisabled,
		SerialCachePubkeys:     serialCachePubkeys,
		SerialCacheEventIds:    serialCacheEventIds,
		ZSTDLevel:              zstdLevel,
		Neo4jURI:               neo4jURI,
		Neo4jUser:              neo4jUser,
		Neo4jPassword:          neo4jPassword,
		Neo4jMaxConnPoolSize:   neo4jMaxConnPoolSize,
		Neo4jFetchSize:         neo4jFetchSize,
		Neo4jMaxTxRetrySeconds: neo4jMaxTxRetrySeconds,
		Neo4jQueryResultLimit:  neo4jQueryResultLimit,
		GRPCServerAddress:      grpcServerAddress,
		GRPCConnectTimeout:     grpcConnectTimeout,
	}
}
