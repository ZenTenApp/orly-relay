//go:build !(js && wasm)

package launcher

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
	"next.orly.dev/pkg/nostr/encoders/hex"
	orlyaclv1 "next.orly.dev/pkg/proto/orlyacl/v1"
	orlydbv1 "next.orly.dev/pkg/proto/orlydb/v1"
)

// Supervisor manages the database, ACL, sync, and relay processes.
// Uses self-exec pattern: spawns the same binary with different subcommands.
type Supervisor struct {
	cfg      *Config
	selfPath string // Path to current executable
	ctx      context.Context
	cancel   context.CancelFunc

	dbProc    *Process
	aclProc   *Process
	relayProc *Process

	// Sync service processes
	distributedSyncProc *Process
	clusterSyncProc     *Process
	relayGroupProc      *Process
	negentropyProc      *Process

	// Certificate service process
	certsProc *Process

	// Email bridge process
	bridgeProc *Process

	// Bridge bot process
	bridgeBotProc *Process

	wg     sync.WaitGroup
	mu     sync.Mutex
	closed bool
}

// Process represents a managed subprocess.
type Process struct {
	name     string
	cmd      *exec.Cmd
	restarts int
	exited   chan struct{} // closed when process exits
	mu       sync.Mutex
}

// ProcessStatus represents the status of a managed process.
type ProcessStatus struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
	PID         int    `json:"pid,omitempty"`
	Restarts    int    `json:"restarts,omitempty"`
}

