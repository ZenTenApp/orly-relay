package types

import (
	"encoding/base64"
	"io"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"github.com/minio/sha256-simd"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

const IdHashLen = 8

type IdHash struct{ val [IdHashLen]byte }

func (i *IdHash) Set(idh []byte) {
	if len(idh) != IdHashLen {
		panic("invalid IdHash length")
	}
	copy(i.val[:], idh)
}

func (i *IdHash) FromId(id []byte) (err error) {
	if len(id) != sha256.Size {
		err = errorf.E(
			"FromId: invalid ID length, got %d require %d (data=%x)", len(id),
			sha256.Size, id,
		)
		return
	}
	idh := sha256.Sum256(id)
	copy(i.val[:], idh[:IdHashLen])
	return
}

func (i *IdHash) FromIdBase64(idb64 string) (err error) {
	// Decode the base64 string
	decoded, err := base64.RawURLEncoding.DecodeString(idb64)
	if chk.E(err) {
		return
	}

	// Check if the decoded ID has the correct length
	if len(decoded) != sha256.Size {
		err = errorf.E(
			"FromIdBase64: invalid ID length, got %d require %d", len(decoded),
			sha256.Size,
		)
		return
	}

	// Hash the decoded ID and take the first IdHashLen bytes
	idh := sha256.Sum256(decoded)
	copy(i.val[:], idh[:IdHashLen])
	return
}

func (i *IdHash) FromIdHex(idh string) (err error) {
	var id []byte
	if id, err = hex.Dec(idh); chk.E(err) {
		return
	}
	if len(id) != sha256.Size {
		err = errorf.E(
			"FromIdHex: invalid ID length, got %d require %d", len(id),
			sha256.Size,
		)
		return
	}
	h := sha256.Sum256(id)
	copy(i.val[:], h[:IdHashLen])
	return

}

func (i *IdHash) Bytes() (b []byte) { return i.val[:] }

func (i *IdHash) MarshalWrite(w io.Writer) (err error) {
	_, err = w.Write(i.val[:])
	return
}

func (i *IdHash) UnmarshalRead(r io.Reader) (err error) {
	_, err = r.Read(i.val[:])
	return
}
