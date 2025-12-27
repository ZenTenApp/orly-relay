package wireguard

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/netip"
	"sync"

	"lukechampine.com/frand"
)

// Subnet represents a /31 point-to-point subnet.
type Subnet struct {
	ServerIP netip.Addr // Even address (server side)
	ClientIP netip.Addr // Odd address (client side)
}

// SubnetPool manages deterministic /31 subnet generation from a seed.
// Given the same seed and sequence number, the same subnet is always generated.
type SubnetPool struct {
	seed       [32]byte       // Random seed for deterministic generation
	basePrefix netip.Prefix   // e.g., 10.0.0.0/8
	maxSeq     uint32         // Current highest sequence number
	assigned   map[string]uint32 // Client pubkey hex -> sequence number
	mu         sync.RWMutex
}

// NewSubnetPool creates a subnet pool with a new random seed.
func NewSubnetPool(baseNetwork string) (*SubnetPool, error) {
	prefix, err := netip.ParsePrefix(baseNetwork)
	if err != nil {
		return nil, fmt.Errorf("invalid base network: %w", err)
	}

	var seed [32]byte
	frand.Read(seed[:])

	return &SubnetPool{
		seed:       seed,
		basePrefix: prefix,
		maxSeq:     0,
		assigned:   make(map[string]uint32),
	}, nil
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

	pool := &SubnetPool{
		basePrefix: prefix,
		maxSeq:     0,
		assigned:   make(map[string]uint32),
	}
	copy(pool.seed[:], seed)

	return pool, nil
}

// Seed returns the pool's seed for persistence.
func (p *SubnetPool) Seed() []byte {
	return p.seed[:]
}

// deriveSubnet deterministically generates a /31 subnet from seed + sequence.
func (p *SubnetPool) deriveSubnet(seq uint32) Subnet {
	// Hash seed + sequence to get deterministic randomness
	h := sha256.New()
	h.Write(p.seed[:])
	binary.Write(h, binary.BigEndian, seq)
	hash := h.Sum(nil)

	// Use first 4 bytes as offset within the prefix
	offset := binary.BigEndian.Uint32(hash[:4])

	// Calculate available address space
	bits := p.basePrefix.Bits()
	availableBits := uint32(32 - bits)
	maxOffset := uint32(1) << availableBits

	// Make offset even (for /31 alignment) and within range
	offset = (offset % (maxOffset / 2)) * 2

	// Calculate server IP (even) and client IP (odd)
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

// ServerIPs returns server-side IPs for sequences 0 to maxSeq (for netstack).
func (p *SubnetPool) ServerIPs() []netip.Addr {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.maxSeq == 0 {
		return nil
	}

	ips := make([]netip.Addr, p.maxSeq)
	for seq := uint32(0); seq < p.maxSeq; seq++ {
		subnet := p.deriveSubnet(seq)
		ips[seq] = subnet.ServerIP
	}
	return ips
}

// GetSubnet returns the subnet for a client, or nil if not assigned.
func (p *SubnetPool) GetSubnet(clientPubkeyHex string) *Subnet {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if seq, ok := p.assigned[clientPubkeyHex]; ok {
		subnet := p.deriveSubnet(seq)
		return &subnet
	}
	return nil
}

// GetSequence returns the sequence number for a client, or -1 if not assigned.
func (p *SubnetPool) GetSequence(clientPubkeyHex string) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if seq, ok := p.assigned[clientPubkeyHex]; ok {
		return int(seq)
	}
	return -1
}

// RestoreAllocation restores a previously saved allocation.
func (p *SubnetPool) RestoreAllocation(clientPubkeyHex string, seq uint32) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.assigned[clientPubkeyHex] = seq
	if seq >= p.maxSeq {
		p.maxSeq = seq + 1
	}
}

// MaxSequence returns the current max sequence number.
func (p *SubnetPool) MaxSequence() uint32 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxSeq
}

// AllocatedCount returns the number of allocated subnets.
func (p *SubnetPool) AllocatedCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.assigned)
}

// SubnetForSequence returns the subnet for a given sequence number.
func (p *SubnetPool) SubnetForSequence(seq uint32) Subnet {
	return p.deriveSubnet(seq)
}
