//go:build js && wasm

package wasmdb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"syscall/js"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/encoders/timestamp"
)

// JSBridge holds the database instance for JavaScript access
var jsBridge *JSBridge

// JSBridge wraps the WasmDB instance for JavaScript interop
// Exposes a relay protocol interface (NIP-01) rather than direct database access
type JSBridge struct {
	db     *W
	ctx    context.Context
	cancel context.CancelFunc
}

// RegisterJSBridge exposes the relay protocol API to JavaScript
func RegisterJSBridge(db *W, ctx context.Context, cancel context.CancelFunc) {
	jsBridge = &JSBridge{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}

	// Create the wasmdb global object with relay protocol interface
	wasmdbObj := map[string]interface{}{
		// Lifecycle
		"isReady": js.FuncOf(jsBridge.jsIsReady),
		"close":   js.FuncOf(jsBridge.jsClose),
		"wipe":    js.FuncOf(jsBridge.jsWipe),

		// Relay Protocol (NIP-01)
		// This is the main entry point - handles EVENT, REQ, CLOSE messages
		"handleMessage": js.FuncOf(jsBridge.jsHandleMessage),

		// Graph Query Extensions
		"queryGraph": js.FuncOf(jsBridge.jsQueryGraph),

		// Marker Extensions (key-value storage via relay protocol)
		// ["MARKER", "set", key, value] -> ["OK", key, true]
		// ["MARKER", "get", key] -> ["MARKER", key, value]
		// ["MARKER", "delete", key] -> ["OK", key, true]
		// These are also handled via handleMessage
	}

	js.Global().Set("wasmdb", wasmdbObj)
}

// jsIsReady returns true if the database is ready
func (b *JSBridge) jsIsReady(this js.Value, args []js.Value) interface{} {
	select {
	case <-b.db.Ready():
		return true
	default:
		return false
	}
}

// jsClose closes the database
func (b *JSBridge) jsClose(this js.Value, args []js.Value) interface{} {
	return promiseWrapper(func() (interface{}, error) {
		err := b.db.Close()
		return nil, err
	})
}

// jsWipe wipes all data from the database
func (b *JSBridge) jsWipe(this js.Value, args []js.Value) interface{} {
	return promiseWrapper(func() (interface{}, error) {
		err := b.db.Wipe()
		return nil, err
	})
}

// jsHandleMessage handles NIP-01 relay protocol messages
// Input: JSON string representing a relay message array
//
//	["EVENT", <event>] - Submit an event
//	["REQ", <sub_id>, <filter>...] - Request events
//	["CLOSE", <sub_id>] - Close a subscription
//	["MARKER", "set"|"get"|"delete", key, value?] - Marker operations
//
// Output: Promise<string[]> - Array of JSON response messages
func (b *JSBridge) jsHandleMessage(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return rejectPromise("handleMessage requires message JSON argument")
	}

	messageJSON := args[0].String()

	return promiseWrapper(func() (interface{}, error) {
		// Parse the message array
		var message []json.RawMessage
		if err := json.Unmarshal([]byte(messageJSON), &message); err != nil {
			return nil, fmt.Errorf("invalid message format: %w", err)
		}

		if len(message) < 1 {
			return nil, fmt.Errorf("empty message")
		}

		// Get message type
		var msgType string
		if err := json.Unmarshal(message[0], &msgType); err != nil {
			return nil, fmt.Errorf("invalid message type: %w", err)
		}

		switch msgType {
		case "EVENT":
			return b.handleEvent(message)
		case "REQ":
			return b.handleReq(message)
		case "CLOSE":
			return b.handleClose(message)
		case "MARKER":
			return b.handleMarker(message)
		default:
			return nil, fmt.Errorf("unknown message type: %s", msgType)
		}
	})
}

