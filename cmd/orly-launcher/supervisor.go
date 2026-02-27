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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	orlyaclv1 "next.orly.dev/pkg/proto/orlyacl/v1"
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

	// Certificate service process
	certsProc *Process

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

	// 6. Start certificate service if enabled (independent of other services)
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

	// Stop certificate service first (independent, nothing depends on it)
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

func (s *Supervisor) startCerts() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Certificate service uses its own environment variables
	// ORLY_CERTS_DOMAIN, ORLY_CERTS_EMAIL, ORLY_CERTS_DNS_PROVIDER, etc.
	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_CERTS_LOG_LEVEL=%s", s.cfg.LogLevel))

	cmd := exec.CommandContext(s.ctx, s.cfg.CertsBinary)
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
// Modules are grouped by category, and some categories are mutually exclusive.
func (s *Supervisor) GetProcessStatuses() []ProcessStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var statuses []ProcessStatus

	// Database backends (mutually exclusive - only one can be active)
	isBadger := s.cfg.DBBackend == "badger"
	isNeo4j := s.cfg.DBBackend == "neo4j"

	statuses = append(statuses, s.getProcessStatusFull(
		s.dbProc, "orly-db-badger", "orly-db-badger",
		isBadger, "database", "Badger embedded key-value store (default)",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		nil, "orly-db-neo4j", "orly-db-neo4j",
		isNeo4j, "database", "Neo4j graph database for WoT queries",
	))

	// ACL backends (mutually exclusive - only one can be active, can be disabled)
	isFollows := s.cfg.ACLEnabled && s.cfg.ACLMode == "follows"
	isManaged := s.cfg.ACLEnabled && s.cfg.ACLMode == "managed"
	isCuration := s.cfg.ACLEnabled && (s.cfg.ACLMode == "curation" || s.cfg.ACLMode == "curating")

	statuses = append(statuses, s.getProcessStatusFull(
		s.aclProc, "orly-acl-follows", "orly-acl-follows",
		isFollows, "acl", "Whitelist based on admin's follow list",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		nil, "orly-acl-managed", "orly-acl-managed",
		isManaged, "acl", "NIP-86 fine-grained access control",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		nil, "orly-acl-curation", "orly-acl-curation",
		isCuration, "acl", "Rate-limited trust tiers for curation",
	))

	// Sync services (independent - multiple can be enabled)
	statuses = append(statuses, s.getProcessStatusFull(
		s.distributedSyncProc, "orly-sync-distributed", s.cfg.DistributedSyncBinary,
		s.cfg.DistributedSyncEnabled, "sync", "Distributed event synchronization",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		s.clusterSyncProc, "orly-sync-cluster", s.cfg.ClusterSyncBinary,
		s.cfg.ClusterSyncEnabled, "sync", "Cluster synchronization for HA",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		s.relayGroupProc, "orly-sync-relaygroup", s.cfg.RelayGroupBinary,
		s.cfg.RelayGroupEnabled, "sync", "NIP-29 relay group synchronization",
	))
	statuses = append(statuses, s.getProcessStatusFull(
		s.negentropyProc, "orly-sync-negentropy", s.cfg.NegentropyBinary,
		s.cfg.NegentropyEnabled, "sync", "NIP-77 negentropy reconciliation",
	))

	// Certificate service (standalone)
	statuses = append(statuses, s.getProcessStatusFull(
		s.certsProc, "orly-certs", s.cfg.CertsBinary,
		s.cfg.CertsEnabled, "certs", "Let's Encrypt certificate management",
	))

	// Relay process - always enabled
	statuses = append(statuses, s.getProcessStatusFull(
		s.relayProc, "orly", s.cfg.RelayBinary,
		true, "relay", "Main Nostr relay server",
	))

	return statuses
}

func (s *Supervisor) getProcessStatus(p *Process, binaryPath string, enabled bool) ProcessStatus {
	status := "stopped"
	pid := 0

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd != nil && p.cmd.Process != nil {
		// Check if process is still running
		select {
		case <-p.exited:
			status = "stopped"
		default:
			status = "running"
			pid = p.cmd.Process.Pid
		}
	}

	return ProcessStatus{
		Name:     p.name,
		Binary:   binaryPath,
		Version:  "", // Will be filled by caller if needed
		Status:   status,
		Enabled:  enabled,
		PID:      pid,
		Restarts: p.restarts,
	}
}

