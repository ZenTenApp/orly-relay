package graph

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter with adaptive throttling
// based on graph query complexity. It allows cooperative scheduling by inserting
// pauses between operations to allow other work to proceed.
type RateLimiter struct {
	mu sync.Mutex

	// Token bucket parameters
	tokens        float64   // Current available tokens
	maxTokens     float64   // Maximum token capacity
	refillRate    float64   // Tokens per second to add
	lastRefill    time.Time // Last time tokens were refilled

	// Throttling parameters
	baseDelay     time.Duration // Minimum delay between operations
	maxDelay      time.Duration // Maximum delay for complex queries
	depthFactor   float64       // Multiplier per depth level
	limitFactor   float64       // Multiplier based on result limit
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

	return &RateLimiter{
		tokens:      cfg.MaxTokens,
		maxTokens:   cfg.MaxTokens,
		refillRate:  cfg.RefillRate,
		lastRefill:  time.Now(),
		baseDelay:   cfg.BaseDelay,
		maxDelay:    cfg.MaxDelay,
		depthFactor: cfg.DepthFactor,
		limitFactor: cfg.LimitFactor,
	}
}

// QueryCost calculates the token cost for a graph query based on its complexity.
// Higher depths and larger limits cost exponentially more tokens.
func (rl *RateLimiter) QueryCost(q *Query) float64 {
	if q == nil {
		return 1.0
	}

	// Base cost is exponential in depth: depthFactor^depth
	// This models the exponential growth of traversal work
	cost := 1.0
	for i := 0; i < q.Depth; i++ {
		cost *= rl.depthFactor
	}

	// Add cost for bidirectional queries (traversing both directions)
	if q.IsBidirectional() {
		cost *= 1.5
	}

	return cost
}

// OperationCost calculates the token cost for a single traversal operation.
// This is used during query execution for per-operation throttling.
func (rl *RateLimiter) OperationCost(depth int, nodesAtDepth int) float64 {
	// Cost increases with depth and number of nodes to process
	depthMultiplier := 1.0
	for i := 0; i < depth; i++ {
		depthMultiplier *= rl.depthFactor
	}

	// More nodes at this depth = more work
	nodeFactor := 1.0 + float64(nodesAtDepth)*0.01

	return depthMultiplier * nodeFactor
}

// refillTokens adds tokens based on elapsed time since last refill.
func (rl *RateLimiter) refillTokens() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.lastRefill = now

	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
}

// Acquire tries to acquire tokens for a query. If not enough tokens are available,
// it waits until they become available or the context is cancelled.
// Returns the delay that was applied, or an error if context was cancelled.
func (rl *RateLimiter) Acquire(ctx context.Context, cost float64) (time.Duration, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refillTokens()

	var totalDelay time.Duration

	// Wait until we have enough tokens
	for rl.tokens < cost {
		// Calculate how long we need to wait for tokens to refill
		tokensNeeded := cost - rl.tokens
		waitTime := time.Duration(tokensNeeded/rl.refillRate*1000) * time.Millisecond

		// Clamp to max delay
		if waitTime > rl.maxDelay {
			waitTime = rl.maxDelay
		}
		if waitTime < rl.baseDelay {
			waitTime = rl.baseDelay
		}

		// Release lock while waiting
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			rl.mu.Lock()
			return totalDelay, ctx.Err()
		case <-time.After(waitTime):
		}

		totalDelay += waitTime
		rl.mu.Lock()
		rl.refillTokens()
	}

	// Consume tokens
	rl.tokens -= cost
	return totalDelay, nil
}

// TryAcquire attempts to acquire tokens without waiting.
// Returns true if successful, false if insufficient tokens.
func (rl *RateLimiter) TryAcquire(cost float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refillTokens()

	if rl.tokens >= cost {
		rl.tokens -= cost
		return true
	}
	return false
}

// Pause inserts a cooperative delay to allow other work to proceed.
// The delay is proportional to the current depth and load.
// This should be called periodically during long-running traversals.
func (rl *RateLimiter) Pause(ctx context.Context, depth int, itemsProcessed int) error {
	// Calculate adaptive delay based on depth and progress
	// Deeper traversals and more processed items = longer pauses
	delay := rl.baseDelay

	// Increase delay with depth
	for i := 0; i < depth; i++ {
		delay += rl.baseDelay
	}

	// Add extra delay every N items to allow other work
	if itemsProcessed > 0 && itemsProcessed%100 == 0 {
		delay += rl.baseDelay * 5
	}

	// Cap at max delay
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
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refillTokens()
	return rl.tokens
}

// Throttler provides a simple interface for cooperative scheduling during traversal.
// It wraps the rate limiter and provides depth-aware throttling.
type Throttler struct {
	rl            *RateLimiter
	depth         int
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

	// Insert cooperative pause periodically
	// More frequent pauses at higher depths
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
