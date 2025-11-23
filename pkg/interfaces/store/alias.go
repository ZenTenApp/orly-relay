package store

import (
	"net/http"

	"git.mleku.dev/mleku/nostr/encoders/envelopes/okenvelope"
)

type Responder = http.ResponseWriter
type Req = *http.Request
type OK = okenvelope.T
