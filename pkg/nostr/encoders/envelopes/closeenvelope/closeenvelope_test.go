package closeenvelope

import (
	"fmt"
	"math"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/envelopes"
	"git.mleku.dev/mleku/nostr/utils"
	"git.mleku.dev/mleku/nostr/utils/bufpool"
	"lol.mleku.dev/chk"
	"lukechampine.com/frand"
)

func TestMarshalUnmarshal(t *testing.T) {
	var err error
	for _ = range 1000 {
		rb, rb1, rb2 := bufpool.Get(), bufpool.Get(), bufpool.Get()
		s := []byte(fmt.Sprintf("sub:%d", frand.Intn(math.MaxInt64)))
		req := NewFrom(s)
		rb = req.Marshal(rb)
		rb1 = append(rb1, rb...)
		var rem []byte
		var l string
		if l, rb, err = envelopes.Identify(rb); chk.E(err) {
			t.Fatal(err)
		}
		if l != L {
			t.Fatalf("invalid sentinel %s, expect %s", l, L)
		}
		req2 := New()
		if rem, err = req2.Unmarshal(rb); chk.E(err) {
			t.Fatal(err)
		}
		// log.I.Ln(req2.ID)
		if len(rem) > 0 {
			t.Fatalf(
				"unmarshal failed, remainder\n%d %s",
				len(rem), rem,
			)
		}
		rb2 = req2.Marshal(rb2)
		if !utils.FastEqual(rb1, rb2) {
			if len(rb1) != len(rb2) {
				t.Fatalf(
					"unmarshal failed, different lengths\n%d %s\n%d %s\n",
					len(rb1), rb1, len(rb2), rb2,
				)
			}
			for i := range rb1 {
				if rb1[i] != rb2[i] {
					t.Fatalf(
						"unmarshal failed, difference at position %d\n%d %s\n%s\n%d %s\n%s\n",
						i, len(rb1), rb1[:i], rb1[i:], len(rb2), rb2[:i],
						rb2[i:],
					)
				}
			}
			t.Fatalf(
				"unmarshal failed\n%d %s\n%d %s\n",
				len(rb1), rb1, len(rb2), rb2,
			)
			bufpool.Put(rb1)
			bufpool.Put(rb2)
			bufpool.Put(rb)
		}
	}
}
