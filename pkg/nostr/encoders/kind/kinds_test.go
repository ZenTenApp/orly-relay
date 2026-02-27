package kind

import (
	"testing"

	"lol.mleku.dev/chk"
	"lukechampine.com/frand"
)

func TestUnmarshalKindsArray(t *testing.T) {
	k := &S{make([]*K, 100)}
	for i := range k.K {
		k.K[i] = New(uint16(frand.Intn(65535)))
	}
	var dst []byte
	var err error
	if dst = k.Marshal(dst); chk.E(err) {
		t.Fatal(err)
	}
	k2 := &S{}
	var rem []byte
	if rem, err = k2.Unmarshal(dst); chk.E(err) {
		return
	}
	if len(rem) > 0 {
		t.Fatalf("failed to unmarshal, remnant afterwards '%s'", rem)
	}
	for i := range k.K {
		if *k.K[i] != *k2.K[i] {
			t.Fatalf(
				"failed to unmarshal at element %d; got %x, expected %x",
				i, k.K[i], k2.K[i],
			)
		}
	}
}
