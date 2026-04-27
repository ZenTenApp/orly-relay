//go:build !js && !wasm

package marmot

import (
	"context"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/ws"
)

// WSRelayAdapter wraps a ws.Client to satisfy the RelayConnection interface.
type WSRelayAdapter struct {
	client *ws.Client
}

// NewWSRelayAdapter creates a RelayConnection from an existing ws.Client.
func NewWSRelayAdapter(c *ws.Client) *WSRelayAdapter {
	return &WSRelayAdapter{client: c}
}

func (a *WSRelayAdapter) Publish(ctx context.Context, ev *event.E) error {
	return a.client.Publish(ctx, ev)
}

func (a *WSRelayAdapter) Subscribe(ctx context.Context, ff *filter.S) (EventStream, error) {
	sub, err := a.client.Subscribe(ctx, ff)
	if err != nil {
		return nil, err
	}
	return &wsEventStream{sub: sub}, nil
}

// wsEventStream wraps ws.Subscription to satisfy EventStream.
type wsEventStream struct {
	sub *ws.Subscription
}

func (s *wsEventStream) Events() <-chan *event.E { return s.sub.Events }
func (s *wsEventStream) Close()                  { s.sub.Unsub() }
