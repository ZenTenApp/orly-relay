package run

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/chk"
	lol "git.smesh.lol/orly/pkg/lol"
	"git.smesh.lol/orly/app"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/ratelimit"
)

// Options configures relay startup behavior.
type Options struct {
	// CleanupDataDir controls whether the data directory is deleted on Stop().
	// Defaults to true. Set to false to preserve the data directory.
	CleanupDataDir *bool

	// StdoutWriter is an optional writer to receive stdout logs.
	// If nil, stdout will be captured to a buffer accessible via Relay.Stdout().
	StdoutWriter io.Writer

	// StderrWriter is an optional writer to receive stderr logs.
	// If nil, stderr will be captured to a buffer accessible via Relay.Stderr().
	StderrWriter io.Writer
}

// Relay represents a running relay instance that can be started and stopped.
type Relay struct {
	ctx            context.Context
	cancel         context.CancelFunc
	db             *database.D
	quit           chan struct{}
	dataDir        string
	cleanupDataDir bool

	// Log capture
	stdoutBuf    *bytes.Buffer
	stderrBuf    *bytes.Buffer
	stdoutWriter io.Writer
	stderrWriter io.Writer

	stdoutStr   actor.Query[string]
	stderrStr   actor.Query[string]
	stdoutBytes actor.Query[[]byte]
	stderrBytes actor.Query[[]byte]
	logLC       actor.Lifecycle
}

// Start initializes and starts a relay with the given configuration.
// It bypasses the configuration loading step and uses the provided config directly.
//
// Parameters:
//   - cfg: The configuration to use for the relay
//   - opts: Optional configuration for relay behavior. If nil, defaults are used.
//
// Returns:
//   - relay: A Relay instance that can be used to stop the relay
//   - err: An error if initialization or startup fails
func Start(cfg *config.C, opts *Options) (relay *Relay, err error) {
	relay = &Relay{
		cleanupDataDir: true,
		stdoutStr:      actor.NewQuery[string](),
		stderrStr:      actor.NewQuery[string](),
		stdoutBytes:    actor.NewQuery[[]byte](),
		stderrBytes:    actor.NewQuery[[]byte](),
		logLC:          actor.NewLifecycle(),
	}

	// Apply options
	var userStdoutWriter, userStderrWriter io.Writer
	if opts != nil {
		if opts.CleanupDataDir != nil {
			relay.cleanupDataDir = *opts.CleanupDataDir
		}
		userStdoutWriter = opts.StdoutWriter
		userStderrWriter = opts.StderrWriter
	}

	// Set up log capture buffers
	relay.stdoutBuf = &bytes.Buffer{}
	relay.stderrBuf = &bytes.Buffer{}

	// Build writers list for stdout
	stdoutWriters := []io.Writer{relay.stdoutBuf}
	if userStdoutWriter != nil {
		stdoutWriters = append(stdoutWriters, userStdoutWriter)
	}
	stdoutWriters = append(stdoutWriters, os.Stdout)
	relay.stdoutWriter = io.MultiWriter(stdoutWriters...)

	// Build writers list for stderr
	stderrWriters := []io.Writer{relay.stderrBuf}
	if userStderrWriter != nil {
		stderrWriters = append(stderrWriters, userStderrWriter)
	}
	stderrWriters = append(stderrWriters, os.Stderr)
	relay.stderrWriter = io.MultiWriter(stderrWriters...)

	// Set up logging - write to appropriate destination and capture
	if cfg.LogToStdout {
		lol.Writer = relay.stdoutWriter
	} else {
		lol.Writer = relay.stderrWriter
	}
	lol.SetLogLevel(cfg.LogLevel)

	// Expand DataDir if needed
	if cfg.DataDir == "" || strings.Contains(cfg.DataDir, "~") {
		cfg.DataDir = filepath.Join(xdg.DataHome, cfg.AppName)
	}
	relay.dataDir = cfg.DataDir

	// Create context
	relay.ctx, relay.cancel = context.WithCancel(context.Background())

	// Initialize database
	if relay.db, err = database.New(
		relay.ctx, relay.cancel, cfg.DataDir, cfg.DBLogLevel,
	); chk.E(err) {
		return
	}

	// Configure ACL
	acl.Registry.SetMode(cfg.ACLMode)
	if err = acl.Registry.Configure(cfg, relay.db, relay.ctx); chk.E(err) {
		return
	}
	acl.Registry.Syncer()

	// Create rate limiter (disabled for test relay instances)
	limiter := ratelimit.NewDisabledLimiter()

	// Start the relay
	relay.quit = app.Run(relay.ctx, cfg, relay.db, limiter)

	// Start log reader actor
	actor.Go(relay.logLC, relay.logActorLoop)

	return
}

// logActorLoop owns the log buffers and services read requests.
func (r *Relay) logActorLoop() {
	for {
		select {
		case <-r.logLC.Stopping():
			return
		case msg := <-r.stdoutStr.Recv():
			s := ""
			if r.stdoutBuf != nil {
				s = r.stdoutBuf.String()
			}
			msg.Reply(s)
		case msg := <-r.stderrStr.Recv():
			s := ""
			if r.stderrBuf != nil {
				s = r.stderrBuf.String()
			}
			msg.Reply(s)
		case msg := <-r.stdoutBytes.Recv():
			var b []byte
			if r.stdoutBuf != nil {
				b = r.stdoutBuf.Bytes()
			}
			msg.Reply(b)
		case msg := <-r.stderrBytes.Recv():
			var b []byte
			if r.stderrBuf != nil {
				b = r.stderrBuf.Bytes()
			}
			msg.Reply(b)
		}
	}
}

// Stop gracefully stops the relay by canceling the context and closing the database.
// If CleanupDataDir is enabled (default), it also removes the data directory.
//
// Returns:
//   - err: An error if shutdown fails
func (r *Relay) Stop() (err error) {
	if r.cancel != nil {
		r.cancel()
	}
	if r.quit != nil {
		<-r.quit
	}
	if r.db != nil {
		err = r.db.Close()
	}
	// Stop log actor
	r.logLC.Stop()
	// Clean up data directory if enabled
	if r.cleanupDataDir && r.dataDir != "" {
		if rmErr := os.RemoveAll(r.dataDir); rmErr != nil {
			if err == nil {
				err = rmErr
			}
		}
	}
	return
}

// Stdout returns the complete stdout log buffer contents.
func (r *Relay) Stdout() string {
	return r.stdoutStr.Call()
}

// Stderr returns the complete stderr log buffer contents.
func (r *Relay) Stderr() string {
	return r.stderrStr.Call()
}

// StdoutBytes returns the complete stdout log buffer as bytes.
func (r *Relay) StdoutBytes() []byte {
	return r.stdoutBytes.Call()
}

// StderrBytes returns the complete stderr log buffer as bytes.
func (r *Relay) StderrBytes() []byte {
	return r.stderrBytes.Call()
}
