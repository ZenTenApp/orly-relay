// Package eventenvelope is a codec for the event Submission request EVENT envelope
// (client) and event Result (to a REQ) from a relay.
package eventenvelope

import (
	"io"

	"git.mleku.dev/mleku/nostr/encoders/envelopes"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/text"
	"git.mleku.dev/mleku/nostr/interfaces/codec"
	"git.mleku.dev/mleku/nostr/utils/constraints"
	"git.mleku.dev/mleku/nostr/utils/units"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
)

// L is the label associated with this type of codec.Envelope.
const L = "EVENT"

type I interface{ Id() []byte }

// Submission is a request from a client for a realy to store an event.
type Submission struct {
	*event.E
}

var _ codec.Envelope = (*Submission)(nil)

// NewSubmission creates an empty new eventenvelope.Submission.
func NewSubmission() *Submission { return &Submission{E: &event.E{}} }

// NewSubmissionWith creates a new eventenvelope.Submission with a provided event.E.
func NewSubmissionWith(ev *event.E) *Submission { return &Submission{E: ev} }

// Label returns the label of a event eventenvelope.Submission envelope.
func (en *Submission) Label() string { return L }

func (en *Submission) Id() []byte { return en.E.ID }

// Write the Submission to a provided io.Writer.
func (en *Submission) Write(w io.Writer) (err error) {
	_, err = w.Write(en.Marshal(nil))
	return
}

// Marshal an event Submission envelope in minified JSON, appending to a
// provided destination slice.
func (en *Submission) Marshal(dst []byte) (b []byte) {
	var err error
	// if the destination capacity is not large enough, allocate a new
	// destination slice.
	if en.EstimateSize() >= cap(dst) {
		dst = make([]byte, 0, en.EstimateSize()+units.Kb)
	}
	b = dst
	b = envelopes.Marshal(
		b, L,
		func(bst []byte) (o []byte) {
			o = bst
			o = en.E.Marshal(o)
			return
		},
	)
	_ = err
	return
}

// Unmarshal an event eventenvelope.Submission from minified JSON, returning the
// remainder after the end of the envelope.
func (en *Submission) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	en.E = event.New()
	if r, err = en.E.Unmarshal(r); chk.T(err) {
		return
	}
	// after parsing the event object, r points just after the event JSON
	// now skip to the end of the envelope (consume comma/closing bracket etc.)
	if r, err = envelopes.SkipToTheEnd(r); chk.E(err) {
		return
	}
	return
}

// ParseSubmission reads an event envelope Submission from minified JSON into a newly
// allocated eventenvelope.Submission.
func ParseSubmission(b []byte) (t *Submission, rem []byte, err error) {
	t = NewSubmission()
	if rem, err = t.Unmarshal(b); chk.E(err) {
		return
	}
	return
}

// Result is an event matching a filter associated with a subscription.
type Result struct {
	Subscription []byte
	Event        *event.E
}

var _ codec.Envelope = (*Result)(nil)

// NewResult creates a new empty eventenvelope.Result.
func NewResult() *Result { return &Result{} }

// NewResultWith creates a new eventenvelope.Result with a provided
// subscription.Id string and event.E.
func NewResultWith[V constraints.Bytes](s V, ev *event.E) (
	res *Result, err error,
) {
	if len(s) < 0 || len(s) > 64 {
		err = errorf.E("subscription id must be length > 0 and <= 64")
		return
	}
	return &Result{[]byte(s), ev}, nil
}

func (en *Result) Id() []byte { return en.Event.ID }

// Label returns the label of a event eventenvelope.Result envelope.
func (en *Result) Label() string { return L }

// Write the eventenvelope.Result to a provided io.Writer.
func (en *Result) Write(w io.Writer) (err error) {
	_, err = w.Write(en.Marshal(nil))
	return
}

// Marshal an eventenvelope.Result envelope in minified JSON, appending to a
// provided destination slice.
func (en *Result) Marshal(dst []byte) (b []byte) {
	// if the destination capacity is not large enough, allocate a new
	// destination slice.
	if en.Event.EstimateSize() >= cap(dst) {
		dst = make([]byte, 0, en.Event.EstimateSize()+units.Kb)
	}
	b = dst
	var err error
	b = envelopes.Marshal(
		b, L,
		func(bst []byte) (o []byte) {
			o = bst
			o = append(o, '"')
			o = append(o, en.Subscription...)
			o = append(o, '"')
			o = append(o, ',')
			o = en.Event.Marshal(o)
			return
		},
	)
	_ = err
	return
}

// Unmarshal an event Result envelope from minified JSON, returning the
// remainder after the end of the envelope.
func (en *Result) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	if en.Subscription, r, err = text.UnmarshalQuoted(r); chk.E(err) {
		return
	}
	en.Event = event.New()
	if r, err = en.Event.Unmarshal(r); err != nil {
		return
	}
	if r, err = envelopes.SkipToTheEnd(r); chk.E(err) {
		return
	}
	return
}

// ParseResult allocates a new eventenvelope.Result and unmarshalls an EVENT
// envelope into it.
func ParseResult(b []byte) (t *Result, rem []byte, err error) {
	t = NewResult()
	if rem, err = t.Unmarshal(b); err != nil {
		return
	}
	return
}
