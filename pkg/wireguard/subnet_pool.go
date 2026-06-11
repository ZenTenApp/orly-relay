package wireguard

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/netip"

	"lukechampine.com/frand"
)

// Subnet represents a /31 point-to-point subnet.
type Subnet struct {
	ServerIP netip.Addr // Even address (server side)
	ClientIP netip.Addr // Odd address (client side)
}

// --- Actor request/response types ---

type spServerIPsReq struct {
	resp chan []netip.Addr
}

type spGetSubnetReq struct {
	clientPubkeyHex string
	resp            chan *Subnet
}

type spGetSequenceReq struct {
	clientPubkeyHex string
	resp            chan int
}

type spRestoreReq struct {
	clientPubkeyHex string
	seq             uint32
}

type spMaxSeqReq struct {
	resp chan uint32
}

type spAllocCountReq struct {
	resp chan int
}

// SubnetPool manages deterministic /31 subnet generation from a seed.
// All mutable state is owned by the actor goroutine.
type SubnetPool struct {
	seed       [32]byte
	basePrefix netip.Prefix

	serverIPsCh  chan spServerIPsReq
	getSubnetCh  chan spGetSubnetReq
	getSeqCh     chan spGetSequenceReq
	restoreCh    chan spRestoreReq
	maxSeqCh     chan spMaxSeqReq
	allocCountCh chan spAllocCountReq
	stop         chan struct{}
	done         chan struct{}
}

// NewSubnetPool creates a subnet pool with a new random seed.
func NewSubnetPool(baseNetwork string) (*SubnetPool, error) {
	prefix, err := netip.ParsePrefix(baseNetwork)
	if err != nil {
		return nil, fmt.Errorf("invalid base network: %w", err)
	}

	var seed [32]byte
	frand.Read(seed[:])

	p := &SubnetPool{
		seed:         seed,
		basePrefix:   prefix,
		serverIPsCh:  make(chan spServerIPsReq),
		getSubnetCh:  make(chan spGetSubnetReq),
		getSeqCh:     make(chan spGetSequenceReq),
		restoreCh:    make(chan spRestoreReq, 16),
		maxSeqCh:     make(chan spMaxSeqReq),
		allocCountCh: make(chan spAllocCountReq),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
	}
	go p.actorLoop()
	return p, nil
}

// NewSubnetPoolWithSeed creates a subnet pool with an existing seed.
func NewSubnetPoolWithSeed(baseNetwork string, seed []byte) (*SubnetPool, error) {
	prefix, err := netip.ParsePrefix(baseNetwork)
	if err != nil {
		return nil, fmt.Errorf("invalid base network: %w", err)
	}

	if len(seed) != 32 {
		return nil, fmt.Errorf("seed must be 32 bytes, got %d", len(seed))
	}

	p := &SubnetPool{
		basePrefix:   prefix,
		serverIPsCh:  make(chan spServerIPsReq),
		getSubnetCh:  make(chan spGetSubnetReq),
		getSeqCh:     make(chan spGetSequenceReq),
		restoreCh:    make(chan spRestoreReq, 16),
		maxSeqCh:     make(chan spMaxSeqReq),
		allocCountCh: make(chan spAllocCountReq),
		stop:         make(chan struct{}),
		done:         make(chan struct{}),
	}
	copy(p.seed[:], seed)
	go p.actorLoop()
	return p, nil
}

func (p *SubnetPool) actorLoop() {
	defer close(p.done)

	var maxSeq uint32
	assigned := make(map[string]uint32)

	for {
		select {
		case <-p.stop:
			return
		case req := <-p.serverIPsCh:
			if maxSeq == 0 {
				req.resp <- nil
				continue
			}
			ips := make([]netip.Addr, maxSeq)
			for seq := uint32(0); seq < maxSeq; seq++ {
				subnet := p.deriveSubnet(seq)
				ips[seq] = subnet.ServerIP
			}
			req.resp <- ips
		case req := <-p.getSubnetCh:
			if seq, ok := assigned[req.clientPubkeyHex]; ok {
				subnet := p.deriveSubnet(seq)
				req.resp <- &subnet
			} else {
				req.resp <- nil
			}
		case req := <-p.getSeqCh:
			if seq, ok := assigned[req.clientPubkeyHex]; ok {
				req.resp <- int(seq)
			} else {
				req.resp <- -1
			}
		case req := <-p.restoreCh:
			assigned[req.clientPubkeyHex] = req.seq
			if req.seq >= maxSeq {
				maxSeq = req.seq + 1
			}
		case req := <-p.maxSeqCh:
			req.resp <- maxSeq
		case req := <-p.allocCountCh:
			req.resp <- len(assigned)
		}
	}
}

