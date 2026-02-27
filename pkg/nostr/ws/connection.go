package ws

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"git.mleku.dev/mleku/nostr/utils/units"
	"github.com/gorilla/websocket"
	"lol.mleku.dev/errorf"
)

// Connection represents a websocket connection to a Nostr relay.
type Connection struct {
	conn *websocket.Conn
}

// NewConnection creates a new websocket connection to a Nostr relay.
func NewConnection(
	ctx context.Context, url string, reqHeader http.Header,
	tlsConfig *tls.Config,
) (c *Connection, err error) {
	var conn *websocket.Conn
	var resp *http.Response
	dialer := getConnectionOptions(reqHeader, tlsConfig)

	// Prepare headers with default User-Agent if not present
	headers := reqHeader
	if headers == nil {
		headers = make(http.Header)
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", "github.com/nbd-wtf/go-nostr")
	}

	if conn, resp, err = dialer.DialContext(ctx, url, headers); err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return
	}
	conn.SetReadLimit(33 * units.Mb)
	// Set a pong handler to extend the read deadline when pong is received.
	// Without this, the 60-second read deadline in ReadMessage expires even
	// though pong frames are being received, because NextReader processes
	// pong frames without resetting the deadline.
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	return &Connection{
		conn: conn,
	}, nil
}

// WriteMessage writes arbitrary bytes to the websocket connection.
func (c *Connection) WriteMessage(
	ctx context.Context, data []byte,
) (err error) {
	deadline := time.Now().Add(10 * time.Second)
	if ctx != nil {
		if d, ok := ctx.Deadline(); ok {
			deadline = d
		}
	}
	c.conn.SetWriteDeadline(deadline)
	if err = c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		err = errorf.E("failed to write message: %w", err)
		return
	}
	return nil
}

// ReadMessage reads arbitrary bytes from the websocket connection into the provided buffer.
func (c *Connection) ReadMessage(
	ctx context.Context, buf io.Writer,
) (err error) {
	deadline := time.Now().Add(60 * time.Second)
	if ctx != nil {
		if d, ok := ctx.Deadline(); ok {
			deadline = d
		}
	}
	c.conn.SetReadDeadline(deadline)
	messageType, reader, err := c.conn.NextReader()
	if err != nil {
		err = fmt.Errorf("failed to get reader: %w", err)
		return
	}
	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		err = fmt.Errorf("unexpected message type: %d", messageType)
		return
	}
	if _, err = io.Copy(buf, reader); err != nil {
		err = fmt.Errorf("failed to read message: %w", err)
		return
	}
	return
}

// Close closes the websocket connection.
func (c *Connection) Close() error {
	c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second),
	)
	return c.conn.Close()
}

// Ping sends a ping message to the websocket connection.
func (c *Connection) Ping(ctx context.Context) error {
	deadline := time.Now().Add(800 * time.Millisecond)
	if ctx != nil {
		if d, ok := ctx.Deadline(); ok {
			deadline = d
		}
	}
	c.conn.SetWriteDeadline(deadline)
	return c.conn.WriteControl(websocket.PingMessage, []byte{}, deadline)
}
