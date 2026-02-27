package negentropy

import (
	"context"
	"encoding/hex"
	"fmt"

	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/encoders/filter"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/encoders/timestamp"
	negentropylib "next.orly.dev/pkg/nostr/negentropy"
	"next.orly.dev/pkg/database"
	negentropyiface "next.orly.dev/pkg/interfaces/negentropy"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
)

// EmbeddedHandler wraps the negentropy Manager to implement the Handler interface.
// This allows negentropy to run embedded in monolithic mode without gRPC.
type EmbeddedHandler struct {
	mgr   *Manager
	db    database.Database
	ready chan struct{}
}

// NewEmbeddedHandler creates a new embedded negentropy handler.
func NewEmbeddedHandler(db database.Database, cfg *Config) *EmbeddedHandler {
	h := &EmbeddedHandler{
		mgr:   NewManager(db, cfg),
		db:    db,
		ready: make(chan struct{}),
	}
	close(h.ready) // Immediately ready in embedded mode
	return h
}

// Start starts the background sync loop.
func (h *EmbeddedHandler) Start() {
	h.mgr.Start()
}

// Stop stops the background sync.
func (h *EmbeddedHandler) Stop() {
	h.mgr.Stop()
}

// Ready returns a channel that closes when the handler is ready.
func (h *EmbeddedHandler) Ready() <-chan struct{} {
	return h.ready
}

// Close cleans up resources.
func (h *EmbeddedHandler) Close() error {
	h.mgr.Stop()
	return nil
}

// HandleNegOpen processes a NEG-OPEN message from a client.
func (h *EmbeddedHandler) HandleNegOpen(ctx context.Context, connectionID, subscriptionID string, protoFilter *commonv1.Filter, initialMessage []byte) ([]byte, [][]byte, [][]byte, bool, string, error) {
	// Open a session for this client
	session := h.mgr.OpenSession(connectionID, subscriptionID)

	// Build storage from local events matching the filter
	storage, err := h.buildStorageForFilter(ctx, protoFilter)
	if err != nil {
		log.E.F("NEG-OPEN: failed to build storage: %v", err)
		return nil, nil, nil, false, fmt.Sprintf("failed to build storage: %v", err), nil
	}

	log.I.F("NEG-OPEN: built storage with %d events", storage.Size())

	// Create negentropy instance for this session
	neg := negentropylib.New(storage, negentropylib.DefaultFrameSizeLimit)

	// Store in session for later use
	session.SetNegentropy(neg, storage)

	// If we have an initial message from client, process it
	var respMsg []byte
	var complete bool
	if len(initialMessage) > 0 {
		respMsg, complete, err = neg.Reconcile(initialMessage)
		if err != nil {
			log.E.F("NEG-OPEN: reconcile failed: %v", err)
			return nil, nil, nil, false, fmt.Sprintf("reconcile failed: %v", err), nil
		}
		log.I.F("NEG-OPEN: reconcile complete=%v, response len=%d", complete, len(respMsg))
		// Debug: dump first bytes and initial message first bytes
		if len(respMsg) > 0 {
			end := 64
			if end > len(respMsg) {
				end = len(respMsg)
			}
			log.D.F("NEG-OPEN: initial msg first 64 bytes: %x", initialMessage[:min(64, len(initialMessage))])
			log.D.F("NEG-OPEN: response first 64 bytes: %x", respMsg[:end])
		}
	} else {
		// No initial message, start as server (initiator)
		respMsg, err = neg.Start()
		if err != nil {
			log.E.F("NEG-OPEN: failed to start: %v", err)
			return nil, nil, nil, false, fmt.Sprintf("failed to start: %v", err), nil
		}
		log.D.F("NEG-OPEN: started negentropy, initial msg len=%d", len(respMsg))
	}

	// Collect IDs we have that client needs (to send as events)
	haveIDs := neg.CollectHaves()
	var haveIDBytes [][]byte
	for _, id := range haveIDs {
		if decoded, err := hex.DecodeString(id); err == nil {
			haveIDBytes = append(haveIDBytes, decoded)
		}
	}

	// Collect IDs we need from client
	needIDs := neg.CollectHaveNots()
	var needIDBytes [][]byte
	for _, id := range needIDs {
		if decoded, err := hex.DecodeString(id); err == nil {
			needIDBytes = append(needIDBytes, decoded)
		}
	}

	log.I.F("NEG-OPEN: complete=%v, haves=%d, needs=%d, response len=%d",
		complete, len(haveIDs), len(needIDs), len(respMsg))

	return respMsg, haveIDBytes, needIDBytes, complete, "", nil
}

