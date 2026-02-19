package bridge

import (
	"strings"
	"testing"
	"time"
)

func TestRateLimiter_AllowsInitialSend(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimitConfig())

	if err := rl.Check("user1"); err != nil {
		t.Errorf("first send should be allowed: %v", err)
	}
}

func TestRateLimiter_MinInterval(t *testing.T) {
	cfg := RateLimitConfig{
		MinInterval:    1 * time.Second,
		PerUserPerHour: 100, // high limits so only interval matters
		PerUserPerDay:  1000,
		GlobalPerHour:  10000,
		GlobalPerDay:   100000,
	}
	rl := NewRateLimiter(cfg)

	// First send
	if err := rl.Check("user1"); err != nil {
		t.Fatalf("first check failed: %v", err)
	}
	rl.Record("user1")

	// Immediate retry should be rate limited
	if err := rl.Check("user1"); err == nil {
		t.Error("expected rate limit error for immediate retry")
	} else if !strings.Contains(err.Error(), "wait") {
		t.Errorf("error should mention wait time: %v", err)
	}

	// Different user should be fine
	if err := rl.Check("user2"); err != nil {
		t.Errorf("different user should not be limited: %v", err)
	}
}

func TestRateLimiter_PerUserPerHour(t *testing.T) {
	cfg := RateLimitConfig{
		MinInterval:    0, // disable interval check
		PerUserPerHour: 3,
		PerUserPerDay:  1000,
		GlobalPerHour:  10000,
		GlobalPerDay:   100000,
	}
	rl := NewRateLimiter(cfg)

	for i := 0; i < 3; i++ {
		if err := rl.Check("user1"); err != nil {
			t.Fatalf("send %d should be allowed: %v", i+1, err)
		}
		rl.Record("user1")
	}

	// 4th send should be rate limited
	if err := rl.Check("user1"); err == nil {
		t.Error("expected per-user hourly rate limit")
	} else if !strings.Contains(err.Error(), "per hour") {
		t.Errorf("error should mention per hour: %v", err)
	}

	// Different user should still be fine
	if err := rl.Check("user2"); err != nil {
		t.Errorf("different user should not be limited: %v", err)
	}
}

func TestRateLimiter_PerUserPerDay(t *testing.T) {
	cfg := RateLimitConfig{
		MinInterval:    0,
		PerUserPerHour: 1000, // high
		PerUserPerDay:  3,
		GlobalPerHour:  10000,
		GlobalPerDay:   100000,
	}
	rl := NewRateLimiter(cfg)

	for i := 0; i < 3; i++ {
		if err := rl.Check("user1"); err != nil {
			t.Fatalf("send %d should be allowed: %v", i+1, err)
		}
		rl.Record("user1")
	}

	if err := rl.Check("user1"); err == nil {
		t.Error("expected per-user daily rate limit")
	} else if !strings.Contains(err.Error(), "per day") {
		t.Errorf("error should mention per day: %v", err)
	}
}

func TestRateLimiter_GlobalPerHour(t *testing.T) {
	cfg := RateLimitConfig{
		MinInterval:    0,
		PerUserPerHour: 1000,
		PerUserPerDay:  10000,
		GlobalPerHour:  3,
		GlobalPerDay:   100000,
	}
	rl := NewRateLimiter(cfg)

	// 3 different users each send once
	for i := 0; i < 3; i++ {
		user := "user" + string(rune('A'+i))
		if err := rl.Check(user); err != nil {
			t.Fatalf("send from %s should be allowed: %v", user, err)
		}
		rl.Record(user)
	}

	// 4th user should hit global limit
	if err := rl.Check("userD"); err == nil {
		t.Error("expected global hourly rate limit")
	} else if !strings.Contains(err.Error(), "global hourly") {
		t.Errorf("error should mention global hourly: %v", err)
	}
}

func TestRateLimiter_GlobalPerDay(t *testing.T) {
	cfg := RateLimitConfig{
		MinInterval:    0,
		PerUserPerHour: 1000,
		PerUserPerDay:  10000,
		GlobalPerHour:  10000,
		GlobalPerDay:   3,
	}
	rl := NewRateLimiter(cfg)

	for i := 0; i < 3; i++ {
		user := "user" + string(rune('A'+i))
		if err := rl.Check(user); err != nil {
			t.Fatalf("send from %s should be allowed: %v", user, err)
		}
		rl.Record(user)
	}

	if err := rl.Check("userD"); err == nil {
		t.Error("expected global daily rate limit")
	} else if !strings.Contains(err.Error(), "global daily") {
		t.Errorf("error should mention global daily: %v", err)
	}
}

func TestRateLimiter_ZeroConfigDisables(t *testing.T) {
	// All zeros means no limits
	cfg := RateLimitConfig{}
	rl := NewRateLimiter(cfg)

	for i := 0; i < 100; i++ {
		if err := rl.Check("user1"); err != nil {
			t.Fatalf("send %d should be allowed with zero config: %v", i+1, err)
		}
		rl.Record("user1")
	}
}

func TestRateLimiter_DefaultConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.PerUserPerHour != 10 {
		t.Errorf("PerUserPerHour = %d, want 10", cfg.PerUserPerHour)
	}
	if cfg.PerUserPerDay != 50 {
		t.Errorf("PerUserPerDay = %d, want 50", cfg.PerUserPerDay)
	}
	if cfg.GlobalPerHour != 100 {
		t.Errorf("GlobalPerHour = %d, want 100", cfg.GlobalPerHour)
	}
	if cfg.GlobalPerDay != 500 {
		t.Errorf("GlobalPerDay = %d, want 500", cfg.GlobalPerDay)
	}
	if cfg.MinInterval != 30*time.Second {
		t.Errorf("MinInterval = %v, want 30s", cfg.MinInterval)
	}
}

func TestWindow_CountSince_Prunes(t *testing.T) {
	w := newWindow()

	now := time.Now()
	// Add 3 old timestamps and 2 recent ones
	w.add(now.Add(-2 * time.Hour))
	w.add(now.Add(-90 * time.Minute))
	w.add(now.Add(-61 * time.Minute))
	w.add(now.Add(-30 * time.Minute))
	w.add(now.Add(-5 * time.Minute))

	count := w.countSince(now.Add(-time.Hour))
	if count != 2 {
		t.Errorf("countSince = %d, want 2", count)
	}

	// Old entries should be pruned
	if len(w.times) != 2 {
		t.Errorf("after prune, len = %d, want 2", len(w.times))
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(DefaultRateLimitConfig())

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			user := "user" + string(rune('A'+id))
			for j := 0; j < 5; j++ {
				rl.Check(user)
				rl.Record(user)
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	// If we get here without a race condition panic, the test passes.
}
