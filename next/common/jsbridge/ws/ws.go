package ws

// Conn is an opaque handle to a WebSocket connection.
type Conn int

// Dial opens a WebSocket connection. Callbacks are JS callback IDs
// from dom.RegisterCallback. Returns connection handle.
func Dial(url string, onMessage func(int, string), onOpen func(int), onClose func(int, int, string), onError func(int)) Conn {
	panic("jsbridge")
}

// Send sends a string message. Returns true if sent.
func Send(conn Conn, msg string) bool { panic("jsbridge") }

// Close closes the connection.
func Close(conn Conn) { panic("jsbridge") }

// ReadyState returns the WebSocket readyState.
// 0=CONNECTING, 1=OPEN, 2=CLOSING, 3=CLOSED, -1=invalid
func ReadyState(conn Conn) int { panic("jsbridge") }