// HandleNegMsg processes a NEG-MSG message from a client.
func (h *EmbeddedHandler) HandleNegMsg(ctx context.Context, connectionID, subscriptionID string, message []byte) ([]byte, [][]byte, [][]byte, bool, string, error) {
	// Update session activity
	h.mgr.UpdateSessionActivity(connectionID, subscriptionID)

	// Look up session
	session, ok := h.mgr.GetSession(connectionID, subscriptionID)
	if !ok {
		return nil, nil, nil, false, "session not found", nil
	}

	neg := session.GetNegentropy()
	if neg == nil {
		return nil, nil, nil, false, "session has no negentropy state", nil
	}

	// Process the message
	respMsg, complete, err := neg.Reconcile(message)
	if err != nil {
		log.E.F("NEG-MSG: reconcile failed: %v", err)
		return nil, nil, nil, false, fmt.Sprintf("reconcile failed: %v", err), nil
	}

	// Collect IDs we have that client needs
	haveIDs := neg.CollectHaves()
	var haveIDBytes [][]byte
	for _, id := range haveIDs {
		if decoded, err := hex.DecodeString(id); err == nil {
			haveIDBytes = append(haveIDBytes, decoded)
		}
	}

	// Collect IDs we need from client
	needIDs := neg.CollectHaveNots()
	var needIDBytes [][]byte
	for _, id := range needIDs {
		if decoded, err := hex.DecodeString(id); err == nil {
			needIDBytes = append(needIDBytes, decoded)
		}
	}

	log.I.F("NEG-MSG: complete=%v, haves=%d, needs=%d, response len=%d",
		complete, len(haveIDs), len(needIDs), len(respMsg))

	return respMsg, haveIDBytes, needIDBytes, complete, "", nil
}

// HandleNegClose processes a NEG-CLOSE message from a client.
func (h *EmbeddedHandler) HandleNegClose(ctx context.Context, connectionID, subscriptionID string) error {
	h.mgr.CloseSession(connectionID, subscriptionID)
	return nil
}

// ListSessions returns active client negentropy sessions.
func (h *EmbeddedHandler) ListSessions(ctx context.Context) ([]*negentropyiface.ClientSession, error) {
	sessions := h.mgr.ListSessions()

	result := make([]*negentropyiface.ClientSession, 0, len(sessions))
	for _, sess := range sessions {
		result = append(result, &negentropyiface.ClientSession{
			SubscriptionID: sess.SubscriptionID,
			ConnectionID:   sess.ConnectionID,
			CreatedAt:      sess.CreatedAt.Unix(),
			LastActivity:   sess.LastActivity.Unix(),
			RoundCount:     sess.RoundCount,
		})
	}
	return result, nil
}

// CloseSession forcefully closes a client session.
func (h *EmbeddedHandler) CloseSession(ctx context.Context, connectionID, subscriptionID string) error {
	if connectionID == "" {
		// Close all sessions with this subscription ID
		sessions := h.mgr.ListSessions()
		for _, sess := range sessions {
			if sess.SubscriptionID == subscriptionID {
				h.mgr.CloseSession(sess.ConnectionID, sess.SubscriptionID)
			}
		}
	} else {
		h.mgr.CloseSession(connectionID, subscriptionID)
	}
	return nil
}

// buildStorageForFilter creates a negentropy Vector from local events matching the filter.
func (h *EmbeddedHandler) buildStorageForFilter(ctx context.Context, protoFilter *commonv1.Filter) (*negentropylib.Vector, error) {
	storage := negentropylib.NewVector()

	// Convert proto filter to nostr filter
	f := protoToFilter(protoFilter)

	// If no filter provided, use a reasonable limit
	if f == nil {
		limit := uint(1000000)
		f = &filter.F{Limit: &limit}
	}
	if f.Limit == nil {
		limit := uint(1000000)
		f.Limit = &limit
	}

	// Query events from database
	idPkTs, err := h.db.QueryForIds(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	for _, item := range idPkTs {
		storage.Insert(item.Ts, item.IDHex())
	}

	storage.Seal()
	return storage, nil
}

// protoToFilter converts a proto filter to a nostr filter.
func protoToFilter(pf *commonv1.Filter) *filter.F {
	if pf == nil {
		return nil
	}

	f := &filter.F{}

	// Convert IDs (binary 32-byte event IDs)
	if len(pf.Ids) > 0 {
		f.Ids = &tag.T{T: pf.Ids}
	}

	// Convert Kinds (uint32 → kind.K with uint16)
	if len(pf.Kinds) > 0 {
		ks := kind.NewWithCap(len(pf.Kinds))
		for _, k := range pf.Kinds {
			ks.K = append(ks.K, kind.New(k))
		}
		f.Kinds = ks
	}

	// Convert Authors (binary 32-byte pubkeys)
	if len(pf.Authors) > 0 {
		f.Authors = &tag.T{T: pf.Authors}
	}

	// Convert Since
	if pf.Since != nil {
		f.Since = timestamp.New(*pf.Since)
	}

	// Convert Until
	if pf.Until != nil {
		f.Until = timestamp.New(*pf.Until)
	}

	// Convert Limit
	if pf.Limit != nil {
		limit := uint(*pf.Limit)
		f.Limit = &limit
	}

	return f
}
