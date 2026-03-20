package relay

import "common/nostr"

// Sub is an active subscription on a relay connection.
type Sub struct {
	ID      string
	Filters []*nostr.Filter
	OnEvent func(*nostr.Event)
	OnEOSE  func()
	conn    *Conn
	gotEOSE bool
}

// GotEOSE returns whether EOSE has been received.
func (s *Sub) GotEOSE() bool {
	return s.gotEOSE
}

// Close unsubscribes.
func (s *Sub) Close() {
	if s.conn != nil {
		s.conn.CloseSubscription(s.ID)
	}
}
