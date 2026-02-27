package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/nostr/encoders/envelopes/countenvelope"
	"next.orly.dev/pkg/nostr/utils/normalize"
)

// HandleCount processes a COUNT envelope by parsing the request, verifying
// permissions, invoking the database CountEvents for each provided filter, and
// responding with a COUNT response containing the aggregate count.
func (l *Listener) HandleCount(msg []byte) (err error) {
	log.D.F("HandleCount: START processing from %s", l.remote)

	// Parse the COUNT request
	env := countenvelope.New()
	if _, err = env.Unmarshal(msg); chk.E(err) {
		return normalize.Error.Errorf(err.Error())
	}
	log.D.C(func() string { return fmt.Sprintf("COUNT sub=%s filters=%d", env.Subscription, len(env.Filters)) })

	// If ACL is active, auth is required, or AuthToWrite is enabled, send a challenge (same as REQ path)
	if len(l.authedPubkey.Load()) != schnorr.PubKeyBytesLen && (acl.Registry.GetMode() != "none" || l.Config.AuthRequired || l.Config.AuthToWrite) {
		if err = authenvelope.NewChallengeWith(l.challenge.Load()).Write(l); chk.E(err) {
			return
		}
	}

	// Check read permissions
	accessLevel := acl.Registry.GetAccessLevel(l.authedPubkey.Load(), l.remote)

	// If auth is required but user is not authenticated, deny access
	if l.Config.AuthRequired && len(l.authedPubkey.Load()) == 0 {
		return errors.New("authentication required")
	}

	// If AuthToWrite is enabled, allow COUNT without auth (but still check ACL)
	if l.Config.AuthToWrite && len(l.authedPubkey.Load()) == 0 {
		// Allow unauthenticated COUNT when AuthToWrite is enabled
		// but still respect ACL access levels if ACL is active
		if acl.Registry.GetMode() != "none" {
			switch accessLevel {
			case "none", "blocked", "banned":
				return errors.New("auth required: user not authed or has no read access")
			}
		}
		// Allow the request to proceed without authentication
	} else {
		// Only check ACL access level if not already handled by AuthToWrite
		switch accessLevel {
		case "none":
			return errors.New("auth required: user not authed or has no read access")
		default:
			// allowed to read
		}
	}

	// Use a bounded context for counting, isolated from the connection context
	// to prevent count timeouts from affecting the long-lived websocket connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Aggregate count across all provided filters
	var total int
	var approx bool // database returns false per implementation
	for _, f := range env.Filters {
		if f == nil {
			continue
		}
		var cnt int
		var a bool
		cnt, a, err = l.DB.CountEvents(ctx, f)
		if chk.E(err) {
			return
		}
		total += cnt
		approx = approx || a
	}

	// Build and send COUNT response
	var res *countenvelope.Response
	if res, err = countenvelope.NewResponseFrom(env.Subscription, total, approx); chk.E(err) {
		return
	}
	if err = res.Write(l); chk.E(err) {
		return
	}

	log.D.F("HandleCount: COMPLETED processing from %s count=%d approx=%v", l.remote, total, approx)
	return nil
}
