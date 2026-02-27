package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/event"
)

// SprocketResponse represents a response from the sprocket script
type SprocketResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"` // accept, reject, or shadowReject
	Msg    string `json:"msg"`    // NIP-20 response message (only used for reject)
}

// SprocketManager handles sprocket script execution and management
type SprocketManager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	configDir     string
	scriptPath    string
	currentCmd    *exec.Cmd
	currentCancel context.CancelFunc
	mutex         sync.RWMutex
	isRunning     bool
	enabled       bool
	disabled      bool // true when sprocket is disabled due to failure
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	responseChan  chan SprocketResponse
}

// NewSprocketManager creates a new sprocket manager
func NewSprocketManager(ctx context.Context, appName string, enabled bool) *SprocketManager {
	configDir := filepath.Join(xdg.ConfigHome, appName)
	scriptPath := filepath.Join(configDir, "sprocket.sh")

	ctx, cancel := context.WithCancel(ctx)

	sm := &SprocketManager{
		ctx:          ctx,
		cancel:       cancel,
		configDir:    configDir,
		scriptPath:   scriptPath,
		enabled:      enabled,
		disabled:     false,
		responseChan: make(chan SprocketResponse, 100), // Buffered channel for responses
	}

	// Start the sprocket script if it exists and is enabled
	if enabled {
		go sm.startSprocketIfExists()
		// Start periodic check for sprocket script availability
		go sm.periodicCheck()
	}

	return sm
}

// disableSprocket disables sprocket due to failure
func (sm *SprocketManager) disableSprocket() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.disabled {
		sm.disabled = true
		log.W.F("sprocket disabled due to failure - all events will be rejected (script location: %s)", sm.scriptPath)
	}
}

// enableSprocket re-enables sprocket and attempts to start it
func (sm *SprocketManager) enableSprocket() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.disabled {
		sm.disabled = false
		log.I.F("sprocket re-enabled, attempting to start")

		// Attempt to start sprocket in background
		go func() {
			if _, err := os.Stat(sm.scriptPath); err == nil {
				if err := sm.StartSprocket(); err != nil {
					log.E.F("failed to restart sprocket: %v", err)
					sm.disableSprocket()
				} else {
					log.I.F("sprocket restarted successfully")
				}
			} else {
				log.W.F("sprocket script still not found, keeping disabled")
				sm.disableSprocket()
			}
		}()
	}
}

// periodicCheck periodically checks if sprocket script becomes available
func (sm *SprocketManager) periodicCheck() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.mutex.RLock()
			disabled := sm.disabled
			running := sm.isRunning
			sm.mutex.RUnlock()

			// Only check if sprocket is disabled or not running
			if disabled || !running {
				if _, err := os.Stat(sm.scriptPath); err == nil {
					// Script is available, try to enable/restart
					if disabled {
						sm.enableSprocket()
					} else if !running {
						// Script exists but sprocket isn't running, try to start
						go func() {
							if err := sm.StartSprocket(); err != nil {
								log.E.F("failed to restart sprocket: %v", err)
								sm.disableSprocket()
							} else {
								log.I.F("sprocket restarted successfully")
							}
						}()
					}
				}
			}
		}
	}
}

// startSprocketIfExists starts the sprocket script if the file exists
func (sm *SprocketManager) startSprocketIfExists() {
	if _, err := os.Stat(sm.scriptPath); err == nil {
		if err := sm.StartSprocket(); err != nil {
			log.E.F("failed to start sprocket: %v", err)
			sm.disableSprocket()
		}
	} else {
		log.W.F("sprocket script not found at %s, disabling sprocket", sm.scriptPath)
		sm.disableSprocket()
	}
}

// StartSprocket starts the sprocket script
func (sm *SprocketManager) StartSprocket() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.isRunning {
		return fmt.Errorf("sprocket is already running")
	}

	if _, err := os.Stat(sm.scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("sprocket script does not exist")
	}

	// Create a new context for this command
	cmdCtx, cmdCancel := context.WithCancel(sm.ctx)

	// Make the script executable
	if err := os.Chmod(sm.scriptPath, 0755); chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	// Start the script
	cmd := exec.CommandContext(cmdCtx, sm.scriptPath)
	cmd.Dir = sm.configDir

	// Set up stdio pipes for communication
	stdin, err := cmd.StdinPipe()
	if chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if chk.E(err) {
		cmdCancel()
		stdin.Close()
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if chk.E(err) {
		cmdCancel()
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); chk.E(err) {
		cmdCancel()
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to start sprocket: %v", err)
	}

	sm.currentCmd = cmd
	sm.currentCancel = cmdCancel
	sm.stdin = stdin
	sm.stdout = stdout
	sm.stderr = stderr
	sm.isRunning = true

	// Start response reader in background
	go sm.readResponses()

	// Log stderr output in background
	go sm.logOutput(stdout, stderr)

	// Monitor the process
	go sm.monitorProcess()

	log.I.F("sprocket started (pid=%d)", cmd.Process.Pid)
	return nil
}

