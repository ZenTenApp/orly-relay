package store

import (
	"net/http"

	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes/okenvelope"
)

type Responder = http.ResponseWriter
type Req = *http.Request
type OK = okenvelope.T
