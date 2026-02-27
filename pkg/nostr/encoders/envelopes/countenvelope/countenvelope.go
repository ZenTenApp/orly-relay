// Package countenvelope is an encoder for the COUNT request (client) and
// response (relay) message types.
package countenvelope

import (
	"bytes"
	"io"

	"git.mleku.dev/mleku/nostr/encoders/envelopes"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/ints"
	"git.mleku.dev/mleku/nostr/encoders/text"
	"git.mleku.dev/mleku/nostr/interfaces/codec"
	"git.mleku.dev/mleku/nostr/utils/constraints"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
)

// L is the label associated with this type of codec.Envelope.
const L = "COUNT"

// Request is a COUNT envelope sent by a client to request a count of results.
// This is a stupid idea because it costs as much processing as fetching the
// events, but doesn't provide the means to actually get them (the HTTP API
// /filter does this by returning the actual event Ids).
type Request struct {
	Subscription []byte
	Filters      filter.S
}

var _ codec.Envelope = (*Request)(nil)

// New creates a new Request with a standard style subscription.Id and empty filter.
func New() *Request { return new(Request) }

// NewRequest creates a new Request with a provided subscription.Id and
// filter.T.
func NewRequest(id []byte, filters filter.S) *Request {
	return &Request{
		Subscription: id,
		Filters:      filters,
	}
}

// Label returns the label of a CLOSED envelope.
func (en *Request) Label() string { return L }

// Write the Request to a provided io.Writer.
func (en *Request) Write(w io.Writer) (err error) {
	var b []byte
	b = en.Marshal(b)
	_, err = w.Write(b)
	return
}

// Marshal a Request appended to the provided destination slice as minified
// JSON.
func (en *Request) Marshal(dst []byte) (b []byte) {
	var err error
	b = dst
	b = envelopes.Marshal(
		b, L,
		func(bst []byte) (o []byte) {
			o = bst
			o = append(o, '"')
			o = append(o, en.Subscription...)
			o = append(o, '"')
			o = append(o, ',')
			for _, f := range en.Filters {
				o = append(o, ',')
				o = f.Marshal(o)
			}
			return
		},
	)
	_ = err
	return
}

// Unmarshal a Request from minified JSON, returning the remainder after the end
// of the envelope.
func (en *Request) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	if en.Subscription, r, err = text.UnmarshalQuoted(r); chk.E(err) {
		return
	}
	if r, err = en.Filters.Unmarshal(r); chk.E(err) {
		return
	}
	if r, err = envelopes.SkipToTheEnd(r); chk.E(err) {
		return
	}
	return
}

// ParseRequest reads a Request in minified JSON into a newly allocated Request.
func ParseRequest(b []byte) (t *Request, rem []byte, err error) {
	t = New()
	if rem, err = t.Unmarshal(b); chk.E(err) {
		return
	}
	return
}

// Response is a COUNT Response returning a count and approximate flag
// associated with the REQ subscription.Id.
type Response struct {
	Subscription []byte
	Count        int
	Approximate  bool
}

var _ codec.Envelope = (*Response)(nil)

// NewResponse creates a new empty countenvelope.Response with a standard formatted
// subscription.Id.
func NewResponse() *Response { return new(Response) }

// NewResponseFrom creates a new countenvelope.Response with provided string for the
// subscription.Id, a count and optional variadic approximate flag, which is
// otherwise false and does not get rendered into the JSON.
func NewResponseFrom[V constraints.Bytes](
	s V, cnt int,
	approx ...bool,
) (res *Response, err error) {
	var a bool
	if len(approx) > 0 {
		a = approx[0]
	}
	if len(s) < 0 || len(s) > 64 {
		err = errorf.E("subscription id must be length > 0 and <= 64")
		return
	}
	return &Response{[]byte(s), cnt, a}, nil
}

// Label returns the COUNT label associated with a Response.
func (en *Response) Label() string { return L }

// Write a Response to a provided io.Writer as minified JSON.
func (en *Response) Write(w io.Writer) (err error) {
	_, err = w.Write(en.Marshal(nil))
	return
}

// Marshal a countenvelope.Response envelope in minified JSON, appending to a
// provided destination slice.
func (en *Response) Marshal(dst []byte) (b []byte) {
	b = dst
	b = envelopes.Marshal(
		b, L,
		func(bst []byte) (o []byte) {
			o = bst
			o = append(o, '"')
			o = append(o, en.Subscription...)
			o = append(o, '"')
			o = append(o, ',')
			c := ints.New(en.Count)
			o = c.Marshal(o)
			if en.Approximate {
				o = append(o, ',')
				o = append(o, "true"...)
			}
			return
		},
	)
	return
}

// Unmarshal a COUNT Response from minified JSON, returning the remainder after
// the end of the envelope.
func (en *Response) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	if en.Subscription, r, err = text.UnmarshalQuoted(r); chk.E(err) {
		return
	}
	if r, err = text.Comma(r); chk.E(err) {
		return
	}
	i := ints.New(0)
	if r, err = i.Unmarshal(r); chk.E(err) {
		return
	}
	en.Count = int(i.N)
	if len(r) > 0 {
		if r[0] == ',' {
			r = r[1:]
			if bytes.HasPrefix(r, []byte("true")) {
				en.Approximate = true
			}
		}
	}
	if r, err = envelopes.SkipToTheEnd(r); chk.E(err) {
		return
	}
	return
}

// Parse reads a Count Response in minified JSON into a newly allocated
// countenvelope.Response.
func Parse(b []byte) (t *Response, rem []byte, err error) {
	t = NewResponse()
	if rem, err = t.Unmarshal(b); chk.E(err) {
		return
	}
	return
}