// NewSupervisor creates a new process supervisor.
func NewSupervisor(ctx context.Context, cancel context.CancelFunc, cfg *Config) (*Supervisor, error) {
	// Get path to current executable for self-exec
	selfPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	return &Supervisor{
		cfg:      cfg,
		selfPath: selfPath,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// IsRunning returns true if any managed processes are running.
func (s *Supervisor) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if any process is running
	if s.dbProc != nil {
		select {
		case <-s.dbProc.exited:
			// Process has exited
		default:
			return true
		}
	}
	if s.relayProc != nil {
		select {
		case <-s.relayProc.exited:
			// Process has exited
		default:
			return true
		}
	}
	return false
}

// Start starts the database, optional ACL server, sync services, and relay processes.
func (s *Supervisor) Start() error {
	s.mu.Lock()
	s.closed = false
	s.mu.Unlock()

	// 1. Start database server (self-exec: orly db --driver=X)
	if err := s.startDB(); err != nil {
		return fmt.Errorf("failed to start database: %w", err)
	}

	// 2. Wait for DB to be ready (health check on gRPC port)
	if err := s.waitForDBReady(s.cfg.DBReadyTimeout); err != nil {
		s.stopDB()
		return fmt.Errorf("database not ready: %w", err)
	}

	log.I.F("database is ready")

	// 3. Start ACL server if enabled (self-exec: orly acl --driver=X)
	if s.cfg.ACLEnabled {
		if err := s.startACL(); err != nil {
			s.stopDB()
			return fmt.Errorf("failed to start ACL server: %w", err)
		}

		// Wait for ACL to be ready
		if err := s.waitForACLReady(s.cfg.ACLReadyTimeout); err != nil {
			s.stopACL()
			s.stopDB()
			return fmt.Errorf("ACL server not ready: %w", err)
		}

		log.I.F("ACL server is ready")
	}

	// 4. Start sync services in parallel (they all depend on DB)
	if err := s.startSyncServices(); err != nil {
		s.stopSyncServices()
		if s.cfg.ACLEnabled {
			s.stopACL()
		}
		s.stopDB()
		return fmt.Errorf("failed to start sync services: %w", err)
	}

	// 5. Start relay with gRPC backend(s) (self-exec: orly with gRPC env vars)
	if err := s.startRelay(); err != nil {
		s.stopSyncServices()
		if s.cfg.ACLEnabled {
			s.stopACL()
		}
		s.stopDB()
		return fmt.Errorf("failed to start relay: %w", err)
	}

	// 6. Start bridge if enabled (connects to relay via WebSocket)
	if s.cfg.BridgeEnabled {
		// Give the relay a moment to start accepting connections
		time.Sleep(2 * time.Second)
		if err := s.startBridge(); err != nil {
			log.W.F("failed to start bridge: %v", err)
			// Don't fail startup - bridge is independent
		} else {
			log.I.F("bridge started")
		}
	}

	// 6b. Start bridge bot if enabled
	if s.cfg.BridgeBotEnabled && s.cfg.BridgeBotRelay != "" {
		time.Sleep(2 * time.Second)
		if err := s.startBridgeBot(); err != nil {
			log.W.F("failed to start bridge bot: %v", err)
		} else {
			log.I.F("bridge bot started")
		}
	}

	// 7. Start certificate service if enabled
	if s.cfg.CertsEnabled {
		if err := s.startCerts(); err != nil {
			log.W.F("failed to start certificate service: %v", err)
			// Don't fail startup - certs are independent
		} else {
			log.I.F("certificate service started")
		}
	}

	// 7. Start monitoring goroutines
	monitorCount := 2 // db + relay
	if s.cfg.ACLEnabled {
		monitorCount++
	}
	if s.cfg.DistributedSyncEnabled {
		monitorCount++
	}
	if s.cfg.ClusterSyncEnabled {
		monitorCount++
	}
	if s.cfg.RelayGroupEnabled {
		monitorCount++
	}
	if s.cfg.NegentropyEnabled {
		monitorCount++
	}
	if s.cfg.CertsEnabled {
		monitorCount++
	}
	if s.cfg.BridgeEnabled {
		monitorCount++
	}
	if s.cfg.BridgeBotEnabled && s.bridgeBotProc != nil {
		monitorCount++
	}

	s.wg.Add(monitorCount)
	go s.monitorProcess(s.dbProc, "db", s.startDB)
	if s.cfg.ACLEnabled {
		go s.monitorProcess(s.aclProc, "acl", s.startACL)
	}
	if s.cfg.DistributedSyncEnabled {
		go s.monitorProcess(s.distributedSyncProc, "distributed-sync", s.startDistributedSync)
	}
	if s.cfg.ClusterSyncEnabled {
		go s.monitorProcess(s.clusterSyncProc, "cluster-sync", s.startClusterSync)
	}
	if s.cfg.RelayGroupEnabled {
		go s.monitorProcess(s.relayGroupProc, "relaygroup", s.startRelayGroup)
	}
	if s.cfg.NegentropyEnabled {
		go s.monitorProcess(s.negentropyProc, "negentropy", s.startNegentropy)
	}
	if s.cfg.CertsEnabled {
		go s.monitorProcess(s.certsProc, "certs", s.startCerts)
	}
	if s.cfg.BridgeEnabled {
		go s.monitorProcess(s.bridgeProc, "bridge", s.startBridge)
	}
	if s.cfg.BridgeBotEnabled && s.bridgeBotProc != nil {
		go s.monitorProcess(s.bridgeBotProc, "bridgebot", s.startBridgeBot)
	}
	go s.monitorProcess(s.relayProc, "relay", s.startRelay)

	return nil
}

// Stop stops all managed processes gracefully.
func (s *Supervisor) Stop() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	// Stop bridge bot first (it connects to relay, must stop before relay)
	if s.bridgeBotProc != nil {
		log.I.F("stopping bridge bot...")
		s.stopProcess(s.bridgeBotProc, 5*time.Second)
	}

	// Stop bridge (it connects to relay, must stop before relay)
	if s.cfg.BridgeEnabled && s.bridgeProc != nil {
		log.I.F("stopping bridge...")
		s.stopProcess(s.bridgeProc, 5*time.Second)
	}

	// Stop certificate service (independent, nothing depends on it)
	if s.cfg.CertsEnabled && s.certsProc != nil {
		log.I.F("stopping certificate service...")
		s.stopProcess(s.certsProc, 5*time.Second)
	}

	// Stop relay (it depends on sync services, ACL, and DB)
	log.I.F("stopping relay...")
	s.stopProcess(s.relayProc, 5*time.Second)

	// Stop sync services in parallel (they depend on DB)
	log.I.F("stopping sync services...")
	s.stopSyncServices()

	// Stop ACL if enabled (it depends on DB)
	if s.cfg.ACLEnabled && s.aclProc != nil {
		log.I.F("stopping ACL server...")
		s.stopProcess(s.aclProc, 5*time.Second)
	}

	// Stop DB with longer timeout for flush
	log.I.F("stopping database...")
	s.stopProcess(s.dbProc, s.cfg.StopTimeout)

	// Wait for monitor goroutines to exit
	s.wg.Wait()

	return nil
}

