package httpauth

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/interfaces/signer"
	"next.orly.dev/pkg/lol/chk"
)

const (
	HeaderKey   = "Authorization"
	NIP98Prefix = "Nostr"
)

// MakeNIP98Event creates a new NIP-98 event. If expiry is given, method is
// ignored; otherwise either option is the same.
func MakeNIP98Event(u, method, hash string, expiry int64) (ev *event.E) {
	var t []*tag.T
	t = append(t, tag.NewFromAny("u", u))
	if expiry > 0 {
		t = append(
			t,
			tag.NewFromAny("expiration", timestamp.FromUnix(expiry).String()),
		)
	} else {
		t = append(
			t,
			tag.NewFromAny("method", strings.ToUpper(method)),
		)
	}
	if hash != "" {
		t = append(t, tag.NewFromAny("payload", hash))
	}
	ev = &event.E{
		CreatedAt: timestamp.Now().V,
		Kind:      kind.HTTPAuth.K,
		Tags:      tag.NewS(t...),
	}
	return
}

func CreateNIP98Blob(
	ur, method, hash string, expiry int64, sign signer.I,
) (blob string, err error) {
	ev := MakeNIP98Event(ur, method, hash, expiry)
	if err = ev.Sign(sign); chk.E(err) {
		return
	}
	// log.T.F("nip-98 http auth event:\n%s\n", ev.SerializeIndented())
	blob = base64.URLEncoding.EncodeToString(ev.Serialize())
	return
}

// AddNIP98Header creates a NIP-98 http auth event and adds the standard header to a provided
// http.Request.
func AddNIP98Header(
	r *http.Request, ur *url.URL, method, hash string,
	sign signer.I, expiry int64,
) (err error) {
	var b64 string
	if b64, err = CreateNIP98Blob(
		ur.String(), method, hash, expiry, sign,
	); chk.E(err) {
		return
	}
	r.Header.Add(HeaderKey, "Nostr "+b64)
	return
}
