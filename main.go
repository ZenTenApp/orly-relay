package main

import (
	"context"
	"fmt"
	"net/http"
	pp "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/adrg/xdg"
	"github.com/pkg/profile"
	"golang.org/x/term"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app"
	"next.orly.dev/app/branding"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	aclgrpc "next.orly.dev/pkg/acl/grpc"
	negentropygrpc "next.orly.dev/pkg/sync/negentropy/grpc"
	"git.mleku.dev/mleku/nostr/crypto/keys"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/database"
	_ "next.orly.dev/pkg/database/grpc" // Import for grpc factory registration
	neo4jdb "next.orly.dev/pkg/neo4j"   // Import for neo4j factory and type
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/ratelimit"
	"next.orly.dev/pkg/utils/interrupt"
	"next.orly.dev/pkg/version"
)

func main() {
	runtime.GOMAXPROCS(128)
	debug.SetGCPercent(10)

	// Handle 'version' subcommand early, before any other initialization
	if config.VersionRequested() {
		fmt.Println(version.V)
		os.Exit(0)
	}

	var err error
	var cfg *config.C
	if cfg, err = config.New(); chk.T(err) {
	}
	log.I.F("starting %s %s", cfg.AppName, version.V)

	// Handle 'init-branding' subcommand: create branding directory with default assets
	if requested, targetDir, style := config.InitBrandingRequested(); requested {
		if targetDir == "" {
			targetDir = filepath.Join(xdg.ConfigHome, cfg.AppName, "branding")
		}

		// Validate and convert style
		var brandingStyle branding.BrandingStyle
		switch style {
		case "orly":
			brandingStyle = branding.StyleORLY
		case "generic", "":
			brandingStyle = branding.StyleGeneric
		default:
			fmt.Fprintf(os.Stderr, "Unknown style: %s (use 'orly' or 'generic')\n", style)
			os.Exit(1)
		}

		fmt.Printf("Initializing %s branding kit at: %s\n", style, targetDir)
		if err := branding.InitBrandingKit(targetDir, app.GetEmbeddedWebFS(), brandingStyle); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\nBranding kit created successfully!")
		fmt.Println("\nFiles created:")
		fmt.Println("  branding.json     - Main configuration file")
		fmt.Println("  assets/           - Logo, favicon, and PWA icons")
		fmt.Println("  css/custom.css    - Full CSS override template")
		fmt.Println("  css/variables.css - CSS variables-only template")
		fmt.Println("\nEdit these files to customize your relay's appearance.")
		fmt.Println("Restart the relay to apply changes.")
		os.Exit(0)
	}

	// Handle 'identity' subcommand: print relay identity secret and pubkey and exit
	if config.IdentityRequested() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var db database.Database
		if db, err = database.NewDatabaseWithConfig(
			ctx, cancel, cfg.DBType, makeDatabaseConfig(cfg),
		); chk.E(err) {
			os.Exit(1)
		}
		defer db.Close()
		skb, err := db.GetOrCreateRelayIdentitySecret()
		if chk.E(err) {
			os.Exit(1)
		}
		pk, err := keys.SecretBytesToPubKeyHex(skb)
		if chk.E(err) {
			os.Exit(1)
		}
		fmt.Printf(
			"identity secret: %s\nidentity pubkey: %s\n", hex.Enc(skb), pk,
		)
		os.Exit(0)
	}

	// Handle 'migrate' subcommand: migrate data between database backends
	if requested, fromType, toType, targetPath := config.MigrateRequested(); requested {
		if fromType == "" || toType == "" {
			fmt.Println("Usage: orly migrate --from <type> --to <type> [--target-path <path>]")
			fmt.Println("")
			fmt.Println("Migrate data between database backends.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("  --from <type>         Source database type (badger, neo4j)")
			fmt.Println("  --to <type>           Destination database type (badger, neo4j)")
			fmt.Println("  --target-path <path>  Optional: destination data directory")
			fmt.Println("                        (default: $ORLY_DATA_DIR/<type>)")
			fmt.Println("")
			fmt.Println("Examples:")
			fmt.Println("  orly migrate --from badger --to neo4j")
			fmt.Println("  orly migrate --from badger --to neo4j --target-path /mnt/hdd/orly-neo4j")
			os.Exit(1)
		}

		// Set target path if not specified
		if targetPath == "" {
			targetPath = cfg.DataDir + "-" + toType
		}

		log.I.F("migrate: %s -> %s", fromType, toType)
		log.I.F("migrate: source path: %s", cfg.DataDir)
		log.I.F("migrate: target path: %s", targetPath)

		// Open source database
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		srcCfg := makeDatabaseConfig(cfg)
		var srcDB database.Database
		if srcDB, err = database.NewDatabaseWithConfig(ctx, cancel, fromType, srcCfg); chk.E(err) {
			log.E.F("migrate: failed to open source database: %v", err)
			os.Exit(1)
		}

		// Wait for source database to be ready
		select {
		case <-srcDB.Ready():
			log.I.F("migrate: source database ready")
		case <-time.After(60 * time.Second):
			log.E.F("migrate: timeout waiting for source database")
			os.Exit(1)
		}

		// Open destination database
		dstCfg := makeDatabaseConfig(cfg)
		dstCfg.DataDir = targetPath
		var dstDB database.Database
		if dstDB, err = database.NewDatabaseWithConfig(ctx, cancel, toType, dstCfg); chk.E(err) {
			log.E.F("migrate: failed to open destination database: %v", err)
			srcDB.Close()
			os.Exit(1)
		}

		// Wait for destination database to be ready
		select {
		case <-dstDB.Ready():
			log.I.F("migrate: destination database ready")
		case <-time.After(60 * time.Second):
			log.E.F("migrate: timeout waiting for destination database")
			srcDB.Close()
			os.Exit(1)
		}

		// Migrate using pipe (export from source, import to destination)
		log.I.F("migrate: starting data transfer...")
		pr, pw, pipeErr := os.Pipe()
		if pipeErr != nil {
			log.E.F("migrate: failed to create pipe: %v", pipeErr)
			srcDB.Close()
			dstDB.Close()
			os.Exit(1)
		}

		var wg sync.WaitGroup
		wg.Add(2)

		// Export goroutine
		go func() {
			defer wg.Done()
			defer pw.Close()
			srcDB.Export(ctx, pw)
			log.I.F("migrate: export complete")
		}()

		// Import goroutine
		go func() {
			defer wg.Done()
			if importErr := dstDB.ImportEventsFromReader(ctx, pr); importErr != nil {
				log.E.F("migrate: import error: %v", importErr)
			}
			log.I.F("migrate: import complete")
		}()

		wg.Wait()

		// Sync and close databases
		if err = dstDB.Sync(); chk.E(err) {
			log.W.F("migrate: sync warning: %v", err)
		}
		srcDB.Close()
		dstDB.Close()

		log.I.F("migrate: migration complete!")
		os.Exit(0)
	}

	// Handle 'nrc' subcommand: NRC (Nostr Relay Connect) utilities
	if requested, subcommand, args := config.NRCRequested(); requested {
		handleNRCCommand(cfg, subcommand, args)
		os.Exit(0)
	}

	// Handle 'serve' subcommand: start ephemeral relay with RAM-based storage
	if config.ServeRequested() {
		const serveDataDir = "/dev/shm/orlyserve"
		log.I.F("serve mode: configuring ephemeral relay at %s", serveDataDir)

		// Delete existing directory completely
		if err = os.RemoveAll(serveDataDir); err != nil && !os.IsNotExist(err) {
			log.E.F("failed to remove existing serve directory: %v", err)
			os.Exit(1)
		}

		// Create fresh directory
		if err = os.MkdirAll(serveDataDir, 0755); chk.E(err) {
			log.E.F("failed to create serve directory: %v", err)
			os.Exit(1)
		}

		// Override configuration for serve mode
		cfg.DataDir = serveDataDir
		cfg.Listen = "0.0.0.0"
		cfg.Port = 10547
		cfg.ACLMode = "none"
		cfg.ServeMode = true // Grant full owner access to all users

		log.I.F("serve mode: listening on %s:%d with ACL mode '%s' (full owner access)",
			cfg.Listen, cfg.Port, cfg.ACLMode)
	}

	// Handle 'curatingmode' subcommand: start relay in curating mode with specified owner
	if requested, ownerKey := config.CuratingModeRequested(); requested {
		if ownerKey == "" {
			fmt.Println("Usage: orly curatingmode <npub|hex_pubkey>")
			fmt.Println("")
			fmt.Println("Starts the relay in curating mode with the specified pubkey as owner.")
			fmt.Println("Opens a browser to the curation setup page where you must log in")
			fmt.Println("with a Nostr extension to configure the relay.")
			fmt.Println("")
			fmt.Println("Press Escape or Ctrl+C to stop the relay.")
			os.Exit(1)
		}

		// Parse the owner key (npub or hex)
		var ownerHex string
		if strings.HasPrefix(ownerKey, "npub1") {
			// Decode npub to hex
			_, pubBytes, err := bech32encoding.Decode([]byte(ownerKey))
			if err != nil {
				fmt.Printf("Error: invalid npub: %v\n", err)
				os.Exit(1)
			}
			if pb, ok := pubBytes.([]byte); ok {
				ownerHex = hex.Enc(pb)
			} else {
				fmt.Println("Error: invalid npub encoding")
				os.Exit(1)
			}
		} else if len(ownerKey) == 64 {
			// Assume hex pubkey
			ownerHex = strings.ToLower(ownerKey)
		} else {
			fmt.Println("Error: owner key must be an npub or 64-character hex pubkey")
			os.Exit(1)
		}

		// Configure for curating mode
		cfg.ACLMode = "curating"
		cfg.Owners = []string{ownerHex}

		log.I.F("curatingmode: starting with owner %s", ownerHex)
		log.I.F("curatingmode: listening on %s:%d", cfg.Listen, cfg.Port)

		// Start a goroutine to open browser after a short delay
		go func() {
			time.Sleep(2 * time.Second)
			url := fmt.Sprintf("http://%s:%d/#curation", cfg.Listen, cfg.Port)
			log.I.F("curatingmode: opening browser to %s", url)
			openBrowser(url)
		}()

		// Start a goroutine to listen for Escape key
		go func() {
			// Set terminal to raw mode to capture individual key presses
			oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				log.W.F("could not set terminal to raw mode: %v", err)
				return
			}
			defer term.Restore(int(os.Stdin.Fd()), oldState)

			buf := make([]byte, 1)
			for {
				_, err := os.Stdin.Read(buf)
				if err != nil {
					return
				}
				// Escape key is 0x1b (27)
				if buf[0] == 0x1b {
					fmt.Println("\nEscape pressed, shutting down...")
					p, _ := os.FindProcess(os.Getpid())
					_ = p.Signal(os.Interrupt)
					return
				}
			}
		}()

		fmt.Println("")
		fmt.Println("Curating Mode Setup")
		fmt.Println("===================")
		fmt.Printf("Owner: %s\n", ownerHex)
		fmt.Printf("URL:   http://%s:%d/#curation\n", cfg.Listen, cfg.Port)
		fmt.Println("")
		fmt.Println("Log in with your Nostr extension to configure allowed event kinds")
		fmt.Println("and rate limiting settings.")
		fmt.Println("")
		fmt.Println("Press Escape or Ctrl+C to stop the relay.")
		fmt.Println("")
	}

	// Ensure profiling is stopped on interrupts (SIGINT/SIGTERM) as well as on normal exit
	var profileStopOnce sync.Once
	profileStop := func() {}
	switch cfg.Pprof {
	case "cpu":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.CPUProfile, profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("cpu profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.CPUProfile)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("cpu profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "memory":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MemProfile, profile.MemProfileRate(32),
				profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("memory profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.MemProfile)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("memory profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "allocation":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MemProfileAllocs, profile.MemProfileRate(32),
				profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("allocation profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.MemProfileAllocs)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("allocation profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "heap":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MemProfileHeap, profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("heap profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.MemProfileHeap)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("heap profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "mutex":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.MutexProfile, profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("mutex profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.MutexProfile)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("mutex profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "threadcreate":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.ThreadcreationProfile,
				profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("threadcreate profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.ThreadcreationProfile)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("threadcreate profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "goroutine":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.GoroutineProfile, profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("goroutine profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.GoroutineProfile)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("goroutine profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}
	case "block":
		if cfg.PprofPath != "" {
			prof := profile.Start(
				profile.BlockProfile, profile.ProfilePath(cfg.PprofPath),
			)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("block profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		} else {
			prof := profile.Start(profile.BlockProfile)
			profileStop = func() {
				profileStopOnce.Do(
					func() {
						prof.Stop()
						log.I.F("block profiling stopped and flushed")
					},
				)
			}
			defer profileStop()
		}

	}

	// Register a handler so profiling is stopped when an interrupt is received
	interrupt.AddHandler(
		func() {
			log.I.F("interrupt received: stopping profiling")
			profileStop()
		},
	)
	ctx, cancel := context.WithCancel(context.Background())
	var db database.Database
	log.I.F("initializing %s database at %s", cfg.DBType, cfg.DataDir)
	if db, err = database.NewDatabaseWithConfig(
		ctx, cancel, cfg.DBType, makeDatabaseConfig(cfg),
	); chk.E(err) {
		os.Exit(1)
	}
	log.I.F("%s database initialized successfully", cfg.DBType)

	// Initialize ACL - either remote gRPC or in-process
	aclType, aclServerAddr, aclConnTimeout := cfg.GetGRPCACLConfigValues()
	if aclType == "grpc" {
		// Use remote ACL server via gRPC
		log.I.F("connecting to gRPC ACL server at %s", aclServerAddr)
		aclClient, aclErr := aclgrpc.New(ctx, &aclgrpc.ClientConfig{
			ServerAddress:  aclServerAddr,
			ConnectTimeout: aclConnTimeout,
		})
		if chk.E(aclErr) {
			log.E.F("failed to connect to gRPC ACL server: %v", aclErr)
			os.Exit(1)
		}
		// Wait for ACL server to be ready
		select {
		case <-aclClient.Ready():
			log.I.F("gRPC ACL client connected, mode: %s", aclClient.Type())
		case <-time.After(30 * time.Second):
			log.E.F("timeout waiting for gRPC ACL server")
			os.Exit(1)
		}
		// Register and activate the gRPC client as the ACL backend
		acl.Registry.RegisterAndActivate(aclClient)
	} else {
		// Use in-process ACL (existing behavior)
		acl.Registry.SetMode(cfg.ACLMode)
		if err = acl.Registry.Configure(cfg, db, ctx); chk.E(err) {
			os.Exit(1)
		}
		acl.Registry.Syncer()
	}

	// Initialize negentropy client if enabled (gRPC mode)
	syncType, _, _, _, negentropyAddr, syncTimeout, negentropyEnabled := cfg.GetGRPCSyncConfigValues()
	if negentropyEnabled && syncType == "grpc" && negentropyAddr != "" {
		log.I.F("connecting to gRPC negentropy server at %s", negentropyAddr)
		negClient, negErr := negentropygrpc.New(ctx, &negentropygrpc.ClientConfig{
			ServerAddress:  negentropyAddr,
			ConnectTimeout: syncTimeout,
		})
		if negErr != nil {
			log.W.F("failed to connect to gRPC negentropy server: %v (NIP-77 disabled)", negErr)
		} else {
			// Wait for negentropy server to be ready
			select {
			case <-negClient.Ready():
				log.I.F("gRPC negentropy client connected")
				app.SetNegentropyClient(negClient)
			case <-time.After(30 * time.Second):
				log.W.F("timeout waiting for gRPC negentropy server (NIP-77 disabled)")
			}
		}
	} else if negentropyEnabled {
		log.I.F("negentropy enabled but sync type is %q, skipping gRPC client", syncType)
	}

	// Create rate limiter if enabled
	var limiter *ratelimit.Limiter
	rateLimitEnabled, targetMB,
		writeKp, writeKi, writeKd,
		readKp, readKi, readKd,
		maxWriteMs, maxReadMs,
		writeTarget, readTarget,
		emergencyThreshold, recoveryThreshold,
		emergencyMaxMs := cfg.GetRateLimitConfigValues()

	if rateLimitEnabled {
		// Auto-detect memory target if set to 0 (default)
		if targetMB == 0 {
			var memErr error
			targetMB, memErr = ratelimit.CalculateTargetMemoryMB(targetMB)
			if memErr != nil {
				log.F.F("FATAL: %v", memErr)
				log.F.F("There is not enough memory to run this relay in this environment.")
				log.F.F("Available: %dMB, Required minimum: %dMB",
					ratelimit.DetectAvailableMemoryMB(), ratelimit.MinimumMemoryMB)
				os.Exit(1)
			}
			stats := ratelimit.GetMemoryStats(targetMB)
			// Calculate what 66% would be to determine if we hit the cap
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
			_, memErr := ratelimit.CalculateTargetMemoryMB(targetMB)
			if memErr != nil {
				log.F.F("FATAL: %v", memErr)
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
			limiter = ratelimit.NewBadgerLimiter(rlConfig, badgerDB.DB)
			// Set the rate limiter on the database for import operations
			badgerDB.SetRateLimiter(limiter)
			log.I.F("rate limiter configured for Badger backend (target: %dMB)", targetMB)
		} else if n4jDB, ok := db.(*neo4jdb.N); ok {
			// Create Neo4j rate limiter with access to driver and querySem
			limiter = ratelimit.NewNeo4jLimiter(
				rlConfig,
				n4jDB.Driver(),
				n4jDB.QuerySem(),
				n4jDB.MaxConcurrentQueries(),
			)
			log.I.F("rate limiter configured for Neo4j backend (target: %dMB)", targetMB)
		} else {
			// For other backends, create a disabled limiter
			limiter = ratelimit.NewDisabledLimiter()
			log.I.F("rate limiter disabled for unknown backend")
		}
	} else {
		limiter = ratelimit.NewDisabledLimiter()
	}

	// Start HTTP pprof server if enabled
	if cfg.PprofHTTP {
		pprofAddr := fmt.Sprintf("%s:%d", cfg.Listen, 6060)
		pprofMux := http.NewServeMux()
		pprofMux.HandleFunc("/debug/pprof/", pp.Index)
		pprofMux.HandleFunc("/debug/pprof/cmdline", pp.Cmdline)
		pprofMux.HandleFunc("/debug/pprof/profile", pp.Profile)
		pprofMux.HandleFunc("/debug/pprof/symbol", pp.Symbol)
		pprofMux.HandleFunc("/debug/pprof/trace", pp.Trace)
		for _, p := range []string{
			"allocs", "block", "goroutine", "heap", "mutex", "threadcreate",
		} {
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
			shutdownCtx, cancelShutdown := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancelShutdown()
			_ = ppSrv.Shutdown(shutdownCtx)
		}()
	}

	// Start health check HTTP server if configured
	var healthSrv *http.Server
	if cfg.HealthPort > 0 {
		mux := http.NewServeMux()
		mux.HandleFunc(
			"/healthz", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
				log.I.F("health check ok")
			},
		)
		// Optional shutdown endpoint to gracefully stop the process so profiling defers run
		if cfg.EnableShutdown {
			mux.HandleFunc(
				"/shutdown", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("shutting down"))
					log.I.F("shutdown requested via /shutdown; sending SIGINT to self")
					go func() {
						p, _ := os.FindProcess(os.Getpid())
						_ = p.Signal(os.Interrupt)
					}()
				},
			)
		}
		healthSrv = &http.Server{
			Addr: fmt.Sprintf(
				"%s:%d", cfg.Listen, cfg.HealthPort,
			), Handler: mux,
		}
		go func() {
			log.I.F("health check server listening on %s", healthSrv.Addr)
			if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.E.F("health server error: %v", err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancelShutdown := context.WithTimeout(
				context.Background(), 2*time.Second,
			)
			defer cancelShutdown()
			_ = healthSrv.Shutdown(shutdownCtx)
		}()
	}

	quit := app.Run(ctx, cfg, db, limiter)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	for {
		select {
		case <-sigs:
			fmt.Printf("\r")
			log.I.F("received shutdown signal, starting graceful shutdown")
			cancel() // This will trigger HTTP server shutdown
			<-quit   // Wait for HTTP server to shut down
			chk.E(db.Close())
			log.I.F("exiting")
			return
		case <-quit:
			log.I.F("application quit signal received")
			cancel()
			chk.E(db.Close())
			log.I.F("exiting")
			return
		}
	}
	// log.I.F("exiting")
}

// makeDatabaseConfig creates a database.DatabaseConfig from the app config.
// This helper function extracts all database-specific configuration values
// and constructs the appropriate struct for the database package.
func makeDatabaseConfig(cfg *config.C) *database.DatabaseConfig {
	dataDir, logLevel,
		blockCacheMB, indexCacheMB, queryCacheSizeMB,
		queryCacheMaxAge,
		queryCacheDisabled,
		serialCachePubkeys, serialCacheEventIds,
		zstdLevel,
		neo4jURI, neo4jUser, neo4jPassword,
		neo4jMaxConnPoolSize, neo4jFetchSize, neo4jMaxTxRetrySeconds, neo4jQueryResultLimit := cfg.GetDatabaseConfigValues()

	// Get gRPC client configuration
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
		// gRPC client settings
		GRPCServerAddress:  grpcServerAddress,
		GRPCConnectTimeout: grpcConnectTimeout,
	}
}

// openBrowser opens the specified URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.W.F("could not open browser: %v", err)
	}
}

// handleNRCCommand handles the 'nrc' CLI subcommand for NRC (Nostr Relay Connect) utilities.
func handleNRCCommand(cfg *config.C, subcommand string, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	switch subcommand {
	case "generate":
		handleNRCGenerate(ctx, cfg, args)
	case "list":
		handleNRCList(cfg)
	case "revoke":
		handleNRCRevoke(args)
	default:
		printNRCUsage()
	}
}

// printNRCUsage prints the usage information for the nrc subcommand.
func printNRCUsage() {
	fmt.Println("Usage: orly nrc <subcommand> [options]")
	fmt.Println("")
	fmt.Println("Nostr Relay Connect (NRC) utilities for private relay access.")
	fmt.Println("")
	fmt.Println("Subcommands:")
	fmt.Println("  generate [--name <device>]  Generate a new connection URI")
	fmt.Println("  list                        List currently configured authorized secrets")
	fmt.Println("  revoke <name>               Revoke access for a device (show instructions)")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  orly nrc generate")
	fmt.Println("  orly nrc generate --name phone")
	fmt.Println("  orly nrc list")
	fmt.Println("  orly nrc revoke phone")
	fmt.Println("")
	fmt.Println("To enable NRC, set these environment variables:")
	fmt.Println("  ORLY_NRC_ENABLED=true")
	fmt.Println("  ORLY_NRC_RENDEZVOUS_URL=wss://public-relay.example.com")
	fmt.Println("  ORLY_NRC_AUTHORIZED_KEYS=<secret1>:<name1>,<secret2>:<name2>")
}

// handleNRCGenerate generates a new NRC connection URI.
func handleNRCGenerate(ctx context.Context, cfg *config.C, args []string) {
	// Parse device name from args
	var deviceName string
	for i := 0; i < len(args); i++ {
		if args[i] == "--name" && i+1 < len(args) {
			deviceName = args[i+1]
			i++
		}
	}

	// Get relay identity
	var db database.Database
	var err error
	if db, err = database.NewDatabaseWithConfig(
		ctx, nil, cfg.DBType, makeDatabaseConfig(cfg),
	); chk.E(err) {
		fmt.Printf("Error: failed to open database: %v\n", err)
		return
	}
	defer db.Close()

	<-db.Ready()

	relaySecretKey, err := db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		fmt.Printf("Error: failed to get relay identity: %v\n", err)
		return
	}

	relayPubkey, err := keys.SecretBytesToPubKeyBytes(relaySecretKey)
	if err != nil {
		fmt.Printf("Error: failed to derive relay pubkey: %v\n", err)
		return
	}

	// Get rendezvous URL from config
	nrcEnabled, nrcRendezvousURL, _, _ := cfg.GetNRCConfigValues()
	if !nrcEnabled || nrcRendezvousURL == "" {
		fmt.Println("Error: NRC is not configured. Set ORLY_NRC_ENABLED=true and ORLY_NRC_RENDEZVOUS_URL")
		return
	}

	// Generate a new random secret
	secret := make([]byte, 32)
	if _, err := os.ReadFile("/dev/urandom"); err != nil {
		// Fallback - use crypto/rand
		fmt.Printf("Error: failed to generate random secret: %v\n", err)
		return
	}
	f, _ := os.Open("/dev/urandom")
	defer f.Close()
	f.Read(secret)

	secretHex := hex.Enc(secret)

	// Build the URI
	uri := fmt.Sprintf("nostr+relayconnect://%s?relay=%s&secret=%s",
		hex.Enc(relayPubkey), nrcRendezvousURL, secretHex)
	if deviceName != "" {
		uri += fmt.Sprintf("&name=%s", deviceName)
	}

	fmt.Println("Generated NRC Connection URI:")
	fmt.Println("")
	fmt.Println(uri)
	fmt.Println("")
	fmt.Println("Add this secret to ORLY_NRC_AUTHORIZED_KEYS:")
	if deviceName != "" {
		fmt.Printf("  %s:%s\n", secretHex, deviceName)
	} else {
		fmt.Printf("  %s\n", secretHex)
	}
	fmt.Println("")
	fmt.Println("IMPORTANT: Store this URI securely - anyone with this URI can access your relay.")
}

// handleNRCList lists configured authorized secrets from environment.
func handleNRCList(cfg *config.C) {
	_, _, authorizedKeys, _ := cfg.GetNRCConfigValues()

	fmt.Println("NRC Configuration:")
	fmt.Println("")

	if len(authorizedKeys) == 0 {
		fmt.Println("  No authorized secrets configured.")
		fmt.Println("")
		fmt.Println("  To add secrets, set ORLY_NRC_AUTHORIZED_KEYS=<secret>:<name>,...")
	} else {
		fmt.Printf("  Authorized secrets: %d\n", len(authorizedKeys))
		fmt.Println("")
		for _, entry := range authorizedKeys {
			parts := strings.SplitN(entry, ":", 2)
			secretHex := parts[0]
			name := "(unnamed)"
			if len(parts) == 2 && parts[1] != "" {
				name = parts[1]
			}
			// Show truncated secret for identification
			truncated := secretHex
			if len(secretHex) > 16 {
				truncated = secretHex[:8] + "..." + secretHex[len(secretHex)-8:]
			}
			fmt.Printf("  - %s: %s\n", name, truncated)
		}
	}
}

// handleNRCRevoke provides instructions for revoking access.
func handleNRCRevoke(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: orly nrc revoke <device-name>")
		fmt.Println("")
		fmt.Println("To revoke access for a device:")
		fmt.Println("1. Remove the corresponding secret from ORLY_NRC_AUTHORIZED_KEYS")
		fmt.Println("2. Restart the relay")
		fmt.Println("")
		fmt.Println("Example: If ORLY_NRC_AUTHORIZED_KEYS=\"abc123:phone,def456:laptop\"")
		fmt.Println("To revoke 'phone', change to: ORLY_NRC_AUTHORIZED_KEYS=\"def456:laptop\"")
		return
	}

	deviceName := args[0]
	fmt.Printf("To revoke access for '%s':\n", deviceName)
	fmt.Println("")
	fmt.Println("1. Edit ORLY_NRC_AUTHORIZED_KEYS and remove the entry for this device")
	fmt.Println("2. Restart the relay")
	fmt.Println("")
	fmt.Println("The device will no longer be able to connect after the restart.")
}
