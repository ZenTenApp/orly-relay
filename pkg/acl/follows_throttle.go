package acl

import (
	"time"

	"git.smesh.lol/actor"
)

// ThrottleState tracks accumulated delay for an identity (IP or pubkey)
type ThrottleState struct {
	AccumulatedDelay time.Duration
	LastEventTime    time.Time
}

type throttleGetDelayArgs struct {
	IP        string
	PubkeyHex string
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

	getDelay actor.Func[throttleGetDelayArgs, time.Duration]
	cleanup  actor.Inbox[struct{}]
	stats    actor.Query[throttleStatsResp]
	actor.Lifecycle
}

// NewProgressiveThrottle creates a new throttle with the given parameters.
// perEvent is the delay added per event (e.g., 200ms).
// maxDelay is the maximum accumulated delay cap (e.g., 60s).
func NewProgressiveThrottle(perEvent, maxDelay time.Duration) *ProgressiveThrottle {
	pt := &ProgressiveThrottle{
		perEvent:  perEvent,
		maxDelay:  maxDelay,
		getDelay:  actor.NewFunc[throttleGetDelayArgs, time.Duration](),
		cleanup:   actor.NewInbox[struct{}](16),
		stats:     actor.NewQuery[throttleStatsResp](),
		Lifecycle: actor.NewLifecycle(),
	}
	actor.Go(pt.Lifecycle, pt.actorLoop)
	return pt
}

func (pt *ProgressiveThrottle) actorLoop() {
	ipStates := make(map[string]*ThrottleState)
	pubkeyStates := make(map[string]*ThrottleState)

	for {
		select {
		case <-pt.Stopping():
			return

		case msg := <-pt.getDelay.Recv():
			now := time.Now()
			var ipDelay, pubkeyDelay time.Duration
			if msg.Req.IP != "" {
				ipDelay = pt.updateState(ipStates, msg.Req.IP, now)
			}
			if msg.Req.PubkeyHex != "" {
				pubkeyDelay = pt.updateState(pubkeyStates, msg.Req.PubkeyHex, now)
			}
			d := pubkeyDelay
			if ipDelay > d {
				d = ipDelay
			}
			msg.Reply(d)

		case <-pt.cleanup.Recv():
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

		case msg := <-pt.stats.Recv():
			msg.Reply(throttleStatsResp{
				ipCount:     len(ipStates),
				pubkeyCount: len(pubkeyStates),
			})
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
	return pt.getDelay.Call(throttleGetDelayArgs{IP: ip, PubkeyHex: pubkeyHex})
}

// Cleanup removes entries that have fully decayed (no remaining delay).
// This should be called periodically to prevent unbounded memory growth.
func (pt *ProgressiveThrottle) Cleanup() {
	pt.cleanup.TrySend(struct{}{})
}

// Stats returns the current number of tracked IPs and pubkeys (for monitoring)
func (pt *ProgressiveThrottle) Stats() (ipCount, pubkeyCount int) {
	r := pt.stats.Call()
	return r.ipCount, r.pubkeyCount
}

// Shutdown stops the actor goroutine and waits for it to exit.
func (pt *ProgressiveThrottle) Shutdown() {
	pt.Stop()
}
