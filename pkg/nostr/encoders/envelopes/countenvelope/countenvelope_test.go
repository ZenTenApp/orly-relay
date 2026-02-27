package countenvelope

import (
	"testing"

	"next.orly.dev/pkg/nostr/encoders/envelopes"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/utils"
	"next.orly.dev/pkg/nostr/utils/bufpool"
	"next.orly.dev/pkg/lol/chk"
	"lukechampine.com/frand"
)

func TestRequest(t *testing.T) {
	var err error
	for i := range 1000 {
		rb, rb1, rb2 := bufpool.Get(), bufpool.Get(), bufpool.Get()
		var f filter.S
		if f, err = filter.GenFilters(); chk.E(err) {
			t.Fatal(err)
		}
		s := utils.NewSubscription(i)
		req := NewRequest(s, f)
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
		}
		bufpool.Put(rb1)
		bufpool.Put(rb2)
		bufpool.Put(rb)
	}
}

func TestResponse(t *testing.T) {
	var err error
	for i := range 1000 {
		rb, rb1, rb2 := bufpool.Get(), bufpool.Get(), bufpool.Get()
		s := utils.NewSubscription(i)
		var res *Response
		if i&2 == 0 {
			if res, err = NewResponseFrom(
				s, frand.Intn(200), true,
			); chk.E(err) {
				t.Fatal(err)
			}
		} else {
			if res, err = NewResponseFrom(s, frand.Intn(200)); chk.E(err) {
				t.Fatal(err)
			}
		}
		rb = res.Marshal(rb)
		rb1 = append(rb1, rb...)
		var rem []byte
		var l string
		if l, rb, err = envelopes.Identify(rb); chk.E(err) {
			t.Fatal(err)
		}
		if l != L {
			t.Fatalf("invalid sentinel %s, expect %s", l, L)
		}
		res2 := NewResponse()
		if rem, err = res2.Unmarshal(rb); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) > 0 {
			t.Fatalf(
				"unmarshal failed, remainder\n%d %s",
				len(rem), rem,
			)
		}
		rb2 = res2.Marshal(rb2)
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
		}
		bufpool.Put(rb1)
		bufpool.Put(rb2)
		bufpool.Put(rb)
	}
}
