// Package tor provides Tor hidden service integration for the ORLY relay.
// It spawns a tor subprocess with automatic configuration and manages
// the hidden service lifecycle.
package tor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Config holds Tor subprocess configuration.
type Config struct {
	// Port is the internal port for the hidden service
	Port int
	// DataDir is the directory for Tor data (torrc, keys, hostname, etc.)
	DataDir string
	// Binary is the path to the tor executable
	Binary string
	// SOCKSPort is the port for outbound SOCKS connections (0 = disabled)
	SOCKSPort int
	// Handler is the HTTP handler to serve (typically the main relay handler)
	Handler http.Handler
}

// Service manages the Tor subprocess and hidden service listener.
type Service struct {
	cfg      *Config
	listener net.Listener
	server   *http.Server

	// Tor subprocess
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser

	// onionAddress is the detected .onion address
	onionAddress string

	// hostname watcher
	hostnameWatcher *HostnameWatcher

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// New creates a new Tor service with the given configuration.
// Returns an error if the tor binary is not found.
func New(cfg *Config) (*Service, error) {
	if cfg.Port == 0 {
		cfg.Port = 3336
	}

	// Find tor binary
	binary := cfg.Binary
	if binary == "" {
		binary = "tor"
	}

	torPath, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("tor binary not found: %w (install tor or set ORLY_TOR_ENABLED=false)", err)
	}
	cfg.Binary = torPath

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create Tor data directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}

	return s, nil
}

// generateTorrc creates the torrc configuration file.
func (s *Service) generateTorrc() (string, error) {
	torrcPath := filepath.Join(s.cfg.DataDir, "torrc")
	hsDir := filepath.Join(s.cfg.DataDir, "hidden_service")

	// Ensure hidden service directory exists with correct permissions
	if err := os.MkdirAll(hsDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create hidden service directory: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# ORLY Tor hidden service configuration\n")
	sb.WriteString("# Auto-generated - do not edit\n\n")

	// Data directory
	sb.WriteString(fmt.Sprintf("DataDirectory %s/data\n", s.cfg.DataDir))

	// Hidden service configuration
	sb.WriteString(fmt.Sprintf("HiddenServiceDir %s\n", hsDir))
	sb.WriteString(fmt.Sprintf("HiddenServicePort 80 127.0.0.1:%d\n", s.cfg.Port))

	// Optional SOCKS port for outbound connections
	if s.cfg.SOCKSPort > 0 {
		sb.WriteString(fmt.Sprintf("SocksPort %d\n", s.cfg.SOCKSPort))
	} else {
		sb.WriteString("SocksPort 0\n")
	}

	// Disable unused features to reduce resource usage
	sb.WriteString("ControlPort 0\n")
	sb.WriteString("Log notice stdout\n")

	// Write torrc
	if err := os.WriteFile(torrcPath, []byte(sb.String()), 0600); err != nil {
		return "", fmt.Errorf("failed to write torrc: %w", err)
	}

	// Create data subdirectory
	if err := os.MkdirAll(filepath.Join(s.cfg.DataDir, "data"), 0700); err != nil {
		return "", fmt.Errorf("failed to create Tor data subdirectory: %w", err)
	}

	return torrcPath, nil
}

// killOrphanedTor finds and kills any Tor process using our torrc that was
// orphaned from a previous run. This prevents "another Tor process is running
// with the same data directory" errors on startup.
func (s *Service) killOrphanedTor(torrcPath string) {
	// Look for tor processes using our specific torrc
	out, err := exec.Command("pgrep", "-f", s.cfg.Binary+" -f "+torrcPath).Output()
	if err != nil {
		// pgrep returns exit 1 when no processes found — that's fine
		return
	}

	pids := strings.TrimSpace(string(out))
	if pids == "" {
		return
	}

	log.W.F("found orphaned Tor process(es) using %s: %s — killing", torrcPath, pids)
	for _, pid := range strings.Split(pids, "\n") {
		pid = strings.TrimSpace(pid)
		if pid == "" {
			continue
		}
		killCmd := exec.Command("kill", "-9", pid)
		if err := killCmd.Run(); err != nil {
			log.W.F("failed to kill orphaned Tor PID %s: %v", pid, err)
		} else {
			log.I.F("killed orphaned Tor PID %s", pid)
		}
	}

	// Give the OS a moment to release the lock file
	time.Sleep(500 * time.Millisecond)
}

// Start spawns the Tor subprocess and initializes the listener.
func (s *Service) Start() error {
	// Generate torrc
	torrcPath, err := s.generateTorrc()
	if err != nil {
		return err
	}

	// Kill any orphaned Tor processes from a previous run before starting
	s.killOrphanedTor(torrcPath)

	log.I.F("starting Tor subprocess with config: %s", torrcPath)

	// Use exec.Command (not CommandContext) so we control the shutdown
	// sequence ourselves. CommandContext sends SIGKILL immediately on
	// context cancel, which races with our graceful SIGTERM shutdown.
	s.cmd = exec.Command(s.cfg.Binary, "-f", torrcPath)

	// Set platform-specific process attributes. On Linux this sets
	// Pdeathsig to SIGKILL, ensuring the kernel kills the Tor subprocess
	// if the parent relay process dies unexpectedly.
	if attr := sysProcAttr(); attr != nil {
		s.cmd.SysProcAttr = attr
	}

	// Capture stdout/stderr for logging
	s.stdout, err = s.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get Tor stdout: %w", err)
	}
	s.stderr, err = s.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get Tor stderr: %w", err)
	}

	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Tor: %w", err)
	}

	log.I.F("Tor subprocess started (PID %d)", s.cmd.Process.Pid)

	// Log Tor output
	s.wg.Add(2)
	go s.logOutput("tor", s.stdout)
	go s.logOutput("tor", s.stderr)

	// Monitor subprocess and context cancellation
	s.wg.Add(1)
	go s.monitorProcess()

	// Start hostname watcher
	hsDir := filepath.Join(s.cfg.DataDir, "hidden_service")
	s.hostnameWatcher = NewHostnameWatcher(hsDir)
	s.hostnameWatcher.OnChange(func(addr string) {
		s.mu.Lock()
		s.onionAddress = addr
		s.mu.Unlock()
		log.I.F("Tor hidden service address: %s", addr)
	})
	if err := s.hostnameWatcher.Start(); err != nil {
		log.W.F("failed to start hostname watcher: %v", err)
	} else {
		// Get initial address
		if addr := s.hostnameWatcher.Address(); addr != "" {
			s.mu.Lock()
			s.onionAddress = addr
			s.mu.Unlock()
		}
	}

	// Create listener for the hidden service port
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Port)
	s.listener, err = net.Listen("tcp", addr)
	if chk.E(err) {
		s.Stop()
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	// Create HTTP server with the provided handler
	s.server = &http.Server{
		Handler:      s.cfg.Handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start serving
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.I.F("Tor hidden service listener started on %s", addr)
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.E.F("Tor server error: %v", err)
		}
	}()

	return nil
}

