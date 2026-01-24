// Package errors provides domain-specific error types for the ORLY relay.
// These typed errors enable structured error handling, machine-readable error codes,
// and proper error categorization throughout the codebase.
package errors

import (
	"fmt"
)

// DomainError is the base interface for all domain errors.
// It extends the standard error interface with structured metadata.
type DomainError interface {
	error
	Code() string      // Machine-readable error code (e.g., "INVALID_ID")
	Category() string  // Error category for grouping (e.g., "validation")
	IsRetryable() bool // Whether the operation can be retried
}

// Base provides common implementation for all domain errors.
type Base struct {
	code      string
	category  string
	message   string
	retryable bool
	cause     error
}

func (e *Base) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *Base) Code() string       { return e.code }
func (e *Base) Category() string   { return e.category }
func (e *Base) IsRetryable() bool  { return e.retryable }
func (e *Base) Unwrap() error      { return e.cause }

// WithCause returns a copy of the error with the given cause.
func (e *Base) WithCause(cause error) *Base {
	return &Base{
		code:      e.code,
		category:  e.category,
		message:   e.message,
		retryable: e.retryable,
		cause:     cause,
	}
}

// WithMessage returns a copy of the error with the given message.
func (e *Base) WithMessage(msg string) *Base {
	return &Base{
		code:      e.code,
		category:  e.category,
		message:   msg,
		retryable: e.retryable,
		cause:     e.cause,
	}
}

// =============================================================================
// Validation Errors
// =============================================================================

// ValidationError represents an error in event validation.
type ValidationError struct {
	Base
	Field string // The field that failed validation
}

// NewValidationError creates a new validation error.
func NewValidationError(code, field, message string) *ValidationError {
	return &ValidationError{
		Base: Base{
			code:     code,
			category: "validation",
			message:  message,
		},
		Field: field,
	}
}

// WithField returns a copy with the specified field.
func (e *ValidationError) WithField(field string) *ValidationError {
	return &ValidationError{
		Base:  e.Base,
		Field: field,
	}
}

// Validation error constants
var (
	ErrInvalidEventID = NewValidationError(
		"INVALID_ID",
		"id",
		"event ID does not match computed hash",
	)
	ErrInvalidSignature = NewValidationError(
		"INVALID_SIG",
		"sig",
		"signature verification failed",
	)
	ErrFutureTimestamp = NewValidationError(
		"FUTURE_TS",
		"created_at",
		"timestamp too far in future",
	)
	ErrPastTimestamp = NewValidationError(
		"PAST_TS",
		"created_at",
		"timestamp too far in past",
	)
	ErrUppercaseHex = NewValidationError(
		"UPPERCASE_HEX",
		"id/pubkey",
		"hex values must be lowercase",
	)
	ErrProtectedTagMismatch = NewValidationError(
		"PROTECTED_TAG",
		"tags",
		"protected event can only be modified by author",
	)
	ErrInvalidJSON = NewValidationError(
		"INVALID_JSON",
		"",
		"malformed JSON",
	)
	ErrEventTooLarge = NewValidationError(
		"EVENT_TOO_LARGE",
		"content",
		"event exceeds size limit",
	)
	ErrInvalidKind = NewValidationError(
		"INVALID_KIND",
		"kind",
		"event kind not allowed",
	)
	ErrMissingTag = NewValidationError(
		"MISSING_TAG",
		"tags",
		"required tag missing",
	)
	ErrInvalidTagValue = NewValidationError(
		"INVALID_TAG",
		"tags",
		"tag value validation failed",
	)
)

// =============================================================================
// Authorization Errors
// =============================================================================

// AuthorizationError represents an authorization failure.
type AuthorizationError struct {
	Base
	Pubkey      []byte // The pubkey that was denied
	AccessLevel string // The access level that was required
	RequireAuth bool   // Whether authentication might resolve this
}

// NeedsAuth returns true if authentication might resolve this error.
func (e *AuthorizationError) NeedsAuth() bool { return e.RequireAuth }

// NewAuthRequired creates an error indicating authentication is required.
func NewAuthRequired(reason string) *AuthorizationError {
	return &AuthorizationError{
		Base: Base{
			code:     "AUTH_REQUIRED",
			category: "authorization",
			message:  reason,
		},
		RequireAuth: true,
	}
}

