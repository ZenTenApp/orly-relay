// Package server provides the gRPC server implementation for cluster sync.
package server

import (
	"context"
	"encoding/json"
	"net/http"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/sync/cluster"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	clusterv1 "next.orly.dev/pkg/proto/orlysync/cluster/v1"
)

// Service implements the ClusterSyncServiceServer interface.
type Service struct {
	clusterv1.UnimplementedClusterSyncServiceServer
	mgr   *cluster.Manager
	db    database.Database
	ready bool
}

// NewService creates a new cluster sync gRPC service.
func NewService(db database.Database, mgr *cluster.Manager) *Service {
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

// Start starts the cluster polling loop.
func (s *Service) Start(ctx context.Context, _ *commonv1.Empty) (*commonv1.Empty, error) {
	s.mgr.Start()
	return &commonv1.Empty{}, nil
}

// Stop stops the cluster polling loop.
func (s *Service) Stop(ctx context.Context, _ *commonv1.Empty) (*commonv1.Empty, error) {
	s.mgr.Stop()
	return &commonv1.Empty{}, nil
}

// HandleLatestSerial proxies GET /cluster/latest HTTP requests.
func (s *Service) HandleLatestSerial(ctx context.Context, req *commonv1.HTTPRequest) (*commonv1.HTTPResponse, error) {
	if req.Method != http.MethodGet {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       []byte("Method not allowed"),
		}, nil
	}

	serial, timestamp := s.mgr.GetLatestSerial()
	resp := map[string]any{
		"serial":    serial,
		"timestamp": timestamp,
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

// HandleEventsRange proxies GET /cluster/events HTTP requests.
func (s *Service) HandleEventsRange(ctx context.Context, req *commonv1.HTTPRequest) (*commonv1.HTTPResponse, error) {
	if req.Method != http.MethodGet {
		return &commonv1.HTTPResponse{
			StatusCode: http.StatusMethodNotAllowed,
			Body:       []byte("Method not allowed"),
		}, nil
	}

	// Parse query parameters
	// In practice, the query string would be parsed from req.QueryString
	// For now, return a placeholder
	return &commonv1.HTTPResponse{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       []byte(`{"events":[],"has_more":false}`),
	}, nil
}

// GetMembers returns the current cluster members.
func (s *Service) GetMembers(ctx context.Context, _ *commonv1.Empty) (*clusterv1.MembersResponse, error) {
	members := s.mgr.GetMembers()
	clusterMembers := make([]*clusterv1.ClusterMember, 0, len(members))
	for _, m := range members {
		clusterMembers = append(clusterMembers, &clusterv1.ClusterMember{
			HttpUrl:      m.HTTPURL,
			WebsocketUrl: m.WebSocketURL,
			LastSerial:   m.LastSerial,
			LastPoll:     m.LastPoll.Unix(),
			Status:       m.Status,
			ErrorCount:   int32(m.ErrorCount),
		})
	}
	return &clusterv1.MembersResponse{
		Members: clusterMembers,
	}, nil
}

// UpdateMembership updates cluster membership.
func (s *Service) UpdateMembership(ctx context.Context, req *clusterv1.UpdateMembershipRequest) (*commonv1.Empty, error) {
	s.mgr.UpdateMembership(req.RelayUrls)
	return &commonv1.Empty{}, nil
}

// HandleMembershipEvent processes a cluster membership event (Kind 39108).
func (s *Service) HandleMembershipEvent(ctx context.Context, req *clusterv1.MembershipEventRequest) (*commonv1.Empty, error) {
	// Convert proto event to internal format and process
	// This would need the actual event conversion
	return &commonv1.Empty{}, nil
}

// GetClusterStatus returns overall cluster status.
func (s *Service) GetClusterStatus(ctx context.Context, _ *commonv1.Empty) (*clusterv1.ClusterStatusResponse, error) {
	members := s.mgr.GetMembers()
	serial, _ := s.mgr.GetLatestSerial()

	activeCount := int32(0)
	clusterMembers := make([]*clusterv1.ClusterMember, 0, len(members))
	for _, m := range members {
		if m.Status == "active" {
			activeCount++
		}
		clusterMembers = append(clusterMembers, &clusterv1.ClusterMember{
			HttpUrl:      m.HTTPURL,
			WebsocketUrl: m.WebSocketURL,
			LastSerial:   m.LastSerial,
			LastPoll:     m.LastPoll.Unix(),
			Status:       m.Status,
			ErrorCount:   int32(m.ErrorCount),
		})
	}

	return &clusterv1.ClusterStatusResponse{
		LatestSerial:              serial,
		ActiveMembers:             activeCount,
		TotalMembers:              int32(len(members)),
		PropagatePrivilegedEvents: s.mgr.PropagatePrivilegedEvents(),
		Members:                   clusterMembers,
	}, nil
}

// GetMemberStatus returns status for a specific member.
func (s *Service) GetMemberStatus(ctx context.Context, req *clusterv1.MemberStatusRequest) (*clusterv1.MemberStatusResponse, error) {
	member := s.mgr.GetMember(req.HttpUrl)
	if member == nil {
		return &clusterv1.MemberStatusResponse{
			Found: false,
		}, nil
	}

	return &clusterv1.MemberStatusResponse{
		Found: true,
		Member: &clusterv1.ClusterMember{
			HttpUrl:      member.HTTPURL,
			WebsocketUrl: member.WebSocketURL,
			LastSerial:   member.LastSerial,
			LastPoll:     member.LastPoll.Unix(),
			Status:       member.Status,
			ErrorCount:   int32(member.ErrorCount),
		},
	}, nil
}

// GetLatestSerial returns the latest serial from this relay's database.
func (s *Service) GetLatestSerial(ctx context.Context, _ *commonv1.Empty) (*clusterv1.LatestSerialResponse, error) {
	serial, timestamp := s.mgr.GetLatestSerial()
	return &clusterv1.LatestSerialResponse{
		Serial:    serial,
		Timestamp: timestamp,
	}, nil
}

// GetEventsInRange returns event info for a serial range.
func (s *Service) GetEventsInRange(ctx context.Context, req *clusterv1.EventsRangeRequest) (*clusterv1.EventsRangeResponse, error) {
	events, hasMore, nextFrom := s.mgr.GetEventsInRange(req.From, req.To, int(req.Limit))

	eventInfos := make([]*clusterv1.EventInfo, 0, len(events))
	for _, e := range events {
		eventInfos = append(eventInfos, &clusterv1.EventInfo{
			Serial:    e.Serial,
			Id:        e.ID,
			Timestamp: e.Timestamp, // Already int64
		})
	}

	return &clusterv1.EventsRangeResponse{
		Events:   eventInfos,
		HasMore:  hasMore,
		NextFrom: nextFrom,
	}, nil
}
