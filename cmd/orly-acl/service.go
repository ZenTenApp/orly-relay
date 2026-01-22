package main

import (
	"context"
	"encoding/hex"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"lol.mleku.dev/log"

	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	orlyaclv1 "next.orly.dev/pkg/proto/orlyacl/v1"
	orlydbv1 "next.orly.dev/pkg/proto/orlydb/v1"
)

// ACLService implements the orlyaclv1.ACLServiceServer interface.
type ACLService struct {
	orlyaclv1.UnimplementedACLServiceServer
	cfg   *Config
	db    database.Database
	ready bool
}

// NewACLService creates a new ACL service.
func NewACLService(cfg *Config, db database.Database) *ACLService {
	return &ACLService{
		cfg:   cfg,
		db:    db,
		ready: false, // Not ready until Configure completes
	}
}

// SetReady marks the service as ready (or not ready).
func (s *ACLService) SetReady(ready bool) {
	s.ready = ready
	if ready {
		log.I.F("ACL service is now ready")
	}
}

// === Core ACL Methods ===

func (s *ACLService) GetAccessLevel(ctx context.Context, req *orlyaclv1.AccessLevelRequest) (*orlyaclv1.AccessLevelResponse, error) {
	level := acl.Registry.GetAccessLevel(req.Pubkey, req.Address)
	return &orlyaclv1.AccessLevelResponse{Level: level}, nil
}

func (s *ACLService) CheckPolicy(ctx context.Context, req *orlyaclv1.PolicyCheckRequest) (*orlyaclv1.PolicyCheckResponse, error) {
	ev := orlydbv1.ProtoToEvent(req.Event)
	allowed, err := acl.Registry.CheckPolicy(ev)
	resp := &orlyaclv1.PolicyCheckResponse{Allowed: allowed}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp, nil
}

func (s *ACLService) GetACLInfo(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ACLInfoResponse, error) {
	name, description, documentation := acl.Registry.GetACLInfo()
	return &orlyaclv1.ACLInfoResponse{
		Name:          name,
		Description:   description,
		Documentation: documentation,
	}, nil
}

func (s *ACLService) GetMode(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ModeResponse, error) {
	return &orlyaclv1.ModeResponse{Mode: acl.Registry.Type()}, nil
}

func (s *ACLService) Ready(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ReadyResponse, error) {
	// Check if service is fully configured and ready
	return &orlyaclv1.ReadyResponse{Ready: s.ready}, nil
}

// === Follows ACL Methods ===

func (s *ACLService) GetThrottleDelay(ctx context.Context, req *orlyaclv1.ThrottleDelayRequest) (*orlyaclv1.ThrottleDelayResponse, error) {
	// Get the active ACL and check if it's Follows
	for _, i := range acl.Registry.ACL {
		if i.Type() == "follows" {
			if follows, ok := i.(*acl.Follows); ok {
				delay := follows.GetThrottleDelay(req.Pubkey, req.Ip)
				return &orlyaclv1.ThrottleDelayResponse{DelayMs: delay.Milliseconds()}, nil
			}
		}
	}
	return &orlyaclv1.ThrottleDelayResponse{DelayMs: 0}, nil
}

func (s *ACLService) AddFollow(ctx context.Context, req *orlyaclv1.AddFollowRequest) (*orlyaclv1.Empty, error) {
	acl.Registry.AddFollow(req.Pubkey)
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) GetFollowedPubkeys(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.FollowedPubkeysResponse, error) {
	for _, i := range acl.Registry.ACL {
		if i.Type() == "follows" {
			if follows, ok := i.(*acl.Follows); ok {
				pubkeys := follows.GetFollowedPubkeys()
				return &orlyaclv1.FollowedPubkeysResponse{Pubkeys: pubkeys}, nil
			}
		}
	}
	return &orlyaclv1.FollowedPubkeysResponse{}, nil
}

func (s *ACLService) GetAdminRelays(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.AdminRelaysResponse, error) {
	for _, i := range acl.Registry.ACL {
		if i.Type() == "follows" {
			if follows, ok := i.(*acl.Follows); ok {
				urls := follows.AdminRelays()
				return &orlyaclv1.AdminRelaysResponse{Urls: urls}, nil
			}
		}
	}
	return &orlyaclv1.AdminRelaysResponse{}, nil
}

