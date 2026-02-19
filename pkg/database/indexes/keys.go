package indexes

import (
	"io"
	"reflect"

	"lol.mleku.dev/chk"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/interfaces/codec"
)

var counter int

func init() {
	// Initialize the counter to ensure it starts from 0
	counter = 0
}

func next() int { counter++; return counter - 1 }

type P struct {
	val []byte
}

func NewPrefix(prf ...int) (p *P) {
	if len(prf) > 0 {
		prefix := Prefix(prf[0])
		if prefix == "" {
			panic("unknown prefix")
		}
		return &P{[]byte(prefix)}
	} else {
		return &P{[]byte{0, 0, 0}}
	}
}

func (p *P) Bytes() (b []byte) { return p.val }

func (p *P) MarshalWrite(w io.Writer) (err error) {
	_, err = w.Write(p.val)
	return
}

func (p *P) UnmarshalRead(r io.Reader) (err error) {
	// Allocate a buffer for val if it's nil or empty
	if p.val == nil || len(p.val) == 0 {
		p.val = make([]byte, 3) // Prefixes are 3 bytes
	}
	_, err = r.Read(p.val)
	return
}

type I string

func (i I) Write(w io.Writer) (n int, err error) { return w.Write([]byte(i)) }

const (
	EventPrefix            = I("evt")
	SmallEventPrefix       = I("sev") // small event with inline data (<=384 bytes)
	ReplaceableEventPrefix = I("rev") // replaceable event (kinds 0,3,10000-19999) with inline data
	AddressableEventPrefix = I("aev") // addressable event (kinds 30000-39999) with inline data
	IdPrefix               = I("eid")
	FullIdPubkeyPrefix     = I("fpc") // full id, pubkey, created at

	CreatedAtPrefix  = I("c--") // created at
	KindPrefix       = I("kc-") // kind, created at
	PubkeyPrefix     = I("pc-") // pubkey, created at
	KindPubkeyPrefix = I("kpc") // kind, pubkey, created at

	TagPrefix           = I("tc-") // tag, created at
	TagKindPrefix       = I("tkc") // tag, kind, created at
	TagPubkeyPrefix     = I("tpc") // tag, pubkey, created at
	TagKindPubkeyPrefix = I("tkp") // tag, kind, pubkey, created at

	WordPrefix       = I("wrd") // word hash, serial
	ExpirationPrefix = I("exp") // timestamp of expiration
	VersionPrefix    = I("ver") // database version number, for triggering reindexes when new keys are added (policy is add-only).

	// Pubkey graph indexes
	PubkeySerialPrefix       = I("pks") // pubkey hash -> pubkey serial
	SerialPubkeyPrefix       = I("spk") // pubkey serial -> pubkey hash (full 32 bytes)
	EventPubkeyGraphPrefix   = I("epg") // event serial -> pubkey serial (graph edges)
	PubkeyEventGraphPrefix   = I("peg") // pubkey serial -> event serial (reverse edges)

	// Compact event storage indexes
	SerialEventIdPrefix = I("sei") // event serial -> full 32-byte event ID
	CompactEventPrefix  = I("cmp") // compact event storage with serial references

	// Event-to-event graph indexes (for e-tag references)
	EventEventGraphPrefix = I("eeg") // source event serial -> target event serial (outbound e-tags)
	GraphEventEventPrefix = I("gee") // target event serial -> source event serial (reverse e-tags)

	// Pubkey-to-pubkey graph indexes (noun-noun edges)
	// Materialized from events containing p-tags, collapsing the two-hop
	// pubkey→event→pubkey traversal into a direct single-hop lookup.
	PubkeyPubkeyGraphPrefix = I("ppg") // source pubkey serial -> target pubkey serial (outbound)
	GraphPubkeyPubkeyPrefix = I("gpp") // target pubkey serial -> source pubkey serial (reverse)
)

