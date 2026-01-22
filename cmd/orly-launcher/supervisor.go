package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Supervisor manages the database, ACL, sync, and relay processes.
type Supervisor struct {
	cfg    *Config
	ctx    context.Context
	cancel context.CancelFunc

	dbProc    *Process
	aclProc   *Process
	relayProc *Process

	// Sync service processes
	distributedSyncProc *Process
	clusterSyncProc     *Process
	relayGroupProc      *Process
	negentropyProc      *Process

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

// NewSupervisor creates a new process supervisor.
func NewSupervisor(ctx context.Context, cancel context.CancelFunc, cfg *Config) *Supervisor {
	return &Supervisor{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start starts the database, optional ACL server, sync services, and relay processes.
func (s *Supervisor) Start() error {
	// 1. Start database server
	if err := s.startDB(); err != nil {
		return fmt.Errorf("failed to start database: %w", err)
	}

	// 2. Wait for DB to be ready (health check on gRPC port)
	if err := s.waitForDBReady(s.cfg.DBReadyTimeout); err != nil {
		s.stopDB()
		return fmt.Errorf("database not ready: %w", err)
	}

	log.I.F("database is ready")

	// 3. Start ACL server if enabled
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

	// 5. Start relay with gRPC backend(s)
	if err := s.startRelay(); err != nil {
		s.stopSyncServices()
		if s.cfg.ACLEnabled {
			s.stopACL()
		}
		s.stopDB()
		return fmt.Errorf("failed to start relay: %w", err)
	}

	// 6. Start monitoring goroutines
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

	// Stop relay first (it depends on sync services, ACL, and DB)
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

	// Wait for monitor goroutines to exit (they will exit when they see closed=true)
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

	cmd := exec.CommandContext(s.ctx, s.cfg.DBBinary)
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

	// Start a goroutine to wait for the process and close the exited channel
	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started database server (pid %d)", cmd.Process.Pid)
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
	env = append(env, fmt.Sprintf("ORLY_ACL_MODE=%s", s.cfg.ACLMode))
	env = append(env, fmt.Sprintf("ORLY_ACL_LOG_LEVEL=%s", s.cfg.LogLevel))

	cmd := exec.CommandContext(s.ctx, s.cfg.ACLBinary)
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

	// Start a goroutine to wait for the process and close the exited channel
	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started ACL server (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) waitForACLReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for ACL server")
			}

			// Try to connect to the gRPC port
			conn, err := net.DialTimeout("tcp", s.cfg.ACLListen, time.Second)
			if err == nil {
				conn.Close()
				return nil // ACL server is accepting connections
			}
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
		env = append(env, fmt.Sprintf("ORLY_ACL_MODE=%s", s.cfg.ACLMode))
	} else {
		// Open relay - no ACL restrictions
		env = append(env, "ORLY_ACL_TYPE=local")
		env = append(env, "ORLY_ACL_MODE=none")
	}

	// Configure sync service connections
	if s.cfg.DistributedSyncEnabled {
		env = append(env, "ORLY_SYNC_TYPE=grpc")
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

	cmd := exec.CommandContext(s.ctx, s.cfg.RelayBinary)
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

	// Start a goroutine to wait for the process and close the exited channel
	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started relay server (pid %d)", cmd.Process.Pid)
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

		// Check again if we're shutting down (process may have been stopped intentionally)
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
	// Stop all in parallel using a WaitGroup
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

	cmd := exec.CommandContext(s.ctx, s.cfg.DistributedSyncBinary)
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

	cmd := exec.CommandContext(s.ctx, s.cfg.ClusterSyncBinary)
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

	cmd := exec.CommandContext(s.ctx, s.cfg.RelayGroupBinary)
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

	cmd := exec.CommandContext(s.ctx, s.cfg.NegentropyBinary)
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
