package keyset

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

// keysetData is the JSON-serializable form of a Keyset.
type keysetData struct {
	ID         string `json:"id"`
	PrivateKey string `json:"private_key"` // hex-encoded
	CreatedAt  int64  `json:"created_at"`
	ActiveAt   int64  `json:"active_at"`
	ExpiresAt  int64  `json:"expires_at"`
	VerifyEnd  int64  `json:"verify_end"`
	Active     bool   `json:"active"`
}

// fileStoreData is the top-level JSON structure.
type fileStoreData struct {
	Version  int          `json:"version"`
	Keysets  []keysetData `json:"keysets"`
	UpdatedAt int64       `json:"updated_at"`
}

// FileStore persists keysets to a JSON file.
type FileStore struct {
	path    string
	mu      sync.RWMutex
	keysets map[string]*Keyset
}

// NewFileStore creates a new file-based keyset store.
// The directory will be created if it doesn't exist.
func NewFileStore(path string) (*FileStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("keyset: failed to create directory: %w", err)
	}

	store := &FileStore{
		path:    path,
		keysets: make(map[string]*Keyset),
	}

	// Load existing keysets if file exists
	if err := store.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("keyset: failed to load keysets: %w", err)
	}

	return store, nil
}

// load reads keysets from the file.
func (s *FileStore) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	var fileData fileStoreData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return fmt.Errorf("keyset: failed to parse file: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, kd := range fileData.Keysets {
		keyset, err := s.fromData(kd)
		if err != nil {
			// Log but continue - don't fail on single corrupt keyset
			continue
		}
		s.keysets[keyset.ID] = keyset
	}

	return nil
}

// save writes all keysets to the file.
func (s *FileStore) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keysets := make([]keysetData, 0, len(s.keysets))
	for _, k := range s.keysets {
		keysets = append(keysets, s.toData(k))
	}

	fileData := fileStoreData{
		Version:   1,
		Keysets:   keysets,
		UpdatedAt: time.Now().Unix(),
	}

	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return fmt.Errorf("keyset: failed to marshal: %w", err)
	}

	// Write atomically using temp file
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("keyset: failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("keyset: failed to rename temp file: %w", err)
	}

	return nil
}

// toData converts a Keyset to its JSON form.
func (s *FileStore) toData(k *Keyset) keysetData {
	return keysetData{
		ID:         k.ID,
		PrivateKey: hex.EncodeToString(k.SerializePrivateKey()),
		CreatedAt:  k.CreatedAt.Unix(),
		ActiveAt:   k.ActiveAt.Unix(),
		ExpiresAt:  k.ExpiresAt.Unix(),
		VerifyEnd:  k.VerifyEnd.Unix(),
		Active:     k.Active,
	}
}

// fromData reconstructs a Keyset from its JSON form.
func (s *FileStore) fromData(kd keysetData) (*Keyset, error) {
	privKeyBytes, err := hex.DecodeString(kd.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("keyset: invalid private key hex: %w", err)
	}

	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("keyset: private key must be 32 bytes")
	}

	privKey := secp256k1.PrivKeyFromBytes(privKeyBytes)
	pubKey := privKey.PubKey()

	return &Keyset{
		ID:         kd.ID,
		PrivateKey: privKey,
		PublicKey:  pubKey,
		CreatedAt:  time.Unix(kd.CreatedAt, 0),
		ActiveAt:   time.Unix(kd.ActiveAt, 0),
		ExpiresAt:  time.Unix(kd.ExpiresAt, 0),
		VerifyEnd:  time.Unix(kd.VerifyEnd, 0),
		Active:     kd.Active,
	}, nil
}

// SaveKeyset persists a keyset.
func (s *FileStore) SaveKeyset(k *Keyset) error {
	s.mu.Lock()
	s.keysets[k.ID] = k
	s.mu.Unlock()

	return s.save()
}

// LoadKeyset loads a keyset by ID.
func (s *FileStore) LoadKeyset(id string) (*Keyset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if k, ok := s.keysets[id]; ok {
		return k, nil
	}
	return nil, nil
}

// ListActiveKeysets returns all keysets that can be used for signing.
func (s *FileStore) ListActiveKeysets() ([]*Keyset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Keyset, 0)
	for _, k := range s.keysets {
		if k.IsActiveForSigning() {
			result = append(result, k)
		}
	}
	return result, nil
}

// ListVerificationKeysets returns all keysets that can be used for verification.
func (s *FileStore) ListVerificationKeysets() ([]*Keyset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Keyset, 0)
	for _, k := range s.keysets {
		if k.IsValidForVerification() {
			result = append(result, k)
		}
	}
	return result, nil
}

// DeleteKeyset removes a keyset from storage.
func (s *FileStore) DeleteKeyset(id string) error {
	s.mu.Lock()
	delete(s.keysets, id)
	s.mu.Unlock()

	return s.save()
}
