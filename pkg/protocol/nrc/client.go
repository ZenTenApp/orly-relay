package nrc

import (
	"context"
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

// --- actor request types for Client state ---

type clientRegisterPendingReq struct {
	eventID string
	ch      chan *ResponseMessage
}

type clientRemovePendingReq struct {
	eventID string
}

type clientGetPendingReq struct {
	eventID string
	resp    chan chan *ResponseMessage
}

type clientRegisterSubReq struct {
	subID string
	ch    chan *event.E
}

type clientGetSubReq struct {
	subID string
	resp  chan chan *event.E
}

type clientCloseSubReq struct {
	subID string
}

type clientChunkReq struct {
	messageID      string
	index          int
	total          int
	data           string
	requestEventID string
	resp           chan *clientChunkResp
}

type clientChunkResp struct {
	complete bool
	respMsg  *ResponseMessage
}

type clientCloseAllReq struct {
	resp chan struct{}
}

// Client connects to a private relay through the NRC tunnel.
type Client struct {
	uri             *ConnectionURI
	sessionID       string
	rendezvousConn  *ws.Client
	responseSub     *ws.Subscription
	conversationKey []byte
	clientSigner    signer.I

	// actor channels
	registerPendingCh chan clientRegisterPendingReq
	removePendingCh   chan clientRemovePendingReq
	getPendingCh      chan clientGetPendingReq
	registerSubCh     chan clientRegisterSubReq
	getSubCh          chan clientGetSubReq
	closeSubCh        chan clientCloseSubReq
	chunkCh           chan clientChunkReq
	closeAllCh        chan clientCloseAllReq

	ctx    context.Context
	cancel context.CancelFunc

	stop chan struct{}
	done chan struct{}
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
	c := &Client{
		uri:             uri,
		sessionID:       uuid.New().String(),
		conversationKey: uri.GetConversationKey(),
		clientSigner:    uri.GetClientSigner(),

		registerPendingCh: make(chan clientRegisterPendingReq, 16),
		removePendingCh:   make(chan clientRemovePendingReq, 16),
		getPendingCh:      make(chan clientGetPendingReq),
		registerSubCh:     make(chan clientRegisterSubReq, 16),
		getSubCh:          make(chan clientGetSubReq),
		closeSubCh:        make(chan clientCloseSubReq, 16),
		chunkCh:           make(chan clientChunkReq),
		closeAllCh:        make(chan clientCloseAllReq),

		ctx:    ctx,
		cancel: cancel,

		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	go c.runActor()
	return c, nil
}

func (c *Client) runActor() {
	defer close(c.done)

	pending := make(map[string]chan *ResponseMessage)
	subscriptions := make(map[string]chan *event.E)
	chunkBuffers := make(map[string]*chunkBuffer)

	for {
		select {
		case <-c.stop:
			// Close all pending channels
			for _, ch := range pending {
				close(ch)
			}
			// Close all subscription channels
			for _, ch := range subscriptions {
				close(ch)
			}
			return
		case req := <-c.registerPendingCh:
			pending[req.eventID] = req.ch
		case req := <-c.removePendingCh:
			delete(pending, req.eventID)
		case req := <-c.getPendingCh:
			ch, exists := pending[req.eventID]
			if exists {
				req.resp <- ch
			} else {
				req.resp <- nil
			}
		case req := <-c.registerSubCh:
			subscriptions[req.subID] = req.ch
		case req := <-c.getSubCh:
			ch, exists := subscriptions[req.subID]
			if exists {
				req.resp <- ch
			} else {
				req.resp <- nil
			}
		case req := <-c.closeSubCh:
			if ch, exists := subscriptions[req.subID]; exists {
				close(ch)
				delete(subscriptions, req.subID)
			}
		case req := <-c.chunkCh:
			buf, exists := chunkBuffers[req.messageID]
			if !exists {
				buf = &chunkBuffer{
					chunks:     make(map[int]string),
					total:      req.total,
					receivedAt: time.Now(),
				}
				chunkBuffers[req.messageID] = buf
			}
			buf.chunks[req.index] = req.data

			if len(buf.chunks) == buf.total {
				// Reassemble
				var encoded string
				complete := true
				for i := 0; i < buf.total; i++ {
					part, ok := buf.chunks[i]
					if !ok {
						complete = false
						break
					}
					encoded += part
				}
				delete(chunkBuffers, req.messageID)

				if !complete {
					req.resp <- &clientChunkResp{complete: false}
				} else {
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						req.resp <- &clientChunkResp{complete: false}
					} else {
						var resp struct {
							Type    string `json:"type"`
							Payload []any  `json:"payload"`
						}
						if err := json.Unmarshal(decoded, &resp); err != nil {
							req.resp <- &clientChunkResp{complete: false}
						} else {
							// Route reassembled response
							if ch, exists := pending[req.requestEventID]; exists {
								respMsg := &ResponseMessage{Type: resp.Type, Payload: resp.Payload}
								select {
								case ch <- respMsg:
								default:
								}
							}
							req.resp <- &clientChunkResp{complete: true, respMsg: &ResponseMessage{Type: resp.Type, Payload: resp.Payload}}
						}
					}
				}
			} else {
				req.resp <- &clientChunkResp{complete: false}
			}

			// Clean up stale buffers
			now := time.Now()
			for id, b := range chunkBuffers {
				if now.Sub(b.receivedAt) > 60*time.Second {
					log.W.F("NRC: discarding stale chunk buffer: %s", id)
					delete(chunkBuffers, id)
				}
			}

		case req := <-c.closeAllCh:
			for _, ch := range pending {
				close(ch)
			}
			pending = make(map[string]chan *ResponseMessage)
			for _, ch := range subscriptions {
				close(ch)
			}
			subscriptions = make(map[string]chan *event.E)
			chunkBuffers = make(map[string]*chunkBuffer)
			req.resp <- struct{}{}
		}
	}
}

func (c *Client) getPending(eventID string) chan *ResponseMessage {
	resp := make(chan chan *ResponseMessage, 1)
	select {
	case c.getPendingCh <- clientGetPendingReq{eventID: eventID, resp: resp}:
		return <-resp
	case <-c.stop:
		return nil
	}
}

func (c *Client) getSub(subID string) chan *event.E {
	resp := make(chan chan *event.E, 1)
	select {
	case c.getSubCh <- clientGetSubReq{subID: subID, resp: resp}:
		return <-resp
	case <-c.stop:
		return nil
	}
}

// Connect establishes the connection to the rendezvous relay.
func (c *Client) Connect(ctx context.Context) error {
	conn, err := ws.RelayConnect(ctx, c.uri.RendezvousRelay)
	if chk.E(err) {
		return fmt.Errorf("%w: %v", ErrRendezvousConnectionFailed, err)
	}
	c.rendezvousConn = conn

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

	// Close all state via actor
	resp := make(chan struct{}, 1)
	select {
	case c.closeAllCh <- clientCloseAllReq{resp: resp}:
		<-resp
	default:
	}

	close(c.stop)
	<-c.done
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
	decrypted, err := encryption.Decrypt(c.conversationKey, string(ev.Content))
	if err != nil {
		log.W.F("NRC response decryption failed: %v", err)
		return
	}

	var resp struct {
		Type    string `json:"type"`
		Payload []any  `json:"payload"`
	}
	if err := json.Unmarshal([]byte(decrypted), &resp); err != nil {
		log.W.F("NRC response parse failed: %v", err)
		return
	}

	var requestEventID string
	eTag := ev.Tags.GetFirst([]byte("e"))
	if eTag != nil && eTag.Len() >= 2 {
		requestEventID = string(eTag.ValueHex())
	}

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
	subID, ok := payload[1].(string)
	if !ok {
		return
	}

	ch := c.getSub(subID)
	if ch == nil {
		return
	}

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
	}
}

