package types

import (
	"io"

	"lol.mleku.dev/chk"
)

const LetterLen = 1

// Edge direction constants for pubkey graph relationships
const (
	EdgeDirectionAuthor  byte = 0 // The pubkey is the event author
	EdgeDirectionPTagOut byte = 1 // Outbound: Event author references this pubkey in p-tag
	EdgeDirectionPTagIn  byte = 2 // Inbound: This pubkey is referenced in event's p-tag
)

// Edge direction constants for event-to-event (e-tag) graph relationships
const (
	EdgeDirectionETagOut byte = 0 // Outbound: This event references target event via e-tag
	EdgeDirectionETagIn  byte = 1 // Inbound: This event is referenced by source event via e-tag
)

// Edge direction constants for pubkey-to-pubkey (noun-noun) graph relationships
// These are materialized from events containing p-tags, collapsing the two-hop
// pubkey→event→pubkey traversal into a direct single-hop edge.
const (
	EdgeDirectionPubkeyOut byte = 0 // Outbound: source pubkey references target pubkey (via p-tag authorship)
	EdgeDirectionPubkeyIn  byte = 1 // Inbound: target pubkey is referenced by source pubkey
)

type Letter struct {
	val byte
}

func (p *Letter) Set(lb byte) { p.val = lb }

func (p *Letter) Letter() byte { return p.val }

func (p *Letter) MarshalWrite(w io.Writer) (err error) {
	_, err = w.Write([]byte{p.val})
	return
}

func (p *Letter) UnmarshalRead(r io.Reader) (err error) {
	var val [1]byte // Fixed array avoids heap escape
	if _, err = r.Read(val[:]); chk.E(err) {
		return
	}
	p.val = val[0]
	return
}
