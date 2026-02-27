package filter

import (
	"math"

	"git.mleku.dev/mleku/nostr/crypto/ec/schnorr"
	"git.mleku.dev/mleku/nostr/crypto/ec/secp256k1"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/utils/values"
	"github.com/minio/sha256-simd"
	"lol.mleku.dev/chk"
	"lukechampine.com/frand"
)

// GenFilter is a testing tool to create random arbitrary filters for tests.
func GenFilter() (f *F, err error) {
	f = New()
	n := frand.Intn(16)
	for _ = range n {
		id := make([]byte, sha256.Size)
		frand.Read(id)
		f.Ids.T = append(f.Ids.T, id)
		// f.Ids.Field = append(f.Ids.Field, id)
	}
	n = frand.Intn(16)
	for _ = range n {
		f.Kinds.K = append(f.Kinds.K, kind.New(frand.Intn(math.MaxUint16)))
	}
	n = frand.Intn(16)
	for _ = range n {
		var sk *secp256k1.SecretKey
		if sk, err = secp256k1.GenerateSecretKey(); chk.E(err) {
			return
		}
		pk := sk.PubKey()
		f.Authors.T = append(f.Authors.T, schnorr.SerializePubKey(pk))
		// f.Authors.Field = append(f.Authors.Field, schnorr.SerializePubKey(pk))
	}
	a := frand.Intn(16)
	if a < n {
		n = a
	}
	for i := range n {
		p := make([]byte, 0, schnorr.PubKeyBytesLen*2)
		p = hex.EncAppend(p, f.Authors.T[i])
	}
	for b := 'a'; b <= 'z'; b++ {
		l := frand.Intn(6)
		var idb [][]byte
		for range l {
			bb := make([]byte, frand.Intn(31)+1)
			frand.Read(bb)
			id := make([]byte, 0, len(bb)*2)
			id = hex.EncAppend(id, bb)
			idb = append(idb, id)
		}
		idb = append([][]byte{{'#', byte(b)}}, idb...)
		*f.Tags = append(*f.Tags, tag.NewFromBytesSlice(idb...))
		// f.Tags.F = append(f.Tags.F, tag.FromBytesSlice(idb...))
	}
	tn := int(timestamp.Now().I64())
	f.Since = &timestamp.T{int64(tn - frand.Intn(10000))}
	f.Until = timestamp.Now()
	if frand.Intn(10) > 5 {
		f.Limit = values.ToUintPointer(uint(frand.Intn(1000)))
	}
	f.Search = []byte("token search text")
	return
}

func GenFilters() (s S, err error) {
	n := frand.Intn(5) + 1
	for _ = range n {
		var f *F
		if f, err = GenFilter(); chk.E(err) {
		}
		s = append(s, f)
	}
	return
}