// StopSprocket stops the sprocket script gracefully, with SIGKILL fallback
func (sm *SprocketManager) StopSprocket() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.isRunning || sm.currentCmd == nil {
		return fmt.Errorf("sprocket is not running")
	}

	// Close stdin first to signal the script to exit
	if sm.stdin != nil {
		sm.stdin.Close()
	}

	// Cancel the context
	if sm.currentCancel != nil {
		sm.currentCancel()
	}

	// Wait for graceful shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- sm.currentCmd.Wait()
	}()

	select {
	case <-done:
		// Process exited gracefully
		log.I.F("sprocket stopped gracefully")
	case <-time.After(5 * time.Second):
		// Force kill after 5 seconds
		log.W.F("sprocket did not stop gracefully, sending SIGKILL")
		if err := sm.currentCmd.Process.Kill(); chk.E(err) {
			log.E.F("failed to kill sprocket process: %v", err)
		}
		<-done // Wait for the kill to complete
	}

	// Clean up pipes
	if sm.stdin != nil {
		sm.stdin.Close()
		sm.stdin = nil
	}
	if sm.stdout != nil {
		sm.stdout.Close()
		sm.stdout = nil
	}
	if sm.stderr != nil {
		sm.stderr.Close()
		sm.stderr = nil
	}

	sm.isRunning = false
	sm.currentCmd = nil
	sm.currentCancel = nil

	return nil
}

// RestartSprocket stops and starts the sprocket script
func (sm *SprocketManager) RestartSprocket() error {
	if sm.isRunning {
		if err := sm.StopSprocket(); chk.E(err) {
			return fmt.Errorf("failed to stop sprocket: %v", err)
		}
		// Give it a moment to fully stop
		time.Sleep(100 * time.Millisecond)
	}

	return sm.StartSprocket()
}

// UpdateSprocket updates the sprocket script and restarts it with zero downtime
func (sm *SprocketManager) UpdateSprocket(scriptContent string) error {
	// Ensure config directory exists
	if err := os.MkdirAll(sm.configDir, 0755); chk.E(err) {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// If script content is empty, delete the script and stop
	if strings.TrimSpace(scriptContent) == "" {
		if sm.isRunning {
			if err := sm.StopSprocket(); chk.E(err) {
				log.E.F("failed to stop sprocket before deletion: %v", err)
			}
		}

		if _, err := os.Stat(sm.scriptPath); err == nil {
			if err := os.Remove(sm.scriptPath); chk.E(err) {
				return fmt.Errorf("failed to delete sprocket script: %v", err)
			}
			log.I.F("sprocket script deleted")
		}
		return nil
	}

	// Create backup of existing script if it exists
	if _, err := os.Stat(sm.scriptPath); err == nil {
		timestamp := time.Now().Format("20060102150405")
		backupPath := sm.scriptPath + "." + timestamp
		if err := os.Rename(sm.scriptPath, backupPath); chk.E(err) {
			log.W.F("failed to create backup: %v", err)
		} else {
			log.I.F("created backup: %s", backupPath)
		}
	}

	// Write new script to temporary file first
	tempPath := sm.scriptPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(scriptContent), 0755); chk.E(err) {
		return fmt.Errorf("failed to write temporary sprocket script: %v", err)
	}

	// If sprocket is running, do zero-downtime update
	if sm.isRunning {
		// Atomically replace the script file
		if err := os.Rename(tempPath, sm.scriptPath); chk.E(err) {
			os.Remove(tempPath) // Clean up temp file
			return fmt.Errorf("failed to replace sprocket script: %v", err)
		}

		log.I.F("sprocket script updated atomically")

		// Restart the sprocket process
		return sm.RestartSprocket()
	} else {
		// Not running, just replace the file
		if err := os.Rename(tempPath, sm.scriptPath); chk.E(err) {
			os.Remove(tempPath) // Clean up temp file
			return fmt.Errorf("failed to replace sprocket script: %v", err)
		}

		log.I.F("sprocket script updated")
		return nil
	}
}

// GetSprocketStatus returns the current status of the sprocket
func (sm *SprocketManager) GetSprocketStatus() map[string]interface{} {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	status := map[string]interface{}{
		"is_running":    sm.isRunning,
		"script_exists": false,
		"script_path":   sm.scriptPath,
	}

	if _, err := os.Stat(sm.scriptPath); err == nil {
		status["script_exists"] = true

		// Get script content
		if content, err := os.ReadFile(sm.scriptPath); err == nil {
			status["script_content"] = string(content)
		}

		// Get file info
		if info, err := os.Stat(sm.scriptPath); err == nil {
			status["script_modified"] = info.ModTime()
		}
	}

	if sm.isRunning && sm.currentCmd != nil && sm.currentCmd.Process != nil {
		status["pid"] = sm.currentCmd.Process.Pid
	}

	return status
}

