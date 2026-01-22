package main

import (
	"context"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"next.orly.dev/pkg/database"
	orlydbv1 "next.orly.dev/pkg/proto/orlydb/v1"
)

// DatabaseService implements the orlydbv1.DatabaseServiceServer interface.
type DatabaseService struct {
	orlydbv1.UnimplementedDatabaseServiceServer
	db  database.Database
	cfg *Config
}

// NewDatabaseService creates a new database service.
func NewDatabaseService(db database.Database, cfg *Config) *DatabaseService {
	return &DatabaseService{
		db:  db,
		cfg: cfg,
	}
}

// === Lifecycle Methods ===

func (s *DatabaseService) GetPath(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.PathResponse, error) {
	return &orlydbv1.PathResponse{Path: s.db.Path()}, nil
}

func (s *DatabaseService) Sync(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.Empty, error) {
	if err := s.db.Sync(); err != nil {
		return nil, status.Errorf(codes.Internal, "sync failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) Ready(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.ReadyResponse, error) {
	// Check if ready channel is closed
	select {
	case <-s.db.Ready():
		return &orlydbv1.ReadyResponse{Ready: true}, nil
	default:
		return &orlydbv1.ReadyResponse{Ready: false}, nil
	}
}

func (s *DatabaseService) SetLogLevel(ctx context.Context, req *orlydbv1.SetLogLevelRequest) (*orlydbv1.Empty, error) {
	s.db.SetLogLevel(req.Level)
	return &orlydbv1.Empty{}, nil
}

// === Event Storage ===

func (s *DatabaseService) SaveEvent(ctx context.Context, req *orlydbv1.SaveEventRequest) (*orlydbv1.SaveEventResponse, error) {
	ev := orlydbv1.ProtoToEvent(req.Event)
	exists, err := s.db.SaveEvent(ctx, ev)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save event failed: %v", err)
	}
	return &orlydbv1.SaveEventResponse{Exists: exists}, nil
}

func (s *DatabaseService) GetSerialsFromFilter(ctx context.Context, req *orlydbv1.GetSerialsFromFilterRequest) (*orlydbv1.SerialList, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	serials, err := s.db.GetSerialsFromFilter(f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get serials failed: %v", err)
	}
	return orlydbv1.Uint40sToProto(serials), nil
}

func (s *DatabaseService) WouldReplaceEvent(ctx context.Context, req *orlydbv1.WouldReplaceEventRequest) (*orlydbv1.WouldReplaceEventResponse, error) {
	ev := orlydbv1.ProtoToEvent(req.Event)
	wouldReplace, replacedSerials, err := s.db.WouldReplaceEvent(ev)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "would replace check failed: %v", err)
	}
	resp := &orlydbv1.WouldReplaceEventResponse{
		WouldReplace: wouldReplace,
	}
	for _, ser := range replacedSerials {
		resp.ReplacedSerials = append(resp.ReplacedSerials, ser.Get())
	}
	return resp, nil
}

// === Event Queries (Streaming) ===

func (s *DatabaseService) QueryEvents(req *orlydbv1.QueryEventsRequest, stream orlydbv1.DatabaseService_QueryEventsServer) error {
	f := orlydbv1.ProtoToFilter(req.Filter)
	events, err := s.db.QueryEvents(stream.Context(), f)
	if err != nil {
		return status.Errorf(codes.Internal, "query events failed: %v", err)
	}
	return s.streamEvents(orlydbv1.EventsToProto(events), stream)
}

func (s *DatabaseService) QueryAllVersions(req *orlydbv1.QueryEventsRequest, stream orlydbv1.DatabaseService_QueryAllVersionsServer) error {
	f := orlydbv1.ProtoToFilter(req.Filter)
	events, err := s.db.QueryAllVersions(stream.Context(), f)
	if err != nil {
		return status.Errorf(codes.Internal, "query all versions failed: %v", err)
	}
	return s.streamEvents(orlydbv1.EventsToProto(events), stream)
}

