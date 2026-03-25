package bc

// BC is an opaque handle to a BroadcastChannel.
type BC int

// Open creates a BroadcastChannel with the given name.
// onMessage is called with each received string message.
func Open(name string, onMessage func(string)) BC { panic("jsbridge") }

// Send posts a string message to the channel.
func Send(bc BC, msg string) { panic("jsbridge") }

// Close closes the channel.
func Close(bc BC) { panic("jsbridge") }
