package relay

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
	if c, ok := p.conns[url]; ok && c.IsOpen() {
		return c
	}
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
	if !ok || !c.IsOpen() {
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
		if c.IsOpen() {
			out = append(out, url)
		}
	}
	return out
}

func (p *Pool) evictOne() {
	for url, c := range p.conns {
		if c.state == StateClosed {
			delete(p.conns, url)
			return
		}
	}
	for url, c := range p.conns {
		c.Close()
		delete(p.conns, url)
		return
	}
}