func (s *DatabaseService) QueryEventsWithOptions(req *orlydbv1.QueryEventsWithOptionsRequest, stream orlydbv1.DatabaseService_QueryEventsWithOptionsServer) error {
	f := orlydbv1.ProtoToFilter(req.Filter)
	events, err := s.db.QueryEventsWithOptions(stream.Context(), f, req.IncludeDeleteEvents, req.ShowAllVersions)
	if err != nil {
		return status.Errorf(codes.Internal, "query events with options failed: %v", err)
	}
	return s.streamEvents(orlydbv1.EventsToProto(events), stream)
}

func (s *DatabaseService) QueryDeleteEventsByTargetId(req *orlydbv1.QueryDeleteEventsByTargetIdRequest, stream orlydbv1.DatabaseService_QueryDeleteEventsByTargetIdServer) error {
	events, err := s.db.QueryDeleteEventsByTargetId(stream.Context(), req.TargetEventId)
	if err != nil {
		return status.Errorf(codes.Internal, "query delete events failed: %v", err)
	}
	return s.streamEvents(orlydbv1.EventsToProto(events), stream)
}

func (s *DatabaseService) QueryForSerials(ctx context.Context, req *orlydbv1.QueryEventsRequest) (*orlydbv1.SerialList, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	serials, err := s.db.QueryForSerials(ctx, f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query for serials failed: %v", err)
	}
	return orlydbv1.Uint40sToProto(serials), nil
}

func (s *DatabaseService) QueryForIds(ctx context.Context, req *orlydbv1.QueryEventsRequest) (*orlydbv1.IdPkTsList, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	idPkTs, err := s.db.QueryForIds(ctx, f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query for ids failed: %v", err)
	}
	return orlydbv1.IdPkTsListToProto(idPkTs), nil
}

func (s *DatabaseService) CountEvents(ctx context.Context, req *orlydbv1.QueryEventsRequest) (*orlydbv1.CountEventsResponse, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	count, approximate, err := s.db.CountEvents(ctx, f)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "count events failed: %v", err)
	}
	return &orlydbv1.CountEventsResponse{
		Count:       int32(count),
		Approximate: approximate,
	}, nil
}

// === Event Retrieval by Serial ===

func (s *DatabaseService) FetchEventBySerial(ctx context.Context, req *orlydbv1.FetchEventBySerialRequest) (*orlydbv1.FetchEventBySerialResponse, error) {
	ser := orlydbv1.ProtoToUint40(&orlydbv1.Uint40{Value: req.Serial})
	ev, err := s.db.FetchEventBySerial(ser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetch event by serial failed: %v", err)
	}
	return &orlydbv1.FetchEventBySerialResponse{
		Event: orlydbv1.EventToProto(ev),
		Found: ev != nil,
	}, nil
}

func (s *DatabaseService) FetchEventsBySerials(ctx context.Context, req *orlydbv1.FetchEventsBySerialRequest) (*orlydbv1.EventMap, error) {
	serials := orlydbv1.ProtoToUint40s(&orlydbv1.SerialList{Serials: req.Serials})
	events, err := s.db.FetchEventsBySerials(serials)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetch events by serials failed: %v", err)
	}
	return orlydbv1.EventMapToProto(events), nil
}

func (s *DatabaseService) GetSerialById(ctx context.Context, req *orlydbv1.GetSerialByIdRequest) (*orlydbv1.GetSerialByIdResponse, error) {
	ser, err := s.db.GetSerialById(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get serial by id failed: %v", err)
	}
	if ser == nil {
		return &orlydbv1.GetSerialByIdResponse{Found: false}, nil
	}
	return &orlydbv1.GetSerialByIdResponse{
		Serial: ser.Get(),
		Found:  true,
	}, nil
}

func (s *DatabaseService) GetSerialsByIds(ctx context.Context, req *orlydbv1.GetSerialsByIdsRequest) (*orlydbv1.SerialMap, error) {
	// Convert request IDs to tag format
	ids := orlydbv1.BytesToTag(req.Ids)
	serials, err := s.db.GetSerialsByIds(ids)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get serials by ids failed: %v", err)
	}
	result := &orlydbv1.SerialMap{
		Serials: make(map[string]uint64),
	}
	for k, v := range serials {
		if v != nil {
			result.Serials[k] = v.Get()
		}
	}
	return result, nil
}

