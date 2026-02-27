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

// groupStateSerialized is the JSON structure for persisting group state.
type groupStateSerialized struct {
	GroupID  []byte `json:"group_id"`
	PeerPub  []byte `json:"peer_pub"`
	MLSState []byte `json:"mls_state"`
}

func marshalGroupState(gs *GroupState) ([]byte, error) {
	return json.Marshal(&groupStateSerialized{
		GroupID:  gs.GroupID,
		PeerPub:  gs.PeerPub,
		MLSState: gs.mlsBytes,
	})
}

func unmarshalGroupState(data []byte) (*groupStateSerialized, error) {
	var s groupStateSerialized
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
