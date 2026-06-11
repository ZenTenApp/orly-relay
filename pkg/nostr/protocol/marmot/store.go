package marmot

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
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

// --- FileGroupStore actor request types ---

type fgsSaveGroupReq struct {
	groupID []byte
	state   []byte
	resp    chan error
}

type fgsLoadGroupReq struct {
	groupID []byte
	resp    chan fgsLoadGroupResp
}

type fgsLoadGroupResp struct {
	data []byte
	err  error
}

type fgsListGroupsReq struct {
	resp chan fgsListGroupsResp
}

type fgsListGroupsResp struct {
	ids [][]byte
	err error
}

type fgsDeleteGroupReq struct {
	groupID []byte
	resp    chan error
}

type fgsSaveKPReq struct {
	data []byte
	resp chan error
}

type fgsLoadKPReq struct {
	resp chan fgsLoadGroupResp
}

// FileGroupStore implements GroupStore using one file per group in a directory.
type FileGroupStore struct {
	dir      string
	saveGrp  chan fgsSaveGroupReq
	loadGrp  chan fgsLoadGroupReq
	listGrps chan fgsListGroupsReq
	delGrp   chan fgsDeleteGroupReq
	saveKP   chan fgsSaveKPReq
	loadKP   chan fgsLoadKPReq
	stop     chan struct{}
	done     chan struct{}
}

// NewFileGroupStore creates a file-backed group store. The directory is created
// if it does not exist.
func NewFileGroupStore(dir string) (*FileGroupStore, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &FileGroupStore{
		dir:      dir,
		saveGrp:  make(chan fgsSaveGroupReq),
		loadGrp:  make(chan fgsLoadGroupReq),
		listGrps: make(chan fgsListGroupsReq),
		delGrp:   make(chan fgsDeleteGroupReq),
		saveKP:   make(chan fgsSaveKPReq),
		loadKP:   make(chan fgsLoadKPReq),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go s.actor()
	return s, nil
}

func (s *FileGroupStore) actor() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case req := <-s.saveGrp:
			req.resp <- os.WriteFile(s.path(req.groupID), req.state, 0600)
		case req := <-s.loadGrp:
			data, err := os.ReadFile(s.path(req.groupID))
			req.resp <- fgsLoadGroupResp{data, err}
		case req := <-s.listGrps:
			entries, err := os.ReadDir(s.dir)
			if err != nil {
				req.resp <- fgsListGroupsResp{nil, err}
				continue
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
			req.resp <- fgsListGroupsResp{ids, nil}
		case req := <-s.delGrp:
			req.resp <- os.Remove(s.path(req.groupID))
		case req := <-s.saveKP:
			req.resp <- os.WriteFile(s.kppPath(), req.data, 0600)
		case req := <-s.loadKP:
			data, err := os.ReadFile(s.kppPath())
			req.resp <- fgsLoadGroupResp{data, err}
		}
	}
}

// Stop shuts down the actor goroutine.
func (s *FileGroupStore) Stop() {
	close(s.stop)
	<-s.done
}

func (s *FileGroupStore) path(groupID []byte) string {
	return filepath.Join(s.dir, hex.EncodeToString(groupID)+".json")
}

func (s *FileGroupStore) kppPath() string {
	return filepath.Join(s.dir, "_keypackage.dat")
}

func (s *FileGroupStore) SaveGroup(groupID, state []byte) error {
	req := fgsSaveGroupReq{groupID: groupID, state: state, resp: make(chan error, 1)}
	s.saveGrp <- req
	return <-req.resp
}

func (s *FileGroupStore) LoadGroup(groupID []byte) ([]byte, error) {
	req := fgsLoadGroupReq{groupID: groupID, resp: make(chan fgsLoadGroupResp, 1)}
	s.loadGrp <- req
	r := <-req.resp
	return r.data, r.err
}

func (s *FileGroupStore) ListGroups() ([][]byte, error) {
	req := fgsListGroupsReq{resp: make(chan fgsListGroupsResp, 1)}
	s.listGrps <- req
	r := <-req.resp
	return r.ids, r.err
}

func (s *FileGroupStore) DeleteGroup(groupID []byte) error {
	req := fgsDeleteGroupReq{groupID: groupID, resp: make(chan error, 1)}
	s.delGrp <- req
	return <-req.resp
}

func (s *FileGroupStore) SaveKeyPackage(data []byte) error {
	req := fgsSaveKPReq{data: data, resp: make(chan error, 1)}
	s.saveKP <- req
	return <-req.resp
}

func (s *FileGroupStore) LoadKeyPackage() ([]byte, error) {
	req := fgsLoadKPReq{resp: make(chan fgsLoadGroupResp, 1)}
	s.loadKP <- req
	r := <-req.resp
	return r.data, r.err
}

