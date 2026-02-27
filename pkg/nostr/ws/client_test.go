//go:build !js

package ws

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"git.mleku.dev/mleku/nostr/utils"
	"git.mleku.dev/mleku/nostr/utils/normalize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/websocket"
	"lol.mleku.dev/chk"
)

func TestPublish(t *testing.T) {
	// test note to be sent over websocket
	priv, pub := makeKeyPair(t)
	textNote := &event.E{
		Kind:      kind.TextNote.K,
		Content:   []byte("hello"),
		CreatedAt: 1672068534, // random fixed timestamp
		Tags:      tag.NewS(tag.NewFromAny("foo", "bar")),
		Pubkey:    pub,
	}
	sign := p8k.MustNew()
	var err error
	if err = sign.InitSec(priv); chk.E(err) {
	}
	err = textNote.Sign(sign)
	assert.NoError(t, err)

	// fake relay server
	var mu sync.Mutex // guards published to satisfy go test -race
	var published bool
	ws := newWebsocketServer(
		func(conn *websocket.Conn) {
			mu.Lock()
			published = true
			mu.Unlock()
			// verify the client sent exactly the textNote
			var raw []json.RawMessage
			err := websocket.JSON.Receive(conn, &raw)
			assert.NoError(t, err)

			event := parseEventMessage(t, raw)
			assert.True(
				t, utils.FastEqual(event.Serialize(), textNote.Serialize()),
			)

			// send back an ok nip-20 command result
			res := []any{"OK", hex.Enc(textNote.ID), true, ""}
			err = websocket.JSON.Send(conn, res)
			assert.NoError(t, err)
		},
	)
	defer ws.Close()

	// connect a client and send the text note
	rl := mustRelayConnect(t, ws.URL)
	err = rl.Publish(context.Background(), textNote)
	assert.NoError(t, err)

	assert.True(t, published, "fake relay server saw no event")
}

// func TestPublishBlocked(t *testing.T) {
// 	// test note to be sent over websocket
// 	textNote := &event.E{
// 		Kind: kind.TextNote, Content: []byte("hello"),
// 		CreatedAt: timestamp.Now(),
// 	}
// 	textNote.ID = textNote.GetIDBytes()
//
// 	// fake relay server
// 	ws := newWebsocketServer(
// 		func(conn *websocket.Conn) {
// 			// discard received message; not interested
// 			var raw []json.RawMessage
// 			err := websocket.JSON.Receive(conn, &raw)
// 			assert.NoError(t, err)
//
// 			// send back a not ok nip-20 command result
// 			res := []any{"OK", textNote.IdString(), false, "blocked"}
// 			websocket.JSON.Send(conn, res)
// 		},
// 	)
// 	defer ws.Close()
//
// 	// connect a client and send a text note
// 	rl := mustRelayConnect(t, ws.URL)
// 	err := rl.Publish(context.Background(), textNote)
// 	assert.Error(t, err)
// }

func TestPublishWriteFailed(t *testing.T) {
	// test note to be sent over websocket
	textNote := &event.E{
		Kind:      kind.TextNote.K,
		Content:   []byte("hello"),
		CreatedAt: time.Now().Unix(),
	}
	textNote.ID = textNote.GetIDBytes()
	// fake relay server
	ws := newWebsocketServer(
		func(conn *websocket.Conn) {
			// reject receive - force send error
			conn.Close()
		},
	)
	defer ws.Close()
	// connect a client and send a text note
	rl := mustRelayConnect(t, ws.URL)
	// Force brief period of time so that publish always fails on closed socket.
	time.Sleep(1 * time.Millisecond)
	err := rl.Publish(context.Background(), textNote)
	assert.Error(t, err)
}

func TestConnectContext(t *testing.T) {
	// fake relay server
	var mu sync.Mutex // guards connected to satisfy go test -race
	var connected bool
	ws := newWebsocketServer(
		func(conn *websocket.Conn) {
			mu.Lock()
			connected = true
			mu.Unlock()
			io.ReadAll(conn) // discard all input
		},
	)
	defer ws.Close()

	// relay client
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, err := RelayConnect(ctx, ws.URL)
	assert.NoError(t, err)

	defer r.Close()

	mu.Lock()
	defer mu.Unlock()
	assert.True(t, connected, "fake relay server saw no client connect")
}

func TestConnectContextCanceled(t *testing.T) {
	// fake relay server
	ws := newWebsocketServer(discardingHandler)
	defer ws.Close()

	// relay client
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // make ctx expired
	_, err := RelayConnect(ctx, ws.URL)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestConnectWithOrigin(t *testing.T) {
	// fake relay server
	// default handler requires origin golang.org/x/net/websocket
	ws := httptest.NewServer(websocket.Handler(discardingHandler))
	defer ws.Close()

	// relay client
	r := NewRelay(
		context.Background(), string(normalize.URL(ws.URL)),
		WithRequestHeader(http.Header{"origin": {"https://example.com"}}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := r.Connect(ctx)
	assert.NoError(t, err)
}

func discardingHandler(conn *websocket.Conn) {
	io.ReadAll(conn) // discard all input
}

func newWebsocketServer(handler func(*websocket.Conn)) *httptest.Server {
	return httptest.NewServer(
		&websocket.Server{
			Handshake: anyOriginHandshake,
			Handler:   handler,
		},
	)
}

// anyOriginHandshake is an alternative to default in golang.org/x/net/websocket
// which checks for origin. nostr client sends no origin and it makes no difference
// for the tests here anyway.
var anyOriginHandshake = func(conf *websocket.Config, r *http.Request) error {
	return nil
}

func makeKeyPair(t *testing.T) (sec, pub []byte) {
	t.Helper()
	sign := p8k.MustNew()
	var err error
	if err = sign.Generate(); chk.E(err) {
		return
	}
	sec = sign.Sec()
	pub = sign.Pub()
	assert.NoError(t, err)

	return
}

func mustRelayConnect(t *testing.T, url string) *Client {
	t.Helper()

	rl, err := RelayConnect(context.Background(), url)
	require.NoError(t, err)

	return rl
}

func parseEventMessage(t *testing.T, raw []json.RawMessage) *event.E {
	t.Helper()

	assert.Condition(
		t, func() (success bool) {
			return len(raw) >= 2
		},
	)

	var typ string
	err := json.Unmarshal(raw[0], &typ)
	assert.NoError(t, err)
	assert.Equal(t, "EVENT", typ)

	event := &event.E{}
	_, err = event.Unmarshal(raw[1])
	require.NoError(t, err)

	return event
}

func parseSubscriptionMessage(
	t *testing.T, raw []json.RawMessage,
) (subid string, ff *filter.S) {
	t.Helper()

	assert.Greater(t, len(raw), 3)

	var typ string
	err := json.Unmarshal(raw[0], &typ)

	assert.NoError(t, err)
	assert.Equal(t, "REQ", typ)

	var id string
	err = json.Unmarshal(raw[1], &id)
	assert.NoError(t, err)
	ff = &filter.S{}
	for _, b := range raw[2:] {
		var f *filter.F
		err = json.Unmarshal(b, &f)
		assert.NoError(t, err)
		*ff = append(*ff, f)
	}
	return id, ff
}