func (s *Supervisor) startDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build environment for database process
	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_DB_LISTEN=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_DATA_DIR=%s", s.cfg.DataDir))
	env = append(env, fmt.Sprintf("ORLY_DB_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly db --driver=badger (or whatever driver is configured)
	cmd := exec.CommandContext(s.ctx, s.selfPath, "db", "--driver="+s.cfg.DBDriver)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.dbProc = &Process{
		name:   "orly-db",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started database server (pid %d) via self-exec: %s db --driver=%s",
		cmd.Process.Pid, s.selfPath, s.cfg.DBDriver)
	return nil
}

func (s *Supervisor) startACL() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build environment for ACL process
	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_ACL_LISTEN=%s", s.cfg.ACLListen))
	env = append(env, "ORLY_ACL_DB_TYPE=grpc")
	env = append(env, fmt.Sprintf("ORLY_ACL_GRPC_DB_SERVER=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_ACL_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly acl --driver=follows (or whatever ACL driver is configured)
	cmd := exec.CommandContext(s.ctx, s.selfPath, "acl", "--driver="+s.cfg.ACLDriver)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.aclProc = &Process{
		name:   "orly-acl",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started ACL server (pid %d) via self-exec: %s acl --driver=%s",
		cmd.Process.Pid, s.selfPath, s.cfg.ACLDriver)
	return nil
}

func (s *Supervisor) waitForACLReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var grpcConn *grpc.ClientConn
	var aclClient orlyaclv1.ACLServiceClient

	for {
		select {
		case <-s.ctx.Done():
			if grpcConn != nil {
				grpcConn.Close()
			}
			return s.ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				if grpcConn != nil {
					grpcConn.Close()
				}
				return fmt.Errorf("timeout waiting for ACL server")
			}

			// First, check if TCP port is open
			conn, err := net.DialTimeout("tcp", s.cfg.ACLListen, time.Second)
			if err != nil {
				continue // Port not open yet
			}
			conn.Close()

			// Port is open, now check gRPC Ready() endpoint
			if grpcConn == nil {
				grpcConn, err = grpc.DialContext(s.ctx, s.cfg.ACLListen,
					grpc.WithTransportCredentials(insecure.NewCredentials()),
				)
				if err != nil {
					continue // Failed to connect
				}
				aclClient = orlyaclv1.NewACLServiceClient(grpcConn)
			}

			// Call Ready() to check if service is fully configured
			ctx, cancel := context.WithTimeout(s.ctx, time.Second)
			resp, err := aclClient.Ready(ctx, &orlyaclv1.Empty{})
			cancel()
			if err == nil && resp.Ready {
				grpcConn.Close()
				return nil // ACL server is fully ready
			}
			// Not ready yet, keep polling
		}
	}
}

func (s *Supervisor) stopACL() {
	s.stopProcess(s.aclProc, 5*time.Second)
}

