package app

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/envelopes/eventenvelope"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	negentropygrpc "next.orly.dev/pkg/sync/negentropy/grpc"
)

// NIP-77 Negentropy envelope constants
const (
	NegOpenLabel  = "NEG-OPEN"
	NegMsgLabel   = "NEG-MSG"
	NegCloseLabel = "NEG-CLOSE"
	NegErrLabel   = "NEG-ERR"
)

// negentropyClient is the gRPC client for negentropy operations
// This should be set via Server.SetNegentropyClient() when using gRPC mode
var negentropyClient *negentropygrpc.Client

// SetNegentropyClient sets the negentropy gRPC client for NIP-77 WebSocket handling
func SetNegentropyClient(client *negentropygrpc.Client) {
	negentropyClient = client
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
	log.I.F("HandleNegOpen called from %s", l.connectionID)
	if negentropyClient == nil {
		log.E.F("negentropy client is nil")
		return l.sendNegErr("", "negentropy not enabled")
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

	// Extract filter
	var f filter.F
	if err := json.Unmarshal(parts[2], &f); err != nil {
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
	protoFilter := filterToProto(&f)

	// Call gRPC service
	ctx := context.Background()
	respMsg, haveIDs, needIDs, complete, errStr, err := negentropyClient.HandleNegOpen(
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
		log.D.F("NEG-OPEN: client needs to send %d events", len(needIDs))
	}

	// Send NEG-MSG response FIRST (before events)
	if err := l.sendNegMsg(subscriptionID, respMsg); err != nil {
		return err
	}

	// If reconciliation is complete, log it
	if complete {
		log.D.F("NEG-OPEN: reconciliation complete for %s", subscriptionID)
	}

	// AFTER sending response, send events for IDs we have that client needs
	if len(haveIDs) > 0 {
		if err := l.sendEventsForIDs(subscriptionID, haveIDs); err != nil {
			log.E.F("failed to send events for NEG-OPEN: %v", err)
		}
	}

	return nil
}

// HandleNegMsg processes NEG-MSG messages
// Format: ["NEG-MSG", subscription_id, message]
func (l *Listener) HandleNegMsg(msg []byte) error {
	if negentropyClient == nil {
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
	respMsg, haveIDs, needIDs, complete, errStr, err := negentropyClient.HandleNegMsg(
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
		log.D.F("NEG-MSG: client needs to send %d events", len(needIDs))
	}

	// Send NEG-MSG response FIRST (before events)
	if err := l.sendNegMsg(subscriptionID, respMsg); err != nil {
		return err
	}

	// If reconciliation is complete, log it
	if complete {
		log.D.F("NEG-MSG: reconciliation complete for %s", subscriptionID)
	}

	// AFTER sending response, send events for IDs we have that client needs
	if len(haveIDs) > 0 {
		if err := l.sendEventsForIDs(subscriptionID, haveIDs); err != nil {
			log.E.F("failed to send events for NEG-MSG: %v", err)
		}
	}

	return nil
}

// HandleNegClose processes NEG-CLOSE messages
// Format: ["NEG-CLOSE", subscription_id]
func (l *Listener) HandleNegClose(msg []byte) error {
	if negentropyClient == nil {
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
	if err := negentropyClient.HandleNegClose(ctx, l.connectionID, subscriptionID); err != nil {
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

	log.I.F("NEG: sending %d events for subscription %s", len(ids), subscriptionID)

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

	log.I.F("NEG: sent %d/%d events for subscription %s", sent, len(ids), subscriptionID)
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

// CloseAllNegentropySessions closes all negentropy sessions for a connection
// Called when a WebSocket connection is closed
func (l *Listener) CloseAllNegentropySessions() {
	if negentropyClient == nil {
		return
	}

	ctx := context.Background()
	sessions, err := negentropyClient.ListSessions(ctx)
	if chk.E(err) {
		return
	}

	for _, sess := range sessions {
		if sess.ConnectionID == l.connectionID {
			negentropyClient.CloseSession(ctx, l.connectionID, sess.SubscriptionID)
		}
	}
}
