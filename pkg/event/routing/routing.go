// Package routing provides event routing services for the ORLY relay.
// It dispatches events to specialized handlers based on event kind.
package routing

import (
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
)

// Action indicates what to do after routing.
type Action int

const (
	// Continue means continue to normal processing.
	Continue Action = iota
	// Handled means event was fully handled, return success.
	Handled
	// Error means an error occurred.
	Error
)

// Result contains the routing decision.
type Result struct {
	Action  Action
	Message string // Success or error message
	Error   error  // Error if Action == Error
}

// ContinueResult returns a result indicating normal processing should continue.
func ContinueResult() Result {
	return Result{Action: Continue}
}

// HandledResult returns a result indicating the event was fully handled.
func HandledResult(msg string) Result {
	return Result{Action: Handled, Message: msg}
}

// ErrorResult returns a result indicating an error occurred.
func ErrorResult(err error) Result {
	return Result{Action: Error, Error: err}
}

// Handler processes a specific event kind.
// authedPubkey is the authenticated pubkey of the connection (may be nil).
type Handler func(ev *event.E, authedPubkey []byte) Result

// KindCheck tests whether an event kind matches a category (e.g., ephemeral).
type KindCheck struct {
	Name    string
	Check   func(kind uint16) bool
	Handler Handler
}

// Router dispatches events to specialized handlers.
type Router interface {
	// Route checks if event should be handled specially.
	Route(ev *event.E, authedPubkey []byte) Result

	// Register adds a handler for a specific kind.
	Register(kind uint16, handler Handler)

	// RegisterKindCheck adds a handler for a kind category.
	RegisterKindCheck(name string, check func(uint16) bool, handler Handler)
}

// DefaultRouter implements Router with a handler registry.
type DefaultRouter struct {
	handlers   map[uint16]Handler
	kindChecks []KindCheck
}

// New creates a new DefaultRouter.
func New() *DefaultRouter {
	return &DefaultRouter{
		handlers:   make(map[uint16]Handler),
		kindChecks: make([]KindCheck, 0),
	}
}

// Register adds a handler for a specific kind.
func (r *DefaultRouter) Register(kind uint16, handler Handler) {
	r.handlers[kind] = handler
}

// RegisterKindCheck adds a handler for a kind category.
func (r *DefaultRouter) RegisterKindCheck(name string, check func(uint16) bool, handler Handler) {
	r.kindChecks = append(r.kindChecks, KindCheck{
		Name:    name,
		Check:   check,
		Handler: handler,
	})
}

// Route checks if event should be handled specially.
func (r *DefaultRouter) Route(ev *event.E, authedPubkey []byte) Result {
	// Check exact kind matches first (higher priority)
	if handler, ok := r.handlers[ev.Kind]; ok {
		return handler(ev, authedPubkey)
	}

	// Check kind property handlers (ephemeral, replaceable, etc.)
	for _, kc := range r.kindChecks {
		if kc.Check(ev.Kind) {
			return kc.Handler(ev, authedPubkey)
		}
	}

	return ContinueResult()
}

// HasHandler returns true if a handler is registered for the given kind.
func (r *DefaultRouter) HasHandler(kind uint16) bool {
	if _, ok := r.handlers[kind]; ok {
		return true
	}
	for _, kc := range r.kindChecks {
		if kc.Check(kind) {
			return true
		}
	}
	return false
}