func (s *Supervisor) startRelay() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build environment for relay process
	env := os.Environ()
	env = append(env, "ORLY_DB_TYPE=grpc")
	env = append(env, fmt.Sprintf("ORLY_GRPC_SERVER=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_LOG_LEVEL=%s", s.cfg.LogLevel))

	// If ACL is enabled, configure relay to use gRPC ACL
	// Otherwise, run in open mode (no ACL restrictions)
	if s.cfg.ACLEnabled {
		env = append(env, "ORLY_ACL_TYPE=grpc")
		env = append(env, fmt.Sprintf("ORLY_GRPC_ACL_SERVER=%s", s.cfg.ACLListen))
		env = append(env, fmt.Sprintf("ORLY_ACL_MODE=%s", s.cfg.ACLDriver))
	} else {
		// Open relay - no ACL restrictions
		env = append(env, "ORLY_ACL_TYPE=local")
		env = append(env, "ORLY_ACL_MODE=none")
	}

	// Configure sync service connections
	// Set ORLY_SYNC_TYPE=grpc when any sync service is enabled in launcher mode,
	// so the relay uses gRPC clients instead of embedded handlers.
	if s.cfg.DistributedSyncEnabled || s.cfg.ClusterSyncEnabled || s.cfg.RelayGroupEnabled || s.cfg.NegentropyEnabled {
		env = append(env, "ORLY_SYNC_TYPE=grpc")
	}
	if s.cfg.DistributedSyncEnabled {
		env = append(env, fmt.Sprintf("ORLY_GRPC_SYNC_DISTRIBUTED=%s", s.cfg.DistributedSyncListen))
	}
	if s.cfg.ClusterSyncEnabled {
		env = append(env, fmt.Sprintf("ORLY_GRPC_SYNC_CLUSTER=%s", s.cfg.ClusterSyncListen))
	}
	if s.cfg.RelayGroupEnabled {
		env = append(env, fmt.Sprintf("ORLY_GRPC_SYNC_RELAYGROUP=%s", s.cfg.RelayGroupListen))
	}
	if s.cfg.NegentropyEnabled {
		env = append(env, "ORLY_NEGENTROPY_ENABLED=true")
		env = append(env, fmt.Sprintf("ORLY_GRPC_SYNC_NEGENTROPY=%s", s.cfg.NegentropyListen))
	}

	// When the launcher manages the bridge as a separate subprocess, disable
	// the in-process bridge in the relay to avoid double-start and port conflicts.
	if s.cfg.BridgeEnabled {
		env = append(env, "ORLY_BRIDGE_ENABLED=false")
	}

	// Self-exec: orly (without subcommand runs as relay)
	cmd := exec.CommandContext(s.ctx, s.selfPath)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.relayProc = &Process{
		name:   "orly",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started relay server (pid %d) via self-exec: %s", cmd.Process.Pid, s.selfPath)
	return nil
}

func (s *Supervisor) waitForDBReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for database")
			}

			// Try to connect to the gRPC port
			conn, err := net.DialTimeout("tcp", s.cfg.DBListen, time.Second)
			if err == nil {
				conn.Close()
				return nil // Database is accepting connections
			}
		}
	}
}

func (s *Supervisor) stopDB() {
	s.stopProcess(s.dbProc, s.cfg.StopTimeout)
}

func (s *Supervisor) stopProcess(p *Process, timeout time.Duration) {
	if p == nil {
		return
	}

	p.mu.Lock()
	if p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	// Send SIGTERM for graceful shutdown
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process may have already exited
		log.D.F("%s already exited: %v", p.name, err)
		return
	}

	// Wait for process to exit using the exited channel
	select {
	case <-p.exited:
		log.I.F("%s stopped gracefully", p.name)
	case <-time.After(timeout):
		log.W.F("%s did not stop in time, killing", p.name)
		p.cmd.Process.Kill()
		<-p.exited // Wait for the kill to complete
	}
}

func (s *Supervisor) monitorProcess(p *Process, procType string, restart func() error) {
	defer s.wg.Done()

	for {
		// Check if we're shutting down
		s.mu.Lock()
		closed := s.closed
		s.mu.Unlock()

		if closed {
			return
		}

		select {
		case <-s.ctx.Done():
			return
		default:
		}

		if p == nil || p.exited == nil {
			return
		}

		// Wait for process to exit
		select {
		case <-p.exited:
			// Process exited
		case <-s.ctx.Done():
			return
		}

		// Check again if we're shutting down
		s.mu.Lock()
		closed = s.closed
		s.mu.Unlock()

		if closed {
			return
		}

		// Process exited unexpectedly
		p.restarts++
		log.W.F("%s exited unexpectedly, restart count: %d", p.name, p.restarts)

		// Backoff before restart
		backoff := time.Duration(p.restarts) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Check one more time before restarting
		s.mu.Lock()
		closed = s.closed
		s.mu.Unlock()

		if closed {
			return
		}

		if err := restart(); err != nil {
			log.E.F("failed to restart %s: %v", p.name, err)
		} else {
			// Update p to point to the new process
			s.mu.Lock()
			switch procType {
			case "db":
				p = s.dbProc
			case "acl":
				p = s.aclProc
			case "distributed-sync":
				p = s.distributedSyncProc
			case "cluster-sync":
				p = s.clusterSyncProc
			case "relaygroup":
				p = s.relayGroupProc
			case "negentropy":
				p = s.negentropyProc
			case "certs":
				p = s.certsProc
			default:
				p = s.relayProc
			}
			s.mu.Unlock()
		}
	}
}

