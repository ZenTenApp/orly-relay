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
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"github.com/adrg/xdg"
)

// SprocketResponse represents a response from the sprocket script
type SprocketResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"` // accept, reject, or shadowReject
	Msg    string `json:"msg"`    // NIP-20 response message (only used for reject)
}

type sprocketStatusResp struct {
	isRunning bool
	disabled  bool
	stdin     io.WriteCloser
}

type sprocketSetStateReq struct {
	isRunning     *bool
	disabled      *bool
	cmd           *exec.Cmd
	cmdCancel     context.CancelFunc
	stdin         io.WriteCloser
	stdout        io.ReadCloser
	stderr        io.ReadCloser
	clearPipes    bool
}

// SprocketManager handles sprocket script execution and management
type SprocketManager struct {
	ctx           context.Context
	cancel        context.CancelFunc
	configDir     string
	scriptPath    string
	enabled       bool

	getStatus     actor.Query[sprocketStatusResp]
	setState      actor.Inbox[sprocketSetStateReq]
	getFullStatus actor.Query[map[string]interface{}]
	lc            actor.Lifecycle

	responseChan  chan SprocketResponse
}

// NewSprocketManager creates a new sprocket manager
func NewSprocketManager(ctx context.Context, appName string, enabled bool) *SprocketManager {
	configDir := filepath.Join(xdg.ConfigHome, appName)
	scriptPath := filepath.Join(configDir, "sprocket.sh")

	ctx, cancel := context.WithCancel(ctx)

	sm := &SprocketManager{
		ctx:           ctx,
		cancel:        cancel,
		configDir:     configDir,
		scriptPath:    scriptPath,
		enabled:       enabled,
		getStatus:     actor.NewQuery[sprocketStatusResp](),
		setState:      actor.NewInbox[sprocketSetStateReq](4),
		getFullStatus: actor.NewQuery[map[string]interface{}](),
		lc:            actor.NewLifecycle(),
		responseChan:  make(chan SprocketResponse, 100),
	}

	actor.Go(sm.lc, sm.actorLoop)

	// Start the sprocket script if it exists and is enabled
	if enabled {
		go sm.startSprocketIfExists()
		go sm.periodicCheck()
	}

	return sm
}

func (sm *SprocketManager) actorLoop() {
	var (
		isRunning     bool
		disabled      bool
		currentCmd    *exec.Cmd
		currentCancel context.CancelFunc
		stdin         io.WriteCloser
		stdout        io.ReadCloser
		stderr        io.ReadCloser
	)

	for {
		select {
		case msg := <-sm.getStatus.Recv():
			msg.Reply(sprocketStatusResp{
				isRunning: isRunning,
				disabled:  disabled,
				stdin:     stdin,
			})
		case req := <-sm.setState.Recv():
			if req.isRunning != nil {
				isRunning = *req.isRunning
			}
			if req.disabled != nil {
				disabled = *req.disabled
			}
			if req.cmd != nil {
				currentCmd = req.cmd
			}
			if req.cmdCancel != nil {
				currentCancel = req.cmdCancel
			}
			if req.stdin != nil {
				stdin = req.stdin
			}
			if req.stdout != nil {
				stdout = req.stdout
			}
			if req.stderr != nil {
				stderr = req.stderr
			}
			if req.clearPipes {
				if stdin != nil {
					stdin.Close()
				}
				stdin = nil
				if stdout != nil {
					stdout.Close()
				}
				stdout = nil
				if stderr != nil {
					stderr.Close()
				}
				stderr = nil
				isRunning = false
				currentCmd = nil
				currentCancel = nil
			}
		case msg := <-sm.getFullStatus.Recv():
			status := map[string]interface{}{
				"is_running":    isRunning,
				"script_exists": false,
				"script_path":   sm.scriptPath,
			}
			if _, err := os.Stat(sm.scriptPath); err == nil {
				status["script_exists"] = true
				if content, err := os.ReadFile(sm.scriptPath); err == nil {
					status["script_content"] = string(content)
				}
				if info, err := os.Stat(sm.scriptPath); err == nil {
					status["script_modified"] = info.ModTime()
				}
			}
			if isRunning && currentCmd != nil && currentCmd.Process != nil {
				status["pid"] = currentCmd.Process.Pid
			}
			msg.Reply(status)
		case <-sm.lc.Stopping():
			// Cleanup on shutdown
			if stdin != nil {
				stdin.Close()
			}
			if currentCancel != nil {
				currentCancel()
			}
			_ = currentCmd
			_ = stdout
			_ = stderr
			return
		case <-sm.ctx.Done():
			// Cleanup on shutdown
			if stdin != nil {
				stdin.Close()
			}
			if currentCancel != nil {
				currentCancel()
			}
			_ = currentCmd
			_ = stdout
			_ = stderr
			return
		}
	}
}

