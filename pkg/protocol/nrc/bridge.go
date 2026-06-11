package nrc

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
type NRCAuthorizer interface {
	GetNRCClientByPubkey(derivedPubkey []byte) (id string, label string, found bool, err error)
	UpdateNRCClientLastUsed(id string) error
}

// BridgeConfig holds configuration for the NRC bridge.
type BridgeConfig struct {
	RendezvousURL     string
	LocalRelayURL     string
	Signer            signer.I
	AuthorizedSecrets map[string]string
	Authorizer        NRCAuthorizer
	SessionTimeout    time.Duration
}

// --- actor request types for Bridge connection state ---

type bridgeGetConnReq struct {
	resp chan *ws.Client
}

type bridgeSetConnReq struct {
	conn *ws.Client
}

type bridgeUpdateSecretsReq struct {
	secrets map[string]string
	resp    chan struct{}
}

type bridgeAddSecretReq struct {
	pubkeyHex  string
	deviceName string
}

type bridgeRemoveSecretReq struct {
	pubkeyHex string
}

type bridgeListSecretsReq struct {
	resp chan map[string]string
}

// Bridge connects a private relay to a public rendezvous relay.
type Bridge struct {
	config   *BridgeConfig
	sessions *SessionManager

	// actor channels for connection state
	getConnCh       chan bridgeGetConnReq
	setConnCh       chan bridgeSetConnReq
	updateSecretsCh chan bridgeUpdateSecretsReq
	addSecretCh     chan bridgeAddSecretReq
	removeSecretCh  chan bridgeRemoveSecretReq
	listSecretsCh   chan bridgeListSecretsReq

	// ctx is the bridge context.
	ctx    context.Context
	cancel context.CancelFunc

	stop chan struct{}
	done chan struct{}
}

