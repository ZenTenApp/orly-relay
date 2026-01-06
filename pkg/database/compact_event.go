//go:build !(js && wasm)

package database

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"git.mleku.dev/mleku/nostr/crypto/ec/schnorr"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/varint"
	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/bufpool"
)

// CompactEventFormat defines the binary format for compact event storage.
// This format uses 5-byte serial references instead of 32-byte IDs/pubkeys,
// dramatically reducing storage requirements.
//
// Format:
//   - 1 byte:  Version (currently 1)
//   - 5 bytes: Author pubkey serial (reference to spk table)
//   - varint:  CreatedAt timestamp
//   - 2 bytes: Kind (uint16 big-endian)
//   - varint:  Number of tags
//   - For each tag:
//   - varint: Number of elements in tag
//   - For each element:
//   - 1 byte: Element type flag
//   - 0x00 = raw bytes (followed by varint length + data)
//   - 0x01 = pubkey serial reference (followed by 5-byte serial)
//   - 0x02 = event ID serial reference (followed by 5-byte serial)
//   - 0x03 = unknown event ID (followed by 32-byte full ID)
//   - Element data based on type
//   - varint:  Content length
//   - Content bytes
//   - 64 bytes: Signature
//
// Space savings example (event with 3 p-tags, 1 e-tag):
//   - Original: 32 (ID) + 32 (pubkey) + 32*4 (tags) = 192 bytes
//   - Compact:  5 (pubkey serial) + 5*4 (tag serials) = 25 bytes
//   - Savings: 167 bytes per event (87%)

const (
	CompactFormatVersion = 1

	// Tag element type flags
	TagElementRaw          = 0x00 // Raw bytes (varint length + data)
	TagElementPubkeySerial = 0x01 // Pubkey serial reference (5 bytes)
	TagElementEventSerial  = 0x02 // Event ID serial reference (5 bytes)
	TagElementEventIdFull  = 0x03 // Full event ID (32 bytes) - for unknown refs
)

// SerialResolver is an interface for resolving serials during compact encoding/decoding.
// This allows the encoder/decoder to look up or create serial mappings.
type SerialResolver interface {
	// GetOrCreatePubkeySerial returns the serial for a pubkey, creating one if needed.
	GetOrCreatePubkeySerial(pubkey []byte) (serial uint64, err error)

	// GetPubkeyBySerial returns the full pubkey for a serial.
	GetPubkeyBySerial(serial uint64) (pubkey []byte, err error)

	// GetEventSerialById returns the serial for an event ID, or 0 if not found.
	GetEventSerialById(eventId []byte) (serial uint64, found bool, err error)

	// GetEventIdBySerial returns the full event ID for a serial.
	GetEventIdBySerial(serial uint64) (eventId []byte, err error)
}

// MarshalCompactEvent encodes an event using compact serial references.
// The resolver is used to look up/create serial mappings for pubkeys and event IDs.
func MarshalCompactEvent(ev *event.E, resolver SerialResolver) (data []byte, err error) {
	buf := bufpool.GetMedium()
	defer bufpool.PutMedium(buf)

	// Version byte
	buf.WriteByte(CompactFormatVersion)

	// Author pubkey serial (5 bytes)
	var authorSerial uint64
	if authorSerial, err = resolver.GetOrCreatePubkeySerial(ev.Pubkey); chk.E(err) {
		return nil, err
	}
	writeUint40(buf, authorSerial)

	// CreatedAt (varint)
	varint.Encode(buf, uint64(ev.CreatedAt))

	// Kind (2 bytes big-endian)
	binary.Write(buf, binary.BigEndian, ev.Kind)

	// Tags
	if ev.Tags == nil || ev.Tags.Len() == 0 {
		varint.Encode(buf, 0)
	} else {
		varint.Encode(buf, uint64(ev.Tags.Len()))
		for _, t := range *ev.Tags {
			if err = encodeCompactTag(buf, t, resolver); chk.E(err) {
				return nil, err
			}
		}
	}

	// Content
	varint.Encode(buf, uint64(len(ev.Content)))
	buf.Write(ev.Content)

	// Signature (64 bytes)
	buf.Write(ev.Sig)

	// Copy bytes before returning buffer to pool
	return bufpool.CopyBytes(buf), nil
}

