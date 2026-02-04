// Package transport provides a manager for pluggable network transports.
package transport

import (
	"context"
	"fmt"
	"sync"

	"lol.mleku.dev/log"

	iface "next.orly.dev/pkg/interfaces/transport"
)

// Manager manages multiple transports and coordinates their lifecycle.
type Manager struct {
	mu         sync.RWMutex
	transports []iface.Transport
}

// NewManager creates a new transport manager.
func NewManager() *Manager {
	return &Manager{}
}

// Add registers a transport with the manager.
func (m *Manager) Add(t iface.Transport) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transports = append(m.transports, t)
}

// StartAll starts all registered transports in order.
// If a transport fails to start, it is logged and skipped.
// Returns an error only if no transports started successfully.
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	started := 0
	for _, t := range m.transports {
		log.I.F("starting transport: %s", t.Name())
		if err := t.Start(ctx); err != nil {
			log.E.F("transport %s failed to start: %v (skipping)", t.Name(), err)
			continue
		}
		log.I.F("transport started: %s", t.Name())
		started++
	}
	if started == 0 {
		return fmt.Errorf("no transports started successfully")
	}
	return nil
}

// StopAll stops all transports in reverse order.
func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var firstErr error
	for i := len(m.transports) - 1; i >= 0; i-- {
		t := m.transports[i]
		log.I.F("stopping transport: %s", t.Name())
		if err := t.Stop(ctx); err != nil {
			log.E.F("failed to stop transport %s: %v", t.Name(), err)
			if firstErr == nil {
				firstErr = err
			}
		} else {
			log.I.F("transport stopped: %s", t.Name())
		}
	}
	return firstErr
}

// Addresses returns all addresses from all transports.
func (m *Manager) Addresses() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var addrs []string
	for _, t := range m.transports {
		addrs = append(addrs, t.Addresses()...)
	}
	return addrs
}
