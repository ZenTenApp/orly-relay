package nostr

import (
	"common/crypto/sha256"
	"common/helpers"
)

// Event is a Nostr event (NIP-01).
type Event struct {
	ID        string `json:"id"`
	PubKey    string `json:"pubkey"`
	CreatedAt int64  `json:"created_at"`
	Kind      int    `json:"kind"`
	Tags      Tags   `json:"tags"`
	Content   string `json:"content"`
	Sig       string `json:"sig"`
}

// Serialize returns the canonical JSON array for ID computation:
// [0,<pubkey>,<created_at>,<kind>,<tags>,<content>]
func (e *Event) Serialize() string {
	buf := make([]byte, 0, 256)
	buf = append(buf, "[0,"...)
	buf = append(buf, helpers.JsonString(e.PubKey)...)
	buf = append(buf, ',')
	buf = append(buf, helpers.Itoa(e.CreatedAt)...)
	buf = append(buf, ',')
	buf = append(buf, helpers.Itoa(int64(e.Kind))...)
	buf = append(buf, ',')
	buf = serializeTags(buf, e.Tags)
	buf = append(buf, ',')
	buf = append(buf, helpers.JsonString(e.Content)...)
	buf = append(buf, ']')
	return string(buf)
}

// ComputeID computes and sets the event ID (SHA-256 of serialized form).
func (e *Event) ComputeID() string {
	ser := e.Serialize()
	hash := sha256.Sum([]byte(ser))
	e.ID = helpers.HexEncode(hash[:])
	return e.ID
}

// CheckID verifies the event ID matches the content.
func (e *Event) CheckID() bool {
	ser := e.Serialize()
	hash := sha256.Sum([]byte(ser))
	expected := helpers.HexEncode(hash[:])
	return e.ID == expected
}

func serializeTags(buf []byte, tags Tags) []byte {
	buf = append(buf, '[')
	for i, tag := range tags {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		for j, s := range tag {
			if j > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, helpers.JsonString(s)...)
		}
		buf = append(buf, ']')
	}
	buf = append(buf, ']')
	return buf
}
