package marmot

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"github.com/emersion/go-mls"
)

// NostrGroupData holds the Marmot group metadata carried in the MLS
// group context extension 0xf2ee. Version 2 is current.
type NostrGroupData struct {
	Version      uint16
	NostrGroupID [32]byte
	Name         string
	Description  string
	AdminPubkeys [][]byte // 32-byte x-only pubkeys
	Relays       []string // WebSocket relay URLs
}

// NewNostrGroupData creates a NostrGroupData with a random group ID.
func NewNostrGroupData(name string, adminPub []byte, relays []string) (*NostrGroupData, error) {
	var groupID [32]byte
	if _, err := rand.Read(groupID[:]); err != nil {
		return nil, fmt.Errorf("generate group id: %w", err)
	}
	return &NostrGroupData{
		Version:      2,
		NostrGroupID: groupID,
		Name:         name,
		AdminPubkeys: [][]byte{adminPub},
		Relays:       relays,
	}, nil
}

// MarshalExtension serializes to the TLS wire format expected by MIP-01.
func (d *NostrGroupData) MarshalExtension() (mls.Extension, error) {
	data, err := d.marshalBytes()
	if err != nil {
		return mls.Extension{}, err
	}
	return mls.NewExtension(mls.ExtensionTypeNostrGroupData, data), nil
}

func (d *NostrGroupData) marshalBytes() ([]byte, error) {
	var buf []byte

	// version (uint16 big-endian)
	buf = binary.BigEndian.AppendUint16(buf, d.Version)

	// nostr_group_id (fixed 32 bytes)
	buf = append(buf, d.NostrGroupID[:]...)

	// name (QUIC varint length prefix + UTF-8)
	buf = appendQUICVec(buf, []byte(d.Name))

	// description (QUIC varint length prefix + UTF-8)
	buf = appendQUICVec(buf, []byte(d.Description))

	// admin_pubkeys (QUIC varint length prefix + concatenated 32-byte keys)
	adminData := make([]byte, 0, 32*len(d.AdminPubkeys))
	for _, pk := range d.AdminPubkeys {
		if len(pk) != 32 {
			return nil, fmt.Errorf("admin pubkey must be 32 bytes, got %d", len(pk))
		}
		adminData = append(adminData, pk...)
	}
	buf = appendQUICVec(buf, adminData)

	// relays (QUIC varint outer length + individually prefixed URLs)
	var relayBuf []byte
	for _, r := range d.Relays {
		relayBuf = appendQUICVec(relayBuf, []byte(r))
	}
	buf = appendQUICVec(buf, relayBuf)

	// image_hash, image_key, image_nonce, image_upload_key (all empty)
	buf = appendQUICVec(buf, nil)
	buf = appendQUICVec(buf, nil)
	buf = appendQUICVec(buf, nil)
	buf = appendQUICVec(buf, nil)

	return buf, nil
}

// UnmarshalNostrGroupData parses the TLS extension data.
func UnmarshalNostrGroupData(data []byte) (*NostrGroupData, error) {
	if len(data) < 34 {
		return nil, fmt.Errorf("extension data too short: %d bytes", len(data))
	}

	d := &NostrGroupData{}
	d.Version = binary.BigEndian.Uint16(data[0:2])
	copy(d.NostrGroupID[:], data[2:34])

	rest := data[34:]
	var err error
	var b []byte

	// name
	b, rest, err = readQUICVec(rest)
	if err != nil {
		return nil, fmt.Errorf("read name: %w", err)
	}
	d.Name = string(b)

	// description
	b, rest, err = readQUICVec(rest)
	if err != nil {
		return nil, fmt.Errorf("read description: %w", err)
	}
	d.Description = string(b)

	// admin_pubkeys
	b, rest, err = readQUICVec(rest)
	if err != nil {
		return nil, fmt.Errorf("read admin_pubkeys: %w", err)
	}
	if len(b)%32 != 0 {
		return nil, fmt.Errorf("admin_pubkeys length %d not multiple of 32", len(b))
	}
	for i := 0; i < len(b); i += 32 {
		pk := make([]byte, 32)
		copy(pk, b[i:i+32])
		d.AdminPubkeys = append(d.AdminPubkeys, pk)
	}

	// relays
	b, rest, err = readQUICVec(rest)
	if err != nil {
		return nil, fmt.Errorf("read relays: %w", err)
	}
	for len(b) > 0 {
		var url []byte
		url, b, err = readQUICVec(b)
		if err != nil {
			return nil, fmt.Errorf("read relay url: %w", err)
		}
		d.Relays = append(d.Relays, string(url))
	}

	// Skip image fields (we don't use them for DMs)
	_ = rest

	return d, nil
}

// appendQUICVec appends a QUIC-style variable-length integer prefix followed
// by the data bytes. RFC 9000 Section 16.
func appendQUICVec(dst, data []byte) []byte {
	n := uint64(len(data))
	if n <= 63 {
		dst = append(dst, byte(n))
	} else if n <= 16383 {
		dst = append(dst, byte(0x40|n>>8), byte(n))
	} else if n <= 1073741823 {
		dst = append(dst, byte(0x80|n>>24), byte(n>>16), byte(n>>8), byte(n))
	} else {
		dst = append(dst,
			byte(0xc0|n>>56), byte(n>>48), byte(n>>40), byte(n>>32),
			byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	dst = append(dst, data...)
	return dst
}

// readQUICVec reads a QUIC varint-prefixed byte vector.
func readQUICVec(data []byte) (vec, rest []byte, err error) {
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("unexpected end of data")
	}

	prefix := data[0] >> 6
	var length uint64

	switch prefix {
	case 0:
		length = uint64(data[0] & 0x3f)
		data = data[1:]
	case 1:
		if len(data) < 2 {
			return nil, nil, fmt.Errorf("truncated 2-byte varint")
		}
		length = uint64(data[0]&0x3f)<<8 | uint64(data[1])
		data = data[2:]
	case 2:
		if len(data) < 4 {
			return nil, nil, fmt.Errorf("truncated 4-byte varint")
		}
		length = uint64(data[0]&0x3f)<<24 | uint64(data[1])<<16 | uint64(data[2])<<8 | uint64(data[3])
		data = data[4:]
	case 3:
		if len(data) < 8 {
			return nil, nil, fmt.Errorf("truncated 8-byte varint")
		}
		length = uint64(data[0]&0x3f)<<56 | uint64(data[1])<<48 | uint64(data[2])<<40 | uint64(data[3])<<32 |
			uint64(data[4])<<24 | uint64(data[5])<<16 | uint64(data[6])<<8 | uint64(data[7])
		data = data[8:]
	}

	if uint64(len(data)) < length {
		return nil, nil, fmt.Errorf("data too short: need %d, have %d", length, len(data))
	}

	return data[:length], data[length:], nil
}
