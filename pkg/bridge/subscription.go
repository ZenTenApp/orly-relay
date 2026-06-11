package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// --- FileSubscriptionStore actor ---

type fileSaveReq struct {
	sub  *Subscription
	resp chan error
}

type fileGetReq struct {
	pubkeyHex string
	resp      chan fileGetResp
}

type fileGetResp struct {
	sub *Subscription
	err error
}

type fileListReq struct {
	resp chan fileListResp
}

type fileListResp struct {
	subs []*Subscription
	err  error
}

type fileDeleteReq struct {
	pubkeyHex string
	resp      chan error
}

// FileSubscriptionStore persists subscriptions as a JSON file.
type FileSubscriptionStore struct {
	path string
	subs map[string]*Subscription

	saveCh   chan fileSaveReq
	getCh    chan fileGetReq
	listCh   chan fileListReq
	deleteCh chan fileDeleteReq
	stop     chan struct{}
	done     chan struct{}
}

// NewFileSubscriptionStore creates a subscription store backed by a JSON file.
func NewFileSubscriptionStore(dataDir string) (*FileSubscriptionStore, error) {
	path := filepath.Join(dataDir, "subscriptions.json")
	store := &FileSubscriptionStore{
		path:     path,
		subs:     make(map[string]*Subscription),
		saveCh:   make(chan fileSaveReq),
		getCh:    make(chan fileGetReq),
		listCh:   make(chan fileListReq),
		deleteCh: make(chan fileDeleteReq),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
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

	go store.loop()
	return store, nil
}

func (s *FileSubscriptionStore) loop() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case req := <-s.saveCh:
			s.subs[req.sub.PubkeyHex] = req.sub
			req.resp <- s.flush()
		case req := <-s.getCh:
			sub, ok := s.subs[req.pubkeyHex]
			if !ok {
				req.resp <- fileGetResp{nil, fmt.Errorf("subscription not found for %s", req.pubkeyHex)}
			} else {
				req.resp <- fileGetResp{sub, nil}
			}
		case req := <-s.listCh:
			var subs []*Subscription
			for _, sub := range s.subs {
				subs = append(subs, sub)
			}
			req.resp <- fileListResp{subs, nil}
		case req := <-s.deleteCh:
			delete(s.subs, req.pubkeyHex)
			req.resp <- s.flush()
		}
	}
}

func (s *FileSubscriptionStore) Save(sub *Subscription) error {
	req := fileSaveReq{sub: sub, resp: make(chan error, 1)}
	s.saveCh <- req
	return <-req.resp
}

func (s *FileSubscriptionStore) Get(pubkeyHex string) (*Subscription, error) {
	req := fileGetReq{pubkeyHex: pubkeyHex, resp: make(chan fileGetResp, 1)}
	s.getCh <- req
	r := <-req.resp
	return r.sub, r.err
}

func (s *FileSubscriptionStore) List() ([]*Subscription, error) {
	req := fileListReq{resp: make(chan fileListResp, 1)}
	s.listCh <- req
	r := <-req.resp
	return r.subs, r.err
}

func (s *FileSubscriptionStore) Delete(pubkeyHex string) error {
	req := fileDeleteReq{pubkeyHex: pubkeyHex, resp: make(chan error, 1)}
	s.deleteCh <- req
	return <-req.resp
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

// Stop shuts down the actor goroutine and waits for it to exit.
func (s *FileSubscriptionStore) Stop() {
	close(s.stop)
	<-s.done
}

// --- MemorySubscriptionStore actor ---

type memSaveReq struct {
	sub  *Subscription
	resp chan error
}

type memGetReq struct {
	pubkeyHex string
	resp      chan memGetResp
}

type memGetResp struct {
	sub *Subscription
	err error
}

type memListReq struct {
	resp chan memListResp
}

type memListResp struct {
	subs []*Subscription
	err  error
}

type memDeleteReq struct {
	pubkeyHex string
	resp      chan error
}

// MemorySubscriptionStore is an in-memory subscription store for testing.
type MemorySubscriptionStore struct {
	subs map[string]*Subscription

	saveCh   chan memSaveReq
	getCh    chan memGetReq
	listCh   chan memListReq
	deleteCh chan memDeleteReq
	stop     chan struct{}
	done     chan struct{}
}

// NewMemorySubscriptionStore creates a new in-memory subscription store.
func NewMemorySubscriptionStore() *MemorySubscriptionStore {
	s := &MemorySubscriptionStore{
		subs:     make(map[string]*Subscription),
		saveCh:   make(chan memSaveReq),
		getCh:    make(chan memGetReq),
		listCh:   make(chan memListReq),
		deleteCh: make(chan memDeleteReq),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go s.loop()
	return s
}

func (s *MemorySubscriptionStore) loop() {
	defer close(s.done)
	for {
		select {
		case <-s.stop:
			return
		case req := <-s.saveCh:
			s.subs[req.sub.PubkeyHex] = req.sub
			req.resp <- nil
		case req := <-s.getCh:
			sub, ok := s.subs[req.pubkeyHex]
			if !ok {
				req.resp <- memGetResp{nil, fmt.Errorf("subscription not found for %s", req.pubkeyHex)}
			} else {
				req.resp <- memGetResp{sub, nil}
			}
		case req := <-s.listCh:
			var subs []*Subscription
			for _, sub := range s.subs {
				subs = append(subs, sub)
			}
			req.resp <- memListResp{subs, nil}
		case req := <-s.deleteCh:
			delete(s.subs, req.pubkeyHex)
			req.resp <- nil
		}
	}
}

func (s *MemorySubscriptionStore) Save(sub *Subscription) error {
	req := memSaveReq{sub: sub, resp: make(chan error, 1)}
	s.saveCh <- req
	return <-req.resp
}

func (s *MemorySubscriptionStore) Get(pubkeyHex string) (*Subscription, error) {
	req := memGetReq{pubkeyHex: pubkeyHex, resp: make(chan memGetResp, 1)}
	s.getCh <- req
	r := <-req.resp
	return r.sub, r.err
}

func (s *MemorySubscriptionStore) List() ([]*Subscription, error) {
	req := memListReq{resp: make(chan memListResp, 1)}
	s.listCh <- req
	r := <-req.resp
	return r.subs, r.err
}

func (s *MemorySubscriptionStore) Delete(pubkeyHex string) error {
	req := memDeleteReq{pubkeyHex: pubkeyHex, resp: make(chan error, 1)}
	s.deleteCh <- req
	return <-req.resp
}

// Stop shuts down the actor goroutine and waits for it to exit.
func (s *MemorySubscriptionStore) Stop() {
	close(s.stop)
	<-s.done
}