// logOutput reads from a pipe and logs each line.
func (s *Service) logOutput(prefix string, r io.ReadCloser) {
	defer s.wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		// Filter out common noise
		if strings.Contains(line, "compression bomb") ||
			strings.Contains(line, "abandoning stream") {
			log.T.F("[%s] %s", prefix, line)
		} else if strings.Contains(line, "Bootstrapped") {
			log.I.F("[%s] %s", prefix, line)
		} else if strings.Contains(line, "[warn]") || strings.Contains(line, "[err]") {
			log.W.F("[%s] %s", prefix, line)
		} else {
			log.D.F("[%s] %s", prefix, line)
		}
	}
}

// monitorProcess watches the Tor subprocess and logs when it exits.
func (s *Service) monitorProcess() {
	defer s.wg.Done()
	err := s.cmd.Wait()
	if err != nil {
		select {
		case <-s.ctx.Done():
			// Expected shutdown
			log.D.F("Tor subprocess exited (shutdown)")
		default:
			log.E.F("Tor subprocess exited unexpectedly: %v", err)
		}
	} else {
		log.I.F("Tor subprocess exited cleanly")
	}
}

// Stop gracefully shuts down the Tor service.
func (s *Service) Stop() error {
	// Terminate Tor subprocess FIRST, before cancelling context.
	// This avoids a race where context cancellation closes pipes/waitgroups
	// while we're still trying to signal the process.
	if s.cmd != nil && s.cmd.Process != nil {
		pid := s.cmd.Process.Pid
		log.D.F("sending SIGTERM to Tor subprocess (PID %d)", pid)
		s.cmd.Process.Signal(os.Interrupt)

		// Give it a few seconds to exit gracefully
		done := make(chan struct{})
		go func() {
			s.cmd.Wait()
			close(done)
		}()

		select {
		case <-done:
			log.D.F("Tor subprocess (PID %d) exited gracefully", pid)
		case <-time.After(5 * time.Second):
			log.W.F("Tor subprocess (PID %d) did not exit, sending SIGKILL", pid)
			s.cmd.Process.Kill()
		}
	}

	// Now cancel context to stop all goroutines
	s.cancel()

	// Stop hostname watcher
	if s.hostnameWatcher != nil {
		s.hostnameWatcher.Stop()
	}

	// Shutdown HTTP server
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(ctx); chk.E(err) {
			// Continue shutdown anyway
		}
	}

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	s.wg.Wait()
	log.I.F("Tor service stopped")
	return nil
}

// OnionAddress returns the current .onion address.
func (s *Service) OnionAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.onionAddress
}

// OnionWSAddress returns the full WebSocket URL for the hidden service.
// Format: ws://<address>.onion/
func (s *Service) OnionWSAddress() string {
	addr := s.OnionAddress()
	if addr == "" {
		return ""
	}
	// Ensure address ends with .onion
	if len(addr) >= 6 && addr[len(addr)-6:] != ".onion" {
		addr = addr + ".onion"
	}
	return "ws://" + addr + "/"
}

// IsRunning returns whether the Tor service is currently running.
func (s *Service) IsRunning() bool {
	return s.listener != nil && s.cmd != nil && s.cmd.Process != nil
}

// Upgrader returns a WebSocket upgrader configured for Tor connections.
// Tor connections don't send Origin headers, so we skip origin check.
func (s *Service) Upgrader() *websocket.Upgrader {
	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for Tor
		},
	}
}

// DataDir returns the Tor data directory path.
func (s *Service) DataDir() string {
	return s.cfg.DataDir
}

// HiddenServiceDir returns the hidden service directory path.
func (s *Service) HiddenServiceDir() string {
	return filepath.Join(s.cfg.DataDir, "hidden_service")
}
