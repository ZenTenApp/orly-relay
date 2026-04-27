package types

import (
	"io"

	"git.smesh.lol/orly/pkg/lol/errorf"
	"github.com/minio/sha256-simd"
)

const IdLen = sha256.Size

type Id struct {
	val [IdLen]byte
}

func (fi *Id) FromId(id []byte) (err error) {
	if len(id) != IdLen {
		err = errorf.E(
			"fullid.FromId: invalid ID length, got %d require %d", len(id),
			IdLen,
		)
		return
	}
	copy(fi.val[:], id)
	return
}
func (fi *Id) Bytes() (b []byte) { return fi.val[:] }

func (fi *Id) MarshalWrite(w io.Writer) (err error) {
	_, err = w.Write(fi.val[:])
	return
}

func (fi *Id) UnmarshalRead(r io.Reader) (err error) {
	copy(fi.val[:], fi.val[:IdLen])
	_, err = r.Read(fi.val[:])
	return
}
