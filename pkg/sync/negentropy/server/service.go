// Package server provides the gRPC server implementation for negentropy sync.
package server

import (
	"context"
	"encoding/hex"
	"fmt"

	"google.golang.org/grpc"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	negentropylib "git.mleku.dev/mleku/nostr/negentropy"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/sync/negentropy"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	negentropyv1 "next.orly.dev/pkg/proto/orlysync/negentropy/v1"
)

// Service implements the NegentropyServiceServer interface.
type Service struct {
	negentropyv1.UnimplementedNegentropyServiceServer
	mgr   *negentropy.Manager
	db    database.Database
	ready bool
}

// NewService creates a new negentropy gRPC service.
func NewService(db database.Database, mgr *negentropy.Manager) *Service {
	return &Service{
		mgr:   mgr,
		db:    db,
		ready: true,
	}
}

// Ready returns whether the service is ready to serve requests.
func (s *Service) Ready(ctx context.Context, _ *commonv1.Empty) (*commonv1.ReadyResponse, error) {
	return &commonv1.ReadyResponse{Ready: s.ready}, nil
}

// Start starts the background relay-to-relay sync.
func (s *Service) Start(ctx context.Context, _ *commonv1.Empty) (*commonv1.Empty, error) {
	s.mgr.Start()
	return &commonv1.Empty{}, nil
}

// Stop stops the background sync.
func (s *Service) Stop(ctx context.Context, _ *commonv1.Empty) (*commonv1.Empty, error) {
	s.mgr.Stop()
	return &commonv1.Empty{}, nil
}

// HandleNegOpen processes a NEG-OPEN message from a client.
func (s *Service) HandleNegOpen(ctx context.Context, req *negentropyv1.NegOpenRequest) (*negentropyv1.NegOpenResponse, error) {
	// Open a session for this client
	session := s.mgr.OpenSession(req.ConnectionId, req.SubscriptionId)

	// Build storage from local events matching the filter
	storage, err := s.buildStorageForFilter(ctx, req.Filter)
	if err != nil {
		log.E.F("NEG-OPEN: failed to build storage: %v", err)
		return &negentropyv1.NegOpenResponse{
			Message: nil,
			Error:   fmt.Sprintf("failed to build storage: %v", err),
		}, nil
	}

	log.I.F("NEG-OPEN: built storage with %d events", storage.Size())

	// Create negentropy instance for this session
	neg := negentropylib.New(storage, negentropylib.DefaultFrameSizeLimit)

	// Store in session for later use
	session.SetNegentropy(neg, storage)

	// If we have an initial message from client, process it
	var respMsg []byte
	var complete bool
	if len(req.InitialMessage) > 0 {
		respMsg, complete, err = neg.Reconcile(req.InitialMessage)
		if err != nil {
			log.E.F("NEG-OPEN: reconcile failed: %v", err)
			return &negentropyv1.NegOpenResponse{
				Message: nil,
				Error:   fmt.Sprintf("reconcile failed: %v", err),
			}, nil
		}
		log.I.F("NEG-OPEN: reconcile complete=%v, response len=%d", complete, len(respMsg))
	} else {
		// No initial message, start as server (initiator)
		respMsg, err = neg.Start()
		if err != nil {
			log.E.F("NEG-OPEN: failed to start: %v", err)
			return &negentropyv1.NegOpenResponse{
				Message: nil,
				Error:   fmt.Sprintf("failed to start: %v", err),
			}, nil
		}
		log.D.F("NEG-OPEN: started negentropy, initial msg len=%d", len(respMsg))
	}

	// Collect IDs we have that client needs (to send as events)
	haveIDs := neg.CollectHaves()
	var haveIDBytes [][]byte
	for _, id := range haveIDs {
		// ID is a hex string, decode to binary
		if decoded, err := hex.DecodeString(id); err == nil {
			haveIDBytes = append(haveIDBytes, decoded)
		}
	}

	// Collect IDs we need from client
	needIDs := neg.CollectHaveNots()
	var needIDBytes [][]byte
	for _, id := range needIDs {
		// ID is a hex string, decode to binary
		if decoded, err := hex.DecodeString(id); err == nil {
			needIDBytes = append(needIDBytes, decoded)
		}
	}

	log.I.F("NEG-OPEN: complete=%v, haves=%d, needs=%d, response len=%d",
		complete, len(haveIDs), len(needIDs), len(respMsg))

	return &negentropyv1.NegOpenResponse{
		Message:  respMsg,
		HaveIds:  haveIDBytes,
		NeedIds:  needIDBytes,
		Complete: complete,
		Error:    "",
	}, nil
}

