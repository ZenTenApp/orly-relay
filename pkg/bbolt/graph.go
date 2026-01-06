//go:build !(js && wasm)

package bbolt

import (
	"bytes"
	"encoding/binary"
	"io"
)

// EventVertex stores the adjacency list for an event.
// Contains the author and all edges to other events/pubkeys.
type EventVertex struct {
	AuthorSerial uint64   // Serial of the author pubkey
	Kind         uint16   // Event kind
	PTagSerials  []uint64 // Serials of pubkeys mentioned (p-tags)
	ETagSerials  []uint64 // Serials of events referenced (e-tags)
}

// Encode serializes the EventVertex to bytes.
// Format: author(5) | kind(2) | ptag_count(varint) | [ptag_serials(5)...] | etag_count(varint) | [etag_serials(5)...]
func (ev *EventVertex) Encode() []byte {
	// Calculate size
	size := 5 + 2 + 2 + len(ev.PTagSerials)*5 + 2 + len(ev.ETagSerials)*5
	buf := make([]byte, 0, size)

	// Author serial (5 bytes)
	authorBuf := make([]byte, 5)
	encodeUint40(ev.AuthorSerial, authorBuf)
	buf = append(buf, authorBuf...)

	// Kind (2 bytes)
	kindBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(kindBuf, ev.Kind)
	buf = append(buf, kindBuf...)

	// P-tag count and serials
	ptagCountBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(ptagCountBuf, uint16(len(ev.PTagSerials)))
	buf = append(buf, ptagCountBuf...)
	for _, serial := range ev.PTagSerials {
		serialBuf := make([]byte, 5)
		encodeUint40(serial, serialBuf)
		buf = append(buf, serialBuf...)
	}

	// E-tag count and serials
	etagCountBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(etagCountBuf, uint16(len(ev.ETagSerials)))
	buf = append(buf, etagCountBuf...)
	for _, serial := range ev.ETagSerials {
		serialBuf := make([]byte, 5)
		encodeUint40(serial, serialBuf)
		buf = append(buf, serialBuf...)
	}

	return buf
}

// Decode deserializes bytes into an EventVertex.
func (ev *EventVertex) Decode(data []byte) error {
	if len(data) < 9 { // minimum: author(5) + kind(2) + ptag_count(2)
		return io.ErrUnexpectedEOF
	}

	reader := bytes.NewReader(data)

	// Author serial
	authorBuf := make([]byte, 5)
	if _, err := reader.Read(authorBuf); err != nil {
		return err
	}
	ev.AuthorSerial = decodeUint40(authorBuf)

	// Kind
	kindBuf := make([]byte, 2)
	if _, err := reader.Read(kindBuf); err != nil {
		return err
	}
	ev.Kind = binary.BigEndian.Uint16(kindBuf)

	// P-tags
	ptagCountBuf := make([]byte, 2)
	if _, err := reader.Read(ptagCountBuf); err != nil {
		return err
	}
	ptagCount := binary.BigEndian.Uint16(ptagCountBuf)
	ev.PTagSerials = make([]uint64, ptagCount)
	for i := uint16(0); i < ptagCount; i++ {
		serialBuf := make([]byte, 5)
		if _, err := reader.Read(serialBuf); err != nil {
			return err
		}
		ev.PTagSerials[i] = decodeUint40(serialBuf)
	}

	// E-tags
	etagCountBuf := make([]byte, 2)
	if _, err := reader.Read(etagCountBuf); err != nil {
		return err
	}
	etagCount := binary.BigEndian.Uint16(etagCountBuf)
	ev.ETagSerials = make([]uint64, etagCount)
	for i := uint16(0); i < etagCount; i++ {
		serialBuf := make([]byte, 5)
		if _, err := reader.Read(serialBuf); err != nil {
			return err
		}
		ev.ETagSerials[i] = decodeUint40(serialBuf)
	}

	return nil
}

// PubkeyVertex stores the adjacency list for a pubkey.
// Contains all events authored by or mentioning this pubkey.
type PubkeyVertex struct {
	AuthoredEvents []uint64 // Event serials this pubkey authored
	MentionedIn    []uint64 // Event serials that mention this pubkey (p-tags)
}

