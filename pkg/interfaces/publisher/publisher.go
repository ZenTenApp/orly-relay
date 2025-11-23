package publisher

import (
	"git.mleku.dev/mleku/nostr/encoders/event"
	"next.orly.dev/pkg/interfaces/typer"
)

type I interface {
	typer.T
	Deliver(ev *event.E)
	Receive(msg typer.T)
}

type Publishers []I
