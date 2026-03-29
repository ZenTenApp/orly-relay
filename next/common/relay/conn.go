package relay

import (
	"common/jsbridge/ws"
	"common/nostr"
)

// State constants for connection readiness.
const (
	StateConnecting = 0
	StateOpen       = 1
	StateClosed     = 2
)

// Conn is a single relay WebSocket connection.
type Conn struct {
	URL    string
	wsConn ws.Conn
	state  int
	subs   map[string]*Sub

	// Callbacks set before Dial returns.
	onReady func(bool)

	onEvent func(string, *nostr.Event)
	onEOSE  func(string)
	onOK    func(string, bool, string)
	onAuth  func(string)

	closing        bool     // true when Close() was called intentionally
	pendingPublish []string // EVENT messages queued while connecting

	// ScheduleReconnect is set by the consumer to provide a delayed callback.
	// When a connection drops with active subscriptions, this is called with
	// the reconnect function. The consumer wraps it in a timer (e.g. 5s delay).
	ScheduleReconnect func(fn func())
}

// Dial opens a connection to a relay.
// Call OnReady to receive the open/fail notification, then Start to begin processing.
func Dial(url string) *Conn {
	c := &Conn{
		URL:   url,
		state: StateConnecting,
		subs:  make(map[string]*Sub),
	}

	c.dial()
	return c
}

func (c *Conn) dial() {
	c.wsConn = ws.Dial(
		c.URL,
		func(connID int, data string) {
			c.handleMessage(data)
		},
		func(connID int) {
			c.state = StateOpen
			if c.onReady != nil {
				c.onReady(true)
				c.onReady = nil
			}
			c.flushSubs()
			c.flushPublish()
		},
		func(connID int, code int, reason string) {
			c.state = StateClosed
			if c.onReady != nil {
				c.onReady(false)
				c.onReady = nil
			}
			c.maybeReconnect()
		},
		func(connID int) {
			c.state = StateClosed
			if c.onReady != nil {
				c.onReady(false)
				c.onReady = nil
			}
			c.maybeReconnect()
		},
	)
}

func (c *Conn) maybeReconnect() {
	if c.closing || len(c.subs) == 0 || c.ScheduleReconnect == nil {
		return
	}
	c.state = StateConnecting // prevent pool from creating a duplicate
	c.ScheduleReconnect(func() {
		if c.closing {
			return
		}
		c.dial()
	})
}

// OnReady sets a callback that fires once when the connection opens (true) or fails (false).
func (c *Conn) OnReady(fn func(bool)) {
	if c.state == StateOpen {
		fn(true)
		return
	}
	if c.state == StateClosed {
		fn(false)
		return
	}
	c.onReady = fn
}

// IsOpen returns whether the connection is open.
func (c *Conn) IsOpen() bool {
	return c.state == StateOpen
}

func (c *Conn) handleMessage(msg string) {
	label, subID, payload := nostr.ParseRelayMessage(msg)

	switch label {
	case "EVENT":
		ev := nostr.ParseEvent(payload)
		if ev == nil {
			return
		}
		if sub, ok := c.subs[subID]; ok {
			if sub.OnEvent != nil {
				sub.OnEvent(ev)
			}
		}
		if c.onEvent != nil {
			c.onEvent(subID, ev)
		}

	case "EOSE":
		if sub, ok := c.subs[subID]; ok {
			sub.gotEOSE = true
			if sub.OnEOSE != nil {
				sub.OnEOSE()
			}
		}
		if c.onEOSE != nil {
			c.onEOSE(subID)
		}

	case "OK":
		ok := len(payload) > 0 && payload[0] == 't'
		msg := ""
		idx := indexOf(payload, ':')
		if idx >= 0 && idx+1 < len(payload) {
			msg = payload[idx+1:]
		}
		if c.onOK != nil {
			c.onOK(subID, ok, msg)
		}

	case "AUTH":
		if c.onAuth != nil {
			c.onAuth(payload)
		}

	case "NOTICE":
		_ = payload
	}
}

