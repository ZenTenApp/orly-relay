package event

import (
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/lol/chk"
)

// Sign the event using the signer.I. Uses github.com/bitcoin-core/secp256k1 if
// available for much faster signatures.
//
// Note that this only populates the Pubkey, ID and Sig. The caller must
// set the CreatedAt timestamp as intended.
func (ev *E) Sign(keys signer.I) (err error) {
	// Copy Pub() and Sign() results — signers may return internal buffers
	// that are reused on subsequent calls (e.g. P256K1Signer.sigBuf).
	pub := keys.Pub()
	ev.Pubkey = make([]byte, len(pub))
	copy(ev.Pubkey, pub)
	ev.ID = ev.GetIDBytes()
	var sig []byte
	if sig, err = keys.Sign(ev.ID); chk.E(err) {
		return
	}
	ev.Sig = make([]byte, len(sig))
	copy(ev.Sig, sig)
	return
}
