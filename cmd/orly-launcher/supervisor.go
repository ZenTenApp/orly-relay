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

// Supervisor manages the database, ACL, and relay processes.
type Supervisor struct {
	cfg    *Config
	ctx    context.Context
	cancel context.CancelFunc

	dbProc    *Process
	aclProc   *Process
	relayProc *Process

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

// Start starts the database, optional ACL server, and relay processes.
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

	// 4. Start relay with gRPC backend(s)
	if err := s.startRelay(); err != nil {
		if s.cfg.ACLEnabled {
			s.stopACL()
		}
		s.stopDB()
		return fmt.Errorf("failed to start relay: %w", err)
	}

	// 5. Start monitoring goroutines
	monitorCount := 2
	if s.cfg.ACLEnabled {
		monitorCount = 3
	}
	s.wg.Add(monitorCount)
	go s.monitorProcess(s.dbProc, "db", s.startDB)
	if s.cfg.ACLEnabled {
		go s.monitorProcess(s.aclProc, "acl", s.startACL)
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

	// Stop relay first (it depends on ACL and DB)
	log.I.F("stopping relay...")
	s.stopProcess(s.relayProc, 5*time.Second)

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
			default:
				p = s.relayProc
			}
			s.mu.Unlock()
		}
	}
}