// Prefix returns the three byte human-readable prefixes that go in front of
// database indexes.
func Prefix(prf int) (i I) {
	switch prf {
	case Event:
		return EventPrefix
	case SmallEvent:
		return SmallEventPrefix
	case ReplaceableEvent:
		return ReplaceableEventPrefix
	case AddressableEvent:
		return AddressableEventPrefix
	case Id:
		return IdPrefix
	case FullIdPubkey:
		return FullIdPubkeyPrefix

	case CreatedAt:
		return CreatedAtPrefix
	case Kind:
		return KindPrefix
	case Pubkey:
		return PubkeyPrefix
	case KindPubkey:
		return KindPubkeyPrefix

	case Tag:
		return TagPrefix
	case TagKind:
		return TagKindPrefix
	case TagPubkey:
		return TagPubkeyPrefix
	case TagKindPubkey:
		return TagKindPubkeyPrefix

	case Expiration:
		return ExpirationPrefix
	case Version:
		return VersionPrefix
	case Word:
		return WordPrefix

	case PubkeySerial:
		return PubkeySerialPrefix
	case SerialPubkey:
		return SerialPubkeyPrefix
	case EventPubkeyGraph:
		return EventPubkeyGraphPrefix
	case PubkeyEventGraph:
		return PubkeyEventGraphPrefix

	case SerialEventId:
		return SerialEventIdPrefix
	case CompactEvent:
		return CompactEventPrefix

	case EventEventGraph:
		return EventEventGraphPrefix
	case GraphEventEvent:
		return GraphEventEventPrefix

	case PubkeyPubkeyGraph:
		return PubkeyPubkeyGraphPrefix
	case GraphPubkeyPubkey:
		return GraphPubkeyPubkeyPrefix
	}
	return
}

func Identify(r io.Reader) (i int, err error) {
	// this is here for completeness; however, searches don't need to identify
	// this as they work via generated prefixes made using Prefix.
	var b [3]byte
	_, err = r.Read(b[:])
	if err != nil {
		i = -1
		return
	}
	switch I(b[:]) {
	case EventPrefix:
		i = Event
	case SmallEventPrefix:
		i = SmallEvent
	case ReplaceableEventPrefix:
		i = ReplaceableEvent
	case AddressableEventPrefix:
		i = AddressableEvent
	case IdPrefix:
		i = Id
	case FullIdPubkeyPrefix:
		i = FullIdPubkey

	case CreatedAtPrefix:
		i = CreatedAt
	case KindPrefix:
		i = Kind
	case PubkeyPrefix:
		i = Pubkey
	case KindPubkeyPrefix:
		i = KindPubkey

	case TagPrefix:
		i = Tag
	case TagKindPrefix:
		i = TagKind
	case TagPubkeyPrefix:
		i = TagPubkey
	case TagKindPubkeyPrefix:
		i = TagKindPubkey

	case ExpirationPrefix:
		i = Expiration
	case WordPrefix:
		i = Word

	case PubkeySerialPrefix:
		i = PubkeySerial
	case SerialPubkeyPrefix:
		i = SerialPubkey
	case EventPubkeyGraphPrefix:
		i = EventPubkeyGraph
	case PubkeyEventGraphPrefix:
		i = PubkeyEventGraph

	case SerialEventIdPrefix:
		i = SerialEventId
	case CompactEventPrefix:
		i = CompactEvent

	case EventEventGraphPrefix:
		i = EventEventGraph
	case GraphEventEventPrefix:
		i = GraphEventEvent

	case PubkeyPubkeyGraphPrefix:
		i = PubkeyPubkeyGraph
	case GraphPubkeyPubkeyPrefix:
		i = GraphPubkeyPubkey
	}
	return
}

type Encs []codec.I

// T is a wrapper around an array of codec.I. The caller provides the Encs so
// they can then call the accessor methods of the codec.I implementation.
type T struct{ Encs }

// New creates a new indexes.T. The helper functions below have an encode and
// decode variant, the decode variant doesn't add the prefix encoder because it
// has been read by Identify or just is being read, and found because it was
// written for the prefix in the iteration.
func New(encoders ...codec.I) (i *T) { return &T{encoders} }
func (t *T) MarshalWrite(w io.Writer) (err error) {
	for _, e := range t.Encs {
		if e == nil || reflect.ValueOf(e).IsNil() {
			// Skip nil encoders instead of returning early. This enables
			// generating search prefixes.
			continue
		}
		if err = e.MarshalWrite(w); chk.E(err) {
			return
		}
	}
	return
}
func (t *T) UnmarshalRead(r io.Reader) (err error) {
	for _, e := range t.Encs {
		if err = e.UnmarshalRead(r); chk.E(err) {
			return
		}
	}
	return
}

