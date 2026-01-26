// Package grpc provides a gRPC client for the negentropy sync service.
package grpc

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"lol.mleku.dev/log"

	negentropyiface "next.orly.dev/pkg/interfaces/negentropy"
	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	negentropyv1 "next.orly.dev/pkg/proto/orlysync/negentropy/v1"
)

// Verify Client implements the Handler interface
var _ negentropyiface.Handler = (*Client)(nil)

// Client is a gRPC client for the negentropy sync service.
type Client struct {
	conn   *grpc.ClientConn
	client negentropyv1.NegentropyServiceClient
	ready  chan struct{}
}

// ClientConfig holds configuration for the gRPC client.
type ClientConfig struct {
	ServerAddress  string
	ConnectTimeout time.Duration
}

// New creates a new gRPC negentropy sync client.
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
			grpc.MaxCallRecvMsgSize(16<<20), // 16MB
			grpc.MaxCallSendMsgSize(16<<20), // 16MB
		),
	)
	if err != nil {
		return nil, err
	}

	c := &Client{
		conn:   conn,
		client: negentropyv1.NewNegentropyServiceClient(conn),
		ready:  make(chan struct{}),
	}

	go c.waitForReady(ctx)

	return c, nil
}

func (c *Client) waitForReady(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			resp, err := c.client.Ready(ctx, &commonv1.Empty{})
			if err == nil && resp.Ready {
				close(c.ready)
				log.I.F("gRPC negentropy sync client connected and ready")
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Ready returns a channel that closes when the client is ready.
func (c *Client) Ready() <-chan struct{} {
	return c.ready
}

// Start starts the background relay-to-relay sync.
func (c *Client) Start(ctx context.Context) error {
	_, err := c.client.Start(ctx, &commonv1.Empty{})
	return err
}

// Stop stops the background sync.
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &commonv1.Empty{})
	return err
}

// HandleNegOpen processes a NEG-OPEN message from a client.
// Returns: message, haveIDs, needIDs, complete, errorStr, error
func (c *Client) HandleNegOpen(ctx context.Context, connectionID, subscriptionID string, filter *commonv1.Filter, initialMessage []byte) ([]byte, [][]byte, [][]byte, bool, string, error) {
	resp, err := c.client.HandleNegOpen(ctx, &negentropyv1.NegOpenRequest{
		ConnectionId:   connectionID,
		SubscriptionId: subscriptionID,
		Filter:         filter,
		InitialMessage: initialMessage,
	})
	if err != nil {
		return nil, nil, nil, false, "", err
	}
	return resp.Message, resp.HaveIds, resp.NeedIds, resp.Complete, resp.Error, nil
}

// HandleNegMsg processes a NEG-MSG message from a client.
func (c *Client) HandleNegMsg(ctx context.Context, connectionID, subscriptionID string, message []byte) ([]byte, [][]byte, [][]byte, bool, string, error) {
	resp, err := c.client.HandleNegMsg(ctx, &negentropyv1.NegMsgRequest{
		ConnectionId:   connectionID,
		SubscriptionId: subscriptionID,
		Message:        message,
	})
	if err != nil {
		return nil, nil, nil, false, "", err
	}
	return resp.Message, resp.HaveIds, resp.NeedIds, resp.Complete, resp.Error, nil
}

// HandleNegClose processes a NEG-CLOSE message from a client.
func (c *Client) HandleNegClose(ctx context.Context, connectionID, subscriptionID string) error {
	_, err := c.client.HandleNegClose(ctx, &negentropyv1.NegCloseRequest{
		ConnectionId:   connectionID,
		SubscriptionId: subscriptionID,
	})
	return err
}

// SyncProgress represents progress during a peer sync operation.
type SyncProgress struct {
	PeerURL      string
	Round        int32
	HaveCount    int64
	NeedCount    int64
	FetchedCount int64
	SentCount    int64
	Complete     bool
	Error        string
}

// SyncWithPeer initiates negentropy sync with a specific peer relay.
// Returns a channel that receives progress updates.
func (c *Client) SyncWithPeer(ctx context.Context, peerURL string, filter *commonv1.Filter, since int64) (<-chan *SyncProgress, error) {
	stream, err := c.client.SyncWithPeer(ctx, &negentropyv1.SyncPeerRequest{
		PeerUrl: peerURL,
		Filter:  filter,
		Since:   since,
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan *SyncProgress, 10)
	go func() {
		defer close(ch)
		for {
			progress, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				ch <- &SyncProgress{
					PeerURL: peerURL,
					Error:   err.Error(),
				}
				return
			}
			ch <- &SyncProgress{
				PeerURL:      progress.PeerUrl,
				Round:        progress.Round,
				HaveCount:    progress.HaveCount,
				NeedCount:    progress.NeedCount,
				FetchedCount: progress.FetchedCount,
				SentCount:    progress.SentCount,
				Complete:     progress.Complete,
				Error:        progress.Error,
			}
		}
	}()

	return ch, nil
}

