//go:build !(js && wasm)

package bbolt

import (
	"context"
	"time"

	"next.orly.dev/pkg/database"
)

func init() {
	database.RegisterBboltFactory(newBboltFromConfig)
}

// newBboltFromConfig creates a BBolt database from DatabaseConfig
func newBboltFromConfig(
	ctx context.Context,
	cancel context.CancelFunc,
	cfg *database.DatabaseConfig,
) (database.Database, error) {
	// Convert DatabaseConfig to BboltConfig
	bboltCfg := &BboltConfig{
		DataDir:  cfg.DataDir,
		LogLevel: cfg.LogLevel,

		// Use bbolt-specific settings from DatabaseConfig if present
		// These will be added to DatabaseConfig later
		BatchMaxEvents:    cfg.BboltBatchMaxEvents,
		BatchMaxBytes:     cfg.BboltBatchMaxBytes,
		BatchFlushTimeout: cfg.BboltFlushTimeout,
		BloomSizeMB:       cfg.BboltBloomSizeMB,
		NoSync:            cfg.BboltNoSync,
		InitialMmapSize:   cfg.BboltMmapSize,
	}

	// Apply defaults if not set
	if bboltCfg.BatchMaxEvents <= 0 {
		bboltCfg.BatchMaxEvents = 5000
	}
	if bboltCfg.BatchMaxBytes <= 0 {
		bboltCfg.BatchMaxBytes = 128 * 1024 * 1024 // 128MB
	}
	if bboltCfg.BatchFlushTimeout <= 0 {
		bboltCfg.BatchFlushTimeout = 30 * time.Second
	}
	if bboltCfg.BloomSizeMB <= 0 {
		bboltCfg.BloomSizeMB = 16
	}
	if bboltCfg.InitialMmapSize <= 0 {
		bboltCfg.InitialMmapSize = 8 * 1024 * 1024 * 1024 // 8GB
	}

	return NewWithConfig(ctx, cancel, bboltCfg)
}