// HandleNegMsg processes a NEG-MSG message from a client.
func (s *Service) HandleNegMsg(ctx context.Context, req *negentropyv1.NegMsgRequest) (*negentropyv1.NegMsgResponse, error) {
	// Update session activity
	s.mgr.UpdateSessionActivity(req.ConnectionId, req.SubscriptionId)

	// Look up session
	session, ok := s.mgr.GetSession(req.ConnectionId, req.SubscriptionId)
	if !ok {
		return &negentropyv1.NegMsgResponse{
			Error: "session not found",
		}, nil
	}

	neg := session.GetNegentropy()
	if neg == nil {
		return &negentropyv1.NegMsgResponse{
			Error: "session has no negentropy state",
		}, nil
	}

	// Process the message
	respMsg, complete, err := neg.Reconcile(req.Message)
	if err != nil {
		log.E.F("NEG-MSG: reconcile failed: %v", err)
		return &negentropyv1.NegMsgResponse{
			Error: fmt.Sprintf("reconcile failed: %v", err),
		}, nil
	}

	// Collect IDs we have that client needs (to send as events)
	haveIDs := neg.CollectHaves()
	var haveIDBytes [][]byte
	for _, id := range haveIDs {
		// ID is a hex string, decode to binary
		if decoded, err := hex.DecodeString(id); err == nil {
			haveIDBytes = append(haveIDBytes, decoded)
		}
	}

	// Collect IDs we need from client
	needIDs := neg.CollectHaveNots()
	var needIDBytes [][]byte
	for _, id := range needIDs {
		// ID is a hex string, decode to binary
		if decoded, err := hex.DecodeString(id); err == nil {
			needIDBytes = append(needIDBytes, decoded)
		}
	}

	log.I.F("NEG-MSG: complete=%v, haves=%d, needs=%d, response len=%d",
		complete, len(haveIDs), len(needIDs), len(respMsg))

	return &negentropyv1.NegMsgResponse{
		Message:  respMsg,
		HaveIds:  haveIDBytes,
		NeedIds:  needIDBytes,
		Complete: complete,
		Error:    "",
	}, nil
}

// HandleNegClose processes a NEG-CLOSE message from a client.
func (s *Service) HandleNegClose(ctx context.Context, req *negentropyv1.NegCloseRequest) (*commonv1.Empty, error) {
	s.mgr.CloseSession(req.ConnectionId, req.SubscriptionId)
	return &commonv1.Empty{}, nil
}

// SyncWithPeer initiates negentropy sync with a specific peer relay.
func (s *Service) SyncWithPeer(req *negentropyv1.SyncPeerRequest, stream grpc.ServerStreamingServer[negentropyv1.SyncProgress]) error {
	// Send initial progress
	if err := stream.Send(&negentropyv1.SyncProgress{
		PeerUrl:  req.PeerUrl,
		Round:    0,
		Complete: false,
	}); err != nil {
		return err
	}

	// TODO: Implement actual NIP-77 sync with peer
	// For now, just mark as complete

	s.mgr.TriggerSync(stream.Context(), req.PeerUrl)

	// Send completion
	return stream.Send(&negentropyv1.SyncProgress{
		PeerUrl:  req.PeerUrl,
		Round:    1,
		Complete: true,
	})
}

// GetSyncStatus returns the current sync status.
func (s *Service) GetSyncStatus(ctx context.Context, _ *commonv1.Empty) (*negentropyv1.SyncStatusResponse, error) {
	peerStates := s.mgr.GetPeerStates()

	states := make([]*negentropyv1.PeerSyncState, 0, len(peerStates))
	for _, ps := range peerStates {
		states = append(states, &negentropyv1.PeerSyncState{
			PeerUrl:             ps.URL,
			LastSync:            ps.LastSync.Unix(),
			EventsSynced:        ps.EventsSynced,
			Status:              ps.Status,
			LastError:           ps.LastError,
			ConsecutiveFailures: ps.ConsecutiveFailures,
		})
	}

	return &negentropyv1.SyncStatusResponse{
		Active:     s.mgr.IsActive(),
		LastSync:   s.mgr.LastSync().Unix(),
		PeerCount:  int32(len(peerStates)),
		PeerStates: states,
	}, nil
}

