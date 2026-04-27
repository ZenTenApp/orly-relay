package relayinfo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"git.smesh.lol/orly/pkg/nostr/utils/normalize"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
)

// Fetch fetches the NIP-11 Info.
func Fetch(c context.Context, u []byte) (info *T, err error) {
	if _, ok := c.Deadline(); !ok {
		// if no timeout is set, force it to 7 seconds
		var cancel context.CancelFunc
		c, cancel = context.WithTimeout(c, 7*time.Second)
		defer cancel()
	}
	u = normalize.URL(u)
	var req *http.Request
	if req, err = http.NewRequestWithContext(
		c, http.MethodGet, string(u), nil,
	); chk.E(err) {
		return
	}
	// add the NIP-11 header
	req.Header.Add("Accept", "application/nostr+json")
	// send the response
	var resp *http.Response
	if resp, err = http.DefaultClient.Do(req); chk.E(err) {
		err = errorf.E("request failed: %w", err)
		return
	}
	defer chk.E(resp.Body.Close())
	var b []byte
	if b, err = io.ReadAll(resp.Body); chk.E(err) {
		return
	}
	info = &T{}
	if err = json.Unmarshal(b, info); chk.E(err) {
		return
	}
	return
}
