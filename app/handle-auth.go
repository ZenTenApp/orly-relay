package app

import (
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/authenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/reason"
	"git.mleku.dev/mleku/nostr/protocol/auth"
)

// zeroEventID is used for OK responses when we cannot parse the event ID
var zeroEventID = make([]byte, 32)

func (l *Listener) HandleAuth(b []byte) (err error) {
	var rem []byte
	env := authenvelope.NewResponse()
	if rem, err = env.Unmarshal(b); chk.E(err) {
		// NIP-42: AUTH messages MUST be answered with an OK message
		// For parse failures, use zero event ID
		log.E.F("%s AUTH unmarshal failed: %v", l.remote, err)
		if writeErr := okenvelope.NewFrom(
			zeroEventID, false, reason.Error.F("failed to parse auth event: %s", err),
		).Write(l); chk.E(writeErr) {
			return writeErr
		}
		return
	}
	defer func() {
		if env != nil && env.Event != nil {
			env.Event.Free()
		}
	}()
	if len(rem) > 0 {
		log.I.F("extra '%s'", rem)
	}
	var valid bool
	if valid, err = auth.Validate(
		env.Event, l.challenge.Load(),
		l.WebSocketURL(l.req),
	); err != nil {
		e := err.Error()
		if err = Ok.Error(l, env, e); chk.E(err) {
			return
		}
		return
	} else if !valid {
		if err = Ok.Error(
			l, env, "auth response event is invalid",
		); chk.E(err) {
			return
		}
		return
	} else {
		if err = okenvelope.NewFrom(
			env.Event.ID, true,
		).Write(l); chk.E(err) {
			return
		}
		log.D.F(
			"%s authed to pubkey %0x", l.remote,
			env.Event.Pubkey,
		)
		l.authedPubkey.Store(env.Event.Pubkey)

		// Check if this is a first-time user and create welcome note
		go l.handleFirstTimeUser(env.Event.Pubkey)
	}
	return
}

// handleFirstTimeUser checks if user is logging in for first time and creates welcome note
func (l *Listener) handleFirstTimeUser(pubkey []byte) {
	// Check if this is a first-time user
	isFirstTime, err := l.Server.DB.IsFirstTimeUser(pubkey)
	if err != nil {
		log.E.F("failed to check first-time user status: %v", err)
		return
	}

	if !isFirstTime {
		return // Not a first-time user
	}

	// Get payment processor to create welcome note
	if l.Server.paymentProcessor != nil {
		// Set the dashboard URL based on the current HTTP request
		dashboardURL := l.Server.DashboardURL(l.req)
		l.Server.paymentProcessor.SetDashboardURL(dashboardURL)

		if err := l.Server.paymentProcessor.CreateWelcomeNote(pubkey); err != nil {
			log.E.F("failed to create welcome note for first-time user: %v", err)
		}
	}
}
