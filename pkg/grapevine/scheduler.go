package grapevine

import (
	"context"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Scheduler runs periodic GrapeVine score computation for configured observers.
type Scheduler struct {
	engine    *Engine
	observers []string // hex pubkeys
	interval  time.Duration

	// Actor channels
	trigger     actor.Func[string, bool]
	computeDone actor.Inbox[string]
	lc          actor.Lifecycle
}

// NewScheduler creates a new scheduler.
func NewScheduler(engine *Engine, observers []string, interval time.Duration) *Scheduler {
	return &Scheduler{
		engine:      engine,
		observers:   observers,
		interval:    interval,
		trigger:     actor.NewFunc[string, bool](),
		computeDone: actor.NewInbox[string](16),
		lc:          actor.NewLifecycle(),
	}
}

// Start runs the periodic computation loop. Blocks until ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	log.I.F("grapevine: scheduler started for %d observers, interval %v", len(s.observers), s.interval)

	actor.Go(s.lc, func() { s.actorLoop(ctx) })

	<-s.lc.Stopped()
}

// Shutdown stops the scheduler actor and waits for it to exit.
func (s *Scheduler) Shutdown() {
	s.lc.Stop()
}

// actorLoop owns all mutable state (computing map) and processes requests via select.
func (s *Scheduler) actorLoop(ctx context.Context) {
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
		case <-s.lc.Stopping():
			log.I.F("grapevine: scheduler stopped")
			return
		case msg := <-s.trigger.Recv():
			if computing[msg.Req] {
				msg.Reply(false)
			} else {
				computing[msg.Req] = true
				go s.computeAsync(msg.Req)
				msg.Reply(true)
			}
		case obs := <-s.computeDone.Recv():
			delete(computing, obs)
		case <-ticker.C:
			s.runAllSync(computing)
		}
	}
}

// TriggerCompute starts an async computation for a single observer.
// Returns immediately. No-op if already computing for that observer.
func (s *Scheduler) TriggerCompute(observerHex string) bool {
	return s.trigger.Call(observerHex)
}

// computeAsync runs a computation and signals completion to the actor.
func (s *Scheduler) computeAsync(observerHex string) {
	if _, err := s.engine.Compute(observerHex); err != nil {
		log.E.F("grapevine: compute failed for %s: %v", observerHex[:12], err)
	}
	s.computeDone.TrySend(observerHex)
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
