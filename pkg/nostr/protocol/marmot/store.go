package marmot

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// GroupStore persists MLS group state so conversations survive restarts.
type GroupStore interface {
	SaveGroup(groupID, state []byte) error
	LoadGroup(groupID []byte) ([]byte, error)
	ListGroups() ([][]byte, error)
	DeleteGroup(groupID []byte) error
	// SaveKeyPackage persists the MLS key package so welcomes referencing it
	// remain valid across restarts. Without this, every restart generates a
	// fresh key package and all pending welcomes become undecryptable.
	SaveKeyPackage(data []byte) error
	// LoadKeyPackage loads the persisted key package. Returns os.ErrNotExist
	// if no key package was saved.
	LoadKeyPackage() ([]byte, error)
}

// FileGroupStore implements GroupStore using one file per group in a directory.
type FileGroupStore struct {
	dir string
	mu  sync.Mutex
}

// NewFileGroupStore creates a file-backed group store. The directory is created
// if it does not exist.
func NewFileGroupStore(dir string) (*FileGroupStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &FileGroupStore{dir: dir}, nil
}

func (s *FileGroupStore) path(groupID []byte) string {
	return filepath.Join(s.dir, hex.EncodeToString(groupID)+".json")
}

func (s *FileGroupStore) SaveGroup(groupID, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.path(groupID), state, 0600)
}

func (s *FileGroupStore) LoadGroup(groupID []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.ReadFile(s.path(groupID))
}

func (s *FileGroupStore) ListGroups() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var ids [][]byte
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		base := name[:len(name)-5] // strip .json
		id, err := hex.DecodeString(base)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *FileGroupStore) DeleteGroup(groupID []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(s.path(groupID))
}

func (s *FileGroupStore) kppPath() string {
	return filepath.Join(s.dir, "_keypackage.dat")
}

func (s *FileGroupStore) SaveKeyPackage(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.WriteFile(s.kppPath(), data, 0600)
}

func (s *FileGroupStore) LoadKeyPackage() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.ReadFile(s.kppPath())
}

// MemoryGroupStore implements GroupStore in memory. Useful for testing.
type MemoryGroupStore struct {
	mu     sync.Mutex
	groups map[string][]byte
}

func NewMemoryGroupStore() *MemoryGroupStore {
	return &MemoryGroupStore{groups: make(map[string][]byte)}
}

func (s *MemoryGroupStore) SaveGroup(groupID, state []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(state))
	copy(cp, state)
	s.groups[string(groupID)] = cp
	return nil
}

func (s *MemoryGroupStore) LoadGroup(groupID []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.groups[string(groupID)]
	if !ok {
		return nil, os.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

func (s *MemoryGroupStore) ListGroups() ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([][]byte, 0, len(s.groups))
	for k := range s.groups {
		ids = append(ids, []byte(k))
	}
	return ids, nil
}

func (s *MemoryGroupStore) DeleteGroup(groupID []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.groups, string(groupID))
	return nil
}

func (s *MemoryGroupStore) SaveKeyPackage(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	s.groups["_kpp"] = cp
	return nil
}

func (s *MemoryGroupStore) LoadKeyPackage() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.groups["_kpp"]
	if !ok {
		return nil, os.ErrNotExist
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// groupStateSerialized is the JSON structure for persisting group state.
type groupStateSerialized struct {
	GroupID      []byte `json:"group_id"`
	NostrGroupID []byte `json:"nostr_group_id,omitempty"`
	PeerPub      []byte `json:"peer_pub"`
	MLSState     []byte `json:"mls_state"`
}

func marshalGroupState(gs *GroupState) ([]byte, error) {
	return json.Marshal(&groupStateSerialized{
		GroupID:      gs.GroupID,
		NostrGroupID: gs.NostrGroupID,
		PeerPub:      gs.PeerPub,
		MLSState:     gs.mlsBytes,
	})
}

func unmarshalGroupState(data []byte) (*groupStateSerialized, error) {
	var s groupStateSerialized
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// backupPayload is the NIP-44-encrypted content of a kind 30078 group backup.
type backupPayload struct {
	Groups      []groupStateBackup `json:"groups"`
	LastEventTS int64              `json:"last_event_ts"`
}

// groupStateBackup is the hex-encoded form for JSON transport in NIP-78 backup.
type groupStateBackup struct {
	GroupID      string `json:"group_id"`
	NostrGroupID string `json:"nostr_group_id"`
	PeerPub      string `json:"peer_pub"`
	MLSState     string `json:"mls_state"` // base64
	Epoch        uint64 `json:"epoch"`
}
