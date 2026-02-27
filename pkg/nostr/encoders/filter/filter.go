package filter

import (
	"bytes"
	"sort"

	"next.orly.dev/pkg/nostr/crypto/ec/schnorr"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/ints"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/text"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	"next.orly.dev/pkg/nostr/utils"
	"next.orly.dev/pkg/nostr/utils/pointers"
	"github.com/minio/sha256-simd"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
)

// F is the primary query form for requesting events from a nostr relay.
//
// The ordering of fields of filters is not specified as in the protocol there
// is no requirement to generate a hash for fast recognition of identical
// filters. However, for internal use in a relay, by applying a consistent sort
// order, this library will produce an identical JSON from the same *set* of
// fields no matter what order they were provided.
//
// This is to facilitate the deduplication of filters so an effective identical
// match is not performed on an identical filter.
type F struct {
	Ids     *tag.T       `json:"ids,omitempty"`
	Kinds   *kind.S      `json:"kinds,omitempty"`
	Authors *tag.T       `json:"authors,omitempty"`
	Tags    *tag.S       `json:"-,omitempty"`
	Since   *timestamp.T `json:"since,omitempty"`
	Until   *timestamp.T `json:"until,omitempty"`
	Search  []byte       `json:"search,omitempty"`
	Limit   *uint        `json:"limit,omitempty"`

	// Extra holds unknown JSON fields for relay extensions (e.g., _graph).
	// Per NIP-01, unknown fields should be ignored by relays that don't support them,
	// but relays implementing extensions can access them here.
	// Keys are stored without quotes, values are raw JSON bytes.
	Extra map[string][]byte `json:"-"`
}

// New creates a new, reasonably initialized filter that will be ready for most uses without
// further allocations.
func New() (f *F) {
	return &F{
		Ids:     tag.NewWithCap(10),
		Kinds:   kind.NewWithCap(10),
		Authors: tag.NewWithCap(10),
		Tags:    tag.NewSWithCap(10),
		Since:   timestamp.New(),
		Until:   timestamp.New(),
	}
}

var (
	// IDs is the JSON object key for IDs.
	IDs = []byte("ids")
	// Kinds is the JSON object key for Kinds.
	Kinds = []byte("kinds")
	// Authors is the JSON object key for Authors.
	Authors = []byte("authors")
	// Since is the JSON object key for Since.
	Since = []byte("since")
	// Until is the JSON object key for Until.
	Until = []byte("until")
	// Limit is the JSON object key for Limit.
	Limit = []byte("limit")
	// Search is the JSON object key for Search.
	Search = []byte("search")
)

// Sort the fields of a filter so a fingerprint on a filter that has the same set of content
// produces the same fingerprint.
func (f *F) Sort() {
	if f.Ids != nil {
		sort.Sort(f.Ids)
	}
	if f.Kinds != nil {
		sort.Sort(f.Kinds)
	}
	if f.Authors != nil {
		sort.Sort(f.Authors)
	}
	if f.Tags != nil {
		for i, v := range *f.Tags {
			if len(v.T) > 2 {
				vv := (v.T)[1:]
				sort.Slice(
					vv, func(i, j int) bool {
						return bytes.Compare((v.T)[i+1], (v.T)[j+1]) < 0
					},
				)
				// keep the first as is, this is the #x prefix
				first := (v.T)[:1]
				// append the sorted values to the prefix
				v.T = append(first, vv...)
				// replace the old value with the sorted one
				(*f.Tags)[i] = v
			}
		}
		sort.Sort(f.Tags)
	}
}

