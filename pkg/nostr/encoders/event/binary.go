package event

import (
	"bytes"
	"io"

	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/varint"
	"next.orly.dev/pkg/lol/chk"
)

// MarshalBinary writes a binary encoding of an event.
//
// [ 32 bytes ID ]
// [ 32 bytes Pubkey ]
// [ varint CreatedAt ]
// [ 2 bytes Kind ]
// [ varint Tags length ]
//
//	[ varint tag length ]
//	  [ varint tag element length ]
//	  [ tag element data ]
//	...
//
// [ varint Content length ]
// [ 64 bytes Sig ]
func (ev *E) MarshalBinary(w io.Writer) {
	_, _ = w.Write(ev.ID)
	_, _ = w.Write(ev.Pubkey)
	varint.Encode(w, uint64(ev.CreatedAt))
	varint.Encode(w, uint64(ev.Kind))
	if ev.Tags == nil {
		varint.Encode(w, 0)
	} else {
		varint.Encode(w, uint64(ev.Tags.Len()))
		for _, x := range *ev.Tags {
			varint.Encode(w, uint64(x.Len()))
			for _, y := range x.T {
				varint.Encode(w, uint64(len(y)))
				_, _ = w.Write(y)
			}
		}
	}
	varint.Encode(w, uint64(len(ev.Content)))
	_, _ = w.Write(ev.Content)
	_, _ = w.Write(ev.Sig)
}

// MarshalBinaryToBytes writes the binary encoding to a byte slice, reusing dst if provided.
// This is more efficient than MarshalBinary when you need the result as []byte.
func (ev *E) MarshalBinaryToBytes(dst []byte) []byte {
	var buf *bytes.Buffer
	if dst == nil {
		// Estimate size: fixed fields + varints + tags + content
		estimatedSize := 32 + 32 + 10 + 10 + 64 // ID + Pubkey + varints + Sig
		if ev.Tags != nil {
			for _, tag := range *ev.Tags {
				estimatedSize += 10 // varint for tag length
				for _, elem := range tag.T {
					estimatedSize += 10 + len(elem) // varint + data
				}
			}
		}
		estimatedSize += 10 + len(ev.Content) // content varint + content
		buf = bytes.NewBuffer(make([]byte, 0, estimatedSize))
	} else {
		buf = bytes.NewBuffer(dst[:0])
	}
	ev.MarshalBinary(buf)
	return buf.Bytes()
}

func (ev *E) UnmarshalBinary(r io.Reader) (err error) {
	ev.ID = make([]byte, 32)
	if _, err = r.Read(ev.ID); chk.E(err) {
		return
	}
	ev.Pubkey = make([]byte, 32)
	if _, err = r.Read(ev.Pubkey); chk.E(err) {
		return
	}
	var ca uint64
	if ca, err = varint.Decode(r); chk.E(err) {
		return
	}
	ev.CreatedAt = int64(ca)
	var k uint64
	if k, err = varint.Decode(r); chk.E(err) {
		return
	}
	ev.Kind = uint16(k)
	var nTags uint64
	if nTags, err = varint.Decode(r); chk.E(err) {
		return
	}
	if nTags == 0 {
		ev.Tags = nil
	} else {
		ev.Tags = tag.NewSWithCap(int(nTags))
		for range nTags {
			var nField uint64
			if nField, err = varint.Decode(r); chk.E(err) {
				return
			}
			t := tag.NewWithCap(int(nField))
			for range nField {
				var lenField uint64
				if lenField, err = varint.Decode(r); chk.E(err) {
					return
				}
				field := make([]byte, lenField)
				if _, err = r.Read(field); chk.E(err) {
					return
				}
				t.T = append(t.T, field)
			}
			*ev.Tags = append(*ev.Tags, t)
		}
	}
	var cLen uint64
	if cLen, err = varint.Decode(r); chk.E(err) {
		return
	}
	ev.Content = make([]byte, cLen)
	if _, err = r.Read(ev.Content); chk.E(err) {
		return
	}
	ev.Sig = make([]byte, schnorr.SignatureSize)
	if _, err = r.Read(ev.Sig); chk.E(err) {
		return
	}
	return
}
