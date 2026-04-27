package types

import (
	"bytes"
	"io"

	"git.smesh.lol/orly/pkg/lol/chk"
)

var zero = []byte{0x00}

type Word struct {
	val []byte // Contains only the raw word (without the zero-byte marker)
}

// FromWord stores the word without any modifications
func (w *Word) FromWord(word []byte) {
	w.val = word // Only store the raw word
}

// Bytes returns the raw word without any end-of-word marker
func (w *Word) Bytes() []byte {
	return w.val
}

// MarshalWrite writes the word to the writer, appending the zero-byte marker
func (w *Word) MarshalWrite(wr io.Writer) (err error) {
	if _, err = wr.Write(w.val); chk.E(err) {
		return
	}
	if _, err = wr.Write(zero); chk.E(err) {
		return
	}
	return err
}

// UnmarshalRead reads the word from the reader, stopping at the zero-byte marker
func (w *Word) UnmarshalRead(r io.Reader) error {
	buf := new(bytes.Buffer)
	var tmp [1]byte // Fixed array avoids heap escape
	foundEndMarker := false

	// Read bytes until the zero byte is encountered
	for {
		n, err := r.Read(tmp[:])
		if n > 0 {
			if tmp[0] == 0x00 { // Stop on encountering the zero-byte marker
				foundEndMarker = true
				break
			}
			buf.WriteByte(tmp[0])
		}
		if err != nil {
			if chk.E(err) {
				return err // Handle unexpected errors
			}
			break
		}
	}

	// Only store the word if we found a valid end marker
	if foundEndMarker {
		// Make a copy of the bytes to avoid them being zeroed when the buffer is returned to the pool
		bytes := buf.Bytes()
		w.val = make([]byte, len(bytes))
		copy(w.val, bytes)
	} else {
		w.val = []byte{} // Empty slice if no valid end marker was found
	}
	return nil
}
