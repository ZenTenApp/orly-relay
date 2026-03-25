package event

import (
	"fmt"
	"io"

	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/encoders/ints"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/text"
	"next.orly.dev/pkg/nostr/utils"
	"github.com/minio/sha256-simd"
	"github.com/templexxx/xhex"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
)

// E is the primary datatype of nostr. This is the form of the structure that
// defines its JSON string-based format.
//
// WARNING: DO NOT use json.Marshal with this type because it will not properly
// encode <, >, and & characters due to legacy bullcrap in the encoding/json
// library. Either call MarshalJSON directly or use a json.Encoder with html
// escaping disabled.
type E struct {

	// ID is the SHA256 hash of the canonical encoding of the event in binary
	// format
	ID []byte

	// Pubkey is the public key of the event creator in binary format
	Pubkey []byte

	// CreatedAt is the UNIX timestamp of the event according to the event
	// creator (never trust a timestamp!)
	CreatedAt int64

	// Kind is the nostr protocol code for the type of event. See kind.T
	Kind uint16

	// Tags are a list of tags, which are a list of strings usually structured
	// as a 3-layer scheme indicating specific features of an event.
	Tags *tag.S

	// Content is an arbitrary string that can contain anything, but usually
	// conforming to a specification relating to the Kind and the Tags.
	Content []byte

	// Sig is the signature on the ID hash that validates as coming from the
	// Pubkey in binary format.
	Sig []byte
}

var (
	jId        = []byte("id")
	jPubkey    = []byte("pubkey")
	jCreatedAt = []byte("created_at")
	jKind      = []byte("kind")
	jTags      = []byte("tags")
	jContent   = []byte("content")
	jSig       = []byte("sig")
)

// New returns a new event.E.
func New() *E {
	return &E{}
}

// Free nils all of the fields to hint to the GC that the event.E can be freed.
func (ev *E) Free() {
	ev.ID = nil
	ev.Pubkey = nil
	ev.Tags = nil
	ev.Content = nil
	ev.Sig = nil
}

// Clone creates a deep copy of the event with independent memory allocations.
// The clone does not use bufpool, ensuring it has a separate lifetime from
// the original event. This prevents corruption when the original is freed
// while the clone is still in use (e.g., in asynchronous delivery).
func (ev *E) Clone() *E {
	clone := &E{
		CreatedAt: ev.CreatedAt,
		Kind:      ev.Kind,
	}

	// Deep copy all byte slices with independent memory
	if ev.ID != nil {
		clone.ID = make([]byte, len(ev.ID))
		copy(clone.ID, ev.ID)
	}
	if ev.Pubkey != nil {
		clone.Pubkey = make([]byte, len(ev.Pubkey))
		copy(clone.Pubkey, ev.Pubkey)
	}
	if ev.Content != nil {
		clone.Content = make([]byte, len(ev.Content))
		copy(clone.Content, ev.Content)
	}
	if ev.Sig != nil {
		clone.Sig = make([]byte, len(ev.Sig))
		copy(clone.Sig, ev.Sig)
	}

	// Deep copy tags
	if ev.Tags != nil {
		clone.Tags = tag.NewS()
		for _, tg := range *ev.Tags {
			if tg != nil {
				// Create new tag with deep-copied elements
				newTag := tag.NewWithCap(len(tg.T))
				for _, element := range tg.T {
					newElement := make([]byte, len(element))
					copy(newElement, element)
					newTag.T = append(newTag.T, newElement)
				}
				clone.Tags.Append(newTag)
			}
		}
	}

	return clone
}

// EstimateSize returns a size for the event that allows for worst case scenario
// expansion of the escaped content and tags.
func (ev *E) EstimateSize() (size int) {
	size = len(ev.ID)*2 + len(ev.Pubkey)*2 + len(ev.Sig)*2 + len(ev.Content)*2
	if ev.Tags == nil {
		return
	}
	for _, v := range *ev.Tags {
		for _, w := range (*v).T {
			size += len(w) * 2
		}
	}
	return
}