// handleEOSEResponse handles an EOSE response.
func (c *Client) handleEOSEResponse(payload []any, requestEventID string) {
	ch := c.getPending(requestEventID)
	if ch != nil {
		resp := &ResponseMessage{Type: "EOSE", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleOKResponse handles an OK response.
func (c *Client) handleOKResponse(payload []any, requestEventID string) {
	ch := c.getPending(requestEventID)
	if ch != nil {
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
			select {
			case c.closeSubCh <- clientCloseSubReq{subID: subID}:
			case <-c.stop:
			}
		}
	}
}

// handleCountResponse handles a COUNT response.
func (c *Client) handleCountResponse(payload []any, requestEventID string) {
	ch := c.getPending(requestEventID)
	if ch != nil {
		resp := &ResponseMessage{Type: "COUNT", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleAuthResponse handles an AUTH challenge.
func (c *Client) handleAuthResponse(payload []any, requestEventID string) {
	ch := c.getPending(requestEventID)
	if ch != nil {
		resp := &ResponseMessage{Type: "AUTH", Payload: payload}
		select {
		case ch <- resp:
		default:
		}
	}
}

// handleIDSResponse handles an IDS response.
func (c *Client) handleIDSResponse(payload []any, requestEventID string) {
	ch := c.getPending(requestEventID)
	if ch != nil {
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

	log.D.F("NRC: received chunk %d/%d for message %s", index+1, total, messageID[:8])

	resp := make(chan *clientChunkResp, 1)
	select {
	case c.chunkCh <- clientChunkReq{
		messageID:      messageID,
		index:          index,
		total:          total,
		data:           data,
		requestEventID: requestEventID,
		resp:           resp,
	}:
		r := <-resp
		if r.complete && r.respMsg != nil {
			log.D.F("NRC: reassembled chunked message: %s", r.respMsg.Type)
		}
	case <-c.stop:
	}
}

// sendRequest sends an NRC request and waits for response.
func (c *Client) sendRequest(ctx context.Context, msgType string, payload []any) (*ResponseMessage, error) {
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

	encrypted, err := encryption.Encrypt(c.conversationKey, contentBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

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

	if err := reqEvent.Sign(c.clientSigner); chk.E(err) {
		return nil, fmt.Errorf("signing failed: %w", err)
	}

	// Set up response channel via actor
	responseCh := make(chan *ResponseMessage, 1)
	requestEventID := string(hex.Enc(reqEvent.ID[:]))

	select {
	case c.registerPendingCh <- clientRegisterPendingReq{eventID: requestEventID, ch: responseCh}:
	case <-c.stop:
		return nil, fmt.Errorf("client stopped")
	}

	defer func() {
		select {
		case c.removePendingCh <- clientRemovePendingReq{eventID: requestEventID}:
		case <-c.stop:
		}
	}()

	if err := c.rendezvousConn.Publish(ctx, reqEvent); chk.E(err) {
		return nil, fmt.Errorf("publish failed: %w", err)
	}

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

	if resp.Type != "OK" || len(resp.Payload) < 4 {
		return false, "", fmt.Errorf("unexpected response type: %s", resp.Type)
	}

	success, _ := resp.Payload[2].(bool)
	message, _ := resp.Payload[3].(string)

	return success, message, nil
}

// Subscribe creates a subscription to the private relay.
func (c *Client) Subscribe(ctx context.Context, subID string, filters ...*filter.F) (<-chan *event.E, error) {
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

	eventCh := make(chan *event.E, 100)

	select {
	case c.registerSubCh <- clientRegisterSubReq{subID: subID, ch: eventCh}:
	case <-c.stop:
		return nil, fmt.Errorf("client stopped")
	}

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
	select {
	case c.closeSubCh <- clientCloseSubReq{subID: subID}:
	case <-c.stop:
	}

	payload := []any{"CLOSE", subID}
	_, err := c.sendRequest(ctx, "CLOSE", payload)
	return err
}

// Count sends a COUNT request to the private relay.
func (c *Client) Count(ctx context.Context, subID string, filters ...*filter.F) (int64, error) {
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

	if resp.Type != "IDS" || len(resp.Payload) < 3 {
		return nil, fmt.Errorf("unexpected response type: %s", resp.Type)
	}

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