// NewAccessDenied creates an error indicating access was denied.
func NewAccessDenied(level, reason string) *AuthorizationError {
	return &AuthorizationError{
		Base: Base{
			code:     "ACCESS_DENIED",
			category: "authorization",
			message:  reason,
		},
		AccessLevel: level,
	}
}

// WithPubkey returns a copy with the specified pubkey.
func (e *AuthorizationError) WithPubkey(pubkey []byte) *AuthorizationError {
	return &AuthorizationError{
		Base:        e.Base,
		Pubkey:      pubkey,
		AccessLevel: e.AccessLevel,
		RequireAuth: e.RequireAuth,
	}
}

// Authorization error constants
var (
	ErrAuthRequired = NewAuthRequired("authentication required")
	ErrBanned       = &AuthorizationError{
		Base: Base{
			code:     "BANNED",
			category: "authorization",
			message:  "pubkey banned",
		},
	}
	ErrIPBlocked = &AuthorizationError{
		Base: Base{
			code:     "IP_BLOCKED",
			category: "authorization",
			message:  "IP address blocked",
		},
	}
	ErrNotFollowed = &AuthorizationError{
		Base: Base{
			code:     "NOT_FOLLOWED",
			category: "authorization",
			message:  "write access requires being followed by admin",
		},
	}
	ErrNotMember = &AuthorizationError{
		Base: Base{
			code:     "NOT_MEMBER",
			category: "authorization",
			message:  "membership required",
		},
	}
	ErrInsufficientAccess = &AuthorizationError{
		Base: Base{
			code:     "INSUFFICIENT_ACCESS",
			category: "authorization",
			message:  "insufficient access level",
		},
	}
)

// =============================================================================
// Processing Errors
// =============================================================================

// ProcessingError represents an error during event processing.
type ProcessingError struct {
	Base
	EventID []byte // The event ID (if known)
	Kind    uint16 // The event kind (if known)
}

// NewProcessingError creates a new processing error.
func NewProcessingError(code string, message string, retryable bool) *ProcessingError {
	return &ProcessingError{
		Base: Base{
			code:      code,
			category:  "processing",
			message:   message,
			retryable: retryable,
		},
	}
}

// WithEventID returns a copy with the specified event ID.
func (e *ProcessingError) WithEventID(id []byte) *ProcessingError {
	return &ProcessingError{
		Base:    e.Base,
		EventID: id,
		Kind:    e.Kind,
	}
}

// WithKind returns a copy with the specified kind.
func (e *ProcessingError) WithKind(kind uint16) *ProcessingError {
	return &ProcessingError{
		Base:    e.Base,
		EventID: e.EventID,
		Kind:    kind,
	}
}

// Processing error constants
var (
	ErrDuplicate = NewProcessingError(
		"DUPLICATE",
		"event already exists",
		false,
	)
	ErrReplaceNotAllowed = NewProcessingError(
		"REPLACE_DENIED",
		"cannot replace event from different author",
		false,
	)
	ErrDeletedEvent = NewProcessingError(
		"DELETED",
		"event has been deleted",
		false,
	)
	ErrEphemeralNotStored = NewProcessingError(
		"EPHEMERAL",
		"ephemeral events are not stored",
		false,
	)
	ErrRateLimited = NewProcessingError(
		"RATE_LIMITED",
		"rate limit exceeded",
		true,
	)
	ErrSprocketRejected = NewProcessingError(
		"SPROCKET_REJECTED",
		"rejected by sprocket",
		false,
	)
)

// =============================================================================
// Policy Errors
// =============================================================================

// PolicyError represents a policy violation.
type PolicyError struct {
	Base
	RuleName string // The rule that was violated
	Action   string // The action taken (block, reject)
}

// NewPolicyBlocked creates an error for a blocked event.
func NewPolicyBlocked(ruleName, reason string) *PolicyError {
	return &PolicyError{
		Base: Base{
			code:     "POLICY_BLOCKED",
			category: "policy",
			message:  reason,
		},
		RuleName: ruleName,
		Action:   "block",
	}
}

// NewPolicyRejected creates an error for a rejected event.
func NewPolicyRejected(ruleName, reason string) *PolicyError {
	return &PolicyError{
		Base: Base{
			code:     "POLICY_REJECTED",
			category: "policy",
			message:  reason,
		},
		RuleName: ruleName,
		Action:   "reject",
	}
}