// MatchesIgnoringTimestampConstraints checks a filter against an event and
// determines if the event matches the filter, ignoring timestamp constraints..
func (f *F) MatchesIgnoringTimestampConstraints(ev *event.E) bool {
	if ev == nil {
		return false
	}
	if f.Ids.Len() > 0 && !f.Ids.Contains(ev.ID) {
		return false
	}
	if f.Kinds.Len() > 0 && !f.Kinds.Contains(ev.Kind) {
		return false
	}
	if f.Authors.Len() > 0 {
		found := false
		for _, author := range f.Authors.T {
			if utils.FastEqual(author, ev.Pubkey) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.Tags.Len() > 0 {
		for _, v := range *f.Tags {
			if v.Len() < 2 {
				continue
			}
			key := v.Key()
			values := v.T[1:]
			if !ev.Tags.ContainsAny(key, values) {
				return false
			}
		}
	}
	return true
}

// Matches checks a filter against an event and determines if the event matches the filter.
func (f *F) Matches(ev *event.E) (match bool) {
	if !f.MatchesIgnoringTimestampConstraints(ev) {
		return
	}
	if f.Since.Int() != 0 && ev.CreatedAt < f.Since.I64() {
		return
	}
	if f.Until.Int() != 0 && ev.CreatedAt > f.Until.I64() {
		return
	}
	return true
}

// EstimateSize returns an estimated size for marshaling the filter to JSON.
// This accounts for worst-case expansion of escaped content and hex encoding.
func (f *F) EstimateSize() (size int) {
	// JSON structure overhead: {, }, commas, quotes, keys
	size = 50

	// IDs: "ids":["hex1","hex2",...]
	if f.Ids != nil && f.Ids.Len() > 0 {
		size += 7 // "ids":[
		for _, id := range f.Ids.T {
			size += 2*len(id) + 4 // hex encoding + quotes + comma
		}
		size += 1 // closing ]
	}

	// Kinds: "kinds":[1,2,3,...]
	if f.Kinds.Len() > 0 {
		size += 9                 // "kinds":[
		size += f.Kinds.Len() * 5 // assume average 5 bytes per kind number
		size += 1                 // closing ]
	}

	// Authors: "authors":["hex1","hex2",...]
	if f.Authors.Len() > 0 {
		size += 11 // "authors":[
		for _, auth := range f.Authors.T {
			size += 2*len(auth) + 4 // hex encoding + quotes + comma
		}
		size += 1 // closing ]
	}

	// Tags: "#x":["val1","val2",...]
	if f.Tags != nil && f.Tags.Len() > 0 {
		for _, tg := range *f.Tags {
			if tg == nil || tg.Len() < 2 {
				continue
			}
			size += 6 // "#x":[
			for _, val := range tg.T[1:] {
				size += len(val)*2 + 4 // escaped value + quotes + comma
			}
			size += 1 // closing ]
		}
	}

	// Since: "since":1234567890
	if f.Since != nil && f.Since.U64() > 0 {
		size += 10 // "since": + timestamp
	}

	// Until: "until":1234567890
	if f.Until != nil && f.Until.U64() > 0 {
		size += 10 // "until": + timestamp
	}

	// Search: "search":"escaped text"
	if len(f.Search) > 0 {
		size += 11                // "search":"
		size += len(f.Search) * 2 // worst case escaping
		size += 1                 // closing quote
	}

	// Limit: "limit":100
	if pointers.Present(f.Limit) {
		size += 11 // "limit": + number
	}

	return
}

// Marshal a filter into raw JSON bytes, minified. The field ordering and sort
// of fields is canonicalized so that a hash can identify the same filter.
func (f *F) Marshal(dst []byte) (b []byte) {
	var err error
	_ = err
	var first bool
	// Pre-allocate buffer if nil to reduce reallocations
	if dst == nil {
		estimatedSize := f.EstimateSize()
		dst = make([]byte, 0, estimatedSize)
	}
	// sort the fields so they come out the same
	f.Sort()
	// open parentheses
	b = dst
	b = append(b, '{')
	if f.Ids != nil && f.Ids.Len() > 0 {
		first = true
		b = text.JSONKey(b, IDs)
		b = text.MarshalHexArray(b, f.Ids.T)
	}
	if f.Kinds.Len() > 0 {
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = text.JSONKey(b, Kinds)
		b = f.Kinds.Marshal(b)
	}
	if f.Authors.Len() > 0 {
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = text.JSONKey(b, Authors)
		b = text.MarshalHexArray(b, f.Authors.T)
	}
	if f.Tags != nil && f.Tags.Len() > 0 {
		// tags are stored as tags with the initial element the "#a" and the rest the list in
		// each element of the tags list. eg:
		//
		//     [["#p","<pubkey1>","<pubkey3"],["#t","hashtag","stuff"]]
		//
		for _, tg := range *f.Tags {
			if tg == nil {
				// nothing here
				continue
			}
			if tg.Len() < 2 {
				// must have at least key and one value
				continue
			}
			tKey := tg.T[0]
			if len(tKey) != 1 ||
				((tKey[0] < 'a' || tKey[0] > 'z') && (tKey[0] < 'A' || tKey[0] > 'Z')) {
				// key must be single alpha character
				continue
			}
			values := tg.T[1:]
			if len(values) == 0 {
				continue
			}
			if first {
				b = append(b, ',')
			} else {
				first = true
			}
			// append the key with # prefix
			b = append(b, '"', '#', tKey[0], '"', ':')
			b = append(b, '[')
			for i, value := range values {
				b = text.AppendQuote(b, value, text.NostrEscape)
				if i < len(values)-1 {
					b = append(b, ',')
				}
			}
			b = append(b, ']')
		}
	}
	if f.Since != nil && f.Since.U64() > 0 {
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = text.JSONKey(b, Since)
		b = f.Since.Marshal(b)
	}
	if f.Until != nil && f.Until.U64() > 0 {
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = text.JSONKey(b, Until)
		b = f.Until.Marshal(b)
	}
	if len(f.Search) > 0 {
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = text.JSONKey(b, Search)
		b = text.AppendQuote(b, f.Search, text.NostrEscape)
	}
	if pointers.Present(f.Limit) {
		if first {
			b = append(b, ',')
		} else {
			first = true
		}
		b = text.JSONKey(b, Limit)
		b = ints.New(*f.Limit).Marshal(b)
	}
	// close parentheses
	b = append(b, '}')
	return
}

// Serialize a filter.F into raw minified JSON bytes.
func (f *F) Serialize() (b []byte) { return f.Marshal(nil) }

// states of the unmarshaler
const (
	beforeOpen = iota
	openParen
	inKey
	inKV
	inVal
	betweenKV
	afterClose
)

// Unmarshal a filter from raw (minified) JSON bytes into the runtime format.
//
// todo: this may tolerate whitespace, not certain currently.
func (f *F) Unmarshal(b []byte) (r []byte, err error) {
	r = b
	var key []byte
	var state int
	for ; len(r) > 0; r = r[1:] {
		// log.I.ToSliceOfBytes("%c", rem[0])
		switch state {
		case beforeOpen:
			if r[0] == '{' {
				state = openParen
				// log.I.Ln("openParen")
			}
		case openParen:
			if r[0] == '"' {
				state = inKey
				// log.I.Ln("inKey")
			}
		case inKey:
			if r[0] == '"' {
				state = inKV
				// log.I.Ln("inKV")
			} else {
				// Pre-allocate key buffer if needed
				if key == nil {
					key = make([]byte, 0, 16)
				}
				key = append(key, r[0])
			}
		case inKV:
			if r[0] == ':' {
				state = inVal
			}
		case inVal:
			if len(key) < 1 {
				err = errorf.E("filter key zero length: '%s'\n'%s", b, r)
				return
			}
			switch key[0] {
			case '#':
				// tags start with # and have 1 letter
				l := len(key)
				if l != 2 {
					err = errorf.E(
						"filter tag keys can only be # and one alpha character: '%s'\n%s",
						key, b,
					)
					return
				}
				// Store just the single-character tag key (e.g., 'p'), not the '#' prefix
				// This matches how event tags store their keys (e.g., ["p", "pubkey"])
				k := make([]byte, 1)
				k[0] = key[1]
				var ff [][]byte
				if ff, r, err = text.UnmarshalStringArray(r); chk.E(err) {
					return
				}
				ff = append([][]byte{k}, ff...)
				if f.Tags == nil {
					f.Tags = tag.NewSWithCap(1)
				}
				s := append(*f.Tags, tag.NewFromBytesSlice(ff...))
				f.Tags = &s
				state = betweenKV
			case IDs[0]:
				if len(key) < len(IDs) {
					goto invalid
				}
				var ff [][]byte
				if ff, r, err = text.UnmarshalHexArray(
					r, sha256.Size,
				); chk.E(err) {
					return
				}
				f.Ids = tag.NewFromBytesSlice(ff...)
				state = betweenKV
			case Kinds[0]:
				if len(key) < len(Kinds) {
					goto invalid
				}
				f.Kinds = kind.NewWithCap(0)
				if r, err = f.Kinds.Unmarshal(r); chk.E(err) {
					return
				}
				state = betweenKV
			case Authors[0]:
				if len(key) < len(Authors) {
					goto invalid
				}
				var ff [][]byte
				if ff, r, err = text.UnmarshalHexArray(
					r, schnorr.PubKeyBytesLen,
				); chk.E(err) {
					return
				}
				f.Authors = tag.NewFromBytesSlice(ff...)
				state = betweenKV
			case Until[0]:
				if len(key) < len(Until) {
					goto invalid
				}
				u := ints.New(0)
				if r, err = u.Unmarshal(r); chk.E(err) {
					return
				}
				f.Until = timestamp.FromUnix(int64(u.N))
				state = betweenKV
			case Limit[0]:
				if len(key) < len(Limit) {
					goto invalid
				}
				l := ints.New(0)
				if r, err = l.Unmarshal(r); chk.E(err) {
					return
				}
				u := uint(l.N)
				f.Limit = &u
				state = betweenKV
			case Search[0]:
				if len(key) < len(Since) {
					goto invalid
				}
				switch key[1] {
				case Search[1]:
					if len(key) < len(Search) {
						goto invalid
					}
					var txt []byte
					if txt, r, err = text.UnmarshalQuoted(r); chk.E(err) {
						return
					}
					f.Search = txt
					// log.I.ToSliceOfBytes("\n%s\n%s", txt, rem)
					state = betweenKV
					// log.I.Ln("betweenKV")
				case Since[1]:
					if len(key) < len(Since) {
						goto invalid
					}
					s := ints.New(0)
					if r, err = s.Unmarshal(r); chk.E(err) {
						return
					}
					f.Since = timestamp.FromUnix(int64(s.N))
					state = betweenKV
					// log.I.Ln("betweenKV")
				}
			default:
				// Per NIP-01, unknown filter fields should be ignored by relays that
				// don't support them. Store them in Extra for extensions to use.
				var val []byte
				if val, r, err = skipJSONValue(r); err != nil {
					goto invalid
				}
				if f.Extra == nil {
					f.Extra = make(map[string][]byte)
				}
				// Store the raw JSON value keyed by the field name
				f.Extra[string(key)] = val
				state = betweenKV
			}
			key = key[:0]
		case betweenKV:
			if len(r) == 0 {
				return
			}
			if r[0] == '}' {
				state = afterClose
				// log.I.Ln("afterClose")
				// rem = rem[1:]
			} else if r[0] == ',' {
				state = openParen
				// log.I.Ln("openParen")
			} else if r[0] == '"' {
				state = inKey
				// log.I.Ln("inKey")
			}
		}
		if len(r) == 0 {
			return
		}
		if r[0] == '}' {
			r = r[1:]
			return
		}
	}
invalid:
	err = errorf.E("invalid key,\n'%s'\n'%s'", string(b), string(r))
	return
}