// startSyncServices starts all enabled sync services in parallel.
func (s *Supervisor) startSyncServices() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	// Start each enabled service in a goroutine
	if s.cfg.DistributedSyncEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.startDistributedSync(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("distributed sync: %w", err))
				mu.Unlock()
				return
			}
			if err := s.waitForServiceReady(s.cfg.DistributedSyncListen, s.cfg.SyncReadyTimeout); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("distributed sync not ready: %w", err))
				mu.Unlock()
				return
			}
			log.I.F("distributed sync service is ready")
		}()
	}

	if s.cfg.ClusterSyncEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.startClusterSync(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("cluster sync: %w", err))
				mu.Unlock()
				return
			}
			if err := s.waitForServiceReady(s.cfg.ClusterSyncListen, s.cfg.SyncReadyTimeout); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("cluster sync not ready: %w", err))
				mu.Unlock()
				return
			}
			log.I.F("cluster sync service is ready")
		}()
	}

	if s.cfg.RelayGroupEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.startRelayGroup(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("relaygroup: %w", err))
				mu.Unlock()
				return
			}
			if err := s.waitForServiceReady(s.cfg.RelayGroupListen, s.cfg.SyncReadyTimeout); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("relaygroup not ready: %w", err))
				mu.Unlock()
				return
			}
			log.I.F("relaygroup service is ready")
		}()
	}

	if s.cfg.NegentropyEnabled {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.startNegentropy(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("negentropy: %w", err))
				mu.Unlock()
				return
			}
			if err := s.waitForServiceReady(s.cfg.NegentropyListen, s.cfg.SyncReadyTimeout); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("negentropy not ready: %w", err))
				mu.Unlock()
				return
			}
			log.I.F("negentropy service is ready")
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// stopSyncServices stops all sync services.
func (s *Supervisor) stopSyncServices() {
	var wg sync.WaitGroup

	if s.distributedSyncProc != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.stopProcess(s.distributedSyncProc, 5*time.Second)
		}()
	}

	if s.clusterSyncProc != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.stopProcess(s.clusterSyncProc, 5*time.Second)
		}()
	}

	if s.relayGroupProc != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.stopProcess(s.relayGroupProc, 5*time.Second)
		}()
	}

	if s.negentropyProc != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.stopProcess(s.negentropyProc, 5*time.Second)
		}()
	}

	wg.Wait()
}

// waitForServiceReady waits for a gRPC service to be accepting connections.
func (s *Supervisor) waitForServiceReady(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for service at %s", address)
			}

			conn, err := net.DialTimeout("tcp", address, time.Second)
			if err == nil {
				conn.Close()
				return nil
			}
		}
	}
}