func (s *DatabaseService) GetSerialsByRange(ctx context.Context, req *orlydbv1.GetSerialsByRangeRequest) (*orlydbv1.SerialList, error) {
	r := orlydbv1.ProtoToRange(req.Range)
	serials, err := s.db.GetSerialsByRange(r)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get serials by range failed: %v", err)
	}
	return orlydbv1.Uint40sToProto(serials), nil
}

func (s *DatabaseService) GetFullIdPubkeyBySerial(ctx context.Context, req *orlydbv1.GetFullIdPubkeyBySerialRequest) (*orlydbv1.IdPkTs, error) {
	ser := orlydbv1.ProtoToUint40(&orlydbv1.Uint40{Value: req.Serial})
	idPkTs, err := s.db.GetFullIdPubkeyBySerial(ser)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get full id pubkey by serial failed: %v", err)
	}
	return orlydbv1.IdPkTsToProto(idPkTs), nil
}

func (s *DatabaseService) GetFullIdPubkeyBySerials(ctx context.Context, req *orlydbv1.GetFullIdPubkeyBySerialsRequest) (*orlydbv1.IdPkTsList, error) {
	serials := orlydbv1.ProtoToUint40s(&orlydbv1.SerialList{Serials: req.Serials})
	idPkTs, err := s.db.GetFullIdPubkeyBySerials(serials)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get full id pubkey by serials failed: %v", err)
	}
	return orlydbv1.IdPkTsListToProto(idPkTs), nil
}

// === Event Deletion ===

