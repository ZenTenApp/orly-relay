package types

import (
	"bytes"
	"io"

	"next.orly.dev/pkg/lol/chk"
)

const TimestampLen = 8

type Timestamp struct{ val int64 }

func (ts *Timestamp) FromInt(t int)     { ts.val = int64(t) }
func (ts *Timestamp) FromInt64(t int64) { ts.val = t }

func FromBytes(timestampBytes []byte) (ts *Timestamp, err error) {
	v := new(Uint64)
	if err = v.UnmarshalRead(bytes.NewBuffer(timestampBytes)); chk.E(err) {
		return
	}
	ts = &Timestamp{val: int64(v.Get())}
	return
}

func (ts *Timestamp) ToTimestamp() (timestamp int64) {
	return ts.val
}
func (ts *Timestamp) Bytes() (b []byte, err error) {
	v := new(Uint64)
	v.Set(uint64(ts.val))
	buf := new(bytes.Buffer)
	if err = v.MarshalWrite(buf); chk.E(err) {
		return
	}
	b = buf.Bytes()
	return
}

func (ts *Timestamp) MarshalWrite(w io.Writer) (err error) {
	v := new(Uint64)
	v.Set(uint64(ts.val))
	if err = v.MarshalWrite(w); chk.E(err) {
		return
	}
	return
}

func (ts *Timestamp) UnmarshalRead(r io.Reader) (err error) {
	v := new(Uint64)
	if err = v.UnmarshalRead(r); chk.E(err) {
		return
	}
	ts.val = int64(v.Get())
	return
}
