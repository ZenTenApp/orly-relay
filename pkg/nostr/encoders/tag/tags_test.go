package tag

import (
	"testing"

	"next.orly.dev/pkg/nostr/utils"
	"next.orly.dev/pkg/lol/chk"
	"lukechampine.com/frand"
)

func TestSMarshalUnmarshal(t *testing.T) {
	for _ = range 100 {
		tgs := new(S)
		n := frand.Intn(8)
		for _ = range n {
			n := frand.Intn(8)
			tg := New()
			for _ = range n {
				b1 := make([]byte, frand.Intn(8))
				_, _ = frand.Read(b1)
				tg.T = append(tg.T, b1)
			}
			*tgs = append(*tgs, tg)
		}
		tgsb, _ := tgs.MarshalJSON()
		var tbc []byte
		tbc = append(tbc, tgsb...)
		tgs2 := new(S)
		if err := tgs2.UnmarshalJSON(tgsb); chk.E(err) {
			t.Fatal(err)
		}
		tgsb2, _ := tgs2.MarshalJSON()
		if !utils.FastEqual(tbc, tgsb2) {
			t.Fatalf("failed to re-marshal back original")
		}
	}
}
