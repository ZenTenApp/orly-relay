package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"

	orlyaclv1 "git.smesh.lol/orly/pkg/proto/orlyacl/v1"
	orlynitsv1 "git.smesh.lol/orly/pkg/proto/orlynits/v1"
)

// procReqKind identifies the type of request sent to a process actor.
type procReqKind int

const (
	procReqStatus procReqKind = iota
	procReqCmdInfo
	procReqIncRestarts
)

// procRequest is a message sent to a process actor goroutine.
type procRequest struct {
	kind procReqKind
	resp chan procResponse
}

// procResponse carries data back from the process actor.
type procResponse struct {
	cmd      *exec.Cmd
	restarts int
}

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

	// Bitcoin node (nits) process
	nitsProc *Process

	// Lightning node (luk) process
	lukProc *Process

	// Wallet (strela) process
	strelaProc *Process

	monitorDone []chan struct{} // one per monitor goroutine
	stopCh      chan struct{}   // closed once to signal shutdown
	stopped     atomic.Bool
}

// Process represents a managed subprocess.
// State is owned by an actor goroutine; all access goes through reqCh.
type Process struct {
	name   string
	exited chan struct{} // closed when process exits
	reqCh  chan procRequest
}

// newProcess creates a Process and starts its actor goroutine.
func newProcess(name string, cmd *exec.Cmd, exited chan struct{}) *Process {
	p := &Process{
		name:   name,
		exited: exited,
		reqCh:  make(chan procRequest),
	}
	go p.actor(cmd, 0)
	return p
}

// actor owns cmd and restarts; processes requests sequentially.
func (p *Process) actor(cmd *exec.Cmd, restarts int) {
	for req := range p.reqCh {
		switch req.kind {
		case procReqStatus, procReqCmdInfo:
			req.resp <- procResponse{cmd: cmd, restarts: restarts}
		case procReqIncRestarts:
			restarts++
			req.resp <- procResponse{restarts: restarts}
		}
	}
}

// query sends a request and waits for the response.
func (p *Process) query(kind procReqKind) procResponse {
	resp := make(chan procResponse, 1)
	p.reqCh <- procRequest{kind: kind, resp: resp}
	return <-resp
}

// closeActor shuts down the actor goroutine.
func (p *Process) closeActor() {
	close(p.reqCh)
}

// NewSupervisor creates a new process supervisor.
func NewSupervisor(ctx context.Context, cancel context.CancelFunc, cfg *Config) *Supervisor {
	return &Supervisor{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		stopCh: make(chan struct{}),
	}
}

// isStopped returns true if Stop has been called.
func (s *Supervisor) isStopped() bool {
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}

// IsRunning returns true if any managed processes are running.
func (s *Supervisor) IsRunning() bool {
	if s.dbProc != nil {
		select {
		case <-s.dbProc.exited:
		default:
			return true
		}
	}
	if s.relayProc != nil {
		select {
		case <-s.relayProc.exited:
		default:
			return true
		}
	}
	return false
}

