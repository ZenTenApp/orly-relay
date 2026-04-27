package types

import (
	"io"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/nostr/crypto/ec/schnorr"
	"github.com/minio/sha256-simd"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
)

const PubHashLen = 8

type PubHash struct{ val [PubHashLen]byte }

func (ph *PubHash) FromPubkey(pk []byte) (err error) {
	if len(pk) == 0 {
		panic("nil pubkey")
	}
	if len(pk) != schnorr.PubKeyBytesLen {
		err = errorf.E(
			"invalid Pubkey length, got %d require %d %0x",
			len(pk), schnorr.PubKeyBytesLen, pk,
		)
		return
	}
	pkh := sha256.Sum256(pk)
	copy(ph.val[:], pkh[:PubHashLen])
	return
}

func (ph *PubHash) FromPubkeyHex(pk string) (err error) {
	if len(pk) != schnorr.PubKeyBytesLen*2 {
		err = errorf.E(
			"invalid Pubkey length, got %d require %d",
			len(pk), schnorr.PubKeyBytesLen*2,
		)
		return
	}
	var pkb []byte
	if pkb, err = hex.Dec(pk); chk.E(err) {
		return
	}
	h := sha256.Sum256(pkb)
	copy(ph.val[:], h[:PubHashLen])
	return
}

func (ph *PubHash) Bytes() (b []byte) { return ph.val[:] }

func (ph *PubHash) MarshalWrite(w io.Writer) (err error) {
	_, err = w.Write(ph.val[:])
	return
}

func (ph *PubHash) UnmarshalRead(r io.Reader) (err error) {
	copy(ph.val[:], ph.val[:PubHashLen])
	_, err = r.Read(ph.val[:])
	return
}