func (ev *E) Marshal(dst []byte) (b []byte) {
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	if b == nil {
		estimatedSize := ev.EstimateSize()
		// Add overhead for JSON structure (keys, quotes, commas, etc.)
		estimatedSize += 100
		b = make([]byte, 0, estimatedSize)
	}
	b = append(b, '{')
	b = append(b, '"')
	b = append(b, jId...)
	b = append(b, `":"`...)
	// Pre-allocate hex encoding space
	hexStart := len(b)
	b = append(b, make([]byte, 2*sha256.Size)...)
	xhex.Encode(b[hexStart:], ev.ID)
	b = append(b, `","`...)
	b = append(b, jPubkey...)
	b = append(b, `":"`...)
	hexStart = len(b)
	b = append(b, make([]byte, 2*schnorr.PubKeyBytesLen)...)
	xhex.Encode(b[hexStart:], ev.Pubkey)
	b = append(b, `","`...)
	b = append(b, jCreatedAt...)
	b = append(b, `":`...)
	b = ints.New(ev.CreatedAt).Marshal(b)
	b = append(b, `,"`...)
	b = append(b, jKind...)
	b = append(b, `":`...)
	b = ints.New(ev.Kind).Marshal(b)
	b = append(b, `,"`...)
	b = append(b, jTags...)
	b = append(b, `":`...)
	if ev.Tags != nil {
		b = ev.Tags.Marshal(b)
	} else {
		// Emit empty array for nil tags to keep JSON valid
		b = append(b, '[', ']')
	}
	b = append(b, `,"`...)
	b = append(b, jContent...)
	b = append(b, `":"`...)
	b = text.NostrEscape(b, ev.Content)
	b = append(b, `","`...)
	b = append(b, jSig...)
	b = append(b, `":"`...)
	hexStart = len(b)
	b = append(b, make([]byte, 2*schnorr.SignatureSize)...)
	xhex.Encode(b[hexStart:], ev.Sig)
	b = append(b, `"}`...)
	return
}

// MarshalJSON marshals an event.E into a JSON byte string.
//
// WARNING: if json.Marshal is called in the hopes of invoking this function on
// an event, if it has <, > or * in the content or tags they are escaped into
// unicode escapes and break the event ID. Call this function directly in order
// to bypass this issue.
func (ev *E) MarshalJSON() (b []byte, err error) {
	b = ev.Marshal(nil)
	return
}

func (ev *E) Serialize() (b []byte) {
	b = ev.Marshal(nil)
	return
}

// Unmarshal unmarshalls a JSON string into an event.E.
func (ev *E) Unmarshal(b []byte) (rem []byte, err error) {
	key := make([]byte, 0, 9)
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		if b[0] == '{' {
			b = b[1:]
			goto BetweenKeys
		}
	}
	goto eof
BetweenKeys:
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		if b[0] == '"' {
			b = b[1:]
			goto InKey
		}
	}
	goto eof
InKey:
	for ; len(b) > 0; b = b[1:] {
		if b[0] == '"' {
			b = b[1:]
			goto InKV
		}
		key = append(key, b[0])
	}
	goto eof
InKV:
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		if b[0] == ':' {
			b = b[1:]
			goto InVal
		}
	}
	goto eof
