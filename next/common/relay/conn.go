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
}

// Dial opens a connection to a relay.
// Call OnReady to receive the open/fail notification, then Start to begin processing.
func Dial(url string) *Conn {
	c := &Conn{
		URL:   url,
		state: StateConnecting,
		subs:  make(map[string]*Sub),
	}

	c.wsConn = ws.Dial(
		url,
		func(connID int, data string) {
			c.handleMessage(data)
		},
		func(connID int) {
			c.state = StateOpen
			if c.onReady != nil {
				c.onReady(true)
				c.onReady = nil
			}
		},
		func(connID int, code int, reason string) {
			c.state = StateClosed
			if c.onReady != nil {
				c.onReady(false)
				c.onReady = nil
			}
		},
		func(connID int) {
			c.state = StateClosed
			if c.onReady != nil {
				c.onReady(false)
				c.onReady = nil
			}
		},
	)

	return c
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
func (c *Conn) Subscribe(id string, filters []*nostr.Filter) *Sub {
	sub := &Sub{
		ID:      id,
		Filters: filters,
		conn:    c,
	}
	c.subs[id] = sub

	msg := "[\"REQ\",\"" + id + "\""
	for _, f := range filters {
		msg += "," + f.Serialize()
	}
	msg += "]"
	ws.Send(c.wsConn, msg)

	return sub
}

// Publish sends an EVENT message.
func (c *Conn) Publish(ev *nostr.Event) {
	msg := "[\"EVENT\"," + eventJSON(ev) + "]"
	ws.Send(c.wsConn, msg)
}

// CloseSubscription sends a CLOSE message.
func (c *Conn) CloseSubscription(id string) {
	delete(c.subs, id)
	msg := "[\"CLOSE\",\"" + id + "\"]"
	ws.Send(c.wsConn, msg)
}

// Close closes the connection.
func (c *Conn) Close() {
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