// Subscribe sends a REQ and tracks the subscription.
// If the connection is still opening, the REQ is deferred to flushSubs().
func (c *Conn) Subscribe(id string, filters []*nostr.Filter) *Sub {
	sub := &Sub{
		ID:      id,
		Filters: filters,
		conn:    c,
	}
	c.subs[id] = sub

	if c.state == StateOpen {
		msg := "[\"REQ\",\"" + id + "\""
		for _, f := range filters {
			msg += "," + f.Serialize()
		}
		msg += "]"
		ws.Send(c.wsConn, msg)
	}
	// else: flushSubs() sends all stored subs when connection opens.

	return sub
}

// Publish sends an EVENT message. Queues if the connection is still opening.
func (c *Conn) Publish(ev *nostr.Event) {
	msg := "[\"EVENT\"," + eventJSON(ev) + "]"
	if c.state != StateOpen {
		c.pendingPublish = append(c.pendingPublish, msg)
		return
	}
	ws.Send(c.wsConn, msg)
}

// CloseSubscription sends a CLOSE message.
func (c *Conn) CloseSubscription(id string) {
	delete(c.subs, id)
	msg := "[\"CLOSE\",\"" + id + "\"]"
	ws.Send(c.wsConn, msg)
}

// Send sends a raw JSON message string.
func (c *Conn) Send(msg string) {
	ws.Send(c.wsConn, msg)
}

// Close closes the connection intentionally (no reconnect).
func (c *Conn) Close() {
	c.closing = true
	c.state = StateClosed
	ws.Close(c.wsConn)
}

// SetOnEvent sets a global event handler (all subscriptions).
func (c *Conn) SetOnEvent(fn func(string, *nostr.Event)) {
	c.onEvent = fn
}

// SetOnEOSE sets a global EOSE handler.
func (c *Conn) SetOnEOSE(fn func(string)) {
	c.onEOSE = fn
}

// SetOnOK sets a handler for OK responses.
func (c *Conn) SetOnOK(fn func(string, bool, string)) {
	c.onOK = fn
}

// SetOnAuth sets a handler for AUTH challenges.
func (c *Conn) SetOnAuth(fn func(string)) {
	c.onAuth = fn
}

// flushPublish sends queued EVENT messages that arrived while connecting.
func (c *Conn) flushPublish() {
	for _, msg := range c.pendingPublish {
		ws.Send(c.wsConn, msg)
	}
	c.pendingPublish = nil
}

// flushSubs re-sends REQ for all stored subscriptions (used after WS opens).
func (c *Conn) flushSubs() {
	for _, sub := range c.subs {
		msg := "[\"REQ\",\"" + sub.ID + "\""
		for _, f := range sub.Filters {
			msg += "," + f.Serialize()
		}
		msg += "]"
		ws.Send(c.wsConn, msg)
	}
}

func eventJSON(ev *nostr.Event) string {
	buf := make([]byte, 0, 512)
	buf = append(buf, '{')
	buf = append(buf, "\"id\":\""...)
	buf = append(buf, ev.ID...)
	buf = append(buf, "\",\"pubkey\":\""...)
	buf = append(buf, ev.PubKey...)
	buf = append(buf, "\",\"created_at\":"...)
	buf = append(buf, itoa(ev.CreatedAt)...)
	buf = append(buf, ",\"kind\":"...)
	buf = append(buf, itoa(int64(ev.Kind))...)
	buf = append(buf, ",\"tags\":"...)
	buf = serializeTags(buf, ev.Tags)
	buf = append(buf, ",\"content\":\""...)
	buf = appendEscaped(buf, ev.Content)
	buf = append(buf, "\",\"sig\":\""...)
	buf = append(buf, ev.Sig...)
	buf = append(buf, '"', '}')
	return string(buf)
}

func serializeTags(buf []byte, tags nostr.Tags) []byte {
	buf = append(buf, '[')
	for i, tag := range tags {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '[')
		for j, s := range tag {
			if j > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, '"')
			buf = appendEscaped(buf, s)
			buf = append(buf, '"')
		}
		buf = append(buf, ']')
	}
	buf = append(buf, ']')
	return buf
}

func appendEscaped(buf []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		default:
			buf = append(buf, c)
		}
	}
	return buf
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
