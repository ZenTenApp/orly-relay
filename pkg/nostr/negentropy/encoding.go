package negentropy

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
)

// Errors
var (
	ErrInvalidVarInt  = errors.New("invalid varint encoding")
	ErrUnexpectedEOF  = errors.New("unexpected end of data")
	ErrInvalidBound   = errors.New("invalid bound encoding")
	ErrInvalidMessage = errors.New("invalid negentropy message")
)

// EncodeVarInt encodes an unsigned integer as a big-endian variable-length integer.
// Matches the C++ negentropy encoding: extract 7-bit groups from LSB,
// reverse to big-endian order, set continuation bits on all but last byte.
func EncodeVarInt(n uint64) []byte {
	if n == 0 {
		return []byte{0}
	}

	// Extract 7-bit groups from LSB
	var buf [10]byte
	i := 0
	for n > 0 {
		buf[i] = byte(n & 0x7F)
		n >>= 7
		i++
	}

	// Reverse to big-endian order and set continuation bits
	result := make([]byte, i)
	for j := 0; j < i; j++ {
		result[j] = buf[i-1-j]
	}
	for j := 0; j < len(result)-1; j++ {
		result[j] |= 0x80
	}

	return result
}

// DecodeVarInt decodes a big-endian variable-length integer from a reader.
// Matches the C++ negentropy decoding: accumulate with res = (res << 7) | (byte & 0x7F).
func DecodeVarInt(r io.ByteReader) (uint64, error) {
	var n uint64

	for i := 0; i < 10; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, ErrUnexpectedEOF
		}

		n = (n << 7) | uint64(b&0x7F)
		if b&0x80 == 0 {
			return n, nil
		}
	}

	return 0, ErrInvalidVarInt
}

// WriteVarInt writes a varint to a byte slice, returning bytes written.
func WriteVarInt(buf []byte, n uint64) int {
	encoded := EncodeVarInt(n)
	copy(buf, encoded)
	return len(encoded)
}

// Encoder handles encoding negentropy protocol messages.
type Encoder struct {
	buf           []byte
	lastTimestamp int64
}

// NewEncoder creates a new encoder with the given initial capacity.
func NewEncoder(capacity int) *Encoder {
	return &Encoder{
		buf:           make([]byte, 0, capacity),
		lastTimestamp: 0,
	}
}

// Reset resets the encoder for reuse.
func (e *Encoder) Reset() {
	e.buf = e.buf[:0]
	e.lastTimestamp = 0
}

// Bytes returns the encoded bytes.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// WriteByte writes a single byte.
func (e *Encoder) WriteByte(b byte) {
	e.buf = append(e.buf, b)
}

// WriteBytes writes a byte slice.
func (e *Encoder) WriteBytes(b []byte) {
	e.buf = append(e.buf, b...)
}

// WriteVarInt writes a variable-length integer.
func (e *Encoder) WriteVarInt(n uint64) {
	e.buf = append(e.buf, EncodeVarInt(n)...)
}

// WriteTimestamp writes a timestamp as a delta from the last timestamp.
func (e *Encoder) WriteTimestamp(ts int64) {
	if ts == MaxTimestamp {
		// Special case: max timestamp is encoded as 0
		e.WriteVarInt(0)
	} else {
		delta := ts - e.lastTimestamp
		if delta < 0 {
			delta = 0
		}
		e.WriteVarInt(uint64(delta) + 1)
		e.lastTimestamp = ts
	}
}

// WriteBound writes a bound (timestamp + optional ID).
func (e *Encoder) WriteBound(b Bound) {
	e.WriteTimestamp(b.Timestamp)

	if b.Timestamp != MaxTimestamp && b.ID != "" {
		// Write ID length and hex-decoded ID
		idBytes, err := hex.DecodeString(b.ID)
		if err != nil {
			e.WriteVarInt(0)
			return
		}
		// Bound ID prefix can be 0-32 bytes per spec
		if len(idBytes) > FullIDSize {
			idBytes = idBytes[:FullIDSize]
		}
		e.WriteVarInt(uint64(len(idBytes)))
		e.WriteBytes(idBytes)
	} else {
		e.WriteVarInt(0)
	}
}

// WriteFingerprint writes a fingerprint.
func (e *Encoder) WriteFingerprint(fp Fingerprint) {
	e.WriteBytes(fp[:])
}

// WriteMode writes a mode byte.
func (e *Encoder) WriteMode(m Mode) {
	e.WriteByte(byte(m))
}

