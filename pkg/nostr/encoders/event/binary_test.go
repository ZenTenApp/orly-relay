package event

import (
	"bufio"
	"bytes"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event/examples"
	"git.smesh.lol/orly/pkg/nostr/utils"
	"git.smesh.lol/orly/pkg/lol/chk"
)

func TestTMarshalBinary_UnmarshalBinary(t *testing.T) {
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	scanner.Buffer(make([]byte, 0, 1_000_000_000), 1_000_000_000)
	var rem, out []byte
	var err error
	now := time.Now()
	var counter int
	for scanner.Scan() {
		// Create new event objects and buffer for each iteration
		buf := new(bytes.Buffer)
		ea, eb := New(), New()

		chk.E(scanner.Err())
		b := scanner.Bytes()
		c := make([]byte, 0, len(b))
		c = append(c, b...)
		if rem, err = ea.Unmarshal(c); chk.E(err) {
			t.Fatal(err)
		}
		if len(rem) != 0 {
			t.Fatalf(
				"some of input remaining after marshal/unmarshal: '%s'",
				rem,
			)
		}
		// Reset buffer before marshaling
		buf.Reset()
		ea.MarshalBinary(buf)

		// Create a new buffer for unmarshaling
		buf2 := bytes.NewBuffer(buf.Bytes())
		if err = eb.UnmarshalBinary(buf2); chk.E(err) {
			t.Fatal(err)
		}

		// Marshal unmarshaled binary event back to JSON
		unmarshaledJSON := eb.Serialize()
		defer func(ev *E) {
			eb.Free()
		}(eb)

		// Compare the two JSON representations
		if !utils.FastEqual(b, unmarshaledJSON) {
			t.Fatalf(
				"JSON representations don't match after binary marshaling/unmarshaling:\nOriginal: %s\nUnmarshaled: %s",
				b, unmarshaledJSON,
			)
		}

		counter++
		out = out[:0]
	}
	chk.E(scanner.Err())
	t.Logf(
		"unmarshaled json, marshaled binary, unmarshaled binary, %d events in %v av %v per event",
		counter, time.Since(now), time.Since(now)/time.Duration(counter),
	)
}
