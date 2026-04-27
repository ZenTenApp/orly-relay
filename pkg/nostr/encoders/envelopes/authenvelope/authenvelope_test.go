package authenvelope

import (
	"testing"

	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"git.smesh.lol/orly/pkg/nostr/protocol/auth"
	"git.smesh.lol/orly/pkg/nostr/utils"
	"git.smesh.lol/orly/pkg/nostr/utils/bufpool"
	"git.smesh.lol/orly/pkg/lol/chk"
)

const relayURL = "wss://example.com"

func TestAuth(t *testing.T) {
	var err error
	signer := p8k.MustNew()
	if err = signer.Generate(); chk.E(err) {
		t.Fatal(err)
	}
	for _ = range 1000 {
		b1, b2, b3, b4 := bufpool.Get(), bufpool.Get(), bufpool.Get(), bufpool.Get()
		ch := auth.GenerateChallenge()
		chal := Challenge{Challenge: ch}
		b1 = chal.Marshal(b1)
		oChal := make([]byte, len(b1))
		copy(oChal, b1)
		var rem []byte
		var l string
		if l, b1, err = envelopes.Identify(b1); chk.E(err) {
			t.Fatal(err)
		}
		if l != L {
			t.Fatalf("invalid sentinel %s, expect %s", l, L)
		}
		c2 := NewChallenge()
		if rem, err = c2.Unmarshal(b1); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) != 0 {
			t.Fatalf("remainder should be empty\n%s", rem)
		}
		if !utils.FastEqual(chal.Challenge, c2.Challenge) {
			t.Fatalf(
				"challenge mismatch\n%s\n%s",
				chal.Challenge, c2.Challenge,
			)
		}
		b2 = c2.Marshal(b2)
		if !utils.FastEqual(oChal, b2) {
			t.Fatalf("challenge mismatch\n%s\n%s", oChal, b2)
		}
		resp := Response{
			Event: auth.CreateUnsigned(
				signer.Pub(), ch,
				relayURL,
			),
		}
		if err = resp.Event.Sign(signer); chk.E(err) {
			t.Fatal(err)
		}
		b3 = resp.Marshal(b3)
		oResp := make([]byte, len(b3))
		copy(oResp, b3)
		if l, b3, err = envelopes.Identify(b3); chk.E(err) {
			t.Fatal(err)
		}
		if l != L {
			t.Fatalf("invalid sentinel %s, expect %s", l, L)
		}
		r2 := NewResponse()
		if _, err = r2.Unmarshal(b3); chk.E(err) {
			t.Fatal(err)
		}
		b4 = r2.Marshal(b4)
		if !utils.FastEqual(oResp, b4) {
			t.Fatalf("challenge mismatch\n%s\n%s", oResp, b4)
		}
		bufpool.Put(b1)
		bufpool.Put(b2)
		bufpool.Put(b3)
		bufpool.Put(b4)
		oChal, oResp = oChal[:0], oResp[:0]
	}
}
