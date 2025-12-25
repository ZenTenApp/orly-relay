package routing

import (
	"context"

	"git.mleku.dev/mleku/nostr/encoders/event"
)

// DeleteProcessor handles event deletion operations.
type DeleteProcessor interface {
	// SaveDeleteEvent saves the delete event itself.
	SaveDeleteEvent(ctx context.Context, ev *event.E) error
	// ProcessDeletion removes the target events.
	ProcessDeletion(ctx context.Context, ev *event.E) error
	// DeliverEvent sends the delete event to subscribers.
	DeliverEvent(ev *event.E)
}

// MakeDeleteHandler creates a handler for delete events (kind 5).
// Delete events:
// - Save the delete event itself first
// - Process target event deletions
// - Deliver the delete event to subscribers
func MakeDeleteHandler(processor DeleteProcessor) Handler {
	return func(ev *event.E, authedPubkey []byte) Result {
		ctx := context.Background()

		// Save delete event first
		if err := processor.SaveDeleteEvent(ctx, ev); err != nil {
			return ErrorResult(err)
		}

		// Process the deletion (remove target events)
		if err := processor.ProcessDeletion(ctx, ev); err != nil {
			// Log but don't fail - delete event was saved
			// Some targets may not exist or may be owned by others
		}

		// Deliver the delete event to subscribers
		cloned := ev.Clone()
		go processor.DeliverEvent(cloned)

		return HandledResult("")
	}
}

// IsDeleteKind returns true if the kind is a delete event (kind 5).
func IsDeleteKind(k uint16) bool {
	return k == 5
}