InVal:
	// Skip whitespace before value
	for len(b) > 0 && isWhitespace(b[0]) {
		b = b[1:]
	}
	switch key[0] {
	case jId[0]:
		if !utils.FastEqual(jId, key) {
			goto invalid
		}
		var id []byte
		if id, b, err = text.UnmarshalHex(b); chk.E(err) {
			return
		}
		if len(id) != sha256.Size {
			err = errorf.E(
				"invalid Subscription, require %d got %d", sha256.Size,
				len(id),
			)
			return
		}
		ev.ID = id
		goto BetweenKV
	case jPubkey[0]:
		if !utils.FastEqual(jPubkey, key) {
			goto invalid
		}
		var pk []byte
		if pk, b, err = text.UnmarshalHex(b); chk.E(err) {
			return
		}
		if len(pk) != schnorr.PubKeyBytesLen {
			err = errorf.E(
				"invalid pubkey, require %d got %d",
				schnorr.PubKeyBytesLen, len(pk),
			)
			return
		}
		ev.Pubkey = pk
		goto BetweenKV
	case jKind[0]:
		if !utils.FastEqual(jKind, key) {
			goto invalid
		}
		k := kind.New(0)
		if b, err = k.Unmarshal(b); chk.E(err) {
			return
		}
		ev.Kind = k.ToU16()
		goto BetweenKV
	case jTags[0]:
		if !utils.FastEqual(jTags, key) {
			goto invalid
		}
		ev.Tags = new(tag.S)
		if b, err = ev.Tags.Unmarshal(b); chk.E(err) {
			return
		}
		goto BetweenKV
	case jSig[0]:
		if !utils.FastEqual(jSig, key) {
			goto invalid
		}
		var sig []byte
		if sig, b, err = text.UnmarshalHex(b); chk.E(err) {
			return
		}
		// NIP-59 rumor events have empty sig — accept as unsigned.
		if len(sig) != 0 && len(sig) != schnorr.SignatureSize {
			err = errorf.E(
				"invalid sig length, require %d got %d '%s'\n%s",
				schnorr.SignatureSize, len(sig), b, b,
			)
			return
		}
		ev.Sig = sig
		goto BetweenKV
	case jContent[0]:
		if key[1] == jContent[1] {
			if !utils.FastEqual(jContent, key) {
				goto invalid
			}
			if ev.Content, b, err = text.UnmarshalQuoted(b); chk.T(err) {
				return
			}
			goto BetweenKV
		} else if key[1] == jCreatedAt[1] {
			if !utils.FastEqual(jCreatedAt, key) {
				goto invalid
			}
			i := ints.New(0)
			if b, err = i.Unmarshal(b); chk.T(err) {
				return
			}
			ev.CreatedAt = i.Int64()
			goto BetweenKV
		} else {
			goto invalid
		}
	default:
		goto invalid
	}
BetweenKV:
	key = key[:0]
	for ; len(b) > 0; b = b[1:] {
		// Skip whitespace
		if isWhitespace(b[0]) {
			continue
		}
		switch {
		case len(b) == 0:
			return
		case b[0] == '}':
			b = b[1:]
			goto AfterClose
		case b[0] == ',':
			b = b[1:]
			goto BetweenKeys
		case b[0] == '"':
			b = b[1:]
			goto InKey
		}
	}
	// If we reach here, the buffer ended unexpectedly. Treat as end-of-object
	goto AfterClose
AfterClose:
	rem = b
	return
invalid:
	err = fmt.Errorf(
		"invalid key,\n'%s'\n'%s'\n'%s'", string(b), string(b[:]),
		string(b),
	)
	return
eof:
	err = io.EOF
	return
}

// UnmarshalJSON unmarshalls a JSON string into an event.E.
//
// Call ev.Free() to return the provided buffer to the bufpool afterwards.
func (ev *E) UnmarshalJSON(b []byte) (err error) {
	// log.I.F("UnmarshalJSON: '%s'", b)
	_, err = ev.Unmarshal(b)
	return
}

// isWhitespace returns true if the byte is a whitespace character (space, tab, newline, carriage return).
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// S is an array of event.E that sorts in reverse chronological order.
type S []*E

// Len returns the length of the event.Es.
func (ev S) Len() int { return len(ev) }

// Less returns whether the first is newer than the second (larger unix
// timestamp).
func (ev S) Less(i, j int) bool { return ev[i].CreatedAt > ev[j].CreatedAt }

// Swap two indexes of the event.Es with each other.
func (ev S) Swap(i, j int) { ev[i], ev[j] = ev[j], ev[i] }

// C is a channel that carries event.E.
type C chan *E
