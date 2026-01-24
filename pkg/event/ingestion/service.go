// Package ingestion provides a service for orchestrating the event processing pipeline.
// It coordinates validation, special kind handling, authorization, routing, and processing.
package ingestion

import (
	"context"
	"fmt"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"next.orly.dev/pkg/event/authorization"
	"next.orly.dev/pkg/event/processing"
	"next.orly.dev/pkg/event/routing"
	"next.orly.dev/pkg/event/specialkinds"
	"next.orly.dev/pkg/event/validation"
)

// SprocketChecker checks events against the sprocket (external filter).
type SprocketChecker interface {
	IsEnabled() bool
	IsDisabled() bool
	IsRunning() bool
	ProcessEvent(*event.E) (*SprocketResponse, error)
}

// SprocketResponse is the response from sprocket processing.
type SprocketResponse struct {
	Action string // "accept", "reject", "shadowReject"
	Msg    string
}

// ConnectionContext provides connection-specific data for event processing.
type ConnectionContext struct {
	// AuthedPubkey is the authenticated pubkey for this connection (may be nil).
	AuthedPubkey []byte

	// Remote is the remote address of the connection.
	Remote string

	// ConnectionID uniquely identifies this connection.
	ConnectionID string
}

// Result represents the outcome of event ingestion.
type Result struct {
	// Accepted indicates the event was accepted for processing.
	Accepted bool

	// Saved indicates the event was saved to the database.
	Saved bool

	// Message is an optional message for the OK response.
	Message string

	// RequireAuth indicates authentication is required.
	RequireAuth bool

	// Error contains any error that occurred.
	Error error
}

// Accepted returns a successful Result.
func Accepted(message string) Result {
	return Result{Accepted: true, Saved: true, Message: message}
}

// AcceptedNotSaved returns a Result where the event was accepted but not saved.
func AcceptedNotSaved(message string) Result {
	return Result{Accepted: true, Saved: false, Message: message}
}

// Rejected returns a rejection Result.
func Rejected(message string) Result {
	return Result{Accepted: false, Message: message}
}

// AuthRequired returns a Result indicating authentication is required.
func AuthRequired(message string) Result {
	return Result{Accepted: false, RequireAuth: true, Message: message}
}

// Errored returns a Result with an error.
func Errored(err error) Result {
	return Result{Accepted: false, Error: err}
}

// Config configures the ingestion service.
type Config struct {
	// SprocketChecker is the optional sprocket checker.
	SprocketChecker SprocketChecker

	// SpecialKinds is the registry for special kind handlers.
	SpecialKinds *specialkinds.Registry

	// ACLMode is the current ACL mode (used for NIP-70 validation).
	ACLMode func() string

	// DeleteHandler handles deletion events.
	DeleteHandler DeleteHandler
}

// DeleteHandler processes delete events (kind 5).
type DeleteHandler interface {
	// HandleDelete processes a delete event after it's been saved.
	HandleDelete(ctx context.Context, ev *event.E) error
}

// Service orchestrates the event ingestion pipeline.
type Service struct {
	validator     *validation.Service
	authorizer    *authorization.Service
	router        *routing.DefaultRouter
	processor     *processing.Service
	sprocket      SprocketChecker
	specialKinds  *specialkinds.Registry
	aclMode       func() string
	deleteHandler DeleteHandler
}

// NewService creates a new ingestion service.
func NewService(
	validator *validation.Service,
	authorizer *authorization.Service,
	router *routing.DefaultRouter,
	processor *processing.Service,
	cfg Config,
) *Service {
	return &Service{
		validator:     validator,
		authorizer:    authorizer,
		router:        router,
		processor:     processor,
		sprocket:      cfg.SprocketChecker,
		specialKinds:  cfg.SpecialKinds,
		aclMode:       cfg.ACLMode,
		deleteHandler: cfg.DeleteHandler,
	}
}

