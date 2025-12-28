package keyset

// Store is the interface for persisting keysets.
// Implement this interface for your database backend.
type Store interface {
	// SaveKeyset persists a keyset.
	SaveKeyset(k *Keyset) error

	// LoadKeyset loads a keyset by ID.
	LoadKeyset(id string) (*Keyset, error)

	// ListActiveKeysets returns all keysets that can be used for signing.
	ListActiveKeysets() ([]*Keyset, error)

	// ListVerificationKeysets returns all keysets that can be used for verification.
	ListVerificationKeysets() ([]*Keyset, error)

	// DeleteKeyset removes a keyset from storage.
	DeleteKeyset(id string) error
}

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	keysets map[string]*Keyset
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		keysets: make(map[string]*Keyset),
	}
}

// SaveKeyset saves a keyset to memory.
func (s *MemoryStore) SaveKeyset(k *Keyset) error {
	s.keysets[k.ID] = k
	return nil
}

// LoadKeyset loads a keyset by ID.
func (s *MemoryStore) LoadKeyset(id string) (*Keyset, error) {
	if k, ok := s.keysets[id]; ok {
		return k, nil
	}
	return nil, nil
}

// ListActiveKeysets returns all active keysets.
func (s *MemoryStore) ListActiveKeysets() ([]*Keyset, error) {
	result := make([]*Keyset, 0)
	for _, k := range s.keysets {
		if k.IsActiveForSigning() {
			result = append(result, k)
		}
	}
	return result, nil
}

// ListVerificationKeysets returns all keysets valid for verification.
func (s *MemoryStore) ListVerificationKeysets() ([]*Keyset, error) {
	result := make([]*Keyset, 0)
	for _, k := range s.keysets {
		if k.IsValidForVerification() {
			result = append(result, k)
		}
	}
	return result, nil
}

// DeleteKeyset removes a keyset.
func (s *MemoryStore) DeleteKeyset(id string) error {
	delete(s.keysets, id)
	return nil
}
