// Package keyset manages Cashu mint keysets for blind signature tokens.
// Keysets rotate periodically to limit key exposure and provide forward secrecy.
package keyset

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// DefaultActiveWindow is how long a keyset is valid for issuing new tokens.
const DefaultActiveWindow = 7 * 24 * time.Hour // 1 week

// DefaultVerifyWindow is how long a keyset remains valid for verification.
const DefaultVerifyWindow = 21 * 24 * time.Hour // 3 weeks

// Keyset represents a signing keyset with lifecycle management.
type Keyset struct {
	ID         string                 // 14-char hex ID (7 bytes)
	PrivateKey *secp256k1.PrivateKey  // Signing key
	PublicKey  *secp256k1.PublicKey   // Verification key
	CreatedAt  time.Time              // When keyset was created
	ActiveAt   time.Time              // When keyset becomes active for signing
	ExpiresAt  time.Time              // When keyset can no longer sign (but can still verify)
	VerifyEnd  time.Time              // When keyset can no longer verify
	Active     bool                   // Whether keyset is currently active for signing
}

// New creates a new keyset with generated keys.
func New() (*Keyset, error) {
	return NewWithTTL(DefaultActiveWindow, DefaultVerifyWindow)
}

// NewWithTTL creates a new keyset with custom lifetimes.
func NewWithTTL(activeTTL, verifyTTL time.Duration) (*Keyset, error) {
	// Generate random private key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("keyset: failed to generate key: %w", err)
	}

	privKey := secp256k1.PrivKeyFromBytes(keyBytes)
	pubKey := privKey.PubKey()

	now := time.Now()
	k := &Keyset{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		CreatedAt:  now,
		ActiveAt:   now,
		ExpiresAt:  now.Add(activeTTL),
		VerifyEnd:  now.Add(verifyTTL),
		Active:     true,
	}

	// Calculate ID from public key
	k.ID = k.calculateID()

	return k, nil
}

// NewFromPrivateKey creates a keyset from an existing private key.
func NewFromPrivateKey(privKeyBytes []byte, createdAt time.Time, activeTTL, verifyTTL time.Duration) (*Keyset, error) {
	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("keyset: private key must be 32 bytes")
	}

	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	pubKey := privKey.PubKey()

	k := &Keyset{
		PrivateKey: privKey,
		PublicKey:  pubKey,
		CreatedAt:  createdAt,
		ActiveAt:   createdAt,
		ExpiresAt:  createdAt.Add(activeTTL),
		VerifyEnd:  createdAt.Add(verifyTTL),
		Active:     true,
	}

	k.ID = k.calculateID()

	return k, nil
}

// calculateID computes the keyset ID from the public key.
// ID = hex(SHA256(compressed_pubkey)[0:7])
func (k *Keyset) calculateID() string {
	compressed := k.PublicKey.SerializeCompressed()
	hash := sha256.Sum256(compressed)
	return hex.EncodeToString(hash[:7])
}

// IsActiveForSigning returns true if keyset can be used to sign new tokens.
func (k *Keyset) IsActiveForSigning() bool {
	now := time.Now()
	return k.Active && now.After(k.ActiveAt) && now.Before(k.ExpiresAt)
}

// IsValidForVerification returns true if keyset can be used to verify tokens.
func (k *Keyset) IsValidForVerification() bool {
	now := time.Now()
	return now.After(k.ActiveAt) && now.Before(k.VerifyEnd)
}

// Deactivate marks the keyset as no longer active for signing.
func (k *Keyset) Deactivate() {
	k.Active = false
}

// SerializePrivateKey returns the private key as bytes for storage.
func (k *Keyset) SerializePrivateKey() []byte {
	return k.PrivateKey.Serialize()
}

// SerializePublicKey returns the compressed public key.
func (k *Keyset) SerializePublicKey() []byte {
	return k.PublicKey.SerializeCompressed()
}

// KeysetInfo is a public view of a keyset (without private key).
type KeysetInfo struct {
	ID        string    `json:"id"`
	PublicKey string    `json:"pubkey"`
	Active    bool      `json:"active"`
	CreatedAt int64     `json:"created_at"`
	ExpiresAt int64     `json:"expires_at"`
	VerifyEnd int64     `json:"verify_end"`
}

// Info returns public information about the keyset.
func (k *Keyset) Info() KeysetInfo {
	return KeysetInfo{
		ID:        k.ID,
		PublicKey: hex.EncodeToString(k.SerializePublicKey()),
		Active:    k.IsActiveForSigning(),
		CreatedAt: k.CreatedAt.Unix(),
		ExpiresAt: k.ExpiresAt.Unix(),
		VerifyEnd: k.VerifyEnd.Unix(),
	}
}