// Shutdown stops the actor goroutine.
func (p *SubnetPool) Shutdown() {
	close(p.stop)
	<-p.done
}

// Seed returns the pool's seed for persistence.
func (p *SubnetPool) Seed() []byte {
	return p.seed[:]
}

// deriveSubnet deterministically generates a /31 subnet from seed + sequence.
func (p *SubnetPool) deriveSubnet(seq uint32) Subnet {
	h := sha256.New()
	h.Write(p.seed[:])
	binary.Write(h, binary.BigEndian, seq)
	hash := h.Sum(nil)

	offset := binary.BigEndian.Uint32(hash[:4])
	bits := p.basePrefix.Bits()
	availableBits := uint32(32 - bits)
	maxOffset := uint32(1) << availableBits
	offset = (offset % (maxOffset / 2)) * 2

	baseAddr := p.basePrefix.Addr()
	baseBytes := baseAddr.As4()
	baseVal := uint32(baseBytes[0])<<24 | uint32(baseBytes[1])<<16 |
		uint32(baseBytes[2])<<8 | uint32(baseBytes[3])

	serverVal := baseVal + offset
	clientVal := serverVal + 1

	serverBytes := [4]byte{
		byte(serverVal >> 24), byte(serverVal >> 16),
		byte(serverVal >> 8), byte(serverVal),
	}
	clientBytes := [4]byte{
		byte(clientVal >> 24), byte(clientVal >> 16),
		byte(clientVal >> 8), byte(clientVal),
	}

	return Subnet{
		ServerIP: netip.AddrFrom4(serverBytes),
		ClientIP: netip.AddrFrom4(clientBytes),
	}
}

// ServerIPs returns server-side IPs for sequences 0 to maxSeq.
func (p *SubnetPool) ServerIPs() []netip.Addr {
	req := spServerIPsReq{resp: make(chan []netip.Addr, 1)}
	p.serverIPsCh <- req
	return <-req.resp
}

// GetSubnet returns the subnet for a client, or nil if not assigned.
func (p *SubnetPool) GetSubnet(clientPubkeyHex string) *Subnet {
	req := spGetSubnetReq{clientPubkeyHex: clientPubkeyHex, resp: make(chan *Subnet, 1)}
	p.getSubnetCh <- req
	return <-req.resp
}

// GetSequence returns the sequence number for a client, or -1 if not assigned.
func (p *SubnetPool) GetSequence(clientPubkeyHex string) int {
	req := spGetSequenceReq{clientPubkeyHex: clientPubkeyHex, resp: make(chan int, 1)}
	p.getSeqCh <- req
	return <-req.resp
}

// RestoreAllocation restores a previously saved allocation.
func (p *SubnetPool) RestoreAllocation(clientPubkeyHex string, seq uint32) {
	p.restoreCh <- spRestoreReq{clientPubkeyHex: clientPubkeyHex, seq: seq}
}

// MaxSequence returns the current max sequence number.
func (p *SubnetPool) MaxSequence() uint32 {
	req := spMaxSeqReq{resp: make(chan uint32, 1)}
	p.maxSeqCh <- req
	return <-req.resp
}

// AllocatedCount returns the number of allocated subnets.
func (p *SubnetPool) AllocatedCount() int {
	req := spAllocCountReq{resp: make(chan int, 1)}
	p.allocCountCh <- req
	return <-req.resp
}

// SubnetForSequence returns the subnet for a given sequence number.
func (p *SubnetPool) SubnetForSequence(seq uint32) Subnet {
	return p.deriveSubnet(seq)
}
