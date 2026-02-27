// Package server provides the gRPC server implementation for distributed sync.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/sync/distributed"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	distributedv1 "next.orly.dev/pkg/proto/orlysync/distributed/v1"
)

// Service implements the DistributedSyncServiceServer interface.
type Service struct {
	distributedv1.UnimplementedDistributedSyncServiceServer
	mgr   *distributed.Manager
	db    database.Database
	ready bool
}

// NewService creates a new distributed sync gRPC service.
func NewService(db database.Database, mgr *distributed.Manager) *Service {
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

// GetInfo returns current sync service information.
func (s *Service) GetInfo(ctx context.Context, _ *commonv1.Empty) (*commonv1.SyncInfo, error) {
	peers := s.mgr.GetPeers()
	return &commonv1.SyncInfo{
		NodeId:        s.mgr.GetNodeID(),
		RelayUrl:      s.mgr.GetRelayURL(),
		CurrentSerial: s.mgr.GetCurrentSerial(),
		PeerCount:     int32(len(peers)),
		Status:        "running",
	}, nil
}

// GetCurrentSerial returns this relay's current serial number.
func (s *Service) GetCurrentSerial(ctx context.Context, req *distributedv1.CurrentRequest) (*distributedv1.CurrentResponse, error) {
	return &distributedv1.CurrentResponse{
		NodeId:   s.mgr.GetNodeID(),
		RelayUrl: s.mgr.GetRelayURL(),
		Serial:   s.mgr.GetCurrentSerial(),
	}, nil
}

// GetEventIDs returns event IDs for a serial range.
func (s *Service) GetEventIDs(ctx context.Context, req *distributedv1.EventIDsRequest) (*distributedv1.EventIDsResponse, error) {
	eventMap, err := s.mgr.GetEventsWithIDs(req.From, req.To)
	if err != nil {
		return nil, err
	}

	return &distributedv1.EventIDsResponse{
		EventMap: eventMap,
	}, nil
}

// HandleCurrentRequest proxies /api/sync/current HTTP requests.
func (s *Service) HandleCurrentRequest(ctx context.Context, req *commonv1.HTTPRequest) (*commonv1.HTTPResponse, error) {
	if req.Method != http.MethodPost {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       []byte("Method not allowed"),
		}, nil
	}

	var currentReq distributed.CurrentRequest
	if err := json.Unmarshal(req.Body, &currentReq); err != nil {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusBadRequest,
			Body:       []byte("Invalid JSON"),
		}, nil
	}

	// Check if request is from ourselves
	if s.mgr.IsSelfNodeID(currentReq.NodeID) {
		if currentReq.RelayURL != "" {
			s.mgr.MarkSelfURL(currentReq.RelayURL)
		}
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusBadRequest,
			Body:       []byte("Cannot sync with self"),
		}, nil
	}

	resp := distributed.CurrentResponse{
		NodeID:   s.mgr.GetNodeID(),
		RelayURL: s.mgr.GetRelayURL(),
		Serial:   s.mgr.GetCurrentSerial(),
	}

	respBody, err := json.Marshal(resp)
	if err != nil {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("Failed to marshal response"),
		}, nil
	}

	return &commonv1.HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       respBody,
	}, nil
}

// HandleEventIDsRequest proxies /api/sync/event-ids HTTP requests.
func (s *Service) HandleEventIDsRequest(ctx context.Context, req *commonv1.HTTPRequest) (*commonv1.HTTPResponse, error) {
	if req.Method != http.MethodPost {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       []byte("Method not allowed"),
		}, nil
	}

	var eventIDsReq distributed.EventIDsRequest
	if err := json.Unmarshal(req.Body, &eventIDsReq); err != nil {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusBadRequest,
			Body:       []byte("Invalid JSON"),
		}, nil
	}

	// Check if request is from ourselves
	if s.mgr.IsSelfNodeID(eventIDsReq.NodeID) {
		if eventIDsReq.RelayURL != "" {
			s.mgr.MarkSelfURL(eventIDsReq.RelayURL)
		}
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusBadRequest,
			Body:       []byte("Cannot sync with self"),
		}, nil
	}

	eventMap, err := s.mgr.GetEventsWithIDs(eventIDsReq.From, eventIDsReq.To)
	if err != nil {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("Failed to get event IDs: " + err.Error()),
		}, nil
	}

	resp := distributed.EventIDsResponse{
		EventMap: eventMap,
	}

	respBody, err := json.Marshal(resp)
	if err != nil {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("Failed to marshal response"),
		}, nil
	}

	return &commonv1.HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       respBody,
	}, nil
}