// Manager handles keyset lifecycle including rotation.
type Manager struct {
	store        Store
	activeTTL    time.Duration
	verifyTTL    time.Duration

	mu           sync.RWMutex
	current      *Keyset   // Current active keyset for signing
	verification []*Keyset // All keysets valid for verification (including current)
}

// NewManager creates a keyset manager.
func NewManager(store Store, activeTTL, verifyTTL time.Duration) *Manager {
	return &Manager{
		store:        store,
		activeTTL:    activeTTL,
		verifyTTL:    verifyTTL,
		verification: make([]*Keyset, 0),
	}
}

// Init initializes the manager by loading existing keysets or creating a new one.
func (m *Manager) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load all valid keysets from store
	keysets, err := m.store.ListVerificationKeysets()
	if err != nil {
		return fmt.Errorf("manager: failed to load keysets: %w", err)
	}

	// Find current active keyset
	var active *Keyset
	for _, k := range keysets {
		if k.IsActiveForSigning() {
			if active == nil || k.CreatedAt.After(active.CreatedAt) {
				active = k
			}
		}
		if k.IsValidForVerification() {
			m.verification = append(m.verification, k)
		}
	}

	// If no active keyset, create one
	if active == nil {
		newKeyset, err := NewWithTTL(m.activeTTL, m.verifyTTL)
		if err != nil {
			return fmt.Errorf("manager: failed to create initial keyset: %w", err)
		}
		if err := m.store.SaveKeyset(newKeyset); err != nil {
			return fmt.Errorf("manager: failed to save initial keyset: %w", err)
		}
		active = newKeyset
		m.verification = append(m.verification, newKeyset)
	}

	m.current = active
	return nil
}

// GetSigningKeyset returns the current active keyset for signing.
func (m *Manager) GetSigningKeyset() *Keyset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// GetVerificationKeysets returns all keysets valid for verification.
func (m *Manager) GetVerificationKeysets() []*Keyset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Keyset, 0, len(m.verification))
	for _, k := range m.verification {
		if k.IsValidForVerification() {
			result = append(result, k)
		}
	}
	return result
}

// FindByID returns the keyset with the given ID, if it's valid for verification.
func (m *Manager) FindByID(id string) *Keyset {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, k := range m.verification {
		if k.ID == id && k.IsValidForVerification() {
			return k
		}
	}
	return nil
}

// RotateIfNeeded checks if rotation is needed and performs it.
// Returns true if a new keyset was created.
func (m *Manager) RotateIfNeeded() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if current keyset is still active
	if m.current != nil && m.current.IsActiveForSigning() {
		return false, nil
	}

	// Create new keyset
	newKeyset, err := NewWithTTL(m.activeTTL, m.verifyTTL)
	if err != nil {
		return false, fmt.Errorf("manager: failed to create new keyset: %w", err)
	}

	// Deactivate old keyset
	if m.current != nil {
		m.current.Deactivate()
	}

	// Save new keyset
	if err := m.store.SaveKeyset(newKeyset); err != nil {
		return false, fmt.Errorf("manager: failed to save new keyset: %w", err)
	}

	// Update manager state
	m.current = newKeyset
	m.verification = append(m.verification, newKeyset)

	// Prune expired verification keysets
	m.pruneExpired()

	return true, nil
}

// pruneExpired removes keysets that are no longer valid for verification.
// Must be called with lock held.
func (m *Manager) pruneExpired() {
	valid := make([]*Keyset, 0, len(m.verification))
	for _, k := range m.verification {
		if k.IsValidForVerification() {
			valid = append(valid, k)
		}
	}
	m.verification = valid
}

// ListKeysetInfo returns public info for all verification keysets.
func (m *Manager) ListKeysetInfo() []KeysetInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]KeysetInfo, 0, len(m.verification))
	for _, k := range m.verification {
		if k.IsValidForVerification() {
			result = append(result, k.Info())
		}
	}
	return result
}

// StartRotationTicker starts a goroutine that rotates keysets periodically.
// Returns a channel that receives true on each rotation.
func (m *Manager) StartRotationTicker(interval time.Duration) (rotated <-chan bool, stop func()) {
	ticker := time.NewTicker(interval)
	ch := make(chan bool, 1)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				rotated, err := m.RotateIfNeeded()
				if err != nil {
					// Log error but continue
					continue
				}
				if rotated {
					select {
					case ch <- true:
					default:
					}
				}
			case <-done:
				ticker.Stop()
				close(ch)
				return
			}
		}
	}()

	return ch, func() { close(done) }
}
