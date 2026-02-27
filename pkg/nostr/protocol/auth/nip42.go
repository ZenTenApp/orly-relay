package auth

import (
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/utils"
	"git.mleku.dev/mleku/nostr/utils/normalize"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
)

// GenerateChallenge creates a reasonable, 16-byte base64 challenge string
func GenerateChallenge() (b []byte) {
	bb := make([]byte, 12)
	b = make([]byte, 16)
	_, _ = rand.Read(bb)
	base64.URLEncoding.Encode(b, bb)
	return
}

// CreateUnsigned creates an event which should be sent via an "AUTH" command.
// If the authentication succeeds, the user will be authenticated as a pubkey.
func CreateUnsigned(pubkey, challenge []byte, relayURL string) (ev *event.E) {
	return &event.E{
		Pubkey:    pubkey,
		CreatedAt: time.Now().Unix(),
		Kind:      kind.ClientAuthentication.K,
		Tags: tag.NewS(
			tag.NewFromAny("relay", relayURL),
			tag.NewFromAny("challenge", string(challenge)),
		),
	}
}

// helper function for ValidateAuthEvent.
// Normalizes the URL to ensure it has a proper ws:// or wss:// scheme before parsing.
func parseURL(input string) (*url.URL, error) {
	// Use normalize.URL to ensure the URL has a proper scheme
	// This handles cases like "relay.example.com:3334" which url.Parse
	// would incorrectly interpret as scheme="relay.example.com" opaque="3334"
	normalized := string(normalize.URL(input))
	return url.Parse(
		strings.ToLower(
			strings.TrimSuffix(normalized, "/"),
		),
	)
}

var (
	// ChallengeTag is the tag for the challenge in a NIP-42 auth event
	// (prevents relay attacks).
	ChallengeTag = []byte("challenge")
	// RelayTag is the relay tag for a NIP-42 auth event (prevents cross-server
	// attacks).
	RelayTag = []byte("relay")
)

// Validate checks whether an event is a valid NIP-42 event for a given
// challenge and relayURL. The result of the validation is encoded in the ok
// bool.
func Validate(evt *event.E, challenge []byte, relayURL string) (
	ok bool, err error,
) {
	if evt.Kind != kind.ClientAuthentication.K {
		err = errorf.E(
			"event incorrect kind for auth: %d %s",
			evt.Kind, kind.GetString(evt.Kind),
		)
		return
	}
	if evt.Tags.GetFirst(ChallengeTag) == nil {
		err = errorf.E("challenge tag missing from auth response")
		return
	}
	if !utils.FastEqual(challenge, evt.Tags.GetFirst(ChallengeTag).Value()) {
		err = errorf.E("challenge tag incorrect from auth response")
		return
	}
	var expected, found *url.URL
	if expected, err = parseURL(relayURL); chk.D(err) {
		return
	}
	r := evt.Tags.
		GetFirst(RelayTag).Value()
	if len(r) == 0 {
		err = errorf.E("relay tag missing from auth response")
		return
	}
	if found, err = parseURL(string(r)); chk.D(err) {
		err = errorf.E("error parsing relay url: %s", err)
		return
	}
	// Allow both ws:// and wss:// schemes when behind a reverse proxy
	// This handles cases where the relay expects ws:// but receives wss:// from clients
	// connecting through HTTPS proxies
	if expected.Scheme != found.Scheme {
		// Check if this is a ws/wss scheme mismatch (acceptable behind proxy)
		if (expected.Scheme == "ws" && found.Scheme == "wss") ||
			(expected.Scheme == "wss" && found.Scheme == "ws") {
			// This is acceptable when behind a reverse proxy
			// The client will always send wss:// when connecting through HTTPS
		} else {
			err = errorf.E(
				"HTTP Scheme incorrect: expected '%s' got '%s",
				expected.Scheme, found.Scheme,
			)
			return
		}
	}
	if expected.Host != found.Host {
		err = errorf.E(
			"HTTP Host incorrect: expected '%s' got '%s",
			expected.Host, found.Host,
		)
		return
	}
	if expected.Path != found.Path {
		err = errorf.E(
			"HTTP Path incorrect: expected '%s' got '%s",
			expected.Path, found.Path,
		)
		return
	}

	now := time.Now().Unix()
	ca := evt.CreatedAt
	if ca > now+10*60 || ca < now-10*60 {
		err = errorf.E(
			"auth event more than 10 minutes before or after current time",
		)
		return
	}
	// save for last, as it is the most expensive operation
	return evt.Verify()
}