// encodeCompactTag encodes a single tag with serial references for e/p tags.
func encodeCompactTag(w io.Writer, t *tag.T, resolver SerialResolver) (err error) {
	if t == nil || t.Len() == 0 {
		varint.Encode(w, 0)
		return nil
	}

	varint.Encode(w, uint64(t.Len()))

	// Get tag key to determine if we should use serial references
	key := t.Key()
	isPTag := len(key) == 1 && key[0] == 'p'
	isETag := len(key) == 1 && key[0] == 'e'

	for i, elem := range t.T {
		if i == 0 {
			// First element is always the tag key - store as raw
			writeTagElement(w, TagElementRaw, elem)
			continue
		}

		if i == 1 {
			// Second element is the value - potentially a serial reference
			if isPTag && len(elem) == 32 {
				// Binary pubkey - look up serial
				serial, serErr := resolver.GetOrCreatePubkeySerial(elem)
				if serErr == nil {
					writeTagElementSerial(w, TagElementPubkeySerial, serial)
					continue
				}
				// Fall through to raw encoding on error
			} else if isPTag && len(elem) == 64 {
				// Hex pubkey - decode and look up serial
				var pubkey []byte
				if pubkey, err = hexDecode(elem); err == nil && len(pubkey) == 32 {
					serial, serErr := resolver.GetOrCreatePubkeySerial(pubkey)
					if serErr == nil {
						writeTagElementSerial(w, TagElementPubkeySerial, serial)
						continue
					}
				}
				// Fall through to raw encoding on error
			} else if isETag && len(elem) == 32 {
				// Binary event ID - look up serial if exists
				serial, found, serErr := resolver.GetEventSerialById(elem)
				if serErr == nil && found {
					writeTagElementSerial(w, TagElementEventSerial, serial)
					continue
				}
				// Event not found - store full ID
				writeTagElement(w, TagElementEventIdFull, elem)
				continue
			} else if isETag && len(elem) == 64 {
				// Hex event ID - decode and look up serial
				var eventId []byte
				if eventId, err = hexDecode(elem); err == nil && len(eventId) == 32 {
					serial, found, serErr := resolver.GetEventSerialById(eventId)
					if serErr == nil && found {
						writeTagElementSerial(w, TagElementEventSerial, serial)
						continue
					}
					// Event not found - store full ID
					writeTagElement(w, TagElementEventIdFull, eventId)
					continue
				}
				// Fall through to raw encoding on error
			}
		}

		// Default: raw encoding
		writeTagElement(w, TagElementRaw, elem)
	}

	return nil
}

// writeTagElement writes a tag element with type flag.
func writeTagElement(w io.Writer, typeFlag byte, data []byte) {
	w.Write([]byte{typeFlag})
	if typeFlag == TagElementEventIdFull {
		// Full event ID - no length prefix, always 32 bytes
		w.Write(data)
	} else {
		// Raw data - length prefix
		varint.Encode(w, uint64(len(data)))
		w.Write(data)
	}
}

// writeTagElementSerial writes a serial reference tag element.
func writeTagElementSerial(w io.Writer, typeFlag byte, serial uint64) {
	w.Write([]byte{typeFlag})
	writeUint40(w, serial)
}

// writeUint40 writes a 5-byte big-endian unsigned integer.
func writeUint40(w io.Writer, value uint64) {
	buf := []byte{
		byte((value >> 32) & 0xFF),
		byte((value >> 24) & 0xFF),
		byte((value >> 16) & 0xFF),
		byte((value >> 8) & 0xFF),
		byte(value & 0xFF),
	}
	w.Write(buf)
}

// readUint40 reads a 5-byte big-endian unsigned integer.
func readUint40(r io.Reader) (value uint64, err error) {
	var buf [5]byte // Fixed array avoids heap escape
	if _, err = io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	value = (uint64(buf[0]) << 32) |
		(uint64(buf[1]) << 24) |
		(uint64(buf[2]) << 16) |
		(uint64(buf[3]) << 8) |
		uint64(buf[4])
	return value, nil
}

