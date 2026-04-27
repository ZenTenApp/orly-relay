package nrc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"git.smesh.lol/orly/pkg/nostr/crypto/encryption"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

const (
	// KindNRCRequest is the event kind for NRC requests.
	KindNRCRequest = 24891
	// KindNRCResponse is the event kind for NRC responses.
	KindNRCResponse = 24892
	// MaxChunkSize is the maximum size for a single chunk (40KB to stay under 65KB limit after NIP-44 + base64).
	MaxChunkSize = 40000
)

// NRCAuthorizer defines the interface for NRC authorization lookups.
// This allows the bridge to look up authorized clients dynamically from the database.
type NRCAuthorizer interface {
	// GetNRCClientByPubkey looks up an authorized client by their derived pubkey.
	// Returns the client ID, label, and whether the client was found.
	// If not found, returns empty strings and false.
	GetNRCClientByPubkey(derivedPubkey []byte) (id string, label string, found bool, err error)
	// UpdateNRCClientLastUsed updates the last used timestamp for tracking.
	UpdateNRCClientLastUsed(id string) error
}

// BridgeConfig holds configuration for the NRC bridge.
type BridgeConfig struct {
	// RendezvousURL is the WebSocket URL of the public relay.
	RendezvousURL string
	// LocalRelayURL is the WebSocket URL of the local private relay.
	LocalRelayURL string
	// Signer is the relay's signer for signing response events.
	Signer signer.I
	// AuthorizedSecrets maps derived pubkeys to device names (secret-based auth).
	// Used when Authorizer is nil.
	AuthorizedSecrets map[string]string
	// Authorizer provides dynamic NRC authorization lookups from database.
	// If set, this takes precedence over AuthorizedSecrets.
	Authorizer NRCAuthorizer
	// SessionTimeout is the inactivity timeout for sessions.
	SessionTimeout time.Duration
}

// Bridge connects a private relay to a public rendezvous relay.
type Bridge struct {
	config   *BridgeConfig
	sessions *SessionManager

	// rendezvousConn is the connection to the rendezvous relay.
	rendezvousConn *ws.Client

	// mu protects connection state.
	mu sync.RWMutex

	// ctx is the bridge context.
	ctx context.Context
	// cancel cancels the bridge context.
	cancel context.CancelFunc
}

// NewBridge creates a new NRC bridge.
func NewBridge(config *BridgeConfig) *Bridge {
	ctx, cancel := context.WithCancel(context.Background())
	timeout := config.SessionTimeout
	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}
	return &Bridge{
		config:   config,
		sessions: NewSessionManager(timeout),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts the bridge and begins listening for NRC requests.
func (b *Bridge) Start() error {
	log.I.F("starting NRC bridge, rendezvous: %s, local: %s",
		b.config.RendezvousURL, b.config.LocalRelayURL)

	// Start session cleanup goroutine
	go b.cleanupLoop()

	// Start the main bridge loop with auto-reconnection
	go b.runLoop()

	return nil
}

// Stop stops the bridge.
func (b *Bridge) Stop() {
	log.I.F("stopping NRC bridge")
	b.cancel()
	b.sessions.Close()

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.rendezvousConn != nil {
		b.rendezvousConn.Close()
	}
}

// UpdateAuthorizedSecrets updates the map of authorized secrets.
// This allows dynamic management of authorized connections through the UI.
func (b *Bridge) UpdateAuthorizedSecrets(secrets map[string]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.config.AuthorizedSecrets = secrets
}

// cleanupLoop periodically cleans up expired sessions.
func (b *Bridge) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			removed := b.sessions.CleanupExpired()
			if removed > 0 {
				log.D.F("cleaned up %d expired NRC sessions", removed)
			}
		}
	}
}

// runLoop runs the main bridge loop with auto-reconnection.
func (b *Bridge) runLoop() {
	delay := time.Second

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
		}

		err := b.runOnce()
		if err != nil {
			if b.ctx.Err() != nil {
				return // Context cancelled, exit cleanly
			}
			log.W.F("NRC bridge error: %v, reconnecting in %v", err, delay)
			select {
			case <-time.After(delay):
				if delay < 30*time.Second {
					delay *= 2
				}
			case <-b.ctx.Done():
				return
			}
			continue
		}
		delay = time.Second
	}
}