// GetSprocketVersions returns a list of all sprocket script versions
func (sm *SprocketManager) GetSprocketVersions() ([]map[string]interface{}, error) {
	versions := []map[string]interface{}{}

	// Check for current script
	if _, err := os.Stat(sm.scriptPath); err == nil {
		if info, err := os.Stat(sm.scriptPath); err == nil {
			if content, err := os.ReadFile(sm.scriptPath); err == nil {
				versions = append(versions, map[string]interface{}{
					"name":       "sprocket.sh",
					"path":       sm.scriptPath,
					"modified":   info.ModTime(),
					"content":    string(content),
					"is_current": true,
				})
			}
		}
	}

	// Check for backup versions
	dir := filepath.Dir(sm.scriptPath)
	files, err := os.ReadDir(dir)
	if chk.E(err) {
		return versions, nil
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "sprocket.sh.") && !file.IsDir() {
			path := filepath.Join(dir, file.Name())
			if info, err := os.Stat(path); err == nil {
				if content, err := os.ReadFile(path); err == nil {
					versions = append(versions, map[string]interface{}{
						"name":       file.Name(),
						"path":       path,
						"modified":   info.ModTime(),
						"content":    string(content),
						"is_current": false,
					})
				}
			}
		}
	}

	return versions, nil
}

// DeleteSprocketVersion deletes a specific sprocket version
func (sm *SprocketManager) DeleteSprocketVersion(filename string) error {
	// Don't allow deleting the current script
	if filename == "sprocket.sh" {
		return fmt.Errorf("cannot delete current sprocket script")
	}

	path := filepath.Join(sm.configDir, filename)
	if err := os.Remove(path); chk.E(err) {
		return fmt.Errorf("failed to delete sprocket version: %v", err)
	}

	log.I.F("deleted sprocket version: %s", filename)
	return nil
}

// logOutput logs the output from stdout and stderr
func (sm *SprocketManager) logOutput(stdout, stderr io.ReadCloser) {
	defer stdout.Close()
	defer stderr.Close()

	// Trace-log stdout lines
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			log.T.F("sprocket stdout: %s", line)
		}
	}()

	// Trace-log stderr lines
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			log.T.F("sprocket stderr: %s", line)
		}
	}()
}

// ProcessEvent sends an event to the sprocket script and waits for a response
func (sm *SprocketManager) ProcessEvent(evt *event.E) (*SprocketResponse, error) {
	sm.mutex.RLock()
	if !sm.isRunning || sm.stdin == nil {
		sm.mutex.RUnlock()
		return nil, fmt.Errorf("sprocket is not running")
	}
	stdin := sm.stdin
	sm.mutex.RUnlock()

	// Serialize the event to JSON
	eventJSON, err := json.Marshal(evt)
	if chk.E(err) {
		return nil, fmt.Errorf("failed to serialize event: %v", err)
	}

	// Send the event JSON to the sprocket script
	// The final ']' should be the only thing after the event's raw JSON
	if _, err := stdin.Write(eventJSON); chk.E(err) {
		return nil, fmt.Errorf("failed to write event to sprocket: %v", err)
	}

	// Wait for response with timeout
	select {
	case response := <-sm.responseChan:
		return &response, nil
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("sprocket response timeout")
	case <-sm.ctx.Done():
		return nil, fmt.Errorf("sprocket context cancelled")
	}
}

// readResponses reads JSONL responses from the sprocket script
func (sm *SprocketManager) readResponses() {
	if sm.stdout == nil {
		return
	}

	scanner := bufio.NewScanner(sm.stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var response SprocketResponse
		if err := json.Unmarshal([]byte(line), &response); chk.E(err) {
			log.E.F("failed to parse sprocket response: %v", err)
			continue
		}

		// Send response to channel (non-blocking)
		select {
		case sm.responseChan <- response:
		default:
			log.W.F("sprocket response channel full, dropping response")
		}
	}

	if err := scanner.Err(); chk.E(err) {
		log.E.F("error reading sprocket responses: %v", err)
	}
}

// IsEnabled returns whether sprocket is enabled
func (sm *SprocketManager) IsEnabled() bool {
	return sm.enabled
}

// IsRunning returns whether sprocket is currently running
func (sm *SprocketManager) IsRunning() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.isRunning
}

// IsDisabled returns whether sprocket is disabled due to failure
func (sm *SprocketManager) IsDisabled() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.disabled
}

// monitorProcess monitors the sprocket process and cleans up when it exits
func (sm *SprocketManager) monitorProcess() {
	if sm.currentCmd == nil {
		return
	}

	err := sm.currentCmd.Wait()

	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	// Clean up pipes
	if sm.stdin != nil {
		sm.stdin.Close()
		sm.stdin = nil
	}
	if sm.stdout != nil {
		sm.stdout.Close()
		sm.stdout = nil
	}
	if sm.stderr != nil {
		sm.stderr.Close()
		sm.stderr = nil
	}

	sm.isRunning = false
	sm.currentCmd = nil
	sm.currentCancel = nil

	if err != nil {
		log.E.F("sprocket process exited with error: %v", err)
		// Auto-disable sprocket on failure
		sm.disabled = true
		log.W.F("sprocket disabled due to process failure - all events will be rejected (script location: %s)", sm.scriptPath)
	} else {
		log.I.F("sprocket process exited normally")
	}
}

// Shutdown gracefully shuts down the sprocket manager
func (sm *SprocketManager) Shutdown() {
	sm.cancel()
	if sm.isRunning {
		sm.StopSprocket()
	}
}