// Start starts the database, optional ACL server, sync services, and relay processes.
func (s *Supervisor) Start() error {
	// Reset stop state for fresh start
	if s.stopped.Load() {
		s.stopped.Store(false)
		s.stopCh = make(chan struct{})
	}

	// Phase 1: Start Bitcoin node if enabled (nits - luk depends on it)
	if s.cfg.NitsEnabled {
		if err := s.startNits(); err != nil {
			return fmt.Errorf("failed to start nits: %w", err)
		}

		if err := s.waitForNitsReady(s.cfg.NitsReadyTimeout); err != nil {
			s.stopProcess(s.nitsProc, 30*time.Second)
			return fmt.Errorf("nits not ready: %w", err)
		}

		log.I.F("bitcoin node is ready")
	}

	// Phase 2: Start Lightning node if enabled (luk - strela depends on it)
	if s.cfg.LukEnabled {
		if err := s.startLuk(); err != nil {
			if s.cfg.NitsEnabled {
				s.stopProcess(s.nitsProc, 30*time.Second)
			}
			return fmt.Errorf("failed to start luk: %w", err)
		}

		if err := s.waitForServiceReady(s.cfg.LukRPCListen, s.cfg.LukReadyTimeout); err != nil {
			s.stopProcess(s.lukProc, 10*time.Second)
			if s.cfg.NitsEnabled {
				s.stopProcess(s.nitsProc, 30*time.Second)
			}
			return fmt.Errorf("luk not ready: %w", err)
		}

		log.I.F("lightning node is ready")
	}

	// Phase 3: Start database server
	if err := s.startDB(); err != nil {
		return fmt.Errorf("failed to start database: %w", err)
	}

	if err := s.waitForDBReady(s.cfg.DBReadyTimeout); err != nil {
		s.stopDB()
		return fmt.Errorf("database not ready: %w", err)
	}

	log.I.F("database is ready")

	// Phase 4: Start ACL server if enabled
	if s.cfg.ACLEnabled {
		if err := s.startACL(); err != nil {
			s.stopDB()
			return fmt.Errorf("failed to start ACL server: %w", err)
		}

		if err := s.waitForACLReady(s.cfg.ACLReadyTimeout); err != nil {
			s.stopACL()
			s.stopDB()
			return fmt.Errorf("ACL server not ready: %w", err)
		}

		log.I.F("ACL server is ready")
	}

	// Start sync services in parallel (they all depend on DB)
	if err := s.startSyncServices(); err != nil {
		s.stopSyncServices()
		if s.cfg.ACLEnabled {
			s.stopACL()
		}
		s.stopDB()
		return fmt.Errorf("failed to start sync services: %w", err)
	}

	// Phase 5: Start strela (needs luk) + relay (needs db/acl) in parallel
	if s.cfg.StrelaEnabled {
		if err := s.startStrela(); err != nil {
			log.W.F("failed to start strela: %v", err)
		} else {
			log.I.F("strela wallet started")
		}
	}

	if err := s.startRelay(); err != nil {
		s.stopSyncServices()
		if s.cfg.ACLEnabled {
			s.stopACL()
		}
		s.stopDB()
		return fmt.Errorf("failed to start relay: %w", err)
	}

	// Phase 6: Start certificate service if enabled (independent)
	if s.cfg.CertsEnabled {
		if err := s.startCerts(); err != nil {
			log.W.F("failed to start certificate service: %v", err)
		} else {
			log.I.F("certificate service started")
		}
	}

	// Start monitoring goroutines
	s.monitorDone = nil

	if s.cfg.NitsEnabled {
		s.launchMonitor(s.nitsProc, "nits", s.startNits)
	}
	if s.cfg.LukEnabled {
		s.launchMonitor(s.lukProc, "luk", s.startLuk)
	}
	s.launchMonitor(s.dbProc, "db", s.startDB)
	if s.cfg.ACLEnabled {
		s.launchMonitor(s.aclProc, "acl", s.startACL)
	}
	if s.cfg.DistributedSyncEnabled {
		s.launchMonitor(s.distributedSyncProc, "distributed-sync", s.startDistributedSync)
	}
	if s.cfg.ClusterSyncEnabled {
		s.launchMonitor(s.clusterSyncProc, "cluster-sync", s.startClusterSync)
	}
	if s.cfg.RelayGroupEnabled {
		s.launchMonitor(s.relayGroupProc, "relaygroup", s.startRelayGroup)
	}
	if s.cfg.NegentropyEnabled {
		s.launchMonitor(s.negentropyProc, "negentropy", s.startNegentropy)
	}
	if s.cfg.StrelaEnabled {
		s.launchMonitor(s.strelaProc, "strela", s.startStrela)
	}
	if s.cfg.CertsEnabled {
		s.launchMonitor(s.certsProc, "certs", s.startCerts)
	}
	s.launchMonitor(s.relayProc, "relay", s.startRelay)

	return nil
}

// launchMonitor starts a monitor goroutine and tracks its done channel.
func (s *Supervisor) launchMonitor(p *Process, procType string, restart func() error) {
	done := make(chan struct{})
	s.monitorDone = append(s.monitorDone, done)
	go s.monitorProcess(p, procType, restart, done)
}

