// Package store is an interface and ancillary helpers and types for defining a
// series of API elements for abstracting the event storage from the
// implementation.
//
// It is composed so that the top-level interface can be
// partially implemented if need be.
package store

import (
	"context"
	"io"

	"next.orly.dev/app/config"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	ntypes "git.mleku.dev/mleku/nostr/types"
)

// I am a type for a persistence layer for nostr events handled by a relay.
type I interface {
	Pather
	io.Closer
	Pather
	Wiper
	Querier
	Querent
	Deleter
	Saver
	Importer
	Exporter
	Syncer
	LogLeveler
	EventIdSerialer
	Initer
	SerialByIder
}

type Initer interface {
	Init(path string) (err error)
}

type Pather interface {
	// Path returns the directory of the database.
	Path() (s string)
}

type Wiper interface {
	// Wipe deletes everything in the database.
	Wipe() (err error)
}

type Querent interface {
	// QueryEvents is invoked upon a client's REQ as described in NIP-01. It
	// returns the matching events in reverse chronological order in a slice.
	QueryEvents(c context.Context, f *filter.F) (evs event.S, err error)
}

type Accountant interface {
	EventCount() (count uint64, err error)
}

// IdPkTs holds event reference data using fixed-size arrays for stack allocation.
// Total size: 80 bytes (32+32+8+8), fits in a cache line.
// Copies of this struct stay on the stack and are value-safe.
type IdPkTs struct {
	Id  ntypes.EventID // 32 bytes - event ID
	Pub ntypes.Pubkey  // 32 bytes - author pubkey
	Ts  int64          // 8 bytes - created_at timestamp
	Ser uint64         // 8 bytes - database serial number
}

// NewIdPkTs creates an IdPkTs from byte slices (copies into fixed arrays).
func NewIdPkTs(id, pub []byte, ts int64, ser uint64) IdPkTs {
	return IdPkTs{
		Id:  ntypes.EventIDFromBytes(id),
		Pub: ntypes.PubkeyFromBytes(pub),
		Ts:  ts,
		Ser: ser,
	}
}

// IDSlice returns the event ID as a byte slice (shares memory with array).
func (i *IdPkTs) IDSlice() []byte {
	return i.Id[:]
}

// PubSlice returns the pubkey as a byte slice (shares memory with array).
func (i *IdPkTs) PubSlice() []byte {
	return i.Pub[:]
}

// IDHex returns the event ID as a lowercase hex string.
func (i *IdPkTs) IDHex() string {
	return i.Id.Hex()
}

// PubHex returns the pubkey as a lowercase hex string.
func (i *IdPkTs) PubHex() string {
	return i.Pub.Hex()
}

// ToEventRef converts IdPkTs to an EventRef.
func (i *IdPkTs) ToEventRef() EventRef {
	return EventRef{
		id:  i.Id,
		pub: i.Pub,
		ts:  i.Ts,
		ser: i.Ser,
	}
}

// EventRef is a stack-friendly event reference using fixed-size arrays.
// Total size: 80 bytes (32+32+8+8), fits in a cache line, copies stay on stack.
// Use this type when you need safe, immutable event references.
type EventRef struct {
	id  ntypes.EventID // 32 bytes
	pub ntypes.Pubkey  // 32 bytes
	ts  int64          // 8 bytes
	ser uint64         // 8 bytes
}

// NewEventRef creates an EventRef from byte slices.
// The slices are copied into fixed-size arrays.
func NewEventRef(id, pub []byte, ts int64, ser uint64) EventRef {
	return EventRef{
		id:  ntypes.EventIDFromBytes(id),
		pub: ntypes.PubkeyFromBytes(pub),
		ts:  ts,
		ser: ser,
	}
}

// ID returns the event ID (copy, stays on stack).
func (r EventRef) ID() ntypes.EventID { return r.id }

// Pub returns the pubkey (copy, stays on stack).
func (r EventRef) Pub() ntypes.Pubkey { return r.pub }

// Ts returns the timestamp.
func (r EventRef) Ts() int64 { return r.ts }

// Ser returns the serial number.
func (r EventRef) Ser() uint64 { return r.ser }

// IDHex returns the event ID as lowercase hex.
func (r EventRef) IDHex() string { return r.id.Hex() }

// PubHex returns the pubkey as lowercase hex.
func (r EventRef) PubHex() string { return r.pub.Hex() }

// IDSlice returns a slice view of the ID (shares memory, use carefully).
func (r *EventRef) IDSlice() []byte { return r.id.Bytes() }

// PubSlice returns a slice view of the pubkey (shares memory, use carefully).
func (r *EventRef) PubSlice() []byte { return r.pub.Bytes() }

// ToIdPkTs converts EventRef to IdPkTs.
func (r EventRef) ToIdPkTs() *IdPkTs {
	return &IdPkTs{
		Id:  r.id,
		Pub: r.pub,
		Ts:  r.ts,
		Ser: r.ser,
	}
}

type Querier interface {
	QueryForIds(c context.Context, f *filter.F) (evs []*IdPkTs, err error)
}

type GetIdsWriter interface {
	FetchIds(c context.Context, evIds *tag.T, out io.Writer) (err error)
}

type Deleter interface {
	// DeleteEvent is used to handle deletion events, as per NIP-09.
	DeleteEvent(c context.Context, ev []byte) (err error)
}

type Saver interface {
	// SaveEvent is called once relay.AcceptEvent reports true. The owners
	// parameter is for designating admins whose delete by e tag events apply
	// the same as author's own.
	SaveEvent(c context.Context, ev *event.E) (replaced bool, err error)
}

type Importer interface {
	// Import reads in a stream of line-structured JSON the events to save into
	// the store.
	Import(r io.Reader)
}

type Exporter interface {
	// Export writes a stream of line structured JSON of all events in the
	// store.
	//
	// If pubkeys are present, only those with these pubkeys in the `pubkey`
	// field and in `p` tags will be included.
	Export(c context.Context, w io.Writer, pubkeys ...[]byte)
}

type Rescanner interface {
	// Rescan triggers the regeneration of indexes of the database to enable old
	// records to be found with new indexes.
	Rescan() (err error)
}

type Syncer interface {
	// Sync signals the event store to flush its buffers.
	Sync() (err error)
}

type Configuration struct {
	BlockList []string `json:"block_list" doc:"list of IP addresses that will be ignored"`
}

type Configurationer interface {
	GetConfiguration() (c config.C, err error)
	SetConfiguration(c config.C) (err error)
}

type LogLeveler interface {
	SetLogLevel(level string)
}

type EventIdSerialer interface {
	EventIdsBySerial(start uint64, count int) (
		evs [][]byte,
		err error,
	)
}

type SerialByIder interface {
	GetSerialById(id []byte) (ser *types.Uint40, err error)
}
