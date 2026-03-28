// Package subscribers provides domain event subscriber implementations.
package subscribers

import (
	"encoding/hex"

	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/domain/events"
)

// LoggingSubscriber logs domain events for analytics and debugging.
type LoggingSubscriber struct {
	logLevel string // "debug", "info", "trace"
}

// NewLoggingSubscriber creates a new logging subscriber.
// logLevel controls verbosity: "trace" logs all events, "debug" logs important events,
// "info" logs only significant events like membership changes.
func NewLoggingSubscriber(logLevel string) *LoggingSubscriber {
	if logLevel == "" {
		logLevel = "debug"
	}
	return &LoggingSubscriber{logLevel: logLevel}
}

// Handle processes a domain event by logging it.
func (s *LoggingSubscriber) Handle(event events.DomainEvent) {
	switch e := event.(type) {
	case *events.EventSaved:
		s.logEventSaved(e)
	case *events.EventDeleted:
		s.logEventDeleted(e)
	case *events.FollowListUpdated:
		s.logFollowListUpdated(e)
	case *events.ACLMembershipChanged:
		s.logACLMembershipChanged(e)
	case *events.PolicyConfigUpdated:
		s.logPolicyConfigUpdated(e)
	case *events.UserAuthenticated:
		s.logUserAuthenticated(e)
	case *events.MemberJoined:
		s.logMemberJoined(e)
	case *events.MemberLeft:
		s.logMemberLeft(e)
	case *events.ConnectionOpened:
		s.logConnectionOpened(e)
	case *events.ConnectionClosed:
		s.logConnectionClosed(e)
	default:
		if s.logLevel == "trace" {
			log.T.F("domain event: %s", event.EventType())
		}
	}
}

// Supports returns true for all event types.
func (s *LoggingSubscriber) Supports(eventType string) bool {
	return true
}

// trunc16 safely truncates a hex string to 16 chars.
func trunc16(s string) string {
	if len(s) < 16 {
		return s
	}
	return s[:16]
}

func (s *LoggingSubscriber) logEventSaved(e *events.EventSaved) {
	if s.logLevel == "trace" {
		pubkeyHex := hex.EncodeToString(e.Event.Pubkey)
		log.T.F("event saved: kind=%d pubkey=%s admin=%v owner=%v",
			e.Event.Kind, trunc16(pubkeyHex), e.IsAdmin, e.IsOwner)
	}
}

func (s *LoggingSubscriber) logEventDeleted(e *events.EventDeleted) {
	if s.logLevel == "trace" || s.logLevel == "debug" {
		eventIDHex := hex.EncodeToString(e.EventID)
		deletedByHex := hex.EncodeToString(e.DeletedBy)
		log.D.F("event deleted: id=%s by=%s", trunc16(eventIDHex), trunc16(deletedByHex))
	}
}

func (s *LoggingSubscriber) logFollowListUpdated(e *events.FollowListUpdated) {
	if s.logLevel == "trace" || s.logLevel == "debug" {
		adminHex := hex.EncodeToString(e.AdminPubkey)
		log.D.F("follow list updated: admin=%s added=%d removed=%d",
			trunc16(adminHex), len(e.AddedFollows), len(e.RemovedFollows))
	}
}

func (s *LoggingSubscriber) logACLMembershipChanged(e *events.ACLMembershipChanged) {
	pubkeyHex := hex.EncodeToString(e.Pubkey)
	log.I.F("ACL membership changed: pubkey=%s %s->%s reason=%s",
		trunc16(pubkeyHex), e.PrevLevel, e.NewLevel, e.Reason)
}

func (s *LoggingSubscriber) logPolicyConfigUpdated(e *events.PolicyConfigUpdated) {
	updatedByHex := hex.EncodeToString(e.UpdatedBy)
	log.I.F("policy config updated by %s: %d changes", trunc16(updatedByHex), len(e.Changes))
}

func (s *LoggingSubscriber) logUserAuthenticated(e *events.UserAuthenticated) {
	if s.logLevel == "trace" || s.logLevel == "debug" {
		pubkeyHex := hex.EncodeToString(e.Pubkey)
		log.D.F("user authenticated: pubkey=%s level=%s firstTime=%v",
			trunc16(pubkeyHex), e.AccessLevel, e.IsFirstTime)
	}
}

func (s *LoggingSubscriber) logMemberJoined(e *events.MemberJoined) {
	pubkeyHex := hex.EncodeToString(e.Pubkey)
	log.I.F("member joined: pubkey=%s invite=%s", trunc16(pubkeyHex), e.InviteCode)
}

func (s *LoggingSubscriber) logMemberLeft(e *events.MemberLeft) {
	pubkeyHex := hex.EncodeToString(e.Pubkey)
	log.I.F("member left: pubkey=%s", trunc16(pubkeyHex))
}

func (s *LoggingSubscriber) logConnectionOpened(e *events.ConnectionOpened) {
	if s.logLevel == "trace" {
		log.T.F("connection opened: id=%s remote=%s", e.ConnectionID, e.RemoteAddr)
	}
}

func (s *LoggingSubscriber) logConnectionClosed(e *events.ConnectionClosed) {
	if s.logLevel == "trace" || s.logLevel == "debug" {
		log.D.F("connection closed: id=%s duration=%v events_rx=%d events_tx=%d",
			e.ConnectionID, e.Duration, e.EventsReceived, e.EventsPublished)
	}
}
