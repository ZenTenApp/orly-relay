package grapevine

import (
	"context"
	"time"

	"git.smesh.lol/orly/pkg/lol/log"
)

// triggerComputeReq is a request to start computation for a single observer.
type triggerComputeReq struct {
	observerHex string
	resp        chan bool
}

// computeDoneMsg signals that a computation has finished.
type computeDoneMsg struct {
	observerHex string
}

// Scheduler runs periodic GrapeVine score computation for configured observers.
type Scheduler struct {
	engine    *Engine
	observers []string // hex pubkeys
	interval  time.Duration

	// Actor channels
	triggerCh chan triggerComputeReq // buffered 1 (resp channel)
	doneCh    chan computeDoneMsg    // buffered 16 (fire-and-forget)
	stop      chan struct{}
	done      chan struct{}
}

// NewScheduler creates a new scheduler.
func NewScheduler(engine *Engine, observers []string, interval time.Duration) *Scheduler {
	return &Scheduler{
		engine:    engine,
		observers: observers,
		interval:  interval,
		triggerCh: make(chan triggerComputeReq),
		doneCh:    make(chan computeDoneMsg, 16),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start runs the periodic computation loop. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	log.I.F("grapevine: scheduler started for %d observers, interval %v", len(s.observers), s.interval)

	go s.actor(ctx)

	<-s.done
}

// Shutdown stops the scheduler actor and waits for it to exit.
func (s *Scheduler) Shutdown() {
	close(s.stop)
	<-s.done
}

// actor owns all mutable state (computing map) and processes requests via select.
func (s *Scheduler) actor(ctx context.Context) {
	defer close(s.done)

	computing := make(map[string]bool)

	// Immediate first run
	s.runAllSync(computing)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.I.F("grapevine: scheduler stopped")
			return
		case <-s.stop:
			log.I.F("grapevine: scheduler stopped")
			return
		case req := <-s.triggerCh:
			if computing[req.observerHex] {
				req.resp <- false
			} else {
				computing[req.observerHex] = true
				go s.computeAsync(req.observerHex)
				req.resp <- true
			}
		case msg := <-s.doneCh:
			delete(computing, msg.observerHex)
		case <-ticker.C:
			s.runAllSync(computing)
		}
	}
}

// TriggerCompute starts an async computation for a single observer.
// Returns immediately. No-op if already computing for that observer.
func (s *Scheduler) TriggerCompute(observerHex string) bool {
	resp := make(chan bool, 1)
	s.triggerCh <- triggerComputeReq{observerHex: observerHex, resp: resp}
	return <-resp
}

// computeAsync runs a computation and signals completion to the actor.
func (s *Scheduler) computeAsync(observerHex string) {
	if _, err := s.engine.Compute(observerHex); err != nil {
		log.E.F("grapevine: compute failed for %s: %v", observerHex[:12], err)
	}
	select {
	case s.doneCh <- computeDoneMsg{observerHex: observerHex}:
	default:
	}
}

// runAllSync runs computation for all observers synchronously, skipping those already computing.
// Must only be called from the actor goroutine.
func (s *Scheduler) runAllSync(computing map[string]bool) {
	for _, obs := range s.observers {
		if computing[obs] {
			log.D.F("grapevine: skipping %s, already computing", obs[:12])
			continue
		}
		computing[obs] = true

		func(observerHex string) {
			defer func() {
				delete(computing, observerHex)
			}()
			if _, err := s.engine.Compute(observerHex); err != nil {
				log.E.F("grapevine: scheduled compute failed for %s: %v", observerHex[:12], err)
			}
		}(obs)
	}
}