// Ingest processes an event through the full ingestion pipeline.
// Returns a Result indicating the outcome.
func (s *Service) Ingest(ctx context.Context, ev *event.E, connCtx *ConnectionContext) Result {
	// Stage 1: Event validation (ID, timestamp, signature)
	if result := s.validator.ValidateEvent(ev); !result.Valid {
		return Rejected(result.Msg)
	}

	// Stage 2: Sprocket check (if enabled)
	if s.sprocket != nil && s.sprocket.IsEnabled() {
		if s.sprocket.IsDisabled() {
			return Rejected("sprocket disabled - events rejected until sprocket is restored")
		}
		if !s.sprocket.IsRunning() {
			return Rejected("sprocket not running - events rejected until sprocket starts")
		}

		response, err := s.sprocket.ProcessEvent(ev)
		if err != nil {
			return Errored(fmt.Errorf("sprocket processing failed: %w", err))
		}

		switch response.Action {
		case "accept":
			// Continue processing
		case "reject":
			return Rejected(response.Msg)
		case "shadowReject":
			// Accept but don't save
			return AcceptedNotSaved("")
		}
	}

	// Stage 3: Special kind handling
	if s.specialKinds != nil {
		hctx := &specialkinds.HandlerContext{
			AuthedPubkey: connCtx.AuthedPubkey,
			Remote:       connCtx.Remote,
			ConnectionID: connCtx.ConnectionID,
		}
		if result, handled := s.specialKinds.TryHandle(ctx, ev, hctx); handled {
			if result.Error != nil {
				return Errored(result.Error)
			}
			if result.Handled {
				if result.SaveEvent {
					// Handler wants the event saved
					procResult := s.processor.Process(ctx, ev)
					if procResult.Error != nil {
						return Errored(procResult.Error)
					}
					return Accepted(result.Message)
				}
				return AcceptedNotSaved(result.Message)
			}
			// result.Continue - fall through to normal processing
		}
	}

	// Stage 4: Authorization check
	decision := s.authorizer.Authorize(ev, connCtx.AuthedPubkey, connCtx.Remote, ev.Kind)
	if !decision.Allowed {
		if decision.RequireAuth {
			return AuthRequired(decision.DenyReason)
		}
		return Rejected(decision.DenyReason)
	}

	// Stage 5: NIP-70 protected tag validation (only when ACL is active)
	if s.aclMode != nil && s.aclMode() != "none" {
		if result := s.validator.ValidateProtectedTag(ev, connCtx.AuthedPubkey); !result.Valid {
			return Rejected(result.Msg)
		}
	}

	// Stage 6: Routing (ephemeral events, etc.)
	if routeResult := s.router.Route(ev, connCtx.AuthedPubkey); routeResult.Action != routing.Continue {
		if routeResult.Action == routing.Handled {
			return AcceptedNotSaved(routeResult.Message)
		}
		if routeResult.Action == routing.Error {
			return Errored(fmt.Errorf("routing error: %s", routeResult.Message))
		}
	}

	// Stage 7: Processing (save, hooks, delivery)
	procResult := s.processor.Process(ctx, ev)
	if procResult.Blocked {
		return Rejected(procResult.BlockMsg)
	}
	if procResult.Error != nil {
		return Errored(procResult.Error)
	}

	// Stage 8: Delete event post-processing
	const kindEventDeletion = 5
	if ev.Kind == kindEventDeletion && s.deleteHandler != nil {
		if err := s.deleteHandler.HandleDelete(ctx, ev); err != nil {
			// Log but don't fail - the delete event is already saved
			// Return success since the event was stored
		}
	}

	return Accepted("")
}

// IngestWithRawValidation includes raw JSON validation before unmarshaling.
// Use this when the event hasn't been validated yet.
func (s *Service) IngestWithRawValidation(ctx context.Context, rawJSON []byte, ev *event.E, connCtx *ConnectionContext) Result {
	// Stage 0: Raw JSON validation
	if result := s.validator.ValidateRawJSON(rawJSON); !result.Valid {
		return Rejected(result.Msg)
	}

	return s.Ingest(ctx, ev, connCtx)
}
