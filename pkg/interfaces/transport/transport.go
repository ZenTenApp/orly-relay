// Package transport defines the interface for pluggable network transports.
package transport

import "context"

// Transport represents a network transport that serves the relay.
type Transport interface {
	// Name returns the transport identifier (e.g., "tcp", "tls", "tor").
	Name() string
	// Start begins accepting connections through this transport.
	Start(ctx context.Context) error
	// Stop gracefully shuts down the transport.
	Stop(ctx context.Context) error
	// Addresses returns the addresses this transport is reachable on.
	Addresses() []string
}
