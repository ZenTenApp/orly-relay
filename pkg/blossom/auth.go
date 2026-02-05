package blossom

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/ints"
)

const (
	// BlossomAuthKind is the Nostr event kind for Blossom authorization events (BUD-01)
	BlossomAuthKind = 24242
	// AuthorizationHeader is the HTTP header name for authorization
	AuthorizationHeader = "Authorization"
	// NostrAuthPrefix is the prefix for Nostr authorization scheme
	NostrAuthPrefix = "Nostr"
)

// AuthEvent represents a validated authorization event
type AuthEvent struct {
	Event   *event.E
	Pubkey  []byte
	Verb    string
	Expires int64
}

// ExtractAuthEvent extracts and parses a kind 24242 authorization event from the Authorization header
func ExtractAuthEvent(r *http.Request) (ev *event.E, err error) {
	authHeader := r.Header.Get(AuthorizationHeader)
	if authHeader == "" {
		err = errorf.E("missing Authorization header")
		return
	}

	// Parse "Nostr <base64>" format
	if !strings.HasPrefix(authHeader, NostrAuthPrefix+" ") {
		err = errorf.E("invalid Authorization scheme, expected 'Nostr'")
		return
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		err = errorf.E("invalid Authorization header format")
		return
	}

	var evb []byte
	if evb, err = base64.StdEncoding.DecodeString(parts[1]); chk.E(err) {
		return
	}

	ev = event.New()
	var rem []byte
	if rem, err = ev.Unmarshal(evb); chk.E(err) {
		return
	}

	if len(rem) > 0 {
		err = errorf.E("unexpected trailing data in auth event")
		return
	}

	return
}

// ValidateAuthEvent validates a kind 24242 authorization event according to BUD-01
func ValidateAuthEvent(
	r *http.Request, verb string, sha256Hash []byte,
) (authEv *AuthEvent, err error) {
	var ev *event.E
	if ev, err = ExtractAuthEvent(r); chk.E(err) {
		return
	}

	// 1. The kind must be 24242
	if ev.Kind != BlossomAuthKind {
		err = errorf.E(
			"invalid kind %d in authorization event, require %d",
			ev.Kind, BlossomAuthKind,
		)
		return
	}

	// 2. created_at must be in the past
	now := time.Now().Unix()
	if ev.CreatedAt > now {
		err = errorf.E(
			"authorization event created_at %d is in the future (now: %d)",
			ev.CreatedAt, now,
		)
		return
	}

	// 3. Check expiration tag (must be set and in the future)
	expTags := ev.Tags.GetAll([]byte("expiration"))
	if len(expTags) == 0 {
		err = errorf.E("authorization event missing expiration tag")
		return
	}
	if len(expTags) > 1 {
		err = errorf.E("authorization event has multiple expiration tags")
		return
	}

	expInt := ints.New(0)
	var rem []byte
	if rem, err = expInt.Unmarshal(expTags[0].Value()); chk.E(err) {
		return
	}
	if len(rem) > 0 {
		err = errorf.E("unexpected trailing data in expiration tag")
		return
	}

	expiration := expInt.Int64()
	if expiration <= now {
		err = errorf.E(
			"authorization event expired: expiration %d <= now %d",
			expiration, now,
		)
		return
	}

	// 4. The t tag must have a verb matching the intended action
	tTags := ev.Tags.GetAll([]byte("t"))
	if len(tTags) == 0 {
		err = errorf.E("authorization event missing 't' tag")
		return
	}
	if len(tTags) > 1 {
		err = errorf.E("authorization event has multiple 't' tags")
		return
	}

	eventVerb := string(tTags[0].Value())
	// If verb is non-empty, verify it matches the event verb
	// Empty verb means "don't check the verb" (used by GetPubkeyFromRequest)
	if verb != "" && eventVerb != verb {
		err = errorf.E(
			"authorization event verb '%s' does not match required verb '%s'",
			eventVerb, verb,
		)
		return
	}

	// 5. If sha256Hash is provided, verify at least one x tag matches
	if sha256Hash != nil && len(sha256Hash) > 0 {
		sha256Hex := hex.Enc(sha256Hash)
		xTags := ev.Tags.GetAll([]byte("x"))
		if len(xTags) == 0 {
			err = errorf.E(
				"authorization event missing 'x' tag for SHA256 hash %s",
				sha256Hex,
			)
			return
		}

		found := false
		for _, xTag := range xTags {
			if string(xTag.Value()) == sha256Hex {
				found = true
				break
			}
		}

		if !found {
			err = errorf.E(
				"authorization event has no 'x' tag matching SHA256 hash %s",
				sha256Hex,
			)
			return
		}
	}

	// 6. Verify event signature
	var valid bool
	if valid, err = ev.Verify(); chk.E(err) {
		return
	}
	if !valid {
		err = errorf.E("authorization event signature verification failed")
		return
	}

	authEv = &AuthEvent{
		Event:   ev,
		Pubkey:  ev.Pubkey,
		Verb:    eventVerb,
		Expires: expiration,
	}

	return
}

