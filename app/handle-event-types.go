package app

import (
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/eventenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/okenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/reason"
	"git.smesh.lol/orly/pkg/event/authorization"
	"git.smesh.lol/orly/pkg/event/routing"
	"git.smesh.lol/orly/pkg/event/validation"
)

// sendValidationError sends an appropriate OK response for a validation failure.
func (l *Listener) sendValidationError(env eventenvelope.I, result validation.Result) error {
	return writeReasonedOK(l, env.Id(), result.PrefixedMessage())
}

// sendAuthorizationDenied sends an appropriate OK response for an authorization denial.
func (l *Listener) sendAuthorizationDenied(env eventenvelope.I, decision authorization.Decision) error {
	if decision.RequireAuth {
		_, detail, found := reason.Parse(decision.DenyReason)
		if !found {
			detail = decision.DenyReason
		}
		return okenvelope.NewFrom(env.Id(), false, reason.AuthRequired.F("%s", detail)).Write(l)
	}
	return writeReasonedOK(l, env.Id(), decision.DenyReason)
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
	return writeReasonedOK(l, env.Id(), reason.Ensure(msg, reason.Error))
}

// sendProcessingBlocked sends an appropriate OK response for a blocked event.
func (l *Listener) sendProcessingBlocked(env eventenvelope.I, msg string) error {
	return writeReasonedOK(l, env.Id(), msg)
}

// sendRawValidationError sends an OK response for raw JSON validation failure (before unmarshal).
// Since we don't have an event ID at this point, we pass nil.
func (l *Listener) sendRawValidationError(result validation.Result) error {
	return writeReasonedOK(l, nil, result.PrefixedMessage())
}
