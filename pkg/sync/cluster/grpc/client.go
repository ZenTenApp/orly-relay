// Package grpc provides a gRPC client for the cluster sync service.
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"next.orly.dev/pkg/lol/log"

	commonv1 "next.orly.dev/pkg/proto/orlysync/common/v1"
	clusterv1 "next.orly.dev/pkg/proto/orlysync/cluster/v1"
)

// Client is a gRPC client for the cluster sync service.
type Client struct {
	conn   *grpc.ClientConn
	client clusterv1.ClusterSyncServiceClient
	ready  chan struct{}
}

// ClientConfig holds configuration for the gRPC client.
type ClientConfig struct {
	ServerAddress  string
	ConnectTimeout time.Duration
}

// New creates a new gRPC cluster sync client.
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
		client: clusterv1.NewClusterSyncServiceClient(conn),
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
				log.I.F("gRPC cluster sync client connected and ready")
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

// Start starts the cluster polling loop.
func (c *Client) Start(ctx context.Context) error {
	_, err := c.client.Start(ctx, &commonv1.Empty{})
	return err
}

// Stop stops the cluster polling loop.
func (c *Client) Stop(ctx context.Context) error {
	_, err := c.client.Stop(ctx, &commonv1.Empty{})
	return err
}

// HandleLatestSerial proxies an HTTP latest serial request.
func (c *Client) HandleLatestSerial(ctx context.Context, method string, body []byte, headers map[string]string) (int, []byte, map[string]string, error) {
	resp, err := c.client.HandleLatestSerial(ctx, &commonv1.HTTPRequest{
		Method:  method,
		Body:    body,
		Headers: headers,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	return int(resp.StatusCode), resp.Body, resp.Headers, nil
}

// HandleEventsRange proxies an HTTP events range request.
func (c *Client) HandleEventsRange(ctx context.Context, method string, body []byte, headers map[string]string, queryString string) (int, []byte, map[string]string, error) {
	resp, err := c.client.HandleEventsRange(ctx, &commonv1.HTTPRequest{
		Method:      method,
		Body:        body,
		Headers:     headers,
		QueryString: queryString,
	})
	if err != nil {
		return 0, nil, nil, err
	}
	return int(resp.StatusCode), resp.Body, resp.Headers, nil
}

// GetMembers returns the current cluster members.
func (c *Client) GetMembers(ctx context.Context) ([]*clusterv1.ClusterMember, error) {
	resp, err := c.client.GetMembers(ctx, &commonv1.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Members, nil
}

// UpdateMembership updates cluster membership.
func (c *Client) UpdateMembership(ctx context.Context, relayURLs []string) error {
	_, err := c.client.UpdateMembership(ctx, &clusterv1.UpdateMembershipRequest{
		RelayUrls: relayURLs,
	})
	return err
}

// HandleMembershipEvent processes a cluster membership event.
func (c *Client) HandleMembershipEvent(ctx context.Context, event *commonv1.Event) error {
	_, err := c.client.HandleMembershipEvent(ctx, &clusterv1.MembershipEventRequest{
		Event: event,
	})
	return err
}

// GetClusterStatus returns overall cluster status.
func (c *Client) GetClusterStatus(ctx context.Context) (*clusterv1.ClusterStatusResponse, error) {
	return c.client.GetClusterStatus(ctx, &commonv1.Empty{})
}

// GetMemberStatus returns status for a specific member.
func (c *Client) GetMemberStatus(ctx context.Context, httpURL string) (*clusterv1.MemberStatusResponse, error) {
	return c.client.GetMemberStatus(ctx, &clusterv1.MemberStatusRequest{
		HttpUrl: httpURL,
	})
}

// GetLatestSerial returns the latest serial from this relay's database.
func (c *Client) GetLatestSerial(ctx context.Context) (uint64, int64, error) {
	resp, err := c.client.GetLatestSerial(ctx, &commonv1.Empty{})
	if err != nil {
		return 0, 0, err
	}
	return resp.Serial, resp.Timestamp, nil
}

// GetEventsInRange returns event info for a serial range.
func (c *Client) GetEventsInRange(ctx context.Context, from, to uint64, limit int32) ([]*clusterv1.EventInfo, bool, uint64, error) {
	resp, err := c.client.GetEventsInRange(ctx, &clusterv1.EventsRangeRequest{
		From:  from,
		To:    to,
		Limit: limit,
	})
	if err != nil {
		return nil, false, 0, err
	}
	return resp.Events, resp.HasMore, resp.NextFrom, nil
}