// Event is the whole event stored in binary format
//
//	prefix|5 serial - event in binary format
var Event = next()

func EventVars() (ser *types.Uint40) { return new(types.Uint40) }
func EventEnc(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(Event), ser)
}
func EventDec(ser *types.Uint40) (enc *T) { return New(NewPrefix(), ser) }

// SmallEvent stores events <=384 bytes with inline data to avoid double lookup.
// This is a Reiser4-inspired optimization for small event packing.
// 384 bytes covers: ID(32) + Pubkey(32) + Sig(64) + basic fields + small content
//
//	prefix|5 serial|2 size_uint16|data (variable length, max 384 bytes)
var SmallEvent = next()

func SmallEventVars() (ser *types.Uint40) { return new(types.Uint40) }
func SmallEventEnc(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(SmallEvent), ser)
}
func SmallEventDec(ser *types.Uint40) (enc *T) { return New(NewPrefix(), ser) }

// ReplaceableEvent stores replaceable events (kinds 0,3,10000-19999) with inline data.
// Optimized storage for metadata events that are frequently replaced.
// Key format enables direct lookup by pubkey+kind without additional index traversal.
//
//	prefix|8 pubkey_hash|2 kind|2 size_uint16|data (variable length, max 384 bytes)
var ReplaceableEvent = next()

func ReplaceableEventVars() (p *types.PubHash, ki *types.Uint16) {
	return new(types.PubHash), new(types.Uint16)
}
func ReplaceableEventEnc(p *types.PubHash, ki *types.Uint16) (enc *T) {
	return New(NewPrefix(ReplaceableEvent), p, ki)
}
func ReplaceableEventDec(p *types.PubHash, ki *types.Uint16) (enc *T) {
	return New(NewPrefix(), p, ki)
}

// AddressableEvent stores parameterized replaceable events (kinds 30000-39999) with inline data.
// Optimized storage for addressable events identified by pubkey+kind+d-tag.
// Key format enables direct lookup without additional index traversal.
//
//	prefix|8 pubkey_hash|2 kind|8 dtag_hash|2 size_uint16|data (variable length, max 384 bytes)
var AddressableEvent = next()

func AddressableEventVars() (p *types.PubHash, ki *types.Uint16, d *types.Ident) {
	return new(types.PubHash), new(types.Uint16), new(types.Ident)
}
func AddressableEventEnc(p *types.PubHash, ki *types.Uint16, d *types.Ident) (enc *T) {
	return New(NewPrefix(AddressableEvent), p, ki, d)
}
func AddressableEventDec(p *types.PubHash, ki *types.Uint16, d *types.Ident) (enc *T) {
	return New(NewPrefix(), p, ki, d)
}

// Id contains a truncated 8-byte hash of an event index. This is the secondary
// key of an event, the primary key is the serial found in the Event.
//
//	3 prefix|8 ID hash|5 serial
var Id = next()

func IdVars() (id *types.IdHash, ser *types.Uint40) {
	return new(types.IdHash), new(types.Uint40)
}
func IdEnc(id *types.IdHash, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(Id), id, ser)
}
func IdDec(id *types.IdHash, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), id, ser)
}

// FullIdPubkey is an index designed to enable sorting and filtering of
// results found via other indexes, without having to decode the event.
//
//	3 prefix|5 serial|32 ID|8 pubkey hash|8 timestamp
var FullIdPubkey = next()

func FullIdPubkeyVars() (
	ser *types.Uint40, fid *types.Id, p *types.PubHash, ca *types.Uint64,
) {
	return new(types.Uint40), new(types.Id), new(types.PubHash), new(types.Uint64)
}
func FullIdPubkeyEnc(
	ser *types.Uint40, fid *types.Id, p *types.PubHash, ca *types.Uint64,
) (enc *T) {
	return New(NewPrefix(FullIdPubkey), ser, fid, p, ca)
}
func FullIdPubkeyDec(
	ser *types.Uint40, fid *types.Id, p *types.PubHash, ca *types.Uint64,
) (enc *T) {
	return New(NewPrefix(), ser, fid, p, ca)
}