// Policy error constants
var (
	ErrKindBlocked = &PolicyError{
		Base: Base{
			code:     "KIND_BLOCKED",
			category: "policy",
			message:  "event kind not allowed by policy",
		},
		Action: "block",
	}
	ErrPubkeyBlocked = &PolicyError{
		Base: Base{
			code:     "PUBKEY_BLOCKED",
			category: "policy",
			message:  "pubkey blocked by policy",
		},
		Action: "block",
	}
	ErrContentBlocked = &PolicyError{
		Base: Base{
			code:     "CONTENT_BLOCKED",
			category: "policy",
			message:  "content blocked by policy",
		},
		Action: "block",
	}
	ErrScriptRejected = &PolicyError{
		Base: Base{
			code:     "SCRIPT_REJECTED",
			category: "policy",
			message:  "rejected by policy script",
		},
		Action: "reject",
	}
)

// =============================================================================
// Storage Errors
// =============================================================================

// StorageError represents a storage-layer error.
type StorageError struct {
	Base
}

// NewStorageError creates a new storage error.
func NewStorageError(code, message string, cause error, retryable bool) *StorageError {
	return &StorageError{
		Base: Base{
			code:      code,
			category:  "storage",
			message:   message,
			cause:     cause,
			retryable: retryable,
		},
	}
}

// Storage error constants
var (
	ErrDatabaseUnavailable = NewStorageError(
		"DB_UNAVAILABLE",
		"database not available",
		nil,
		true,
	)
	ErrWriteTimeout = NewStorageError(
		"WRITE_TIMEOUT",
		"write operation timed out",
		nil,
		true,
	)
	ErrReadTimeout = NewStorageError(
		"READ_TIMEOUT",
		"read operation timed out",
		nil,
		true,
	)
	ErrStorageFull = NewStorageError(
		"STORAGE_FULL",
		"storage capacity exceeded",
		nil,
		false,
	)
	ErrCorruptedData = NewStorageError(
		"CORRUPTED_DATA",
		"data corruption detected",
		nil,
		false,
	)
)

// =============================================================================
// Service Errors
// =============================================================================

// ServiceError represents a service-level error.
type ServiceError struct {
	Base
	ServiceName string
}

// NewServiceError creates a new service error.
func NewServiceError(code, service, message string, retryable bool) *ServiceError {
	return &ServiceError{
		Base: Base{
			code:      code,
			category:  "service",
			message:   message,
			retryable: retryable,
		},
		ServiceName: service,
	}
}

// Service error constants
var (
	ErrServiceUnavailable = NewServiceError(
		"SERVICE_UNAVAILABLE",
		"",
		"service temporarily unavailable",
		true,
	)
	ErrServiceTimeout = NewServiceError(
		"SERVICE_TIMEOUT",
		"",
		"service request timed out",
		true,
	)
	ErrServiceOverloaded = NewServiceError(
		"SERVICE_OVERLOADED",
		"",
		"service is overloaded",
		true,
	)
)

// =============================================================================
// Helper Functions
// =============================================================================

// Is checks if an error matches a target domain error by code.
func Is(err error, target DomainError) bool {
	if de, ok := err.(DomainError); ok {
		return de.Code() == target.Code()
	}
	return false
}

// Code returns the error code if err is a DomainError, empty string otherwise.
func Code(err error) string {
	if de, ok := err.(DomainError); ok {
		return de.Code()
	}
	return ""
}

// Category returns the category of a domain error, or "unknown" for other errors.
func Category(err error) string {
	if de, ok := err.(DomainError); ok {
		return de.Category()
	}
	return "unknown"
}

// IsRetryable checks if an error indicates the operation can be retried.
func IsRetryable(err error) bool {
	if de, ok := err.(DomainError); ok {
		return de.IsRetryable()
	}
	return false
}

// IsValidation checks if an error is a validation error.
func IsValidation(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsAuthorization checks if an error is an authorization error.
func IsAuthorization(err error) bool {
	_, ok := err.(*AuthorizationError)
	return ok
}

// IsProcessing checks if an error is a processing error.
func IsProcessing(err error) bool {
	_, ok := err.(*ProcessingError)
	return ok
}

// IsPolicy checks if an error is a policy error.
func IsPolicy(err error) bool {
	_, ok := err.(*PolicyError)
	return ok
}

// IsStorage checks if an error is a storage error.
func IsStorage(err error) bool {
	_, ok := err.(*StorageError)
	return ok
}

// NeedsAuth checks if an authorization error requires authentication.
func NeedsAuth(err error) bool {
	if ae, ok := err.(*AuthorizationError); ok {
		return ae.NeedsAuth()
	}
	return false
}
