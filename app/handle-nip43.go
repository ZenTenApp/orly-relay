package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"next.orly.dev/pkg/protocol/nip43"
)

// HandleNIP43JoinRequest processes a kind 28934 join request
func (l *Listener) HandleNIP43JoinRequest(ev *event.E) error {
	log.I.F("handling NIP-43 join request from %s", hex.Enc(ev.Pubkey))

	// Validate the join request
	inviteCode, valid, reason := nip43.ValidateJoinRequest(ev)
	if !valid {
		log.W.F("invalid join request: %s", reason)
		return l.sendOKResponse(ev.ID, false, fmt.Sprintf("restricted: %s", reason))
	}

	// Check if user is already a member
	isMember, err := l.DB.IsNIP43Member(ev.Pubkey)
	if chk.E(err) {
		log.E.F("error checking membership: %v", err)
		return l.sendOKResponse(ev.ID, false, "error: internal server error")
	}

	if isMember {
		log.I.F("user %s is already a member", hex.Enc(ev.Pubkey))
		return l.sendOKResponse(ev.ID, true, "duplicate: you are already a member of this relay")
	}

	// Validate the invite code
	validCode, reason := l.Server.InviteManager.ValidateAndConsume(inviteCode, ev.Pubkey)

	if !validCode {
		log.W.F("invalid or expired invite code: %s - %s", inviteCode, reason)
		return l.sendOKResponse(ev.ID, false, fmt.Sprintf("restricted: %s", reason))
	}

	// Add the member
	if err = l.DB.AddNIP43Member(ev.Pubkey, inviteCode); chk.E(err) {
		log.E.F("error adding member: %v", err)
		return l.sendOKResponse(ev.ID, false, "error: failed to add member")
	}

	log.I.F("successfully added member %s via invite code", hex.Enc(ev.Pubkey))

	// Publish kind 8000 "add member" event if configured
	if l.Config.NIP43PublishEvents {
		if err = l.publishAddUserEvent(ev.Pubkey); chk.E(err) {
			log.W.F("failed to publish add user event: %v", err)
		}
	}

	// Update membership list if configured
	if l.Config.NIP43PublishMemberList {
		if err = l.publishMembershipList(); chk.E(err) {
			log.W.F("failed to publish membership list: %v", err)
		}
	}

	relayURL := l.Config.RelayURL
	if relayURL == "" {
		relayURL = fmt.Sprintf("wss://%s:%d", l.Config.Listen, l.Config.Port)
	}

	return l.sendOKResponse(ev.ID, true, fmt.Sprintf("welcome to %s!", relayURL))
}

// HandleNIP43LeaveRequest processes a kind 28936 leave request
func (l *Listener) HandleNIP43LeaveRequest(ev *event.E) error {
	log.I.F("handling NIP-43 leave request from %s", hex.Enc(ev.Pubkey))

	// Validate the leave request
	valid, reason := nip43.ValidateLeaveRequest(ev)
	if !valid {
		log.W.F("invalid leave request: %s", reason)
		return l.sendOKResponse(ev.ID, false, fmt.Sprintf("error: %s", reason))
	}

	// Check if user is a member
	isMember, err := l.DB.IsNIP43Member(ev.Pubkey)
	if chk.E(err) {
		log.E.F("error checking membership: %v", err)
		return l.sendOKResponse(ev.ID, false, "error: internal server error")
	}

	if !isMember {
		log.I.F("user %s is not a member", hex.Enc(ev.Pubkey))
		return l.sendOKResponse(ev.ID, true, "you are not a member of this relay")
	}

	// Remove the member
	if err = l.DB.RemoveNIP43Member(ev.Pubkey); chk.E(err) {
		log.E.F("error removing member: %v", err)
		return l.sendOKResponse(ev.ID, false, "error: failed to remove member")
	}

	log.I.F("successfully removed member %s", hex.Enc(ev.Pubkey))

	// Publish kind 8001 "remove member" event if configured
	if l.Config.NIP43PublishEvents {
		if err = l.publishRemoveUserEvent(ev.Pubkey); chk.E(err) {
			log.W.F("failed to publish remove user event: %v", err)
		}
	}

	// Update membership list if configured
	if l.Config.NIP43PublishMemberList {
		if err = l.publishMembershipList(); chk.E(err) {
			log.W.F("failed to publish membership list: %v", err)
		}
	}

	return l.sendOKResponse(ev.ID, true, "you have been removed from this relay")
}

