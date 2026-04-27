package publish

import (
	"time"

	"github.com/gorilla/websocket"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/interfaces/publisher"
	"git.smesh.lol/orly/pkg/interfaces/typer"
)

// WriteRequest represents a write operation to be performed by the write worker
type WriteRequest struct {
	Data      []byte
	MsgType   int
	IsControl bool
	Deadline  time.Time
	IsPing    bool // Special marker for ping messages
}

// WriteChanSetter defines the interface for setting write channels
type WriteChanSetter interface {
	SetWriteChan(*websocket.Conn, chan WriteRequest)
	GetWriteChan(*websocket.Conn) (chan WriteRequest, bool)
}

// NIP46SignerChecker defines the interface for checking active NIP-46 signers
type NIP46SignerChecker interface {
	HasActiveNIP46Signer(signerPubkey []byte) bool
}

// S is the control structure for the subscription management scheme.
type S struct {
	publisher.Publishers
}

// New creates a new publish.S.
func New(p ...publisher.I) (s *S) {
	s = &S{Publishers: p}
	return
}

var _ publisher.I = &S{}

func (s *S) Type() string { return "publish" }

func (s *S) Deliver(ev *event.E) {
	for _, p := range s.Publishers {
		p.Deliver(ev)
	}
}

func (s *S) Receive(msg typer.T) {
	t := msg.Type()
	for _, p := range s.Publishers {
		if p.Type() == t {
			p.Receive(msg)
			return
		}
	}
}

// GetSocketPublisher returns the socketapi publisher instance
func (s *S) GetSocketPublisher() WriteChanSetter {
	for _, p := range s.Publishers {
		if p.Type() == "socketapi" {
			if socketPub, ok := p.(WriteChanSetter); ok {
				return socketPub
			}
		}
	}
	return nil
}