// Word index for tokenized search terms
//
//	3 prefix|8 word-hash|5 serial
var Word = next()

func WordVars() (w *types.Word, ser *types.Uint40) {
	return new(types.Word), new(types.Uint40)
}
func WordEnc(w *types.Word, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(Word), w, ser)
}
func WordDec(w *types.Word, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), w, ser)
}

// CreatedAt is an index that allows search for the timestamp on the event.
//
//	3 prefix|8 timestamp|5 serial
var CreatedAt = next()

func CreatedAtVars() (ca *types.Uint64, ser *types.Uint40) {
	return new(types.Uint64), new(types.Uint40)
}
func CreatedAtEnc(ca *types.Uint64, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(CreatedAt), ca, ser)
}
func CreatedAtDec(ca *types.Uint64, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), ca, ser)
}

// Kind
//
//	3 prefix|2 kind|8 timestamp|5 serial
var Kind = next()

func KindVars() (ki *types.Uint16, ca *types.Uint64, ser *types.Uint40) {
	return new(types.Uint16), new(types.Uint64), new(types.Uint40)
}
func KindEnc(ki *types.Uint16, ca *types.Uint64, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(Kind), ki, ca, ser)
}
func KindDec(ki *types.Uint16, ca *types.Uint64, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), ki, ca, ser)
}

// Pubkey is a composite index that allows search by pubkey
// filtered by timestamp.
//
//	3 prefix|8 pubkey hash|8 timestamp|5 serial
var Pubkey = next()

func PubkeyVars() (p *types.PubHash, ca *types.Uint64, ser *types.Uint40) {
	return new(types.PubHash), new(types.Uint64), new(types.Uint40)
}
func PubkeyEnc(p *types.PubHash, ca *types.Uint64, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(Pubkey), p, ca, ser)
}
func PubkeyDec(p *types.PubHash, ca *types.Uint64, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), p, ca, ser)
}

// KindPubkey
//
//	3 prefix|2 kind|8 pubkey hash|8 timestamp|5 serial
var KindPubkey = next()

func KindPubkeyVars() (
	ki *types.Uint16, p *types.PubHash, ca *types.Uint64, ser *types.Uint40,
) {
	return new(types.Uint16), new(types.PubHash), new(types.Uint64), new(types.Uint40)
}
func KindPubkeyEnc(
	ki *types.Uint16, p *types.PubHash, ca *types.Uint64, ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(KindPubkey), ki, p, ca, ser)
}
func KindPubkeyDec(
	ki *types.Uint16, p *types.PubHash, ca *types.Uint64, ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(), ki, p, ca, ser)
}

// Tag allows searching for a tag and filter by timestamp.
//
//	3 prefix|1 key letter|8 value hash|8 timestamp|5 serial
var Tag = next()

func TagVars() (
	k *types.Letter, v *types.Ident, ca *types.Uint64, ser *types.Uint40,
) {
	return new(types.Letter), new(types.Ident), new(types.Uint64), new(types.Uint40)
}
func TagEnc(
	k *types.Letter, v *types.Ident, ca *types.Uint64, ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(Tag), k, v, ca, ser)
}
func TagDec(
	k *types.Letter, v *types.Ident, ca *types.Uint64, ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(), k, v, ca, ser)
}

// TagKind
//
//	3 prefix|1 key letter|8 value hash|2 kind|8 timestamp|5 serial
var TagKind = next()

func TagKindVars() (
	k *types.Letter, v *types.Ident, ki *types.Uint16, ca *types.Uint64,
	ser *types.Uint40,
) {
	return new(types.Letter), new(types.Ident), new(types.Uint16), new(types.Uint64), new(types.Uint40)
}
func TagKindEnc(
	k *types.Letter, v *types.Ident, ki *types.Uint16, ca *types.Uint64,
	ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(TagKind), ki, k, v, ca, ser)
}
func TagKindDec(
	k *types.Letter, v *types.Ident, ki *types.Uint16, ca *types.Uint64,
	ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(), ki, k, v, ca, ser)
}

