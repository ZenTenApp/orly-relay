// Package grpc provides a gRPC client for the relay group service.
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"git.smesh.lol/orly/pkg/lol/log"

	commonv1 "git.smesh.lol/orly/pkg/proto/orlysync/common/v1"
	relaygroupv1 "git.smesh.lol/orly/pkg/proto/orlysync/relaygroup/v1"
)

// Client is a gRPC client for the relay group service.
type Client struct {
	conn   *grpc.ClientConn
	client relaygroupv1.RelayGroupServiceClient
	ready  chan struct{}
}

// ClientConfig holds configuration for the gRPC client.
type ClientConfig struct {
	ServerAddress  string
	ConnectTimeout time.Duration
}

// New creates a new gRPC relay group client.
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
		client: relaygroupv1.NewRelayGroupServiceClient(conn),
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
				log.I.F("gRPC relay group client connected and ready")
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

// FindAuthoritativeConfig finds the authoritative relay group configuration.
func (c *Client) FindAuthoritativeConfig(ctx context.Context) (*relaygroupv1.RelayGroupConfigResponse, error) {
	return c.client.FindAuthoritativeConfig(ctx, &commonv1.Empty{})
}

// GetRelays returns the list of relays from the authoritative config.
func (c *Client) GetRelays(ctx context.Context) ([]string, error) {
	resp, err := c.client.GetRelays(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Relays, nil
}

// IsAuthorizedPublisher checks if a pubkey can publish relay group configs.
func (c *Client) IsAuthorizedPublisher(ctx context.Context, pubkey []byte) (bool, error) {
	resp, err := c.client.IsAuthorizedPublisher(ctx, &relaygroupv1.AuthorizedPublisherRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return false, err
	}
	return resp.Authorized, nil
}

// GetAuthorizedPubkeys returns all authorized publisher pubkeys.
func (c *Client) GetAuthorizedPubkeys(ctx context.Context) ([][]byte, error) {
	resp, err := c.client.GetAuthorizedPubkeys(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Pubkeys, nil
}

// ValidateRelayGroupEvent validates a relay group configuration event.
func (c *Client) ValidateRelayGroupEvent(ctx context.Context, event *commonv1.Event) (bool, string, error) {
	resp, err := c.client.ValidateRelayGroupEvent(ctx, &relaygroupv1.ValidateEventRequest{
		Event: event,
	})
	if err != nil {
		return false, "", err
	}
	return resp.Valid, resp.Error, nil
}

// HandleRelayGroupEvent processes a relay group event and triggers peer updates.
func (c *Client) HandleRelayGroupEvent(ctx context.Context, event *commonv1.Event) error {
	_, err := c.client.HandleRelayGroupEvent(ctx, &relaygroupv1.HandleEventRequest{
		Event: event,
	})
	return err
}
