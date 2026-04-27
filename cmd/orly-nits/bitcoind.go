package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Bitcoind manages a bitcoind child process and provides a JSON-RPC client.
type Bitcoind struct {
	cfg  *Config
	cmd  *exec.Cmd
	mu   sync.Mutex
	done chan struct{}

	// cached state from last getblockchaininfo call
	lastInfo    atomic.Pointer[BlockchainInfo]
	responsive  atomic.Bool
}

// BlockchainInfo holds the result of getblockchaininfo.
type BlockchainInfo struct {
	Chain                string  `json:"chain"`
	Blocks               int64   `json:"blocks"`
	Headers              int64   `json:"headers"`
	VerificationProgress float64 `json:"verificationprogress"`
	InitialBlockDownload bool    `json:"initialblockdownload"`
	Pruned               bool    `json:"pruned"`
	PruneHeight          int64   `json:"pruneheight"`
	SizeOnDisk           int64   `json:"size_on_disk"`
}

// EstimateFeeResult holds the result of estimatesmartfee.
type EstimateFeeResult struct {
	FeeRate float64  `json:"feerate"`
	Errors  []string `json:"errors"`
	Blocks  int      `json:"blocks"`
}

// rpcResponse is the JSON-RPC response envelope.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	ID     int             `json:"id"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewBitcoind(cfg *Config) *Bitcoind {
	return &Bitcoind{
		cfg:  cfg,
		done: make(chan struct{}),
	}
}

// Start launches the bitcoind process and begins polling for readiness.
func (b *Bitcoind) Start(ctx context.Context) error {
	args := b.buildArgs()
	log.I.F("starting bitcoind: %s %s", b.cfg.Binary, strings.Join(args, " "))

	b.cmd = exec.CommandContext(ctx, b.cfg.Binary, args...)
	b.cmd.Stdout = os.Stdout
	b.cmd.Stderr = os.Stderr

	if err := b.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start bitcoind: %w", err)
	}

	log.I.F("bitcoind started with PID %d", b.cmd.Process.Pid)

	// Monitor process exit
	go func() {
		err := b.cmd.Wait()
		if err != nil {
			log.W.F("bitcoind exited: %v", err)
		} else {
			log.I.F("bitcoind exited cleanly")
		}
		b.responsive.Store(false)
		close(b.done)
	}()

	// Poll for RPC readiness
	go b.pollLoop(ctx)

	return nil
}

// Stop sends SIGINT to bitcoind and waits for clean shutdown.
func (b *Bitcoind) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cmd == nil || b.cmd.Process == nil {
		return
	}

	log.I.F("stopping bitcoind (PID %d)", b.cmd.Process.Pid)

	// Send SIGINT for graceful shutdown
	if err := b.cmd.Process.Signal(os.Interrupt); err != nil {
		log.W.F("failed to send SIGINT to bitcoind: %v", err)
		return
	}

	// Wait up to 30 seconds for clean exit
	select {
	case <-b.done:
		log.I.F("bitcoind stopped")
	case <-time.After(30 * time.Second):
		log.W.F("bitcoind did not stop in 30s, sending SIGKILL")
		b.cmd.Process.Kill()
		<-b.done
	}
}

// IsResponsive returns whether bitcoind is answering RPC calls.
func (b *Bitcoind) IsResponsive() bool {
	return b.responsive.Load()
}

// IsSynced returns whether bitcoind has finished initial block download.
func (b *Bitcoind) IsSynced() bool {
	info := b.lastInfo.Load()
	return info != nil && !info.InitialBlockDownload
}

// GetBlockchainInfo returns the last cached blockchain info.
func (b *Bitcoind) GetBlockchainInfo() *BlockchainInfo {
	return b.lastInfo.Load()
}

// EstimateFee calls estimatesmartfee on bitcoind.
func (b *Bitcoind) EstimateFee(confTarget int) (*EstimateFeeResult, error) {
	raw, err := b.rpcCall("estimatesmartfee", []interface{}{confTarget})
	if err != nil {
		return nil, err
	}
	var result EstimateFeeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("failed to parse estimatesmartfee response: %w", err)
	}
	return &result, nil
}

func (b *Bitcoind) buildArgs() []string {
	args := []string{
		fmt.Sprintf("-datadir=%s", b.cfg.DataDir),
		fmt.Sprintf("-rpcport=%d", b.cfg.RPCPort),
		"-server=1",
		"-rpcallowip=127.0.0.1",
		"-rpcbind=127.0.0.1",
	}

	if b.cfg.PruneSize > 0 {
		args = append(args, fmt.Sprintf("-prune=%d", b.cfg.PruneSize))
	}

	switch b.cfg.Network {
	case "testnet":
		args = append(args, "-testnet")
	case "signet":
		args = append(args, "-signet")
	case "regtest":
		args = append(args, "-regtest")
	}

	if b.cfg.RPCUser != "" {
		args = append(args, fmt.Sprintf("-rpcuser=%s", b.cfg.RPCUser))
	}
	if b.cfg.RPCPass != "" {
		args = append(args, fmt.Sprintf("-rpcpassword=%s", b.cfg.RPCPass))
	}

	return args
}

// pollLoop continuously polls bitcoind for status.
func (b *Bitcoind) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.done:
			return
		case <-ticker.C:
			b.updateStatus()
		}
	}
}

func (b *Bitcoind) updateStatus() {
	raw, err := b.rpcCall("getblockchaininfo", nil)
	if err != nil {
		if b.responsive.Load() {
			log.W.F("bitcoind became unresponsive: %v", err)
		}
		b.responsive.Store(false)
		return
	}

	var info BlockchainInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		log.W.F("failed to parse getblockchaininfo: %v", err)
		return
	}

	wasResponsive := b.responsive.Load()
	b.responsive.Store(true)
	b.lastInfo.Store(&info)

	if !wasResponsive {
		log.I.F("bitcoind is responsive (chain=%s blocks=%d headers=%d progress=%.4f)",
			info.Chain, info.Blocks, info.Headers, info.VerificationProgress)
	}
}

// rpcCall makes a JSON-RPC call to bitcoind.
func (b *Bitcoind) rpcCall(method string, params interface{}) (json.RawMessage, error) {
	if params == nil {
		params = []interface{}{}
	}

	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RPC request: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/", b.cfg.RPCPort)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Use cookie auth if no explicit credentials
	if b.cfg.RPCUser != "" {
		req.SetBasicAuth(b.cfg.RPCUser, b.cfg.RPCPass)
	} else {
		user, pass, err := b.readCookieAuth()
		if err != nil {
			return nil, fmt.Errorf("no RPC credentials and cookie auth failed: %w", err)
		}
		req.SetBasicAuth(user, pass)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read RPC response: %w", err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	return rpcResp.Result, nil
}

// readCookieAuth reads the .cookie file from the bitcoind data directory.
func (b *Bitcoind) readCookieAuth() (user, pass string, err error) {
	cookiePath := filepath.Join(b.cfg.DataDir, ".cookie")

	// Testnet/signet/regtest use subdirectories
	switch b.cfg.Network {
	case "testnet":
		cookiePath = filepath.Join(b.cfg.DataDir, "testnet3", ".cookie")
	case "signet":
		cookiePath = filepath.Join(b.cfg.DataDir, "signet", ".cookie")
	case "regtest":
		cookiePath = filepath.Join(b.cfg.DataDir, "regtest", ".cookie")
	}

	data, err := os.ReadFile(cookiePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to read cookie file %s: %w", cookiePath, err)
	}

	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid cookie file format")
	}

	_ = chk.E // suppress unused import if linter complains
	return parts[0], parts[1], nil
}