// handleEvent processes an EVENT message
// ["EVENT", <event>] -> ["OK", <id>, true/false, "message"]
func (b *JSBridge) handleEvent(message []json.RawMessage) (interface{}, error) {
	if len(message) < 2 {
		return []interface{}{makeOK("", false, "missing event")}, nil
	}

	// Parse the event
	ev, err := parseEventFromRawJSON(message[1])
	if err != nil {
		return []interface{}{makeOK("", false, fmt.Sprintf("invalid event: %s", err))}, nil
	}

	eventIDHex := hex.Enc(ev.ID)

	// Save to database
	replaced, err := b.db.SaveEvent(b.ctx, ev)
	if err != nil {
		return []interface{}{makeOK(eventIDHex, false, err.Error())}, nil
	}

	var msg string
	if replaced {
		msg = "replaced"
	} else {
		msg = "saved"
	}

	return []interface{}{makeOK(eventIDHex, true, msg)}, nil
}

// handleReq processes a REQ message
// ["REQ", <sub_id>, <filter>...] -> ["EVENT", <sub_id>, <event>]..., ["EOSE", <sub_id>]
func (b *JSBridge) handleReq(message []json.RawMessage) (interface{}, error) {
	if len(message) < 2 {
		return nil, fmt.Errorf("REQ requires subscription ID")
	}

	// Get subscription ID
	var subID string
	if err := json.Unmarshal(message[1], &subID); err != nil {
		return nil, fmt.Errorf("invalid subscription ID: %w", err)
	}

	// Parse filters (can have multiple)
	var allEvents []*event.E
	for i := 2; i < len(message); i++ {
		f, err := parseFilterFromRawJSON(message[i])
		if err != nil {
			continue
		}

		events, err := b.db.QueryEvents(b.ctx, f)
		if err != nil {
			continue
		}

		allEvents = append(allEvents, events...)
	}

	// Build response messages
	responses := make([]interface{}, 0, len(allEvents)+1)

	// Add EVENT messages
	for _, ev := range allEvents {
		eventJSON, err := eventToJSON(ev)
		if err != nil {
			continue
		}
		responses = append(responses, makeEvent(subID, string(eventJSON)))
	}

	// Add EOSE
	responses = append(responses, makeEOSE(subID))

	return responses, nil
}

// handleClose processes a CLOSE message
// ["CLOSE", <sub_id>] -> (no response for local relay)
func (b *JSBridge) handleClose(message []json.RawMessage) (interface{}, error) {
	// For the local relay, subscriptions are stateless (single query/response)
	// CLOSE is a no-op but we acknowledge it
	return []interface{}{}, nil
}

// handleMarker processes MARKER extension messages
// ["MARKER", "set", key, value] -> ["OK", key, true]
// ["MARKER", "get", key] -> ["MARKER", key, value] or ["MARKER", key, null]
// ["MARKER", "delete", key] -> ["OK", key, true]
func (b *JSBridge) handleMarker(message []json.RawMessage) (interface{}, error) {
	if len(message) < 3 {
		return nil, fmt.Errorf("MARKER requires operation and key")
	}

	var operation string
	if err := json.Unmarshal(message[1], &operation); err != nil {
		return nil, fmt.Errorf("invalid marker operation: %w", err)
	}

	var key string
	if err := json.Unmarshal(message[2], &key); err != nil {
		return nil, fmt.Errorf("invalid marker key: %w", err)
	}

	switch operation {
	case "set":
		if len(message) < 4 {
			return nil, fmt.Errorf("MARKER set requires value")
		}
		var value string
		if err := json.Unmarshal(message[3], &value); err != nil {
			return nil, fmt.Errorf("invalid marker value: %w", err)
		}
		if err := b.db.SetMarker(key, []byte(value)); err != nil {
			return []interface{}{makeMarkerOK(key, false, err.Error())}, nil
		}
		return []interface{}{makeMarkerOK(key, true, "")}, nil

	case "get":
		value, err := b.db.GetMarker(key)
		if err != nil || value == nil {
			return []interface{}{makeMarkerResult(key, nil)}, nil
		}
		valueStr := string(value)
		return []interface{}{makeMarkerResult(key, &valueStr)}, nil

	case "delete":
		if err := b.db.DeleteMarker(key); err != nil {
			return []interface{}{makeMarkerOK(key, false, err.Error())}, nil
		}
		return []interface{}{makeMarkerOK(key, true, "")}, nil

	case "has":
		has := b.db.HasMarker(key)
		return []interface{}{makeMarkerHas(key, has)}, nil

	default:
		return nil, fmt.Errorf("unknown marker operation: %s", operation)
	}
}

