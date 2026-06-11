package nrc

import (
	"context"
	"encoding/json"
	"time"
)

const (
	// DefaultSessionTimeout is the default inactivity timeout for sessions.
	DefaultSessionTimeout = 30 * time.Minute
	// DefaultMaxSubscriptions is the default maximum subscriptions per session.
	DefaultMaxSubscriptions = 100
)

// --- Session actor request types ---

type sessAddSubReq struct {
	subID string
	resp  chan error
}

type sessRemoveSubReq struct {
	subID string
}

type sessGetSubReq struct {
	subID string
	resp  chan *Subscription
}

type sessHasSubReq struct {
	subID string
	resp  chan bool
}

type sessSubCountReq struct {
	resp chan int
}

type sessMarkEOSEReq struct {
	subID string
}

type sessIncrementEventCountReq struct {
	subID string
}

// Session represents an NRC client session through the tunnel.
type Session struct {
	// ID is the unique session identifier.
	ID string
	// ClientPubkey is the public key of the connected client.
	ClientPubkey []byte
	// ConversationKey is the NIP-44 conversation key for this session.
	ConversationKey []byte
	// DeviceName is the optional device identifier.
	DeviceName string
	// AuthMode is the authentication mode used.
	AuthMode AuthMode

	// CreatedAt is when the session was created.
	CreatedAt time.Time
	// LastActivity is the timestamp of the last activity.
	LastActivity time.Time

	// actor channels for subscription state
	addSubCh          chan sessAddSubReq
	removeSubCh       chan sessRemoveSubReq
	getSubCh          chan sessGetSubReq
	hasSubCh          chan sessHasSubReq
	subCountCh        chan sessSubCountReq
	markEOSECh        chan sessMarkEOSEReq
	incrEventCountCh  chan sessIncrementEventCountReq

	// ctx is the session context.
	ctx    context.Context
	cancel context.CancelFunc

	// eventCh receives events from the local relay for this session.
	eventCh chan *SessionEvent

	stop chan struct{}
	done chan struct{}
}

// Subscription represents a tunneled subscription.
type Subscription struct {
	// ID is the client's subscription ID.
	ID string
	// CreatedAt is when the subscription was created.
	CreatedAt time.Time
	// EventCount tracks how many events have been sent.
	EventCount int64
	// EOSESent indicates whether EOSE has been sent.
	EOSESent bool
}

// SessionEvent wraps a relay response for delivery to the client.
type SessionEvent struct {
	// Type is the response type (EVENT, OK, EOSE, NOTICE, CLOSED, COUNT, AUTH).
	Type string
	// Payload is the response payload array.
	Payload []any
	// RequestEventID is the ID of the request event this responds to (if applicable).
	RequestEventID string
}

