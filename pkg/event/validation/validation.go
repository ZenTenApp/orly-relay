// Package validation provides event validation services for the ORLY relay.
// It handles structural validation (hex case, JSON format), cryptographic
// validation (signature, ID), and protocol validation (timestamp, NIP-70).
package validation

import (
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/reason"
)

// ReasonCode identifies the type of validation failure for response formatting.
type ReasonCode int

const (
	ReasonNone ReasonCode = iota
	ReasonBlocked
	ReasonInvalid
	ReasonError
)

// Result contains the outcome of a validation check.
type Result struct {
	Valid  bool
	Code   ReasonCode // For response formatting
	Msg    string     // Human-readable error message
}

// OK returns a successful validation result.
func OK() Result {
	return Result{Valid: true}
}

// Blocked returns a blocked validation result.
func Blocked(msg string) Result {
	return Result{Valid: false, Code: ReasonBlocked, Msg: msg}
}

// Invalid returns an invalid validation result.
func Invalid(msg string) Result {
	return Result{Valid: false, Code: ReasonInvalid, Msg: msg}
}

// Error returns an error validation result.
func Error(msg string) Result {
	return Result{Valid: false, Code: ReasonError, Msg: msg}
}

// PrefixedMessage returns the validation failure as a NIP-01 reason string.
// Messages that already have a known prefix are returned unchanged.
func (r Result) PrefixedMessage() string {
	if r.Valid {
		return ""
	}
	if _, _, found := reason.Parse(r.Msg); found {
		return r.Msg
	}
	switch r.Code {
	case ReasonInvalid:
		return reason.Ensure(r.Msg, reason.Invalid)
	case ReasonError:
		return reason.Ensure(r.Msg, reason.Error)
	default:
		return reason.Ensure(r.Msg, reason.Blocked)
	}
}

// Validator validates events before processing.
type Validator interface {
	// ValidateRawJSON validates raw message before unmarshaling.
	// This catches issues like uppercase hex that are lost after unmarshal.
	ValidateRawJSON(msg []byte) Result

	// ValidateEvent validates an unmarshaled event.
	// Checks ID computation, signature, and timestamp.
	ValidateEvent(ev *event.E) Result

	// ValidateProtectedTag checks NIP-70 protected tag requirements.
	// The authedPubkey is the authenticated pubkey of the connection.
	ValidateProtectedTag(ev *event.E, authedPubkey []byte) Result
}

// Config holds configuration for the validation service.
type Config struct {
	// MaxFutureSeconds is how far in the future a timestamp can be (default: 3600 = 1 hour)
	MaxFutureSeconds int64
}

// DefaultConfig returns the default validation configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxFutureSeconds: 3600,
	}
}

// Service implements the Validator interface.
type Service struct {
	cfg *Config
}

// New creates a new validation service with default configuration.
func New() *Service {
	return &Service{cfg: DefaultConfig()}
}

// NewWithConfig creates a new validation service with the given configuration.
func NewWithConfig(cfg *Config) *Service {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Service{cfg: cfg}
}

// ValidateRawJSON validates raw message before unmarshaling.
func (s *Service) ValidateRawJSON(msg []byte) Result {
	if errMsg := ValidateLowercaseHexInJSON(msg); errMsg != "" {
		return Blocked(errMsg)
	}
	return OK()
}

// ValidateEvent validates an unmarshaled event.
func (s *Service) ValidateEvent(ev *event.E) Result {
	// Validate event ID
	if result := ValidateEventID(ev); !result.Valid {
		return result
	}

	// Validate signature
	if result := ValidateSignature(ev); !result.Valid {
		return result
	}

	// Validate timestamp
	if result := ValidateTimestamp(ev, s.cfg.MaxFutureSeconds); !result.Valid {
		return result
	}

	return OK()
}

// ValidateProtectedTag checks NIP-70 protected tag requirements.
func (s *Service) ValidateProtectedTag(ev *event.E, authedPubkey []byte) Result {
	return ValidateProtectedTagMatch(ev, authedPubkey)
}
