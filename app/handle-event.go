package app

import (
	"context"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/authenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/eventenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/noticeenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/okenvelope"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/reason"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/event/ingestion"
	"git.smesh.lol/orly/pkg/protocol/nip43"
)

// HandleEvent processes incoming EVENT messages.
// This is a thin protocol adapter that delegates to the ingestion service.
func (l *Listener) HandleEvent(msg []byte) (err error) {
	log.T.F("HandleEvent: START handling event: %s", string(msg[:min(200, len(msg))]))

	// Stage 1: Raw JSON validation (before unmarshal)
	if result := l.eventValidator.ValidateRawJSON(msg); !result.Valid {
		log.W.F("HandleEvent: rejecting event with validation error: %s", result.Msg)
		if noticeErr := noticeenvelope.NewFrom(result.Msg).Write(l); noticeErr != nil {
			log.E.F("failed to send NOTICE for validation error: %v", noticeErr)
		}
		if err = l.sendRawValidationError(result); chk.E(err) {
			return
		}
		return nil
	}

	// Stage 2: Unmarshal the envelope
	env := eventenvelope.NewSubmission()
	if msg, err = env.Unmarshal(msg); chk.E(err) {
		log.E.F("HandleEvent: failed to unmarshal event: %v", err)
		return
	}
	log.D.F("HandleEvent: unmarshaled event, kind: %d, pubkey: %s, id: %0x",
		env.E.Kind, hex.Enc(env.E.Pubkey), env.E.ID)
	defer func() {
		if env != nil && env.E != nil {
			env.E.Free()
		}
	}()

	// Stage 3: Handle special kinds that need connection context
	if handled, err := l.handleSpecialKinds(env); handled {
		return err
	}

	// Stage 4: Progressive throttle for follows ACL mode
	if delay := l.getFollowsThrottleDelay(env.E); delay > 0 {
		log.D.F("HandleEvent: applying progressive throttle delay of %v", delay)
		select {
		case <-l.ctx.Done():
			return l.ctx.Err()
		case <-time.After(delay):
		}
	}

	// Stage 5: DM stranger rate limit check
	if l.dmRateLimiter != nil {
		if allowed, msg := l.dmRateLimiter.CheckDM(l.ctx, env.E); !allowed {
			if err := writeReasonedOK(l, env.Id(), msg); chk.E(err) {
				return err
			}
			return nil
		}
	}

	// Stage 6: Delegate to ingestion service
	connCtx := &ingestion.ConnectionContext{
		AuthedPubkey: l.authedPubkey.Load(),
		Remote:       l.remote,
		ConnectionID: l.connectionID,
	}
	result := l.ingestionService.Ingest(context.Background(), env.E, connCtx)

	// Post-ingestion hooks
	if result.Saved {
		// Invalidate channel membership cache when kind 41 (ChannelMetadata) is saved
		if env.E.Kind == kind.ChannelMetadata.K && l.channelMembership != nil {
			if channelID := ExtractChannelIDFromEvent(env.E); channelID != "" {
				l.channelMembership.InvalidateChannel(channelID)
			}
		}
		// Update DM bidirectional cache when a DM is saved
		if l.dmRateLimiter != nil {
			l.dmRateLimiter.OnDMIngested(env.E)
		}
	}

	// Stage 7: Send response based on result
	return l.sendIngestionResult(env, result)
}