// getProcessStatusOrDisabled returns the status of a process, or a disabled status if the process is nil.
func (s *Supervisor) getProcessStatusOrDisabled(p *Process, name, binaryPath string, enabled bool) ProcessStatus {
	if p != nil {
		return s.getProcessStatus(p, binaryPath, enabled)
	}

	// Process doesn't exist - show as disabled or stopped
	status := "stopped"
	if !enabled {
		status = "disabled"
	}

	return ProcessStatus{
		Name:    name,
		Binary:  binaryPath,
		Status:  status,
		Enabled: enabled,
	}
}

// getProcessStatusFull returns the status of a process with category and description.
func (s *Supervisor) getProcessStatusFull(p *Process, name, binaryPath string, enabled bool, category, description string) ProcessStatus {
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
		Binary:      binaryPath,
		Status:      status,
		Enabled:     enabled,
		Category:    category,
		Description: description,
		PID:         pid,
		Restarts:    restarts,
	}
}

// RestartService restarts a specific service with dependency handling.
// If a service's dependencies need to restart, they are handled appropriately.
// Returns the list of services that were restarted.
func (s *Supervisor) RestartService(serviceName string) ([]string, error) {
	log.I.F("restarting service: %s", serviceName)

	var restarted []string

	// Determine which services need to restart based on dependencies
	// db → acl, sync services, relay all depend on db
	// acl → relay depends on acl
	// sync services → relay may depend on sync services
	// relay → nothing depends on relay

	switch serviceName {
	case "orly-db", "db":
		// Restart db and all dependent services
		// First stop in reverse order: relay, sync, acl, db
		s.stopProcess(s.relayProc, 5*time.Second)
		s.stopSyncServices()
		if s.cfg.ACLEnabled && s.aclProc != nil {
			s.stopProcess(s.aclProc, 5*time.Second)
		}
		s.stopProcess(s.dbProc, s.cfg.StopTimeout)

		time.Sleep(500 * time.Millisecond)

		// Start in dependency order
		if err := s.startDB(); err != nil {
			return restarted, fmt.Errorf("failed to restart db: %w", err)
		}
		restarted = append(restarted, "orly-db")

		if err := s.waitForDBReady(s.cfg.DBReadyTimeout); err != nil {
			return restarted, fmt.Errorf("db not ready: %w", err)
		}

		if s.cfg.ACLEnabled {
			if err := s.startACL(); err != nil {
				return restarted, fmt.Errorf("failed to restart acl: %w", err)
			}
			restarted = append(restarted, "orly-acl")
			if err := s.waitForACLReady(s.cfg.ACLReadyTimeout); err != nil {
				return restarted, fmt.Errorf("acl not ready: %w", err)
			}
		}

		if err := s.startSyncServices(); err != nil {
			return restarted, fmt.Errorf("failed to restart sync services: %w", err)
		}
		if s.cfg.DistributedSyncEnabled {
			restarted = append(restarted, "orly-sync-distributed")
		}
		if s.cfg.ClusterSyncEnabled {
			restarted = append(restarted, "orly-sync-cluster")
		}
		if s.cfg.RelayGroupEnabled {
			restarted = append(restarted, "orly-sync-relaygroup")
		}
		if s.cfg.NegentropyEnabled {
			restarted = append(restarted, "orly-sync-negentropy")
		}

		if err := s.startRelay(); err != nil {
			return restarted, fmt.Errorf("failed to restart relay: %w", err)
		}
		restarted = append(restarted, "orly")

	case "orly-acl", "acl":
		if !s.cfg.ACLEnabled {
			return restarted, fmt.Errorf("ACL is not enabled")
		}
		// Restart acl and relay (relay depends on acl)
		s.stopProcess(s.relayProc, 5*time.Second)
		s.stopProcess(s.aclProc, 5*time.Second)

		time.Sleep(500 * time.Millisecond)

		if err := s.startACL(); err != nil {
			return restarted, fmt.Errorf("failed to restart acl: %w", err)
		}
		restarted = append(restarted, "orly-acl")

		if err := s.waitForACLReady(s.cfg.ACLReadyTimeout); err != nil {
			return restarted, fmt.Errorf("acl not ready: %w", err)
		}

		if err := s.startRelay(); err != nil {
			return restarted, fmt.Errorf("failed to restart relay: %w", err)
		}
		restarted = append(restarted, "orly")

	case "orly-sync-distributed", "distributed-sync":
		if !s.cfg.DistributedSyncEnabled {
			return restarted, fmt.Errorf("distributed sync is not enabled")
		}
		s.stopProcess(s.distributedSyncProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startDistributedSync(); err != nil {
			return restarted, fmt.Errorf("failed to restart distributed sync: %w", err)
		}
		restarted = append(restarted, "orly-sync-distributed")

	case "orly-sync-cluster", "cluster-sync":
		if !s.cfg.ClusterSyncEnabled {
			return restarted, fmt.Errorf("cluster sync is not enabled")
		}
		s.stopProcess(s.clusterSyncProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startClusterSync(); err != nil {
			return restarted, fmt.Errorf("failed to restart cluster sync: %w", err)
		}
		restarted = append(restarted, "orly-sync-cluster")

	case "orly-sync-relaygroup", "relaygroup":
		if !s.cfg.RelayGroupEnabled {
			return restarted, fmt.Errorf("relaygroup is not enabled")
		}
		s.stopProcess(s.relayGroupProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startRelayGroup(); err != nil {
			return restarted, fmt.Errorf("failed to restart relaygroup: %w", err)
		}
		restarted = append(restarted, "orly-sync-relaygroup")

	case "orly-sync-negentropy", "negentropy":
		if !s.cfg.NegentropyEnabled {
			return restarted, fmt.Errorf("negentropy is not enabled")
		}
		s.stopProcess(s.negentropyProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startNegentropy(); err != nil {
			return restarted, fmt.Errorf("failed to restart negentropy: %w", err)
		}
		restarted = append(restarted, "orly-sync-negentropy")

	case "orly-certs", "certs":
		if !s.cfg.CertsEnabled {
			return restarted, fmt.Errorf("certificate service is not enabled")
		}
		s.stopProcess(s.certsProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startCerts(); err != nil {
			return restarted, fmt.Errorf("failed to restart certificate service: %w", err)
		}
		restarted = append(restarted, "orly-certs")

	case "orly", "relay":
		// Just restart relay
		s.stopProcess(s.relayProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startRelay(); err != nil {
			return restarted, fmt.Errorf("failed to restart relay: %w", err)
		}
		restarted = append(restarted, "orly")

	default:
		return restarted, fmt.Errorf("unknown service: %s", serviceName)
	}

	log.I.F("restarted services: %v", restarted)
	return restarted, nil
}

// StartService starts a specific service if it's not already running.
func (s *Supervisor) StartService(serviceName string) error {
	log.I.F("starting service: %s", serviceName)

	switch serviceName {
	case "orly-db", "db":
		if err := s.startDB(); err != nil {
			return fmt.Errorf("failed to start db: %w", err)
		}
		if err := s.waitForDBReady(s.cfg.DBReadyTimeout); err != nil {
			return fmt.Errorf("db not ready: %w", err)
		}

	case "orly-acl", "acl":
		if !s.cfg.ACLEnabled {
			return fmt.Errorf("ACL is not enabled in configuration")
		}
		if err := s.startACL(); err != nil {
			return fmt.Errorf("failed to start acl: %w", err)
		}
		if err := s.waitForACLReady(s.cfg.ACLReadyTimeout); err != nil {
			return fmt.Errorf("acl not ready: %w", err)
		}

	case "orly-sync-distributed", "distributed-sync":
		if !s.cfg.DistributedSyncEnabled {
			return fmt.Errorf("distributed sync is not enabled in configuration")
		}
		if err := s.startDistributedSync(); err != nil {
			return fmt.Errorf("failed to start distributed sync: %w", err)
		}

	case "orly-sync-cluster", "cluster-sync":
		if !s.cfg.ClusterSyncEnabled {
			return fmt.Errorf("cluster sync is not enabled in configuration")
		}
		if err := s.startClusterSync(); err != nil {
			return fmt.Errorf("failed to start cluster sync: %w", err)
		}

	case "orly-sync-relaygroup", "relaygroup":
		if !s.cfg.RelayGroupEnabled {
			return fmt.Errorf("relaygroup is not enabled in configuration")
		}
		if err := s.startRelayGroup(); err != nil {
			return fmt.Errorf("failed to start relaygroup: %w", err)
		}

	case "orly-sync-negentropy", "negentropy":
		if !s.cfg.NegentropyEnabled {
			return fmt.Errorf("negentropy is not enabled in configuration")
		}
		if err := s.startNegentropy(); err != nil {
			return fmt.Errorf("failed to start negentropy: %w", err)
		}

	case "orly-certs", "certs":
		if !s.cfg.CertsEnabled {
			return fmt.Errorf("certificate service is not enabled in configuration")
		}
		if err := s.startCerts(); err != nil {
			return fmt.Errorf("failed to start certificate service: %w", err)
		}

	case "orly", "relay":
		if err := s.startRelay(); err != nil {
			return fmt.Errorf("failed to start relay: %w", err)
		}

	default:
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	log.I.F("started service: %s", serviceName)
	return nil
}

// StopService stops a specific service and its dependents.
// Dependency chain: db → acl, sync services → relay
// Stopping a service will first stop all services that depend on it.
func (s *Supervisor) StopService(serviceName string) error {
	log.I.F("stopping service: %s", serviceName)

	switch serviceName {
	case "orly-db", "db":
		// DB is the root - everything depends on it
		// Stop in reverse dependency order: relay, sync, acl, then db
		log.I.F("stopping relay (depends on db)")
		s.stopProcess(s.relayProc, 5*time.Second)

		log.I.F("stopping sync services (depend on db)")
		s.stopSyncServices()

		if s.cfg.ACLEnabled && s.aclProc != nil {
			log.I.F("stopping acl (depends on db)")
			s.stopProcess(s.aclProc, 5*time.Second)
		}

		log.I.F("stopping db")
		s.stopProcess(s.dbProc, s.cfg.StopTimeout)

	case "orly-acl", "acl":
		// Relay depends on ACL when ACL is enabled
		if s.cfg.ACLEnabled {
			log.I.F("stopping relay (depends on acl)")
			s.stopProcess(s.relayProc, 5*time.Second)
		}
		if s.aclProc != nil {
			s.stopProcess(s.aclProc, 5*time.Second)
		}

	case "orly-sync-distributed", "distributed-sync":
		// Relay may depend on sync services
		if s.distributedSyncProc != nil {
			s.stopProcess(s.distributedSyncProc, 5*time.Second)
		}

	case "orly-sync-cluster", "cluster-sync":
		if s.clusterSyncProc != nil {
			s.stopProcess(s.clusterSyncProc, 5*time.Second)
		}

	case "orly-sync-relaygroup", "relaygroup":
		if s.relayGroupProc != nil {
			s.stopProcess(s.relayGroupProc, 5*time.Second)
		}

	case "orly-sync-negentropy", "negentropy":
		if s.negentropyProc != nil {
			s.stopProcess(s.negentropyProc, 5*time.Second)
		}

	case "orly-certs", "certs":
		// Certs is independent
		if s.certsProc != nil {
			s.stopProcess(s.certsProc, 5*time.Second)
		}

	case "orly", "relay":
		// Relay is a leaf - nothing depends on it
		s.stopProcess(s.relayProc, 5*time.Second)

	default:
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	log.I.F("stopped service: %s", serviceName)
	return nil
}

// RestartAll stops all processes and starts them again.
func (s *Supervisor) RestartAll() error {
	log.I.F("restarting all processes...")

	// Stop in reverse dependency order
	s.mu.Lock()
	if s.relayProc != nil {
		s.mu.Unlock()
		s.stopProcess(s.relayProc, 5*time.Second)
		s.mu.Lock()
	}

	s.mu.Unlock()
	s.stopSyncServices()
	s.mu.Lock()

	if s.cfg.ACLEnabled && s.aclProc != nil {
		s.mu.Unlock()
		s.stopProcess(s.aclProc, 5*time.Second)
		s.mu.Lock()
	}

	if s.dbProc != nil {
		s.mu.Unlock()
		s.stopProcess(s.dbProc, s.cfg.StopTimeout)
		s.mu.Lock()
	}
	s.mu.Unlock()

	// Small delay to ensure ports are released
	time.Sleep(500 * time.Millisecond)

	// Start again in dependency order
	if err := s.startDB(); err != nil {
		return fmt.Errorf("failed to restart database: %w", err)
	}

	if err := s.waitForDBReady(s.cfg.DBReadyTimeout); err != nil {
		return fmt.Errorf("database not ready after restart: %w", err)
	}

	if s.cfg.ACLEnabled {
		if err := s.startACL(); err != nil {
			return fmt.Errorf("failed to restart ACL: %w", err)
		}
		if err := s.waitForACLReady(s.cfg.ACLReadyTimeout); err != nil {
			return fmt.Errorf("ACL not ready after restart: %w", err)
		}
	}

	if err := s.startSyncServices(); err != nil {
		return fmt.Errorf("failed to restart sync services: %w", err)
	}

	if err := s.startRelay(); err != nil {
		return fmt.Errorf("failed to restart relay: %w", err)
	}

	log.I.F("all processes restarted successfully")
	return nil
}