// jsQueryGraph handles graph query extensions
// Args: [queryJSON: string] - JSON-encoded graph query
// Returns: Promise<string> - JSON-encoded graph result
func (b *JSBridge) jsQueryGraph(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return rejectPromise("queryGraph requires query JSON argument")
	}

	queryJSON := args[0].String()

	return promiseWrapper(func() (interface{}, error) {
		var query struct {
			Type   string `json:"type"`
			Pubkey string `json:"pubkey"`
			Depth  int    `json:"depth,omitempty"`
			Limit  int    `json:"limit,omitempty"`
		}

		if err := json.Unmarshal([]byte(queryJSON), &query); err != nil {
			return nil, fmt.Errorf("invalid graph query: %w", err)
		}

		// Set defaults
		if query.Depth == 0 {
			query.Depth = 1
		}
		if query.Limit == 0 {
			query.Limit = 1000
		}

		switch query.Type {
		case "follows":
			return b.queryFollows(query.Pubkey, query.Depth, query.Limit)
		case "followers":
			return b.queryFollowers(query.Pubkey, query.Limit)
		case "mutes":
			return b.queryMutes(query.Pubkey)
		default:
			return nil, fmt.Errorf("unknown graph query type: %s", query.Type)
		}
	})
}

// queryFollows returns who a pubkey follows
func (b *JSBridge) queryFollows(pubkeyHex string, depth, limit int) (interface{}, error) {
	// Query kind 3 (contact list) for the pubkey
	f := &filter.F{
		Kinds: kind.NewWithCap(1),
	}
	f.Kinds.K = append(f.Kinds.K, kind.New(3))
	f.Authors = tag.NewWithCap(1)
	f.Authors.T = append(f.Authors.T, []byte(pubkeyHex))
	one := uint(1)
	f.Limit = &one

	events, err := b.db.QueryEvents(b.ctx, f)
	if err != nil {
		return nil, err
	}

	var follows []string
	if len(events) > 0 && events[0].Tags != nil {
		for _, t := range *events[0].Tags {
			if t != nil && len(t.T) >= 2 && string(t.T[0]) == "p" {
				follows = append(follows, string(t.T[1]))
			}
		}
	}

	result := map[string]interface{}{
		"nodes": follows,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

// queryFollowers returns who follows a pubkey
func (b *JSBridge) queryFollowers(pubkeyHex string, limit int) (interface{}, error) {
	// Query kind 3 events that tag this pubkey
	f := &filter.F{
		Kinds: kind.NewWithCap(1),
		Tags:  tag.NewSWithCap(1),
	}
	f.Kinds.K = append(f.Kinds.K, kind.New(3))

	// Add #p tag filter
	pTag := tag.NewWithCap(2)
	pTag.T = append(pTag.T, []byte("p"))
	pTag.T = append(pTag.T, []byte(pubkeyHex))
	f.Tags.Append(pTag)

	lim := uint(limit)
	f.Limit = &lim

	events, err := b.db.QueryEvents(b.ctx, f)
	if err != nil {
		return nil, err
	}

	var followers []string
	for _, ev := range events {
		followers = append(followers, hex.Enc(ev.Pubkey))
	}

	result := map[string]interface{}{
		"nodes": followers,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

// queryMutes returns who a pubkey has muted
func (b *JSBridge) queryMutes(pubkeyHex string) (interface{}, error) {
	// Query kind 10000 (mute list) for the pubkey
	f := &filter.F{
		Kinds: kind.NewWithCap(1),
	}
	f.Kinds.K = append(f.Kinds.K, kind.New(10000))
	f.Authors = tag.NewWithCap(1)
	f.Authors.T = append(f.Authors.T, []byte(pubkeyHex))
	one := uint(1)
	f.Limit = &one

	events, err := b.db.QueryEvents(b.ctx, f)
	if err != nil {
		return nil, err
	}

	var mutes []string
	if len(events) > 0 && events[0].Tags != nil {
		for _, t := range *events[0].Tags {
			if t != nil && len(t.T) >= 2 && string(t.T[0]) == "p" {
				mutes = append(mutes, string(t.T[1]))
			}
		}
	}

	result := map[string]interface{}{
		"nodes": mutes,
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

// Response message builders

func makeOK(eventID string, accepted bool, message string) string {
	msg := []interface{}{"OK", eventID, accepted, message}
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes)
}

func makeEvent(subID, eventJSON string) string {
	// We return the raw event JSON embedded in the array
	return fmt.Sprintf(`["EVENT","%s",%s]`, subID, eventJSON)
}

func makeEOSE(subID string) string {
	msg := []interface{}{"EOSE", subID}
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes)
}

func makeMarkerOK(key string, success bool, message string) string {
	msg := []interface{}{"OK", key, success}
	if message != "" {
		msg = append(msg, message)
	}
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes)
}

func makeMarkerResult(key string, value *string) string {
	var msg []interface{}
	if value == nil {
		msg = []interface{}{"MARKER", key, nil}
	} else {
		msg = []interface{}{"MARKER", key, *value}
	}
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes)
}

func makeMarkerHas(key string, has bool) string {
	msg := []interface{}{"MARKER", key, has}
	jsonBytes, _ := json.Marshal(msg)
	return string(jsonBytes)
}

// Helper functions

// promiseWrapper wraps a function in a JavaScript Promise
func promiseWrapper(fn func() (interface{}, error)) interface{} {
	handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		resolve := args[0]
		reject := args[1]

		go func() {
			result, err := fn()
			if err != nil {
				reject.Invoke(err.Error())
			} else {
				resolve.Invoke(result)
			}
		}()

		return nil
	})

	promiseConstructor := js.Global().Get("Promise")
	return promiseConstructor.New(handler)
}

