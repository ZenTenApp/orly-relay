package nrc

import (
	"context"
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
	"github.com/google/uuid"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// chunkBuffer holds chunks for a message being reassembled.
type chunkBuffer struct {
	chunks     map[int]string
	total      int
	receivedAt time.Time
}

// Client connects to a private relay through the NRC tunnel.
type Client struct {
	uri             *ConnectionURI
	sessionID       string
	rendezvousConn  *ws.Client
	responseSub     *ws.Subscription
	conversationKey []byte
	clientSigner    signer.I

	// pending maps request event IDs to response channels.
	pending   map[string]chan *ResponseMessage
	pendingMu sync.Mutex

	// subscriptions maps subscription IDs to event channels.
	subscriptions   map[string]chan *event.E
	subscriptionsMu sync.Mutex

	// chunkBuffers holds partially received chunked messages.
	chunkBuffers   map[string]*chunkBuffer
	chunkBuffersMu sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc
}

// NewClient creates a new NRC client from a connection URI.
func NewClient(connectionURI string) (*Client, error) {
	uri, err := ParseConnectionURI(connectionURI)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %w", err)
	}

	if uri.AuthMode != AuthModeSecret {
		return nil, fmt.Errorf("CAT authentication not yet supported in client")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		uri:             uri,
		sessionID:       uuid.New().String(),
		conversationKey: uri.GetConversationKey(),
		clientSigner:    uri.GetClientSigner(),
		pending:         make(map[string]chan *ResponseMessage),
		subscriptions:   make(map[string]chan *event.E),
		chunkBuffers:    make(map[string]*chunkBuffer),
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

// Connect establishes the connection to the rendezvous relay.
func (c *Client) Connect(ctx context.Context) error {
	// Connect to rendezvous relay
	conn, err := ws.RelayConnect(ctx, c.uri.RendezvousRelay)
	if chk.E(err) {
		return fmt.Errorf("%w: %v", ErrRendezvousConnectionFailed, err)
	}
	c.rendezvousConn = conn

	// Subscribe to response events
	clientPubkeyHex := hex.Enc(c.clientSigner.Pub())
	sub, err := conn.Subscribe(
		ctx,
		filter.NewS(&filter.F{
			Kinds: kind.NewS(kind.New(KindNRCResponse)),
			Tags: tag.NewS(
				tag.NewFromAny("p", clientPubkeyHex),
			),
			Since: &timestamp.T{V: time.Now().Unix()},
		}),
	)
	if chk.E(err) {
		conn.Close()
		return fmt.Errorf("subscription failed: %w", err)
	}
	c.responseSub = sub

	// Start response handler
	go c.handleResponses()

	log.I.F("NRC client connected to %s via %s",
		hex.Enc(c.uri.RelayPubkey), c.uri.RendezvousRelay)

	return nil
}

// Close closes the client connection.
func (c *Client) Close() {
	c.cancel()
	if c.responseSub != nil {
		c.responseSub.Unsub()
	}
	if c.rendezvousConn != nil {
		c.rendezvousConn.Close()
	}

	// Close all pending channels
	c.pendingMu.Lock()
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[string]chan *ResponseMessage)
	c.pendingMu.Unlock()

	// Close all subscription channels
	c.subscriptionsMu.Lock()
	for _, ch := range c.subscriptions {
		close(ch)
	}
	c.subscriptions = make(map[string]chan *event.E)
	c.subscriptionsMu.Unlock()

	// Clear chunk buffers
	c.chunkBuffersMu.Lock()
	c.chunkBuffers = make(map[string]*chunkBuffer)
	c.chunkBuffersMu.Unlock()
}

// handleResponses processes incoming NRC response events.
func (c *Client) handleResponses() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case ev := <-c.responseSub.Events:
			if ev == nil {
				return
			}
			c.processResponse(ev)
		}
	}
}