// GetPeers returns the current list of sync peers.
func (s *Service) GetPeers(ctx context.Context, _ *commonv1.Empty) (*distributedv1.PeersResponse, error) {
	peers := s.mgr.GetPeers()
	return &distributedv1.PeersResponse{
		Peers: peers,
	}, nil
}

// UpdatePeers updates the peer list.
func (s *Service) UpdatePeers(ctx context.Context, req *distributedv1.UpdatePeersRequest) (*commonv1.Empty, error) {
	s.mgr.UpdatePeers(req.Peers)
	return &commonv1.Empty{}, nil
}

// IsAuthorizedPeer checks if a peer is authorized by validating its NIP-11 pubkey.
func (s *Service) IsAuthorizedPeer(ctx context.Context, req *distributedv1.AuthorizedPeerRequest) (*distributedv1.AuthorizedPeerResponse, error) {
	authorized := s.mgr.IsAuthorizedPeer(req.PeerUrl, req.ExpectedPubkey)
	return &distributedv1.AuthorizedPeerResponse{
		Authorized: authorized,
	}, nil
}

// GetPeerPubkey fetches the pubkey for a peer relay via NIP-11.
func (s *Service) GetPeerPubkey(ctx context.Context, req *distributedv1.PeerPubkeyRequest) (*distributedv1.PeerPubkeyResponse, error) {
	pubkey, err := s.mgr.GetPeerPubkey(req.PeerUrl)
	if err != nil {
		// Return empty pubkey on error since the proto doesn't have an error field
		return &distributedv1.PeerPubkeyResponse{
			Pubkey: "",
		}, nil
	}
	return &distributedv1.PeerPubkeyResponse{
		Pubkey: pubkey,
	}, nil
}

// UpdateSerial updates the current serial from database.
func (s *Service) UpdateSerial(ctx context.Context, _ *commonv1.Empty) (*commonv1.Empty, error) {
	s.mgr.UpdateSerial()
	return &commonv1.Empty{}, nil
}

// NotifyNewEvent notifies the service of a new event being stored.
func (s *Service) NotifyNewEvent(ctx context.Context, req *distributedv1.NewEventNotification) (*commonv1.Empty, error) {
	s.mgr.NotifyNewEvent(req.EventId, req.Serial)
	return &commonv1.Empty{}, nil
}

// TriggerSync manually triggers a sync cycle with all peers.
func (s *Service) TriggerSync(ctx context.Context, _ *commonv1.Empty) (*commonv1.Empty, error) {
	log.I.F("manual sync trigger requested")
	return &commonv1.Empty{}, nil
}

// GetSyncStatus returns current sync status for all peers.
func (s *Service) GetSyncStatus(ctx context.Context, _ *commonv1.Empty) (*distributedv1.SyncStatusResponse, error) {
	peerStatus := s.mgr.GetPeerStatus()
	peerInfos := make([]*commonv1.PeerInfo, 0, len(peerStatus))
	for url, serial := range peerStatus {
		peerInfos = append(peerInfos, &commonv1.PeerInfo{
			Url:        url,
			LastSerial: serial,
			Status:     "active",
		})
	}
	return &distributedv1.SyncStatusResponse{
		CurrentSerial: s.mgr.GetCurrentSerial(),
		Peers:         peerInfos,
	}, nil
}

// httpResponseWriter is a simple buffer for capturing HTTP responses.
type httpResponseWriter struct {
	statusCode int
	headers    http.Header
	body       bytes.Buffer
}

func (w *httpResponseWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *httpResponseWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *httpResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}
