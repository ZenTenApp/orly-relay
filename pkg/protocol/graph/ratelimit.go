package graph

import (
	"context"
	"time"
)

// --- actor request/response types ---

type rlAcquireReq struct {
	ctx  context.Context
	cost float64
	resp chan rlAcquireResp
}

type rlAcquireResp struct {
	delay time.Duration
	err   error
}

type rlTryAcquireReq struct {
	cost float64
	resp chan bool
}

type rlAvailableTokensReq struct {
	resp chan float64
}

// RateLimiter implements a token bucket rate limiter with adaptive throttling
// based on graph query complexity. It allows cooperative scheduling by inserting
// pauses between operations to allow other work to proceed.
type RateLimiter struct {
	// Token bucket parameters - owned by actor
	tokens     float64
	maxTokens  float64
	refillRate float64
	lastRefill time.Time

	// Throttling parameters - immutable after construction
	baseDelay   time.Duration
	maxDelay    time.Duration
	depthFactor float64
	limitFactor float64

	// actor channels
	acquireCh        chan rlAcquireReq
	tryAcquireCh     chan rlTryAcquireReq
	availableTokensCh chan rlAvailableTokensReq

	stop chan struct{}
	done chan struct{}
}

// RateLimiterConfig configures the rate limiter behavior.
type RateLimiterConfig struct {
	// MaxTokens is the maximum number of tokens in the bucket (default: 100)
	MaxTokens float64

	// RefillRate is tokens added per second (default: 10)
	RefillRate float64

	// BaseDelay is the minimum delay between operations (default: 1ms)
	BaseDelay time.Duration

	// MaxDelay is the maximum delay for complex queries (default: 100ms)
	MaxDelay time.Duration

	// DepthFactor is the cost multiplier per depth level (default: 2.0)
	// A depth-3 query costs 2^3 = 8x more tokens than depth-1
	DepthFactor float64

	// LimitFactor is additional cost per 100 results requested (default: 0.1)
	LimitFactor float64
}

// DefaultRateLimiterConfig returns sensible defaults for the rate limiter.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		MaxTokens:   100.0,
		RefillRate:  10.0, // Refills fully in 10 seconds
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    100 * time.Millisecond,
		DepthFactor: 2.0,
		LimitFactor: 0.1,
	}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = DefaultRateLimiterConfig().MaxTokens
	}
	if cfg.RefillRate <= 0 {
		cfg.RefillRate = DefaultRateLimiterConfig().RefillRate
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = DefaultRateLimiterConfig().BaseDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = DefaultRateLimiterConfig().MaxDelay
	}
	if cfg.DepthFactor <= 0 {
		cfg.DepthFactor = DefaultRateLimiterConfig().DepthFactor
	}
	if cfg.LimitFactor <= 0 {
		cfg.LimitFactor = DefaultRateLimiterConfig().LimitFactor
	}

	rl := &RateLimiter{
		tokens:      cfg.MaxTokens,
		maxTokens:   cfg.MaxTokens,
		refillRate:  cfg.RefillRate,
		lastRefill:  time.Now(),
		baseDelay:   cfg.BaseDelay,
		maxDelay:    cfg.MaxDelay,
		depthFactor: cfg.DepthFactor,
		limitFactor: cfg.LimitFactor,

		acquireCh:         make(chan rlAcquireReq),
		tryAcquireCh:      make(chan rlTryAcquireReq),
		availableTokensCh: make(chan rlAvailableTokensReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go rl.run()
	return rl
}

// Stop shuts down the actor goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stop)
	<-rl.done
}

func (rl *RateLimiter) run() {
	defer close(rl.done)
	for {
		select {
		case <-rl.stop:
			return
		case req := <-rl.acquireCh:
			// Handle acquire in a separate goroutine so the actor doesn't block
			// waiting for tokens to refill. The goroutine re-enters the actor
			// via tryAcquireCh in a loop.
			go rl.doAcquire(req)
		case req := <-rl.tryAcquireCh:
			rl.refillTokens()
			if rl.tokens >= req.cost {
				rl.tokens -= req.cost
				req.resp <- true
			} else {
				req.resp <- false
			}
		case req := <-rl.availableTokensCh:
			rl.refillTokens()
			req.resp <- rl.tokens
		}
	}
}

