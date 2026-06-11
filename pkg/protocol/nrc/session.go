package nrc

import (
	"context"
	"encoding/json"
	"time"

	"git.smesh.lol/actor"
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

	// actor channels for subscription state
	addSub         actor.Func[string, error]
	removeSub      actor.Inbox[string]
	getSub         actor.Func[string, *Subscription]
	hasSub         actor.Func[string, bool]
	subCount       actor.Query[int]
	markEOSE       actor.Inbox[string]
	incrEventCount actor.Inbox[string]
	lc             actor.Lifecycle

	// ctx is the session context.
	ctx    context.Context
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
	s := &Session{
		ID:              id,
		ClientPubkey:    clientPubkey,
		ConversationKey: conversationKey,
		DeviceName:      deviceName,
		AuthMode:        authMode,
		CreatedAt:       now,
		LastActivity:    now,

		addSub:         actor.NewFunc[string, error](),
		removeSub:      actor.NewInbox[string](16),
		getSub:         actor.NewFunc[string, *Subscription](),
		hasSub:         actor.NewFunc[string, bool](),
		subCount:       actor.NewQuery[int](),
		markEOSE:       actor.NewInbox[string](16),
		incrEventCount: actor.NewInbox[string](16),
		lc:             actor.NewLifecycle(),

		ctx:     ctx,
		cancel:  cancel,
		eventCh: make(chan *SessionEvent, 100),
	}
	actor.Go(s.lc, s.run)
	return s
}

func (s *Session) run() {
	subscriptions := make(map[string]*Subscription)

	for {
		select {
		case <-s.lc.Stopping():
			return
		case msg := <-s.addSub.Recv():
			if len(subscriptions) >= DefaultMaxSubscriptions {
				msg.Reply(ErrTooManySubscriptions)
			} else {
				subscriptions[msg.Req] = &Subscription{
					ID:        msg.Req,
					CreatedAt: time.Now(),
				}
				msg.Reply(nil)
			}
		case subID := <-s.removeSub.Recv():
			delete(subscriptions, subID)
		case msg := <-s.getSub.Recv():
			msg.Reply(subscriptions[msg.Req])
		case msg := <-s.hasSub.Recv():
			_, ok := subscriptions[msg.Req]
			msg.Reply(ok)
		case msg := <-s.subCount.Recv():
			msg.Reply(len(subscriptions))
		case subID := <-s.markEOSE.Recv():
			if sub, ok := subscriptions[subID]; ok {
				sub.EOSESent = true
			}
		case subID := <-s.incrEventCount.Recv():
			if sub, ok := subscriptions[subID]; ok {
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
	s.lc.Stop()
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
func (s *Session) AddSubscription(subID string) error { return s.addSub.Call(subID) }

// RemoveSubscription removes a subscription from the session.
func (s *Session) RemoveSubscription(subID string) { s.removeSub.TrySend(subID) }

// GetSubscription returns a subscription by ID.
func (s *Session) GetSubscription(subID string) *Subscription { return s.getSub.Call(subID) }

// HasSubscription checks if a subscription exists.
func (s *Session) HasSubscription(subID string) bool { return s.hasSub.Call(subID) }

// SubscriptionCount returns the number of active subscriptions.
func (s *Session) SubscriptionCount() int { return s.subCount.Call() }

// MarkEOSE marks a subscription as having sent EOSE.
func (s *Session) MarkEOSE(subID string) { s.markEOSE.TrySend(subID) }

// IncrementEventCount increments the event count for a subscription.
func (s *Session) IncrementEventCount(subID string) { s.incrEventCount.TrySend(subID) }

// getOrCreateArgs holds multi-arg parameters for GetOrCreate.
type getOrCreateArgs struct {
	sessionID       string
	clientPubkey    []byte
	conversationKey []byte
	authMode        AuthMode
	deviceName      string
}

// SessionManager manages multiple NRC sessions.
type SessionManager struct {
	timeout time.Duration

	get            actor.Func[string, *Session]
	getOrCreate    actor.Func[getOrCreateArgs, *Session]
	remove         actor.Inbox[string]
	cleanupExpired actor.Query[int]
	count          actor.Query[int]
	lc             actor.Lifecycle
}

// NewSessionManager creates a new session manager.
func NewSessionManager(timeout time.Duration) *SessionManager {
	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}
	m := &SessionManager{
		timeout: timeout,

		get:            actor.NewFunc[string, *Session](),
		getOrCreate:    actor.NewFunc[getOrCreateArgs, *Session](),
		remove:         actor.NewInbox[string](16),
		cleanupExpired: actor.NewQuery[int](),
		count:          actor.NewQuery[int](),
		lc:             actor.NewLifecycle(),
	}
	actor.Go(m.lc, m.run)
	return m
}

func (m *SessionManager) run() {
	sessions := make(map[string]*Session)

	for {
		select {
		case <-m.lc.Stopping():
			for _, session := range sessions {
				session.Close()
			}
			return
		case msg := <-m.get.Recv():
			msg.Reply(sessions[msg.Req])
		case msg := <-m.getOrCreate.Recv():
			if session, ok := sessions[msg.Req.sessionID]; ok {
				session.Touch()
				msg.Reply(session)
			} else {
				session := NewSession(msg.Req.sessionID, msg.Req.clientPubkey, msg.Req.conversationKey, msg.Req.authMode, msg.Req.deviceName)
				sessions[msg.Req.sessionID] = session
				msg.Reply(session)
			}
		case sessionID := <-m.remove.Recv():
			if session, ok := sessions[sessionID]; ok {
				session.Close()
				delete(sessions, sessionID)
			}
		case msg := <-m.cleanupExpired.Recv():
			var removed int
			for id, session := range sessions {
				if session.IsExpired(m.timeout) {
					session.Close()
					delete(sessions, id)
					removed++
				}
			}
			msg.Reply(removed)
		case msg := <-m.count.Recv():
			msg.Reply(len(sessions))
		}
	}
}

// Get returns a session by ID.
func (m *SessionManager) Get(sessionID string) *Session { return m.get.Call(sessionID) }

// GetOrCreate gets an existing session or creates a new one.
func (m *SessionManager) GetOrCreate(sessionID string, clientPubkey, conversationKey []byte, authMode AuthMode, deviceName string) *Session {
	return m.getOrCreate.Call(getOrCreateArgs{
		sessionID:       sessionID,
		clientPubkey:    clientPubkey,
		conversationKey: conversationKey,
		authMode:        authMode,
		deviceName:      deviceName,
	})
}

// Remove removes a session.
func (m *SessionManager) Remove(sessionID string) { m.remove.TrySend(sessionID) }

// CleanupExpired removes expired sessions.
func (m *SessionManager) CleanupExpired() int { return m.cleanupExpired.Call() }

// Count returns the number of active sessions.
func (m *SessionManager) Count() int { return m.count.Call() }

// Close closes all sessions and stops the manager.
func (m *SessionManager) Close() { m.lc.Stop() }

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