// rejectPromise creates a rejected promise with an error message
func rejectPromise(msg string) interface{} {
	handler := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		reject := args[1]
		reject.Invoke(msg)
		return nil
	})

	promiseConstructor := js.Global().Get("Promise")
	return promiseConstructor.New(handler)
}

// parseEventFromRawJSON parses a Nostr event from raw JSON
func parseEventFromRawJSON(raw json.RawMessage) (*event.E, error) {
	return parseEventFromJSON(string(raw))
}

// parseEventFromJSON parses a Nostr event from JSON
func parseEventFromJSON(jsonStr string) (*event.E, error) {
	// Parse into intermediate struct for JSON compatibility
	var raw struct {
		ID        string     `json:"id"`
		Pubkey    string     `json:"pubkey"`
		CreatedAt int64      `json:"created_at"`
		Kind      int        `json:"kind"`
		Tags      [][]string `json:"tags"`
		Content   string     `json:"content"`
		Sig       string     `json:"sig"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	ev := &event.E{
		Kind:      uint16(raw.Kind),
		CreatedAt: raw.CreatedAt,
		Content:   []byte(raw.Content),
	}

	// Decode ID
	if id, err := hex.Dec(raw.ID); err == nil && len(id) == 32 {
		ev.ID = id
	}

	// Decode Pubkey
	if pk, err := hex.Dec(raw.Pubkey); err == nil && len(pk) == 32 {
		ev.Pubkey = pk
	}

	// Decode Sig
	if sig, err := hex.Dec(raw.Sig); err == nil && len(sig) == 64 {
		ev.Sig = sig
	}

	// Convert tags
	if len(raw.Tags) > 0 {
		ev.Tags = tag.NewSWithCap(len(raw.Tags))
		for _, t := range raw.Tags {
			tagBytes := make([][]byte, len(t))
			for i, s := range t {
				tagBytes[i] = []byte(s)
			}
			newTag := tag.NewFromBytesSlice(tagBytes...)
			ev.Tags.Append(newTag)
		}
	}

	return ev, nil
}

// parseFilterFromRawJSON parses a Nostr filter from raw JSON
func parseFilterFromRawJSON(raw json.RawMessage) (*filter.F, error) {
	return parseFilterFromJSON(string(raw))
}

// parseFilterFromJSON parses a Nostr filter from JSON
func parseFilterFromJSON(jsonStr string) (*filter.F, error) {
	// Parse into intermediate struct
	var raw struct {
		IDs     []string `json:"ids,omitempty"`
		Authors []string `json:"authors,omitempty"`
		Kinds   []int    `json:"kinds,omitempty"`
		Since   *int64   `json:"since,omitempty"`
		Until   *int64   `json:"until,omitempty"`
		Limit   *uint    `json:"limit,omitempty"`
		Search  *string  `json:"search,omitempty"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	f := &filter.F{}

	// Set IDs
	if len(raw.IDs) > 0 {
		f.Ids = tag.NewWithCap(len(raw.IDs))
		for _, idHex := range raw.IDs {
			f.Ids.T = append(f.Ids.T, []byte(idHex))
		}
	}

	// Set Authors
	if len(raw.Authors) > 0 {
		f.Authors = tag.NewWithCap(len(raw.Authors))
		for _, pkHex := range raw.Authors {
			f.Authors.T = append(f.Authors.T, []byte(pkHex))
		}
	}

	// Set Kinds
	if len(raw.Kinds) > 0 {
		f.Kinds = kind.NewWithCap(len(raw.Kinds))
		for _, k := range raw.Kinds {
			f.Kinds.K = append(f.Kinds.K, kind.New(uint16(k)))
		}
	}

	// Set timestamps
	if raw.Since != nil {
		f.Since = timestamp.New(*raw.Since)
	}
	if raw.Until != nil {
		f.Until = timestamp.New(*raw.Until)
	}

	// Set limit
	if raw.Limit != nil {
		f.Limit = raw.Limit
	}

	// Set search
	if raw.Search != nil {
		f.Search = []byte(*raw.Search)
	}

	// Handle tag filters (e.g., #e, #p, #t)
	var rawMap map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &rawMap)
	for key, val := range rawMap {
		if len(key) == 2 && key[0] == '#' {
			if arr, ok := val.([]interface{}); ok {
				tagFilter := tag.NewWithCap(len(arr) + 1)
				// First element is the tag name (e.g., "e", "p")
				tagFilter.T = append(tagFilter.T, []byte{key[1]})
				for _, v := range arr {
					if s, ok := v.(string); ok {
						tagFilter.T = append(tagFilter.T, []byte(s))
					}
				}
				if f.Tags == nil {
					f.Tags = tag.NewSWithCap(4)
				}
				f.Tags.Append(tagFilter)
			}
		}
	}

	return f, nil
}

// eventToJSON converts a Nostr event to JSON
func eventToJSON(ev *event.E) ([]byte, error) {
	// Build tags array
	var tags [][]string
	if ev.Tags != nil {
		for _, t := range *ev.Tags {
			if t == nil {
				continue
			}
			tagStrs := make([]string, len(t.T))
			for i, elem := range t.T {
				tagStrs[i] = string(elem)
			}
			tags = append(tags, tagStrs)
		}
	}

	raw := struct {
		ID        string     `json:"id"`
		Pubkey    string     `json:"pubkey"`
		CreatedAt int64      `json:"created_at"`
		Kind      int        `json:"kind"`
		Tags      [][]string `json:"tags"`
		Content   string     `json:"content"`
		Sig       string     `json:"sig"`
	}{
		ID:        hex.Enc(ev.ID),
		Pubkey:    hex.Enc(ev.Pubkey),
		CreatedAt: ev.CreatedAt,
		Kind:      int(ev.Kind),
		Tags:      tags,
		Content:   string(ev.Content),
		Sig:       hex.Enc(ev.Sig),
	}

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(raw); err != nil {
		return nil, err
	}

	// Remove trailing newline from encoder
	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}
