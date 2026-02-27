package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/encoders/envelopes/eventenvelope"
	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	negentropyiface "next.orly.dev/pkg/interfaces/negentropy"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
)

// NIP-77 Negentropy envelope constants
const (
	NegOpenLabel  = "NEG-OPEN"
	NegMsgLabel   = "NEG-MSG"
	NegCloseLabel = "NEG-CLOSE"
	NegErrLabel   = "NEG-ERR"
)

// negentropyHandler handles NIP-77 negentropy operations
// This can be either a gRPC client or an embedded handler
var negentropyHandler negentropyiface.Handler

// SetNegentropyHandler sets the negentropy handler for NIP-77 WebSocket handling
func SetNegentropyHandler(handler negentropyiface.Handler) {
	negentropyHandler = handler
}

// IsNegentropyEnvelope checks if a message starts with a NEG-* envelope type
func IsNegentropyEnvelope(msg []byte) bool {
	// Quick check: must start with '["NEG-'
	if len(msg) < 8 {
		return false
	}
	return bytes.HasPrefix(msg, []byte(`["NEG-`))
}

// IdentifyNegentropyEnvelope extracts the envelope type and remaining payload
func IdentifyNegentropyEnvelope(msg []byte) (envelopeType string, ok bool) {
	// Parse enough to get the envelope type
	if !IsNegentropyEnvelope(msg) {
		return "", false
	}

	// Find the first comma after the opening label
	end := bytes.IndexByte(msg[2:], '"')
	if end < 0 {
		return "", false
	}
	envelopeType = string(msg[2 : 2+end])
	return envelopeType, true
}

// HandleNegOpen processes NEG-OPEN messages
// Format: ["NEG-OPEN", subscription_id, filter, initial_message?]
func (l *Listener) HandleNegOpen(msg []byte) error {
	log.D.F("HandleNegOpen called from %s", l.connectionID)
	if negentropyHandler == nil {
		log.E.F("negentropy handler not initialized — client sent NEG-OPEN but NIP-77 is not enabled (check ORLY_NEGENTROPY_ENABLED and startup logs)")
		return l.sendNegErr("", "negentropy not enabled on this relay")
	}

	// Parse the message array
	var parts []json.RawMessage
	if err := json.Unmarshal(msg, &parts); err != nil {
		return l.sendNegErr("", fmt.Sprintf("invalid NEG-OPEN format: %v", err))
	}

	if len(parts) < 3 {
		return l.sendNegErr("", "NEG-OPEN requires at least 3 elements")
	}

	// Extract subscription ID
	var subscriptionID string
	if err := json.Unmarshal(parts[1], &subscriptionID); err != nil {
		return l.sendNegErr("", fmt.Sprintf("invalid subscription_id: %v", err))
	}

	// Extract filter - use custom parsing because filter.F's kinds field
	// doesn't support standard JSON array unmarshaling
	f, err := parseNegentropyFilter(parts[2])
	if err != nil {
		return l.sendNegErr(subscriptionID, fmt.Sprintf("invalid filter: %v", err))
	}

	// Extract optional initial message (hex encoded per NIP-77)
	var initialMessage []byte
	if len(parts) >= 4 {
		var msgStr string
		if err := json.Unmarshal(parts[3], &msgStr); err == nil && msgStr != "" {
			// NIP-77 uses hex encoding
			if decoded, err := hex.DecodeString(msgStr); err == nil {
				initialMessage = decoded
			} else {
				log.W.F("NEG-OPEN: invalid hex message: %v", err)
			}
		}
	}

	// Convert filter to proto format
	protoFilter := filterToProto(f)

	// Call gRPC service
	ctx := context.Background()
	respMsg, haveIDs, needIDs, complete, errStr, err := negentropyHandler.HandleNegOpen(
		ctx,
		l.connectionID,
		subscriptionID,
		protoFilter,
		initialMessage,
	)
	if err != nil {
		log.E.F("NEG-OPEN gRPC error: %v", err)
		return l.sendNegErr(subscriptionID, "internal error")
	}

	if errStr != "" {
		return l.sendNegErr(subscriptionID, errStr)
	}

	// Log need_ids (events client should send us)
	if len(needIDs) > 0 {
		log.D.F("NEG-OPEN: relay needs %d events from client", len(needIDs))
	}

	// Send NEG-MSG response FIRST (before events)
	if err := l.sendNegMsg(subscriptionID, respMsg); err != nil {
		return err
	}

	// If reconciliation is complete, send events we have that client needs.
	// Per NIP-77: The haves/needs are only final when reconcile returns complete=true.
	if complete {
		log.D.F("NEG-OPEN: reconciliation complete for %s, sending %d events", subscriptionID, len(haveIDs))
		if len(haveIDs) > 0 {
			if err := l.sendEventsForIDs(subscriptionID, haveIDs); err != nil {
				log.E.F("failed to send events for NEG-OPEN: %v", err)
			}
		}
	}

	return nil
}