func (sm *SprocketManager) getState() sprocketStatusResp {
	return sm.getStatus.Call()
}

func boolPtr(b bool) *bool { return &b }

// disableSprocket disables sprocket due to failure
func (sm *SprocketManager) disableSprocket() {
	state := sm.getState()
	if !state.disabled {
		sm.setState.TrySend(sprocketSetStateReq{disabled: boolPtr(true)})
		log.W.F("sprocket disabled due to failure - all events will be rejected (script location: %s)", sm.scriptPath)
	}
}

// enableSprocket re-enables sprocket and attempts to start it
func (sm *SprocketManager) enableSprocket() {
	state := sm.getState()
	if state.disabled {
		sm.setState.TrySend(sprocketSetStateReq{disabled: boolPtr(false)})
		log.I.F("sprocket re-enabled, attempting to start")

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
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			state := sm.getState()
			if state.disabled || !state.isRunning {
				if _, err := os.Stat(sm.scriptPath); err == nil {
					if state.disabled {
						sm.enableSprocket()
					} else if !state.isRunning {
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
	state := sm.getState()
	if state.isRunning {
		return fmt.Errorf("sprocket is already running")
	}

	if _, err := os.Stat(sm.scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("sprocket script does not exist")
	}

	cmdCtx, cmdCancel := context.WithCancel(sm.ctx)

	if err := os.Chmod(sm.scriptPath, 0755); chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to make script executable: %v", err)
	}

	cmd := exec.CommandContext(cmdCtx, sm.scriptPath)
	cmd.Dir = sm.configDir

	stdinPipe, err := cmd.StdinPipe()
	if chk.E(err) {
		cmdCancel()
		return fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if chk.E(err) {
		cmdCancel()
		stdinPipe.Close()
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if chk.E(err) {
		cmdCancel()
		stdinPipe.Close()
		stdoutPipe.Close()
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); chk.E(err) {
		cmdCancel()
		stdinPipe.Close()
		stdoutPipe.Close()
		stderrPipe.Close()
		return fmt.Errorf("failed to start sprocket: %v", err)
	}

	sm.setState.TrySend(sprocketSetStateReq{
		isRunning: boolPtr(true),
		cmd:       cmd,
		cmdCancel: cmdCancel,
		stdin:     stdinPipe,
		stdout:    stdoutPipe,
		stderr:    stderrPipe,
	})

	go sm.readResponses(stdoutPipe)
	go sm.logOutput(stdoutPipe, stderrPipe)
	go sm.monitorProcess(cmd)

	log.I.F("sprocket started (pid=%d)", cmd.Process.Pid)
	return nil
}

// StopSprocket stops the sprocket script gracefully, with SIGKILL fallback
func (sm *SprocketManager) StopSprocket() error {
	state := sm.getState()
	if !state.isRunning {
		return fmt.Errorf("sprocket is not running")
	}

	// Close stdin to signal script to exit, then cancel context
	if state.stdin != nil {
		state.stdin.Close()
	}

	// The monitorProcess goroutine will handle cleanup when the process exits
	// We need to set state to clear pipes
	sm.setState.TrySend(sprocketSetStateReq{clearPipes: true})

	log.I.F("sprocket stopped")
	return nil
}

// RestartSprocket stops and starts the sprocket script
func (sm *SprocketManager) RestartSprocket() error {
	state := sm.getState()
	if state.isRunning {
		if err := sm.StopSprocket(); chk.E(err) {
			return fmt.Errorf("failed to stop sprocket: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return sm.StartSprocket()
}

// UpdateSprocket updates the sprocket script and restarts it with zero downtime
func (sm *SprocketManager) UpdateSprocket(scriptContent string) error {
	if err := os.MkdirAll(sm.configDir, 0755); chk.E(err) {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	if strings.TrimSpace(scriptContent) == "" {
		state := sm.getState()
		if state.isRunning {
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

	if _, err := os.Stat(sm.scriptPath); err == nil {
		timestamp := time.Now().Format("20060102150405")
		backupPath := sm.scriptPath + "." + timestamp
		if err := os.Rename(sm.scriptPath, backupPath); chk.E(err) {
			log.W.F("failed to create backup: %v", err)
		} else {
			log.I.F("created backup: %s", backupPath)
		}
	}

	tempPath := sm.scriptPath + ".tmp"
	if err := os.WriteFile(tempPath, []byte(scriptContent), 0755); chk.E(err) {
		return fmt.Errorf("failed to write temporary sprocket script: %v", err)
	}

	state := sm.getState()
	if state.isRunning {
		if err := os.Rename(tempPath, sm.scriptPath); chk.E(err) {
			os.Remove(tempPath)
			return fmt.Errorf("failed to replace sprocket script: %v", err)
		}
		log.I.F("sprocket script updated atomically")
		return sm.RestartSprocket()
	}

	if err := os.Rename(tempPath, sm.scriptPath); chk.E(err) {
		os.Remove(tempPath)
		return fmt.Errorf("failed to replace sprocket script: %v", err)
	}
	log.I.F("sprocket script updated")
	return nil
}

// GetSprocketStatus returns the current status of the sprocket
func (sm *SprocketManager) GetSprocketStatus() map[string]interface{} {
	return sm.getFullStatus.Call()
}

// GetSprocketVersions returns a list of all sprocket script versions
func (sm *SprocketManager) GetSprocketVersions() ([]map[string]interface{}, error) {
	versions := []map[string]interface{}{}

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
	state := sm.getState()
	if !state.isRunning || state.stdin == nil {
		return nil, fmt.Errorf("sprocket is not running")
	}

	eventJSON, err := json.Marshal(evt)
	if chk.E(err) {
		return nil, fmt.Errorf("failed to serialize event: %v", err)
	}

	if _, err := state.stdin.Write(eventJSON); chk.E(err) {
		return nil, fmt.Errorf("failed to write event to sprocket: %v", err)
	}

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
func (sm *SprocketManager) readResponses(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
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
	return sm.getState().isRunning
}

// IsDisabled returns whether sprocket is disabled due to failure
func (sm *SprocketManager) IsDisabled() bool {
	return sm.getState().disabled
}

// monitorProcess monitors the sprocket process and cleans up when it exits
func (sm *SprocketManager) monitorProcess(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}

	err := cmd.Wait()

	sm.setState.TrySend(sprocketSetStateReq{clearPipes: true})

	if err != nil {
		log.E.F("sprocket process exited with error: %v", err)
		sm.setState.TrySend(sprocketSetStateReq{disabled: boolPtr(true)})
		log.W.F("sprocket disabled due to process failure - all events will be rejected (script location: %s)", sm.scriptPath)
	} else {
		log.I.F("sprocket process exited normally")
	}
}

// Shutdown gracefully shuts down the sprocket manager
func (sm *SprocketManager) Shutdown() {
	sm.cancel()
	state := sm.getState()
	if state.isRunning {
		sm.StopSprocket()
	}
	sm.lc.Stop()
}
