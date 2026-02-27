// Package reqenvelope is a message from a client to a relay containing a
// subscription identifier and an array of filters to search for events.
package reqenvelope

import (
	"io"

	"git.mleku.dev/mleku/nostr/encoders/envelopes"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/text"
	"git.mleku.dev/mleku/nostr/interfaces/codec"
	"git.mleku.dev/mleku/nostr/utils/constraints"
	"lol.mleku.dev/chk"
)

// L is the label associated with this type of codec.Envelope.
const L = "REQ"

// T is a filter/subscription request envelope that can contain multiple
// filters. These prompt the relay to search its event store and return all
// events and if the limit is unset or large enough, it will continue to return
// newly received events after it returns an eoseenvelope.T.
type T struct {
	Subscription []byte
	Filters      *filter.S
}

var _ codec.Envelope = (*T)(nil)

// New creates a new reqenvelope.T with a standard subscription.Id and empty
// filters.T.
func New() *T { return new(T) }

// NewFrom creates a new reqenvelope.T with a provided subscription.Id and
// filters.T.
func NewFrom(id []byte, ff *filter.S) *T {
	return &T{
		Subscription: id,
		Filters:      ff,
	}
}

func NewWithId[V constraints.Bytes](id V, ff *filter.S) (sub *T) {
	return &T{
		Subscription: []byte(id),
		Filters:      ff,
	}
}

// Label returns the label of a reqenvelope.T.
func (en *T) Label() string { return L }

// Write the REQ T to a provided io.Writer.
func (en *T) Write(w io.Writer) (err error) {
	_, err = w.Write(en.Marshal(nil))
	return
}

// Marshal a reqenvelope.T envelope into minified JSON, appending to a provided
// destination slice. Note that this ensures correct string escaping on the
// subscription.Id field.
func (en *T) Marshal(dst []byte) (b []byte) {
	var err error
	_ = err
	b = dst
	b = envelopes.Marshal(
		b, L,
		func(bst []byte) (o []byte) {
			o = bst
			o = append(o, '"')
			o = append(o, en.Subscription...)
			o = append(o, '"')
			for _, f := range *en.Filters {
				o = append(o, ',')
				o = f.Marshal(o)
			}
			return
		},
	)
	return
}

// Unmarshal into a reqenvelope.T from minified JSON, returning the remainder
// after the end of the envelope. Note that this ensures the subscription.Id
// string is correctly unescaped by NIP-01 escaping rules.
func (en *T) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	if en.Subscription, r, err = text.UnmarshalQuoted(r); chk.E(err) {
		return
	}
	if r, err = text.Comma(r); chk.E(err) {
		return
	}
	en.Filters = new(filter.S)
	if r, err = en.Filters.Unmarshal(r); chk.E(err) {
		return
	}
	if r, err = envelopes.SkipToTheEnd(r); chk.E(err) {
		return
	}
	return
}

// Parse reads a REQ envelope from minified JSON into a newly allocated
// reqenvelope.T.
func (en *T) Parse(b []byte) (t *T, rem []byte, err error) {
	t = New()
	if rem, err = t.Unmarshal(b); chk.E(err) {
		return
	}
	return
}
