package atag

import (
	"math"
	"testing"

	"git.mleku.dev/mleku/nostr/crypto/ec/schnorr"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/utils"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"lukechampine.com/frand"
)

func TestT_Marshal_Unmarshal(t *testing.T) {
	k := kind.New(frand.Intn(math.MaxUint16))
	pk := make([]byte, schnorr.PubKeyBytesLen)
	frand.Read(pk)
	d := make([]byte, frand.Intn(10)+3)
	frand.Read(d)
	var dtag string
	dtag = hex.Enc(d)
	t1 := &T{
		Kind:   k,
		Pubkey: pk,
		DTag:   []byte(dtag),
	}
	b1 := t1.Marshal(nil)
	log.I.F("%s", b1)
	t2 := &T{}
	var r []byte
	var err error
	if r, err = t2.Unmarshal(b1); chk.E(err) {
		t.Fatal(err)
	}
	if len(r) > 0 {
		log.I.S(r)
		t.Fatalf("remainder")
	}
	b2 := t2.Marshal(nil)
	if !utils.FastEqual(b1, b2) {
		t.Fatalf("failed to re-marshal back original")
	}
}
