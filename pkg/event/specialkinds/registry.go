// Package specialkinds provides a registry for handling special event kinds
// that require custom processing before normal storage/delivery.
// This includes NIP-43 join/leave requests, policy configuration, and
// curating configuration events.
package specialkinds

import (
	"context"

	"next.orly.dev/pkg/nostr/encoders/event"
)

// Result represents the outcome of handling a special kind event.
type Result struct {
	// Handled indicates the event was fully processed by a handler.
	// If true, normal event processing should be skipped.
	Handled bool

	// Continue indicates processing should continue to normal event handling.
	Continue bool

	// Error is set if the handler encountered an error.
	Error error

	// Message is an optional message to include in the OK response.
	Message string

	// SaveEvent indicates the event should still be saved even if handled.
	SaveEvent bool
}

// HandlerContext provides access to connection-specific data needed by handlers.
type HandlerContext struct {
	// AuthedPubkey is the authenticated pubkey for this connection (may be nil).
	AuthedPubkey []byte

	// Remote is the remote address of the connection.
	Remote string

	// ConnectionID uniquely identifies this connection.
	ConnectionID string
}

// Handler processes special event kinds.
type Handler interface {
	// CanHandle returns true if this handler can process the given event.
	// This is typically based on the event kind and possibly tag values.
	CanHandle(ev *event.E) bool

	// Handle processes the event.
	// Returns a Result indicating the outcome.
	Handle(ctx context.Context, ev *event.E, hctx *HandlerContext) Result

	// Name returns a descriptive name for this handler (for logging).
	Name() string
}

// Registry holds registered special kind handlers.
type Registry struct {
	handlers []Handler
}

// NewRegistry creates a new special kind registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make([]Handler, 0),
	}
}

// Register adds a handler to the registry.
// Handlers are checked in registration order.
func (r *Registry) Register(h Handler) {
	r.handlers = append(r.handlers, h)
}

// TryHandle attempts to handle an event with registered handlers.
// Returns (result, true) if a handler processed the event.
// Returns (nil, false) if no handler matched.
func (r *Registry) TryHandle(ctx context.Context, ev *event.E, hctx *HandlerContext) (*Result, bool) {
	for _, h := range r.handlers {
		if h.CanHandle(ev) {
			result := h.Handle(ctx, ev, hctx)
			return &result, true
		}
	}
	return nil, false
}

// ListHandlers returns the names of all registered handlers.
func (r *Registry) ListHandlers() []string {
	names := make([]string, len(r.handlers))
	for i, h := range r.handlers {
		names[i] = h.Name()
	}
	return names
}

// HandlerCount returns the number of registered handlers.
func (r *Registry) HandlerCount() int {
	return len(r.handlers)
}

// =============================================================================
// Handler Function Adapter
// =============================================================================

// HandlerFunc is a function adapter that implements Handler.
type HandlerFunc struct {
	name         string
	canHandleFn  func(*event.E) bool
	handleFn     func(context.Context, *event.E, *HandlerContext) Result
}

// NewHandlerFunc creates a Handler from functions.
func NewHandlerFunc(
	name string,
	canHandle func(*event.E) bool,
	handle func(context.Context, *event.E, *HandlerContext) Result,
) *HandlerFunc {
	return &HandlerFunc{
		name:        name,
		canHandleFn: canHandle,
		handleFn:    handle,
	}
}

// CanHandle implements Handler.
func (h *HandlerFunc) CanHandle(ev *event.E) bool {
	return h.canHandleFn(ev)
}

// Handle implements Handler.
func (h *HandlerFunc) Handle(ctx context.Context, ev *event.E, hctx *HandlerContext) Result {
	return h.handleFn(ctx, ev, hctx)
}

// Name implements Handler.
func (h *HandlerFunc) Name() string {
	return h.name
}

// =============================================================================
// Convenience Result Constructors
// =============================================================================

// Handled returns a Result indicating the event was fully handled.
func Handled(message string) Result {
	return Result{
		Handled: true,
		Message: message,
	}
}

// HandledWithSave returns a Result indicating the event was handled but should also be saved.
func HandledWithSave(message string) Result {
	return Result{
		Handled:   true,
		SaveEvent: true,
		Message:   message,
	}
}

// ContinueProcessing returns a Result indicating normal processing should continue.
func ContinueProcessing() Result {
	return Result{
		Continue: true,
	}
}

// ErrorResult returns a Result with an error.
func ErrorResult(err error) Result {
	return Result{
		Error: err,
	}
}
