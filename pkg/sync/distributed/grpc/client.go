// Package grpc provides a gRPC client for the distributed sync service.
package grpc

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"next.orly.dev/pkg/lol/log"

	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	distributedv1 "next.orly.dev/pkg/proto/orlysync/distributed/v1"
)

// Client is a gRPC client for the distributed sync service.
type Client struct {
	conn   *grpc.ClientConn
	client distributedv1.DistributedSyncServiceClient
	ready  chan struct{}
}

// ClientConfig holds configuration for the gRPC client.
type ClientConfig struct {
	ServerAddress  string
	ConnectTimeout time.Duration
}

// New creates a new gRPC distributed sync client.
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
		client: distributedv1.NewDistributedSyncServiceClient(conn),
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
				log.I.F("gRPC distributed sync client connected and ready")
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

// GetInfo returns current sync service information.
func (c *Client) GetInfo(ctx context.Context) (*commonv1.SyncInfo, error) {
	return c.client.GetInfo(ctx, &commonv1.Empty{})
}

// GetCurrentSerial returns the current serial number.
func (c *Client) GetCurrentSerial(ctx context.Context) (uint64, error) {
	resp, err := c.client.GetCurrentSerial(ctx, &distributedv1.CurrentRequest{})
	if err != nil {
		return 0, err
	}
	return resp.Serial, nil
}

// GetEventIDs returns event IDs for a serial range.
func (c *Client) GetEventIDs(ctx context.Context, from, to uint64) (map[string]uint64, error) {
	resp, err := c.client.GetEventIDs(ctx, &distributedv1.EventIDsRequest{
		From: from,
		To:   to,
	})
	if err != nil {
		return nil, err
	}
	return resp.EventMap, nil
}

// HandleCurrentRequest proxies an HTTP current request.
func (c *Client) HandleCurrentRequest(ctx context.Context, method string, body []byte, headers map[string]string) (int, []byte, map[string]string, error) {
	resp, err := c.client.HandleCurrentRequest(ctx, &commonv1.HTTPRequest{
		Method:  method,
		Body:    body,
		Headers: headers,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	return int(resp.StatusCode), resp.Body, resp.Headers, nil
}

// HandleEventIDsRequest proxies an HTTP event-ids request.
func (c *Client) HandleEventIDsRequest(ctx context.Context, method string, body []byte, headers map[string]string) (int, []byte, map[string]string, error) {
	resp, err := c.client.HandleEventIDsRequest(ctx, &commonv1.HTTPRequest{
		Method:  method,
		Body:    body,
		Headers: headers,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	return int(resp.StatusCode), resp.Body, resp.Headers, nil
}

// GetPeers returns the current list of sync peers.
func (c *Client) GetPeers(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetPeers(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Peers, nil
}

// UpdatePeers updates the peer list.
func (c *Client) UpdatePeers(ctx context.Context, peers []string) error {
	_, err := c.client.UpdatePeers(ctx, &distributedv1.UpdatePeersRequest{
		Peers: peers,
	})
	return err
}

// IsAuthorizedPeer checks if a peer is authorized.
func (c *Client) IsAuthorizedPeer(ctx context.Context, peerURL, expectedPubkey string) (bool, error) {
	resp, err := c.client.IsAuthorizedPeer(ctx, &distributedv1.AuthorizedPeerRequest{
		PeerUrl:        peerURL,
		ExpectedPubkey: expectedPubkey,
	})
	if err != nil {
		return false, err
	}
	return resp.Authorized, nil
}

// GetPeerPubkey fetches the pubkey for a peer relay.
func (c *Client) GetPeerPubkey(ctx context.Context, peerURL string) (string, error) {
	resp, err := c.client.GetPeerPubkey(ctx, &distributedv1.PeerPubkeyRequest{
		PeerUrl: peerURL,
	})
	if err != nil {
		return "", err
	}
	if resp.Pubkey == "" {
		return "", errors.New("peer pubkey not found")
	}
	return resp.Pubkey, nil
}

// UpdateSerial updates the current serial from database.
func (c *Client) UpdateSerial(ctx context.Context) error {
	_, err := c.client.UpdateSerial(ctx, &commonv1.Empty{})
	return err
}

// NotifyNewEvent notifies the service of a new event.
func (c *Client) NotifyNewEvent(ctx context.Context, eventID []byte, serial uint64) error {
	_, err := c.client.NotifyNewEvent(ctx, &distributedv1.NewEventNotification{
		EventId: eventID,
		Serial:  serial,
	})
	return err
}

// TriggerSync manually triggers a sync cycle.
func (c *Client) TriggerSync(ctx context.Context) error {
	_, err := c.client.TriggerSync(ctx, &commonv1.Empty{})
	return err
}

// GetSyncStatus returns current sync status.
func (c *Client) GetSyncStatus(ctx context.Context) (uint64, map[string]uint64, error) {
	resp, err := c.client.GetSyncStatus(ctx, &commonv1.Empty{})
	if err != nil {
		return 0, nil, err
	}
	peerSerials := make(map[string]uint64)
	for _, p := range resp.Peers {
		peerSerials[p.Url] = p.LastSerial
	}
	return resp.CurrentSerial, peerSerials, nil
}
