package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/authenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/noticeenvelope"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/reason"
	"next.orly.dev/pkg/protocol/nip43"
	"next.orly.dev/pkg/ratelimit"
	"next.orly.dev/pkg/utils"
)

// validateLowercaseHexInJSON checks that all hex-encoded fields in the raw JSON are lowercase.
// NIP-01 specifies that hex encoding must be lowercase.
// This must be called on the raw message BEFORE unmarshaling, since unmarshal converts
// hex strings to binary and loses case information.
// Returns an error message if validation fails, or empty string if valid.
func validateLowercaseHexInJSON(msg []byte) string {
	// Find and validate "id" field (64 hex chars)
	if err := validateJSONHexField(msg, `"id"`); err != "" {
		return err + " (id)"
	}

	// Find and validate "pubkey" field (64 hex chars)
	if err := validateJSONHexField(msg, `"pubkey"`); err != "" {
		return err + " (pubkey)"
	}

	// Find and validate "sig" field (128 hex chars)
	if err := validateJSONHexField(msg, `"sig"`); err != "" {
		return err + " (sig)"
	}

	// Validate e and p tags in the tags array
	// Tags format: ["e", "hexvalue", ...] or ["p", "hexvalue", ...]
	if err := validateEPTagsInJSON(msg); err != "" {
		return err
	}

	return "" // Valid
}

// validateJSONHexField finds a JSON field and checks if its hex value contains uppercase.
func validateJSONHexField(msg []byte, fieldName string) string {
	// Find the field name
	idx := bytes.Index(msg, []byte(fieldName))
	if idx == -1 {
		return "" // Field not found, skip
	}

	// Find the colon after the field name
	colonIdx := bytes.Index(msg[idx:], []byte(":"))
	if colonIdx == -1 {
		return ""
	}

	// Find the opening quote of the value
	valueStart := idx + colonIdx + 1
	for valueStart < len(msg) && (msg[valueStart] == ' ' || msg[valueStart] == '\t' || msg[valueStart] == '\n' || msg[valueStart] == '\r') {
		valueStart++
	}
	if valueStart >= len(msg) || msg[valueStart] != '"' {
		return ""
	}
	valueStart++ // Skip the opening quote

	// Find the closing quote
	valueEnd := valueStart
	for valueEnd < len(msg) && msg[valueEnd] != '"' {
		valueEnd++
	}

	// Extract the hex value and check for uppercase
	hexValue := msg[valueStart:valueEnd]
	if containsUppercaseHex(hexValue) {
		return "blocked: hex fields may only be lower case, see NIP-01"
	}

	return ""
}

// validateEPTagsInJSON checks e and p tags in the JSON for uppercase hex.
func validateEPTagsInJSON(msg []byte) string {
	// Find the tags array
	tagsIdx := bytes.Index(msg, []byte(`"tags"`))
	if tagsIdx == -1 {
		return "" // No tags
	}

	// Find the opening bracket of the tags array
	bracketIdx := bytes.Index(msg[tagsIdx:], []byte("["))
	if bracketIdx == -1 {
		return ""
	}

	tagsStart := tagsIdx + bracketIdx

	// Scan through to find ["e", ...] and ["p", ...] patterns
	// This is a simplified parser that looks for specific patterns
	pos := tagsStart
	for pos < len(msg) {
		// Look for ["e" or ["p" pattern
		eTagPattern := bytes.Index(msg[pos:], []byte(`["e"`))
		pTagPattern := bytes.Index(msg[pos:], []byte(`["p"`))

		var tagType string
		var nextIdx int

		if eTagPattern == -1 && pTagPattern == -1 {
			break // No more e or p tags
		} else if eTagPattern == -1 {
			nextIdx = pos + pTagPattern
			tagType = "p"
		} else if pTagPattern == -1 {
			nextIdx = pos + eTagPattern
			tagType = "e"
		} else if eTagPattern < pTagPattern {
			nextIdx = pos + eTagPattern
			tagType = "e"
		} else {
			nextIdx = pos + pTagPattern
			tagType = "p"
		}

		// Find the hex value after the tag type
		// Pattern: ["e", "hexvalue" or ["p", "hexvalue"
		commaIdx := bytes.Index(msg[nextIdx:], []byte(","))
		if commaIdx == -1 {
			pos = nextIdx + 4
			continue
		}

		// Find the opening quote of the hex value
		valueStart := nextIdx + commaIdx + 1
		for valueStart < len(msg) && (msg[valueStart] == ' ' || msg[valueStart] == '\t' || msg[valueStart] == '"') {
			if msg[valueStart] == '"' {
				valueStart++
				break
			}
			valueStart++
		}

		// Find the closing quote
		valueEnd := valueStart
		for valueEnd < len(msg) && msg[valueEnd] != '"' {
			valueEnd++
		}

		// Check if this looks like a hex value (64 chars for pubkey/event ID)
		hexValue := msg[valueStart:valueEnd]
		if len(hexValue) == 64 && containsUppercaseHex(hexValue) {
			return fmt.Sprintf("blocked: hex fields may only be lower case, see NIP-01 (%s tag)", tagType)
		}

		pos = valueEnd + 1
	}

	return ""
}

// containsUppercaseHex checks if a byte slice (representing hex) contains uppercase letters A-F.
func containsUppercaseHex(b []byte) bool {
	for _, c := range b {
		if c >= 'A' && c <= 'F' {
			return true
		}
	}
	return false
}

func (l *Listener) HandleEvent(msg []byte) (err error) {
	log.D.F("HandleEvent: START handling event: %s", msg)

	// Validate that all hex fields are lowercase BEFORE unmarshaling
	// (unmarshal converts hex to binary and loses case information)
	if errMsg := validateLowercaseHexInJSON(msg); errMsg != "" {
		log.W.F("HandleEvent: rejecting event with uppercase hex: %s", errMsg)
		// Send NOTICE to alert client developers about the issue
		if noticeErr := noticeenvelope.NewFrom(errMsg).Write(l); noticeErr != nil {
			log.E.F("failed to send NOTICE for uppercase hex: %v", noticeErr)
		}
		// Send OK false with the error message
		if err = okenvelope.NewFrom(
			nil, false,
			reason.Blocked.F(errMsg),
		).Write(l); chk.E(err) {
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

	// Check if policy is enabled and process event through it
	if l.policyManager.IsEnabled() {

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
		// Apply rate limiting before write operation
		if l.rateLimiter != nil && l.rateLimiter.IsEnabled() {
			l.rateLimiter.Wait(saveCtx, int(ratelimit.Write))
		}
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
		// Skip deletion check when ACL is "none" (open relay mode)
		if acl.Registry.Active.Load() != "none" {
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
	}
	// store the event - use a separate context to prevent cancellation issues
	saveCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// Apply rate limiting before write operation
	if l.rateLimiter != nil && l.rateLimiter.IsEnabled() {
		l.rateLimiter.Wait(saveCtx, int(ratelimit.Write))
	}
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
