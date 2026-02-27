package eventenvelope

import (
	"bufio"
	"bytes"
	"testing"

	"next.orly.dev/pkg/nostr/encoders/envelopes"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/event/examples"
	"next.orly.dev/pkg/nostr/utils"
	"next.orly.dev/pkg/nostr/utils/bufpool"
	"next.orly.dev/pkg/lol/chk"
)

func TestSubmission(t *testing.T) {
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	var err error
	for scanner.Scan() {
		c, rem, out := bufpool.Get(), bufpool.Get(), bufpool.Get()
		b := scanner.Bytes()
		ev := event.New()
		if _, err = ev.Unmarshal(b); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) != 0 {
			t.Fatalf(
				"some of input remaining after marshal/unmarshal: '%s'",
				rem,
			)
		}
		rem = rem[:0]
		ea := NewSubmissionWith(ev)
		rem = ea.Marshal(rem)
		c = append(c, rem...)
		var l string
		if l, rem, err = envelopes.Identify(rem); chk.E(err) {
			t.Fatal(err)
		}
		if l != L {
			t.Fatalf("invalid sentinel %s, expect %s", l, L)
		}
		if rem, err = ea.Unmarshal(rem); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) != 0 {
			t.Fatalf(
				"some of input remaining after marshal/unmarshal: '%s'",
				rem,
			)
		}
		out = ea.Marshal(out)
		if !utils.FastEqual(out, c) {
			t.Fatalf("mismatched output\n%s\n\n%s\n", c, out)
		}
		bufpool.Put(c)
		bufpool.Put(rem)
		bufpool.Put(out)
	}
}

func TestResult(t *testing.T) {
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	var err error
	var count int
	for scanner.Scan() {
		c, rem, out := bufpool.Get(), bufpool.Get(), bufpool.Get()
		count++
		b := scanner.Bytes()
		ev := event.New()
		if _, err = ev.Unmarshal(b); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) != 0 {
			t.Fatalf(
				"some of input remaining after marshal/unmarshal: '%s'",
				rem,
			)
		}
		var ea *Result
		if ea, err = NewResultWith(
			utils.NewSubscription(count), ev,
		); chk.E(err) {
			t.Fatal(err)
		}
		rem = ea.Marshal(rem)
		c = append(c, rem...)
		var l string
		if l, rem, err = envelopes.Identify(rem); chk.E(err) {
			t.Fatal(err)
		}
		if l != L {
			t.Fatalf("invalid sentinel %s, expect %s", l, L)
		}
		if rem, err = ea.Unmarshal(rem); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) != 0 {
			t.Fatalf(
				"some of input remaining after marshal/unmarshal: '%s'",
				rem,
			)
		}
		out = ea.Marshal(out)
		if !utils.FastEqual(out, c) {
			t.Fatalf("mismatched output\n%s\n\n%s\n", c, out)
		}
		bufpool.Put(c)
		bufpool.Put(rem)
		bufpool.Put(out)
	}
}