// runOnce runs a single iteration of the bridge.
func (b *Bridge) runOnce() error {
	// Connect to rendezvous relay
	rendezvousConn, err := ws.RelayConnect(b.ctx, b.config.RendezvousURL)
	if chk.E(err) {
		return fmt.Errorf("%w: %v", ErrRendezvousConnectionFailed, err)
	}
	defer rendezvousConn.Close()

	b.mu.Lock()
	b.rendezvousConn = rendezvousConn
	b.mu.Unlock()

	// Subscribe to NRC request events
	relayPubkeyHex := hex.Enc(b.config.Signer.Pub())
	sub, err := rendezvousConn.Subscribe(
		b.ctx,
		filter.NewS(&filter.F{
			Kinds: kind.NewS(kind.New(KindNRCRequest)),
			Tags: tag.NewS(
				tag.NewFromAny("p", relayPubkeyHex),
			),
			Since: &timestamp.T{V: time.Now().Unix()},
		}),
	)
	if chk.E(err) {
		return fmt.Errorf("subscription failed: %w", err)
	}
	defer sub.Unsub()

	log.I.F("NRC bridge listening for requests on %s", b.config.RendezvousURL)

	// Process incoming request events
	for {
		select {
		case <-b.ctx.Done():
			return nil
		case ev := <-sub.Events:
			if ev == nil {
				return fmt.Errorf("subscription closed")
			}
			go b.handleRequest(ev)
		}
	}
}

// handleRequest handles a single NRC request event.
func (b *Bridge) handleRequest(ev *event.E) {
	ctx, cancel := context.WithTimeout(b.ctx, 30*time.Second)
	defer cancel()

	// Extract session ID from tags
	sessionID := ""
	sessionTag := ev.Tags.GetFirst([]byte("session"))
	if sessionTag != nil && sessionTag.Len() >= 2 {
		sessionID = string(sessionTag.Value())
	}
	if sessionID == "" {
		log.W.F("NRC request missing session tag from %s", hex.Enc(ev.Pubkey[:]))
		return
	}

	// Verify authorization
	conversationKey, authMode, deviceName, err := b.authorize(ctx, ev)
	if err != nil {
		log.W.F("NRC authorization failed for %s: %v", hex.Enc(ev.Pubkey[:]), err)
		b.sendError(ctx, ev, sessionID, "unauthorized: "+err.Error())
		return
	}

	// Get or create session
	session := b.sessions.GetOrCreate(sessionID, ev.Pubkey[:], conversationKey, authMode, deviceName)
	session.Touch()

	// Decrypt request content
	decrypted, err := encryption.Decrypt(conversationKey, string(ev.Content))
	if err != nil {
		log.W.F("NRC decryption failed: %v", err)
		b.sendError(ctx, ev, sessionID, "decryption failed")
		return
	}

	// Parse request message
	reqMsg, err := ParseRequestContent([]byte(decrypted))
	if err != nil {
		log.W.F("NRC invalid request format: %v", err)
		b.sendError(ctx, ev, sessionID, "invalid request format")
		return
	}

	log.D.F("NRC request: type=%s session=%s from=%s",
		reqMsg.Type, sessionID, hex.Enc(ev.Pubkey[:]))

	// Forward to local relay and handle response
	if err := b.forwardToLocalRelay(ctx, session, ev, reqMsg); err != nil {
		log.W.F("NRC forward failed: %v", err)
		b.sendError(ctx, ev, sessionID, "relay error: "+err.Error())
	}
}

// authorize checks if the request is authorized and returns the conversation key.
func (b *Bridge) authorize(ctx context.Context, ev *event.E) (conversationKey []byte, authMode AuthMode, deviceName string, err error) {
	clientPubkey := ev.Pubkey[:]
	clientPubkeyHex := string(hex.Enc(clientPubkey))

	// Try database-backed authorization first (if Authorizer is set)
	if b.config.Authorizer != nil {
		clientID, clientLabel, found, authErr := b.config.Authorizer.GetNRCClientByPubkey(clientPubkey)
		if authErr == nil && found {
			// Client is authorized via database
			conversationKey, err = encryption.GenerateConversationKey(
				b.config.Signer.Sec(),
				clientPubkey,
			)
			if chk.E(err) {
				return
			}
			authMode = AuthModeSecret
			deviceName = clientLabel

			// Update last used timestamp in background
			go func() {
				if updateErr := b.config.Authorizer.UpdateNRCClientLastUsed(clientID); updateErr != nil {
					log.W.F("failed to update NRC client last used: %v", updateErr)
				}
			}()
			return
		}
	}

	// Fallback to static map (for backwards compatibility)
	if name, ok := b.config.AuthorizedSecrets[clientPubkeyHex]; ok {
		// Secret auth uses ECDH between relay key and client's derived key
		conversationKey, err = encryption.GenerateConversationKey(
			b.config.Signer.Sec(),
			clientPubkey,
		)
		if chk.E(err) {
			return
		}
		authMode = AuthModeSecret
		deviceName = name
		return
	}

	err = ErrUnauthorized
	return
}