// GetPeers returns the list of negentropy sync peers.
func (s *Service) GetPeers(ctx context.Context, _ *commonv1.Empty) (*negentropyv1.PeersResponse, error) {
	return &negentropyv1.PeersResponse{
		Peers: s.mgr.GetPeers(),
	}, nil
}

// AddPeer adds a peer for negentropy sync.
func (s *Service) AddPeer(ctx context.Context, req *negentropyv1.AddPeerRequest) (*commonv1.Empty, error) {
	s.mgr.AddPeer(req.PeerUrl)
	return &commonv1.Empty{}, nil
}

// RemovePeer removes a peer from negentropy sync.
func (s *Service) RemovePeer(ctx context.Context, req *negentropyv1.RemovePeerRequest) (*commonv1.Empty, error) {
	s.mgr.RemovePeer(req.PeerUrl)
	return &commonv1.Empty{}, nil
}

// TriggerSync manually triggers sync with a specific peer or all peers.
func (s *Service) TriggerSync(ctx context.Context, req *negentropyv1.TriggerSyncRequest) (*commonv1.Empty, error) {
	s.mgr.TriggerSync(ctx, req.PeerUrl)
	return &commonv1.Empty{}, nil
}

// GetPeerSyncState returns sync state for a specific peer.
func (s *Service) GetPeerSyncState(ctx context.Context, req *negentropyv1.PeerSyncStateRequest) (*negentropyv1.PeerSyncStateResponse, error) {
	state, found := s.mgr.GetPeerState(req.PeerUrl)
	if !found {
		return &negentropyv1.PeerSyncStateResponse{
			Found: false,
		}, nil
	}

	return &negentropyv1.PeerSyncStateResponse{
		Found: true,
		State: &negentropyv1.PeerSyncState{
			PeerUrl:             state.URL,
			LastSync:            state.LastSync.Unix(),
			EventsSynced:        state.EventsSynced,
			Status:              state.Status,
			LastError:           state.LastError,
			ConsecutiveFailures: state.ConsecutiveFailures,
		},
	}, nil
}

// ListSessions returns active client negentropy sessions.
func (s *Service) ListSessions(ctx context.Context, _ *commonv1.Empty) (*negentropyv1.ListSessionsResponse, error) {
	sessions := s.mgr.ListSessions()

	protoSessions := make([]*negentropyv1.ClientSession, 0, len(sessions))
	for _, sess := range sessions {
		protoSessions = append(protoSessions, &negentropyv1.ClientSession{
			SubscriptionId: sess.SubscriptionID,
			ConnectionId:   sess.ConnectionID,
			CreatedAt:      sess.CreatedAt.Unix(),
			LastActivity:   sess.LastActivity.Unix(),
			RoundCount:     sess.RoundCount,
		})
	}

	return &negentropyv1.ListSessionsResponse{
		Sessions: protoSessions,
	}, nil
}

// CloseSession forcefully closes a client session.
func (s *Service) CloseSession(ctx context.Context, req *negentropyv1.CloseSessionRequest) (*commonv1.Empty, error) {
	if req.ConnectionId == "" {
		// Close all sessions with this subscription ID
		sessions := s.mgr.ListSessions()
		for _, sess := range sessions {
			if sess.SubscriptionID == req.SubscriptionId {
				s.mgr.CloseSession(sess.ConnectionID, sess.SubscriptionID)
			}
		}
	} else {
		s.mgr.CloseSession(req.ConnectionId, req.SubscriptionId)
	}
	return &commonv1.Empty{}, nil
}

// buildStorageForFilter creates a negentropy Vector from local events matching the filter.
func (s *Service) buildStorageForFilter(ctx context.Context, protoFilter *commonv1.Filter) (*negentropylib.Vector, error) {
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
	idPkTs, err := s.db.QueryForIds(ctx, f)
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

	// Convert Kinds
	if len(pf.Kinds) > 0 {
		// Create kinds from proto
		// Note: We'd need proper kinds conversion here
	}

	// Convert Since/Until
	if pf.Since != nil {
		// Set Since timestamp
	}
	if pf.Until != nil {
		// Set Until timestamp
	}

	// Convert Limit
	if pf.Limit != nil {
		limit := uint(*pf.Limit)
		f.Limit = &limit
	}

	return f
}