func (s *DatabaseService) DeleteEvent(ctx context.Context, req *orlydbv1.DeleteEventRequest) (*orlydbv1.Empty, error) {
	if err := s.db.DeleteEvent(ctx, req.EventId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete event failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) DeleteEventBySerial(ctx context.Context, req *orlydbv1.DeleteEventBySerialRequest) (*orlydbv1.Empty, error) {
	ser := orlydbv1.ProtoToUint40(&orlydbv1.Uint40{Value: req.Serial})
	ev := orlydbv1.ProtoToEvent(req.Event)
	if err := s.db.DeleteEventBySerial(ctx, ser, ev); err != nil {
		return nil, status.Errorf(codes.Internal, "delete event by serial failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) DeleteExpired(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.Empty, error) {
	s.db.DeleteExpired()
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) ProcessDelete(ctx context.Context, req *orlydbv1.ProcessDeleteRequest) (*orlydbv1.Empty, error) {
	ev := orlydbv1.ProtoToEvent(req.Event)
	if err := s.db.ProcessDelete(ev, req.Admins); err != nil {
		return nil, status.Errorf(codes.Internal, "process delete failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) CheckForDeleted(ctx context.Context, req *orlydbv1.CheckForDeletedRequest) (*orlydbv1.Empty, error) {
	ev := orlydbv1.ProtoToEvent(req.Event)
	if err := s.db.CheckForDeleted(ev, req.Admins); err != nil {
		return nil, status.Errorf(codes.Internal, "check for deleted failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

// === Import/Export ===

func (s *DatabaseService) Import(stream orlydbv1.DatabaseService_ImportServer) error {
	pr, pw := io.Pipe()

	// Goroutine to read from gRPC stream and write to pipe
	go func() {
		defer pw.Close()
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.E.F("import stream error: %v", err)
				pw.CloseWithError(err)
				return
			}
			if _, err := pw.Write(chunk.Data); chk.E(err) {
				return
			}
		}
	}()

	// Import from pipe
	s.db.Import(pr)

	return stream.SendAndClose(&orlydbv1.ImportResponse{
		EventsImported: 0, // TODO: Track count
		EventsSkipped:  0,
	})
}

func (s *DatabaseService) Export(req *orlydbv1.ExportRequest, stream orlydbv1.DatabaseService_ExportServer) error {
	pr, pw := io.Pipe()

	// Goroutine to export to pipe
	go func() {
		defer pw.Close()
		s.db.Export(stream.Context(), pw, req.Pubkeys...)
	}()

	// Read from pipe and send to stream
	buf := make([]byte, 64*1024) // 64KB chunks
	for {
		n, err := pr.Read(buf)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "export failed: %v", err)
		}
		if err := stream.Send(&orlydbv1.ExportChunk{Data: buf[:n]}); err != nil {
			return err
		}
	}
}

func (s *DatabaseService) ImportEventsFromStrings(ctx context.Context, req *orlydbv1.ImportEventsFromStringsRequest) (*orlydbv1.ImportResponse, error) {
	// Note: We can't pass policy manager over gRPC, so we pass nil
	if err := s.db.ImportEventsFromStrings(ctx, req.EventJsons, nil); err != nil {
		return nil, status.Errorf(codes.Internal, "import events from strings failed: %v", err)
	}
	return &orlydbv1.ImportResponse{
		EventsImported: int64(len(req.EventJsons)),
	}, nil
}

// === Relay Identity ===

func (s *DatabaseService) GetRelayIdentitySecret(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.GetRelayIdentitySecretResponse, error) {
	secret, err := s.db.GetRelayIdentitySecret()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get relay identity secret failed: %v", err)
	}
	return &orlydbv1.GetRelayIdentitySecretResponse{SecretKey: secret}, nil
}

func (s *DatabaseService) SetRelayIdentitySecret(ctx context.Context, req *orlydbv1.SetRelayIdentitySecretRequest) (*orlydbv1.Empty, error) {
	if err := s.db.SetRelayIdentitySecret(req.SecretKey); err != nil {
		return nil, status.Errorf(codes.Internal, "set relay identity secret failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetOrCreateRelayIdentitySecret(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.GetRelayIdentitySecretResponse, error) {
	secret, err := s.db.GetOrCreateRelayIdentitySecret()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get or create relay identity secret failed: %v", err)
	}
	return &orlydbv1.GetRelayIdentitySecretResponse{SecretKey: secret}, nil
}

// === Markers ===

func (s *DatabaseService) SetMarker(ctx context.Context, req *orlydbv1.SetMarkerRequest) (*orlydbv1.Empty, error) {
	if err := s.db.SetMarker(req.Key, req.Value); err != nil {
		return nil, status.Errorf(codes.Internal, "set marker failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetMarker(ctx context.Context, req *orlydbv1.GetMarkerRequest) (*orlydbv1.GetMarkerResponse, error) {
	value, err := s.db.GetMarker(req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get marker failed: %v", err)
	}
	return &orlydbv1.GetMarkerResponse{
		Value: value,
		Found: value != nil,
	}, nil
}

func (s *DatabaseService) HasMarker(ctx context.Context, req *orlydbv1.HasMarkerRequest) (*orlydbv1.HasMarkerResponse, error) {
	exists := s.db.HasMarker(req.Key)
	return &orlydbv1.HasMarkerResponse{Exists: exists}, nil
}

func (s *DatabaseService) DeleteMarker(ctx context.Context, req *orlydbv1.DeleteMarkerRequest) (*orlydbv1.Empty, error) {
	if err := s.db.DeleteMarker(req.Key); err != nil {
		return nil, status.Errorf(codes.Internal, "delete marker failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

// === Subscriptions ===

func (s *DatabaseService) GetSubscription(ctx context.Context, req *orlydbv1.GetSubscriptionRequest) (*orlydbv1.Subscription, error) {
	sub, err := s.db.GetSubscription(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get subscription failed: %v", err)
	}
	return orlydbv1.SubscriptionToProto(sub, req.Pubkey), nil
}

func (s *DatabaseService) IsSubscriptionActive(ctx context.Context, req *orlydbv1.IsSubscriptionActiveRequest) (*orlydbv1.IsSubscriptionActiveResponse, error) {
	active, err := s.db.IsSubscriptionActive(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "is subscription active failed: %v", err)
	}
	return &orlydbv1.IsSubscriptionActiveResponse{Active: active}, nil
}

func (s *DatabaseService) ExtendSubscription(ctx context.Context, req *orlydbv1.ExtendSubscriptionRequest) (*orlydbv1.Empty, error) {
	if err := s.db.ExtendSubscription(req.Pubkey, int(req.Days)); err != nil {
		return nil, status.Errorf(codes.Internal, "extend subscription failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) RecordPayment(ctx context.Context, req *orlydbv1.RecordPaymentRequest) (*orlydbv1.Empty, error) {
	if err := s.db.RecordPayment(req.Pubkey, req.Amount, req.Invoice, req.Preimage); err != nil {
		return nil, status.Errorf(codes.Internal, "record payment failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetPaymentHistory(ctx context.Context, req *orlydbv1.GetPaymentHistoryRequest) (*orlydbv1.PaymentList, error) {
	payments, err := s.db.GetPaymentHistory(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get payment history failed: %v", err)
	}
	return orlydbv1.PaymentListToProto(payments), nil
}

func (s *DatabaseService) ExtendBlossomSubscription(ctx context.Context, req *orlydbv1.ExtendBlossomSubscriptionRequest) (*orlydbv1.Empty, error) {
	if err := s.db.ExtendBlossomSubscription(req.Pubkey, req.Tier, req.StorageMb, int(req.DaysExtended)); err != nil {
		return nil, status.Errorf(codes.Internal, "extend blossom subscription failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetBlossomStorageQuota(ctx context.Context, req *orlydbv1.GetBlossomStorageQuotaRequest) (*orlydbv1.GetBlossomStorageQuotaResponse, error) {
	quota, err := s.db.GetBlossomStorageQuota(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get blossom storage quota failed: %v", err)
	}
	return &orlydbv1.GetBlossomStorageQuotaResponse{QuotaMb: quota}, nil
}

func (s *DatabaseService) IsFirstTimeUser(ctx context.Context, req *orlydbv1.IsFirstTimeUserRequest) (*orlydbv1.IsFirstTimeUserResponse, error) {
	firstTime, err := s.db.IsFirstTimeUser(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "is first time user failed: %v", err)
	}
	return &orlydbv1.IsFirstTimeUserResponse{FirstTime: firstTime}, nil
}

// === NIP-43 ===

func (s *DatabaseService) AddNIP43Member(ctx context.Context, req *orlydbv1.AddNIP43MemberRequest) (*orlydbv1.Empty, error) {
	if err := s.db.AddNIP43Member(req.Pubkey, req.InviteCode); err != nil {
		return nil, status.Errorf(codes.Internal, "add NIP-43 member failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) RemoveNIP43Member(ctx context.Context, req *orlydbv1.RemoveNIP43MemberRequest) (*orlydbv1.Empty, error) {
	if err := s.db.RemoveNIP43Member(req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "remove NIP-43 member failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) IsNIP43Member(ctx context.Context, req *orlydbv1.IsNIP43MemberRequest) (*orlydbv1.IsNIP43MemberResponse, error) {
	isMember, err := s.db.IsNIP43Member(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "is NIP-43 member failed: %v", err)
	}
	return &orlydbv1.IsNIP43MemberResponse{IsMember: isMember}, nil
}

func (s *DatabaseService) GetNIP43Membership(ctx context.Context, req *orlydbv1.GetNIP43MembershipRequest) (*orlydbv1.NIP43Membership, error) {
	membership, err := s.db.GetNIP43Membership(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get NIP-43 membership failed: %v", err)
	}
	return orlydbv1.NIP43MembershipToProto(membership), nil
}

func (s *DatabaseService) GetAllNIP43Members(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.PubkeyList, error) {
	members, err := s.db.GetAllNIP43Members()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get all NIP-43 members failed: %v", err)
	}
	return &orlydbv1.PubkeyList{Pubkeys: members}, nil
}

func (s *DatabaseService) StoreInviteCode(ctx context.Context, req *orlydbv1.StoreInviteCodeRequest) (*orlydbv1.Empty, error) {
	expiresAt := orlydbv1.TimeFromUnix(req.ExpiresAt)
	if err := s.db.StoreInviteCode(req.Code, expiresAt); err != nil {
		return nil, status.Errorf(codes.Internal, "store invite code failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) ValidateInviteCode(ctx context.Context, req *orlydbv1.ValidateInviteCodeRequest) (*orlydbv1.ValidateInviteCodeResponse, error) {
	valid, err := s.db.ValidateInviteCode(req.Code)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "validate invite code failed: %v", err)
	}
	return &orlydbv1.ValidateInviteCodeResponse{Valid: valid}, nil
}

func (s *DatabaseService) DeleteInviteCode(ctx context.Context, req *orlydbv1.DeleteInviteCodeRequest) (*orlydbv1.Empty, error) {
	if err := s.db.DeleteInviteCode(req.Code); err != nil {
		return nil, status.Errorf(codes.Internal, "delete invite code failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) PublishNIP43MembershipEvent(ctx context.Context, req *orlydbv1.PublishNIP43MembershipEventRequest) (*orlydbv1.Empty, error) {
	if err := s.db.PublishNIP43MembershipEvent(int(req.Kind), req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "publish NIP-43 membership event failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

// === Query Cache ===

func (s *DatabaseService) GetCachedJSON(ctx context.Context, req *orlydbv1.GetCachedJSONRequest) (*orlydbv1.GetCachedJSONResponse, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	jsonItems, found := s.db.GetCachedJSON(f)
	return &orlydbv1.GetCachedJSONResponse{
		JsonItems: jsonItems,
		Found:     found,
	}, nil
}

func (s *DatabaseService) CacheMarshaledJSON(ctx context.Context, req *orlydbv1.CacheMarshaledJSONRequest) (*orlydbv1.Empty, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	s.db.CacheMarshaledJSON(f, req.JsonItems)
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetCachedEvents(ctx context.Context, req *orlydbv1.GetCachedEventsRequest) (*orlydbv1.GetCachedEventsResponse, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	events, found := s.db.GetCachedEvents(f)
	return &orlydbv1.GetCachedEventsResponse{
		Events: orlydbv1.EventsToProto(events),
		Found:  found,
	}, nil
}

func (s *DatabaseService) CacheEvents(ctx context.Context, req *orlydbv1.CacheEventsRequest) (*orlydbv1.Empty, error) {
	f := orlydbv1.ProtoToFilter(req.Filter)
	events := orlydbv1.ProtoToEvents(req.Events)
	s.db.CacheEvents(f, events)
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) InvalidateQueryCache(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.Empty, error) {
	s.db.InvalidateQueryCache()
	return &orlydbv1.Empty{}, nil
}

// === Access Tracking ===

func (s *DatabaseService) RecordEventAccess(ctx context.Context, req *orlydbv1.RecordEventAccessRequest) (*orlydbv1.Empty, error) {
	if err := s.db.RecordEventAccess(req.Serial, req.ConnectionId); err != nil {
		return nil, status.Errorf(codes.Internal, "record event access failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetEventAccessInfo(ctx context.Context, req *orlydbv1.GetEventAccessInfoRequest) (*orlydbv1.GetEventAccessInfoResponse, error) {
	lastAccess, accessCount, err := s.db.GetEventAccessInfo(req.Serial)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get event access info failed: %v", err)
	}
	return &orlydbv1.GetEventAccessInfoResponse{
		LastAccess:  lastAccess,
		AccessCount: accessCount,
	}, nil
}

func (s *DatabaseService) GetLeastAccessedEvents(ctx context.Context, req *orlydbv1.GetLeastAccessedEventsRequest) (*orlydbv1.SerialList, error) {
	serials, err := s.db.GetLeastAccessedEvents(int(req.Limit), req.MinAgeSec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get least accessed events failed: %v", err)
	}
	return &orlydbv1.SerialList{Serials: serials}, nil
}

// === Utility ===

func (s *DatabaseService) EventIdsBySerial(ctx context.Context, req *orlydbv1.EventIdsBySerialRequest) (*orlydbv1.EventIdsBySerialResponse, error) {
	eventIds, err := s.db.EventIdsBySerial(req.Start, int(req.Count))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "event ids by serial failed: %v", err)
	}
	return &orlydbv1.EventIdsBySerialResponse{EventIds: eventIds}, nil
}

func (s *DatabaseService) RunMigrations(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.Empty, error) {
	s.db.RunMigrations()
	return &orlydbv1.Empty{}, nil
}

// === Blob Storage (Blossom) ===

func (s *DatabaseService) SaveBlob(ctx context.Context, req *orlydbv1.SaveBlobRequest) (*orlydbv1.Empty, error) {
	if err := s.db.SaveBlob(req.Sha256Hash, req.Data, req.Pubkey, req.MimeType, req.Extension); err != nil {
		return nil, status.Errorf(codes.Internal, "save blob failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) GetBlob(ctx context.Context, req *orlydbv1.GetBlobRequest) (*orlydbv1.GetBlobResponse, error) {
	data, metadata, err := s.db.GetBlob(req.Sha256Hash)
	if err != nil {
		// Return not found as a response, not an error
		return &orlydbv1.GetBlobResponse{Found: false}, nil
	}
	return &orlydbv1.GetBlobResponse{
		Found:    true,
		Data:     data,
		Metadata: orlydbv1.BlobMetadataToProto(metadata),
	}, nil
}

func (s *DatabaseService) HasBlob(ctx context.Context, req *orlydbv1.HasBlobRequest) (*orlydbv1.HasBlobResponse, error) {
	exists, err := s.db.HasBlob(req.Sha256Hash)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "has blob failed: %v", err)
	}
	return &orlydbv1.HasBlobResponse{Exists: exists}, nil
}

func (s *DatabaseService) DeleteBlob(ctx context.Context, req *orlydbv1.DeleteBlobRequest) (*orlydbv1.Empty, error) {
	if err := s.db.DeleteBlob(req.Sha256Hash, req.Pubkey); err != nil {
		return nil, status.Errorf(codes.Internal, "delete blob failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) ListBlobs(ctx context.Context, req *orlydbv1.ListBlobsRequest) (*orlydbv1.ListBlobsResponse, error) {
	descriptors, err := s.db.ListBlobs(req.Pubkey, req.Since, req.Until)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list blobs failed: %v", err)
	}
	return &orlydbv1.ListBlobsResponse{
		Descriptors: orlydbv1.BlobDescriptorListToProto(descriptors),
	}, nil
}

func (s *DatabaseService) GetBlobMetadata(ctx context.Context, req *orlydbv1.GetBlobMetadataRequest) (*orlydbv1.BlobMetadata, error) {
	metadata, err := s.db.GetBlobMetadata(req.Sha256Hash)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "blob metadata not found: %v", err)
	}
	return orlydbv1.BlobMetadataToProto(metadata), nil
}

func (s *DatabaseService) GetTotalBlobStorageUsed(ctx context.Context, req *orlydbv1.GetTotalBlobStorageUsedRequest) (*orlydbv1.GetTotalBlobStorageUsedResponse, error) {
	totalMB, err := s.db.GetTotalBlobStorageUsed(req.Pubkey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get total blob storage used failed: %v", err)
	}
	return &orlydbv1.GetTotalBlobStorageUsedResponse{TotalMb: totalMB}, nil
}

func (s *DatabaseService) SaveBlobReport(ctx context.Context, req *orlydbv1.SaveBlobReportRequest) (*orlydbv1.Empty, error) {
	if err := s.db.SaveBlobReport(req.Sha256Hash, req.ReportData); err != nil {
		return nil, status.Errorf(codes.Internal, "save blob report failed: %v", err)
	}
	return &orlydbv1.Empty{}, nil
}

func (s *DatabaseService) ListAllBlobUserStats(ctx context.Context, req *orlydbv1.Empty) (*orlydbv1.ListAllBlobUserStatsResponse, error) {
	stats, err := s.db.ListAllBlobUserStats()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list all blob user stats failed: %v", err)
	}
	return &orlydbv1.ListAllBlobUserStatsResponse{
		Stats: orlydbv1.UserBlobStatsListToProto(stats),
	}, nil
}

// === Helper Methods ===

// streamEvents is a helper to stream events in batches.
type eventStreamer interface {
	Send(*orlydbv1.EventBatch) error
	Context() context.Context
}

func (s *DatabaseService) streamEvents(events []*orlydbv1.Event, stream eventStreamer) error {
	batchSize := s.cfg.StreamBatchSize
	if batchSize == 0 {
		batchSize = 100
	}

	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := &orlydbv1.EventBatch{
			Events: events[i:end],
		}

		if err := stream.Send(batch); err != nil {
			return err
		}
	}

	return nil
}