// HandleNegMsg processes NEG-MSG messages
// Format: ["NEG-MSG", subscription_id, message]
func (l *Listener) HandleNegMsg(msg []byte) error {
	if negentropyHandler == nil {
		return l.sendNegErr("", "negentropy not enabled")
	}

	// Parse the message array
	var parts []json.RawMessage
	if err := json.Unmarshal(msg, &parts); err != nil {
		return l.sendNegErr("", fmt.Sprintf("invalid NEG-MSG format: %v", err))
	}

	if len(parts) < 3 {
		return l.sendNegErr("", "NEG-MSG requires 3 elements")
	}

	// Extract subscription ID
	var subscriptionID string
	if err := json.Unmarshal(parts[1], &subscriptionID); err != nil {
		return l.sendNegErr("", fmt.Sprintf("invalid subscription_id: %v", err))
	}

	// Extract message (hex or base64 encoded)
	var msgStr string
	if err := json.Unmarshal(parts[2], &msgStr); err != nil {
		return l.sendNegErr(subscriptionID, fmt.Sprintf("invalid message: %v", err))
	}

	// Decode message (NIP-77 uses hex encoding)
	negentropyMsg, err := hex.DecodeString(msgStr)
	if err != nil {
		return l.sendNegErr(subscriptionID, fmt.Sprintf("invalid hex message: %v", err))
	}

	// Call gRPC service
	ctx := context.Background()
	respMsg, haveIDs, needIDs, complete, errStr, err := negentropyHandler.HandleNegMsg(
		ctx,
		l.connectionID,
		subscriptionID,
		negentropyMsg,
	)
	if err != nil {
		log.E.F("NEG-MSG gRPC error: %v", err)
		return l.sendNegErr(subscriptionID, "internal error")
	}

	if errStr != "" {
		return l.sendNegErr(subscriptionID, errStr)
	}

	// Log need_ids (events client should send us)
	if len(needIDs) > 0 {
		log.D.F("NEG-MSG: relay needs %d events from client", len(needIDs))
	}

	// Send NEG-MSG response FIRST (before events)
	if err := l.sendNegMsg(subscriptionID, respMsg); err != nil {
		return err
	}

	// If reconciliation is complete, send events we have that client needs.
	// Per NIP-77: The haves/needs are only final when reconcile returns complete=true.
	if complete {
		log.D.F("NEG-MSG: reconciliation complete for %s, sending %d events", subscriptionID, len(haveIDs))
		if len(haveIDs) > 0 {
			if err := l.sendEventsForIDs(subscriptionID, haveIDs); err != nil {
				log.E.F("failed to send events for NEG-MSG: %v", err)
			}
		}
	}

	return nil
}

// HandleNegClose processes NEG-CLOSE messages
// Format: ["NEG-CLOSE", subscription_id]
func (l *Listener) HandleNegClose(msg []byte) error {
	if negentropyHandler == nil {
		return nil // Silently ignore if not enabled
	}

	// Parse the message array
	var parts []json.RawMessage
	if err := json.Unmarshal(msg, &parts); err != nil {
		return nil // Silently ignore malformed close
	}

	if len(parts) < 2 {
		return nil
	}

	// Extract subscription ID
	var subscriptionID string
	if err := json.Unmarshal(parts[1], &subscriptionID); err != nil {
		return nil
	}

	// Call gRPC service to close the session
	ctx := context.Background()
	if err := negentropyHandler.HandleNegClose(ctx, l.connectionID, subscriptionID); err != nil {
		log.E.F("NEG-CLOSE gRPC error: %v", err)
	}

	return nil
}

// sendNegMsg sends a NEG-MSG response to the client
func (l *Listener) sendNegMsg(subscriptionID string, message []byte) error {
	// Encode message as hex (per NIP-77)
	encoded := ""
	if len(message) > 0 {
		encoded = hex.EncodeToString(message)
	}

	// Format: ["NEG-MSG", subscription_id, message]
	resp, err := json.Marshal([]any{NegMsgLabel, subscriptionID, encoded})
	if err != nil {
		return err
	}

	_, err = l.Write(resp)
	return err
}

// sendNegErr sends a NEG-ERR response to the client
func (l *Listener) sendNegErr(subscriptionID, reason string) error {
	// Format: ["NEG-ERR", subscription_id, reason]
	resp, err := json.Marshal([]any{NegErrLabel, subscriptionID, reason})
	if err != nil {
		return err
	}

	_, err = l.Write(resp)
	return err
}