// Encode serializes the PubkeyVertex to bytes.
// Format: authored_count(varint) | [serials(5)...] | mentioned_count(varint) | [serials(5)...]
func (pv *PubkeyVertex) Encode() []byte {
	size := 2 + len(pv.AuthoredEvents)*5 + 2 + len(pv.MentionedIn)*5
	buf := make([]byte, 0, size)

	// Authored events
	authoredCountBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(authoredCountBuf, uint16(len(pv.AuthoredEvents)))
	buf = append(buf, authoredCountBuf...)
	for _, serial := range pv.AuthoredEvents {
		serialBuf := make([]byte, 5)
		encodeUint40(serial, serialBuf)
		buf = append(buf, serialBuf...)
	}

	// Mentioned in events
	mentionedCountBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(mentionedCountBuf, uint16(len(pv.MentionedIn)))
	buf = append(buf, mentionedCountBuf...)
	for _, serial := range pv.MentionedIn {
		serialBuf := make([]byte, 5)
		encodeUint40(serial, serialBuf)
		buf = append(buf, serialBuf...)
	}

	return buf
}

// Decode deserializes bytes into a PubkeyVertex.
func (pv *PubkeyVertex) Decode(data []byte) error {
	if len(data) < 4 { // minimum: authored_count(2) + mentioned_count(2)
		return io.ErrUnexpectedEOF
	}

	reader := bytes.NewReader(data)

	// Authored events
	authoredCountBuf := make([]byte, 2)
	if _, err := reader.Read(authoredCountBuf); err != nil {
		return err
	}
	authoredCount := binary.BigEndian.Uint16(authoredCountBuf)
	pv.AuthoredEvents = make([]uint64, authoredCount)
	for i := uint16(0); i < authoredCount; i++ {
		serialBuf := make([]byte, 5)
		if _, err := reader.Read(serialBuf); err != nil {
			return err
		}
		pv.AuthoredEvents[i] = decodeUint40(serialBuf)
	}

	// Mentioned in events
	mentionedCountBuf := make([]byte, 2)
	if _, err := reader.Read(mentionedCountBuf); err != nil {
		return err
	}
	mentionedCount := binary.BigEndian.Uint16(mentionedCountBuf)
	pv.MentionedIn = make([]uint64, mentionedCount)
	for i := uint16(0); i < mentionedCount; i++ {
		serialBuf := make([]byte, 5)
		if _, err := reader.Read(serialBuf); err != nil {
			return err
		}
		pv.MentionedIn[i] = decodeUint40(serialBuf)
	}

	return nil
}

// AddAuthored adds an event serial to the authored list if not already present.
func (pv *PubkeyVertex) AddAuthored(eventSerial uint64) {
	for _, s := range pv.AuthoredEvents {
		if s == eventSerial {
			return
		}
	}
	pv.AuthoredEvents = append(pv.AuthoredEvents, eventSerial)
}

// AddMention adds an event serial to the mentioned list if not already present.
func (pv *PubkeyVertex) AddMention(eventSerial uint64) {
	for _, s := range pv.MentionedIn {
		if s == eventSerial {
			return
		}
	}
	pv.MentionedIn = append(pv.MentionedIn, eventSerial)
}

// RemoveAuthored removes an event serial from the authored list.
func (pv *PubkeyVertex) RemoveAuthored(eventSerial uint64) {
	for i, s := range pv.AuthoredEvents {
		if s == eventSerial {
			pv.AuthoredEvents = append(pv.AuthoredEvents[:i], pv.AuthoredEvents[i+1:]...)
			return
		}
	}
}

// RemoveMention removes an event serial from the mentioned list.
func (pv *PubkeyVertex) RemoveMention(eventSerial uint64) {
	for i, s := range pv.MentionedIn {
		if s == eventSerial {
			pv.MentionedIn = append(pv.MentionedIn[:i], pv.MentionedIn[i+1:]...)
			return
		}
	}
}

// HasAuthored checks if the pubkey authored the given event.
func (pv *PubkeyVertex) HasAuthored(eventSerial uint64) bool {
	for _, s := range pv.AuthoredEvents {
		if s == eventSerial {
			return true
		}
	}
	return false
}

// IsMentionedIn checks if the pubkey is mentioned in the given event.
func (pv *PubkeyVertex) IsMentionedIn(eventSerial uint64) bool {
	for _, s := range pv.MentionedIn {
		if s == eventSerial {
			return true
		}
	}
	return false
}