// ValidateAuthEventOptional validates authorization but returns nil if no auth header is present
// This is used for endpoints where authorization is optional
func ValidateAuthEventOptional(
	r *http.Request, verb string, sha256Hash []byte,
) (authEv *AuthEvent, err error) {
	authHeader := r.Header.Get(AuthorizationHeader)
	if authHeader == "" {
		// No authorization provided, but that's OK for optional endpoints
		return nil, nil
	}

	return ValidateAuthEvent(r, verb, sha256Hash)
}

// ValidateAuthEventForGet validates authorization for GET requests (BUD-01)
// GET requests may have either:
// - A server tag matching the server URL
// - At least one x tag matching the blob hash
func ValidateAuthEventForGet(
	r *http.Request, serverURL string, sha256Hash []byte,
) (authEv *AuthEvent, err error) {
	var ev *event.E
	if ev, err = ExtractAuthEvent(r); chk.E(err) {
		return
	}

	// Basic validation
	if authEv, err = ValidateAuthEvent(r, "get", sha256Hash); chk.E(err) {
		return
	}

	// For GET requests, check server tag or x tag
	serverTags := ev.Tags.GetAll([]byte("server"))
	xTags := ev.Tags.GetAll([]byte("x"))

	// If server tag exists, verify it matches
	if len(serverTags) > 0 {
		serverTagValue := string(serverTags[0].Value())
		if !strings.HasPrefix(serverURL, serverTagValue) {
			err = errorf.E(
				"server tag '%s' does not match server URL '%s'",
				serverTagValue, serverURL,
			)
			return
		}
		return
	}

	// Otherwise, verify at least one x tag matches the hash
	if sha256Hash != nil && len(sha256Hash) > 0 {
		sha256Hex := hex.Enc(sha256Hash)
		found := false
		for _, xTag := range xTags {
			if string(xTag.Value()) == sha256Hex {
				found = true
				break
			}
		}
		if !found {
			err = errorf.E(
				"no 'x' tag matching SHA256 hash %s",
				sha256Hex,
			)
			return
		}
	} else if len(xTags) == 0 {
		err = errorf.E(
			"authorization event must have either 'server' tag or 'x' tag",
		)
		return
	}

	return
}

// ValidateAuthEventForDelete validates authorization for DELETE requests (BUD-02)
// If requireServerTag is true, the auth event must include a 'server' tag matching the serverURL
// This prevents cross-server replay attacks where a malicious server replays delete auth events
func ValidateAuthEventForDelete(
	r *http.Request, serverURL string, sha256Hash []byte, requireServerTag bool,
) (authEv *AuthEvent, err error) {
	// First do the standard validation
	if authEv, err = ValidateAuthEvent(r, "delete", sha256Hash); chk.E(err) {
		return
	}

	// If server tag is not required, we're done
	if !requireServerTag {
		return
	}

	// Extract event again to check server tags
	var ev *event.E
	if ev, err = ExtractAuthEvent(r); chk.E(err) {
		return
	}

	// Check for server tag
	serverTags := ev.Tags.GetAll([]byte("server"))
	if len(serverTags) == 0 {
		err = errorf.E(
			"delete authorization requires 'server' tag for replay protection",
		)
		return
	}

	// Verify at least one server tag matches
	found := false
	for _, serverTag := range serverTags {
		serverTagValue := string(serverTag.Value())
		if strings.HasPrefix(serverURL, serverTagValue) {
			found = true
			break
		}
	}

	if !found {
		err = errorf.E(
			"no 'server' tag matches this server URL '%s'",
			serverURL,
		)
		return
	}

	return
}

// GetPubkeyFromRequest extracts pubkey from Authorization header if present
func GetPubkeyFromRequest(r *http.Request) (pubkey []byte, err error) {
	authHeader := r.Header.Get(AuthorizationHeader)
	if authHeader == "" {
		return nil, nil
	}

	authEv, err := ValidateAuthEventOptional(r, "", nil)
	if err != nil {
		// If validation fails, return empty pubkey but no error
		// This allows endpoints to work without auth
		return nil, nil
	}

	if authEv != nil {
		return authEv.Pubkey, nil
	}

	return nil, nil
}
