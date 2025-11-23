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

type IdPkTs struct {
	Id  []byte
	Pub []byte
	Ts  int64
	Ser uint64
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
