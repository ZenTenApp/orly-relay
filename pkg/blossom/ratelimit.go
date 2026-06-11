package blossom

import (
	"time"

	"git.smesh.lol/actor"
)

// BandwidthState tracks upload bandwidth for an identity
type BandwidthState struct {
	BucketBytes int64     // Current token bucket level (bytes available)
	LastUpdate  time.Time // Last time bucket was updated
}

type checkAndConsumeArgs struct {
	Identity  string
	SizeBytes int64
}

// BandwidthLimiter implements token bucket rate limiting for uploads.
// Each identity gets a bucket that replenishes at dailyLimit/day rate.
// Uploads consume tokens from the bucket.
// All mutable state is owned by the actor goroutine.
type BandwidthLimiter struct {
	checkAndConsume actor.Func[checkAndConsumeArgs, bool]
	getAvailable    actor.Func[string, int64]
	cleanup         actor.Signal
	stats           actor.Query[int]
	actor.Lifecycle

	burstLimit int64   // exposed for GetTimeUntilAvailable
	refillRate float64 // exposed for GetTimeUntilAvailable
}

// NewBandwidthLimiter creates a new bandwidth limiter.
// dailyLimitMB is the average daily limit in megabytes.
// burstLimitMB is the maximum burst capacity in megabytes.
func NewBandwidthLimiter(dailyLimitMB, burstLimitMB int64) *BandwidthLimiter {
	dailyBytes := dailyLimitMB * 1024 * 1024
	burstBytes := burstLimitMB * 1024 * 1024

	bl := &BandwidthLimiter{
		checkAndConsume: actor.NewFunc[checkAndConsumeArgs, bool](),
		getAvailable:    actor.NewFunc[string, int64](),
		cleanup:         actor.NewSignal(),
		stats:           actor.NewQuery[int](),
		Lifecycle:       actor.NewLifecycle(),
		burstLimit:      burstBytes,
		refillRate:      float64(dailyBytes) / 86400.0,
	}

	actor.Go(bl.Lifecycle, func() { bl.actorLoop(burstBytes, float64(dailyBytes)/86400.0) })
	return bl
}

// actorLoop owns all mutable state.
func (bl *BandwidthLimiter) actorLoop(burstLimit int64, refillRate float64) {
	states := make(map[string]*BandwidthState)

	for {
		select {
		case <-bl.Stopping():
			return

		case msg := <-bl.checkAndConsume.Recv():
			now := time.Now()
			state, exists := states[msg.Req.Identity]
			if !exists {
				state = &BandwidthState{
					BucketBytes: burstLimit,
					LastUpdate:  now,
				}
				states[msg.Req.Identity] = state
			} else {
				elapsed := now.Sub(state.LastUpdate).Seconds()
				refill := int64(elapsed * refillRate)
				state.BucketBytes += refill
				if state.BucketBytes > burstLimit {
					state.BucketBytes = burstLimit
				}
				state.LastUpdate = now
			}
			if state.BucketBytes >= msg.Req.SizeBytes {
				state.BucketBytes -= msg.Req.SizeBytes
				msg.Reply(true)
			} else {
				msg.Reply(false)
			}

		case msg := <-bl.getAvailable.Recv():
			state, exists := states[msg.Req]
			if !exists {
				msg.Reply(burstLimit)
			} else {
				now := time.Now()
				elapsed := now.Sub(state.LastUpdate).Seconds()
				refill := int64(elapsed * refillRate)
				available := state.BucketBytes + refill
				if available > burstLimit {
					available = burstLimit
				}
				msg.Reply(available)
			}

		case msg := <-bl.cleanup.Recv():
			now := time.Now()
			for key, state := range states {
				elapsed := now.Sub(state.LastUpdate).Seconds()
				refill := int64(elapsed * refillRate)
				if state.BucketBytes+refill >= burstLimit {
					delete(states, key)
				}
			}
			msg.Done()

		case msg := <-bl.stats.Recv():
			msg.Reply(len(states))
		}
	}
}

// Shutdown stops the actor goroutine.
func (bl *BandwidthLimiter) Shutdown() {
	bl.Stop()
}

// CheckAndConsume checks if an upload of the given size is allowed for the identity,
// and if so, consumes the tokens. Returns true if allowed, false if rate limited.
// The identity should be pubkey hex for authenticated users, or IP for anonymous.
func (bl *BandwidthLimiter) CheckAndConsume(identity string, sizeBytes int64) bool {
	return bl.checkAndConsume.Call(checkAndConsumeArgs{Identity: identity, SizeBytes: sizeBytes})
}

// GetAvailable returns the currently available bytes for an identity.
func (bl *BandwidthLimiter) GetAvailable(identity string) int64 {
	return bl.getAvailable.Call(identity)
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
	bl.cleanup.Call()
}

// Stats returns the number of tracked identities.
func (bl *BandwidthLimiter) Stats() int {
	return bl.stats.Call()
}
