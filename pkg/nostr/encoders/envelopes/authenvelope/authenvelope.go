// Package authenvelope defines the auth challenge (relay message) and response
// (client message) of the NIP-42 authentication protocol.
package authenvelope

import (
	"io"

	"git.smesh.lol/orly/pkg/nostr/encoders/envelopes"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/text"
	"git.smesh.lol/orly/pkg/nostr/interfaces/codec"
	"git.smesh.lol/orly/pkg/nostr/utils/constraints"
	"git.smesh.lol/orly/pkg/nostr/utils/units"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
)

// L is the label associated with this type of codec.Envelope.
const L = "AUTH"

// Challenge is the relay-sent message containing a relay-chosen random string
// to prevent replay attacks on NIP-42 authentication.
type Challenge struct {
	Challenge []byte
}

var _ codec.Envelope = (*Challenge)(nil)

// NewChallenge creates a new empty authenvelope.Challenge.
func NewChallenge() *Challenge { return &Challenge{} }

// NewChallengeWith creates a new authenvelope.Challenge with provided bytes.
func NewChallengeWith[V constraints.Bytes](challenge V) *Challenge {
	return &Challenge{[]byte(challenge)}
}

// Label returns the label of a authenvelope.Challenge.
func (en *Challenge) Label() string { return L }

// Write encodes and writes the Challenge instance to the provided writer.
//
// # Parameters
//
// - w (io.Writer): The destination where the encoded data will be written.
//
// # Return Values
//
// - err (error): An error if writing to the writer fails.
//
// # Expected behaviour
//
// Encodes the Challenge instance into a byte slice using Marshal, logs the
// encoded challenge, and writes it to the provided io.Writer.
func (en *Challenge) Write(w io.Writer) (err error) {
	var b []byte
	b = en.Marshal(b)
	// log.D.F("writing out challenge envelope: '%s'", b)
	_, err = w.Write(b)
	return
}

// Marshal encodes the Challenge instance into a byte slice, formatting it as
// a JSON-like structure with a specific label and escaping rules applied to
// its content.
//
// # Parameters
//
// - dst ([]byte): The destination buffer where the encoded data will be written.
//
// # Return Values
//
// - b ([]byte): The byte slice containing the encoded Challenge data.
//
// # Expected behaviour
//
// - Prepares the destination buffer and applies a label to it.
//
// - Escapes the challenge content according to Nostr-specific rules before
// appending it to the output.
//
// - Returns the resulting byte slice with the complete encoded structure.
func (en *Challenge) Marshal(dst []byte) (b []byte) {
	b = dst
	var err error
	b = envelopes.Marshal(
		b, L,
		func(bst []byte) (o []byte) {
			o = bst
			o = append(o, '"')
			o = text.NostrEscape(o, en.Challenge)
			o = append(o, '"')
			return
		},
	)
	_ = err
	return
}

// Unmarshal parses the provided byte slice and extracts the challenge value,
// leaving any remaining bytes after parsing.
//
// # Parameters
//
// - b ([]byte): The byte slice containing the encoded challenge data.
//
// # Return Values
//
// - r ([]byte): Any remaining bytes after parsing the challenge.
//
// - err (error): An error if parsing fails.
//
// # Expected behaviour
//
// - Extracts the quoted challenge string from the input byte slice.
//
// - Trims any trailing characters following the closing quote.
func (en *Challenge) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	if en.Challenge, r, err = text.UnmarshalQuoted(r); chk.E(err) {
		return
	}
	for ; len(r) >= 0; r = r[1:] {
		if r[0] == ']' {
			r = r[:0]
			return
		}
	}
	return
}

// ParseChallenge parses the provided byte slice into a new Challenge instance,
// extracting the challenge value and returning any remaining bytes after parsing.
//
// # Parameters
//
// - b ([]byte): The byte slice containing the encoded challenge data.
//
// # Return Values
//
//   - t (*Challenge): A pointer to the newly created and populated Challenge
//     instance.
//
// - rem ([]byte): Any remaining bytes in the input slice after parsing.
//
// - err (error): An error if parsing fails.
//
// # Expected behaviour
//
// Parses the byte slice into a new Challenge instance using Unmarshal,
// returning any remaining bytes and an error if parsing fails.
func ParseChallenge(b []byte) (t *Challenge, rem []byte, err error) {
	t = NewChallenge()
	if rem, err = t.Unmarshal(b); chk.E(err) {
		return
	}
	return
}

// Response is a client-side envelope containing the signed event bearing the
// relay's URL and Challenge string.
type Response struct {
	Event *event.E
}

var _ codec.Envelope = (*Response)(nil)

// NewResponse creates a new empty Response.
func NewResponse() *Response { return &Response{} }

// NewResponseWith creates a new Response with a provided event.E.
func NewResponseWith(event *event.E) *Response { return &Response{Event: event} }

// Label returns the label of a auth Response envelope.
func (en *Response) Label() string { return L }

func (en *Response) Id() []byte { return en.Event.ID }

// Write the Response to a provided io.Writer.
func (en *Response) Write(w io.Writer) (err error) {
	var b []byte
	b = en.Marshal(b)
	_, err = w.Write(b)
	return
}

// Marshal a Response to minified JSON, appending to a provided destination
// slice. Note that this ensures correct string escaping on the challenge field.
func (en *Response) Marshal(dst []byte) (b []byte) {
	var err error
	if en == nil {
		err = errorf.E("nil response")
		return
	}
	if en.Event == nil {
		err = errorf.E("nil event in response")
		return
	}
	// if the destination capacity is not large enough, allocate a new
	// destination slice.
	if en.Event.EstimateSize() >= cap(dst) {
		dst = make([]byte, 0, en.Event.EstimateSize()+units.Kb)
	}
	b = dst
	b = envelopes.Marshal(b, L, en.Event.Marshal)
	_ = err
	return
}

// Unmarshal a Response from minified JSON, returning the remainder after the en
// of the envelope. Note that this ensures the challenge string was correctly
// escaped by NIP-01 escaping rules.
func (en *Response) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	// literally just unmarshal the event
	en.Event = event.New()
	if r, err = en.Event.Unmarshal(r); chk.E(err) {
		return
	}
	if r, err = envelopes.SkipToTheEnd(r); chk.E(err) {
		return
	}
	return
}

// ParseResponse reads a Response encoded in minified JSON and unpacks it to
// the runtime format.
func ParseResponse(b []byte) (t *Response, rem []byte, err error) {
	t = NewResponse()
	if rem, err = t.Unmarshal(b); chk.E(err) {
		return
	}
	return
}
