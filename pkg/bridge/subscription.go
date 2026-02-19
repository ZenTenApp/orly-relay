package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Subscription represents a user's bridge subscription.
type Subscription struct {
	// PubkeyHex is the subscriber's 32-byte pubkey in hex.
	PubkeyHex string `json:"pubkey"`
	// ExpiresAt is the subscription expiration time.
	ExpiresAt time.Time `json:"expires_at"`
	// CreatedAt is when the subscription was created.
	CreatedAt time.Time `json:"created_at"`
	// InvoiceHash is the payment hash of the last paid invoice.
	InvoiceHash string `json:"invoice_hash,omitempty"`
}

// IsActive returns true if the subscription has not expired.
func (s *Subscription) IsActive() bool {
	return time.Now().Before(s.ExpiresAt)
}

// SubscriptionStore persists and queries subscriptions.
type SubscriptionStore interface {
	Save(sub *Subscription) error
	Get(pubkeyHex string) (*Subscription, error)
	List() ([]*Subscription, error)
	Delete(pubkeyHex string) error
}

// FileSubscriptionStore persists subscriptions as a JSON file.
type FileSubscriptionStore struct {
	path string
	mu   sync.RWMutex
	subs map[string]*Subscription
}

// NewFileSubscriptionStore creates a subscription store backed by a JSON file.
func NewFileSubscriptionStore(dataDir string) (*FileSubscriptionStore, error) {
	path := filepath.Join(dataDir, "subscriptions.json")
	store := &FileSubscriptionStore{
		path: path,
		subs: make(map[string]*Subscription),
	}

	// Load existing file if present
	data, err := os.ReadFile(path)
	if err == nil {
		var subs []*Subscription
		if err := json.Unmarshal(data, &subs); err == nil {
			for _, s := range subs {
				store.subs[s.PubkeyHex] = s
			}
		}
	}

	return store, nil
}

func (s *FileSubscriptionStore) Save(sub *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subs[sub.PubkeyHex] = sub
	return s.flush()
}

func (s *FileSubscriptionStore) Get(pubkeyHex string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subs[pubkeyHex]
	if !ok {
		return nil, fmt.Errorf("subscription not found for %s", pubkeyHex)
	}
	return sub, nil
}

func (s *FileSubscriptionStore) List() ([]*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var subs []*Subscription
	for _, sub := range s.subs {
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *FileSubscriptionStore) Delete(pubkeyHex string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.subs, pubkeyHex)
	return s.flush()
}

func (s *FileSubscriptionStore) flush() error {
	var subs []*Subscription
	for _, sub := range s.subs {
		subs = append(subs, sub)
	}

	data, err := json.MarshalIndent(subs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal subscriptions: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create subscription dir: %w", err)
	}

	return os.WriteFile(s.path, data, 0600)
}

// MemorySubscriptionStore is an in-memory subscription store for testing.
type MemorySubscriptionStore struct {
	mu   sync.RWMutex
	subs map[string]*Subscription
}

// NewMemorySubscriptionStore creates a new in-memory subscription store.
func NewMemorySubscriptionStore() *MemorySubscriptionStore {
	return &MemorySubscriptionStore{
		subs: make(map[string]*Subscription),
	}
}

func (s *MemorySubscriptionStore) Save(sub *Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[sub.PubkeyHex] = sub
	return nil
}

func (s *MemorySubscriptionStore) Get(pubkeyHex string) (*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sub, ok := s.subs[pubkeyHex]
	if !ok {
		return nil, fmt.Errorf("subscription not found for %s", pubkeyHex)
	}
	return sub, nil
}

func (s *MemorySubscriptionStore) List() ([]*Subscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var subs []*Subscription
	for _, sub := range s.subs {
		subs = append(subs, sub)
	}
	return subs, nil
}

func (s *MemorySubscriptionStore) Delete(pubkeyHex string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, pubkeyHex)
	return nil
}
