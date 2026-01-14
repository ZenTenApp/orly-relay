package blossom

import (
	"sync"
	"time"
)

// BandwidthState tracks upload bandwidth for an identity
type BandwidthState struct {
	BucketBytes   int64     // Current token bucket level (bytes available)
	LastUpdate    time.Time // Last time bucket was updated
}

// BandwidthLimiter implements token bucket rate limiting for uploads.
// Each identity gets a bucket that replenishes at dailyLimit/day rate.
// Uploads consume tokens from the bucket.
type BandwidthLimiter struct {
	mu           sync.Mutex
	states       map[string]*BandwidthState // keyed by pubkey hex or IP
	dailyLimit   int64                      // bytes per day
	burstLimit   int64                      // max bucket size (burst capacity)
	refillRate   float64                    // bytes per second refill rate
}

// NewBandwidthLimiter creates a new bandwidth limiter.
// dailyLimitMB is the average daily limit in megabytes.
// burstLimitMB is the maximum burst capacity in megabytes.
func NewBandwidthLimiter(dailyLimitMB, burstLimitMB int64) *BandwidthLimiter {
	dailyBytes := dailyLimitMB * 1024 * 1024
	burstBytes := burstLimitMB * 1024 * 1024

	return &BandwidthLimiter{
		states:     make(map[string]*BandwidthState),
		dailyLimit: dailyBytes,
		burstLimit: burstBytes,
		refillRate: float64(dailyBytes) / 86400.0, // bytes per second
	}
}

// CheckAndConsume checks if an upload of the given size is allowed for the identity,
// and if so, consumes the tokens. Returns true if allowed, false if rate limited.
// The identity should be pubkey hex for authenticated users, or IP for anonymous.
func (bl *BandwidthLimiter) CheckAndConsume(identity string, sizeBytes int64) bool {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	now := time.Now()
	state, exists := bl.states[identity]

	if !exists {
		// New identity starts with full burst capacity
		state = &BandwidthState{
			BucketBytes: bl.burstLimit,
			LastUpdate:  now,
		}
		bl.states[identity] = state
	} else {
		// Refill bucket based on elapsed time
		elapsed := now.Sub(state.LastUpdate).Seconds()
		refill := int64(elapsed * bl.refillRate)
		state.BucketBytes += refill
		if state.BucketBytes > bl.burstLimit {
			state.BucketBytes = bl.burstLimit
		}
		state.LastUpdate = now
	}

	// Check if upload fits in bucket
	if state.BucketBytes >= sizeBytes {
		state.BucketBytes -= sizeBytes
		return true
	}

	return false
}

// GetAvailable returns the currently available bytes for an identity.
func (bl *BandwidthLimiter) GetAvailable(identity string) int64 {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	state, exists := bl.states[identity]
	if !exists {
		return bl.burstLimit // New users have full capacity
	}

	// Calculate current level with refill
	now := time.Now()
	elapsed := now.Sub(state.LastUpdate).Seconds()
	refill := int64(elapsed * bl.refillRate)
	available := state.BucketBytes + refill
	if available > bl.burstLimit {
		available = bl.burstLimit
	}

	return available
}

// GetTimeUntilAvailable returns how long until the given bytes will be available.
func (bl *BandwidthLimiter) GetTimeUntilAvailable(identity string, sizeBytes int64) time.Duration {
	available := bl.GetAvailable(identity)
	if available >= sizeBytes {
		return 0
	}

	needed := sizeBytes - available
	seconds := float64(needed) / bl.refillRate
	return time.Duration(seconds * float64(time.Second))
}

// Cleanup removes entries that have fully replenished (at burst limit).
func (bl *BandwidthLimiter) Cleanup() {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	now := time.Now()
	for key, state := range bl.states {
		elapsed := now.Sub(state.LastUpdate).Seconds()
		refill := int64(elapsed * bl.refillRate)
		if state.BucketBytes+refill >= bl.burstLimit {
			delete(bl.states, key)
		}
	}
}

// Stats returns the number of tracked identities.
func (bl *BandwidthLimiter) Stats() int {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	return len(bl.states)
}