// processResponse decrypts and routes a response event.
func (c *Client) processResponse(ev *event.E) {
	// Decrypt content
	decrypted, err := encryption.Decrypt(c.conversationKey, string(ev.Content))
	if err != nil {
		log.W.F("NRC response decryption failed: %v", err)
		return
	}

	// Parse response
	var resp struct {
		Type    string `json:"type"`
		Payload []any  `json:"payload"`
	}
	if err := json.Unmarshal([]byte(decrypted), &resp); err != nil {
		log.W.F("NRC response parse failed: %v", err)
		return
	}

	// Extract request event ID for routing
	var requestEventID string
	eTag := ev.Tags.GetFirst([]byte("e"))
	if eTag != nil && eTag.Len() >= 2 {
		requestEventID = string(eTag.ValueHex())
	}

	// Route based on response type
	switch resp.Type {
	case "EVENT":
		c.handleEventResponse(resp.Payload)
	case "EOSE":
		c.handleEOSEResponse(resp.Payload, requestEventID)
	case "OK":
		c.handleOKResponse(resp.Payload, requestEventID)
	case "NOTICE":
		c.handleNoticeResponse(resp.Payload)
	case "CLOSED":
		c.handleClosedResponse(resp.Payload)
	case "COUNT":
		c.handleCountResponse(resp.Payload, requestEventID)
	case "AUTH":
		c.handleAuthResponse(resp.Payload, requestEventID)
	case "IDS":
		c.handleIDSResponse(resp.Payload, requestEventID)
	case "CHUNK":
		c.handleChunkResponse(resp.Payload, requestEventID)
	}
}

// handleEventResponse routes an EVENT to the appropriate subscription.
func (c *Client) handleEventResponse(payload []any) {
	if len(payload) < 3 {
		return
	}
	// Payload: ["EVENT", "<sub_id>", {...event...}]
	subID, ok := payload[1].(string)
	if !ok {
		return
	}

	c.subscriptionsMu.Lock()
	ch, exists := c.subscriptions[subID]
	c.subscriptionsMu.Unlock()

	if !exists {
		return
	}

	// Parse event from payload
	eventData, ok := payload[2].(map[string]any)
	if !ok {
		return
	}

	eventBytes, err := json.Marshal(eventData)
	if err != nil {
		return
	}

	var ev event.E
	if err := json.Unmarshal(eventBytes, &ev); err != nil {
		return
	}

	select {
	case ch <- &ev:
	default:
		// Channel full, drop event
	}
}