// NewSession creates a new session.
func NewSession(id string, clientPubkey, conversationKey []byte, authMode AuthMode, deviceName string) *Session {
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now()
	s := &Session{
		ID:              id,
		ClientPubkey:    clientPubkey,
		ConversationKey: conversationKey,
		DeviceName:      deviceName,
		AuthMode:        authMode,
		CreatedAt:       now,
		LastActivity:    now,

		addSubCh:         make(chan sessAddSubReq),
		removeSubCh:      make(chan sessRemoveSubReq, 16),
		getSubCh:         make(chan sessGetSubReq),
		hasSubCh:         make(chan sessHasSubReq),
		subCountCh:       make(chan sessSubCountReq),
		markEOSECh:       make(chan sessMarkEOSEReq, 16),
		incrEventCountCh: make(chan sessIncrementEventCountReq, 16),

		ctx:     ctx,
		cancel:  cancel,
		eventCh: make(chan *SessionEvent, 100),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *Session) run() {
	defer close(s.done)

	subscriptions := make(map[string]*Subscription)

	for {
		select {
		case <-s.stop:
			return
		case req := <-s.addSubCh:
			if len(subscriptions) >= DefaultMaxSubscriptions {
				req.resp <- ErrTooManySubscriptions
			} else {
				subscriptions[req.subID] = &Subscription{
					ID:        req.subID,
					CreatedAt: time.Now(),
				}
				req.resp <- nil
			}
		case req := <-s.removeSubCh:
			delete(subscriptions, req.subID)
		case req := <-s.getSubCh:
			req.resp <- subscriptions[req.subID]
		case req := <-s.hasSubCh:
			_, ok := subscriptions[req.subID]
			req.resp <- ok
		case req := <-s.subCountCh:
			req.resp <- len(subscriptions)
		case req := <-s.markEOSECh:
			if sub, ok := subscriptions[req.subID]; ok {
				sub.EOSESent = true
			}
		case req := <-s.incrEventCountCh:
			if sub, ok := subscriptions[req.subID]; ok {
				sub.EventCount++
			}
		}
	}
}

// Context returns the session's context.
func (s *Session) Context() context.Context {
	return s.ctx
}

// Close closes the session and cleans up resources.
func (s *Session) Close() {
	s.cancel()
	close(s.stop)
	<-s.done
	close(s.eventCh)
}

// Events returns the channel for receiving events destined for this session.
func (s *Session) Events() <-chan *SessionEvent {
	return s.eventCh
}

// SendEvent sends an event to the session's event channel.
func (s *Session) SendEvent(ev *SessionEvent) bool {
	select {
	case s.eventCh <- ev:
		return true
	case <-s.ctx.Done():
		return false
	default:
		return false
	}
}

// Touch updates the last activity timestamp.
func (s *Session) Touch() {
	s.LastActivity = time.Now()
}

// IsExpired checks if the session has been inactive too long.
func (s *Session) IsExpired(timeout time.Duration) bool {
	return time.Since(s.LastActivity) > timeout
}

// AddSubscription adds a new subscription to the session.
func (s *Session) AddSubscription(subID string) error {
	resp := make(chan error, 1)
	select {
	case s.addSubCh <- sessAddSubReq{subID: subID, resp: resp}:
		return <-resp
	case <-s.stop:
		return ErrTooManySubscriptions
	}
}

// RemoveSubscription removes a subscription from the session.
func (s *Session) RemoveSubscription(subID string) {
	select {
	case s.removeSubCh <- sessRemoveSubReq{subID: subID}:
	case <-s.stop:
	}
}

// GetSubscription returns a subscription by ID.
func (s *Session) GetSubscription(subID string) *Subscription {
	resp := make(chan *Subscription, 1)
	select {
	case s.getSubCh <- sessGetSubReq{subID: subID, resp: resp}:
		return <-resp
	case <-s.stop:
		return nil
	}
}

// HasSubscription checks if a subscription exists.
func (s *Session) HasSubscription(subID string) bool {
	resp := make(chan bool, 1)
	select {
	case s.hasSubCh <- sessHasSubReq{subID: subID, resp: resp}:
		return <-resp
	case <-s.stop:
		return false
	}
}

// SubscriptionCount returns the number of active subscriptions.
func (s *Session) SubscriptionCount() int {
	resp := make(chan int, 1)
	select {
	case s.subCountCh <- sessSubCountReq{resp: resp}:
		return <-resp
	case <-s.stop:
		return 0
	}
}

// MarkEOSE marks a subscription as having sent EOSE.
func (s *Session) MarkEOSE(subID string) {
	select {
	case s.markEOSECh <- sessMarkEOSEReq{subID: subID}:
	case <-s.stop:
	}
}

// IncrementEventCount increments the event count for a subscription.
func (s *Session) IncrementEventCount(subID string) {
	select {
	case s.incrEventCountCh <- sessIncrementEventCountReq{subID: subID}:
	case <-s.stop:
	}
}

// --- SessionManager actor request types ---

type smGetReq struct {
	sessionID string
	resp      chan *Session
}

type smGetOrCreateReq struct {
	sessionID       string
	clientPubkey    []byte
	conversationKey []byte
	authMode        AuthMode
	deviceName      string
	resp            chan *Session
}

type smRemoveReq struct {
	sessionID string
}

type smCleanupExpiredReq struct {
	resp chan int
}

type smCountReq struct {
	resp chan int
}

type smCloseReq struct {
	resp chan struct{}
}

// SessionManager manages multiple NRC sessions.
type SessionManager struct {
	timeout time.Duration

	getCh             chan smGetReq
	getOrCreateCh     chan smGetOrCreateReq
	removeCh          chan smRemoveReq
	cleanupExpiredCh  chan smCleanupExpiredReq
	countCh           chan smCountReq
	closeCh           chan smCloseReq

	stop chan struct{}
	done chan struct{}
}

// NewSessionManager creates a new session manager.
func NewSessionManager(timeout time.Duration) *SessionManager {
	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}
	m := &SessionManager{
		timeout: timeout,

		getCh:            make(chan smGetReq),
		getOrCreateCh:    make(chan smGetOrCreateReq),
		removeCh:         make(chan smRemoveReq, 16),
		cleanupExpiredCh: make(chan smCleanupExpiredReq),
		countCh:          make(chan smCountReq),
		closeCh:          make(chan smCloseReq),

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go m.run()
	return m
}

func (m *SessionManager) run() {
	defer close(m.done)

	sessions := make(map[string]*Session)

	for {
		select {
		case <-m.stop:
			// Close all sessions
			for _, session := range sessions {
				session.Close()
			}
			return
		case req := <-m.getCh:
			req.resp <- sessions[req.sessionID]
		case req := <-m.getOrCreateCh:
			if session, ok := sessions[req.sessionID]; ok {
				session.Touch()
				req.resp <- session
			} else {
				session := NewSession(req.sessionID, req.clientPubkey, req.conversationKey, req.authMode, req.deviceName)
				sessions[req.sessionID] = session
				req.resp <- session
			}
		case req := <-m.removeCh:
			if session, ok := sessions[req.sessionID]; ok {
				session.Close()
				delete(sessions, req.sessionID)
			}
		case req := <-m.cleanupExpiredCh:
			var removed int
			for id, session := range sessions {
				if session.IsExpired(m.timeout) {
					session.Close()
					delete(sessions, id)
					removed++
				}
			}
			req.resp <- removed
		case req := <-m.countCh:
			req.resp <- len(sessions)
		case req := <-m.closeCh:
			for _, session := range sessions {
				session.Close()
			}
			sessions = make(map[string]*Session)
			req.resp <- struct{}{}
		}
	}
}

// Get returns a session by ID.
func (m *SessionManager) Get(sessionID string) *Session {
	resp := make(chan *Session, 1)
	select {
	case m.getCh <- smGetReq{sessionID: sessionID, resp: resp}:
		return <-resp
	case <-m.stop:
		return nil
	}
}

// GetOrCreate gets an existing session or creates a new one.
func (m *SessionManager) GetOrCreate(sessionID string, clientPubkey, conversationKey []byte, authMode AuthMode, deviceName string) *Session {
	resp := make(chan *Session, 1)
	select {
	case m.getOrCreateCh <- smGetOrCreateReq{
		sessionID:       sessionID,
		clientPubkey:    clientPubkey,
		conversationKey: conversationKey,
		authMode:        authMode,
		deviceName:      deviceName,
		resp:            resp,
	}:
		return <-resp
	case <-m.stop:
		return nil
	}
}

// Remove removes a session.
func (m *SessionManager) Remove(sessionID string) {
	select {
	case m.removeCh <- smRemoveReq{sessionID: sessionID}:
	case <-m.stop:
	}
}

// CleanupExpired removes expired sessions.
func (m *SessionManager) CleanupExpired() int {
	resp := make(chan int, 1)
	select {
	case m.cleanupExpiredCh <- smCleanupExpiredReq{resp: resp}:
		return <-resp
	case <-m.stop:
		return 0
	}
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() int {
	resp := make(chan int, 1)
	select {
	case m.countCh <- smCountReq{resp: resp}:
		return <-resp
	case <-m.stop:
		return 0
	}
}

// Close closes all sessions.
func (m *SessionManager) Close() {
	resp := make(chan struct{}, 1)
	select {
	case m.closeCh <- smCloseReq{resp: resp}:
		<-resp
	case <-m.stop:
	}
	close(m.stop)
	<-m.done
}

// RequestMessage represents a parsed NRC request message.
type RequestMessage struct {
	Type    string // EVENT, REQ, CLOSE, AUTH, COUNT, IDS
	Payload []any
}

// ResponseMessage represents an NRC response message to be sent.
type ResponseMessage struct {
	Type    string // EVENT, OK, EOSE, NOTICE, CLOSED, COUNT, AUTH, IDS, CHUNK
	Payload []any
}

// EventManifestEntry describes an event for manifest diffing (used by IDS).
type EventManifestEntry struct {
	Kind      int    `json:"kind"`
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	D         string `json:"d,omitempty"` // For parameterized replaceable events (kinds 30000-39999)
}

// ChunkMessage represents a chunk of a large message.
type ChunkMessage struct {
	Type      string `json:"type"`       // Always "CHUNK"
	MessageID string `json:"messageId"`  // Unique ID for the chunked message
	Index     int    `json:"index"`      // 0-based chunk index
	Total     int    `json:"total"`      // Total number of chunks
	Data      string `json:"data"`       // Base64 encoded chunk data
}

// ParseRequestContent parses the decrypted content of an NRC request.
func ParseRequestContent(content []byte) (*RequestMessage, error) {
	var msg struct {
		Type    string `json:"type"`
		Payload []any  `json:"payload"`
	}

	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, err
	}

	if msg.Type == "" {
		return nil, ErrInvalidMessageType
	}

	return &RequestMessage{
		Type:    msg.Type,
		Payload: msg.Payload,
	}, nil
}

// MarshalResponseContent marshals an NRC response for encryption.
func MarshalResponseContent(resp *ResponseMessage) ([]byte, error) {
	msg := struct {
		Type    string `json:"type"`
		Payload []any  `json:"payload"`
	}{
		Type:    resp.Type,
		Payload: resp.Payload,
	}
	return json.Marshal(msg)
}