// NewBridge creates a new NRC bridge.
func NewBridge(config *BridgeConfig) *Bridge {
	ctx, cancel := context.WithCancel(context.Background())
	timeout := config.SessionTimeout
	if timeout == 0 {
		timeout = DefaultSessionTimeout
	}
	b := &Bridge{
		config:   config,
		sessions: NewSessionManager(timeout),

		getConnCh:       make(chan bridgeGetConnReq),
		setConnCh:       make(chan bridgeSetConnReq, 16),
		updateSecretsCh: make(chan bridgeUpdateSecretsReq),
		addSecretCh:     make(chan bridgeAddSecretReq, 16),
		removeSecretCh:  make(chan bridgeRemoveSecretReq, 16),
		listSecretsCh:   make(chan bridgeListSecretsReq),

		ctx:    ctx,
		cancel: cancel,

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go b.runActor()
	return b
}

func (b *Bridge) runActor() {
	defer close(b.done)

	var rendezvousConn *ws.Client

	for {
		select {
		case <-b.stop:
			return
		case req := <-b.getConnCh:
			req.resp <- rendezvousConn
		case req := <-b.setConnCh:
			rendezvousConn = req.conn
		case req := <-b.updateSecretsCh:
			b.config.AuthorizedSecrets = req.secrets
			req.resp <- struct{}{}
		case req := <-b.addSecretCh:
			b.config.AuthorizedSecrets[req.pubkeyHex] = req.deviceName
		case req := <-b.removeSecretCh:
			delete(b.config.AuthorizedSecrets, req.pubkeyHex)
		case req := <-b.listSecretsCh:
			result := make(map[string]string)
			for k, v := range b.config.AuthorizedSecrets {
				result[k] = v
			}
			req.resp <- result
		}
	}
}

func (b *Bridge) getConn() *ws.Client {
	resp := make(chan *ws.Client, 1)
	select {
	case b.getConnCh <- bridgeGetConnReq{resp: resp}:
		return <-resp
	case <-b.stop:
		return nil
	}
}

func (b *Bridge) setConn(conn *ws.Client) {
	select {
	case b.setConnCh <- bridgeSetConnReq{conn: conn}:
	case <-b.stop:
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

	conn := b.getConn()
	if conn != nil {
		conn.Close()
	}
	close(b.stop)
	<-b.done
}

// UpdateAuthorizedSecrets updates the map of authorized secrets.
func (b *Bridge) UpdateAuthorizedSecrets(secrets map[string]string) {
	resp := make(chan struct{}, 1)
	select {
	case b.updateSecretsCh <- bridgeUpdateSecretsReq{secrets: secrets, resp: resp}:
		<-resp
	case <-b.stop:
	}
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

	b.setConn(rendezvousConn)

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
			conversationKey, err = encryption.GenerateConversationKey(
				b.config.Signer.Sec(),
				clientPubkey,
			)
			if chk.E(err) {
				return
			}
			authMode = AuthModeSecret
			deviceName = clientLabel

			go func() {
				if updateErr := b.config.Authorizer.UpdateNRCClientLastUsed(clientID); updateErr != nil {
					log.W.F("failed to update NRC client last used: %v", updateErr)
				}
			}()
			return
		}
	}

	// Fallback to static map - read via actor
	secrets := b.ListAuthorizedSecrets()
	if name, ok := secrets[clientPubkeyHex]; ok {
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
	localConn, err := ws.RelayConnect(ctx, b.config.LocalRelayURL)
	if chk.E(err) {
		return fmt.Errorf("%w: %v", ErrRelayConnectionFailed, err)
	}
	defer localConn.Close()

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
	if len(reqMsg.Payload) < 3 {
		return fmt.Errorf("invalid REQ payload")
	}
	subID, ok := reqMsg.Payload[1].(string)
	if !ok {
		return fmt.Errorf("invalid subscription ID")
	}

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

	if err := session.AddSubscription(subID); err != nil {
		return err
	}

	filterSet := filter.NewS(filters...)
	sub, err := conn.Subscribe(ctx, filterSet)
	if chk.E(err) {
		session.RemoveSubscription(subID)
		return fmt.Errorf("local subscribe failed: %w", err)
	}
	defer sub.Unsub()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-sub.Events:
			if ev == nil {
				resp := &ResponseMessage{
					Type:    "EOSE",
					Payload: []any{"EOSE", subID},
				}
				return b.sendResponse(ctx, reqEvent, session, resp)
			}

			eventBytes, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			var eventMap map[string]any
			if err := json.Unmarshal(eventBytes, &eventMap); err != nil {
				continue
			}

			resp := &ResponseMessage{
				Type:    "EVENT",
				Payload: []any{"EVENT", subID, eventMap},
			}
			if err := b.sendResponse(ctx, reqEvent, session, resp); err != nil {
				log.W.F("failed to send event response: %v", err)
			}
			session.IncrementEventCount(subID)
		case <-sub.EndOfStoredEvents:
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
	if len(reqMsg.Payload) < 2 {
		return fmt.Errorf("invalid EVENT payload")
	}

	eventMap, ok := reqMsg.Payload[1].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid event data")
	}

	eventBytes, err := json.Marshal(eventMap)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	var ev event.E
	if err := json.Unmarshal(eventBytes, &ev); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	err = conn.Publish(ctx, &ev)
	success := err == nil
	message := ""
	if err != nil {
		message = err.Error()
	}

	resp := &ResponseMessage{
		Type:    "OK",
		Payload: []any{"OK", string(hex.Enc(ev.ID[:])), success, message},
	}
	return b.sendResponse(ctx, reqEvent, session, resp)
}

// handleCLOSE handles a CLOSE message.
func (b *Bridge) handleCLOSE(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage) error {
	if len(reqMsg.Payload) >= 2 {
		if subID, ok := reqMsg.Payload[1].(string); ok {
			session.RemoveSubscription(subID)
		}
	}
	return nil
}

