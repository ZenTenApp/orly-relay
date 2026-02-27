package httpauth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/ints"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
)

var ErrMissingKey = fmt.Errorf(
	"'%s' key missing from request header", HeaderKey,
)

// CheckAuth verifies a received http.Request has got a valid authentication
// event in it, with an optional specification for tolerance of before and
// after, and provides the public key that should be verified to be authorized
// to access the resource associated with the request.
func CheckAuth(r *http.Request, tolerance ...time.Duration) (
	valid bool,
	pubkey []byte, err error,
) {
	val := r.Header.Get(HeaderKey)
	if val == "" {
		err = ErrMissingKey
		valid = true
		return
	}
	if len(tolerance) == 0 {
		tolerance = append(tolerance, time.Minute)
	}
	// log.I.S(tolerance)
	if tolerance[0] == 0 {
		tolerance[0] = time.Minute
	}
	tolerate := int64(tolerance[0] / time.Second)
	log.T.C(func() string { return fmt.Sprintf("validating auth '%s'", val) })
	switch {
	case strings.HasPrefix(val, NIP98Prefix):
		split := strings.Split(val, " ")
		if len(split) == 1 {
			err = errorf.E(
				"missing nip-98 auth event from '%s' http header key: '%s'",
				HeaderKey, val,
			)
		}
		if len(split) > 2 {
			err = errorf.E(
				"extraneous content after second field space separated: %s",
				val,
			)
			return
		}
		var evb []byte
		if evb, err = base64.URLEncoding.DecodeString(split[1]); chk.E(err) {
			return
		}
		ev := event.New()
		var rem []byte
		if rem, err = ev.Unmarshal(evb); chk.E(err) {
			return
		}
		if len(rem) > 0 {
			err = errorf.E("rem", rem)
			return
		}
		// log.T.F("received http auth event:\n%s\n", ev.SerializeIndented())
		// The kind MUST be 27235.
		if ev.Kind != kind.HTTPAuth.K {
			err = errorf.E(
				"invalid kind %d %s in nip-98 http auth event, require %d %s",
				ev.Kind, kind.GetString(ev.Kind), kind.HTTPAuth.K,
				kind.HTTPAuth.Name(),
			)
			return
		}
		// if there is an expiration timestamp, check it supersedes the
		// created_at for validity.
		exp := ev.Tags.GetAll([]byte("expiration"))
		if len(exp) > 1 {
			err = errorf.E(
				"more than one \"expiration\" tag found",
			)
			return
		}
		var expiring bool
		if len(exp) == 1 {
			ex := ints.New(0)
			exp1 := exp[0]
			if rem, err = ex.Unmarshal(exp1.Value()); chk.E(err) {
				return
			}
			tn := time.Now().Unix()
			if tn > ex.Int64()+tolerate {
				err = errorf.E(
					"HTTP auth event is expired %d time now %d",
					tn, ex.Int64()+tolerate,
				)
				return
			}
			expiring = true
		} else {
			// The created_at timestamp MUST be within a reasonable time window
			// (suggestion 60 seconds)
			ts := ev.CreatedAt
			tn := time.Now().Unix()
			if ts < tn-tolerate || ts > tn+tolerate {
				err = errorf.E(
					"timestamp %d is more than %d seconds divergent from now %d",
					ts, tolerate, tn,
				)
				return
			}
		}
		ut := ev.Tags.GetAll([]byte("u"))
		if len(ut) > 1 {
			err = errorf.E(
				"more than one \"u\" tag found",
			)
			return
		}
		// The u tag MUST be exactly the same as the absolute request URL
		// (including query parameters).
		proto := r.URL.Scheme
		// if this came through a proxy, we need to get the protocol to match
		// the event
		if p := r.Header.Get("X-Forwarded-Proto"); p != "" {
			proto = p
		}
		if proto == "" {
			// Check if this is a direct TLS connection
			if r.TLS != nil {
				proto = "https"
			} else {
				proto = "http"
			}
		}
		fullUrl := proto + "://" + r.Host + r.URL.RequestURI()
		evUrl := string(ut[0].Value())
		log.T.F("full URL: %s event u tag value: %s", fullUrl, evUrl)
		if expiring {
			// if it is expiring, the URL only needs to be the same prefix to
			// allow its use with multiple endpoints.
			if !strings.HasPrefix(fullUrl, evUrl) {
				err = errorf.E(
					"request URL %s is not prefixed with the u tag URL %s",
					fullUrl, evUrl,
				)
				return
			}
		} else if fullUrl != evUrl {
			err = errorf.E(
				"request has URL %s but signed nip-98 event has url %s",
				fullUrl, string(ut[0].Value()),
			)
			return
		}
		if !expiring {
			// The method tag MUST be the same HTTP method used for the
			// requested resource.
			mt := ev.Tags.GetAll([]byte("method"))
			if len(mt) != 1 {
				err = errorf.E(
					"more than one \"method\" tag found",
				)
				return
			}
			if !strings.EqualFold(string(mt[0].Value()), r.Method) {
				err = errorf.E(
					"request has method %s but event has method %s",
					string(mt[0].Value()), r.Method,
				)
				return
			}
		}
		if valid, err = ev.Verify(); chk.E(err) {
			return
		}
		if !valid {
			return
		}
		pubkey = ev.Pubkey
	default:
		err = errorf.E("invalid '%s' value: '%s'", HeaderKey, val)
		return
	}

	return
}
