//go:build !(js && wasm)

package acl

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	AliasMinLength = 3
	AliasMaxLength = 32
)

// aliasPattern allows lowercase alphanumeric, dots, hyphens, underscores.
// Must start and end with alphanumeric.
var aliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*[a-z0-9]$`)

var reservedAliases = map[string]struct{}{
	"admin":         {},
	"postmaster":    {},
	"abuse":         {},
	"noreply":       {},
	"no-reply":      {},
	"root":          {},
	"info":          {},
	"support":       {},
	"webmaster":     {},
	"mailer-daemon": {},
	"bridge":        {},
	"marmot":        {},
	"help":          {},
	"subscribe":     {},
	"status":        {},
}

// ValidateAlias checks whether an alias is valid for use as an email local part.
func ValidateAlias(alias string) error {
	alias = strings.ToLower(alias)

	if len(alias) < AliasMinLength {
		return fmt.Errorf("alias must be at least %d characters", AliasMinLength)
	}
	if len(alias) > AliasMaxLength {
		return fmt.Errorf("alias must be at most %d characters", AliasMaxLength)
	}
	if !aliasPattern.MatchString(alias) {
		return fmt.Errorf("alias must contain only lowercase letters, numbers, dots, hyphens, and underscores, and must start/end with a letter or number")
	}
	if _, ok := reservedAliases[alias]; ok {
		return fmt.Errorf("alias %q is reserved", alias)
	}
	// Reject npub-like strings to avoid confusion
	if strings.HasPrefix(alias, "npub1") {
		return fmt.Errorf("alias cannot start with npub1")
	}
	return nil
}