// handleCOUNT handles a COUNT message.
func (b *Bridge) handleCOUNT(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage, conn *ws.Client) error {
	resp := &ResponseMessage{
		Type:    "NOTICE",
		Payload: []any{"NOTICE", "COUNT not supported through NRC tunnel"},
	}
	return b.sendResponse(ctx, reqEvent, session, resp)
}

// handleIDS handles an IDS message - returns event manifests for diffing.
func (b *Bridge) handleIDS(ctx context.Context, session *Session, reqEvent *event.E, reqMsg *RequestMessage, conn *ws.Client) error {
	if len(reqMsg.Payload) < 3 {
		return fmt.Errorf("invalid IDS payload")
	}
	subID, ok := reqMsg.Payload[1].(string)
	if !ok {
		return fmt.Errorf("invalid subscription ID")
	}

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

	if err := session.AddSubscription(subID); err != nil {
		return err
	}
	defer session.RemoveSubscription(subID)

	filterSet := filter.NewS(filters...)
	sub, err := conn.Subscribe(ctx, filterSet)
	if chk.E(err) {
		return fmt.Errorf("local subscribe failed: %w", err)
	}
	defer sub.Unsub()

	var manifest []EventManifestEntry
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-sub.Events:
			if ev == nil {
				return b.sendIDSResponse(ctx, reqEvent, session, subID, manifest)
			}

			entry := EventManifestEntry{
				Kind:      int(ev.Kind),
				ID:        string(hex.Enc(ev.ID[:])),
				CreatedAt: ev.CreatedAt,
			}

			dTag := ev.Tags.GetFirst([]byte("d"))
			if dTag != nil && dTag.Len() >= 2 {
				entry.D = string(dTag.Value())
			}

			manifest = append(manifest, entry)
		case <-sub.EndOfStoredEvents:
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
	content, err := MarshalResponseContent(resp)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	if len(content) <= MaxChunkSize {
		return b.sendResponse(ctx, reqEvent, session, resp)
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	var chunks []string

	for i := 0; i < len(encoded); i += MaxChunkSize {
		end := i + MaxChunkSize
		if end > len(encoded) {
			end = len(encoded)
		}
		chunks = append(chunks, encoded[i:end])
	}

	messageID := generateMessageID()
	log.D.F("NRC: chunking large message (%d bytes) into %d chunks", len(content), len(chunks))

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
	content, err := MarshalResponseContent(resp)
	if err != nil {
		return fmt.Errorf("marshal failed: %w", err)
	}

	encrypted, err := encryption.Encrypt(session.ConversationKey, content, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

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

	if err := respEvent.Sign(b.config.Signer); chk.E(err) {
		return fmt.Errorf("signing failed: %w", err)
	}

	conn := b.getConn()
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

	conn := b.getConn()
	if conn != nil {
		conn.Publish(ctx, respEvent)
	}
}

// AddAuthorizedSecret adds an authorized secret (derived pubkey).
func (b *Bridge) AddAuthorizedSecret(pubkeyHex, deviceName string) {
	select {
	case b.addSecretCh <- bridgeAddSecretReq{pubkeyHex: pubkeyHex, deviceName: deviceName}:
	case <-b.stop:
	}
}

// RemoveAuthorizedSecret removes an authorized secret.
func (b *Bridge) RemoveAuthorizedSecret(pubkeyHex string) {
	select {
	case b.removeSecretCh <- bridgeRemoveSecretReq{pubkeyHex: pubkeyHex}:
	case <-b.stop:
	}
}

// ListAuthorizedSecrets returns a copy of the authorized secrets map.
func (b *Bridge) ListAuthorizedSecrets() map[string]string {
	resp := make(chan map[string]string, 1)
	select {
	case b.listSecretsCh <- bridgeListSecretsReq{resp: resp}:
		return <-resp
	case <-b.stop:
		return nil
	}
}

// SessionCount returns the number of active sessions.
func (b *Bridge) SessionCount() int {
	return b.sessions.Count()
}