// SyncStatus represents the sync status response.
type SyncStatus struct {
	Active     bool
	LastSync   int64
	PeerCount  int32
	PeerStates []*PeerSyncState
}

// PeerSyncState represents sync state for a peer.
type PeerSyncState struct {
	PeerURL             string
	LastSync            int64
	EventsSynced        int64
	Status              string
	LastError           string
	ConsecutiveFailures int32
}

// GetSyncStatus returns the current sync status.
func (c *Client) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	resp, err := c.client.GetSyncStatus(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}

	states := make([]*PeerSyncState, 0, len(resp.PeerStates))
	for _, ps := range resp.PeerStates {
		states = append(states, &PeerSyncState{
			PeerURL:             ps.PeerUrl,
			LastSync:            ps.LastSync,
			EventsSynced:        ps.EventsSynced,
			Status:              ps.Status,
			LastError:           ps.LastError,
			ConsecutiveFailures: ps.ConsecutiveFailures,
		})
	}

	return &SyncStatus{
		Active:     resp.Active,
		LastSync:   resp.LastSync,
		PeerCount:  resp.PeerCount,
		PeerStates: states,
	}, nil
}

// GetPeers returns the list of negentropy sync peers.
func (c *Client) GetPeers(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetPeers(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Peers, nil
}

// AddPeer adds a peer for negentropy sync.
func (c *Client) AddPeer(ctx context.Context, peerURL string) error {
	_, err := c.client.AddPeer(ctx, &negentropyv1.AddPeerRequest{
		PeerUrl: peerURL,
	})
	return err
}

// RemovePeer removes a peer from negentropy sync.
func (c *Client) RemovePeer(ctx context.Context, peerURL string) error {
	_, err := c.client.RemovePeer(ctx, &negentropyv1.RemovePeerRequest{
		PeerUrl: peerURL,
	})
	return err
}

// TriggerSync manually triggers sync with a specific peer or all peers.
func (c *Client) TriggerSync(ctx context.Context, peerURL string, filter *commonv1.Filter) error {
	_, err := c.client.TriggerSync(ctx, &negentropyv1.TriggerSyncRequest{
		PeerUrl: peerURL,
		Filter:  filter,
	})
	return err
}

// GetPeerSyncState returns sync state for a specific peer.
func (c *Client) GetPeerSyncState(ctx context.Context, peerURL string) (*PeerSyncState, bool, error) {
	resp, err := c.client.GetPeerSyncState(ctx, &negentropyv1.PeerSyncStateRequest{
		PeerUrl: peerURL,
	})
	if err != nil {
		return nil, false, err
	}
	if !resp.Found {
		return nil, false, nil
	}
	return &PeerSyncState{
		PeerURL:             resp.State.PeerUrl,
		LastSync:            resp.State.LastSync,
		EventsSynced:        resp.State.EventsSynced,
		Status:              resp.State.Status,
		LastError:           resp.State.LastError,
		ConsecutiveFailures: resp.State.ConsecutiveFailures,
	}, true, nil
}

// ListSessions returns active client negentropy sessions.
func (c *Client) ListSessions(ctx context.Context) ([]*negentropyiface.ClientSession, error) {
	resp, err := c.client.ListSessions(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}

	sessions := make([]*negentropyiface.ClientSession, 0, len(resp.Sessions))
	for _, s := range resp.Sessions {
		sessions = append(sessions, &negentropyiface.ClientSession{
			SubscriptionID: s.SubscriptionId,
			ConnectionID:   s.ConnectionId,
			CreatedAt:      s.CreatedAt,
			LastActivity:   s.LastActivity,
			RoundCount:     s.RoundCount,
		})
	}
	return sessions, nil
}

// CloseSession forcefully closes a client session.
func (c *Client) CloseSession(ctx context.Context, connectionID, subscriptionID string) error {
	_, err := c.client.CloseSession(ctx, &negentropyv1.CloseSessionRequest{
		ConnectionId:   connectionID,
		SubscriptionId: subscriptionID,
	})
	return err
}
