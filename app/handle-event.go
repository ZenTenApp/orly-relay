package app

import (
	"context"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/event/routing"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/authenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/noticeenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/reason"
	"next.orly.dev/pkg/protocol/nip43"
)

func (l *Listener) HandleEvent(msg []byte) (err error) {
	log.D.F("HandleEvent: START handling event: %s", msg)

	// 1. Raw JSON validation (before unmarshal) - use validation service
	if result := l.eventValidator.ValidateRawJSON(msg); !result.Valid {
		log.W.F("HandleEvent: rejecting event with validation error: %s", result.Msg)
		// Send NOTICE to alert client developers about the issue
		if noticeErr := noticeenvelope.NewFrom(result.Msg).Write(l); noticeErr != nil {
			log.E.F("failed to send NOTICE for validation error: %v", noticeErr)
		}
		// Send OK false with the error message
		if err = l.sendRawValidationError(result); chk.E(err) {
			return
		}
		return nil
	}

	// decode the envelope
	env := eventenvelope.NewSubmission()
	log.I.F("HandleEvent: received event message length: %d", len(msg))
	if msg, err = env.Unmarshal(msg); chk.E(err) {
		log.E.F("HandleEvent: failed to unmarshal event: %v", err)
		return
	}
	log.I.F(
		"HandleEvent: successfully unmarshaled event, kind: %d, pubkey: %s, id: %0x",
		env.E.Kind, hex.Enc(env.E.Pubkey), env.E.ID,
	)
	defer func() {
		if env != nil && env.E != nil {
			env.E.Free()
		}
	}()

	if len(msg) > 0 {
		log.I.F("extra '%s'", msg)
	}

	// Check if sprocket is enabled and process event through it
	if l.sprocketManager != nil && l.sprocketManager.IsEnabled() {
		if l.sprocketManager.IsDisabled() {
			// Sprocket is disabled due to failure - reject all events
			log.W.F("sprocket is disabled, rejecting event %0x", env.E.ID)
			if err = Ok.Error(
				l, env,
				"sprocket disabled - events rejected until sprocket is restored",
			); chk.E(err) {
				return
			}
			return
		}

		if !l.sprocketManager.IsRunning() {
			// Sprocket is enabled but not running - reject all events
			log.W.F(
				"sprocket is enabled but not running, rejecting event %0x",
				env.E.ID,
			)
			if err = Ok.Error(
				l, env,
				"sprocket not running - events rejected until sprocket starts",
			); chk.E(err) {
				return
			}
			return
		}

		// Process event through sprocket
		response, sprocketErr := l.sprocketManager.ProcessEvent(env.E)
		if chk.E(sprocketErr) {
			log.E.F("sprocket processing failed: %v", sprocketErr)
			if err = Ok.Error(
				l, env, "sprocket processing failed",
			); chk.E(err) {
				return
			}
			return
		}

		// Handle sprocket response
		switch response.Action {
		case "accept":
			// Continue with normal processing
			log.D.F("sprocket accepted event %0x", env.E.ID)
		case "reject":
			// Return OK false with message
			if err = okenvelope.NewFrom(
				env.Id(), false,
				reason.Error.F(response.Msg),
			).Write(l); chk.E(err) {
				return
			}
			return
		case "shadowReject":
			// Return OK true but abort processing
			if err = Ok.Ok(l, env, ""); chk.E(err) {
				return
			}
			log.D.F("sprocket shadow rejected event %0x", env.E.ID)
			return
		default:
			log.W.F("unknown sprocket action: %s", response.Action)
			// Default to accept for unknown actions
		}
	}

	// Event validation (ID, timestamp, signature) - use validation service
	if result := l.eventValidator.ValidateEvent(env.E); !result.Valid {
		if err = l.sendValidationError(env, result); chk.E(err) {
			return
		}
		return
	}

	// Handle NIP-43 special events before ACL checks
	switch env.E.Kind {
	case nip43.KindJoinRequest:
		// Process join request and return early
		if err = l.HandleNIP43JoinRequest(env.E); chk.E(err) {
			log.E.F("failed to process NIP-43 join request: %v", err)
		}
		return
	case nip43.KindLeaveRequest:
		// Process leave request and return early
		if err = l.HandleNIP43LeaveRequest(env.E); chk.E(err) {
			log.E.F("failed to process NIP-43 leave request: %v", err)
		}
		return
	case kind.PolicyConfig.K:
		// Handle policy configuration update events (kind 12345)
		// Only policy admins can update policy configuration
		if err = l.HandlePolicyConfigUpdate(env.E); chk.E(err) {
			log.E.F("failed to process policy config update: %v", err)
			if err = Ok.Error(l, env, err.Error()); chk.E(err) {
				return
			}
			return
		}
		// Send OK response
		if err = Ok.Ok(l, env, "policy configuration updated"); chk.E(err) {
			return
		}
		return
	case kind.FollowList.K:
		// Check if this is a follow list update from a policy admin
		// If so, refresh the policy follows cache immediately
		if l.IsPolicyAdminFollowListEvent(env.E) {
			// Process the follow list update (async, don't block)
			go func() {
				if updateErr := l.HandlePolicyAdminFollowListUpdate(env.E); updateErr != nil {
					log.W.F("failed to update policy follows from admin follow list: %v", updateErr)
				}
			}()
		}
		// Continue with normal follow list processing (store the event)
	}

	// Authorization check (policy + ACL) - use authorization service
	decision := l.eventAuthorizer.Authorize(env.E, l.authedPubkey.Load(), l.remote, env.E.Kind)
	if !decision.Allowed {
		log.D.F("HandleEvent: authorization denied: %s (requireAuth=%v)", decision.DenyReason, decision.RequireAuth)
		if decision.RequireAuth {
			// Send OK false with reason
			if err = okenvelope.NewFrom(
				env.Id(), false,
				reason.AuthRequired.F(decision.DenyReason),
			).Write(l); chk.E(err) {
				return
			}
			// Send AUTH challenge
			if err = authenvelope.NewChallengeWith(l.challenge.Load()).Write(l); chk.E(err) {
				return
			}
		} else {
			// Send OK false with blocked reason
			if err = Ok.Blocked(l, env, decision.DenyReason); chk.E(err) {
				return
			}
		}
		return
	}
	log.I.F("HandleEvent: authorized with access level %s", decision.AccessLevel)

	// Route special event kinds (ephemeral, etc.) - use routing service
	if routeResult := l.eventRouter.Route(env.E, l.authedPubkey.Load()); routeResult.Action != routing.Continue {
		if routeResult.Action == routing.Handled {
			// Event fully handled by router, send OK and return
			log.D.F("event %0x handled by router", env.E.ID)
			if err = Ok.Ok(l, env, routeResult.Message); chk.E(err) {
				return
			}
			return
		} else if routeResult.Action == routing.Error {
			// Router encountered an error
			if err = l.sendRoutingError(env, routeResult); chk.E(err) {
				return
			}
			return
		}
	}
	log.D.F("processing regular event %0x (kind %d)", env.E.ID, env.E.Kind)

	// NIP-70 protected tag validation - use validation service
	if acl.Registry.Active.Load() != "none" {
		if result := l.eventValidator.ValidateProtectedTag(env.E, l.authedPubkey.Load()); !result.Valid {
			if err = l.sendValidationError(env, result); chk.E(err) {
				return
			}
			return
		}
	}
	// Handle delete events specially - save first, then process deletions
	if env.E.Kind == kind.EventDeletion.K {
		log.I.F("processing delete event %0x", env.E.ID)

		// Save and deliver using processing service
		result := l.eventProcessor.Process(context.Background(), env.E)
		if result.Blocked {
			if err = Ok.Error(l, env, result.BlockMsg); chk.E(err) {
				return
			}
			return
		}
		if result.Error != nil {
			chk.E(result.Error)
			return
		}

		// Process deletion targets (remove referenced events)
		if err = l.HandleDelete(env); err != nil {
			log.W.F("HandleDelete failed for event %0x: %v", env.E.ID, err)
		}

		if err = Ok.Ok(l, env, ""); chk.E(err) {
			return
		}
		log.D.F("processed delete event %0x", env.E.ID)
		return
	}
	// Process event: save, run hooks, and deliver to subscribers
	result := l.eventProcessor.Process(context.Background(), env.E)
	if result.Blocked {
		if err = Ok.Error(l, env, result.BlockMsg); chk.E(err) {
			return
		}
		return
	}
	if result.Error != nil {
		chk.E(result.Error)
		return
	}

	// Send success response
	if err = Ok.Ok(l, env, ""); chk.E(err) {
		return
	}
	log.D.F("saved event %0x", env.E.ID)
	return
}

// isPeerRelayPubkey checks if the given pubkey belongs to a peer relay
func (l *Listener) isPeerRelayPubkey(pubkey []byte) bool {
	if l.syncManager == nil {
		return false
	}

	peerPubkeyHex := hex.Enc(pubkey)

	// Check if this pubkey matches any of our configured peer relays' NIP-11 pubkeys
	for _, peerURL := range l.syncManager.GetPeers() {
		if l.syncManager.IsAuthorizedPeer(peerURL, peerPubkeyHex) {
			return true
		}
	}

	return false
}
