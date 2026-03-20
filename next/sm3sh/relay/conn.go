package relay

import (
	"smesh3/jsbridge/ws"
	"smesh3/nostr"
)

// State constants for connection readiness.
const (
	StateConnecting = 0
	StateOpen       = 1
	StateClosing    = 2
	StateClosed     = 3
)

// Conn is a single relay WebSocket connection.
type Conn struct {
	URL     string
	wsConn  ws.Conn
	msgCh   chan string
	openCh  chan struct{}
	closeCh chan struct{}
	closed  bool
	subs    map[string]*Sub
	onEvent func(string, *nostr.Event)
	onEOSE  func(string)
	onOK    func(string, bool, string)
	onAuth  func(string)
}

// Dial opens a connection to a relay.
func Dial(url string) *Conn {
	c := &Conn{
		URL:     url,
		msgCh:   make(chan string, 64),
		openCh:  make(chan struct{}, 1),
		closeCh: make(chan struct{}, 1),
		subs:    make(map[string]*Sub),
	}

	c.wsConn = ws.Dial(
		url,
		func(connID int, data string) {
			select {
			case c.msgCh <- data:
			default:
				// Drop message if buffer full.
			}
		},
		func(connID int) {
			select {
			case c.openCh <- struct{}{}:
			default:
			}
		},
		func(connID int, code int, reason string) {
			c.closed = true
			select {
			case c.closeCh <- struct{}{}:
			default:
			}
		},
		func(connID int) {
			c.closed = true
			select {
			case c.closeCh <- struct{}{}:
			default:
			}
		},
	)

	// Start read loop.
	go c.readLoop()

	return c
}

// WaitOpen blocks until the connection is open or closed.
// Returns true if connected.
func (c *Conn) WaitOpen() bool {
	select {
	case <-c.openCh:
		return true
	case <-c.closeCh:
		return false
	}
}

// readLoop processes incoming messages.
func (c *Conn) readLoop() {
	for {
		select {
		case msg := <-c.msgCh:
			c.handleMessage(msg)
		case <-c.closeCh:
			return
		}
	}
}

func (c *Conn) handleMessage(msg string) {
	label, subID, payload := nostr.ParseRelayMessage(msg)

	switch label {
	case "EVENT":
		ev := nostr.ParseEvent(payload)
		if ev == nil {
			return
		}
		// Dispatch to subscription handler.
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
		// Log notices but don't crash.
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

	// Build REQ message: ["REQ","subId",filter1,filter2,...]
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
	c.closed = true
	ws.Close(c.wsConn)
}

// OnEvent sets a global event handler (all subscriptions).
func (c *Conn) OnEvent(fn func(string, *nostr.Event)) {
	c.onEvent = fn
}

// OnEOSE sets a global EOSE handler.
func (c *Conn) OnEOSE(fn func(string)) {
	c.onEOSE = fn
}

// OnOK sets a handler for OK responses.
func (c *Conn) OnOK(fn func(string, bool, string)) {
	c.onOK = fn
}

// OnAuth sets a handler for AUTH challenges.
func (c *Conn) OnAuth(fn func(string)) {
	c.onAuth = fn
}

// eventJSON serializes an Event to JSON object string.
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
