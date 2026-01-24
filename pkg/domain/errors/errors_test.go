package errors

import (
	"errors"
	"testing"
)

func TestValidationError(t *testing.T) {
	err := ErrInvalidEventID
	if err.Code() != "INVALID_ID" {
		t.Errorf("expected code INVALID_ID, got %s", err.Code())
	}
	if err.Category() != "validation" {
		t.Errorf("expected category validation, got %s", err.Category())
	}
	if err.Field != "id" {
		t.Errorf("expected field id, got %s", err.Field)
	}
	if err.IsRetryable() {
		t.Error("validation errors should not be retryable")
	}
}

func TestAuthorizationError(t *testing.T) {
	err := ErrAuthRequired
	if err.Code() != "AUTH_REQUIRED" {
		t.Errorf("expected code AUTH_REQUIRED, got %s", err.Code())
	}
	if !err.NeedsAuth() {
		t.Error("ErrAuthRequired should indicate auth is needed")
	}

	err2 := ErrBanned
	if err2.NeedsAuth() {
		t.Error("ErrBanned should not indicate auth is needed")
	}
}

func TestProcessingError(t *testing.T) {
	err := ErrRateLimited
	if err.Code() != "RATE_LIMITED" {
		t.Errorf("expected code RATE_LIMITED, got %s", err.Code())
	}
	if !err.IsRetryable() {
		t.Error("rate limit errors should be retryable")
	}

	err2 := ErrDuplicate
	if err2.IsRetryable() {
		t.Error("duplicate errors should not be retryable")
	}
}

func TestStorageError(t *testing.T) {
	err := ErrDatabaseUnavailable
	if err.Code() != "DB_UNAVAILABLE" {
		t.Errorf("expected code DB_UNAVAILABLE, got %s", err.Code())
	}
	if err.Category() != "storage" {
		t.Errorf("expected category storage, got %s", err.Category())
	}
	if !err.IsRetryable() {
		t.Error("database unavailable should be retryable")
	}
}

func TestPolicyError(t *testing.T) {
	err := NewPolicyBlocked("kind_whitelist", "kind 30023 not allowed")
	if err.Code() != "POLICY_BLOCKED" {
		t.Errorf("expected code POLICY_BLOCKED, got %s", err.Code())
	}
	if err.RuleName != "kind_whitelist" {
		t.Errorf("expected rule name kind_whitelist, got %s", err.RuleName)
	}
	if err.Action != "block" {
		t.Errorf("expected action block, got %s", err.Action)
	}
}

func TestIsHelper(t *testing.T) {
	if !Is(ErrInvalidEventID, ErrInvalidEventID) {
		t.Error("Is should return true for same error")
	}
	if Is(ErrInvalidEventID, ErrInvalidSignature) {
		t.Error("Is should return false for different errors")
	}

	// Test with non-domain error
	stdErr := errors.New("standard error")
	if Is(stdErr, ErrInvalidEventID) {
		t.Error("Is should return false for non-domain errors")
	}
}

func TestCodeHelper(t *testing.T) {
	if Code(ErrInvalidEventID) != "INVALID_ID" {
		t.Errorf("expected INVALID_ID, got %s", Code(ErrInvalidEventID))
	}

	stdErr := errors.New("standard error")
	if Code(stdErr) != "" {
		t.Errorf("expected empty string for non-domain error, got %s", Code(stdErr))
	}
}

func TestCategoryHelper(t *testing.T) {
	if Category(ErrInvalidEventID) != "validation" {
		t.Errorf("expected validation, got %s", Category(ErrInvalidEventID))
	}
	if Category(ErrBanned) != "authorization" {
		t.Errorf("expected authorization, got %s", Category(ErrBanned))
	}
	if Category(ErrDuplicate) != "processing" {
		t.Errorf("expected processing, got %s", Category(ErrDuplicate))
	}

	stdErr := errors.New("standard error")
	if Category(stdErr) != "unknown" {
		t.Errorf("expected unknown for non-domain error, got %s", Category(stdErr))
	}
}

func TestIsRetryableHelper(t *testing.T) {
	if !IsRetryable(ErrRateLimited) {
		t.Error("ErrRateLimited should be retryable")
	}
	if !IsRetryable(ErrDatabaseUnavailable) {
		t.Error("ErrDatabaseUnavailable should be retryable")
	}
	if IsRetryable(ErrDuplicate) {
		t.Error("ErrDuplicate should not be retryable")
	}
	if IsRetryable(ErrInvalidEventID) {
		t.Error("ErrInvalidEventID should not be retryable")
	}
}

func TestTypeCheckers(t *testing.T) {
	if !IsValidation(ErrInvalidEventID) {
		t.Error("ErrInvalidEventID should be a validation error")
	}
	if !IsAuthorization(ErrBanned) {
		t.Error("ErrBanned should be an authorization error")
	}
	if !IsProcessing(ErrDuplicate) {
		t.Error("ErrDuplicate should be a processing error")
	}
	if !IsPolicy(ErrKindBlocked) {
		t.Error("ErrKindBlocked should be a policy error")
	}
	if !IsStorage(ErrDatabaseUnavailable) {
		t.Error("ErrDatabaseUnavailable should be a storage error")
	}
}

func TestNeedsAuthHelper(t *testing.T) {
	if !NeedsAuth(ErrAuthRequired) {
		t.Error("ErrAuthRequired should need auth")
	}
	if NeedsAuth(ErrBanned) {
		t.Error("ErrBanned should not need auth")
	}
	if NeedsAuth(ErrInvalidEventID) {
		t.Error("validation errors should not need auth")
	}
}

func TestWithCause(t *testing.T) {
	cause := errors.New("underlying error")
	err := ErrDatabaseUnavailable.Base.WithCause(cause)
	if err.Unwrap() != cause {
		t.Error("WithCause should set the cause")
	}
	if err.Error() != "database not available: underlying error" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestWithMessage(t *testing.T) {
	err := ErrInvalidEventID.Base.WithMessage("custom message")
	if err.Error() != "custom message" {
		t.Errorf("expected custom message, got %s", err.Error())
	}
	if err.Code() != "INVALID_ID" {
		t.Error("code should be preserved")
	}
}

func TestWithPubkey(t *testing.T) {
	pubkey := []byte{1, 2, 3, 4}
	err := ErrBanned.WithPubkey(pubkey)
	if len(err.Pubkey) != 4 {
		t.Error("pubkey should be set")
	}
}

func TestWithEventID(t *testing.T) {
	eventID := []byte{1, 2, 3, 4}
	err := ErrDuplicate.WithEventID(eventID)
	if len(err.EventID) != 4 {
		t.Error("event ID should be set")
	}
}

func TestWithKind(t *testing.T) {
	err := ErrDuplicate.WithKind(30023)
	if err.Kind != 30023 {
		t.Errorf("expected kind 30023, got %d", err.Kind)
	}
}
