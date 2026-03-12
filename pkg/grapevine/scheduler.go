package grapevine

import (
	"context"
	"sync"
	"time"

	"next.orly.dev/pkg/lol/log"
)

// Scheduler runs periodic GrapeVine score computation for configured observers.
type Scheduler struct {
	engine    *Engine
	observers []string // hex pubkeys
	interval  time.Duration
	mu        sync.Mutex
	computing map[string]bool
}

// NewScheduler creates a new scheduler.
func NewScheduler(engine *Engine, observers []string, interval time.Duration) *Scheduler {
	return &Scheduler{
		engine:    engine,
		observers: observers,
		interval:  interval,
		computing: make(map[string]bool),
	}
}

// Start runs the periodic computation loop. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	log.I.F("grapevine: scheduler started for %d observers, interval %v", len(s.observers), s.interval)

	// Immediate first run
	s.runAll()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.I.F("grapevine: scheduler stopped")
			return
		case <-ticker.C:
			s.runAll()
		}
	}
}

// TriggerCompute starts an async computation for a single observer.
// Returns immediately. No-op if already computing for that observer.
func (s *Scheduler) TriggerCompute(observerHex string) bool {
	s.mu.Lock()
	if s.computing[observerHex] {
		s.mu.Unlock()
		return false
	}
	s.computing[observerHex] = true
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.computing, observerHex)
			s.mu.Unlock()
		}()
		if _, err := s.engine.Compute(observerHex); err != nil {
			log.E.F("grapevine: compute failed for %s: %v", observerHex[:12], err)
		}
	}()
	return true
}

func (s *Scheduler) runAll() {
	for _, obs := range s.observers {
		s.mu.Lock()
		if s.computing[obs] {
			s.mu.Unlock()
			log.D.F("grapevine: skipping %s, already computing", obs[:12])
			continue
		}
		s.computing[obs] = true
		s.mu.Unlock()

		func(observerHex string) {
			defer func() {
				s.mu.Lock()
				delete(s.computing, observerHex)
				s.mu.Unlock()
			}()
			if _, err := s.engine.Compute(observerHex); err != nil {
				log.E.F("grapevine: scheduled compute failed for %s: %v", observerHex[:12], err)
			}
		}(obs)
	}
}