// --- MemoryGroupStore actor request types ---

type mgsSaveGroupReq struct {
	groupID []byte
	state   []byte
	resp    chan error
}

type mgsLoadGroupReq struct {
	groupID []byte
	resp    chan fgsLoadGroupResp
}

type mgsListGroupsReq struct {
	resp chan fgsListGroupsResp
}

type mgsDeleteGroupReq struct {
	groupID []byte
	resp    chan error
}

type mgsSaveKPReq struct {
	data []byte
	resp chan error
}

type mgsLoadKPReq struct {
	resp chan fgsLoadGroupResp
}

// MemoryGroupStore implements GroupStore in memory. Useful for testing.
type MemoryGroupStore struct {
	saveGrp  chan mgsSaveGroupReq
	loadGrp  chan mgsLoadGroupReq
	listGrps chan mgsListGroupsReq
	delGrp   chan mgsDeleteGroupReq
	saveKP   chan mgsSaveKPReq
	loadKP   chan mgsLoadKPReq
	stop     chan struct{}
	done     chan struct{}
}

func NewMemoryGroupStore() *MemoryGroupStore {
	s := &MemoryGroupStore{
		saveGrp:  make(chan mgsSaveGroupReq),
		loadGrp:  make(chan mgsLoadGroupReq),
		listGrps: make(chan mgsListGroupsReq),
		delGrp:   make(chan mgsDeleteGroupReq),
		saveKP:   make(chan mgsSaveKPReq),
		loadKP:   make(chan mgsLoadKPReq),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go s.actor()
	return s
}

func (s *MemoryGroupStore) actor() {
	defer close(s.done)
	groups := make(map[string][]byte)
	for {
		select {
		case <-s.stop:
			return
		case req := <-s.saveGrp:
			cp := make([]byte, len(req.state))
			copy(cp, req.state)
			groups[string(req.groupID)] = cp
			req.resp <- nil
		case req := <-s.loadGrp:
			data, ok := groups[string(req.groupID)]
			if !ok {
				req.resp <- fgsLoadGroupResp{nil, os.ErrNotExist}
				continue
			}
			cp := make([]byte, len(data))
			copy(cp, data)
			req.resp <- fgsLoadGroupResp{cp, nil}
		case req := <-s.listGrps:
			ids := make([][]byte, 0, len(groups))
			for k := range groups {
				ids = append(ids, []byte(k))
			}
			req.resp <- fgsListGroupsResp{ids, nil}
		case req := <-s.delGrp:
			delete(groups, string(req.groupID))
			req.resp <- nil
		case req := <-s.saveKP:
			cp := make([]byte, len(req.data))
			copy(cp, req.data)
			groups["_kpp"] = cp
			req.resp <- nil
		case req := <-s.loadKP:
			data, ok := groups["_kpp"]
			if !ok {
				req.resp <- fgsLoadGroupResp{nil, os.ErrNotExist}
				continue
			}
			cp := make([]byte, len(data))
			copy(cp, data)
			req.resp <- fgsLoadGroupResp{cp, nil}
		}
	}
}

// Stop shuts down the actor goroutine.
func (s *MemoryGroupStore) Stop() {
	close(s.stop)
	<-s.done
}

func (s *MemoryGroupStore) SaveGroup(groupID, state []byte) error {
	req := mgsSaveGroupReq{groupID: groupID, state: state, resp: make(chan error, 1)}
	s.saveGrp <- req
	return <-req.resp
}

func (s *MemoryGroupStore) LoadGroup(groupID []byte) ([]byte, error) {
	req := mgsLoadGroupReq{groupID: groupID, resp: make(chan fgsLoadGroupResp, 1)}
	s.loadGrp <- req
	r := <-req.resp
	return r.data, r.err
}

func (s *MemoryGroupStore) ListGroups() ([][]byte, error) {
	req := mgsListGroupsReq{resp: make(chan fgsListGroupsResp, 1)}
	s.listGrps <- req
	r := <-req.resp
	return r.ids, r.err
}

func (s *MemoryGroupStore) DeleteGroup(groupID []byte) error {
	req := mgsDeleteGroupReq{groupID: groupID, resp: make(chan error, 1)}
	s.delGrp <- req
	return <-req.resp
}

func (s *MemoryGroupStore) SaveKeyPackage(data []byte) error {
	req := mgsSaveKPReq{data: data, resp: make(chan error, 1)}
	s.saveKP <- req
	return <-req.resp
}

func (s *MemoryGroupStore) LoadKeyPackage() ([]byte, error) {
	req := mgsLoadKPReq{resp: make(chan fgsLoadGroupResp, 1)}
	s.loadKP <- req
	r := <-req.resp
	return r.data, r.err
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
