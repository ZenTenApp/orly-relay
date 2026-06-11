// Package transport provides a manager for pluggable network transports.
package transport

import (
	"context"
	"fmt"

	"git.smesh.lol/orly/pkg/lol/log"

	iface "git.smesh.lol/orly/pkg/interfaces/transport"
)

// --- Actor request/response types ---

type tmAddReq struct {
	t    iface.Transport
	done chan struct{}
}

type tmStartAllReq struct {
	ctx  context.Context
	resp chan error
}

type tmStopAllReq struct {
	ctx  context.Context
	resp chan error
}

type tmAddressesReq struct {
	resp chan []string
}

// Manager manages multiple transports and coordinates their lifecycle.
// All mutable state is owned by the actor goroutine.
type Manager struct {
	addCh       chan tmAddReq
	startAllCh  chan tmStartAllReq
	stopAllCh   chan tmStopAllReq
	addressesCh chan tmAddressesReq
	stop        chan struct{}
	done        chan struct{}
}

// NewManager creates a new transport manager.
func NewManager() *Manager {
	m := &Manager{
		addCh:       make(chan tmAddReq),
		startAllCh:  make(chan tmStartAllReq),
		stopAllCh:   make(chan tmStopAllReq),
		addressesCh: make(chan tmAddressesReq),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	go m.actorLoop()
	return m
}

func (m *Manager) actorLoop() {
	defer close(m.done)

	var transports []iface.Transport

	for {
		select {
		case <-m.stop:
			return
		case req := <-m.addCh:
			transports = append(transports, req.t)
			close(req.done)
		case req := <-m.startAllCh:
			started := 0
			for _, t := range transports {
				log.I.F("starting transport: %s", t.Name())
				if err := t.Start(req.ctx); err != nil {
					log.E.F("transport %s failed to start: %v (skipping)", t.Name(), err)
					continue
				}
				log.I.F("transport started: %s", t.Name())
				started++
			}
			if started == 0 {
				req.resp <- fmt.Errorf("no transports started successfully")
			} else {
				req.resp <- nil
			}
		case req := <-m.stopAllCh:
			var firstErr error
			for i := len(transports) - 1; i >= 0; i-- {
				t := transports[i]
				log.I.F("stopping transport: %s", t.Name())
				if err := t.Stop(req.ctx); err != nil {
					log.E.F("failed to stop transport %s: %v", t.Name(), err)
					if firstErr == nil {
						firstErr = err
					}
				} else {
					log.I.F("transport stopped: %s", t.Name())
				}
			}
			req.resp <- firstErr
		case req := <-m.addressesCh:
			var addrs []string
			for _, t := range transports {
				addrs = append(addrs, t.Addresses()...)
			}
			req.resp <- addrs
		}
	}
}

// Shutdown stops the actor goroutine.
func (m *Manager) Shutdown() {
	close(m.stop)
	<-m.done
}

// Add registers a transport with the manager.
// Synchronous: blocks until the actor has processed the add.
func (m *Manager) Add(t iface.Transport) {
	done := make(chan struct{})
	m.addCh <- tmAddReq{t: t, done: done}
	<-done
}

// StartAll starts all registered transports in order.
func (m *Manager) StartAll(ctx context.Context) error {
	req := tmStartAllReq{ctx: ctx, resp: make(chan error, 1)}
	m.startAllCh <- req
	return <-req.resp
}

// StopAll stops all transports in reverse order.
func (m *Manager) StopAll(ctx context.Context) error {
	req := tmStopAllReq{ctx: ctx, resp: make(chan error, 1)}
	m.stopAllCh <- req
	return <-req.resp
}

// Addresses returns all addresses from all transports.
func (m *Manager) Addresses() []string {
	req := tmAddressesReq{resp: make(chan []string, 1)}
	m.addressesCh <- req
	return <-req.resp
}
