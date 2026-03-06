package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/app/branding"
	"next.orly.dev/app/config"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/nostr/crypto/keys"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/neo4j"
	"next.orly.dev/pkg/policy"
	"next.orly.dev/pkg/protocol/graph"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/bunker"
	"next.orly.dev/pkg/protocol/nrc"
	"next.orly.dev/pkg/ratelimit"
	"next.orly.dev/pkg/crawler"
	"next.orly.dev/pkg/spider"
	"next.orly.dev/pkg/storage"
	dsync "next.orly.dev/pkg/sync"
	"next.orly.dev/pkg/transport"
	"next.orly.dev/pkg/transport/tcp"
	tlstransport "next.orly.dev/pkg/transport/tls"
	tortransport "next.orly.dev/pkg/transport/tor"
	"next.orly.dev/pkg/wireguard"
	"next.orly.dev/pkg/archive"
	emailbridge "next.orly.dev/pkg/bridge"
	"next.orly.dev/pkg/httpguard"

	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
)

func Run(
	ctx context.Context, cfg *config.C, db database.Database, limiter *ratelimit.Limiter,
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
	channelMembership := NewChannelMembership(db)
	dmLimiter := NewDMRateLimiter(db)
	wsPublisher := NewPublisher(ctx)
	wsPublisher.ChannelMembership = channelMembership

	l := &Server{
		Ctx:               ctx,
		Config:            cfg,
		DB:                db,
		publishers:        publish.New(wsPublisher),
		Admins:            adminKeys,
		Owners:            ownerKeys,
		rateLimiter:       limiter,
		cfg:               cfg,
		db:                db,
		connPerIP:         make(map[string]int),
		aclRegistry:       acl.Registry, // Inject ACL registry (transitional from global)
		channelMembership: channelMembership,
		dmRateLimiter:     dmLimiter,
	}

	// Configure connection storm mitigation limits on the rate limiter
	if limiter != nil {
		limiter.SetConnectionLimits(
			cfg.MaxGlobalConnections,
			cfg.ConnectionDelayMaxMs,
			cfg.GoroutineWarningCount,
			cfg.GoroutineMaxCount,
		)
	}

	// Initialize HTTP guard (bot blocking + per-IP rate limiting)
	if cfg.HTTPGuardEnabled {
		l.httpGuard = httpguard.New(httpguard.Config{
			Enabled:     true,
			BotBlock:    cfg.HTTPGuardBotBlock,
			RPM:         cfg.HTTPGuardRPM,
			WSPerMin:    cfg.HTTPGuardWSPerMin,
			IPBlacklist: cfg.IPBlacklist,
		})
	}

	// Initialize branding/white-label manager if enabled
	if cfg.BrandingEnabled {
		brandingDir := cfg.BrandingDir
		if brandingDir == "" {
			brandingDir = filepath.Join(xdg.ConfigHome, cfg.AppName, "branding")
		}
		if _, err := os.Stat(brandingDir); err == nil {
			if l.brandingMgr, err = branding.New(brandingDir); err != nil {
				log.W.F("failed to load branding from %s: %v", brandingDir, err)
			} else {
				log.I.F("custom branding loaded from %s", brandingDir)
			}
		}
	}

	// Initialize NIP-43 invite manager if enabled
	if cfg.NIP43Enabled {
		l.InviteManager = nip43.NewInviteManager(cfg.NIP43InviteExpiry)
		log.I.F("NIP-43 invite system enabled with %v expiry", cfg.NIP43InviteExpiry)
	}

	// Initialize sprocket manager
	l.sprocketManager = NewSprocketManager(ctx, cfg.AppName, cfg.SprocketEnabled)

	// Initialize policy manager
	l.policyManager = policy.NewWithManager(ctx, cfg.AppName, cfg.PolicyEnabled, cfg.PolicyPath)

	// Merge policy-defined owners with environment-defined owners
	// This allows cloud deployments to add owners via policy.json when env vars cannot be modified
	if l.policyManager != nil {
		policyOwners := l.policyManager.GetOwnersBin()
		if len(policyOwners) > 0 {
			// Deduplicate when merging
			existingOwners := make(map[string]struct{})
			for _, owner := range l.Owners {
				existingOwners[string(owner)] = struct{}{}
			}
			for _, policyOwner := range policyOwners {
				if _, exists := existingOwners[string(policyOwner)]; !exists {
					l.Owners = append(l.Owners, policyOwner)
					existingOwners[string(policyOwner)] = struct{}{}
				}
			}
			log.I.F("merged %d policy-defined owners with %d environment-defined owners (total: %d unique owners)",
				len(policyOwners), len(ownerKeys), len(l.Owners))
		}
	}

	// Initialize policy follows from database (load follow lists of policy admins)
	// This must be done after policy manager initialization but before accepting connections
	if err := l.InitializePolicyFollows(); err != nil {
		log.W.F("failed to initialize policy follows: %v", err)
		// Continue anyway - follows can be loaded when admins update their follow lists
	}

	// Cleanup any kind 3 events that lost their p tags (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok {
		if err := badgerDB.CleanupKind3WithoutPTags(ctx); chk.E(err) {
			log.E.F("failed to cleanup kind 3 events: %v", err)
		}
	}

	// Initialize graph query executor (Badger backend) if enabled
	if badgerDB, ok := db.(*database.D); ok && cfg.GraphQueriesEnabled {
		// Get relay identity key for signing graph query responses
		relaySecretKey, err := badgerDB.GetOrCreateRelayIdentitySecret()
		if err != nil {
			log.E.F("failed to get relay identity key for graph executor: %v", err)
		} else {
			// Create the graph adapter and executor
			graphAdapter := database.NewGraphAdapter(badgerDB)
			if l.graphExecutor, err = graph.NewExecutor(graphAdapter, relaySecretKey); err != nil {
				log.E.F("failed to create graph executor: %v", err)
			} else {
				graphEnabled, maxDepth, maxResults, rateLimitRPM := cfg.GetGraphConfigValues()
				log.I.F("graph query executor initialized (Badger backend, enabled=%v, max_depth=%d, max_results=%d, rate_limit=%d/min)",
					graphEnabled, maxDepth, maxResults, rateLimitRPM)
			}
		}
	}

	// Initialize graph query executor (Neo4j backend) if enabled
	if neo4jDB, ok := db.(*neo4j.N); ok && cfg.GraphQueriesEnabled {
		// Get relay identity key for signing graph query responses
		relaySecretKey, err := neo4jDB.GetOrCreateRelayIdentitySecret()
		if err != nil {
			log.E.F("failed to get relay identity key for graph executor: %v", err)
		} else {
			// Create the graph adapter and executor
			graphAdapter := neo4j.NewGraphAdapter(neo4jDB)
			if l.graphExecutor, err = graph.NewExecutor(graphAdapter, relaySecretKey); err != nil {
				log.E.F("failed to create graph executor: %v", err)
			} else {
				graphEnabled, maxDepth, maxResults, rateLimitRPM := cfg.GetGraphConfigValues()
				log.I.F("graph query executor initialized (Neo4j backend, enabled=%v, max_depth=%d, max_results=%d, rate_limit=%d/min)",
					graphEnabled, maxDepth, maxResults, rateLimitRPM)
			}
		}
	}

	// Initialize NRC (Nostr Relay Connect) bridge if enabled
	nrcEnabled, nrcRendezvousURL, nrcAuthorizedKeys, nrcSessionTimeout := cfg.GetNRCConfigValues()
	if nrcEnabled && nrcRendezvousURL != "" {
		// Get relay identity for signing NRC responses
		relaySecretKey, err := db.GetOrCreateRelayIdentitySecret()
		if err != nil {
			log.E.F("failed to get relay identity for NRC bridge: %v", err)
		} else {
			// Create signer from secret key
			relaySigner, sigErr := p8k.New()
			if sigErr != nil {
				log.E.F("failed to create signer for NRC bridge: %v", sigErr)
			} else if sigErr = relaySigner.InitSec(relaySecretKey); sigErr != nil {
				log.E.F("failed to init signer for NRC bridge: %v", sigErr)
			} else {
				// Parse authorized secrets (format: secret:name,secret:name,...)
				authorizedSecrets := make(map[string]string)
				for _, entry := range nrcAuthorizedKeys {
					parts := strings.SplitN(entry, ":", 2)
					if len(parts) >= 1 {
						secretHex := parts[0]
						name := ""
						if len(parts) == 2 {
							name = parts[1]
						}
						// Derive pubkey from secret
						secretBytes, decErr := hex.Dec(secretHex)
						if decErr != nil || len(secretBytes) != 32 {
							log.W.F("NRC: skipping invalid secret key: %s", secretHex[:8])
							continue
						}
						derivedSigner, signerErr := p8k.New()
						if signerErr != nil {
							log.W.F("NRC: failed to create signer: %v", signerErr)
							continue
						}
						if signerErr = derivedSigner.InitSec(secretBytes); signerErr != nil {
							log.W.F("NRC: failed to init signer: %v", signerErr)
							continue
						}
						derivedPubkeyHex := string(hex.Enc(derivedSigner.Pub()))
						authorizedSecrets[derivedPubkeyHex] = name
					}
				}

				// Construct local relay URL
				localRelayURL := fmt.Sprintf("ws://localhost:%d", cfg.Port)

				// Create bridge config
				bridgeConfig := &nrc.BridgeConfig{
					RendezvousURL:     nrcRendezvousURL,
					LocalRelayURL:     localRelayURL,
					Signer:            relaySigner,
					AuthorizedSecrets: authorizedSecrets,
					SessionTimeout:    nrcSessionTimeout,
				}

				// Create event-based NRC store for dynamic connection management
				// This works with any database backend
				l.nrcEventStore = database.NewNRCEventStore(db, relaySigner)
				bridgeConfig.Authorizer = database.NewNRCEventAuthorizer(l.nrcEventStore)
				log.D.F("NRC bridge using event-based authorization")

				// Create and start the bridge
				l.nrcBridge = nrc.NewBridge(bridgeConfig)
				if err := l.nrcBridge.Start(); err != nil {
					log.E.F("failed to start NRC bridge: %v", err)
					l.nrcBridge = nil
					l.nrcEventStore = nil
				} else {
					log.I.F("NRC bridge started (rendezvous: %s, authorized: %d env, event-store: %v)",
						nrcRendezvousURL, len(authorizedSecrets), l.nrcEventStore != nil)
				}
			}
		}
	}

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
						for _, aclInstance := range acl.Registry.ACLs() {
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
						for _, aclInstance := range acl.Registry.ACLs() {
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
					for _, aclInstance := range acl.Registry.ACLs() {
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
				for _, aclInstance := range acl.Registry.ACLs() {
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

	// Initialize corpus crawler if enabled (works with any database backend)
	if cfg.CrawlerEnabled {
		crawlerCfg := crawler.DefaultConfig()
		if cfg.CrawlerDiscoveryInterval > 0 {
			crawlerCfg.DiscoveryInterval = cfg.CrawlerDiscoveryInterval
		}
		if cfg.CrawlerSyncInterval > 0 {
			crawlerCfg.SyncInterval = cfg.CrawlerSyncInterval
		}
		if cfg.CrawlerMaxHops > 0 {
			crawlerCfg.MaxHops = cfg.CrawlerMaxHops
		}
		if cfg.CrawlerConcurrency > 0 {
			crawlerCfg.Concurrency = cfg.CrawlerConcurrency
		}

		if l.corpusCrawler, err = crawler.New(ctx, db, l.publishers, crawlerCfg); err != nil {
			log.E.F("failed to create corpus crawler: %v", err)
		} else {
			l.corpusCrawler.SetSeedCallback(func() [][]byte {
				var pubkeys [][]byte
				for _, aclInstance := range acl.Registry.ACLs() {
					if aclInstance.Type() == "follows" {
						if follows, ok := aclInstance.(*acl.Follows); ok {
							pubkeys = append(pubkeys, follows.GetFollowedPubkeys()...)
						}
					}
				}
				if len(pubkeys) == 0 {
					pubkeys = adminKeys
				}
				return pubkeys
			})

			if err = l.corpusCrawler.Start(); err != nil {
				log.E.F("failed to start corpus crawler: %v", err)
			} else {
				log.I.F("corpus crawler started (discovery: %v, sync: %v, hops: %d, concurrency: %d)",
					crawlerCfg.DiscoveryInterval, crawlerCfg.SyncInterval,
					crawlerCfg.MaxHops, crawlerCfg.Concurrency)
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

	// Initialize Blossom blob storage server
	// Now works with any database backend that implements blob storage methods.
	// MUST be done before UserInterface() which registers routes.
	if cfg.BlossomEnabled {
		log.I.F("initializing Blossom server...")
		if l.blossomServer, err = initializeBlossomServer(ctx, cfg, db); err != nil {
			log.E.F("failed to initialize blossom server: %v", err)
			// Continue without blossom server
		} else if l.blossomServer != nil {
			log.I.F("blossom blob storage server initialized")
		} else {
			log.W.F("blossom server initialization returned nil without error")
		}
	} else {
		log.I.F("Blossom server disabled via ORLY_BLOSSOM_ENABLED=false")
	}

	// Initialize Nostr-Email bridge if enabled
	bridgeEnabled, bridgeDomain, bridgeNSEC, bridgeRelayURL,
		bridgeSMTPPort, bridgeSMTPHost, bridgeDataDir,
		bridgeDKIMKeyPath, bridgeDKIMSelector,
		bridgeNWCURI, bridgeMonthlyPriceSats, bridgeComposeURL,
		bridgeSMTPRelayHost, bridgeSMTPRelayPort,
		bridgeSMTPRelayUsername, bridgeSMTPRelayPassword,
		bridgeACLGRPCServer, bridgeAliasPriceSats, bridgeProfilePath := cfg.GetBridgeConfigValues()

	if bridgeEnabled {
		bridgeCfg := &emailbridge.Config{
			Domain:            bridgeDomain,
			NSEC:              bridgeNSEC,
			RelayURL:          bridgeRelayURL,
			SMTPPort:          bridgeSMTPPort,
			SMTPHost:          bridgeSMTPHost,
			DataDir:           bridgeDataDir,
			DKIMKeyPath:       bridgeDKIMKeyPath,
			DKIMSelector:      bridgeDKIMSelector,
			NWCURI:            bridgeNWCURI,
			MonthlyPriceSats:  bridgeMonthlyPriceSats,
			ComposeURL:        bridgeComposeURL,
			SMTPRelayHost:     bridgeSMTPRelayHost,
			SMTPRelayPort:     bridgeSMTPRelayPort,
			SMTPRelayUsername: bridgeSMTPRelayUsername,
			SMTPRelayPassword: bridgeSMTPRelayPassword,
			ACLGRPCServer:     bridgeACLGRPCServer,
			AliasPriceSats:    bridgeAliasPriceSats,
			ProfilePath:       bridgeProfilePath,
		}

		// In monolithic mode, provide a database getter for identity resolution
		dbGetter := func() ([]byte, error) {
			return db.GetOrCreateRelayIdentitySecret()
		}

		l.emailBridge = emailbridge.New(bridgeCfg, dbGetter)
		if err := l.emailBridge.Start(ctx); err != nil {
			log.E.F("failed to start email bridge: %v", err)
			l.emailBridge = nil
		} else {
			log.I.F("email bridge started (domain: %s, SMTP: %s:%d)",
				bridgeDomain, bridgeSMTPHost, bridgeSMTPPort)
		}
	}

	// Initialize WireGuard VPN and NIP-46 Bunker (only for Badger backend)
	// Requires ACL mode 'follows' or 'managed' - no point for open relays
	if badgerDB, ok := db.(*database.D); ok && cfg.WGEnabled && cfg.ACLMode != "none" {
		if cfg.WGEndpoint == "" {
			log.E.F("WireGuard enabled but ORLY_WG_ENDPOINT not set - skipping")
		} else {
			// Get or create the subnet pool (restores seed and allocations from DB)
			subnetPool, err := badgerDB.GetOrCreateSubnetPool(cfg.WGNetwork)
			if err != nil {
				log.E.F("failed to create subnet pool: %v", err)
			} else {
				l.subnetPool = subnetPool

				// Get or create WireGuard server key
				wgServerKey, err := badgerDB.GetOrCreateWireGuardServerKey()
				if err != nil {
					log.E.F("failed to get WireGuard server key: %v", err)
				} else {
					// Create WireGuard server
					wgConfig := &wireguard.Config{
						Port:       cfg.WGPort,
						Endpoint:   cfg.WGEndpoint,
						PrivateKey: wgServerKey,
						Network:    cfg.WGNetwork,
						ServerIP:   "10.73.0.1",
					}

					l.wireguardServer, err = wireguard.New(wgConfig)
					if err != nil {
						log.E.F("failed to create WireGuard server: %v", err)
					} else {
						if err = l.wireguardServer.Start(); err != nil {
							log.E.F("failed to start WireGuard server: %v", err)
						} else {
							log.I.F("WireGuard VPN server started on UDP port %d", cfg.WGPort)

							// Load existing peers from database and add to server
							peers, err := badgerDB.GetAllWireGuardPeers()
							if err != nil {
								log.W.F("failed to load existing WireGuard peers: %v", err)
							} else {
								for _, peer := range peers {
									// Derive client IP from sequence
									subnet := subnetPool.SubnetForSequence(peer.Sequence)
									clientIP := subnet.ClientIP.String()
									if err := l.wireguardServer.AddPeer(peer.NostrPubkey, peer.WGPublicKey, clientIP); err != nil {
										log.W.F("failed to add existing peer: %v", err)
									}
								}
								if len(peers) > 0 {
									log.I.F("loaded %d existing WireGuard peers", len(peers))
								}
							}

						// Initialize bunker if enabled
						if cfg.BunkerEnabled {
							// Get relay identity for signing
							relaySecretKey, err := badgerDB.GetOrCreateRelayIdentitySecret()
							if err != nil {
								log.E.F("failed to get relay identity for bunker: %v", err)
							} else {
								// Create signer from secret key
								relaySigner, sigErr := p8k.New()
								if sigErr != nil {
									log.E.F("failed to create signer for bunker: %v", sigErr)
								} else if sigErr = relaySigner.InitSec(relaySecretKey); sigErr != nil {
									log.E.F("failed to init signer for bunker: %v", sigErr)
								} else {
								relayPubkey := relaySigner.Pub()

								bunkerConfig := &bunker.Config{
									RelaySigner: relaySigner,
									RelayPubkey: relayPubkey[:],
									Netstack:    l.wireguardServer.GetNetstack(),
									ListenAddr:  fmt.Sprintf("10.73.0.1:%d", cfg.BunkerPort),
								}

								l.bunkerServer = bunker.New(bunkerConfig)
								if err = l.bunkerServer.Start(); err != nil {
									log.E.F("failed to start bunker server: %v", err)
								} else {
									log.I.F("NIP-46 bunker server started on 10.73.0.1:%d (WireGuard only)", cfg.BunkerPort)
								}
								}
							}
						}
						}
					}
				}
			}
		}
	} else if cfg.WGEnabled && cfg.ACLMode == "none" {
		log.I.F("WireGuard disabled: requires ACL mode 'follows' or 'managed' (currently: 'none')")
	}

	// Initialize event domain services (validation, routing, processing)
	l.InitEventServices()

	// Initialize the user interface (registers routes)
	l.UserInterface()

	// Start embedded Smesh web client if enabled
	if cfg.SmeshEnabled && cfg.SmeshPort > 0 {
		l.smeshServer = NewSmeshServer(cfg.SmeshPort)
		if err := l.smeshServer.Start(ctx); err != nil {
			log.E.F("failed to start smesh server: %v", err)
			l.smeshServer = nil
		}
	}

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

	// Initialize access tracker for storage management (only for Badger backend)
	if badgerDB, ok := db.(*database.D); ok {
		l.accessTracker = storage.NewAccessTracker(badgerDB, 100000) // 100k dedup cache
		l.accessTracker.Start()
		log.I.F("access tracker initialized")

		// Initialize garbage collector if enabled
		maxBytes, gcEnabled, gcIntervalSec, gcBatchSize := cfg.GetStorageConfigValues()
		if gcEnabled {
			gcCfg := storage.GCConfig{
				MaxStorageBytes: maxBytes,
				Interval:        time.Duration(gcIntervalSec) * time.Second,
				BatchSize:       gcBatchSize,
				MinAgeSec:       3600, // Minimum 1 hour before eviction
			}
			l.garbageCollector = storage.NewGarbageCollector(ctx, badgerDB, l.accessTracker, gcCfg)
			l.garbageCollector.Start()
			log.I.F("garbage collector started (interval: %ds, batch: %d)", gcIntervalSec, gcBatchSize)
		}
	}

	// Initialize archive relay manager if enabled
	archiveEnabled, archiveRelays, archiveTimeoutSec, archiveCacheTTLHrs := cfg.GetArchiveConfigValues()
	if archiveEnabled && len(archiveRelays) > 0 {
		archiveCfg := archive.Config{
			Enabled:     true,
			Relays:      archiveRelays,
			TimeoutSec:  archiveTimeoutSec,
			CacheTTLHrs: archiveCacheTTLHrs,
		}
		l.archiveManager = archive.New(ctx, db, archiveCfg)
		log.I.F("archive relay manager initialized with %d relays", len(archiveRelays))
	}

	// Build transport manager
	l.transportMgr = transport.NewManager()

	// Add Tor transport if enabled (can start before db is ready)
	torEnabled, torPort, torDataDir, torBinary, torSOCKSPort := cfg.GetTorConfigValues()
	if torEnabled {
		tt := tortransport.New(&tortransport.Config{
			Port:      torPort,
			DataDir:   torDataDir,
			Binary:    torBinary,
			SOCKSPort: torSOCKSPort,
			Handler:   l,
		})
		l.transportMgr.Add(tt)
	}

	// Start rate limiter if enabled
	if limiter != nil && limiter.IsEnabled() {
		limiter.Start()
		log.I.F("adaptive rate limiter started")
	}

	// Wait for database to be ready before accepting requests
	log.I.F("waiting for database warmup to complete...")
	<-db.Ready()
	log.I.F("database ready, starting transports")

	// Add TLS or plain TCP transport (mutually exclusive)
	if len(cfg.TLSDomains) > 0 {
		l.transportMgr.Add(tlstransport.New(&tlstransport.Config{
			Domains: cfg.TLSDomains,
			Certs:   cfg.Certs,
			DataDir: cfg.DataDir,
			Handler: l,
		}))
	} else {
		l.transportMgr.Add(tcp.New(&tcp.Config{
			Addr:    fmt.Sprintf("%s:%d", cfg.Listen, cfg.Port),
			Handler: l,
		}))
	}

	// Start all transports
	if err := l.transportMgr.StartAll(ctx); err != nil {
		log.E.F("transport startup failed: %v", err)
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

		// Stop corpus crawler if running
		if l.corpusCrawler != nil {
			l.corpusCrawler.Stop()
			log.I.F("corpus crawler stopped")
		}

		// Stop rate limiter if running
		if l.rateLimiter != nil && l.rateLimiter.IsEnabled() {
			l.rateLimiter.Stop()
			log.I.F("rate limiter stopped")
		}

		// Stop HTTP guard
		if l.httpGuard != nil {
			l.httpGuard.Stop()
			log.I.F("HTTP guard stopped")
		}

		// Stop archive manager if running
		if l.archiveManager != nil {
			l.archiveManager.Stop()
			log.I.F("archive manager stopped")
		}

		// Stop garbage collector if running
		if l.garbageCollector != nil {
			l.garbageCollector.Stop()
			log.I.F("garbage collector stopped")
		}

		// Stop access tracker if running
		if l.accessTracker != nil {
			l.accessTracker.Stop()
			log.I.F("access tracker stopped")
		}

		// Stop bunker server if running
		if l.bunkerServer != nil {
			l.bunkerServer.Stop()
			log.I.F("bunker server stopped")
		}

		// Stop email bridge if running
		if l.emailBridge != nil {
			l.emailBridge.Stop()
			log.I.F("email bridge stopped")
		}

		// Stop smesh server if running
		if l.smeshServer != nil {
			l.smeshServer.Stop()
			log.I.F("smesh server stopped")
		}

		// Stop NRC bridge if running
		if l.nrcBridge != nil {
			l.nrcBridge.Stop()
			log.I.F("NRC bridge stopped")
		}

		// Stop WireGuard server if running
		if l.wireguardServer != nil {
			l.wireguardServer.Stop()
			log.I.F("WireGuard server stopped")
		}

		// Stop all transports (TCP/TLS/Tor)
		if l.transportMgr != nil {
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancelShutdown()
			if err := l.transportMgr.StopAll(shutdownCtx); err != nil {
				log.E.F("transport shutdown error: %v", err)
			}
		}

		once.Do(func() { close(quit) })
	}()

	return
}
