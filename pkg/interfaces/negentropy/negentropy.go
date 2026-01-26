// Package negentropy defines the interface for NIP-77 negentropy operations.
package negentropy

import (
	"context"

	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
)

// ClientSession represents an active client negentropy session.
type ClientSession struct {
	SubscriptionID string
	ConnectionID   string
	CreatedAt      int64
	LastActivity   int64
	RoundCount     int32
}

// Handler defines the interface for handling NIP-77 negentropy messages.
// This interface is implemented by both the gRPC client and the embedded handler.
type Handler interface {
	// HandleNegOpen processes a NEG-OPEN message from a client.
	// Returns: message, haveIDs, needIDs, complete, errorStr, error
	HandleNegOpen(ctx context.Context, connectionID, subscriptionID string, filter *commonv1.Filter, initialMessage []byte) ([]byte, [][]byte, [][]byte, bool, string, error)

	// HandleNegMsg processes a NEG-MSG message from a client.
	// Returns: message, haveIDs, needIDs, complete, errorStr, error
	HandleNegMsg(ctx context.Context, connectionID, subscriptionID string, message []byte) ([]byte, [][]byte, [][]byte, bool, string, error)

	// HandleNegClose processes a NEG-CLOSE message from a client.
	HandleNegClose(ctx context.Context, connectionID, subscriptionID string) error

	// ListSessions returns active client negentropy sessions.
	ListSessions(ctx context.Context) ([]*ClientSession, error)

	// CloseSession forcefully closes a client session.
	CloseSession(ctx context.Context, connectionID, subscriptionID string) error

	// Ready returns a channel that closes when the handler is ready.
	Ready() <-chan struct{}

	// Close cleans up resources.
	Close() error
}