// handleEOSEResponse handles an EOSE response.
func (c *Client) handleEOSEResponse(payload []any, requestEventID string) {
	// Route to pending request
	c.pendingMu.Lock()
	ch, exists := c.pending[requestEventID]
	c.pendingMu.Unlock()

	if exists {
		resp := &ResponseMessage{Type: "EOSE", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleOKResponse handles an OK response.
func (c *Client) handleOKResponse(payload []any, requestEventID string) {
	c.pendingMu.Lock()
	ch, exists := c.pending[requestEventID]
	c.pendingMu.Unlock()

	if exists {
		resp := &ResponseMessage{Type: "OK", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleNoticeResponse logs a NOTICE.
func (c *Client) handleNoticeResponse(payload []any) {
	if len(payload) >= 2 {
		if msg, ok := payload[1].(string); ok {
			log.W.F("NRC NOTICE: %s", msg)
		}
	}
}

// handleClosedResponse handles a subscription close.
func (c *Client) handleClosedResponse(payload []any) {
	if len(payload) >= 2 {
		if subID, ok := payload[1].(string); ok {
			c.subscriptionsMu.Lock()
			if ch, exists := c.subscriptions[subID]; exists {
				close(ch)
				delete(c.subscriptions, subID)
			}
			c.subscriptionsMu.Unlock()
		}
	}
}

// handleCountResponse handles a COUNT response.
func (c *Client) handleCountResponse(payload []any, requestEventID string) {
	c.pendingMu.Lock()
	ch, exists := c.pending[requestEventID]
	c.pendingMu.Unlock()

	if exists {
		resp := &ResponseMessage{Type: "COUNT", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleAuthResponse handles an AUTH challenge.
func (c *Client) handleAuthResponse(payload []any, requestEventID string) {
	c.pendingMu.Lock()
	ch, exists := c.pending[requestEventID]
	c.pendingMu.Unlock()

	if exists {
		resp := &ResponseMessage{Type: "AUTH", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleIDSResponse handles an IDS response.
func (c *Client) handleIDSResponse(payload []any, requestEventID string) {
	c.pendingMu.Lock()
	ch, exists := c.pending[requestEventID]
	c.pendingMu.Unlock()

	if exists {
		resp := &ResponseMessage{Type: "IDS", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleChunkResponse handles a CHUNK response and reassembles the message.
func (c *Client) handleChunkResponse(payload []any, requestEventID string) {
	if len(payload) < 1 {
		return
	}

	// Parse chunk message from payload
	chunkData, ok := payload[0].(map[string]any)
	if !ok {
		log.W.F("NRC: invalid chunk payload format")
		return
	}

	messageID, _ := chunkData["messageId"].(string)
	indexFloat, _ := chunkData["index"].(float64)
	totalFloat, _ := chunkData["total"].(float64)
	data, _ := chunkData["data"].(string)

	if messageID == "" || data == "" {
		log.W.F("NRC: chunk missing required fields")
		return
	}

	index := int(indexFloat)
	total := int(totalFloat)

	c.chunkBuffersMu.Lock()
	defer c.chunkBuffersMu.Unlock()

	// Get or create buffer for this message
	buf, exists := c.chunkBuffers[messageID]
	if !exists {
		buf = &chunkBuffer{
			chunks:     make(map[int]string),
			total:      total,
			receivedAt: time.Now(),
		}
		c.chunkBuffers[messageID] = buf
	}

	// Store the chunk
	buf.chunks[index] = data
	log.D.F("NRC: received chunk %d/%d for message %s", index+1, total, messageID[:8])

	// Check if we have all chunks
	if len(buf.chunks) == buf.total {
		// Reassemble the message
		var encoded string
		for i := 0; i < buf.total; i++ {
			part, ok := buf.chunks[i]
			if !ok {
				log.W.F("NRC: missing chunk %d for message %s", i, messageID)
				delete(c.chunkBuffers, messageID)
				return
			}
			encoded += part
		}

		// Decode from base64
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			log.W.F("NRC: failed to decode chunked message: %v", err)
			delete(c.chunkBuffers, messageID)
			return
		}

		// Parse the reassembled response
		var resp struct {
			Type    string `json:"type"`
			Payload []any  `json:"payload"`
		}
		if err := json.Unmarshal(decoded, &resp); err != nil {
			log.W.F("NRC: failed to parse reassembled message: %v", err)
			delete(c.chunkBuffers, messageID)
			return
		}

		log.D.F("NRC: reassembled chunked message: %s", resp.Type)

		// Clean up buffer
		delete(c.chunkBuffers, messageID)

		// Route the reassembled response
		c.pendingMu.Lock()
		ch, exists := c.pending[requestEventID]
		c.pendingMu.Unlock()

		if exists {
			respMsg := &ResponseMessage{Type: resp.Type, Payload: resp.Payload}
			select {
			case ch <- respMsg:
			default:
			}
		}
	}

	// Clean up stale buffers (older than 60 seconds)
	now := time.Now()
	for id, b := range c.chunkBuffers {
		if now.Sub(b.receivedAt) > 60*time.Second {
			log.W.F("NRC: discarding stale chunk buffer: %s", id)
			delete(c.chunkBuffers, id)
		}
	}
}

// sendRequest sends an NRC request and waits for response.
func (c *Client) sendRequest(ctx context.Context, msgType string, payload []any) (*ResponseMessage, error) {
	// Build request content
	reqContent := struct {
		Type    string `json:"type"`
		Payload []any  `json:"payload"`
	}{
		Type:    msgType,
		Payload: payload,
	}

	contentBytes, err := json.Marshal(reqContent)
	if err != nil {
		return nil, fmt.Errorf("marshal failed: %w", err)
	}

	// Encrypt content
	encrypted, err := encryption.Encrypt(c.conversationKey, contentBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	// Build request event
	reqEvent := &event.E{
		Content:   []byte(encrypted),
		CreatedAt: time.Now().Unix(),
		Kind:      KindNRCRequest,
		Tags: tag.NewS(
			tag.NewFromAny("p", hex.Enc(c.uri.RelayPubkey)),
			tag.NewFromAny("encryption", "nip44_v2"),
			tag.NewFromAny("session", c.sessionID),
		),
	}

	// Sign with client key
	if err := reqEvent.Sign(c.clientSigner); chk.E(err) {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	// Set up response channel
	responseCh := make(chan *ResponseMessage, 1)
	requestEventID := string(hex.Enc(reqEvent.ID[:]))

	c.pendingMu.Lock()
	c.pending[requestEventID] = responseCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, requestEventID)
		c.pendingMu.Unlock()
	}()

	// Publish request
	if err := c.rendezvousConn.Publish(ctx, reqEvent); chk.E(err) {
		return nil, fmt.Errorf("publish failed: %w", err)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-responseCh:
		if resp == nil {
			return nil, fmt.Errorf("response channel closed")
		}
		return resp, nil
	}
}

// Publish publishes an event to the private relay.
func (c *Client) Publish(ctx context.Context, ev *event.E) (bool, string, error) {
	// Convert event to JSON for payload
	eventBytes, err := json.Marshal(ev)
	if err != nil {
		return false, "", fmt.Errorf("marshal event failed: %w", err)
	}

	var eventMap map[string]any
	if err := json.Unmarshal(eventBytes, &eventMap); err != nil {
		return false, "", fmt.Errorf("unmarshal event failed: %w", err)
	}

	payload := []any{"EVENT", eventMap}

	resp, err := c.sendRequest(ctx, "EVENT", payload)
	if err != nil {
		return false, "", err
	}

	// Parse OK response: ["OK", "<event_id>", <success>, "<message>"]
	if resp.Type != "OK" || len(resp.Payload) < 4 {
		return false, "", fmt.Errorf("unexpected response type: %s", resp.Type)
	}

	success, _ := resp.Payload[2].(bool)
	message, _ := resp.Payload[3].(string)

	return success, message, nil
}

// Subscribe creates a subscription to the private relay.
func (c *Client) Subscribe(ctx context.Context, subID string, filters ...*filter.F) (<-chan *event.E, error) {
	// Build payload: ["REQ", "<sub_id>", filter1, filter2, ...]
	payload := []any{"REQ", subID}
	for _, f := range filters {
		filterBytes, err := json.Marshal(f)
		if err != nil {
			return nil, fmt.Errorf("marshal filter failed: %w", err)
		}
		var filterMap map[string]any
		if err := json.Unmarshal(filterBytes, &filterMap); err != nil {
			return nil, fmt.Errorf("unmarshal filter failed: %w", err)
		}
		payload = append(payload, filterMap)
	}

	// Create event channel for this subscription
	eventCh := make(chan *event.E, 100)

	c.subscriptionsMu.Lock()
	c.subscriptions[subID] = eventCh
	c.subscriptionsMu.Unlock()

	// Send request (don't wait for EOSE, events will come asynchronously)
	go func() {
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		_, err := c.sendRequest(reqCtx, "REQ", payload)
		if err != nil {
			log.W.F("NRC subscribe failed: %v", err)
		}
	}()

	return eventCh, nil
}

// Unsubscribe closes a subscription.
func (c *Client) Unsubscribe(ctx context.Context, subID string) error {
	// Remove from local tracking
	c.subscriptionsMu.Lock()
	if ch, exists := c.subscriptions[subID]; exists {
		close(ch)
		delete(c.subscriptions, subID)
	}
	c.subscriptionsMu.Unlock()

	// Send CLOSE to relay
	payload := []any{"CLOSE", subID}
	_, err := c.sendRequest(ctx, "CLOSE", payload)
	return err
}

// Count sends a COUNT request to the private relay.
func (c *Client) Count(ctx context.Context, subID string, filters ...*filter.F) (int64, error) {
	// Build payload: ["COUNT", "<sub_id>", filter1, filter2, ...]
	payload := []any{"COUNT", subID}
	for _, f := range filters {
		filterBytes, err := json.Marshal(f)
		if err != nil {
			return 0, fmt.Errorf("marshal filter failed: %w", err)
		}
		var filterMap map[string]any
		if err := json.Unmarshal(filterBytes, &filterMap); err != nil {
			return 0, fmt.Errorf("unmarshal filter failed: %w", err)
		}
		payload = append(payload, filterMap)
	}

	resp, err := c.sendRequest(ctx, "COUNT", payload)
	if err != nil {
		return 0, err
	}

	// Parse COUNT response: ["COUNT", "<sub_id>", {"count": N}]
	if resp.Type != "COUNT" || len(resp.Payload) < 3 {
		return 0, fmt.Errorf("unexpected response type: %s", resp.Type)
	}

	countData, ok := resp.Payload[2].(map[string]any)
	if !ok {
		return 0, fmt.Errorf("invalid count response")
	}

	count, ok := countData["count"].(float64)
	if !ok {
		return 0, fmt.Errorf("missing count field")
	}

	return int64(count), nil
}

// RelayURL returns a pseudo-URL for this NRC connection.
func (c *Client) RelayURL() string {
	return "nrc://" + string(hex.Enc(c.uri.RelayPubkey))
}

// RequestIDs sends an IDS request to get event manifests for diffing.
func (c *Client) RequestIDs(ctx context.Context, subID string, filters ...*filter.F) ([]EventManifestEntry, error) {
	// Build payload: ["IDS", "<sub_id>", filter1, filter2, ...]
	payload := []any{"IDS", subID}
	for _, f := range filters {
		filterBytes, err := json.Marshal(f)
		if err != nil {
			return nil, fmt.Errorf("marshal filter failed: %w", err)
		}
		var filterMap map[string]any
		if err := json.Unmarshal(filterBytes, &filterMap); err != nil {
			return nil, fmt.Errorf("unmarshal filter failed: %w", err)
		}
		payload = append(payload, filterMap)
	}

	resp, err := c.sendRequest(ctx, "IDS", payload)
	if err != nil {
		return nil, err
	}

	// Parse IDS response: ["IDS", "<sub_id>", [...manifest...]]
	if resp.Type != "IDS" || len(resp.Payload) < 3 {
		return nil, fmt.Errorf("unexpected response type: %s", resp.Type)
	}

	// Parse manifest entries
	manifestData, ok := resp.Payload[2].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid manifest response")
	}

	var manifest []EventManifestEntry
	for _, item := range manifestData {
		entryMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		entry := EventManifestEntry{}
		if k, ok := entryMap["kind"].(float64); ok {
			entry.Kind = int(k)
		}
		if id, ok := entryMap["id"].(string); ok {
			entry.ID = id
		}
		if ca, ok := entryMap["created_at"].(float64); ok {
			entry.CreatedAt = int64(ca)
		}
		if d, ok := entryMap["d"].(string); ok {
			entry.D = d
		}
		manifest = append(manifest, entry)
	}

	return manifest, nil
}