// UnmarshalCompactEvent decodes a compact event back to a full event.E.
// The resolver is used to look up pubkeys and event IDs from serials.
// The eventId parameter is the full 32-byte event ID (from SerialEventId table).
func UnmarshalCompactEvent(data []byte, eventId []byte, resolver SerialResolver) (ev *event.E, err error) {
	// Validate eventId upfront to prevent returning events with zero IDs
	if len(eventId) != 32 {
		return nil, errors.New("invalid eventId: must be exactly 32 bytes")
	}

	r := bytes.NewReader(data)
	ev = new(event.E)

	// Version byte
	version, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != CompactFormatVersion {
		return nil, errors.New("unsupported compact event format version")
	}

	// Set the event ID (passed separately from SerialEventId lookup)
	ev.ID = make([]byte, 32)
	copy(ev.ID, eventId)

	// Author pubkey serial (5 bytes) -> full pubkey
	authorSerial, err := readUint40(r)
	if err != nil {
		return nil, err
	}
	if ev.Pubkey, err = resolver.GetPubkeyBySerial(authorSerial); chk.E(err) {
		return nil, err
	}

	// CreatedAt (varint)
	var ca uint64
	if ca, err = varint.Decode(r); chk.E(err) {
		return nil, err
	}
	ev.CreatedAt = int64(ca)

	// Kind (2 bytes big-endian)
	if err = binary.Read(r, binary.BigEndian, &ev.Kind); chk.E(err) {
		return nil, err
	}

	// Tags
	var nTags uint64
	if nTags, err = varint.Decode(r); chk.E(err) {
		return nil, err
	}
	if nTags > 0 {
		ev.Tags = tag.NewSWithCap(int(nTags))
		for i := uint64(0); i < nTags; i++ {
			var t *tag.T
			if t, err = decodeCompactTag(r, resolver); chk.E(err) {
				return nil, err
			}
			*ev.Tags = append(*ev.Tags, t)
		}
	}

	// Content
	var contentLen uint64
	if contentLen, err = varint.Decode(r); chk.E(err) {
		return nil, err
	}
	ev.Content = make([]byte, contentLen)
	if _, err = io.ReadFull(r, ev.Content); chk.E(err) {
		return nil, err
	}

	// Signature (64 bytes)
	ev.Sig = make([]byte, schnorr.SignatureSize)
	if _, err = io.ReadFull(r, ev.Sig); chk.E(err) {
		return nil, err
	}

	return ev, nil
}

// decodeCompactTag decodes a single tag from compact format.
func decodeCompactTag(r io.Reader, resolver SerialResolver) (t *tag.T, err error) {
	var nElems uint64
	if nElems, err = varint.Decode(r); chk.E(err) {
		return nil, err
	}

	t = tag.NewWithCap(int(nElems))

	for i := uint64(0); i < nElems; i++ {
		var elem []byte
		if elem, err = decodeTagElement(r, resolver); chk.E(err) {
			return nil, err
		}
		t.T = append(t.T, elem)
	}

	return t, nil
}

// decodeTagElement decodes a single tag element from compact format.
func decodeTagElement(r io.Reader, resolver SerialResolver) (elem []byte, err error) {
	// Read type flag (fixed array avoids heap escape)
	var typeBuf [1]byte
	if _, err = io.ReadFull(r, typeBuf[:]); err != nil {
		return nil, err
	}
	typeFlag := typeBuf[0]

	switch typeFlag {
	case TagElementRaw:
		// Raw bytes: varint length + data
		var length uint64
		if length, err = varint.Decode(r); chk.E(err) {
			return nil, err
		}
		elem = make([]byte, length)
		if _, err = io.ReadFull(r, elem); err != nil {
			return nil, err
		}
		return elem, nil

	case TagElementPubkeySerial:
		// Pubkey serial: 5 bytes -> lookup full pubkey -> return as 33-byte binary
		serial, err := readUint40(r)
		if err != nil {
			return nil, err
		}
		pubkey, err := resolver.GetPubkeyBySerial(serial)
		if err != nil {
			return nil, err
		}
		// Return as 33-byte binary (32 bytes + null terminator) for tag.Marshal detection
		result := make([]byte, 33)
		copy(result, pubkey)
		result[32] = 0 // null terminator
		return result, nil

	case TagElementEventSerial:
		// Event serial: 5 bytes -> lookup full event ID -> return as 33-byte binary
		serial, err := readUint40(r)
		if err != nil {
			return nil, err
		}
		eventId, err := resolver.GetEventIdBySerial(serial)
		if err != nil {
			return nil, err
		}
		// Return as 33-byte binary (32 bytes + null terminator) for tag.Marshal detection
		result := make([]byte, 33)
		copy(result, eventId)
		result[32] = 0 // null terminator
		return result, nil

	case TagElementEventIdFull:
		// Full event ID: 32 bytes (for unknown/forward references)
		// Return as 33-byte binary (32 bytes + null terminator) for tag.Marshal detection
		elem = make([]byte, 33)
		if _, err = io.ReadFull(r, elem[:32]); err != nil {
			return nil, err
		}
		elem[32] = 0 // null terminator
		return elem, nil

	default:
		return nil, errors.New("unknown tag element type flag")
	}
}

// hexDecode decodes hex bytes to binary.
// This is a simple implementation - the real one uses the optimized hex package.
func hexDecode(src []byte) (dst []byte, err error) {
	if len(src)%2 != 0 {
		return nil, errors.New("hex string has odd length")
	}
	dst = make([]byte, len(src)/2)
	for i := 0; i < len(dst); i++ {
		a := unhex(src[i*2])
		b := unhex(src[i*2+1])
		if a == 0xFF || b == 0xFF {
			return nil, errors.New("invalid hex character")
		}
		dst[i] = (a << 4) | b
	}
	return dst, nil
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0xFF
}
