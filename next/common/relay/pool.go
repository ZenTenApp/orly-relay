package relay

import "common/nostr"

// Pool manages connections to multiple relays.
type Pool struct {
	conns   map[string]*Conn
	maxSize int
}

// NewPool creates a relay pool.
func NewPool(maxSize int) *Pool {
	return &Pool{
		conns:   make(map[string]*Conn),
		maxSize: maxSize,
	}
}

// Connect gets or creates a connection to a relay.
func (p *Pool) Connect(url string) *Conn {
	if c, ok := p.conns[url]; ok && !c.closed {
		return c
	}
	// Evict if at capacity.
	if len(p.conns) >= p.maxSize {
		p.evictOne()
	}
	c := Dial(url)
	p.conns[url] = c
	return c
}

// Get returns an existing connection, or nil.
func (p *Pool) Get(url string) *Conn {
	c, ok := p.conns[url]
	if !ok || c.closed {
		return nil
	}
	return c
}

// Disconnect closes and removes a connection.
func (p *Pool) Disconnect(url string) {
	if c, ok := p.conns[url]; ok {
		c.Close()
		delete(p.conns, url)
	}
}

// Subscribe sends a subscription to specific relays.
func (p *Pool) Subscribe(relays []string, id string, filters []*nostr.Filter) []*Sub {
	var subs []*Sub
	for _, url := range relays {
		c := p.Connect(url)
		if c.WaitOpen() {
			sub := c.Subscribe(id, filters)
			subs = append(subs, sub)
		}
	}
	return subs
}

// Publish sends an event to specific relays.
func (p *Pool) Publish(relays []string, ev *nostr.Event) {
	for _, url := range relays {
		c := p.Connect(url)
		if c.WaitOpen() {
			c.Publish(ev)
		}
	}
}

// CloseAll closes all connections.
func (p *Pool) CloseAll() {
	for url, c := range p.conns {
		c.Close()
		delete(p.conns, url)
	}
}

// URLs returns all connected relay URLs.
func (p *Pool) URLs() []string {
	var out []string
	for url, c := range p.conns {
		if !c.closed {
			out = append(out, url)
		}
	}
	return out
}

func (p *Pool) evictOne() {
	// Evict first found closed connection, or oldest.
	for url, c := range p.conns {
		if c.closed {
			delete(p.conns, url)
			return
		}
	}
	// All open — evict arbitrary one.
	for url, c := range p.conns {
		c.Close()
		delete(p.conns, url)
		return
	}
}
