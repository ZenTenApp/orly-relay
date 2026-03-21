package nostr

import (
	"common/crypto/secp256k1"
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

// Sign computes the event ID and signs it with the given secret key.
// Sets ID, PubKey, and Sig. Returns false on failure.
func (e *Event) Sign(seckey [32]byte, auxRand [32]byte) bool {
	pk, ok := secp256k1.PubKeyFromSecKey(seckey)
	if !ok {
		return false
	}
	e.PubKey = helpers.HexEncode(pk[:])
	e.ComputeID()
	idBytes, ok := helpers.HexDecode32(e.ID)
	if !ok {
		return false
	}
	sig, ok := secp256k1.SignSchnorr(seckey, idBytes, auxRand)
	if !ok {
		return false
	}
	e.Sig = helpers.HexEncode(sig[:])
	return true
}

// CheckSig verifies the event signature against its pubkey and ID.
func (e *Event) CheckSig() bool {
	pk, ok := helpers.HexDecode32(e.PubKey)
	if !ok {
		return false
	}
	id, ok := helpers.HexDecode32(e.ID)
	if !ok {
		return false
	}
	if len(e.Sig) != 128 {
		return false
	}
	var sig [64]byte
	sigBytes := helpers.HexDecode(e.Sig)
	if len(sigBytes) != 64 {
		return false
	}
	copy(sig[:], sigBytes)
	return secp256k1.VerifySchnorr(pk, id, sig)
}

// ToJSON returns the event as a JSON object string.
func (e *Event) ToJSON() string {
	buf := make([]byte, 0, 512)
	buf = append(buf, `{"id":`...)
	buf = append(buf, helpers.JsonString(e.ID)...)
	buf = append(buf, `,"pubkey":`...)
	buf = append(buf, helpers.JsonString(e.PubKey)...)
	buf = append(buf, `,"created_at":`...)
	buf = append(buf, helpers.Itoa(e.CreatedAt)...)
	buf = append(buf, `,"kind":`...)
	buf = append(buf, helpers.Itoa(int64(e.Kind))...)
	buf = append(buf, `,"tags":`...)
	buf = serializeTags(buf, e.Tags)
	buf = append(buf, `,"content":`...)
	buf = append(buf, helpers.JsonString(e.Content)...)
	buf = append(buf, `,"sig":`...)
	buf = append(buf, helpers.JsonString(e.Sig)...)
	buf = append(buf, '}')
	return string(buf)
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
