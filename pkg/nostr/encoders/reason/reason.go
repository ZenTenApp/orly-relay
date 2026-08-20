package reason

import (
	"bytes"
	"fmt"
	"strings"
)

// R is the machine-readable prefix before the colon in an OK or CLOSED envelope message.
// Below are the most common kinds that are mentioned in NIP-01.
type R []byte

var (
	AuthRequired = R("auth-required")
	PoW          = R("pow")
	Duplicate    = R("duplicate")
	Blocked      = R("blocked")
	RateLimited  = R("rate-limited")
	Invalid      = R("invalid")
	Error        = R("error")
	Unsupported  = R("unsupported")
	Restricted   = R("restricted")
)

// S returns the R as a string
func (r R) S() string { return string(r) }

// B returns the R as a byte slice.
func (r R) B() []byte { return r }

// IsPrefix returns whether a text contains the same R prefix.
func (r R) IsPrefix(reason []byte) bool {
	return bytes.HasPrefix(
		reason, r.B(),
	)
}

// F allows creation of a full R text with a printf style format.
func (r R) F(format string, params ...any) (o []byte) {
	return Msg(r, format, params...)
}

// Msg constructs a properly formatted message with a machine-readable prefix
// for OK and CLOSED envelopes.
func Msg(prefix R, format string, params ...any) (o []byte) {
	if len(prefix) < 1 {
		prefix = Error
	}
	return []byte(fmt.Sprintf(prefix.S()+": "+format, params...))
}

// knownPrefixes are the NIP-01 machine-readable OK/CLOSED prefixes.
var knownPrefixes = map[string]R{
	AuthRequired.S(): AuthRequired,
	PoW.S():          PoW,
	Duplicate.S():    Duplicate,
	Blocked.S():      Blocked,
	RateLimited.S():  RateLimited,
	Invalid.S():      Invalid,
	Error.S():        Error,
	Unsupported.S():  Unsupported,
	Restricted.S():   Restricted,
}

// Parse splits a NIP-01 reason into prefix and detail.
// found is true only when msg starts with a known prefix followed by ": ".
func Parse(msg string) (prefix R, detail string, found bool) {
	i := strings.Index(msg, ": ")
	if i <= 0 {
		return nil, msg, false
	}
	if p, ok := knownPrefixes[msg[:i]]; ok {
		return p, msg[i+2:], true
	}
	return nil, msg, false
}

// Split is Parse with a Blocked default when no known prefix is present.
func Split(msg string) (prefix R, detail string) {
	p, d, ok := Parse(msg)
	if !ok {
		return Blocked, msg
	}
	return p, d
}

// Ensure returns msg unchanged if it already has a known NIP-01 prefix.
// Empty msg becomes `fallback: rejected`. Otherwise fallback is prepended.
func Ensure(msg string, fallback R) string {
	if len(fallback) < 1 {
		fallback = Blocked
	}
	if msg == "" {
		return fallback.S() + ": rejected"
	}
	if _, _, found := Parse(msg); found {
		return msg
	}
	return fallback.S() + ": " + msg
}