// sendEventsForIDs fetches and sends events for the given IDs
func (l *Listener) sendEventsForIDs(subscriptionID string, ids [][]byte) error {
	if len(ids) == 0 {
		return nil
	}

	log.D.F("NEG: sending %d events for subscription %s", len(ids), subscriptionID)

	// Build filter with binary IDs (32 bytes each)
	f := &filter.F{}
	f.Ids = &tag.T{}
	for _, id := range ids {
		// IDs are binary (32 bytes full or 16 bytes truncated per NIP-77)
		f.Ids.T = append(f.Ids.T, id)
	}

	// Query events by IDs
	ctx := l.ctx
	events, err := l.Server.db.QueryEvents(ctx, f)
	if err != nil {
		log.E.F("NEG: failed to query events: %v", err)
		return err
	}

	// Send each event via EVENT envelope with subscription ID
	sent := 0
	for _, ev := range events {
		if ev == nil {
			continue
		}
		// Create proper EVENT envelope: ["EVENT", subscription_id, event]
		res, err := eventenvelope.NewResultWith([]byte(subscriptionID), ev)
		if err != nil {
			log.W.F("NEG: failed to create event envelope: %v", err)
			continue
		}
		if err := res.Write(l); err != nil {
			log.W.F("NEG: failed to send event: %v", err)
			continue
		}
		sent++
	}

	log.D.F("NEG: sent %d/%d events for subscription %s", sent, len(ids), subscriptionID)
	return nil
}

// filterToProto converts a nostr filter to proto format
func filterToProto(f *filter.F) *commonv1.Filter {
	if f == nil {
		return nil
	}

	pf := &commonv1.Filter{}

	// Convert Ids
	if f.Ids != nil {
		for _, id := range f.Ids.T {
			pf.Ids = append(pf.Ids, id)
		}
	}

	// Convert Authors
	if f.Authors != nil {
		for _, author := range f.Authors.T {
			pf.Authors = append(pf.Authors, author)
		}
	}

	// Convert Kinds - kind.S has ToUint16() method
	if f.Kinds != nil && f.Kinds.Len() > 0 {
		for _, k := range f.Kinds.ToUint16() {
			pf.Kinds = append(pf.Kinds, uint32(k))
		}
	}

	// Convert Since/Until - timestamp.T has .V field (int64)
	if f.Since != nil && f.Since.V != 0 {
		since := f.Since.V
		pf.Since = &since
	}
	if f.Until != nil && f.Until.V != 0 {
		until := f.Until.V
		pf.Until = &until
	}

	// Convert Limit
	if f.Limit != nil {
		limit := uint32(*f.Limit)
		pf.Limit = &limit
	}

	// Note: Tag filters (e, p, etc.) would need more complex conversion
	// This is a simplified implementation

	return pf
}

// parseNegentropyFilter parses a NIP-01 filter from JSON.
// This is needed because filter.F uses kind.S which doesn't implement
// json.Unmarshaler, so we parse manually and construct the filter.
func parseNegentropyFilter(data []byte) (*filter.F, error) {
	// Parse into a generic map first
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	f := filter.New()

	// Parse kinds array
	if kindsRaw, ok := raw["kinds"]; ok {
		var kinds []int
		if err := json.Unmarshal(kindsRaw, &kinds); err != nil {
			return nil, fmt.Errorf("invalid kinds: %v", err)
		}
		f.Kinds = kind.FromIntSlice(kinds)
	}

	// Parse authors array (hex pubkeys)
	if authorsRaw, ok := raw["authors"]; ok {
		var authors []string
		if err := json.Unmarshal(authorsRaw, &authors); err != nil {
			return nil, fmt.Errorf("invalid authors: %v", err)
		}
		f.Authors = tag.NewWithCap(len(authors))
		for _, a := range authors {
			if decoded, err := hex.DecodeString(a); err == nil {
				f.Authors.T = append(f.Authors.T, decoded)
			}
		}
	}

	// Parse ids array (hex event IDs)
	if idsRaw, ok := raw["ids"]; ok {
		var ids []string
		if err := json.Unmarshal(idsRaw, &ids); err != nil {
			return nil, fmt.Errorf("invalid ids: %v", err)
		}
		f.Ids = tag.NewWithCap(len(ids))
		for _, id := range ids {
			if decoded, err := hex.DecodeString(id); err == nil {
				f.Ids.T = append(f.Ids.T, decoded)
			}
		}
	}

	// Parse since timestamp
	if sinceRaw, ok := raw["since"]; ok {
		var since int64
		if err := json.Unmarshal(sinceRaw, &since); err == nil {
			f.Since = timestamp.FromUnix(since)
		}
	}

	// Parse until timestamp
	if untilRaw, ok := raw["until"]; ok {
		var until int64
		if err := json.Unmarshal(untilRaw, &until); err == nil {
			f.Until = timestamp.FromUnix(until)
		}
	}

	// Parse limit
	if limitRaw, ok := raw["limit"]; ok {
		var limit uint
		if err := json.Unmarshal(limitRaw, &limit); err == nil {
			f.Limit = &limit
		}
	}

	return f, nil
}

// CloseAllNegentropySessions closes all negentropy sessions for a connection
// Called when a WebSocket connection is closed
func (l *Listener) CloseAllNegentropySessions() {
	if negentropyHandler == nil {
		return
	}

	ctx := context.Background()
	sessions, err := negentropyHandler.ListSessions(ctx)
	if chk.E(err) {
		return
	}

	for _, sess := range sessions {
		if sess.ConnectionID == l.connectionID {
			negentropyHandler.CloseSession(ctx, l.connectionID, sess.SubscriptionID)
		}
	}
}
