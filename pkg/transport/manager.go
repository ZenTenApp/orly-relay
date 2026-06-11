// Package transport provides a manager for pluggable network transports.
package transport

import (
	"context"
	"fmt"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/log"

	iface "git.smesh.lol/orly/pkg/interfaces/transport"
)

type tmStartAllArgs struct {
	Ctx context.Context
}

type tmStopAllArgs struct {
	Ctx context.Context
}

// Manager manages multiple transports and coordinates their lifecycle.
// All mutable state is owned by the actor goroutine.
type Manager struct {
	add       actor.Proc[iface.Transport]
	startAll  actor.Func[tmStartAllArgs, error]
	stopAll   actor.Func[tmStopAllArgs, error]
	addresses actor.Query[[]string]
	actor.Lifecycle
}

// NewManager creates a new transport manager.
func NewManager() *Manager {
	m := &Manager{
		add:       actor.NewProc[iface.Transport](),
		startAll:  actor.NewFunc[tmStartAllArgs, error](),
		stopAll:   actor.NewFunc[tmStopAllArgs, error](),
		addresses: actor.NewQuery[[]string](),
		Lifecycle: actor.NewLifecycle(),
	}
	actor.Go(m.Lifecycle, m.actorLoop)
	return m
}

func (m *Manager) actorLoop() {
	var transports []iface.Transport

	for {
		select {
		case <-m.Stopping():
			return
		case msg := <-m.add.Recv():
			transports = append(transports, msg.Req)
			msg.Done()
		case msg := <-m.startAll.Recv():
			started := 0
			for _, t := range transports {
				log.I.F("starting transport: %s", t.Name())
				if err := t.Start(msg.Req.Ctx); err != nil {
					log.E.F("transport %s failed to start: %v (skipping)", t.Name(), err)
					continue
				}
				log.I.F("transport started: %s", t.Name())
				started++
			}
			if started == 0 {
				msg.Reply(fmt.Errorf("no transports started successfully"))
			} else {
				msg.Reply(nil)
			}
		case msg := <-m.stopAll.Recv():
			var firstErr error
			for i := len(transports) - 1; i >= 0; i-- {
				t := transports[i]
				log.I.F("stopping transport: %s", t.Name())
				if err := t.Stop(msg.Req.Ctx); err != nil {
					log.E.F("failed to stop transport %s: %v", t.Name(), err)
					if firstErr == nil {
						firstErr = err
					}
				} else {
					log.I.F("transport stopped: %s", t.Name())
				}
			}
			msg.Reply(firstErr)
		case msg := <-m.addresses.Recv():
			var addrs []string
			for _, t := range transports {
				addrs = append(addrs, t.Addresses()...)
			}
			msg.Reply(addrs)
		}
	}
}

// Shutdown stops the actor goroutine.
func (m *Manager) Shutdown() {
	m.Stop()
}

// Add registers a transport with the manager.
// Synchronous: blocks until the actor has processed the add.
func (m *Manager) Add(t iface.Transport) {
	m.add.Call(t)
}

// StartAll starts all registered transports in order.
func (m *Manager) StartAll(ctx context.Context) error {
	return m.startAll.Call(tmStartAllArgs{Ctx: ctx})
}

// StopAll stops all transports in reverse order.
func (m *Manager) StopAll(ctx context.Context) error {
	return m.stopAll.Call(tmStopAllArgs{Ctx: ctx})
}

// Addresses returns all addresses from all transports.
func (m *Manager) Addresses() []string {
	return m.addresses.Call()
}