// forwardToLocalRelay forwards a request to the local relay and handles responses.
func (b *Bridge) forwardToLocalRelay(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage) error {
	// Connect to local relay
	localConn, err := ws.RelayConnect(ctx, b.config.LocalRelayURL)
	if chk.E(err) {
		return fmt.Errorf("%w: %v", ErrRelayConnectionFailed, err)
	}
	defer localConn.Close()

	// Handle different message types
	switch reqMsg.Type {
	case "REQ":
		return b.handleREQ(ctx, session, reqEvent, reqMsg, localConn)
	case "EVENT":
		return b.handleEVENT(ctx, session, reqEvent, reqMsg, localConn)
	case "CLOSE":
		return b.handleCLOSE(ctx, session, reqEvent, reqMsg)
	case "COUNT":
		return b.handleCOUNT(ctx, session, reqEvent, reqMsg, localConn)
	case "IDS":
		return b.handleIDS(ctx, session, reqEvent, reqMsg, localConn)
	default:
		return fmt.Errorf("unsupported message type: %s", reqMsg.Type)
	}
}

// handleREQ handles a REQ message and forwards responses.
func (b *Bridge) handleREQ(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage, conn *ws.Client) error {
	// Extract subscription ID and filters from payload
	// Payload: ["REQ", "<sub_id>", filter1, filter2, ...]
	if len(reqMsg.Payload) < 3 {
		return fmt.Errorf("invalid REQ payload")
	}
	subID, ok := reqMsg.Payload[1].(string)
	if !ok {
		return fmt.Errorf("invalid subscription ID")
	}

	// Parse filters from payload
	var filters []*filter.F
	for i := 2; i < len(reqMsg.Payload); i++ {
		filterMap, ok := reqMsg.Payload[i].(map[string]any)
		if !ok {
			continue
		}
		filterBytes, err := json.Marshal(filterMap)
		if err != nil {
			continue
		}
		var f filter.F
		if err := json.Unmarshal(filterBytes, &f); err != nil {
			continue
		}
		filters = append(filters, &f)
	}

	if len(filters) == 0 {
		return fmt.Errorf("no valid filters in REQ")
	}

	// Add subscription to session
	if err := session.AddSubscription(subID); err != nil {
		return err
	}

	// Create filter set
	filterSet := filter.NewS(filters...)

	// Subscribe to local relay
	sub, err := conn.Subscribe(ctx, filterSet)
	if chk.E(err) {
		session.RemoveSubscription(subID)
		return fmt.Errorf("local subscribe failed: %w", err)
	}
	defer sub.Unsub()

	// Forward events until EOSE or timeout
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-sub.Events:
			if ev == nil {
				// Subscription closed, send EOSE
				resp := &ResponseMessage{
					Type:    "EOSE",
					Payload: []any{"EOSE", subID},
				}
				return b.sendResponse(ctx, reqEvent, session, resp)
			}

			// Convert event to JSON-compatible map
			eventBytes, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			var eventMap map[string]any
			if err := json.Unmarshal(eventBytes, &eventMap); err != nil {
				continue
			}

			// Send EVENT response
			resp := &ResponseMessage{
				Type:    "EVENT",
				Payload: []any{"EVENT", subID, eventMap},
			}
			if err := b.sendResponse(ctx, reqEvent, session, resp); err != nil {
				log.W.F("failed to send event response: %v", err)
			}
			session.IncrementEventCount(subID)
		case <-sub.EndOfStoredEvents:
			// Send EOSE
			session.MarkEOSE(subID)
			resp := &ResponseMessage{
				Type:    "EOSE",
				Payload: []any{"EOSE", subID},
			}
			return b.sendResponse(ctx, reqEvent, session, resp)
		}
	}
}

// handleEVENT handles an EVENT message and forwards the OK response.
func (b *Bridge) handleEVENT(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage, conn *ws.Client) error {
	// Extract event from payload: ["EVENT", {...event...}]
	if len(reqMsg.Payload) < 2 {
		return fmt.Errorf("invalid EVENT payload")
	}

	eventMap, ok := reqMsg.Payload[1].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid event data")
	}

	// Parse event
	eventBytes, err := json.Marshal(eventMap)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	var ev event.E
	if err := json.Unmarshal(eventBytes, &ev); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Publish to local relay
	err = conn.Publish(ctx, &ev)
	success := err == nil
	message := ""
	if err != nil {
		message = err.Error()
	}

	// Send OK response
	resp := &ResponseMessage{
		Type:    "OK",
		Payload: []any{"OK", string(hex.Enc(ev.ID[:])), success, message},
	}
	return b.sendResponse(ctx, reqEvent, session, resp)
}

