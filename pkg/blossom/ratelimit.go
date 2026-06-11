package blossom

import (
	"time"
)

// BandwidthState tracks upload bandwidth for an identity
type BandwidthState struct {
	BucketBytes int64     // Current token bucket level (bytes available)
	LastUpdate  time.Time // Last time bucket was updated
}

// checkAndConsumeReq is sent to the actor to check and consume bandwidth.
type checkAndConsumeReq struct {
	identity  string
	sizeBytes int64
	resp      chan bool
}

// getAvailableReq is sent to the actor to query available bandwidth.
type getAvailableReq struct {
	identity string
	resp     chan int64
}

// cleanupReq signals the actor to run cleanup.
type cleanupReq struct {
	resp chan struct{}
}

// statsReq asks the actor for the count of tracked identities.
type statsReq struct {
	resp chan int
}

// BandwidthLimiter implements token bucket rate limiting for uploads.
// Each identity gets a bucket that replenishes at dailyLimit/day rate.
// Uploads consume tokens from the bucket.
type BandwidthLimiter struct {
	checkAndConsumeCh chan checkAndConsumeReq
	getAvailableCh    chan getAvailableReq
	cleanupCh         chan cleanupReq
	statsCh           chan statsReq

	stop chan struct{}
	done chan struct{}

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
		checkAndConsumeCh: make(chan checkAndConsumeReq),
		getAvailableCh:    make(chan getAvailableReq),
		cleanupCh:         make(chan cleanupReq),
		statsCh:           make(chan statsReq),
		stop:              make(chan struct{}),
		done:              make(chan struct{}),
		burstLimit:        burstBytes,
		refillRate:        float64(dailyBytes) / 86400.0,
	}

	go bl.run(burstBytes, float64(dailyBytes)/86400.0)
	return bl
}

// run is the actor goroutine that owns all mutable state.
func (bl *BandwidthLimiter) run(burstLimit int64, refillRate float64) {
	defer close(bl.done)

	states := make(map[string]*BandwidthState)

	for {
		select {
		case <-bl.stop:
			return

		case req := <-bl.checkAndConsumeCh:
			now := time.Now()
			state, exists := states[req.identity]
			if !exists {
				state = &BandwidthState{
					BucketBytes: burstLimit,
					LastUpdate:  now,
				}
				states[req.identity] = state
			} else {
				elapsed := now.Sub(state.LastUpdate).Seconds()
				refill := int64(elapsed * refillRate)
				state.BucketBytes += refill
				if state.BucketBytes > burstLimit {
					state.BucketBytes = burstLimit
				}
				state.LastUpdate = now
			}
			if state.BucketBytes >= req.sizeBytes {
				state.BucketBytes -= req.sizeBytes
				req.resp <- true
			} else {
				req.resp <- false
			}

		case req := <-bl.getAvailableCh:
			state, exists := states[req.identity]
			if !exists {
				req.resp <- burstLimit
			} else {
				now := time.Now()
				elapsed := now.Sub(state.LastUpdate).Seconds()
				refill := int64(elapsed * refillRate)
				available := state.BucketBytes + refill
				if available > burstLimit {
					available = burstLimit
				}
				req.resp <- available
			}

		case req := <-bl.cleanupCh:
			now := time.Now()
			for key, state := range states {
				elapsed := now.Sub(state.LastUpdate).Seconds()
				refill := int64(elapsed * refillRate)
				if state.BucketBytes+refill >= burstLimit {
					delete(states, key)
				}
			}
			close(req.resp)

		case req := <-bl.statsCh:
			req.resp <- len(states)
		}
	}
}

// Stop shuts down the actor goroutine.
func (bl *BandwidthLimiter) Stop() {
	close(bl.stop)
	<-bl.done
}

// CheckAndConsume checks if an upload of the given size is allowed for the identity,
// and if so, consumes the tokens. Returns true if allowed, false if rate limited.
// The identity should be pubkey hex for authenticated users, or IP for anonymous.
func (bl *BandwidthLimiter) CheckAndConsume(identity string, sizeBytes int64) bool {
	resp := make(chan bool, 1)
	bl.checkAndConsumeCh <- checkAndConsumeReq{
		identity:  identity,
		sizeBytes: sizeBytes,
		resp:      resp,
	}
	return <-resp
}

// GetAvailable returns the currently available bytes for an identity.
func (bl *BandwidthLimiter) GetAvailable(identity string) int64 {
	resp := make(chan int64, 1)
	bl.getAvailableCh <- getAvailableReq{
		identity: identity,
		resp:     resp,
	}
	return <-resp
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
	resp := make(chan struct{})
	bl.cleanupCh <- cleanupReq{resp: resp}
	<-resp
}

// Stats returns the number of tracked identities.
func (bl *BandwidthLimiter) Stats() int {
	resp := make(chan int, 1)
	bl.statsCh <- statsReq{resp: resp}
	return <-resp
}
