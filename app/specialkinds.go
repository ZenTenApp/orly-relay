package app

import (
	"context"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/event/specialkinds"
	"next.orly.dev/pkg/protocol/nip43"
)

// registerSpecialKindHandlers registers handlers for special event kinds
// that require custom processing before normal storage/delivery.
func (s *Server) registerSpecialKindHandlers() {
	// NIP-43 Join Request handler - signals that special handling is needed
	s.specialKinds.Register(specialkinds.NewHandlerFunc(
		"nip43-join",
		func(ev *event.E) bool {
			return ev.Kind == nip43.KindJoinRequest
		},
		func(ctx context.Context, ev *event.E, hctx *specialkinds.HandlerContext) specialkinds.Result {
			// Signal to handler that this needs NIP-43 join processing
			// The actual processing happens in the Listener which has the connection context
			return specialkinds.Result{
				Handled: true,
				Message: "nip43-join", // Marker for handler
			}
		},
	))

	// NIP-43 Leave Request handler - signals that special handling is needed
	s.specialKinds.Register(specialkinds.NewHandlerFunc(
		"nip43-leave",
		func(ev *event.E) bool {
			return ev.Kind == nip43.KindLeaveRequest
		},
		func(ctx context.Context, ev *event.E, hctx *specialkinds.HandlerContext) specialkinds.Result {
			return specialkinds.Result{
				Handled: true,
				Message: "nip43-leave", // Marker for handler
			}
		},
	))

	// Curating config handler (kind 30078 with d-tag "curating-config")
	s.specialKinds.Register(specialkinds.NewHandlerFunc(
		"curating-config",
		func(ev *event.E) bool {
			if ev.Kind != acl.CuratingConfigKind {
				return false
			}
			dTag := ev.Tags.GetFirst([]byte("d"))
			return dTag != nil && string(dTag.Value()) == acl.CuratingConfigDTag
		},
		s.handleCuratingConfig,
	))

	// Policy config handler (kind 12345)
	s.specialKinds.Register(specialkinds.NewHandlerFunc(
		"policy-config",
		func(ev *event.E) bool {
			return ev.Kind == kind.PolicyConfig.K
		},
		func(ctx context.Context, ev *event.E, hctx *specialkinds.HandlerContext) specialkinds.Result {
			return specialkinds.Result{
				Handled: true,
				Message: "policy-config", // Marker for handler
			}
		},
	))

}

// handleCuratingConfig handles curating configuration events
func (s *Server) handleCuratingConfig(ctx context.Context, ev *event.E, hctx *specialkinds.HandlerContext) specialkinds.Result {
	if acl.Registry.Type() != "curating" {
		return specialkinds.ContinueProcessing()
	}

	for _, aclInstance := range acl.Registry.ACLs() {
		if aclInstance.Type() == "curating" {
			if curating, ok := aclInstance.(*acl.Curating); ok {
				if err := curating.ProcessConfigEvent(ev); err != nil {
					return specialkinds.ErrorResult(err)
				}
				// Save the event and signal success
				return specialkinds.HandledWithSave("curating configuration updated")
			}
		}
	}

	return specialkinds.ContinueProcessing()
}
