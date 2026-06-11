package acl

import (
	"time"
)

// ThrottleState tracks accumulated delay for an identity (IP or pubkey)
type ThrottleState struct {
	AccumulatedDelay time.Duration
	LastEventTime    time.Time
}

// throttleGetDelayReq is sent to the actor to compute delay for an identity.
type throttleGetDelayReq struct {
	ip        string
	pubkeyHex string
	resp      chan time.Duration
}

// throttleStatsReq is sent to the actor to retrieve tracking counts.
type throttleStatsReq struct {
	resp chan throttleStatsResp
}

type throttleStatsResp struct {
	ipCount     int
	pubkeyCount int
}

// ProgressiveThrottle implements linear delay with time decay.
// Each event adds perEvent delay, and delay decays at 1:1 ratio with elapsed time.
// This creates a natural rate limit that averages to 1 event per perEvent interval.
//
// All state is owned by a single actor goroutine; no mutexes.
type ProgressiveThrottle struct {
	perEvent time.Duration // delay increment per event (default 200ms)
	maxDelay time.Duration // cap (default 60s)

	getDelayCh chan throttleGetDelayReq
	cleanupCh  chan struct{}
	statsCh    chan throttleStatsReq
	stop       chan struct{}
	done       chan struct{}
}

// NewProgressiveThrottle creates a new throttle with the given parameters.
// perEvent is the delay added per event (e.g., 200ms).
// maxDelay is the maximum accumulated delay cap (e.g., 60s).
func NewProgressiveThrottle(perEvent, maxDelay time.Duration) *ProgressiveThrottle {
	pt := &ProgressiveThrottle{
		perEvent:   perEvent,
		maxDelay:   maxDelay,
		getDelayCh: make(chan throttleGetDelayReq),
		cleanupCh:  make(chan struct{}, 16),
		statsCh:    make(chan throttleStatsReq),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go pt.actor()
	return pt
}

func (pt *ProgressiveThrottle) actor() {
	defer close(pt.done)

	ipStates := make(map[string]*ThrottleState)
	pubkeyStates := make(map[string]*ThrottleState)

	for {
		select {
		case <-pt.stop:
			return

		case req := <-pt.getDelayCh:
			now := time.Now()
			var ipDelay, pubkeyDelay time.Duration
			if req.ip != "" {
				ipDelay = pt.updateState(ipStates, req.ip, now)
			}
			if req.pubkeyHex != "" {
				pubkeyDelay = pt.updateState(pubkeyStates, req.pubkeyHex, now)
			}
			d := pubkeyDelay
			if ipDelay > d {
				d = ipDelay
			}
			req.resp <- d

		case <-pt.cleanupCh:
			now := time.Now()
			for k, v := range ipStates {
				elapsed := now.Sub(v.LastEventTime)
				if elapsed >= v.AccumulatedDelay {
					delete(ipStates, k)
				}
			}
			for k, v := range pubkeyStates {
				elapsed := now.Sub(v.LastEventTime)
				if elapsed >= v.AccumulatedDelay {
					delete(pubkeyStates, k)
				}
			}

		case req := <-pt.statsCh:
			req.resp <- throttleStatsResp{
				ipCount:     len(ipStates),
				pubkeyCount: len(pubkeyStates),
			}
		}
	}
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

// GetDelay returns accumulated delay for this identity and updates state.
// It tracks both IP and pubkey independently and returns the maximum of both.
// This prevents evasion via different pubkeys from same IP or vice versa.
func (pt *ProgressiveThrottle) GetDelay(ip, pubkeyHex string) time.Duration {
	resp := make(chan time.Duration, 1)
	pt.getDelayCh <- throttleGetDelayReq{ip: ip, pubkeyHex: pubkeyHex, resp: resp}
	return <-resp
}

// Cleanup removes entries that have fully decayed (no remaining delay).
// This should be called periodically to prevent unbounded memory growth.
func (pt *ProgressiveThrottle) Cleanup() {
	select {
	case pt.cleanupCh <- struct{}{}:
	default:
	}
}

// Stats returns the current number of tracked IPs and pubkeys (for monitoring)
func (pt *ProgressiveThrottle) Stats() (ipCount, pubkeyCount int) {
	resp := make(chan throttleStatsResp, 1)
	pt.statsCh <- throttleStatsReq{resp: resp}
	r := <-resp
	return r.ipCount, r.pubkeyCount
}

// Stop shuts down the actor goroutine and waits for it to exit.
func (pt *ProgressiveThrottle) Stop() {
	close(pt.stop)
	<-pt.done
}