// handleCLOSE handles a CLOSE message.
func (b *Bridge) handleCLOSE(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage) error {
	// Extract subscription ID: ["CLOSE", "<sub_id>"]
	if len(reqMsg.Payload) >= 2 {
		if subID, ok := reqMsg.Payload[1].(string); ok {
			session.RemoveSubscription(subID)
		}
	}
	// CLOSE doesn't have a response
	return nil
}

// handleCOUNT handles a COUNT message.
func (b *Bridge) handleCOUNT(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage, conn *ws.Client) error {
	// COUNT is not supported via ws.Client directly, return error
	resp := &ResponseMessage{
		Type:    "NOTICE",
		Payload: []any{"NOTICE", "COUNT not supported through NRC tunnel"},
	}
	return b.sendResponse(ctx, reqEvent, session, resp)
}

// handleIDS handles an IDS message - returns event manifests for diffing.
func (b *Bridge) handleIDS(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage, conn *ws.Client) error {
	// Extract subscription ID and filters from payload
	// Payload: ["IDS", "<sub_id>", filter1, filter2, ...]
	if len(reqMsg.Payload) < 3 {
		return fmt.Errorf("invalid IDS payload")
	}
	subID, ok := reqMsg.Payload[1].(string)
	if !ok {
		return fmt.Errorf("invalid subscription ID")
	}

	// Parse filters from payload
	var filters []*filter.F
	for i := 2; i < len(reqMsg.Payload); i++ {
		filterMap, ok := reqMsg.Payload[i].(map[string]any)
		if !ok {
			continue
		}
		filterBytes, err := json.Marshal(filterMap)
		if err != nil {
			continue
		}
		var f filter.F
		if err := json.Unmarshal(filterBytes, &f); err != nil {
			continue
		}
		filters = append(filters, &f)
	}

	if len(filters) == 0 {
		return fmt.Errorf("no valid filters in IDS")
	}

	// Add subscription to session
	if err := session.AddSubscription(subID); err != nil {
		return err
	}
	defer session.RemoveSubscription(subID)

	// Create filter set
	filterSet := filter.NewS(filters...)

	// Subscribe to local relay
	sub, err := conn.Subscribe(ctx, filterSet)
	if chk.E(err) {
		return fmt.Errorf("local subscribe failed: %w", err)
	}
	defer sub.Unsub()

	// Collect events and build manifest
	var manifest []EventManifestEntry
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-sub.Events:
			if ev == nil {
				// Subscription closed, send IDS response
				return b.sendIDSResponse(ctx, reqEvent, session, subID, manifest)
			}

			// Build manifest entry
			entry := EventManifestEntry{
				Kind:      int(ev.Kind),
				ID:        string(hex.Enc(ev.ID[:])),
				CreatedAt: ev.CreatedAt,
			}

			// Check for d tag (parameterized replaceable events)
			dTag := ev.Tags.GetFirst([]byte("d"))
			if dTag != nil && dTag.Len() >= 2 {
				entry.D = string(dTag.Value())
			}

			manifest = append(manifest, entry)
		case <-sub.EndOfStoredEvents:
			// Send IDS response with manifest
			return b.sendIDSResponse(ctx, reqEvent, session, subID, manifest)
		}
	}
}

// sendIDSResponse sends an IDS response with the event manifest, chunking if necessary.
func (b *Bridge) sendIDSResponse(ctx context.Context, reqEvent *event.E, session *Session, subID string, manifest []EventManifestEntry) error {
	resp := &ResponseMessage{
		Type:    "IDS",
		Payload: []any{"IDS", subID, manifest},
	}
	return b.sendResponseChunked(ctx, reqEvent, session, resp)
}

