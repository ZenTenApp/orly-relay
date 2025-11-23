package app

import (
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/reason"
)

// OK represents a function that processes events or operations, using provided
// parameters to generate formatted messages and return errors if any issues
// occur during processing.
type OK func(
	l *Listener, env eventenvelope.I, format string, params ...any,
) (err error)

// OKs provides a collection of handler functions for managing different types
// of operational outcomes, each corresponding to specific error or status
// conditions such as authentication requirements, rate limiting, and invalid
// inputs.
type OKs struct {
	Ok           OK
	AuthRequired OK
	PoW          OK
	Duplicate    OK
	Blocked      OK
	RateLimited  OK
	Invalid      OK
	Error        OK
	Unsupported  OK
	Restricted   OK
}

// Ok provides a collection of handler functions for managing different types of
// operational outcomes, each corresponding to specific error or status
// conditions such as authentication requirements, rate limiting, and invalid
// inputs.
var Ok = OKs{
	Ok: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), true, []byte{},
		).Write(l)
	},
	AuthRequired: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.AuthRequired.F(format, params...),
		).Write(l)
	},
	PoW: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.PoW.F(format, params...),
		).Write(l)
	},
	Duplicate: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.Duplicate.F(format, params...),
		).Write(l)
	},
	Blocked: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.Blocked.F(format, params...),
		).Write(l)
	},
	RateLimited: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.RateLimited.F(format, params...),
		).Write(l)
	},
	Invalid: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.Invalid.F(format, params...),
		).Write(l)
	},
	Error: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.Error.F(format, params...),
		).Write(l)
	},
	Unsupported: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.Unsupported.F(format, params...),
		).Write(l)
	},
	Restricted: func(
		l *Listener, env eventenvelope.I, format string,
		params ...any,
	) (err error) {
		return okenvelope.NewFrom(
			env.Id(), false, reason.Restricted.F(format, params...),
		).Write(l)
	},
}