func (s *Supervisor) startDistributedSync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_SYNC_DISTRIBUTED_LISTEN=%s", s.cfg.DistributedSyncListen))
	env = append(env, "ORLY_SYNC_DISTRIBUTED_DB_TYPE=grpc")
	env = append(env, fmt.Sprintf("ORLY_SYNC_DISTRIBUTED_DB_SERVER=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_SYNC_DISTRIBUTED_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly sync --driver=distributed
	cmd := exec.CommandContext(s.ctx, s.selfPath, "sync", "--driver=distributed")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.distributedSyncProc = &Process{
		name:   "orly-sync-distributed",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started distributed sync service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startClusterSync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_SYNC_CLUSTER_LISTEN=%s", s.cfg.ClusterSyncListen))
	env = append(env, "ORLY_SYNC_CLUSTER_DB_TYPE=grpc")
	env = append(env, fmt.Sprintf("ORLY_SYNC_CLUSTER_DB_SERVER=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_SYNC_CLUSTER_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly sync --driver=cluster
	cmd := exec.CommandContext(s.ctx, s.selfPath, "sync", "--driver=cluster")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.clusterSyncProc = &Process{
		name:   "orly-sync-cluster",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started cluster sync service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startRelayGroup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_SYNC_RELAYGROUP_LISTEN=%s", s.cfg.RelayGroupListen))
	env = append(env, "ORLY_SYNC_RELAYGROUP_DB_TYPE=grpc")
	env = append(env, fmt.Sprintf("ORLY_SYNC_RELAYGROUP_DB_SERVER=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_SYNC_RELAYGROUP_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly sync --driver=relaygroup
	cmd := exec.CommandContext(s.ctx, s.selfPath, "sync", "--driver=relaygroup")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.relayGroupProc = &Process{
		name:   "orly-sync-relaygroup",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started relaygroup service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startNegentropy() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_SYNC_NEGENTROPY_LISTEN=%s", s.cfg.NegentropyListen))
	env = append(env, "ORLY_SYNC_NEGENTROPY_DB_TYPE=grpc")
	env = append(env, fmt.Sprintf("ORLY_SYNC_NEGENTROPY_DB_SERVER=%s", s.cfg.DBListen))
	env = append(env, fmt.Sprintf("ORLY_SYNC_NEGENTROPY_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly sync --driver=negentropy
	cmd := exec.CommandContext(s.ctx, s.selfPath, "sync", "--driver=negentropy")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.negentropyProc = &Process{
		name:   "orly-sync-negentropy",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started negentropy service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startCerts() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Certificate service uses its own environment variables
	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_CERTS_LOG_LEVEL=%s", s.cfg.LogLevel))

	// Self-exec: orly certs
	cmd := exec.CommandContext(s.ctx, s.selfPath, "certs")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.certsProc = &Process{
		name:   "orly-certs",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started certificate service (pid %d)", cmd.Process.Pid)
	return nil
}

// GetProcessStatuses returns the status of all available modules with categories.
func (s *Supervisor) GetProcessStatuses() []ProcessStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var statuses []ProcessStatus

	// Database (using unified binary self-exec)
	statuses = append(statuses, s.getProcessStatusFull(
		s.dbProc, "orly-db", true, "database",
		fmt.Sprintf("Database server (%s driver)", s.cfg.DBDriver),
	))

	// ACL (using unified binary self-exec)
	statuses = append(statuses, s.getProcessStatusFull(
		s.aclProc, "orly-acl", s.cfg.ACLEnabled, "acl",
		fmt.Sprintf("ACL server (%s driver)", s.cfg.ACLDriver),
	))

	// Sync services
	statuses = append(statuses, s.getProcessStatusFull(
		s.distributedSyncProc, "orly-sync-distributed", s.cfg.DistributedSyncEnabled, "sync",
		"Distributed event synchronization",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		s.clusterSyncProc, "orly-sync-cluster", s.cfg.ClusterSyncEnabled, "sync",
		"Cluster synchronization for HA",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		s.relayGroupProc, "orly-sync-relaygroup", s.cfg.RelayGroupEnabled, "sync",
		"NIP-29 relay group synchronization",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		s.negentropyProc, "orly-sync-negentropy", s.cfg.NegentropyEnabled, "sync",
		"NIP-77 negentropy reconciliation",
	))

	// Certificate service
	statuses = append(statuses, s.getProcessStatusFull(
		s.certsProc, "orly-certs", s.cfg.CertsEnabled, "certs",
		"Let's Encrypt certificate management",
	))

	// Bridge
	statuses = append(statuses, s.getProcessStatusFull(
		s.bridgeProc, "orly-bridge", s.cfg.BridgeEnabled, "bridge",
		"Nostr-Email bridge (Marmot)",
	))

	// Relay process - always enabled
	statuses = append(statuses, s.getProcessStatusFull(
		s.relayProc, "orly", true, "relay",
		"Main Nostr relay server",
	))

	return statuses
}

// getProcessStatusFull returns the status of a process with category and description.
func (s *Supervisor) getProcessStatusFull(p *Process, name string, enabled bool, category, description string) ProcessStatus {
	status := "disabled"
	pid := 0
	restarts := 0

	if enabled {
		status = "stopped"
	}

	if p != nil {
		p.mu.Lock()
		defer p.mu.Unlock()

		if p.cmd != nil && p.cmd.Process != nil {
			select {
			case <-p.exited:
				status = "stopped"
			default:
				status = "running"
				pid = p.cmd.Process.Pid
			}
		}
		restarts = p.restarts
	}

	return ProcessStatus{
		Name:        name,
		Status:      status,
		Enabled:     enabled,
		Category:    category,
		Description: description,
		PID:         pid,
		Restarts:    restarts,
	}
}

func (s *Supervisor) startBridge() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build environment for bridge process
	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_LOG_LEVEL=%s", s.cfg.LogLevel))
	env = append(env, fmt.Sprintf("ORLY_DATA_DIR=%s", s.cfg.DataDir))
	if s.cfg.BridgeDomain != "" {
		env = append(env, fmt.Sprintf("ORLY_BRIDGE_DOMAIN=%s", s.cfg.BridgeDomain))
	}

	// The bridge connects to the relay via WebSocket. Construct the local URL.
	// Use ws://localhost:<port> since relay is on the same host.
	relayPort := getEnvOrDefault("ORLY_PORT", "3334")
	env = append(env, fmt.Sprintf("ORLY_BRIDGE_RELAY_URL=ws://localhost:%s", relayPort))

	// Fetch the relay identity from the database gRPC server and inject it
	// so the bridge uses the same identity as the relay (owner/admin privileges).
	if nsec, err := s.fetchRelayNSEC(); err != nil {
		log.W.F("could not fetch relay identity for bridge: %v", err)
	} else {
		env = append(env, fmt.Sprintf("ORLY_BRIDGE_NSEC=%s", nsec))
	}

	// Inject ACL gRPC server address so the bridge can manage subscriptions
	if s.cfg.ACLEnabled && s.cfg.ACLListen != "" {
		env = append(env, fmt.Sprintf("ORLY_BRIDGE_ACL_GRPC_SERVER=%s", s.cfg.ACLListen))
	}

	// Self-exec: orly bridge
	cmd := exec.CommandContext(s.ctx, s.selfPath, "bridge")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.bridgeProc = &Process{
		name:   "orly-bridge",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started bridge (pid %d) via self-exec: %s bridge",
		cmd.Process.Pid, s.selfPath)
	return nil
}

func (s *Supervisor) startBridgeBot() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_LOG_LEVEL=%s", s.cfg.LogLevel))
	env = append(env, fmt.Sprintf("ORLY_BRIDGE_BOT_RELAY=%s", s.cfg.BridgeBotRelay))
	env = append(env, fmt.Sprintf("ORLY_BRIDGE_BOT_DATA_DIR=%s", s.cfg.DataDir))
	if s.cfg.BridgeBotFree {
		env = append(env, "ORLY_BRIDGE_BOT_FREE=true")
	}

	cmd := exec.CommandContext(s.ctx, s.selfPath, "bridgebot")
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.bridgeBotProc = &Process{
		name:   "orly-bridgebot",
		cmd:    cmd,
		exited: exited,
	}

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started bridge bot (pid %d) via self-exec: %s bridgebot",
		cmd.Process.Pid, s.selfPath)
	return nil
}

// fetchRelayNSEC connects to the database gRPC server, retrieves the relay
// identity secret key, and returns it as a bech32 nsec string.
func (s *Supervisor) fetchRelayNSEC() (string, error) {
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(s.cfg.DBListen,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("connect to db: %w", err)
	}
	defer conn.Close()

	client := orlydbv1.NewDatabaseServiceClient(conn)
	resp, err := client.GetOrCreateRelayIdentitySecret(ctx, &orlydbv1.Empty{})
	if err != nil {
		return "", fmt.Errorf("get relay identity: %w", err)
	}

	sk := resp.GetSecretKey()
	if len(sk) != 32 {
		// The DB may store as hex string; try decoding
		if decoded, err := hex.Dec(string(sk)); err == nil && len(decoded) == 32 {
			sk = decoded
		} else {
			return "", fmt.Errorf("unexpected secret key length: %d", len(sk))
		}
	}

	nsec, err := bech32encoding.BinToNsec(sk)
	if err != nil {
		return "", fmt.Errorf("encode nsec: %w", err)
	}
	return string(nsec), nil
}