// TagPubkey allows searching for a pubkey, tag and timestamp.
//
//	3 prefix|1 key letter|8 value hash|8 pubkey hash|8 timestamp|5 serial
var TagPubkey = next()

func TagPubkeyVars() (
	k *types.Letter, v *types.Ident, p *types.PubHash, ca *types.Uint64,
	ser *types.Uint40,
) {
	return new(types.Letter), new(types.Ident), new(types.PubHash), new(types.Uint64), new(types.Uint40)
}
func TagPubkeyEnc(
	k *types.Letter, v *types.Ident, p *types.PubHash, ca *types.Uint64,
	ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(TagPubkey), p, k, v, ca, ser)
}
func TagPubkeyDec(
	k *types.Letter, v *types.Ident, p *types.PubHash, ca *types.Uint64,
	ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(), p, k, v, ca, ser)
}

// TagKindPubkey
//
//	3 prefix|1 key letter|8 value hash|2 kind|8 pubkey hash|8 bytes timestamp|5 serial
var TagKindPubkey = next()

func TagKindPubkeyVars() (
	k *types.Letter, v *types.Ident, ki *types.Uint16, p *types.PubHash,
	ca *types.Uint64,
	ser *types.Uint40,
) {
	return new(types.Letter), new(types.Ident), new(types.Uint16), new(types.PubHash), new(types.Uint64), new(types.Uint40)
}
func TagKindPubkeyEnc(
	k *types.Letter, v *types.Ident, ki *types.Uint16, p *types.PubHash,
	ca *types.Uint64,
	ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(TagKindPubkey), ki, p, k, v, ca, ser)
}
func TagKindPubkeyDec(
	k *types.Letter, v *types.Ident, ki *types.Uint16, p *types.PubHash,
	ca *types.Uint64,
	ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(), ki, p, k, v, ca, ser)
}

// Expiration
//
// 3 prefix|8 timestamp|5 serial
var Expiration = next()

func ExpirationVars() (
	exp *types.Uint64, ser *types.Uint40,
) {
	return new(types.Uint64), new(types.Uint40)
}
func ExpirationEnc(
	exp *types.Uint64, ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(Expiration), exp, ser)
}
func ExpirationDec(
	exp *types.Uint64, ser *types.Uint40,
) (enc *T) {
	return New(NewPrefix(), exp, ser)
}

// Version
//
// 3 prefix|4 version
var Version = next()

func VersionVars() (
	ver *types.Uint32,
) {
	return new(types.Uint32)
}
func VersionEnc(
	ver *types.Uint32,
) (enc *T) {
	return New(NewPrefix(Version), ver)
}
func VersionDec(
	ver *types.Uint32,
) (enc *T) {
	return New(NewPrefix(), ver)
}

// PubkeySerial maps a pubkey hash to its unique serial number
//
//	3 prefix|8 pubkey hash|5 serial
var PubkeySerial = next()

func PubkeySerialVars() (p *types.PubHash, ser *types.Uint40) {
	return new(types.PubHash), new(types.Uint40)
}
func PubkeySerialEnc(p *types.PubHash, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(PubkeySerial), p, ser)
}
func PubkeySerialDec(p *types.PubHash, ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), p, ser)
}

// SerialPubkey maps a pubkey serial to the full 32-byte pubkey
// This stores the full pubkey (32 bytes) as the value, not inline
//
//	3 prefix|5 serial -> 32 byte pubkey value
var SerialPubkey = next()

func SerialPubkeyVars() (ser *types.Uint40) {
	return new(types.Uint40)
}
func SerialPubkeyEnc(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(SerialPubkey), ser)
}
func SerialPubkeyDec(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), ser)
}

// EventPubkeyGraph creates a bidirectional graph edge between events and pubkeys
// This stores event_serial -> pubkey_serial relationships with event kind and direction
// Direction: 0=author, 1=p-tag-out (event references pubkey)
//
//	3 prefix|5 event serial|5 pubkey serial|2 kind|1 direction
var EventPubkeyGraph = next()

