// Package grpc provides a gRPC client that implements the database.Database interface.
// This allows the relay to use a remote database server via gRPC.
package grpc

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/database"
	indextypes "next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
	orlydbv1 "next.orly.dev/pkg/proto/orlydb/v1"
)

// Client implements the database.Database interface via gRPC.
type Client struct {
	conn   *grpc.ClientConn
	client orlydbv1.DatabaseServiceClient
	ready  chan struct{}
	path   string
}

// Verify Client implements database.Database at compile time.
var _ database.Database = (*Client)(nil)

// ClientConfig holds configuration for the gRPC client.
type ClientConfig struct {
	ServerAddress  string
	ConnectTimeout time.Duration
}

// New creates a new gRPC database client.
func New(ctx context.Context, cfg *ClientConfig) (*Client, error) {
	timeout := cfg.ConnectTimeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, cfg.ServerAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64<<20), // 64MB
			grpc.MaxCallSendMsgSize(64<<20), // 64MB
		),
	)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn:   conn,
		client: orlydbv1.NewDatabaseServiceClient(conn),
		ready:  make(chan struct{}),
		path:   "grpc://" + cfg.ServerAddress,
	}

	// Check if server is ready
	go c.waitForReady(ctx)

	return c, nil
}

func (c *Client) waitForReady(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			resp, err := c.client.Ready(ctx, &orlydbv1.Empty{})
			if err == nil && resp.Ready {
				close(c.ready)
				log.I.F("gRPC database client connected and ready")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// === Lifecycle Methods ===

func (c *Client) Path() string {
	// Get the actual database path from the server so blossom can find blob files
	resp, err := c.client.GetPath(context.Background(), &orlydbv1.Empty{})
	if err != nil {
		log.W.F("failed to get path from database server: %v, using local path", err)
		return c.path
	}
	return resp.Path
}

func (c *Client) Init(path string) error {
	// Not applicable for remote database
	return nil
}

func (c *Client) Sync() error {
	_, err := c.client.Sync(context.Background(), &orlydbv1.Empty{})
	return err
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Wipe() error {
	// Not implemented for remote database (dangerous operation)
	return nil
}

func (c *Client) SetLogLevel(level string) {
	_, _ = c.client.SetLogLevel(context.Background(), &orlydbv1.SetLogLevelRequest{Level: level})
}

func (c *Client) Ready() <-chan struct{} {
	return c.ready
}

// === Event Storage ===

func (c *Client) SaveEvent(ctx context.Context, ev *event.E) (exists bool, err error) {
	resp, err := c.client.SaveEvent(ctx, &orlydbv1.SaveEventRequest{
		Event: orlydbv1.EventToProto(ev),
	})
	if err != nil {
		return false, err
	}
	return resp.Exists, nil
}

func (c *Client) GetSerialsFromFilter(f *filter.F) (serials indextypes.Uint40s, err error) {
	resp, err := c.client.GetSerialsFromFilter(context.Background(), &orlydbv1.GetSerialsFromFilterRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, err
	}
	return protoToUint40s(resp), nil
}

func (c *Client) WouldReplaceEvent(ev *event.E) (bool, indextypes.Uint40s, error) {
	resp, err := c.client.WouldReplaceEvent(context.Background(), &orlydbv1.WouldReplaceEventRequest{
		Event: orlydbv1.EventToProto(ev),
	})
	if err != nil {
		return false, nil, err
	}
	serials := make(indextypes.Uint40s, 0, len(resp.ReplacedSerials))
	for _, s := range resp.ReplacedSerials {
		u := &indextypes.Uint40{}
		_ = u.Set(s)
		serials = append(serials, u)
	}
	return resp.WouldReplace, serials, nil
}

// === Event Queries ===

func (c *Client) QueryEvents(ctx context.Context, f *filter.F) (evs event.S, err error) {
	stream, err := c.client.QueryEvents(ctx, &orlydbv1.QueryEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, err
	}
	return c.collectStreamedEvents(stream)
}

func (c *Client) QueryAllVersions(ctx context.Context, f *filter.F) (evs event.S, err error) {
	stream, err := c.client.QueryAllVersions(ctx, &orlydbv1.QueryEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, err
	}
	return c.collectStreamedEvents(stream)
}

func (c *Client) QueryEventsWithOptions(ctx context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool) (evs event.S, err error) {
	stream, err := c.client.QueryEventsWithOptions(ctx, &orlydbv1.QueryEventsWithOptionsRequest{
		Filter:              orlydbv1.FilterToProto(f),
		IncludeDeleteEvents: includeDeleteEvents,
		ShowAllVersions:     showAllVersions,
	})
	if err != nil {
		return nil, err
	}
	return c.collectStreamedEvents(stream)
}

func (c *Client) QueryDeleteEventsByTargetId(ctx context.Context, targetEventId []byte) (evs event.S, err error) {
	stream, err := c.client.QueryDeleteEventsByTargetId(ctx, &orlydbv1.QueryDeleteEventsByTargetIdRequest{
		TargetEventId: targetEventId,
	})
	if err != nil {
		return nil, err
	}
	return c.collectStreamedEvents(stream)
}

func (c *Client) QueryForSerials(ctx context.Context, f *filter.F) (serials indextypes.Uint40s, err error) {
	resp, err := c.client.QueryForSerials(ctx, &orlydbv1.QueryEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, err
	}
	return protoToUint40s(resp), nil
}

func (c *Client) QueryForIds(ctx context.Context, f *filter.F) (idPkTs []*store.IdPkTs, err error) {
	resp, err := c.client.QueryForIds(ctx, &orlydbv1.QueryEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToIdPkTsList(resp), nil
}

func (c *Client) CountEvents(ctx context.Context, f *filter.F) (count int, approximate bool, err error) {
	resp, err := c.client.CountEvents(ctx, &orlydbv1.QueryEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return 0, false, err
	}
	return int(resp.Count), resp.Approximate, nil
}

// === Event Retrieval by Serial ===

func (c *Client) FetchEventBySerial(ser *indextypes.Uint40) (ev *event.E, err error) {
	resp, err := c.client.FetchEventBySerial(context.Background(), &orlydbv1.FetchEventBySerialRequest{
		Serial: ser.Get(),
	})
	if err != nil {
		return nil, err
	}
	if !resp.Found {
		return nil, nil
	}
	return orlydbv1.ProtoToEvent(resp.Event), nil
}

func (c *Client) FetchEventsBySerials(serials []*indextypes.Uint40) (events map[uint64]*event.E, err error) {
	serialList := make([]uint64, 0, len(serials))
	for _, s := range serials {
		serialList = append(serialList, s.Get())
	}
	resp, err := c.client.FetchEventsBySerials(context.Background(), &orlydbv1.FetchEventsBySerialRequest{
		Serials: serialList,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToEventMap(resp), nil
}

func (c *Client) GetSerialById(id []byte) (ser *indextypes.Uint40, err error) {
	resp, err := c.client.GetSerialById(context.Background(), &orlydbv1.GetSerialByIdRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Found {
		return nil, nil
	}
	u := &indextypes.Uint40{}
	_ = u.Set(resp.Serial)
	return u, nil
}

func (c *Client) GetSerialsByIds(ids *tag.T) (serials map[string]*indextypes.Uint40, err error) {
	idList := make([][]byte, 0, len(ids.T))
	for _, id := range ids.T {
		idList = append(idList, id)
	}
	resp, err := c.client.GetSerialsByIds(context.Background(), &orlydbv1.GetSerialsByIdsRequest{
		Ids: idList,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[string]*indextypes.Uint40)
	for k, v := range resp.Serials {
		u := &indextypes.Uint40{}
		_ = u.Set(v)
		result[k] = u
	}
	return result, nil
}

func (c *Client) GetSerialsByIdsWithFilter(ids *tag.T, fn func(ev *event.E, ser *indextypes.Uint40) bool) (serials map[string]*indextypes.Uint40, err error) {
	// Note: Filter function cannot be passed over gRPC, so we just get all serials
	return c.GetSerialsByIds(ids)
}

func (c *Client) GetSerialsByRange(idx database.Range) (serials indextypes.Uint40s, err error) {
	resp, err := c.client.GetSerialsByRange(context.Background(), &orlydbv1.GetSerialsByRangeRequest{
		Range: orlydbv1.RangeToProto(idx),
	})
	if err != nil {
		return nil, err
	}
	return protoToUint40s(resp), nil
}

func (c *Client) GetFullIdPubkeyBySerial(ser *indextypes.Uint40) (fidpk *store.IdPkTs, err error) {
	resp, err := c.client.GetFullIdPubkeyBySerial(context.Background(), &orlydbv1.GetFullIdPubkeyBySerialRequest{
		Serial: ser.Get(),
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToIdPkTs(resp), nil
}

func (c *Client) GetFullIdPubkeyBySerials(sers []*indextypes.Uint40) (fidpks []*store.IdPkTs, err error) {
	serialList := make([]uint64, 0, len(sers))
	for _, s := range sers {
		serialList = append(serialList, s.Get())
	}
	resp, err := c.client.GetFullIdPubkeyBySerials(context.Background(), &orlydbv1.GetFullIdPubkeyBySerialsRequest{
		Serials: serialList,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToIdPkTsList(resp), nil
}

// === Event Deletion ===

func (c *Client) DeleteEvent(ctx context.Context, eid []byte) error {
	_, err := c.client.DeleteEvent(ctx, &orlydbv1.DeleteEventRequest{
		EventId: eid,
	})
	return err
}

func (c *Client) DeleteEventBySerial(ctx context.Context, ser *indextypes.Uint40, ev *event.E) error {
	_, err := c.client.DeleteEventBySerial(ctx, &orlydbv1.DeleteEventBySerialRequest{
		Serial: ser.Get(),
		Event:  orlydbv1.EventToProto(ev),
	})
	return err
}

func (c *Client) DeleteExpired() {
	_, _ = c.client.DeleteExpired(context.Background(), &orlydbv1.Empty{})
}

func (c *Client) ProcessDelete(ev *event.E, admins [][]byte) error {
	_, err := c.client.ProcessDelete(context.Background(), &orlydbv1.ProcessDeleteRequest{
		Event:  orlydbv1.EventToProto(ev),
		Admins: admins,
	})
	return err
}

func (c *Client) CheckForDeleted(ev *event.E, admins [][]byte) error {
	_, err := c.client.CheckForDeleted(context.Background(), &orlydbv1.CheckForDeletedRequest{
		Event:  orlydbv1.EventToProto(ev),
		Admins: admins,
	})
	return err
}

// === Import/Export ===

func (c *Client) Import(rr io.Reader) {
	stream, err := c.client.Import(context.Background())
	if chk.E(err) {
		return
	}

	buf := make([]byte, 64*1024)
	for {
		n, err := rr.Read(buf)
		if err == io.EOF {
			break
		}
		if chk.E(err) {
			return
		}
		if err := stream.Send(&orlydbv1.ImportChunk{Data: buf[:n]}); chk.E(err) {
			return
		}
	}

	_, _ = stream.CloseAndRecv()
}

func (c *Client) Export(ctx context.Context, w io.Writer, pubkeys ...[]byte) {
	stream, err := c.client.Export(ctx, &orlydbv1.ExportRequest{
		Pubkeys: pubkeys,
	})
	if chk.E(err) {
		return
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if chk.E(err) {
			return
		}
		if _, err := w.Write(chunk.Data); chk.E(err) {
			return
		}
	}
}

func (c *Client) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	c.Import(rr)
	return nil
}

func (c *Client) ImportEventsFromStrings(ctx context.Context, eventJSONs []string, policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) }) error {
	_, err := c.client.ImportEventsFromStrings(ctx, &orlydbv1.ImportEventsFromStringsRequest{
		EventJsons:  eventJSONs,
		CheckPolicy: policyManager != nil,
	})
	return err
}

// === Relay Identity ===

func (c *Client) GetRelayIdentitySecret() (skb []byte, err error) {
	resp, err := c.client.GetRelayIdentitySecret(context.Background(), &orlydbv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.SecretKey, nil
}

func (c *Client) SetRelayIdentitySecret(skb []byte) error {
	_, err := c.client.SetRelayIdentitySecret(context.Background(), &orlydbv1.SetRelayIdentitySecretRequest{
		SecretKey: skb,
	})
	return err
}

func (c *Client) GetOrCreateRelayIdentitySecret() (skb []byte, err error) {
	resp, err := c.client.GetOrCreateRelayIdentitySecret(context.Background(), &orlydbv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.SecretKey, nil
}

// === Markers ===

func (c *Client) SetMarker(key string, value []byte) error {
	_, err := c.client.SetMarker(context.Background(), &orlydbv1.SetMarkerRequest{
		Key:   key,
		Value: value,
	})
	return err
}

func (c *Client) GetMarker(key string) (value []byte, err error) {
	resp, err := c.client.GetMarker(context.Background(), &orlydbv1.GetMarkerRequest{
		Key: key,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Found {
		return nil, nil
	}
	return resp.Value, nil
}

func (c *Client) HasMarker(key string) bool {
	resp, err := c.client.HasMarker(context.Background(), &orlydbv1.HasMarkerRequest{
		Key: key,
	})
	if err != nil {
		return false
	}
	return resp.Exists
}

func (c *Client) DeleteMarker(key string) error {
	_, err := c.client.DeleteMarker(context.Background(), &orlydbv1.DeleteMarkerRequest{
		Key: key,
	})
	return err
}

// === Subscriptions ===

func (c *Client) GetSubscription(pubkey []byte) (*database.Subscription, error) {
	resp, err := c.client.GetSubscription(context.Background(), &orlydbv1.GetSubscriptionRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToSubscription(resp), nil
}

func (c *Client) IsSubscriptionActive(pubkey []byte) (bool, error) {
	resp, err := c.client.IsSubscriptionActive(context.Background(), &orlydbv1.IsSubscriptionActiveRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return false, err
	}
	return resp.Active, nil
}

func (c *Client) ExtendSubscription(pubkey []byte, days int) error {
	_, err := c.client.ExtendSubscription(context.Background(), &orlydbv1.ExtendSubscriptionRequest{
		Pubkey: pubkey,
		Days:   int32(days),
	})
	return err
}

func (c *Client) RecordPayment(pubkey []byte, amount int64, invoice, preimage string) error {
	_, err := c.client.RecordPayment(context.Background(), &orlydbv1.RecordPaymentRequest{
		Pubkey:   pubkey,
		Amount:   amount,
		Invoice:  invoice,
		Preimage: preimage,
	})
	return err
}

func (c *Client) GetPaymentHistory(pubkey []byte) ([]database.Payment, error) {
	resp, err := c.client.GetPaymentHistory(context.Background(), &orlydbv1.GetPaymentHistoryRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToPaymentList(resp), nil
}

func (c *Client) ExtendBlossomSubscription(pubkey []byte, tier string, storageMB int64, daysExtended int) error {
	_, err := c.client.ExtendBlossomSubscription(context.Background(), &orlydbv1.ExtendBlossomSubscriptionRequest{
		Pubkey:       pubkey,
		Tier:         tier,
		StorageMb:    storageMB,
		DaysExtended: int32(daysExtended),
	})
	return err
}

func (c *Client) GetBlossomStorageQuota(pubkey []byte) (quotaMB int64, err error) {
	resp, err := c.client.GetBlossomStorageQuota(context.Background(), &orlydbv1.GetBlossomStorageQuotaRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return 0, err
	}
	return resp.QuotaMb, nil
}

func (c *Client) IsFirstTimeUser(pubkey []byte) (bool, error) {
	resp, err := c.client.IsFirstTimeUser(context.Background(), &orlydbv1.IsFirstTimeUserRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return false, err
	}
	return resp.FirstTime, nil
}

// === NIP-43 ===

func (c *Client) AddNIP43Member(pubkey []byte, inviteCode string) error {
	_, err := c.client.AddNIP43Member(context.Background(), &orlydbv1.AddNIP43MemberRequest{
		Pubkey:     pubkey,
		InviteCode: inviteCode,
	})
	return err
}

func (c *Client) RemoveNIP43Member(pubkey []byte) error {
	_, err := c.client.RemoveNIP43Member(context.Background(), &orlydbv1.RemoveNIP43MemberRequest{
		Pubkey: pubkey,
	})
	return err
}

func (c *Client) IsNIP43Member(pubkey []byte) (isMember bool, err error) {
	resp, err := c.client.IsNIP43Member(context.Background(), &orlydbv1.IsNIP43MemberRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return false, err
	}
	return resp.IsMember, nil
}

func (c *Client) GetNIP43Membership(pubkey []byte) (*database.NIP43Membership, error) {
	resp, err := c.client.GetNIP43Membership(context.Background(), &orlydbv1.GetNIP43MembershipRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToNIP43Membership(resp), nil
}

func (c *Client) GetAllNIP43Members() ([][]byte, error) {
	resp, err := c.client.GetAllNIP43Members(context.Background(), &orlydbv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Pubkeys, nil
}

func (c *Client) StoreInviteCode(code string, expiresAt time.Time) error {
	_, err := c.client.StoreInviteCode(context.Background(), &orlydbv1.StoreInviteCodeRequest{
		Code:      code,
		ExpiresAt: expiresAt.Unix(),
	})
	return err
}

func (c *Client) ValidateInviteCode(code string) (valid bool, err error) {
	resp, err := c.client.ValidateInviteCode(context.Background(), &orlydbv1.ValidateInviteCodeRequest{
		Code: code,
	})
	if err != nil {
		return false, err
	}
	return resp.Valid, nil
}

func (c *Client) DeleteInviteCode(code string) error {
	_, err := c.client.DeleteInviteCode(context.Background(), &orlydbv1.DeleteInviteCodeRequest{
		Code: code,
	})
	return err
}

func (c *Client) PublishNIP43MembershipEvent(kind int, pubkey []byte) error {
	_, err := c.client.PublishNIP43MembershipEvent(context.Background(), &orlydbv1.PublishNIP43MembershipEventRequest{
		Kind:   int32(kind),
		Pubkey: pubkey,
	})
	return err
}

// === Migrations ===

func (c *Client) RunMigrations() {
	_, _ = c.client.RunMigrations(context.Background(), &orlydbv1.Empty{})
}

// === Query Cache ===

func (c *Client) GetCachedJSON(f *filter.F) ([][]byte, bool) {
	resp, err := c.client.GetCachedJSON(context.Background(), &orlydbv1.GetCachedJSONRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, false
	}
	return resp.JsonItems, resp.Found
}

func (c *Client) CacheMarshaledJSON(f *filter.F, marshaledJSON [][]byte) {
	_, _ = c.client.CacheMarshaledJSON(context.Background(), &orlydbv1.CacheMarshaledJSONRequest{
		Filter:    orlydbv1.FilterToProto(f),
		JsonItems: marshaledJSON,
	})
}

func (c *Client) GetCachedEvents(f *filter.F) (event.S, bool) {
	resp, err := c.client.GetCachedEvents(context.Background(), &orlydbv1.GetCachedEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
	})
	if err != nil {
		return nil, false
	}
	return orlydbv1.ProtoToEvents(resp.Events), resp.Found
}

func (c *Client) CacheEvents(f *filter.F, events event.S) {
	_, _ = c.client.CacheEvents(context.Background(), &orlydbv1.CacheEventsRequest{
		Filter: orlydbv1.FilterToProto(f),
		Events: orlydbv1.EventsToProto(events),
	})
}

func (c *Client) InvalidateQueryCache() {
	_, _ = c.client.InvalidateQueryCache(context.Background(), &orlydbv1.Empty{})
}

// === Access Tracking ===

func (c *Client) RecordEventAccess(serial uint64, connectionID string) error {
	_, err := c.client.RecordEventAccess(context.Background(), &orlydbv1.RecordEventAccessRequest{
		Serial:       serial,
		ConnectionId: connectionID,
	})
	return err
}

func (c *Client) GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	resp, err := c.client.GetEventAccessInfo(context.Background(), &orlydbv1.GetEventAccessInfoRequest{
		Serial: serial,
	})
	if err != nil {
		return 0, 0, err
	}
	return resp.LastAccess, resp.AccessCount, nil
}

func (c *Client) GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error) {
	resp, err := c.client.GetLeastAccessedEvents(context.Background(), &orlydbv1.GetLeastAccessedEventsRequest{
		Limit:     int32(limit),
		MinAgeSec: minAgeSec,
	})
	if err != nil {
		return nil, err
	}
	return resp.Serials, nil
}

// === Blob Storage (Blossom) ===

func (c *Client) SaveBlob(sha256Hash []byte, data []byte, pubkey []byte, mimeType string, extension string) error {
	_, err := c.client.SaveBlob(context.Background(), &orlydbv1.SaveBlobRequest{
		Sha256Hash: sha256Hash,
		Data:       data,
		Pubkey:     pubkey,
		MimeType:   mimeType,
		Extension:  extension,
	})
	return err
}

func (c *Client) GetBlob(sha256Hash []byte) (data []byte, metadata *database.BlobMetadata, err error) {
	resp, err := c.client.GetBlob(context.Background(), &orlydbv1.GetBlobRequest{
		Sha256Hash: sha256Hash,
	})
	if err != nil {
		return nil, nil, err
	}
	if !resp.Found {
		return nil, nil, nil
	}
	return resp.Data, orlydbv1.ProtoToBlobMetadata(resp.Metadata), nil
}

func (c *Client) HasBlob(sha256Hash []byte) (exists bool, err error) {
	resp, err := c.client.HasBlob(context.Background(), &orlydbv1.HasBlobRequest{
		Sha256Hash: sha256Hash,
	})
	if err != nil {
		return false, err
	}
	return resp.Exists, nil
}

func (c *Client) DeleteBlob(sha256Hash []byte, pubkey []byte) error {
	_, err := c.client.DeleteBlob(context.Background(), &orlydbv1.DeleteBlobRequest{
		Sha256Hash: sha256Hash,
		Pubkey:     pubkey,
	})
	return err
}

func (c *Client) ListBlobs(pubkey []byte, since, until int64) ([]*database.BlobDescriptor, error) {
	resp, err := c.client.ListBlobs(context.Background(), &orlydbv1.ListBlobsRequest{
		Pubkey: pubkey,
		Since:  since,
		Until:  until,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToBlobDescriptorList(resp.Descriptors), nil
}

func (c *Client) GetBlobMetadata(sha256Hash []byte) (*database.BlobMetadata, error) {
	resp, err := c.client.GetBlobMetadata(context.Background(), &orlydbv1.GetBlobMetadataRequest{
		Sha256Hash: sha256Hash,
	})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToBlobMetadata(resp), nil
}

func (c *Client) GetTotalBlobStorageUsed(pubkey []byte) (totalMB int64, err error) {
	resp, err := c.client.GetTotalBlobStorageUsed(context.Background(), &orlydbv1.GetTotalBlobStorageUsedRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return 0, err
	}
	return resp.TotalMb, nil
}

func (c *Client) SaveBlobReport(sha256Hash []byte, reportData []byte) error {
	_, err := c.client.SaveBlobReport(context.Background(), &orlydbv1.SaveBlobReportRequest{
		Sha256Hash: sha256Hash,
		ReportData: reportData,
	})
	return err
}

func (c *Client) ListAllBlobUserStats() ([]*database.UserBlobStats, error) {
	resp, err := c.client.ListAllBlobUserStats(context.Background(), &orlydbv1.Empty{})
	if err != nil {
		return nil, err
	}
	return orlydbv1.ProtoToUserBlobStatsList(resp.Stats), nil
}

func (c *Client) ReconcileBlobMetadata() (reconciled int, err error) {
	resp, err := c.client.ReconcileBlobMetadata(context.Background(), &orlydbv1.Empty{})
	if err != nil {
		return 0, err
	}
	return int(resp.Reconciled), nil
}

// === Utility ===

func (c *Client) EventIdsBySerial(start uint64, count int) (evs []uint64, err error) {
	resp, err := c.client.EventIdsBySerial(context.Background(), &orlydbv1.EventIdsBySerialRequest{
		Start: start,
		Count: int32(count),
	})
	if err != nil {
		return nil, err
	}
	return resp.EventIds, nil
}

// === Helper Methods ===

type eventStream interface {
	Recv() (*orlydbv1.EventBatch, error)
}

func (c *Client) collectStreamedEvents(stream eventStream) (event.S, error) {
	var result event.S
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		for _, ev := range batch.Events {
			result = append(result, orlydbv1.ProtoToEvent(ev))
		}
	}
}

func protoToUint40s(resp *orlydbv1.SerialList) indextypes.Uint40s {
	if resp == nil {
		return nil
	}
	result := make(indextypes.Uint40s, 0, len(resp.Serials))
	for _, s := range resp.Serials {
		u := &indextypes.Uint40{}
		_ = u.Set(s)
		result = append(result, u)
	}
	return result
}
