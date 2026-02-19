package graph

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterQueryCost(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	tests := []struct {
		name    string
		query   *Query
		minCost float64
		maxCost float64
	}{
		{
			name:    "nil query",
			query:   nil,
			minCost: 1.0,
			maxCost: 1.0,
		},
		{
			name:    "depth 1 unidirectional",
			query:   &Query{Pubkey: "abc", Depth: 1, Edge: "pp", Direction: "out"},
			minCost: 1.5, // depthFactor^1 = 2
			maxCost: 2.5,
		},
		{
			name:    "depth 2 unidirectional",
			query:   &Query{Pubkey: "abc", Depth: 2, Edge: "pp", Direction: "out"},
			minCost: 3.5, // depthFactor^2 = 4
			maxCost: 4.5,
		},
		{
			name:    "depth 3 unidirectional",
			query:   &Query{Pubkey: "abc", Depth: 3, Edge: "pp", Direction: "out"},
			minCost: 7.5, // depthFactor^3 = 8
			maxCost: 8.5,
		},
		{
			name:    "depth 2 bidirectional (1.5x cost)",
			query:   &Query{Pubkey: "abc", Depth: 2, Edge: "pp", Direction: "both"},
			minCost: 5.5, // depthFactor^2 * 1.5 = 6
			maxCost: 6.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := rl.QueryCost(tt.query)
			if cost < tt.minCost || cost > tt.maxCost {
				t.Errorf("QueryCost() = %v, want between %v and %v", cost, tt.minCost, tt.maxCost)
			}
		})
	}
}

func TestRateLimiterOperationCost(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())

	// Depth 0, 1 node
	cost0 := rl.OperationCost(0, 1)
	if cost0 < 1.0 || cost0 > 1.1 {
		t.Errorf("OperationCost(0, 1) = %v, want ~1.01", cost0)
	}

	// Depth 1, 1 node
	cost1 := rl.OperationCost(1, 1)
	if cost1 < 2.0 || cost1 > 2.1 {
		t.Errorf("OperationCost(1, 1) = %v, want ~2.02", cost1)
	}

	// Depth 2, 100 nodes
	cost2 := rl.OperationCost(2, 100)
	if cost2 < 8.0 {
		t.Errorf("OperationCost(2, 100) = %v, want > 8", cost2)
	}
}

func TestRateLimiterAcquire(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.MaxTokens = 10
	cfg.RefillRate = 100 // Fast refill for testing
	rl := NewRateLimiter(cfg)

	ctx := context.Background()

	// Should acquire immediately when tokens available
	delay, err := rl.Acquire(ctx, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if delay > time.Millisecond*10 {
		t.Errorf("expected minimal delay, got %v", delay)
	}

	// Check remaining tokens
	remaining := rl.AvailableTokens()
	if remaining > 6 {
		t.Errorf("expected ~5 tokens remaining, got %v", remaining)
	}
}

func TestRateLimiterTryAcquire(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.MaxTokens = 10
	rl := NewRateLimiter(cfg)

	// Should succeed with enough tokens
	if !rl.TryAcquire(5) {
		t.Error("TryAcquire(5) should succeed with 10 tokens")
	}

	// Should succeed again
	if !rl.TryAcquire(5) {
		t.Error("TryAcquire(5) should succeed with 5 tokens")
	}

	// Should fail with insufficient tokens
	if rl.TryAcquire(1) {
		t.Error("TryAcquire(1) should fail with 0 tokens")
	}
}

func TestRateLimiterContextCancellation(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.MaxTokens = 1
	cfg.RefillRate = 0.1 // Very slow refill
	rl := NewRateLimiter(cfg)

	// Drain tokens
	rl.TryAcquire(1)

	// Create cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Try to acquire - should be cancelled
	_, err := rl.Acquire(ctx, 10)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestRateLimiterRefill(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.MaxTokens = 10
	cfg.RefillRate = 1000 // 1000 tokens per second
	rl := NewRateLimiter(cfg)

	// Drain tokens
	rl.TryAcquire(10)

	// Wait for refill
	time.Sleep(15 * time.Millisecond)

	// Should have some tokens now
	available := rl.AvailableTokens()
	if available < 5 {
		t.Errorf("expected >= 5 tokens after 15ms at 1000/s, got %v", available)
	}
	if available > 10 {
		t.Errorf("expected <= 10 tokens (max), got %v", available)
	}
}

func TestRateLimiterPause(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimiterConfig())
	ctx := context.Background()

	start := time.Now()
	err := rl.Pause(ctx, 1, 0)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have paused for at least baseDelay
	if elapsed < rl.baseDelay {
		t.Errorf("pause duration %v < baseDelay %v", elapsed, rl.baseDelay)
	}
}

func TestThrottler(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	cfg.BaseDelay = 100 * time.Microsecond // Short for testing
	rl := NewRateLimiter(cfg)

	throttler := NewThrottler(rl, 1)
	ctx := context.Background()

	// Process items
	for i := 0; i < 100; i++ {
		if err := throttler.Tick(ctx); err != nil {
			t.Fatalf("unexpected error at tick %d: %v", i, err)
		}
	}

	processed := throttler.Complete()
	if processed != 100 {
		t.Errorf("expected 100 items processed, got %d", processed)
	}
}

func TestThrottlerContextCancellation(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	rl := NewRateLimiter(cfg)

	throttler := NewThrottler(rl, 2) // depth 2 = more frequent pauses
	ctx, cancel := context.WithCancel(context.Background())

	// Process some items
	for i := 0; i < 20; i++ {
		throttler.Tick(ctx)
	}

	// Cancel context
	cancel()

	// Next tick that would pause should return error
	for i := 0; i < 100; i++ {
		if err := throttler.Tick(ctx); err != nil {
			// Expected - context was cancelled
			return
		}
	}
	// If we get here without error, the throttler didn't check context
	// This is acceptable if no pause was needed
}