func EventPubkeyGraphVars() (eventSer *types.Uint40, pubkeySer *types.Uint40, kind *types.Uint16, direction *types.Letter) {
	return new(types.Uint40), new(types.Uint40), new(types.Uint16), new(types.Letter)
}
func EventPubkeyGraphEnc(eventSer *types.Uint40, pubkeySer *types.Uint40, kind *types.Uint16, direction *types.Letter) (enc *T) {
	return New(NewPrefix(EventPubkeyGraph), eventSer, pubkeySer, kind, direction)
}
func EventPubkeyGraphDec(eventSer *types.Uint40, pubkeySer *types.Uint40, kind *types.Uint16, direction *types.Letter) (enc *T) {
	return New(NewPrefix(), eventSer, pubkeySer, kind, direction)
}

// PubkeyEventGraph creates the reverse edge: pubkey_serial -> event_serial with event kind and direction
// This enables querying all events related to a pubkey, optionally filtered by kind and direction
// Direction: 0=is-author, 2=p-tag-in (pubkey is referenced by event)
//
//	3 prefix|5 pubkey serial|2 kind|1 direction|5 event serial
var PubkeyEventGraph = next()

func PubkeyEventGraphVars() (pubkeySer *types.Uint40, kind *types.Uint16, direction *types.Letter, eventSer *types.Uint40) {
	return new(types.Uint40), new(types.Uint16), new(types.Letter), new(types.Uint40)
}
func PubkeyEventGraphEnc(pubkeySer *types.Uint40, kind *types.Uint16, direction *types.Letter, eventSer *types.Uint40) (enc *T) {
	return New(NewPrefix(PubkeyEventGraph), pubkeySer, kind, direction, eventSer)
}
func PubkeyEventGraphDec(pubkeySer *types.Uint40, kind *types.Uint16, direction *types.Letter, eventSer *types.Uint40) (enc *T) {
	return New(NewPrefix(), pubkeySer, kind, direction, eventSer)
}

// SerialEventId maps an event serial to its full 32-byte event ID.
// This enables reconstruction of the original event ID from compact storage.
// The event ID is stored as the value (32 bytes), not inline in the key.
//
//	3 prefix|5 serial -> 32 byte event ID value
var SerialEventId = next()

func SerialEventIdVars() (ser *types.Uint40) {
	return new(types.Uint40)
}
func SerialEventIdEnc(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(SerialEventId), ser)
}
func SerialEventIdDec(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(), ser)
}

// CompactEvent stores events using serial references instead of full IDs/pubkeys.
// This dramatically reduces storage size by replacing:
// - 32-byte event ID with 5-byte serial
// - 32-byte author pubkey with 5-byte pubkey serial
// - 32-byte e-tag values with 5-byte event serials (or full ID if unknown)
// - 32-byte p-tag values with 5-byte pubkey serials
//
// Format: cmp|5 serial|compact event data (variable length)
var CompactEvent = next()

func CompactEventVars() (ser *types.Uint40) { return new(types.Uint40) }
func CompactEventEnc(ser *types.Uint40) (enc *T) {
	return New(NewPrefix(CompactEvent), ser)
}
func CompactEventDec(ser *types.Uint40) (enc *T) { return New(NewPrefix(), ser) }

// EventEventGraph creates a bidirectional graph edge between events via e-tags.
// This stores source_event_serial -> target_event_serial relationships with event kind and direction.
// Used for thread traversal and finding replies/reactions/reposts to events.
// Direction: 0=outbound (this event references target)
//
//	3 prefix|5 source event serial|5 target event serial|2 kind|1 direction
var EventEventGraph = next()

func EventEventGraphVars() (srcSer *types.Uint40, tgtSer *types.Uint40, kind *types.Uint16, direction *types.Letter) {
	return new(types.Uint40), new(types.Uint40), new(types.Uint16), new(types.Letter)
}
func EventEventGraphEnc(srcSer *types.Uint40, tgtSer *types.Uint40, kind *types.Uint16, direction *types.Letter) (enc *T) {
	return New(NewPrefix(EventEventGraph), srcSer, tgtSer, kind, direction)
}
func EventEventGraphDec(srcSer *types.Uint40, tgtSer *types.Uint40, kind *types.Uint16, direction *types.Letter) (enc *T) {
	return New(NewPrefix(), srcSer, tgtSer, kind, direction)
}