// handleSpecialKinds handles event kinds that need connection context.
// Returns (true, err) if the event was handled, (false, nil) to continue normal processing.
func (l *Listener) handleSpecialKinds(env *eventenvelope.Submission) (bool, error) {
	switch env.E.Kind {
	case nip43.KindJoinRequest:
		if err := l.HandleNIP43JoinRequest(env.E); chk.E(err) {
			log.E.F("failed to process NIP-43 join request: %v", err)
		}
		return true, nil

	case nip43.KindLeaveRequest:
		if err := l.HandleNIP43LeaveRequest(env.E); chk.E(err) {
			log.E.F("failed to process NIP-43 leave request: %v", err)
		}
		return true, nil

	case kind.PolicyConfig.K:
		if err := l.HandlePolicyConfigUpdate(env.E); chk.E(err) {
			log.E.F("failed to process policy config update: %v", err)
			if err = Ok.Error(l, env, err.Error()); chk.E(err) {
				return true, err
			}
			return true, nil
		}
		if err := Ok.Ok(l, env, "policy configuration updated"); chk.E(err) {
			return true, err
		}
		return true, nil

	case kind.FollowList.K:
		// Check if this is a follow list update from a policy admin
		if l.IsPolicyAdminFollowListEvent(env.E) {
			go func() {
				if updateErr := l.HandlePolicyAdminFollowListUpdate(env.E); updateErr != nil {
					log.W.F("failed to update policy follows: %v", updateErr)
				}
			}()
		}
		// Continue with normal processing
		return false, nil
	}

	// Enforce channel membership for write operations on kinds 42-44
	// Channel kinds always require membership check regardless of ACL mode
	if kind.IsChannelKind(env.E.Kind) && !kind.IsDiscoverableChannelKind(env.E.Kind) {
		if l.channelMembership != nil {
			// Use the event's author pubkey (already signature-verified) for membership check
			if !l.channelMembership.IsChannelMember(env.E, env.E.Pubkey, l.ctx) {
				log.D.F("HandleEvent: channel write denied for pubkey %s (not a member)", hex.Enc(env.E.Pubkey))
				if err := writeReasonedOK(l, env.Id(), "restricted: not a channel member"); chk.E(err) {
					return true, err
				}
				return true, nil
			}
		}
	}

	// Enforce channel membership for non-channel events that reference channel
	// events via e-tags (reactions, reposts, reports, zaps, deletions targeting
	// channel messages). Two-step indirection: resolve e-tag → referenced event
	// → channel ID → membership check.
	if !kind.IsChannelKind(env.E.Kind) && l.channelMembership != nil {
		if channelIDHex, isChannel := l.channelMembership.ReferencesChannelEvent(env.E, l.ctx); isChannel {
			if !l.channelMembership.IsChannelMemberByID(channelIDHex, env.E.Kind, env.E.Pubkey, l.ctx) {
				log.D.F("HandleEvent: channel reference write denied for pubkey %s kind %d (not a member of channel %s)",
					hex.Enc(env.E.Pubkey), env.E.Kind, channelIDHex)
				if err := writeReasonedOK(l, env.Id(), "restricted: not a channel member"); chk.E(err) {
					return true, err
				}
				return true, nil
			}
		}
	}

	return false, nil
}

// sendIngestionResult sends the appropriate response based on the ingestion result.
func (l *Listener) sendIngestionResult(env *eventenvelope.Submission, result ingestion.Result) error {
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "already exists") {
			log.D.F("HandleEvent: duplicate event: %v", result.Error)
		} else {
			log.E.F("HandleEvent: ingestion error: %v", result.Error)
		}
		errMsg := result.Error.Error()
		if _, _, found := reason.Parse(errMsg); found {
			return writeReasonedOK(l, env.Id(), errMsg)
		}
		return Ok.Error(l, env, errMsg)
	}

	if result.RequireAuth {
		// Send OK false with auth required reason
		if err := okenvelope.NewFrom(
			env.Id(), false,
			reason.AuthRequired.F(result.Message),
		).Write(l); chk.E(err) {
			return err
		}
		// Send AUTH challenge
		return authenvelope.NewChallengeWith(l.challenge.Load()).Write(l)
	}

	if !result.Accepted {
		return writeReasonedOK(l, env.Id(), result.Message)
	}

	// Success
	log.D.F("HandleEvent: event %0x processed successfully", env.E.ID)
	return Ok.Ok(l, env, result.Message)
}

// isPeerRelayPubkey checks if the given pubkey belongs to a peer relay
func (l *Listener) isPeerRelayPubkey(pubkey []byte) bool {
	if l.syncManager == nil {
		return false
	}

	peerPubkeyHex := hex.Enc(pubkey)

	for _, peerURL := range l.syncManager.GetPeers() {
		if l.syncManager.IsAuthorizedPeer(peerURL, peerPubkeyHex) {
			return true
		}
	}

	return false
}

// HandleCuratingConfigUpdate processes curating configuration events (kind 30078)
func (l *Listener) HandleCuratingConfigUpdate(ev *event.E) error {
	if acl.Registry.Type() != "curating" {
		return nil
	}

	for _, aclInstance := range acl.Registry.ACLs() {
		if aclInstance.Type() == "curating" {
			if curating, ok := aclInstance.(*acl.Curating); ok {
				return curating.ProcessConfigEvent(ev)
			}
		}
	}

	return nil
}
