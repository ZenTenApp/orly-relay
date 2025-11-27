package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/crypto/keys"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/spider"
	dsync "next.orly.dev/pkg/sync"
)

func Run(
	ctx context.Context, cfg *config.C, db database.Database,
) (quit chan struct{}) {
	quit = make(chan struct{})
	var once sync.Once

	// shutdown handler
	go func() {
		<-ctx.Done()
		log.I.F("shutting down")
		once.Do(func() { close(quit) })
	}()
	// get the admins
	var err error
	var adminKeys [][]byte
	for _, admin := range cfg.Admins {
		if len(admin) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(admin); chk.E(err) {
			continue
		}
		adminKeys = append(adminKeys, pk)
	}
	// get the owners
	var ownerKeys [][]byte
	for _, owner := range cfg.Owners {
		if len(owner) == 0 {
			continue
		}
		var pk []byte
		if pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(owner); chk.E(err) {
			continue
		}
		ownerKeys = append(ownerKeys, pk)
	}
	// start listener
	l := &Server{
		Ctx:        ctx,
		Config:     cfg,
		DB:         db,
		publishers: publish.New(NewPublisher(ctx)),
		Admins:     adminKeys,
		Owners:     ownerKeys,
		cfg:        cfg,
		db:         db,
	}

	// Initialize NIP-43 invite manager if enabled
	if cfg.NIP43Enabled {
		l.InviteManager = nip43.NewInviteManager(cfg.NIP43InviteExpiry)
		log.I.F("NIP-43 invite system enabled with %v expiry", cfg.NIP43InviteExpiry)
	}

	// Initialize sprocket manager
	l.sprocketManager = NewSprocketManager(ctx, cfg.AppName, cfg.SprocketEnabled)

	// Initialize policy manager
	l.policyManager = policy.NewWithManager(ctx, cfg.AppName, cfg.PolicyEnabled)

	// Initialize spider manager based on mode (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok && cfg.SpiderMode != "none" {
		if l.spiderManager, err = spider.New(ctx, badgerDB, l.publishers, cfg.SpiderMode); chk.E(err) {
			log.E.F("failed to create spider manager: %v", err)
		} else {
			// Set up callbacks for follows mode
			if cfg.SpiderMode == "follows" {
				l.spiderManager.SetCallbacks(
					func() []string {
						// Get admin relays from follows ACL if available
						for _, aclInstance := range acl.Registry.ACL {
							if aclInstance.Type() == "follows" {
								if follows, ok := aclInstance.(*acl.Follows); ok {
									return follows.AdminRelays()
								}
							}
						}
						return nil
					},
					func() [][]byte {
						// Get followed pubkeys from follows ACL if available
						for _, aclInstance := range acl.Registry.ACL {
							if aclInstance.Type() == "follows" {
								if follows, ok := aclInstance.(*acl.Follows); ok {
									return follows.GetFollowedPubkeys()
								}
							}
						}
						return nil
					},
				)
			}

			if err = l.spiderManager.Start(); chk.E(err) {
				log.E.F("failed to start spider manager: %v", err)
			} else {
				log.I.F("spider manager started successfully in '%s' mode", cfg.SpiderMode)

				// Hook up follow list update notifications from ACL to spider
				if cfg.SpiderMode == "follows" {
					for _, aclInstance := range acl.Registry.ACL {
						if aclInstance.Type() == "follows" {
							if follows, ok := aclInstance.(*acl.Follows); ok {
								follows.SetFollowListUpdateCallback(func() {
									log.I.F("follow list updated, notifying spider")
									l.spiderManager.NotifyFollowListUpdate()
								})
								log.I.F("spider: follow list update notifications configured")
							}
						}
					}
				}
			}
		}
	}

	// Initialize directory spider if enabled (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok && cfg.DirectorySpiderEnabled {
		if l.directorySpider, err = spider.NewDirectorySpider(
			ctx,
			badgerDB,
			l.publishers,
			cfg.DirectorySpiderInterval,
			cfg.DirectorySpiderMaxHops,
		); chk.E(err) {
			log.E.F("failed to create directory spider: %v", err)
		} else {
			// Set up callback to get seed pubkeys (whitelisted users)
			l.directorySpider.SetSeedCallback(func() [][]byte {
				var pubkeys [][]byte
				// Get followed pubkeys from follows ACL if available
				for _, aclInstance := range acl.Registry.ACL {
					if aclInstance.Type() == "follows" {
						if follows, ok := aclInstance.(*acl.Follows); ok {
							pubkeys = append(pubkeys, follows.GetFollowedPubkeys()...)
						}
					}
				}
				// Fall back to admin keys if no follows ACL
				if len(pubkeys) == 0 {
					pubkeys = adminKeys
				}
				return pubkeys
			})

			if err = l.directorySpider.Start(); chk.E(err) {
				log.E.F("failed to start directory spider: %v", err)
			} else {
				log.I.F("directory spider started (interval: %v, max hops: %d)",
					cfg.DirectorySpiderInterval, cfg.DirectorySpiderMaxHops)
			}
		}
	}

	// Initialize relay group manager (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok {
		l.relayGroupMgr = dsync.NewRelayGroupManager(badgerDB, cfg.RelayGroupAdmins)
	} else if cfg.SpiderMode != "none" || len(cfg.RelayPeers) > 0 || len(cfg.ClusterAdmins) > 0 {
		log.I.Ln("spider, sync, and cluster features require Badger backend (currently using alternative backend)")
	}

	// Initialize sync manager if relay peers are configured (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok {
		var peers []string
		if len(cfg.RelayPeers) > 0 {
			peers = cfg.RelayPeers
		} else {
			// Try to get peers from relay group configuration
			if l.relayGroupMgr != nil {
				if config, err := l.relayGroupMgr.FindAuthoritativeConfig(ctx); err == nil && config != nil {
					peers = config.Relays
					log.I.F("using relay group configuration with %d peers", len(peers))
				}
			}
		}

		if len(peers) > 0 {
			// Get relay identity for node ID
			sk, err := db.GetOrCreateRelayIdentitySecret()
			if err != nil {
				log.E.F("failed to get relay identity for sync: %v", err)
			} else {
				nodeID, err := keys.SecretBytesToPubKeyHex(sk)
				if err != nil {
					log.E.F("failed to derive pubkey for sync node ID: %v", err)
				} else {
					relayURL := cfg.RelayURL
					if relayURL == "" {
						relayURL = fmt.Sprintf("http://localhost:%d", cfg.Port)
					}
					l.syncManager = dsync.NewManager(ctx, badgerDB, nodeID, relayURL, peers, l.relayGroupMgr, l.policyManager)
					log.I.F("distributed sync manager initialized with %d peers", len(peers))
				}
			}
		}
	}

	// Initialize cluster manager for cluster replication (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok {
		var clusterAdminNpubs []string
		if len(cfg.ClusterAdmins) > 0 {
			clusterAdminNpubs = cfg.ClusterAdmins
		} else {
			// Default to regular admins if no cluster admins specified
			for _, admin := range cfg.Admins {
				clusterAdminNpubs = append(clusterAdminNpubs, admin)
			}
		}

		if len(clusterAdminNpubs) > 0 {
			l.clusterManager = dsync.NewClusterManager(ctx, badgerDB, clusterAdminNpubs, cfg.ClusterPropagatePrivilegedEvents, l.publishers)
			l.clusterManager.Start()
			log.I.F("cluster replication manager initialized with %d admin npubs", len(clusterAdminNpubs))
		}
	}

	// Initialize Blossom blob storage server (only for Badger backend)
	// MUST be done before UserInterface() which registers routes
	if badgerDB, ok := db.(*database.D); ok {
		log.I.F("Badger backend detected, initializing Blossom server...")
		if l.blossomServer, err = initializeBlossomServer(ctx, cfg, badgerDB); err != nil {
			log.E.F("failed to initialize blossom server: %v", err)
			// Continue without blossom server
		} else if l.blossomServer != nil {
			log.I.F("blossom blob storage server initialized")
		} else {
			log.W.F("blossom server initialization returned nil without error")
		}
	} else {
		log.I.F("Non-Badger backend detected (type: %T), Blossom server not available", db)
	}

	// Initialize the user interface (registers routes)
	l.UserInterface()

	// Ensure a relay identity secret key exists when subscriptions and NWC are enabled
	if cfg.SubscriptionEnabled && cfg.NWCUri != "" {
		if skb, e := db.GetOrCreateRelayIdentitySecret(); e != nil {
			log.E.F("failed to ensure relay identity key: %v", e)
		} else if pk, e2 := keys.SecretBytesToPubKeyHex(skb); e2 == nil {
			log.I.F("relay identity loaded (pub=%s)", pk)
			// ensure relay identity pubkey is considered an admin for ACL follows mode
			found := false
			for _, a := range cfg.Admins {
				if a == pk {
					found = true
					break
				}
			}
			if !found {
				cfg.Admins = append(cfg.Admins, pk)
				log.I.F("added relay identity to admins for follow-list whitelisting")
			}
			// also ensure relay identity pubkey is considered an owner for full control
			found = false
			for _, o := range cfg.Owners {
				if o == pk {
					found = true
					break
				}
			}
			if !found {
				cfg.Owners = append(cfg.Owners, pk)
				log.I.F("added relay identity to owners for full control")
			}
		}
	}

	// Initialize payment processor (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok {
		if l.paymentProcessor, err = NewPaymentProcessor(ctx, cfg, badgerDB); err != nil {
			// log.E.F("failed to create payment processor: %v", err)
			// Continue without payment processor
		} else {
			if err = l.paymentProcessor.Start(); err != nil {
				log.E.F("failed to start payment processor: %v", err)
			} else {
				log.I.F("payment processor started successfully")
			}
		}
	}

	// Wait for database to be ready before accepting requests
	log.I.F("waiting for database warmup to complete...")
	<-db.Ready()
	log.I.F("database ready, starting HTTP servers")

	// Check if TLS is enabled
	var tlsEnabled bool
	var tlsServer *http.Server
	var httpServer *http.Server

	if len(cfg.TLSDomains) > 0 {
		// Validate TLS configuration
		if err = ValidateTLSConfig(cfg.TLSDomains, cfg.Certs); chk.E(err) {
			log.E.F("invalid TLS configuration: %v", err)
		} else {
			tlsEnabled = true
			log.I.F("TLS enabled for domains: %v", cfg.TLSDomains)

			// Create cache directory for autocert
			cacheDir := filepath.Join(cfg.DataDir, "autocert")
			if err = os.MkdirAll(cacheDir, 0700); chk.E(err) {
				log.E.F("failed to create autocert cache directory: %v", err)
				tlsEnabled = false
			} else {
				// Set up autocert manager
				m := &autocert.Manager{
					Prompt:     autocert.AcceptTOS,
					Cache:      autocert.DirCache(cacheDir),
					HostPolicy: autocert.HostWhitelist(cfg.TLSDomains...),
				}

				// Create TLS server on port 443
				tlsServer = &http.Server{
					Addr:      ":443",
					Handler:   l,
					TLSConfig: TLSConfig(m, cfg.Certs...),
				}

				// Create HTTP server for ACME challenges and redirects on port 80
				httpServer = &http.Server{
					Addr:    ":80",
					Handler: m.HTTPHandler(nil),
				}

				// Start TLS server
				go func() {
					log.I.F("starting TLS listener on https://:443")
					if err := tlsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
						log.E.F("TLS server error: %v", err)
					}
				}()

				// Start HTTP server for ACME challenges
				go func() {
					log.I.F("starting HTTP listener on http://:80 for ACME challenges")
					if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
						log.E.F("HTTP server error: %v", err)
					}
				}()
			}
		}
	}

	// Start regular HTTP server if TLS is not enabled or as fallback
	if !tlsEnabled {
		addr := fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port)
		log.I.F("starting listener on http://%s", addr)

		httpServer = &http.Server{
			Addr:    addr,
			Handler: l,
		}

		go func() {
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.E.F("HTTP server error: %v", err)
			}
		}()
	}

	// Graceful shutdown handler
	go func() {
		<-ctx.Done()
		log.I.F("shutting down servers gracefully")

		// Stop spider manager if running
		if l.spiderManager != nil {
			l.spiderManager.Stop()
			log.I.F("spider manager stopped")
		}

		// Stop directory spider if running
		if l.directorySpider != nil {
			l.directorySpider.Stop()
			log.I.F("directory spider stopped")
		}

		// Create shutdown context with timeout
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()

		// Shutdown TLS server if running
		if tlsServer != nil {
			if err := tlsServer.Shutdown(shutdownCtx); err != nil {
				log.E.F("TLS server shutdown error: %v", err)
			} else {
				log.I.F("TLS server shutdown completed")
			}
		}

		// Shutdown HTTP server
		if httpServer != nil {
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				log.E.F("HTTP server shutdown error: %v", err)
			} else {
				log.I.F("HTTP server shutdown completed")
			}
		}

		once.Do(func() { close(quit) })
	}()

	return
}
