package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/authenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/reason"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/utils"
)

func (l *Listener) HandleEvent(msg []byte) (err error) {
	log.D.F("HandleEvent: START handling event: %s", msg)
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

	// Check if policy is enabled and process event through it
	if l.policyManager != nil && l.policyManager.Manager != nil && l.policyManager.Manager.IsEnabled() {

		// Check policy for write access
		allowed, policyErr := l.policyManager.CheckPolicy("write", env.E, l.authedPubkey.Load(), l.remote)
		if chk.E(policyErr) {
			log.E.F("policy check failed: %v", policyErr)
			if err = Ok.Error(
				l, env, "policy check failed",
			); chk.E(err) {
				return
			}
			return
		}

		if !allowed {
			log.D.F("policy rejected event %0x", env.E.ID)
			if err = Ok.Blocked(
				l, env, "event blocked by policy",
			); chk.E(err) {
				return
			}
			return
		}

		log.D.F("policy allowed event %0x", env.E.ID)

		// Check ACL policy for managed ACL mode, but skip for peer relay sync events
		if acl.Registry.Active.Load() == "managed" && !l.isPeerRelayPubkey(l.authedPubkey.Load()) {
			allowed, aclErr := acl.Registry.CheckPolicy(env.E)
			if chk.E(aclErr) {
				log.E.F("ACL policy check failed: %v", aclErr)
				if err = Ok.Error(
					l, env, "ACL policy check failed",
				); chk.E(err) {
					return
				}
				return
			}

			if !allowed {
				log.D.F("ACL policy rejected event %0x", env.E.ID)
				if err = Ok.Blocked(
					l, env, "event blocked by ACL policy",
				); chk.E(err) {
					return
				}
				return
			}

			log.D.F("ACL policy allowed event %0x", env.E.ID)
		}
	}

	// check the event ID is correct
	calculatedId := env.E.GetIDBytes()
	if !utils.FastEqual(calculatedId, env.E.ID) {
		if err = Ok.Invalid(
			l, env, "event id is computed incorrectly, "+
				"event has ID %0x, but when computed it is %0x",
			env.E.ID, calculatedId,
		); chk.E(err) {
			return
		}
		return
	}
	// validate timestamp - reject events too far in the future (more than 1 hour)
	now := time.Now().Unix()
	if env.E.CreatedAt > now+3600 {
		if err = Ok.Invalid(
			l, env,
			"timestamp too far in the future",
		); chk.E(err) {
			return
		}
		return
	}

	// verify the signature
	var ok bool
	if ok, err = env.Verify(); chk.T(err) {
		if err = Ok.Error(
			l, env, fmt.Sprintf(
				"failed to verify signature: %s",
				err.Error(),
			),
		); chk.E(err) {
			return
		}
	} else if !ok {
		if err = Ok.Invalid(
			l, env,
			"signature is invalid",
		); chk.E(err) {
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
	}

	// check permissions of user
	log.I.F(
		"HandleEvent: checking ACL permissions for pubkey: %s",
		hex.Enc(l.authedPubkey.Load()),
	)

	// If ACL mode is "none" and no pubkey is set, use the event's pubkey
	// But if auth is required or AuthToWrite is enabled, always use the authenticated pubkey
	var pubkeyForACL []byte
	if len(l.authedPubkey.Load()) == 0 && acl.Registry.Active.Load() == "none" && !l.Config.AuthRequired && !l.Config.AuthToWrite {
		pubkeyForACL = env.E.Pubkey
		log.I.F(
			"HandleEvent: ACL mode is 'none' and auth not required, using event pubkey for ACL check: %s",
			hex.Enc(pubkeyForACL),
		)
	} else {
		pubkeyForACL = l.authedPubkey.Load()
	}

	// If auth is required or AuthToWrite is enabled but user is not authenticated, deny access
	if (l.Config.AuthRequired || l.Config.AuthToWrite) && len(l.authedPubkey.Load()) == 0 {
		log.D.F("HandleEvent: authentication required for write operations but user not authenticated")
		if err = okenvelope.NewFrom(
			env.Id(), false,
			reason.AuthRequired.F("authentication required for write operations"),
		).Write(l); chk.E(err) {
			return
		}
		// Send AUTH challenge to prompt authentication
		log.D.F("HandleEvent: sending AUTH challenge to %s", l.remote)
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).
			Write(l); chk.E(err) {
			return
		}
		return
	}

	accessLevel := acl.Registry.GetAccessLevel(pubkeyForACL, l.remote)
	log.I.F("HandleEvent: ACL access level: %s", accessLevel)

	// Skip ACL check for admin/owner delete events
	skipACLCheck := false
	if env.E.Kind == kind.EventDeletion.K {
		// Check if the delete event signer is admin or owner
		for _, admin := range l.Admins {
			if utils.FastEqual(admin, env.E.Pubkey) {
				skipACLCheck = true
				log.I.F("HandleEvent: admin delete event - skipping ACL check")
				break
			}
		}
		if !skipACLCheck {
			for _, owner := range l.Owners {
				if utils.FastEqual(owner, env.E.Pubkey) {
					skipACLCheck = true
					log.I.F("HandleEvent: owner delete event - skipping ACL check")
					break
				}
			}
		}
	}

	if !skipACLCheck {
		switch accessLevel {
		case "none":
			log.D.F(
				"handle event: sending 'OK,false,auth-required...' to %s",
				l.remote,
			)
			if err = okenvelope.NewFrom(
				env.Id(), false,
				reason.AuthRequired.F("auth required for write access"),
			).Write(l); chk.E(err) {
				// return
			}
			log.D.F("handle event: sending challenge to %s", l.remote)
			if err = authenvelope.NewChallengeWith(l.challenge.Load()).
				Write(l); chk.E(err) {
				return
			}
			return
		case "read":
			log.D.F(
				"handle event: sending 'OK,false,auth-required:...' to %s",
				l.remote,
			)
			if err = okenvelope.NewFrom(
				env.Id(), false,
				reason.AuthRequired.F("auth required for write access"),
			).Write(l); chk.E(err) {
				return
			}
			log.D.F("handle event: sending challenge to %s", l.remote)
			if err = authenvelope.NewChallengeWith(l.challenge.Load()).
				Write(l); chk.E(err) {
				return
			}
			return
		case "blocked":
			log.D.F(
				"handle event: sending 'OK,false,blocked...' to %s",
				l.remote,
			)
			if err = okenvelope.NewFrom(
				env.Id(), false,
				reason.AuthRequired.F("IP address blocked"),
			).Write(l); chk.E(err) {
				return
			}
			return
		case "banned":
			log.D.F(
				"handle event: sending 'OK,false,banned...' to %s",
				l.remote,
			)
			if err = okenvelope.NewFrom(
				env.Id(), false,
				reason.AuthRequired.F("pubkey banned"),
			).Write(l); chk.E(err) {
				return
			}
			return
		default:
			// user has write access or better, continue
			log.I.F("HandleEvent: user has %s access, continuing", accessLevel)
		}
	} else {
		log.I.F("HandleEvent: skipping ACL check for admin/owner delete event")
	}

	// check if event is ephemeral - if so, deliver and return early
	if kind.IsEphemeral(env.E.Kind) {
		log.D.F("handling ephemeral event %0x (kind %d)", env.E.ID, env.E.Kind)
		// Send OK response for ephemeral events
		if err = Ok.Ok(l, env, ""); chk.E(err) {
			return
		}
		// Deliver the event to subscribers immediately
		clonedEvent := env.E.Clone()
		go l.publishers.Deliver(clonedEvent)
		log.D.F("delivered ephemeral event %0x", env.E.ID)
		return
	}
	log.D.F("processing regular event %0x (kind %d)", env.E.ID, env.E.Kind)

	// check for protected tag (NIP-70)
	protectedTag := env.E.Tags.GetFirst([]byte("-"))
	if protectedTag != nil && acl.Registry.Active.Load() != "none" {
		// check that the pubkey of the event matches the authed pubkey
		if !utils.FastEqual(l.authedPubkey.Load(), env.E.Pubkey) {
			if err = Ok.Blocked(
				l, env,
				"protected tag may only be published by user authed to the same pubkey",
			); chk.E(err) {
				return
			}
			return
		}
	}
	// if the event is a delete, process the delete
	log.I.F(
		"HandleEvent: checking if event is delete - kind: %d, EventDeletion.K: %d",
		env.E.Kind, kind.EventDeletion.K,
	)
	if env.E.Kind == kind.EventDeletion.K {
		log.I.F("processing delete event %0x", env.E.ID)

		// Store the delete event itself FIRST to ensure it's available for queries
		saveCtx, cancel := context.WithTimeout(
			context.Background(), 30*time.Second,
		)
		defer cancel()
		log.I.F(
			"attempting to save delete event %0x from pubkey %0x", env.E.ID,
			env.E.Pubkey,
		)
		log.I.F("delete event pubkey hex: %s", hex.Enc(env.E.Pubkey))
		if _, err = l.DB.SaveEvent(saveCtx, env.E); err != nil {
			log.E.F("failed to save delete event %0x: %v", env.E.ID, err)
			if strings.HasPrefix(err.Error(), "blocked:") {
				errStr := err.Error()[len("blocked: "):len(err.Error())]
				if err = Ok.Error(
					l, env, errStr,
				); chk.E(err) {
					return
				}
				return
			}
			chk.E(err)
			return
		}
		log.I.F("successfully saved delete event %0x", env.E.ID)

		// Now process the deletion (remove target events)
		if err = l.HandleDelete(env); err != nil {
			log.E.F("HandleDelete failed for event %0x: %v", env.E.ID, err)
			if strings.HasPrefix(err.Error(), "blocked:") {
				errStr := err.Error()[len("blocked: "):len(err.Error())]
				if err = Ok.Error(
					l, env, errStr,
				); chk.E(err) {
					return
				}
				return
			}
			// For non-blocked errors, still send OK but log the error
			log.W.F("Delete processing failed but continuing: %v", err)
		} else {
			log.I.F(
				"HandleDelete completed successfully for event %0x", env.E.ID,
			)
		}

		// Send OK response for delete events
		if err = Ok.Ok(l, env, ""); chk.E(err) {
			return
		}

		// Deliver the delete event to subscribers
		clonedEvent := env.E.Clone()
		go l.publishers.Deliver(clonedEvent)
		log.D.F("processed delete event %0x", env.E.ID)
		return
	} else {
		// check if the event was deleted
		// Combine admins and owners for deletion checking
		adminOwners := append(l.Admins, l.Owners...)
		if err = l.DB.CheckForDeleted(env.E, adminOwners); err != nil {
			if strings.HasPrefix(err.Error(), "blocked:") {
				errStr := err.Error()[len("blocked: "):len(err.Error())]
				if err = Ok.Error(
					l, env, errStr,
				); chk.E(err) {
					return
				}
			}
		}
	}
	// store the event - use a separate context to prevent cancellation issues
	saveCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// log.I.F("saving event %0x, %s", env.E.ID, env.E.Serialize())
	if _, err = l.DB.SaveEvent(saveCtx, env.E); err != nil {
		if strings.HasPrefix(err.Error(), "blocked:") {
			errStr := err.Error()[len("blocked: "):len(err.Error())]
			if err = Ok.Error(
				l, env, errStr,
			); chk.E(err) {
				return
			}
			return
		}
		chk.E(err)
		return
	}

	// Handle relay group configuration events
	if l.relayGroupMgr != nil {
		if err := l.relayGroupMgr.ValidateRelayGroupEvent(env.E); err != nil {
			log.W.F("invalid relay group config event %s: %v", hex.Enc(env.E.ID), err)
		}
		// Process the event and potentially update peer lists
		if l.syncManager != nil {
			l.relayGroupMgr.HandleRelayGroupEvent(env.E, l.syncManager)
		}
	}

	// Handle cluster membership events (Kind 39108)
	if env.E.Kind == 39108 && l.clusterManager != nil {
		if err := l.clusterManager.HandleMembershipEvent(env.E); err != nil {
			log.W.F("invalid cluster membership event %s: %v", hex.Enc(env.E.ID), err)
		}
	}

	// Update serial for distributed synchronization
	if l.syncManager != nil {
		l.syncManager.UpdateSerial()
		log.D.F("updated serial for event %s", hex.Enc(env.E.ID))
	}
	// Send a success response storing
	if err = Ok.Ok(l, env, ""); chk.E(err) {
		return
	}
	// Deliver the event to subscribers immediately after sending OK response
	// Clone the event to prevent corruption when the original is freed
	clonedEvent := env.E.Clone()
	go l.publishers.Deliver(clonedEvent)
	log.D.F("saved event %0x", env.E.ID)
	var isNewFromAdmin bool
	// Check if event is from admin or owner
	for _, admin := range l.Admins {
		if utils.FastEqual(admin, env.E.Pubkey) {
			isNewFromAdmin = true
			break
		}
	}
	if !isNewFromAdmin {
		for _, owner := range l.Owners {
			if utils.FastEqual(owner, env.E.Pubkey) {
				isNewFromAdmin = true
				break
			}
		}
	}
	if isNewFromAdmin {
		log.I.F("new event from admin %0x", env.E.Pubkey)
		// if a follow list was saved, reconfigure ACLs now that it is persisted
		if env.E.Kind == kind.FollowList.K ||
			env.E.Kind == kind.RelayListMetadata.K {
			// Run ACL reconfiguration asynchronously to prevent blocking websocket operations
			go func() {
				if err := acl.Registry.Configure(); chk.E(err) {
					log.E.F("failed to reconfigure ACL: %v", err)
				}
			}()
		}
	}
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