// === Managed ACL Methods ===

func (s *ACLService) BanPubkey(ctx context.Context, req *orlyaclv1.BanPubkeyRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.SaveBannedPubkey(req.Pubkey, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ban pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) UnbanPubkey(ctx context.Context, req *orlyaclv1.PubkeyRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.RemoveBannedPubkey(req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unban pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListBannedPubkeys(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListBannedPubkeysResponse, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	banned, err := managedACL.ListBannedPubkeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list banned pubkeys: %v", err)
	}
	resp := &orlyaclv1.ListBannedPubkeysResponse{}
	for _, b := range banned {
		resp.Pubkeys = append(resp.Pubkeys, &orlyaclv1.BannedPubkey{
			Pubkey: b.Pubkey,
			Reason: b.Reason,
			Added:  b.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) AllowPubkey(ctx context.Context, req *orlyaclv1.AllowPubkeyRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.SaveAllowedPubkey(req.Pubkey, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to allow pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) DisallowPubkey(ctx context.Context, req *orlyaclv1.PubkeyRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.RemoveAllowedPubkey(req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disallow pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListAllowedPubkeys(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListAllowedPubkeysResponse, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	allowed, err := managedACL.ListAllowedPubkeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list allowed pubkeys: %v", err)
	}
	resp := &orlyaclv1.ListAllowedPubkeysResponse{}
	for _, a := range allowed {
		resp.Pubkeys = append(resp.Pubkeys, &orlyaclv1.AllowedPubkey{
			Pubkey: a.Pubkey,
			Reason: a.Reason,
			Added:  a.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) BanEvent(ctx context.Context, req *orlyaclv1.BanEventRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.SaveBannedEvent(req.EventId, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to ban event: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) UnbanEvent(ctx context.Context, req *orlyaclv1.EventRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.RemoveBannedEvent(req.EventId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unban event: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListBannedEvents(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListBannedEventsResponse, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	banned, err := managedACL.ListBannedEvents()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list banned events: %v", err)
	}
	resp := &orlyaclv1.ListBannedEventsResponse{}
	for _, b := range banned {
		resp.Events = append(resp.Events, &orlyaclv1.BannedEvent{
			EventId: b.ID,
			Reason:  b.Reason,
			Added:   b.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) AllowEvent(ctx context.Context, req *orlyaclv1.BanEventRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.SaveAllowedEvent(req.EventId, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to allow event: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) DisallowEvent(ctx context.Context, req *orlyaclv1.EventRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.RemoveAllowedEvent(req.EventId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disallow event: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListAllowedEvents(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListAllowedEventsResponse, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	allowed, err := managedACL.ListAllowedEvents()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list allowed events: %v", err)
	}
	resp := &orlyaclv1.ListAllowedEventsResponse{}
	for _, a := range allowed {
		resp.Events = append(resp.Events, &orlyaclv1.AllowedEvent{
			EventId: a.ID,
			Reason:  a.Reason,
			Added:   a.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) BlockIP(ctx context.Context, req *orlyaclv1.BlockIPRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.SaveBlockedIP(req.Ip, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to block IP: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) UnblockIP(ctx context.Context, req *orlyaclv1.IPRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.RemoveBlockedIP(req.Ip); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unblock IP: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListBlockedIPs(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListBlockedIPsResponse, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	blocked, err := managedACL.ListBlockedIPs()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list blocked IPs: %v", err)
	}
	resp := &orlyaclv1.ListBlockedIPsResponse{}
	for _, b := range blocked {
		resp.Ips = append(resp.Ips, &orlyaclv1.BlockedIP{
			Ip:     b.IP,
			Reason: b.Reason,
			Added:  b.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) AllowKind(ctx context.Context, req *orlyaclv1.AllowKindRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.SaveAllowedKind(int(req.Kind)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to allow kind: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) DisallowKind(ctx context.Context, req *orlyaclv1.KindRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	if err := managedACL.RemoveAllowedKind(int(req.Kind)); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to disallow kind: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListAllowedKinds(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListAllowedKindsResponse, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managedACL := managed.GetManagedACL()
	if managedACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL database not available")
	}
	kinds, err := managedACL.ListAllowedKinds()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list allowed kinds: %v", err)
	}
	resp := &orlyaclv1.ListAllowedKindsResponse{}
	for _, k := range kinds {
		resp.Kinds = append(resp.Kinds, int32(k))
	}
	return resp, nil
}

func (s *ACLService) UpdatePeerAdmins(ctx context.Context, req *orlyaclv1.UpdatePeerAdminsRequest) (*orlyaclv1.Empty, error) {
	managed := s.getManagedACL()
	if managed == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "managed ACL not available")
	}
	managed.UpdatePeerAdmins(req.PeerPubkeys)
	return &orlyaclv1.Empty{}, nil
}

// === Curating ACL Methods ===

func (s *ACLService) TrustPubkey(ctx context.Context, req *orlyaclv1.TrustPubkeyRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	if err := curating.TrustPubkey(req.Pubkey, req.Note); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to trust pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) UntrustPubkey(ctx context.Context, req *orlyaclv1.PubkeyRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	if err := curating.UntrustPubkey(req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to untrust pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListTrustedPubkeys(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListTrustedPubkeysResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	trusted, err := curatingACL.ListTrustedPubkeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list trusted pubkeys: %v", err)
	}
	resp := &orlyaclv1.ListTrustedPubkeysResponse{}
	for _, t := range trusted {
		resp.Pubkeys = append(resp.Pubkeys, &orlyaclv1.TrustedPubkey{
			Pubkey: t.Pubkey,
			Note:   t.Note,
			Added:  t.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) BlacklistPubkey(ctx context.Context, req *orlyaclv1.BlacklistPubkeyRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	if err := curating.BlacklistPubkey(req.Pubkey, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to blacklist pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) UnblacklistPubkey(ctx context.Context, req *orlyaclv1.PubkeyRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	if err := curating.UnblacklistPubkey(req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unblacklist pubkey: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListBlacklistedPubkeys(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListBlacklistedPubkeysResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	blacklisted, err := curatingACL.ListBlacklistedPubkeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list blacklisted pubkeys: %v", err)
	}
	resp := &orlyaclv1.ListBlacklistedPubkeysResponse{}
	for _, b := range blacklisted {
		resp.Pubkeys = append(resp.Pubkeys, &orlyaclv1.BlacklistedPubkey{
			Pubkey: b.Pubkey,
			Reason: b.Reason,
			Added:  b.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) MarkSpam(ctx context.Context, req *orlyaclv1.MarkSpamRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	if err := curatingACL.MarkEventAsSpam(req.EventId, req.Pubkey, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mark spam: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) UnmarkSpam(ctx context.Context, req *orlyaclv1.EventRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	if err := curatingACL.UnmarkEventAsSpam(req.EventId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmark spam: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) ListSpamEvents(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ListSpamEventsResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	spam, err := curatingACL.ListSpamEvents()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list spam events: %v", err)
	}
	resp := &orlyaclv1.ListSpamEventsResponse{}
	for _, se := range spam {
		resp.Events = append(resp.Events, &orlyaclv1.SpamEvent{
			EventId: se.EventID,
			Pubkey:  se.Pubkey,
			Reason:  se.Reason,
			Added:   se.Added.Unix(),
		})
	}
	return resp, nil
}

func (s *ACLService) RateLimitCheck(ctx context.Context, req *orlyaclv1.RateLimitCheckRequest) (*orlyaclv1.RateLimitCheckResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	allowed, message, err := curating.RateLimitCheck(req.Pubkey, req.Ip)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check rate limit: %v", err)
	}
	return &orlyaclv1.RateLimitCheckResponse{
		Allowed: allowed,
		Message: message,
	}, nil
}

func (s *ACLService) ProcessConfigEvent(ctx context.Context, req *orlyaclv1.ConfigEventRequest) (*orlyaclv1.Empty, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	ev := orlydbv1.ProtoToEvent(req.Event)
	if err := curating.ProcessConfigEvent(ev); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process config event: %v", err)
	}
	return &orlyaclv1.Empty{}, nil
}

func (s *ACLService) GetCuratingConfig(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.CuratingConfig, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	config, err := curating.GetConfig()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get config: %v", err)
	}
	resp := &orlyaclv1.CuratingConfig{
		ConfigEventId:  config.ConfigEventID,
		ConfigPubkey:   config.ConfigPubkey,
		ConfiguredAt:   config.ConfiguredAt,
		DailyLimit:     int32(config.DailyLimit),
		IpDailyLimit:   int32(config.IPDailyLimit),
		FirstBanHours:  int32(config.FirstBanHours),
		SecondBanHours: int32(config.SecondBanHours),
		KindCategories: config.KindCategories,
		AllowedRanges:  config.AllowedRanges,
	}
	for _, k := range config.AllowedKinds {
		resp.AllowedKinds = append(resp.AllowedKinds, int32(k))
	}
	return resp, nil
}

func (s *ACLService) IsCuratingConfigured(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.BoolResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return &orlyaclv1.BoolResponse{Value: false}, nil
	}
	configured, err := curating.IsConfigured()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if configured: %v", err)
	}
	return &orlyaclv1.BoolResponse{Value: configured}, nil
}

func (s *ACLService) ListUnclassifiedUsers(ctx context.Context, req *orlyaclv1.PaginationRequest) (*orlyaclv1.ListUnclassifiedUsersResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	// The underlying ListUnclassifiedUsers only takes limit, not offset
	// We'll request limit+offset and skip the first offset items
	limit := int(req.Limit)
	offset := int(req.Offset)
	if limit == 0 {
		limit = 100 // Default limit
	}
	users, err := curatingACL.ListUnclassifiedUsers(limit + offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list unclassified users: %v", err)
	}
	// Apply offset
	if offset > 0 && len(users) > offset {
		users = users[offset:]
	} else if offset > 0 {
		users = nil
	}
	// Apply limit
	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}
	resp := &orlyaclv1.ListUnclassifiedUsersResponse{Total: int32(len(users))}
	for _, u := range users {
		resp.Users = append(resp.Users, &orlyaclv1.UnclassifiedUser{
			Pubkey:     u.Pubkey,
			EventCount: int32(u.EventCount),
			FirstSeen:  u.LastEvent.Format("2006-01-02T15:04:05Z"),
		})
	}
	return resp, nil
}

func (s *ACLService) GetEventsForPubkey(ctx context.Context, req *orlyaclv1.GetEventsForPubkeyRequest) (*orlyaclv1.EventsForPubkeyResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	events, total, err := curatingACL.GetEventsForPubkey(req.Pubkey, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get events for pubkey: %v", err)
	}
	resp := &orlyaclv1.EventsForPubkeyResponse{Total: int32(total)}
	for _, ev := range events {
		resp.Events = append(resp.Events, &orlyaclv1.EventSummary{
			Id:        ev.ID,
			Kind:      uint32(ev.Kind),
			Content:   []byte(ev.Content),
			CreatedAt: ev.CreatedAt,
		})
	}
	return resp, nil
}

func (s *ACLService) DeleteEventsForPubkey(ctx context.Context, req *orlyaclv1.DeleteEventsForPubkeyRequest) (*orlyaclv1.DeleteCountResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	count, err := curatingACL.DeleteEventsForPubkey(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete events for pubkey: %v", err)
	}
	return &orlyaclv1.DeleteCountResponse{Count: int32(count)}, nil
}

func (s *ACLService) ScanAllPubkeys(ctx context.Context, req *orlyaclv1.Empty) (*orlyaclv1.ScanResultResponse, error) {
	curating := s.getCuratingACL()
	if curating == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL not available")
	}
	curatingACL := curating.GetCuratingACL()
	if curatingACL == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "curating ACL database not available")
	}
	result, err := curatingACL.ScanAllPubkeys()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to scan all pubkeys: %v", err)
	}
	return &orlyaclv1.ScanResultResponse{
		TotalPubkeys: int32(result.TotalPubkeys),
		TotalEvents:  int32(result.TotalEvents),
	}, nil
}

// === Helper Methods ===

func (s *ACLService) getManagedACL() *acl.Managed {
	for _, i := range acl.Registry.ACL {
		if i.Type() == "managed" {
			if managed, ok := i.(*acl.Managed); ok {
				return managed
			}
		}
	}
	return nil
}

func (s *ACLService) getCuratingACL() *acl.Curating {
	for _, i := range acl.Registry.ACL {
		if i.Type() == "curating" {
			if curating, ok := i.(*acl.Curating); ok {
				return curating
			}
		}
	}
	return nil
}

// Unused but may be needed for debugging
var _ = log.T
var _ = hex.EncodeToString
