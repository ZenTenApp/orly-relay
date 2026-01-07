// Package bunker implements NIP-46 remote signing with Cashu token authentication.
package bunker

import (
	"context"

	"next.orly.dev/pkg/acl"
	acliface "next.orly.dev/pkg/interfaces/acl"
	cashuiface "next.orly.dev/pkg/interfaces/cashu"
	"next.orly.dev/pkg/cashu/token"
)

// ACLAuthzChecker adapts ORLY's ACL system to cashu.AuthzChecker.
// This allows the Cashu token system to use the existing ACL for authorization.
type ACLAuthzChecker struct {
	// ScopeRequirements maps scopes to required access levels.
	// If not set, defaults are used.
	ScopeRequirements map[string]string
}

// NewACLAuthzChecker creates a new ACL-based authorization checker.
func NewACLAuthzChecker() *ACLAuthzChecker {
	return &ACLAuthzChecker{
		ScopeRequirements: map[string]string{
			token.ScopeRelay:   acliface.Write, // Relay access requires write
			token.ScopeNIP46:   acliface.Write, // Bunker access requires write
			token.ScopeBlossom: acliface.Write, // Blossom access requires write
			token.ScopeAPI:     acliface.Admin, // API access requires admin
			token.ScopeNRC:     acliface.Write, // NRC tunnel access requires write
		},
	}
}

// CheckAuthorization checks if a pubkey is authorized for a scope.
func (a *ACLAuthzChecker) CheckAuthorization(ctx context.Context, pubkey []byte, scope string, remoteAddr string) error {
	// Get access level from ACL registry
	level := acl.Registry.GetAccessLevel(pubkey, remoteAddr)

	// Check against required level for scope
	requiredLevel, ok := a.ScopeRequirements[scope]
	if !ok {
		// Default to write access for unknown scopes
		requiredLevel = acliface.Write
	}

	if !hasAccessLevel(level, requiredLevel) {
		return cashuiface.NewAuthzError(
			cashuiface.ErrCodeInsufficientAccess,
			"insufficient access level for scope "+scope,
		)
	}

	// Check for banned/blocked status
	if level == "banned" {
		return cashuiface.ErrBanned
	}
	if level == "blocked" {
		return cashuiface.ErrBlocked
	}

	return nil
}

// ReauthorizationEnabled returns true - we always re-check ACL on each verification.
func (a *ACLAuthzChecker) ReauthorizationEnabled() bool {
	return true
}

// hasAccessLevel checks if the actual level meets or exceeds the required level.
func hasAccessLevel(actual, required string) bool {
	levels := map[string]int{
		acliface.None:  0,
		"banned":       0,
		"blocked":      0,
		acliface.Read:  1,
		acliface.Write: 2,
		acliface.Admin: 3,
		acliface.Owner: 4,
	}

	actualLevel, aok := levels[actual]
	requiredLevel, rok := levels[required]

	if !aok || !rok {
		return false
	}

	return actualLevel >= requiredLevel
}

// SetScopeRequirement sets the required access level for a scope.
func (a *ACLAuthzChecker) SetScopeRequirement(scope, level string) {
	if a.ScopeRequirements == nil {
		a.ScopeRequirements = make(map[string]string)
	}
	a.ScopeRequirements[scope] = level
}

// Ensure ACLAuthzChecker implements both interfaces.
var _ cashuiface.AuthzChecker = (*ACLAuthzChecker)(nil)
var _ cashuiface.ReauthorizationChecker = (*ACLAuthzChecker)(nil)
