package nrc

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

const (
	// DefaultSessionTimeout is the default inactivity timeout for sessions.
	DefaultSessionTimeout = 30 * time.Minute
	// DefaultMaxSubscriptions is the default maximum subscriptions per session.
	DefaultMaxSubscriptions = 100
)

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

	// subscriptions maps client subscription IDs to internal subscription state.
	subscriptions map[string]*Subscription
	// subMu protects the subscriptions map.
	subMu sync.RWMutex

	// ctx is the session context.
	ctx context.Context
	// cancel cancels the session context.
	cancel context.CancelFunc

	// eventCh receives events from the local relay for this session.
	eventCh chan *SessionEvent
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
	return &Session{
		ID:              id,
		ClientPubkey:    clientPubkey,
		ConversationKey: conversationKey,
		DeviceName:      deviceName,
		AuthMode:        authMode,
		CreatedAt:       now,
		LastActivity:    now,
		subscriptions:   make(map[string]*Subscription),
		ctx:             ctx,
		cancel:          cancel,
		eventCh:         make(chan *SessionEvent, 100),
	}
}

// Context returns the session's context.
func (s *Session) Context() context.Context {
	return s.ctx
}

// Close closes the session and cleans up resources.
func (s *Session) Close() {
	s.cancel()
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
		// Channel full, drop event
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
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if len(s.subscriptions) >= DefaultMaxSubscriptions {
		return ErrTooManySubscriptions
	}

	s.subscriptions[subID] = &Subscription{
		ID:        subID,
		CreatedAt: time.Now(),
	}
	return nil
}

// RemoveSubscription removes a subscription from the session.
func (s *Session) RemoveSubscription(subID string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	delete(s.subscriptions, subID)
}

// GetSubscription returns a subscription by ID.
func (s *Session) GetSubscription(subID string) *Subscription {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	return s.subscriptions[subID]
}

// HasSubscription checks if a subscription exists.
func (s *Session) HasSubscription(subID string) bool {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	_, ok := s.subscriptions[subID]
	return ok
}

// SubscriptionCount returns the number of active subscriptions.
func (s *Session) SubscriptionCount() int {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	return len(s.subscriptions)
}

// MarkEOSE marks a subscription as having sent EOSE.
func (s *Session) MarkEOSE(subID string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if sub, ok := s.subscriptions[subID]; ok {
		sub.EOSESent = true
	}
}

// IncrementEventCount increments the event count for a subscription.
func (s *Session) IncrementEventCount(subID string) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if sub, ok := s.subscriptions[subID]; ok {
		sub.EventCount++
	}
}

// SessionManager manages multiple NRC sessions.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewSessionManager creates a new session manager.
func NewSessionManager(timeout time.Duration) *SessionManager {
	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}
	return &SessionManager{
		sessions: make(map[string]*Session),
		timeout:  timeout,
	}
}

// Get returns a session by ID.
func (m *SessionManager) Get(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// GetOrCreate gets an existing session or creates a new one.
func (m *SessionManager) GetOrCreate(sessionID string, clientPubkey, conversationKey []byte, authMode AuthMode, deviceName string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[sessionID]; ok {
		session.Touch()
		return session
	}

	session := NewSession(sessionID, clientPubkey, conversationKey, authMode, deviceName)
	m.sessions[sessionID] = session
	return session
}

// Remove removes a session.
func (m *SessionManager) Remove(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[sessionID]; ok {
		session.Close()
		delete(m.sessions, sessionID)
	}
}

// CleanupExpired removes expired sessions.
func (m *SessionManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	var removed int
	for id, session := range m.sessions {
		if session.IsExpired(m.timeout) {
			session.Close()
			delete(m.sessions, id)
			removed++
		}
	}
	return removed
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Close closes all sessions.
func (m *SessionManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, session := range m.sessions {
		session.Close()
	}
	m.sessions = make(map[string]*Session)
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
	// Content format: {"type": "EVENT|REQ|...", "payload": [...]}
	// Parse as generic JSON
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
