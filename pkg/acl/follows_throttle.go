package acl

import (
	"sync"
	"time"
)

// ThrottleState tracks accumulated delay for an identity (IP or pubkey)
type ThrottleState struct {
	AccumulatedDelay time.Duration
	LastEventTime    time.Time
}

// ProgressiveThrottle implements linear delay with time decay.
// Each event adds perEvent delay, and delay decays at 1:1 ratio with elapsed time.
// This creates a natural rate limit that averages to 1 event per perEvent interval.
type ProgressiveThrottle struct {
	mu           sync.Mutex
	ipStates     map[string]*ThrottleState
	pubkeyStates map[string]*ThrottleState
	perEvent     time.Duration // delay increment per event (default 200ms)
	maxDelay     time.Duration // cap (default 60s)
}

// NewProgressiveThrottle creates a new throttle with the given parameters.
// perEvent is the delay added per event (e.g., 200ms).
// maxDelay is the maximum accumulated delay cap (e.g., 60s).
func NewProgressiveThrottle(perEvent, maxDelay time.Duration) *ProgressiveThrottle {
	return &ProgressiveThrottle{
		ipStates:     make(map[string]*ThrottleState),
		pubkeyStates: make(map[string]*ThrottleState),
		perEvent:     perEvent,
		maxDelay:     maxDelay,
	}
}

// GetDelay returns accumulated delay for this identity and updates state.
// It tracks both IP and pubkey independently and returns the maximum of both.
// This prevents evasion via different pubkeys from same IP or vice versa.
func (pt *ProgressiveThrottle) GetDelay(ip, pubkeyHex string) time.Duration {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	var ipDelay, pubkeyDelay time.Duration

	if ip != "" {
		ipDelay = pt.updateState(pt.ipStates, ip, now)
	}
	if pubkeyHex != "" {
		pubkeyDelay = pt.updateState(pt.pubkeyStates, pubkeyHex, now)
	}

	// Return max of both to prevent evasion
	if ipDelay > pubkeyDelay {
		return ipDelay
	}
	return pubkeyDelay
}

// updateState calculates and updates the delay for a single identity.
// The algorithm:
// 1. Decay: subtract elapsed time from accumulated delay (1:1 ratio)
// 2. Add: add perEvent for this new event
// 3. Cap: limit to maxDelay
func (pt *ProgressiveThrottle) updateState(states map[string]*ThrottleState, key string, now time.Time) time.Duration {
	state, exists := states[key]
	if !exists {
		// First event from this identity
		states[key] = &ThrottleState{
			AccumulatedDelay: pt.perEvent,
			LastEventTime:    now,
		}
		return pt.perEvent
	}

	// Decay: subtract elapsed time (1:1 ratio)
	elapsed := now.Sub(state.LastEventTime)
	state.AccumulatedDelay -= elapsed
	if state.AccumulatedDelay < 0 {
		state.AccumulatedDelay = 0
	}

	// Add new event's delay
	state.AccumulatedDelay += pt.perEvent
	state.LastEventTime = now

	// Cap at max
	if state.AccumulatedDelay > pt.maxDelay {
		state.AccumulatedDelay = pt.maxDelay
	}

	return state.AccumulatedDelay
}

// Cleanup removes entries that have fully decayed (no remaining delay).
// This should be called periodically to prevent unbounded memory growth.
func (pt *ProgressiveThrottle) Cleanup() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()

	// Remove IP entries that have fully decayed
	for k, v := range pt.ipStates {
		elapsed := now.Sub(v.LastEventTime)
		if elapsed >= v.AccumulatedDelay {
			delete(pt.ipStates, k)
		}
	}

	// Remove pubkey entries that have fully decayed
	for k, v := range pt.pubkeyStates {
		elapsed := now.Sub(v.LastEventTime)
		if elapsed >= v.AccumulatedDelay {
			delete(pt.pubkeyStates, k)
		}
	}
}

// Stats returns the current number of tracked IPs and pubkeys (for monitoring)
func (pt *ProgressiveThrottle) Stats() (ipCount, pubkeyCount int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return len(pt.ipStates), len(pt.pubkeyStates)
}