// sendResponseChunked sends a response, chunking if necessary for large payloads.
func (b *Bridge) sendResponseChunked(ctx context.Context, reqEvent *event.E, session *Session, resp *ResponseMessage) error {
	// Marshal response content
	content, err := MarshalResponseContent(resp)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// If small enough, send directly
	if len(content) <= MaxChunkSize {
		return b.sendResponse(ctx, reqEvent, session, resp)
	}

	// Need to chunk - encode to base64 for safe transmission
	encoded := base64.StdEncoding.EncodeToString(content)
	var chunks []string

	// Split into chunks
	for i := 0; i < len(encoded); i += MaxChunkSize {
		end := i + MaxChunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunks = append(chunks, encoded[i:end])
	}

	// Generate message ID
	messageID := generateMessageID()
	log.D.F("NRC: chunking large message (%d bytes) into %d chunks", len(content), len(chunks))

	// Send each chunk
	for i, chunkData := range chunks {
		chunkMsg := ChunkMessage{
			Type:      "CHUNK",
			MessageID: messageID,
			Index:     i,
			Total:     len(chunks),
			Data:      chunkData,
		}

		chunkResp := &ResponseMessage{
			Type:    "CHUNK",
			Payload: []any{chunkMsg},
		}

		if err := b.sendResponse(ctx, reqEvent, session, chunkResp); err != nil {
			return fmt.Errorf("failed to send chunk %d/%d: %w", i+1, len(chunks), err)
		}
	}

	return nil
}

// generateMessageID generates a random message ID for chunking.
func generateMessageID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return string(hex.Enc(b))
}

// sendResponse encrypts and sends a response to the client.
func (b *Bridge) sendResponse(ctx context.Context, reqEvent *event.E, session *Session, resp *ResponseMessage) error {
	// Marshal response content
	content, err := MarshalResponseContent(resp)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	// Encrypt content
	encrypted, err := encryption.Encrypt(session.ConversationKey, content, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	// Build response event
	respEvent := &event.E{
		Content:   []byte(encrypted),
		CreatedAt: time.Now().Unix(),
		Kind:      KindNRCResponse,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(reqEvent.Pubkey[:])),
			tag.NewFromAny("encryption", "nip44_v2"),
			tag.NewFromAny("session", session.ID),
			tag.NewFromAny("e", hex.Enc(reqEvent.ID[:])),
		),
	}

	// Sign with relay key
	if err := respEvent.Sign(b.config.Signer); chk.E(err) {
		return fmt.Errorf("signing failed: %w", err)
	}

	// Publish to rendezvous relay
	b.mu.RLock()
	conn := b.rendezvousConn
	b.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected to rendezvous relay")
	}

	if err := conn.Publish(ctx, respEvent); chk.E(err) {
		return fmt.Errorf("publish failed: %w", err)
	}

	return nil
}

// sendError sends an error response to the client.
func (b *Bridge) sendError(ctx context.Context, reqEvent *event.E, sessionID string, errMsg string) {
	// For errors, we need to get or create a conversation key
	// This is best-effort since we may not be able to authenticate
	conversationKey, err := encryption.GenerateConversationKey(
		b.config.Signer.Sec(),
		reqEvent.Pubkey[:],
	)
	if err != nil {
		log.W.F("failed to generate conversation key for error response: %v", err)
		return
	}

	resp := &ResponseMessage{
		Type:    "NOTICE",
		Payload: []any{"NOTICE", "nrc: " + errMsg},
	}

	content, err := MarshalResponseContent(resp)
	if err != nil {
		return
	}

	encrypted, err := encryption.Encrypt(conversationKey, content, nil)
	if err != nil {
		return
	}

	respEvent := &event.E{
		Content:   []byte(encrypted),
		CreatedAt: time.Now().Unix(),
		Kind:      KindNRCResponse,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(reqEvent.Pubkey[:])),
			tag.NewFromAny("encryption", "nip44_v2"),
			tag.NewFromAny("session", sessionID),
			tag.NewFromAny("e", hex.Enc(reqEvent.ID[:])),
		),
	}

	if err := respEvent.Sign(b.config.Signer); err != nil {
		return
	}

	b.mu.RLock()
	conn := b.rendezvousConn
	b.mu.RUnlock()

	if conn != nil {
		conn.Publish(ctx, respEvent)
	}
}

// AddAuthorizedSecret adds an authorized secret (derived pubkey).
func (b *Bridge) AddAuthorizedSecret(pubkeyHex, deviceName string) {
	b.config.AuthorizedSecrets[pubkeyHex] = deviceName
}

// RemoveAuthorizedSecret removes an authorized secret.
func (b *Bridge) RemoveAuthorizedSecret(pubkeyHex string) {
	delete(b.config.AuthorizedSecrets, pubkeyHex)
}

// ListAuthorizedSecrets returns a copy of the authorized secrets map.
func (b *Bridge) ListAuthorizedSecrets() map[string]string {
	result := make(map[string]string)
	for k, v := range b.config.AuthorizedSecrets {
		result[k] = v
	}
	return result
}

// SessionCount returns the number of active sessions.
func (b *Bridge) SessionCount() int {
	return b.sessions.Count()
}
