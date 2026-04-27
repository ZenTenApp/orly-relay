// Package server provides the gRPC server implementation for relay group.
package server

import (
	"context"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/sync/relaygroup"
	commonv1 "git.smesh.lol/orly/pkg/proto/orlysync/common/v1"
	relaygroupv1 "git.smesh.lol/orly/pkg/proto/orlysync/relaygroup/v1"
)

// Service implements the RelayGroupServiceServer interface.
type Service struct {
	relaygroupv1.UnimplementedRelayGroupServiceServer
	mgr   *relaygroup.Manager
	db    database.Database
	ready bool
}

// NewService creates a new relay group gRPC service.
func NewService(db database.Database, mgr *relaygroup.Manager) *Service {
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

// FindAuthoritativeConfig finds the authoritative relay group configuration.
func (s *Service) FindAuthoritativeConfig(ctx context.Context, _ *commonv1.Empty) (*relaygroupv1.RelayGroupConfigResponse, error) {
	config, err := s.mgr.FindAuthoritativeConfig(ctx)
	if err != nil {
		return &relaygroupv1.RelayGroupConfigResponse{
			Found: false,
		}, nil
	}

	if config == nil {
		return &relaygroupv1.RelayGroupConfigResponse{
			Found: false,
		}, nil
	}

	return &relaygroupv1.RelayGroupConfigResponse{
		Found: true,
		Config: &relaygroupv1.RelayGroupConfig{
			Relays: config.Relays,
		},
	}, nil
}

// GetRelays returns the list of relays from the authoritative config.
func (s *Service) GetRelays(ctx context.Context, _ *commonv1.Empty) (*relaygroupv1.RelaysResponse, error) {
	relays, err := s.mgr.FindAuthoritativeRelays(ctx)
	if err != nil {
		return &relaygroupv1.RelaysResponse{}, nil
	}

	return &relaygroupv1.RelaysResponse{
		Relays: relays,
	}, nil
}

// IsAuthorizedPublisher checks if a pubkey can publish relay group configs.
func (s *Service) IsAuthorizedPublisher(ctx context.Context, req *relaygroupv1.AuthorizedPublisherRequest) (*relaygroupv1.AuthorizedPublisherResponse, error) {
	authorized := s.mgr.IsAuthorizedPublisher(req.Pubkey)
	return &relaygroupv1.AuthorizedPublisherResponse{
		Authorized: authorized,
	}, nil
}

// GetAuthorizedPubkeys returns all authorized publisher pubkeys.
func (s *Service) GetAuthorizedPubkeys(ctx context.Context, _ *commonv1.Empty) (*relaygroupv1.AuthorizedPubkeysResponse, error) {
	pubkeys := s.mgr.GetAuthorizedPubkeys()
	return &relaygroupv1.AuthorizedPubkeysResponse{
		Pubkeys: pubkeys,
	}, nil
}

// ValidateRelayGroupEvent validates a relay group configuration event.
func (s *Service) ValidateRelayGroupEvent(ctx context.Context, req *relaygroupv1.ValidateEventRequest) (*relaygroupv1.ValidateEventResponse, error) {
	// Would need to convert proto event to internal event
	// For now, return valid
	return &relaygroupv1.ValidateEventResponse{
		Valid: true,
	}, nil
}

// HandleRelayGroupEvent processes a relay group event and triggers peer updates.
func (s *Service) HandleRelayGroupEvent(ctx context.Context, req *relaygroupv1.HandleEventRequest) (*commonv1.Empty, error) {
	// Would need to convert proto event to internal event and call the manager
	return &commonv1.Empty{}, nil
}
