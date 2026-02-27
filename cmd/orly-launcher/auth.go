package main

import (
	"encoding/hex"
	"net/http"
	"strings"

	"next.orly.dev/pkg/nostr/httpauth"
	"next.orly.dev/pkg/lol/chk"
)

// AuthMiddleware provides NIP-98 authentication for the admin API.
type AuthMiddleware struct {
	owners map[string]struct{}
}

// NewAuthMiddleware creates a new auth middleware with the given owner pubkeys.
func NewAuthMiddleware(owners []string) *AuthMiddleware {
	ownerMap := make(map[string]struct{}, len(owners))
	for _, o := range owners {
		// Normalize to lowercase hex
		ownerMap[strings.ToLower(o)] = struct{}{}
	}
	return &AuthMiddleware{owners: ownerMap}
}

// RequireAuth wraps a handler to require NIP-98 authentication from an owner.
func (a *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate NIP-98 authentication
		valid, pubkeyBytes, err := httpauth.CheckAuth(r)
		if chk.E(err) || !valid {
			errorMsg := "NIP-98 authentication required"
			if err != nil {
				errorMsg = err.Error()
			}
			http.Error(w, errorMsg, http.StatusUnauthorized)
			return
		}

		// Convert pubkey bytes to hex string
		pubkeyHex := hex.EncodeToString(pubkeyBytes)

		// Check if pubkey is in owners list
		if !a.IsOwner(pubkeyHex) {
			http.Error(w, "Not authorized - owner access required", http.StatusForbidden)
			return
		}

		// Authentication successful, call the next handler
		next(w, r)
	}
}

// IsOwner checks if the given pubkey is in the owners list.
func (a *AuthMiddleware) IsOwner(pubkey string) bool {
	if len(a.owners) == 0 {
		// No owners configured - deny all access
		return false
	}
	_, ok := a.owners[strings.ToLower(pubkey)]
	return ok
}

// AddOwner adds a pubkey to the owners list.
func (a *AuthMiddleware) AddOwner(pubkey string) {
	a.owners[strings.ToLower(pubkey)] = struct{}{}
}

// RemoveOwner removes a pubkey from the owners list.
func (a *AuthMiddleware) RemoveOwner(pubkey string) {
	delete(a.owners, strings.ToLower(pubkey))
}

// Owners returns the list of owner pubkeys.
func (a *AuthMiddleware) Owners() []string {
	owners := make([]string, 0, len(a.owners))
	for o := range a.owners {
		owners = append(owners, o)
	}
	return owners
}