// Stop stops all managed processes gracefully.
func (s *Supervisor) Stop() error {
	if !s.stopped.CompareAndSwap(false, true) {
		return nil // already stopped
	}
	close(s.stopCh)

	// Stop in reverse startup order: certs - relay, strela - acl, sync - db - luk - nits

	if s.cfg.CertsEnabled && s.certsProc != nil {
		log.I.F("stopping certificate service...")
		s.stopProcess(s.certsProc, 5*time.Second)
	}

	log.I.F("stopping relay...")
	s.stopProcess(s.relayProc, 5*time.Second)

	if s.cfg.StrelaEnabled && s.strelaProc != nil {
		log.I.F("stopping strela...")
		s.stopProcess(s.strelaProc, 5*time.Second)
	}

	log.I.F("stopping sync services...")
	s.stopSyncServices()

	if s.cfg.ACLEnabled && s.aclProc != nil {
		log.I.F("stopping ACL server...")
		s.stopProcess(s.aclProc, 5*time.Second)
	}

	log.I.F("stopping database...")
	s.stopProcess(s.dbProc, s.cfg.StopTimeout)

	if s.cfg.LukEnabled && s.lukProc != nil {
		log.I.F("stopping lightning node...")
		s.stopProcess(s.lukProc, 10*time.Second)
	}

	if s.cfg.NitsEnabled && s.nitsProc != nil {
		log.I.F("stopping bitcoin node...")
		s.stopProcess(s.nitsProc, 30*time.Second)
	}

	// Wait for all monitor goroutines to exit
	for _, done := range s.monitorDone {
		<-done
	}

	return nil
}

