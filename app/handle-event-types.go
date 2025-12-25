package app

import (
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/reason"
	"next.orly.dev/pkg/event/authorization"
	"next.orly.dev/pkg/event/routing"
	"next.orly.dev/pkg/event/validation"
)

// sendValidationError sends an appropriate OK response for a validation failure.
func (l *Listener) sendValidationError(env eventenvelope.I, result validation.Result) error {
	var r []byte
	switch result.Code {
	case validation.ReasonBlocked:
		r = reason.Blocked.F(result.Msg)
	case validation.ReasonInvalid:
		r = reason.Invalid.F(result.Msg)
	case validation.ReasonError:
		r = reason.Error.F(result.Msg)
	default:
		r = reason.Error.F(result.Msg)
	}
	return okenvelope.NewFrom(env.Id(), false, r).Write(l)
}

// sendAuthorizationDenied sends an appropriate OK response for an authorization denial.
func (l *Listener) sendAuthorizationDenied(env eventenvelope.I, decision authorization.Decision) error {
	var r []byte
	if decision.RequireAuth {
		r = reason.AuthRequired.F(decision.DenyReason)
	} else {
		r = reason.Blocked.F(decision.DenyReason)
	}
	return okenvelope.NewFrom(env.Id(), false, r).Write(l)
}

// sendRoutingError sends an appropriate OK response for a routing error.
func (l *Listener) sendRoutingError(env eventenvelope.I, result routing.Result) error {
	if result.Error != nil {
		return okenvelope.NewFrom(env.Id(), false, reason.Error.F(result.Error.Error())).Write(l)
	}
	return nil
}

// sendProcessingError sends an appropriate OK response for a processing failure.
func (l *Listener) sendProcessingError(env eventenvelope.I, msg string) error {
	return okenvelope.NewFrom(env.Id(), false, reason.Error.F(msg)).Write(l)
}

// sendProcessingBlocked sends an appropriate OK response for a blocked event.
func (l *Listener) sendProcessingBlocked(env eventenvelope.I, msg string) error {
	return okenvelope.NewFrom(env.Id(), false, reason.Blocked.F(msg)).Write(l)
}

// sendRawValidationError sends an OK response for raw JSON validation failure (before unmarshal).
// Since we don't have an event ID at this point, we pass nil.
func (l *Listener) sendRawValidationError(result validation.Result) error {
	var r []byte
	switch result.Code {
	case validation.ReasonBlocked:
		r = reason.Blocked.F(result.Msg)
	case validation.ReasonInvalid:
		r = reason.Invalid.F(result.Msg)
	case validation.ReasonError:
		r = reason.Error.F(result.Msg)
	default:
		r = reason.Error.F(result.Msg)
	}
	return okenvelope.NewFrom(nil, false, r).Write(l)
}
