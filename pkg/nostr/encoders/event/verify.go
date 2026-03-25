//go:build !js && !wasm

package event

import (
	"next.orly.dev/pkg/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/nostr/utils"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/lol/log"
)

// Verify an event is signed by the pubkey it contains. Uses
// github.com/bitcoin-core/secp256k1 if available for faster verification.
func (ev *E) Verify() (valid bool, err error) {
	var keys *p8k.Signer
	if keys, err = p8k.New(); chk.E(err) {
		return
	}
	if err = keys.InitPub(ev.Pubkey); chk.E(err) {
		return
	}
	if valid, err = keys.Verify(ev.ID, ev.Sig); chk.T(err) {
		// check that this isn't because of a bogus ID
		id := ev.GetIDBytes()
		if !utils.FastEqual(id, ev.ID) {
			log.E.Ln("event Subscription incorrect")
			ev.ID = id
			err = nil
			if valid, err = keys.Verify(ev.ID, ev.Sig); chk.E(err) {
				return
			}
			err = errorf.W("event Subscription incorrect but signature is valid on correct Subscription")
		}
		return
	}
	return
}
