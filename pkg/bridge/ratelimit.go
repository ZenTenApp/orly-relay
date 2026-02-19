package bridge

import (
	"fmt"
	"sync"
	"time"
)

// RateLimitConfig holds rate limit configuration values.
type RateLimitConfig struct {
	PerUserPerHour  int           // Max emails per user per hour (default: 10)
	PerUserPerDay   int           // Max emails per user per day (default: 50)
	GlobalPerHour   int           // Max emails globally per hour (default: 100)
	GlobalPerDay    int           // Max emails globally per day (default: 500)
	MinInterval     time.Duration // Min time between sends per user (default: 30s)
}

// DefaultRateLimitConfig returns sensible defaults per the spec.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		PerUserPerHour: 10,
		PerUserPerDay:  50,
		GlobalPerHour:  100,
		GlobalPerDay:   500,
		MinInterval:    30 * time.Second,
	}
}

// RateLimiter tracks outbound email sending rates using sliding windows.
type RateLimiter struct {
	cfg    RateLimitConfig
	mu     sync.Mutex
	users  map[string]*userWindow
	global *window
}

// NewRateLimiter creates a rate limiter with the given config.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		cfg:    cfg,
		users:  make(map[string]*userWindow),
		global: newWindow(),
	}
}

// Check returns nil if the user is allowed to send, or an error describing
// when they can retry.
func (rl *RateLimiter) Check(pubkeyHex string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	uw := rl.getUser(pubkeyHex)

	// Check minimum interval
	if rl.cfg.MinInterval > 0 && !uw.lastSend.IsZero() {
		elapsed := now.Sub(uw.lastSend)
		if elapsed < rl.cfg.MinInterval {
			wait := rl.cfg.MinInterval - elapsed
			return fmt.Errorf("rate limited: wait %v between sends", wait.Round(time.Second))
		}
	}

	// Check per-user per-hour
	if rl.cfg.PerUserPerHour > 0 {
		count := uw.hourly.countSince(now.Add(-time.Hour))
		if count >= rl.cfg.PerUserPerHour {
			return fmt.Errorf("rate limited: %d emails per hour limit reached", rl.cfg.PerUserPerHour)
		}
	}

	// Check per-user per-day
	if rl.cfg.PerUserPerDay > 0 {
		count := uw.daily.countSince(now.Add(-24 * time.Hour))
		if count >= rl.cfg.PerUserPerDay {
			return fmt.Errorf("rate limited: %d emails per day limit reached", rl.cfg.PerUserPerDay)
		}
	}

	// Check global per-hour
	if rl.cfg.GlobalPerHour > 0 {
		count := rl.global.countSince(now.Add(-time.Hour))
		if count >= rl.cfg.GlobalPerHour {
			return fmt.Errorf("rate limited: global hourly limit (%d) reached", rl.cfg.GlobalPerHour)
		}
	}

	// Check global per-day
	if rl.cfg.GlobalPerDay > 0 {
		count := rl.global.countSince(now.Add(-24 * time.Hour))
		if count >= rl.cfg.GlobalPerDay {
			return fmt.Errorf("rate limited: global daily limit (%d) reached", rl.cfg.GlobalPerDay)
		}
	}

	return nil
}

// Record records a send event for rate limiting purposes.
// Call this after a successful send.
func (rl *RateLimiter) Record(pubkeyHex string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	uw := rl.getUser(pubkeyHex)
	uw.lastSend = now
	uw.hourly.add(now)
	uw.daily.add(now)

	rl.global.add(now)
}

func (rl *RateLimiter) getUser(pubkeyHex string) *userWindow {
	uw, ok := rl.users[pubkeyHex]
	if !ok {
		uw = &userWindow{
			hourly: newWindow(),
			daily:  newWindow(),
		}
		rl.users[pubkeyHex] = uw
	}
	return uw
}

// userWindow tracks per-user rate limiting state.
type userWindow struct {
	lastSend time.Time
	hourly   *window
	daily    *window
}

// window is a sliding window of timestamps for counting events.
type window struct {
	times []time.Time
}

func newWindow() *window {
	return &window{}
}

// add records a new event timestamp.
func (w *window) add(t time.Time) {
	w.times = append(w.times, t)
}

// countSince returns the number of events since the given time,
// pruning old entries as a side effect.
func (w *window) countSince(since time.Time) int {
	// Prune old entries
	n := 0
	for _, t := range w.times {
		if !t.Before(since) {
			w.times[n] = t
			n++
		}
	}
	w.times = w.times[:n]
	return n
}