// Decoder handles decoding negentropy protocol messages.
type Decoder struct {
	data          []byte
	pos           int
	lastTimestamp int64
}

// NewDecoder creates a new decoder for the given data.
func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		data:          data,
		pos:           0,
		lastTimestamp: 0,
	}
}

// ReadByte reads a single byte.
func (d *Decoder) ReadByte() (byte, error) {
	if d.pos >= len(d.data) {
		return 0, ErrUnexpectedEOF
	}
	b := d.data[d.pos]
	d.pos++
	return b, nil
}

// ReadBytes reads n bytes.
func (d *Decoder) ReadBytes(n int) ([]byte, error) {
	if d.pos+n > len(d.data) {
		return nil, ErrUnexpectedEOF
	}
	b := d.data[d.pos : d.pos+n]
	d.pos += n
	return b, nil
}

// ReadVarInt reads a variable-length integer.
func (d *Decoder) ReadVarInt() (uint64, error) {
	return DecodeVarInt(d)
}

// ReadTimestamp reads a timestamp (delta encoded).
func (d *Decoder) ReadTimestamp() (int64, error) {
	delta, err := d.ReadVarInt()
	if err != nil {
		return 0, err
	}

	if delta == 0 {
		return MaxTimestamp, nil
	}

	ts := d.lastTimestamp + int64(delta-1)
	d.lastTimestamp = ts
	return ts, nil
}

// ReadBound reads a bound (timestamp + optional ID).
func (d *Decoder) ReadBound() (Bound, error) {
	ts, err := d.ReadTimestamp()
	if err != nil {
		return Bound{}, err
	}

	idLen, err := d.ReadVarInt()
	if err != nil {
		return Bound{}, err
	}

	var id string
	if idLen > 0 {
		idBytes, err := d.ReadBytes(int(idLen))
		if err != nil {
			return Bound{}, err
		}
		id = hex.EncodeToString(idBytes)
	}

	return Bound{Item{Timestamp: ts, ID: id}}, nil
}

// ReadFingerprint reads a fingerprint.
func (d *Decoder) ReadFingerprint() (Fingerprint, error) {
	data, err := d.ReadBytes(DefaultIDSize)
	if err != nil {
		return Fingerprint{}, err
	}
	var fp Fingerprint
	copy(fp[:], data)
	return fp, nil
}

// ReadMode reads a mode byte.
func (d *Decoder) ReadMode() (Mode, error) {
	b, err := d.ReadByte()
	if err != nil {
		return 0, err
	}
	return Mode(b), nil
}

// Remaining returns the number of unread bytes.
func (d *Decoder) Remaining() int {
	return len(d.data) - d.pos
}

// HasMore returns true if there are more bytes to read.
func (d *Decoder) HasMore() bool {
	return d.pos < len(d.data)
}

// encodeHex encodes bytes to hex string.
func encodeHex(b []byte) string {
	return hex.EncodeToString(b)
}

// decodeHex decodes hex string to bytes.
func decodeHex(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// EncodeMessage encodes raw bytes to hex for NIP-77 transport.
func EncodeMessage(msg []byte) string {
	return hex.EncodeToString(msg)
}

// DecodeMessage decodes hex-encoded NIP-77 message to raw bytes.
func DecodeMessage(hexMsg string) ([]byte, error) {
	return hex.DecodeString(hexMsg)
}

// VarIntSize returns the number of bytes needed to encode n as a varint.
func VarIntSize(n uint64) int {
	if n == 0 {
		return 1
	}
	size := 0
	for n > 0 {
		size++
		n >>= 7
	}
	return size
}

// BoundSize returns the approximate size of a bound when encoded.
func BoundSize(b Bound) int {
	size := VarIntSize(uint64(b.Timestamp)) + 1 // timestamp + id length byte
	if b.ID != "" {
		size += min(len(b.ID)/2, DefaultIDSize)
	}
	return size
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Read implements io.ByteReader for binary.Read compatibility.
func (d *Decoder) Read(p []byte) (int, error) {
	if d.pos >= len(d.data) {
		return 0, io.EOF
	}
	n := copy(p, d.data[d.pos:])
	d.pos += n
	return n, nil
}

// ReadUint64 reads a little-endian uint64.
func (d *Decoder) ReadUint64() (uint64, error) {
	data, err := d.ReadBytes(8)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(data), nil
}
