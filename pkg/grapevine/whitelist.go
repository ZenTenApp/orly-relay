package grapevine

import (
	"context"
	"encoding/hex"
	"time"

	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/bech32encoding"
)

// WhitelistUpdater manages automatic whitelist updates based on GrapeVine scores.
type WhitelistUpdater struct {
	engine            *Engine
	influenceThreshold float64
	updateInterval    time.Duration
	onUpdate          func(pubkeys []string) error
	ctx               context.Context
	cancel            context.CancelFunc
	running           bool
}

// WhitelistConfig configures the whitelist updater.
type WhitelistConfig struct {
	// InfluenceThreshold is the minimum influence score for whitelist inclusion (default: 0.5)
	InfluenceThreshold float64
	// UpdateInterval is how often to recalculate and update the whitelist (default: 6h)
	UpdateInterval time.Duration
	// OnUpdate is called with the new whitelist pubkeys when an update occurs
	OnUpdate func(pubkeys []string) error
}

// NewWhitelistUpdater creates a new whitelist updater.
func NewWhitelistUpdater(ctx context.Context, engine *Engine, cfg WhitelistConfig) *WhitelistUpdater {
	if cfg.InfluenceThreshold <= 0 {
		cfg.InfluenceThreshold = 0.5
	}
	if cfg.UpdateInterval <= 0 {
		cfg.UpdateInterval = 6 * time.Hour
	}

	ctx, cancel := context.WithCancel(ctx)
	return &WhitelistUpdater{
		engine:            engine,
		influenceThreshold: cfg.InfluenceThreshold,
		updateInterval:    cfg.UpdateInterval,
		onUpdate:          cfg.OnUpdate,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// Start begins the automatic whitelist update loop.
func (w *WhitelistUpdater) Start(observerHex string) error {
	if w.running {
		return nil
	}
	w.running = true

	go w.updateLoop(observerHex)
	log.I.F("grapevine whitelist updater: started (threshold: %.2f, interval: %v)",
		w.influenceThreshold, w.updateInterval)
	return nil
}

// Stop stops the whitelist updater.
func (w *WhitelistUpdater) Stop() {
	if !w.running {
		return
	}
	w.running = false
	w.cancel()
	log.I.F("grapevine whitelist updater: stopped")
}

// updateLoop runs the periodic whitelist update.
func (w *WhitelistUpdater) updateLoop(observerHex string) {
	// Run immediately on start
	w.updateOnce(observerHex)

	ticker := time.NewTicker(w.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.updateOnce(observerHex)
		}
	}
}

// updateOnce performs a single whitelist update.
func (w *WhitelistUpdater) updateOnce(observerHex string) {
	start := time.Now()
	log.I.F("grapevine whitelist updater: starting update for observer %s", observerHex[:12])

	// Compute fresh GrapeVine scores
	scoreSet, err := w.engine.Compute(observerHex)
	if err != nil {
		log.E.F("grapevine whitelist updater: failed to compute scores: %v", err)
		return
	}

	// Filter pubkeys by influence threshold
	var whitelist []string
	for _, score := range scoreSet.Scores {
		if score.Influence >= w.influenceThreshold {
			whitelist = append(whitelist, score.PubkeyHex)
		}
	}

	log.I.F("grapevine whitelist updater: computed %d whitelisted pubkeys (threshold: %.2f, total: %d)",
		len(whitelist), w.influenceThreshold, len(scoreSet.Scores))

	// Call the update callback
	if w.onUpdate != nil {
		if err := w.onUpdate(whitelist); err != nil {
			log.E.F("grapevine whitelist updater: failed to apply whitelist update: %v", err)
			return
		}
	}

	log.I.F("grapevine whitelist updater: completed update in %v", time.Since(start))
}

// TriggerNow forces an immediate whitelist update.
func (w *WhitelistUpdater) TriggerNow(observerHex string) {
	go w.updateOnce(observerHex)
}

// GetWhitelist returns the current whitelist for an observer without updating.
func (w *WhitelistUpdater) GetWhitelist(observerHex string) ([]string, error) {
	scoreSet, err := w.engine.GetScores(observerHex)
	if err != nil {
		return nil, err
	}
	if scoreSet == nil {
		return nil, nil
	}

	var whitelist []string
	for _, score := range scoreSet.Scores {
		if score.Influence >= w.influenceThreshold {
			whitelist = append(whitelist, score.PubkeyHex)
		}
	}
	return whitelist, nil
}

// PubkeysToHexStrings converts binary pubkeys to hex strings.
func PubkeysToHexStrings(pubkeys [][]byte) []string {
	result := make([]string, 0, len(pubkeys))
	for _, pk := range pubkeys {
		result = append(result, hex.EncodeToString(pk))
	}
	return result
}

// HexStringsToPubkeys converts hex strings to binary pubkeys.
func HexStringsToPubkeys(hexKeys []string) ([][]byte, error) {
	result := make([][]byte, 0, len(hexKeys))
	for _, h := range hexKeys {
		pk, err := hex.DecodeString(h)
		if err != nil {
			// Try npub format
			pk, err = bech32encoding.NpubOrHexToPublicKeyBinary(h)
			if err != nil {
				continue
			}
		}
		if len(pk) == 32 {
			result = append(result, pk)
		}
	}
	return result, nil
}