// GraphEventEvent creates the reverse edge: target_event_serial -> source_event_serial with kind and direction.
// This enables querying all events that reference a target event (e.g., all replies to a post).
// Direction: 1=inbound (target is referenced by source)
//
//	3 prefix|5 target event serial|2 kind|1 direction|5 source event serial
var GraphEventEvent = next()

func GraphEventEventVars() (tgtSer *types.Uint40, kind *types.Uint16, direction *types.Letter, srcSer *types.Uint40) {
	return new(types.Uint40), new(types.Uint16), new(types.Letter), new(types.Uint40)
}
func GraphEventEventEnc(tgtSer *types.Uint40, kind *types.Uint16, direction *types.Letter, srcSer *types.Uint40) (enc *T) {
	return New(NewPrefix(GraphEventEvent), tgtSer, kind, direction, srcSer)
}
func GraphEventEventDec(tgtSer *types.Uint40, kind *types.Uint16, direction *types.Letter, srcSer *types.Uint40) (enc *T) {
	return New(NewPrefix(), tgtSer, kind, direction, srcSer)
}

// PubkeyPubkeyGraph creates a direct noun-noun edge between pubkeys.
// This materializes the pubkey→event→pubkey two-hop traversal into a single-hop
// lookup, enabling direct social graph queries without event decode.
// The event kind that created the relationship is preserved for filtering
// (e.g., kind 3 = follow, kind 1 = mention, kind 7 = reaction).
// The event serial is included to enable back-traversal to the originating event.
//
//	3 prefix|5 source pubkey serial|5 target pubkey serial|2 kind|1 direction|5 event serial
var PubkeyPubkeyGraph = next()

func PubkeyPubkeyGraphVars() (srcPk *types.Uint40, tgtPk *types.Uint40, kind *types.Uint16, direction *types.Letter, eventSer *types.Uint40) {
	return new(types.Uint40), new(types.Uint40), new(types.Uint16), new(types.Letter), new(types.Uint40)
}
func PubkeyPubkeyGraphEnc(srcPk *types.Uint40, tgtPk *types.Uint40, kind *types.Uint16, direction *types.Letter, eventSer *types.Uint40) (enc *T) {
	return New(NewPrefix(PubkeyPubkeyGraph), srcPk, tgtPk, kind, direction, eventSer)
}
func PubkeyPubkeyGraphDec(srcPk *types.Uint40, tgtPk *types.Uint40, kind *types.Uint16, direction *types.Letter, eventSer *types.Uint40) (enc *T) {
	return New(NewPrefix(), srcPk, tgtPk, kind, direction, eventSer)
}

// GraphPubkeyPubkey creates the reverse noun-noun edge: target pubkey -> source pubkey.
// This enables querying "who references this pubkey?" as a single prefix scan.
// Kind and direction are positioned before the trailing serial for efficient
// filtered scans (e.g., "all kind-3 followers of pubkey X").
//
//	3 prefix|5 target pubkey serial|2 kind|1 direction|5 source pubkey serial|5 event serial
var GraphPubkeyPubkey = next()

func GraphPubkeyPubkeyVars() (tgtPk *types.Uint40, kind *types.Uint16, direction *types.Letter, srcPk *types.Uint40, eventSer *types.Uint40) {
	return new(types.Uint40), new(types.Uint16), new(types.Letter), new(types.Uint40), new(types.Uint40)
}
func GraphPubkeyPubkeyEnc(tgtPk *types.Uint40, kind *types.Uint16, direction *types.Letter, srcPk *types.Uint40, eventSer *types.Uint40) (enc *T) {
	return New(NewPrefix(GraphPubkeyPubkey), tgtPk, kind, direction, srcPk, eventSer)
}
func GraphPubkeyPubkeyDec(tgtPk *types.Uint40, kind *types.Uint16, direction *types.Letter, srcPk *types.Uint40, eventSer *types.Uint40) (enc *T) {
	return New(NewPrefix(), tgtPk, kind, direction, srcPk, eventSer)
}