// HandleNIP43InviteRequest processes a kind 28935 invite request (REQ subscription)
func (s *Server) HandleNIP43InviteRequest(pubkey []byte) (*event.E, error) {
	log.I.F("generating NIP-43 invite for pubkey %s", hex.Enc(pubkey))

	// Check if requester has permission to request invites
	// This could be based on ACL, admins, etc.
	accessLevel := acl.Registry.GetAccessLevel(pubkey, "")
	if accessLevel != "admin" && accessLevel != "owner" {
		log.W.F("unauthorized invite request from %s (level: %s)", hex.Enc(pubkey), accessLevel)
		return nil, fmt.Errorf("unauthorized: only admins can request invites")
	}

	// Generate a new invite code
	code, err := s.InviteManager.GenerateCode()
	if chk.E(err) {
		return nil, err
	}

	// Get relay identity
	relaySecret, err := s.db.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		return nil, err
	}

	// Build the invite event
	inviteEvent, err := nip43.BuildInviteEvent(relaySecret, code)
	if chk.E(err) {
		return nil, err
	}

	log.I.F("generated invite code for %s", hex.Enc(pubkey))
	return inviteEvent, nil
}

// publishAddUserEvent publishes a kind 8000 add user event
func (l *Listener) publishAddUserEvent(userPubkey []byte) error {
	relaySecret, err := l.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		return err
	}

	ev, err := nip43.BuildAddUserEvent(relaySecret, userPubkey)
	if chk.E(err) {
		return err
	}

	// Save to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = l.DB.SaveEvent(ctx, ev); chk.E(err) {
		return err
	}

	// Publish to subscribers
	l.publishers.Deliver(ev)

	log.I.F("published kind 8000 add user event for %s", hex.Enc(userPubkey))
	return nil
}

// publishRemoveUserEvent publishes a kind 8001 remove user event
func (l *Listener) publishRemoveUserEvent(userPubkey []byte) error {
	relaySecret, err := l.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		return err
	}

	ev, err := nip43.BuildRemoveUserEvent(relaySecret, userPubkey)
	if chk.E(err) {
		return err
	}

	// Save to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = l.DB.SaveEvent(ctx, ev); chk.E(err) {
		return err
	}

	// Publish to subscribers
	l.publishers.Deliver(ev)

	log.I.F("published kind 8001 remove user event for %s", hex.Enc(userPubkey))
	return nil
}

// publishMembershipList publishes a kind 13534 membership list event
func (l *Listener) publishMembershipList() error {
	// Get all members
	members, err := l.DB.GetAllNIP43Members()
	if chk.E(err) {
		return err
	}

	relaySecret, err := l.DB.GetOrCreateRelayIdentitySecret()
	if chk.E(err) {
		return err
	}

	ev, err := nip43.BuildMemberListEvent(relaySecret, members)
	if chk.E(err) {
		return err
	}

	// Save to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err = l.DB.SaveEvent(ctx, ev); chk.E(err) {
		return err
	}

	// Publish to subscribers
	l.publishers.Deliver(ev)

	log.I.F("published kind 13534 membership list event with %d members", len(members))
	return nil
}

// sendOKResponse sends an OK envelope response
func (l *Listener) sendOKResponse(eventID []byte, accepted bool, message string) error {
	// Ensure message doesn't have "restricted: " prefix if already present
	if accepted && strings.HasPrefix(message, "restricted: ") {
		message = strings.TrimPrefix(message, "restricted: ")
	}

	env := okenvelope.NewFrom(eventID, accepted, []byte(message))
	return env.Write(l)
}