func (s *Supervisor) startDB() error {
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
	s.dbProc = newProcess("orly-db", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started database server (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startACL() error {
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
	s.aclProc = newProcess("orly-acl", cmd, exited)

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
	s.relayProc = newProcess("orly", cmd, exited)

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

	r := p.query(procReqCmdInfo)
	if r.cmd == nil || r.cmd.Process == nil {
		return
	}

	// Send SIGTERM for graceful shutdown
	if err := r.cmd.Process.Signal(syscall.SIGTERM); err != nil {
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
		r.cmd.Process.Kill()
		<-p.exited // Wait for the kill to complete
	}
}

func (s *Supervisor) monitorProcess(p *Process, procType string, restart func() error, done chan struct{}) {
	defer close(done)

	for {
		if s.isStopped() {
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
		if s.isStopped() {
			return
		}

		// Process exited unexpectedly - increment restarts via actor
		r := p.query(procReqIncRestarts)
		log.W.F("%s exited unexpectedly, restart count: %d", p.name, r.restarts)

		// Backoff before restart
		backoff := time.Duration(r.restarts) * time.Second
		if backoff > 30*time.Second {
			backoff = 30 * time.Second
		}

		select {
		case <-s.ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Check one more time before restarting
		if s.isStopped() {
			return
		}

		if err := restart(); err != nil {
			log.E.F("failed to restart %s: %v", p.name, err)
		} else {
			// Update p to point to the new process
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
			case "nits":
				p = s.nitsProc
			case "luk":
				p = s.lukProc
			case "strela":
				p = s.strelaProc
			default:
				p = s.relayProc
			}
		}
	}
}

// startSyncServices starts all enabled sync services in parallel.
func (s *Supervisor) startSyncServices() error {
	type result struct {
		err error
	}

	var chans []chan result

	startOne := func(start func() error, addr string, timeout time.Duration, label string) {
		ch := make(chan result, 1)
		chans = append(chans, ch)
		go func() {
			if err := start(); err != nil {
				ch <- result{fmt.Errorf("%s: %w", label, err)}
				return
			}
			if err := s.waitForServiceReady(addr, timeout); err != nil {
				ch <- result{fmt.Errorf("%s not ready: %w", label, err)}
				return
			}
			log.I.F("%s service is ready", label)
			ch <- result{}
		}()
	}

	if s.cfg.DistributedSyncEnabled {
		startOne(s.startDistributedSync, s.cfg.DistributedSyncListen, s.cfg.SyncReadyTimeout, "distributed sync")
	}
	if s.cfg.ClusterSyncEnabled {
		startOne(s.startClusterSync, s.cfg.ClusterSyncListen, s.cfg.SyncReadyTimeout, "cluster sync")
	}
	if s.cfg.RelayGroupEnabled {
		startOne(s.startRelayGroup, s.cfg.RelayGroupListen, s.cfg.SyncReadyTimeout, "relaygroup")
	}
	if s.cfg.NegentropyEnabled {
		startOne(s.startNegentropy, s.cfg.NegentropyListen, s.cfg.SyncReadyTimeout, "negentropy")
	}

	// Wait for all
	for _, ch := range chans {
		r := <-ch
		if r.err != nil {
			return r.err
		}
	}
	return nil
}

// stopSyncServices stops all sync services.
func (s *Supervisor) stopSyncServices() {
	var doneChans []chan struct{}

	stopOne := func(p *Process) {
		if p == nil {
			return
		}
		ch := make(chan struct{})
		doneChans = append(doneChans, ch)
		go func() {
			s.stopProcess(p, 5*time.Second)
			close(ch)
		}()
	}

	stopOne(s.distributedSyncProc)
	stopOne(s.clusterSyncProc)
	stopOne(s.relayGroupProc)
	stopOne(s.negentropyProc)

	for _, ch := range doneChans {
		<-ch
	}
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
	s.distributedSyncProc = newProcess("orly-sync-distributed", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started distributed sync service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startClusterSync() error {
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
	s.clusterSyncProc = newProcess("orly-sync-cluster", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started cluster sync service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startRelayGroup() error {
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
	s.relayGroupProc = newProcess("orly-sync-relaygroup", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started relaygroup service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startNegentropy() error {
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
	s.negentropyProc = newProcess("orly-sync-negentropy", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started negentropy service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startCerts() error {
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
	s.certsProc = newProcess("orly-certs", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started certificate service (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startNits() error {
	env := os.Environ()
	env = append(env, fmt.Sprintf("ORLY_NITS_LISTEN=%s", s.cfg.NitsListen))
	env = append(env, fmt.Sprintf("ORLY_NITS_BITCOIND=%s", s.cfg.NitsBinary))
	env = append(env, fmt.Sprintf("ORLY_NITS_RPC_PORT=%d", s.cfg.NitsRPCPort))
	env = append(env, fmt.Sprintf("ORLY_NITS_DATA_DIR=%s", s.cfg.NitsDataDir))
	env = append(env, fmt.Sprintf("ORLY_NITS_PRUNE_MB=%d", s.cfg.NitsPruneMB))
	env = append(env, fmt.Sprintf("ORLY_NITS_NETWORK=%s", s.cfg.NitsNetwork))
	env = append(env, fmt.Sprintf("ORLY_NITS_LOG_LEVEL=%s", s.cfg.LogLevel))

	cmd := exec.CommandContext(s.ctx, s.cfg.NitsShimBinary)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.nitsProc = newProcess("orly-nits", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started bitcoin node shim (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) waitForNitsReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var grpcConn *grpc.ClientConn
	var nitsClient orlynitsv1.NitsServiceClient

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
				return fmt.Errorf("timeout waiting for bitcoin node")
			}

			// Check TCP first
			conn, err := net.DialTimeout("tcp", s.cfg.NitsListen, time.Second)
			if err != nil {
				continue
			}
			conn.Close()

			// Then check gRPC Ready()
			if grpcConn == nil {
				grpcConn, err = grpc.DialContext(s.ctx, s.cfg.NitsListen,
					grpc.WithTransportCredentials(insecure.NewCredentials()),
				)
				if err != nil {
					continue
				}
				nitsClient = orlynitsv1.NewNitsServiceClient(grpcConn)
			}

			ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
			resp, err := nitsClient.Ready(ctx, &orlynitsv1.Empty{})
			cancel()
			if err == nil && resp.Ready {
				grpcConn.Close()
				return nil
			}
			// Bitcoind responsive but still syncing is fine for luk to start
			if err == nil && resp.Status == "syncing" {
				grpcConn.Close()
				log.I.F("bitcoin node responsive (syncing %d%%), proceeding", resp.SyncProgressPercent)
				return nil
			}
		}
	}
}

func (s *Supervisor) startLuk() error {
	// luk (LND fork) uses command-line flags
	args := []string{
		fmt.Sprintf("--lnddir=%s", s.cfg.LukDataDir),
		fmt.Sprintf("--rpclisten=%s", s.cfg.LukRPCListen),
		fmt.Sprintf("--listen=%s", s.cfg.LukPeerListen),
		fmt.Sprintf("--bitcoind.rpchost=127.0.0.1:%d", s.cfg.NitsRPCPort),
	}

	// Set network flag if not mainnet
	switch s.cfg.NitsNetwork {
	case "testnet":
		args = append(args, "--bitcoin.testnet")
	case "signet":
		args = append(args, "--bitcoin.signet")
	case "regtest":
		args = append(args, "--bitcoin.regtest")
	}

	cmd := exec.CommandContext(s.ctx, s.cfg.LukBinary, args...)
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.lukProc = newProcess("luk", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started lightning node (pid %d)", cmd.Process.Pid)
	return nil
}

func (s *Supervisor) startStrela() error {
	env := os.Environ()
	env = append(env, "LN_BACKEND_TYPE=LND")
	env = append(env, fmt.Sprintf("LND_ADDRESS=%s", s.cfg.LukRPCListen))
	env = append(env, fmt.Sprintf("LND_CERT_FILE=%s/tls.cert", s.cfg.LukDataDir))
	env = append(env, fmt.Sprintf("LND_MACAROON_FILE=%s/data/chain/bitcoin/mainnet/admin.macaroon", s.cfg.LukDataDir))
	env = append(env, fmt.Sprintf("PORT=%d", s.cfg.StrelaPort))
	env = append(env, fmt.Sprintf("WORK_DIR=%s", s.cfg.StrelaDataDir))
	env = append(env, fmt.Sprintf("LOG_LEVEL=%s", s.cfg.LogLevel))

	cmd := exec.CommandContext(s.ctx, s.cfg.StrelaBinary)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); chk.E(err) {
		return err
	}

	exited := make(chan struct{})
	s.strelaProc = newProcess("strela", cmd, exited)

	go func() {
		cmd.Wait()
		close(exited)
	}()

	log.I.F("started strela wallet (pid %d)", cmd.Process.Pid)
	return nil
}

// GetProcessStatuses returns the status of all available modules with categories.
// Modules are grouped by category, and some categories are mutually exclusive.
func (s *Supervisor) GetProcessStatuses() []ProcessStatus {
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

	// Bitcoin node (nits)
	statuses = append(statuses, s.getProcessStatusFull(
		s.nitsProc, "orly-nits", s.cfg.NitsShimBinary,
		s.cfg.NitsEnabled, "bitcoin", "Bitcoin node manager (bitcoind)",
	))

	// Lightning node (luk)
	statuses = append(statuses, s.getProcessStatusFull(
		s.lukProc, "luk", s.cfg.LukBinary,
		s.cfg.LukEnabled, "lightning", "Lightning network node (LND fork)",
	))

	// Wallet (strela)
	statuses = append(statuses, s.getProcessStatusFull(
		s.strelaProc, "strela", s.cfg.StrelaBinary,
		s.cfg.StrelaEnabled, "wallet", "Lightning wallet web UI",
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

	r := p.query(procReqStatus)

	if r.cmd != nil && r.cmd.Process != nil {
		// Check if process is still running
		select {
		case <-p.exited:
			status = "stopped"
		default:
			status = "running"
			pid = r.cmd.Process.Pid
		}
	}

	return ProcessStatus{
		Name:     p.name,
		Binary:   binaryPath,
		Version:  "", // Will be filled by caller if needed
		Status:   status,
		Enabled:  enabled,
		PID:      pid,
		Restarts: r.restarts,
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
		r := p.query(procReqStatus)

		if r.cmd != nil && r.cmd.Process != nil {
			select {
			case <-p.exited:
				status = "stopped"
			default:
				status = "running"
				pid = r.cmd.Process.Pid
			}
		}
		restarts = r.restarts
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
	// db - acl, sync services, relay all depend on db
	// acl - relay depends on acl
	// sync services - relay may depend on sync services
	// relay - nothing depends on relay

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

	case "orly-nits", "nits":
		if !s.cfg.NitsEnabled {
			return restarted, fmt.Errorf("nits is not enabled")
		}
		// nits restart cascades to luk - strela
		if s.cfg.StrelaEnabled && s.strelaProc != nil {
			s.stopProcess(s.strelaProc, 5*time.Second)
		}
		if s.cfg.LukEnabled && s.lukProc != nil {
			s.stopProcess(s.lukProc, 10*time.Second)
		}
		s.stopProcess(s.nitsProc, 30*time.Second)
		time.Sleep(500 * time.Millisecond)

		if err := s.startNits(); err != nil {
			return restarted, fmt.Errorf("failed to restart nits: %w", err)
		}
		restarted = append(restarted, "orly-nits")

		if err := s.waitForNitsReady(s.cfg.NitsReadyTimeout); err != nil {
			return restarted, fmt.Errorf("nits not ready: %w", err)
		}

		if s.cfg.LukEnabled {
			if err := s.startLuk(); err != nil {
				return restarted, fmt.Errorf("failed to restart luk: %w", err)
			}
			restarted = append(restarted, "luk")
			if err := s.waitForServiceReady(s.cfg.LukRPCListen, s.cfg.LukReadyTimeout); err != nil {
				return restarted, fmt.Errorf("luk not ready: %w", err)
			}
		}

		if s.cfg.StrelaEnabled {
			if err := s.startStrela(); err != nil {
				return restarted, fmt.Errorf("failed to restart strela: %w", err)
			}
			restarted = append(restarted, "strela")
		}

	case "luk", "lightning":
		if !s.cfg.LukEnabled {
			return restarted, fmt.Errorf("luk is not enabled")
		}
		// luk restart cascades to strela
		if s.cfg.StrelaEnabled && s.strelaProc != nil {
			s.stopProcess(s.strelaProc, 5*time.Second)
		}
		s.stopProcess(s.lukProc, 10*time.Second)
		time.Sleep(500 * time.Millisecond)

		if err := s.startLuk(); err != nil {
			return restarted, fmt.Errorf("failed to restart luk: %w", err)
		}
		restarted = append(restarted, "luk")

		if err := s.waitForServiceReady(s.cfg.LukRPCListen, s.cfg.LukReadyTimeout); err != nil {
			return restarted, fmt.Errorf("luk not ready: %w", err)
		}

		if s.cfg.StrelaEnabled {
			if err := s.startStrela(); err != nil {
				return restarted, fmt.Errorf("failed to restart strela: %w", err)
			}
			restarted = append(restarted, "strela")
		}

	case "strela", "wallet":
		if !s.cfg.StrelaEnabled {
			return restarted, fmt.Errorf("strela is not enabled")
		}
		// strela is a leaf node, no cascade
		s.stopProcess(s.strelaProc, 5*time.Second)
		time.Sleep(500 * time.Millisecond)
		if err := s.startStrela(); err != nil {
			return restarted, fmt.Errorf("failed to restart strela: %w", err)
		}
		restarted = append(restarted, "strela")

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

	case "orly-nits", "nits":
		if !s.cfg.NitsEnabled {
			return fmt.Errorf("nits is not enabled in configuration")
		}
		if err := s.startNits(); err != nil {
			return fmt.Errorf("failed to start nits: %w", err)
		}
		if err := s.waitForNitsReady(s.cfg.NitsReadyTimeout); err != nil {
			return fmt.Errorf("nits not ready: %w", err)
		}

	case "luk", "lightning":
		if !s.cfg.LukEnabled {
			return fmt.Errorf("luk is not enabled in configuration")
		}
		if err := s.startLuk(); err != nil {
			return fmt.Errorf("failed to start luk: %w", err)
		}
		if err := s.waitForServiceReady(s.cfg.LukRPCListen, s.cfg.LukReadyTimeout); err != nil {
			return fmt.Errorf("luk not ready: %w", err)
		}

	case "strela", "wallet":
		if !s.cfg.StrelaEnabled {
			return fmt.Errorf("strela is not enabled in configuration")
		}
		if err := s.startStrela(); err != nil {
			return fmt.Errorf("failed to start strela: %w", err)
		}

	default:
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	log.I.F("started service: %s", serviceName)
	return nil
}

// StopService stops a specific service and its dependents.
// Dependency chain: db - acl, sync services - relay
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

	case "orly-nits", "nits":
		// nits is root of bitcoin stack: strela - luk - nits
		if s.cfg.StrelaEnabled && s.strelaProc != nil {
			log.I.F("stopping strela (depends on luk)")
			s.stopProcess(s.strelaProc, 5*time.Second)
		}
		if s.cfg.LukEnabled && s.lukProc != nil {
			log.I.F("stopping luk (depends on nits)")
			s.stopProcess(s.lukProc, 10*time.Second)
		}
		if s.nitsProc != nil {
			s.stopProcess(s.nitsProc, 30*time.Second)
		}

	case "luk", "lightning":
		// strela depends on luk
		if s.cfg.StrelaEnabled && s.strelaProc != nil {
			log.I.F("stopping strela (depends on luk)")
			s.stopProcess(s.strelaProc, 5*time.Second)
		}
		if s.lukProc != nil {
			s.stopProcess(s.lukProc, 10*time.Second)
		}

	case "strela", "wallet":
		// strela is a leaf
		if s.strelaProc != nil {
			s.stopProcess(s.strelaProc, 5*time.Second)
		}

	default:
		return fmt.Errorf("unknown service: %s", serviceName)
	}

	log.I.F("stopped service: %s", serviceName)
	return nil
}

// RestartAll stops all processes and starts them again.
func (s *Supervisor) RestartAll() error {
	log.I.F("restarting all processes...")

	// Stop in reverse dependency order: certs, relay, strela, sync, acl, db, luk, nits
	if s.certsProc != nil && s.cfg.CertsEnabled {
		s.stopProcess(s.certsProc, 5*time.Second)
	}

	if s.relayProc != nil {
		s.stopProcess(s.relayProc, 5*time.Second)
	}

	if s.strelaProc != nil && s.cfg.StrelaEnabled {
		s.stopProcess(s.strelaProc, 5*time.Second)
	}

	s.stopSyncServices()

	if s.cfg.ACLEnabled && s.aclProc != nil {
		s.stopProcess(s.aclProc, 5*time.Second)
	}

	if s.dbProc != nil {
		s.stopProcess(s.dbProc, s.cfg.StopTimeout)
	}

	if s.lukProc != nil && s.cfg.LukEnabled {
		s.stopProcess(s.lukProc, 10*time.Second)
	}

	if s.nitsProc != nil && s.cfg.NitsEnabled {
		s.stopProcess(s.nitsProc, 30*time.Second)
	}

	time.Sleep(500 * time.Millisecond)

	// Start again in dependency order: nits - luk - db - acl - sync - relay, strela - certs
	if s.cfg.NitsEnabled {
		if err := s.startNits(); err != nil {
			return fmt.Errorf("failed to restart nits: %w", err)
		}
		if err := s.waitForNitsReady(s.cfg.NitsReadyTimeout); err != nil {
			return fmt.Errorf("nits not ready after restart: %w", err)
		}
	}

	if s.cfg.LukEnabled {
		if err := s.startLuk(); err != nil {
			return fmt.Errorf("failed to restart luk: %w", err)
		}
		if err := s.waitForServiceReady(s.cfg.LukRPCListen, s.cfg.LukReadyTimeout); err != nil {
			return fmt.Errorf("luk not ready after restart: %w", err)
		}
	}

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

	if s.cfg.StrelaEnabled {
		if err := s.startStrela(); err != nil {
			log.W.F("failed to restart strela: %v", err)
		}
	}

	if err := s.startRelay(); err != nil {
		return fmt.Errorf("failed to restart relay: %w", err)
	}

	if s.cfg.CertsEnabled {
		if err := s.startCerts(); err != nil {
			log.W.F("failed to restart certificate service: %v", err)
		}
	}

	log.I.F("all processes restarted successfully")
	return nil
}