func (rl *RateLimiter) doAcquire(req rlAcquireReq) {
	var totalDelay time.Duration

	for {
		// Try to acquire via the actor
		tryResp := make(chan bool, 1)
		select {
		case rl.tryAcquireCh <- rlTryAcquireReq{cost: req.cost, resp: tryResp}:
			if <-tryResp {
				req.resp <- rlAcquireResp{delay: totalDelay, err: nil}
				return
			}
		case <-req.ctx.Done():
			req.resp <- rlAcquireResp{delay: totalDelay, err: req.ctx.Err()}
			return
		case <-rl.stop:
			req.resp <- rlAcquireResp{delay: totalDelay, err: context.Canceled}
			return
		}

		// Not enough tokens - wait for refill
		tokensNeeded := req.cost // approximate - we don't know exact deficit
		waitTime := time.Duration(tokensNeeded/rl.refillRate*1000) * time.Millisecond
		if waitTime > rl.maxDelay {
			waitTime = rl.maxDelay
		}
		if waitTime < rl.baseDelay {
			waitTime = rl.baseDelay
		}

		select {
		case <-req.ctx.Done():
			req.resp <- rlAcquireResp{delay: totalDelay, err: req.ctx.Err()}
			return
		case <-rl.stop:
			req.resp <- rlAcquireResp{delay: totalDelay, err: context.Canceled}
			return
		case <-time.After(waitTime):
			totalDelay += waitTime
		}
	}
}

// refillTokens adds tokens based on elapsed time since last refill.
// Must be called from the actor goroutine only.
func (rl *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.lastRefill = now

	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
}

// QueryCost calculates the token cost for a graph query based on its complexity.
// Higher depths and larger limits cost exponentially more tokens.
// This method is safe to call from any goroutine (reads immutable fields only).
func (rl *RateLimiter) QueryCost(q *Query) float64 {
	if q == nil {
		return 1.0
	}

	cost := 1.0
	for i := 0; i < q.Depth; i++ {
		cost *= rl.depthFactor
	}

	if q.IsBidirectional() {
		cost *= 1.5
	}

	return cost
}

// OperationCost calculates the token cost for a single traversal operation.
// This method is safe to call from any goroutine (reads immutable fields only).
func (rl *RateLimiter) OperationCost(depth int, nodesAtDepth int) float64 {
	depthMultiplier := 1.0
	for i := 0; i < depth; i++ {
		depthMultiplier *= rl.depthFactor
	}
	nodeFactor := 1.0 + float64(nodesAtDepth)*0.01
	return depthMultiplier * nodeFactor
}

// Acquire tries to acquire tokens for a query. If not enough tokens are available,
// it waits until they become available or the context is cancelled.
// Returns the delay that was applied, or an error if context was cancelled.
func (rl *RateLimiter) Acquire(ctx context.Context, cost float64) (time.Duration, error) {
	resp := make(chan rlAcquireResp, 1)
	select {
	case rl.acquireCh <- rlAcquireReq{ctx: ctx, cost: cost, resp: resp}:
		r := <-resp
		return r.delay, r.err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-rl.stop:
		return 0, context.Canceled
	}
}

// TryAcquire attempts to acquire tokens without waiting.
// Returns true if successful, false if insufficient tokens.
func (rl *RateLimiter) TryAcquire(cost float64) bool {
	resp := make(chan bool, 1)
	select {
	case rl.tryAcquireCh <- rlTryAcquireReq{cost: cost, resp: resp}:
		return <-resp
	case <-rl.stop:
		return false
	}
}

// Pause inserts a cooperative delay to allow other work to proceed.
// The delay is proportional to the current depth and load.
// This method does not touch actor state - it only uses immutable config.
func (rl *RateLimiter) Pause(ctx context.Context, depth int, itemsProcessed int) error {
	delay := rl.baseDelay

	for i := 0; i < depth; i++ {
		delay += rl.baseDelay
	}

	if itemsProcessed > 0 && itemsProcessed%100 == 0 {
		delay += rl.baseDelay * 5
	}

	if delay > rl.maxDelay {
		delay = rl.maxDelay
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// AvailableTokens returns the current number of available tokens.
func (rl *RateLimiter) AvailableTokens() float64 {
	resp := make(chan float64, 1)
	select {
	case rl.availableTokensCh <- rlAvailableTokensReq{resp: resp}:
		return <-resp
	case <-rl.stop:
		return 0
	}
}

// Throttler provides a simple interface for cooperative scheduling during traversal.
// It wraps the rate limiter and provides depth-aware throttling.
type Throttler struct {
	rl             *RateLimiter
	depth          int
	itemsProcessed int
}

// NewThrottler creates a throttler for a specific traversal operation.
func NewThrottler(rl *RateLimiter, depth int) *Throttler {
	return &Throttler{
		rl:    rl,
		depth: depth,
	}
}

// Tick should be called after processing each item.
// It tracks progress and inserts pauses as needed.
func (t *Throttler) Tick(ctx context.Context) error {
	t.itemsProcessed++

	interval := 50
	if t.depth >= 2 {
		interval = 25
	}
	if t.depth >= 4 {
		interval = 10
	}

	if t.itemsProcessed%interval == 0 {
		return t.rl.Pause(ctx, t.depth, t.itemsProcessed)
	}
	return nil
}

// Complete marks the throttler as complete and returns stats.
func (t *Throttler) Complete() (itemsProcessed int) {
	return t.itemsProcessed
}
