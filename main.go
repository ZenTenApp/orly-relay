package main

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
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/crypto/keys"
	"next.orly.dev/pkg/database"
	_ "next.orly.dev/pkg/dgraph" // Import to register dgraph factory
	_ "next.orly.dev/pkg/neo4j"  // Import to register neo4j factory
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/utils/interrupt"
	"next.orly.dev/pkg/version"
)

func main() {
	runtime.GOMAXPROCS(128)
	debug.SetGCPercent(10)
	var err error
	var cfg *config.C
	if cfg, err = config.New(); chk.T(err) {
	}
	log.I.F("starting %s %s", cfg.AppName, version.V)

	// Handle 'identity' subcommand: print relay identity secret and pubkey and exit
	if config.IdentityRequested() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var db database.Database
		if db, err = database.NewDatabase(
			ctx, cancel, cfg.DBType, cfg.DataDir, cfg.DBLogLevel,
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
	if db, err = database.NewDatabase(
		ctx, cancel, cfg.DBType, cfg.DataDir, cfg.DBLogLevel,
	); chk.E(err) {
		os.Exit(1)
	}
	log.I.F("%s database initialized successfully", cfg.DBType)
	acl.Registry.Active.Store(cfg.ACLMode)
	if err = acl.Registry.Configure(cfg, db, ctx); chk.E(err) {
		os.Exit(1)
	}
	acl.Registry.Syncer()

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

	quit := app.Run(ctx, cfg, db)
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
