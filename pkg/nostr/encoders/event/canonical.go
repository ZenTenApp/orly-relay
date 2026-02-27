package event

import (
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/ints"
	"git.mleku.dev/mleku/nostr/encoders/text"
	"github.com/minio/sha256-simd"
)

// ToCanonical converts the event to the canonical encoding used to derive the
// event ID.
func (ev *E) ToCanonical(dst []byte) (b []byte) {
	b = dst
	// Pre-allocate buffer if nil to reduce reallocations
	if b == nil {
		// Estimate size: [0," + hex(pubkey) + "," + timestamp + "," + kind + "," + tags + "," + content + ]
		estimatedSize := 5 + 2*len(ev.Pubkey) + 20 + 10 + 100
		if ev.Tags != nil {
			for _, tag := range *ev.Tags {
				for _, elem := range tag.T {
					estimatedSize += len(elem)*2 + 10 // escaped element + overhead
				}
			}
		}
		estimatedSize += len(ev.Content)*2 + 10 // escaped content + overhead
		b = make([]byte, 0, estimatedSize)
	}
	b = append(b, "[0,\""...)
	b = hex.EncAppend(b, ev.Pubkey)
	b = append(b, "\","...)
	b = ints.New(ev.CreatedAt).Marshal(b)
	b = append(b, ',')
	b = ints.New(ev.Kind).Marshal(b)
	b = append(b, ',')
	if ev.Tags != nil {
		b = ev.Tags.Marshal(b)
	} else {
		b = append(b, '[')
		b = append(b, ']')
	}
	b = append(b, ',')
	b = text.AppendQuote(b, ev.Content, text.NostrEscape)
	b = append(b, ']')
	return
}

// GetIDBytes returns the raw SHA256 hash of the canonical form of an event.E.
func (ev *E) GetIDBytes() []byte { return Hash(ev.ToCanonical(nil)) }

// Hash is a little helper generate a hash and return a slice instead of an
// array.
func Hash(in []byte) (out []byte) {
	h := sha256.Sum256(in)
	return h[:]
}
