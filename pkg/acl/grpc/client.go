// Package grpc provides a gRPC client that implements the acl.I interface.
// This allows the relay to use a remote ACL server via gRPC.
package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"

	"next.orly.dev/pkg/nostr/encoders/event"
	acliface "next.orly.dev/pkg/interfaces/acl"
	orlyaclv1 "next.orly.dev/pkg/proto/orlyacl/v1"
	orlydbv1 "next.orly.dev/pkg/proto/orlydb/v1"
)

// Client implements the acl.I interface via gRPC.
type Client struct {
	conn   *grpc.ClientConn
	client orlyaclv1.ACLServiceClient
	ready  chan struct{}
	mode   string
}

// Verify Client implements acl.I at compile time.
var _ acliface.I = (*Client)(nil)

// Verify Client implements acl.PolicyChecker at compile time.
var _ acliface.PolicyChecker = (*Client)(nil)

// ClientConfig holds configuration for the gRPC ACL client.
type ClientConfig struct {
	ServerAddress  string
	ConnectTimeout time.Duration
}

// New creates a new gRPC ACL client.
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
		client: orlyaclv1.NewACLServiceClient(conn),
		ready:  make(chan struct{}),
	}

	// Check if server is ready and get mode
	go c.waitForReady(ctx)

	return c, nil
}

func (c *Client) waitForReady(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			resp, err := c.client.Ready(ctx, &orlyaclv1.Empty{})
			if err == nil && resp.Ready {
				// Get mode from server
				modeResp, err := c.client.GetMode(ctx, &orlyaclv1.Empty{})
				if err == nil {
					c.mode = modeResp.Mode
				}
				close(c.ready)
				log.I.F("gRPC ACL client connected and ready, mode: %s", c.mode)
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

// === acl.I Interface Implementation ===

func (c *Client) Configure(cfg ...any) error {
	// Configuration is done on the server side
	// The client just passes through to the server
	return nil
}

func (c *Client) GetAccessLevel(pub []byte, address string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetAccessLevel(ctx, &orlyaclv1.AccessLevelRequest{
		Pubkey:  pub,
		Address: address,
	})
	if chk.E(err) {
		return "none"
	}
	return resp.Level
}

func (c *Client) GetACLInfo() (name, description, documentation string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetACLInfo(ctx, &orlyaclv1.Empty{})
	if chk.E(err) {
		return "", "", ""
	}
	return resp.Name, resp.Description, resp.Documentation
}

func (c *Client) Syncer() {
	// The syncer runs on the ACL server, not the client
	// This is a no-op for the gRPC client
}

func (c *Client) Type() string {
	return c.mode
}

// === acl.PolicyChecker Interface Implementation ===

func (c *Client) CheckPolicy(ev *event.E) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.CheckPolicy(ctx, &orlyaclv1.PolicyCheckRequest{
		Event: orlydbv1.EventToProto(ev),
	})
	if err != nil {
		return false, err
	}
	if resp.Error != "" {
		return resp.Allowed, &policyError{msg: resp.Error}
	}
	return resp.Allowed, nil
}

// policyError is a simple error type for policy check failures
type policyError struct {
	msg string
}

func (e *policyError) Error() string {
	return e.msg
}

// === Follows ACL Methods ===

// GetThrottleDelay returns the progressive throttle delay for a pubkey.
func (c *Client) GetThrottleDelay(pubkey []byte, ip string) time.Duration {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetThrottleDelay(ctx, &orlyaclv1.ThrottleDelayRequest{
		Pubkey: pubkey,
		Ip:     ip,
	})
	if chk.E(err) {
		return 0
	}
	return time.Duration(resp.DelayMs) * time.Millisecond
}

// AddFollow adds a pubkey to the followed list.
func (c *Client) AddFollow(pubkey []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.AddFollow(ctx, &orlyaclv1.AddFollowRequest{
		Pubkey: pubkey,
	})
	return err
}

// GetFollowedPubkeys returns all followed pubkeys.
func (c *Client) GetFollowedPubkeys() [][]byte {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.client.GetFollowedPubkeys(ctx, &orlyaclv1.Empty{})
	if chk.E(err) {
		return nil
	}
	return resp.Pubkeys
}

// GetAdminRelays returns the admin relay URLs.
func (c *Client) GetAdminRelays() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetAdminRelays(ctx, &orlyaclv1.Empty{})
	if chk.E(err) {
		return nil
	}
	return resp.Urls
}

// === Managed ACL Methods ===

// BanPubkey adds a pubkey to the ban list.
func (c *Client) BanPubkey(pubkey, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.BanPubkey(ctx, &orlyaclv1.BanPubkeyRequest{
		Pubkey: pubkey,
		Reason: reason,
	})
	return err
}

// UnbanPubkey removes a pubkey from the ban list.
func (c *Client) UnbanPubkey(pubkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.UnbanPubkey(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	return err
}

// AllowPubkey adds a pubkey to the allow list.
func (c *Client) AllowPubkey(pubkey, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.AllowPubkey(ctx, &orlyaclv1.AllowPubkeyRequest{
		Pubkey: pubkey,
		Reason: reason,
	})
	return err
}

// DisallowPubkey removes a pubkey from the allow list.
func (c *Client) DisallowPubkey(pubkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.DisallowPubkey(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	return err
}

// BlockIP adds an IP to the block list.
func (c *Client) BlockIP(ip, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.BlockIP(ctx, &orlyaclv1.BlockIPRequest{
		Ip:     ip,
		Reason: reason,
	})
	return err
}

// UnblockIP removes an IP from the block list.
func (c *Client) UnblockIP(ip string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.UnblockIP(ctx, &orlyaclv1.IPRequest{
		Ip: ip,
	})
	return err
}

// UpdatePeerAdmins updates the peer relay identity pubkeys.
func (c *Client) UpdatePeerAdmins(peerPubkeys [][]byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.UpdatePeerAdmins(ctx, &orlyaclv1.UpdatePeerAdminsRequest{
		PeerPubkeys: peerPubkeys,
	})
	return err
}

// === Curating ACL Methods ===

// TrustPubkey adds a pubkey to the trusted list.
func (c *Client) TrustPubkey(pubkey, note string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.TrustPubkey(ctx, &orlyaclv1.TrustPubkeyRequest{
		Pubkey: pubkey,
		Note:   note,
	})
	return err
}

// UntrustPubkey removes a pubkey from the trusted list.
func (c *Client) UntrustPubkey(pubkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.UntrustPubkey(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	return err
}

// BlacklistPubkey adds a pubkey to the blacklist.
func (c *Client) BlacklistPubkey(pubkey, reason string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.BlacklistPubkey(ctx, &orlyaclv1.BlacklistPubkeyRequest{
		Pubkey: pubkey,
		Reason: reason,
	})
	return err
}

// UnblacklistPubkey removes a pubkey from the blacklist.
func (c *Client) UnblacklistPubkey(pubkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.UnblacklistPubkey(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	return err
}

// RateLimitCheck checks if a pubkey/IP can publish.
func (c *Client) RateLimitCheck(pubkey, ip string) (allowed bool, message string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.RateLimitCheck(ctx, &orlyaclv1.RateLimitCheckRequest{
		Pubkey: pubkey,
		Ip:     ip,
	})
	if err != nil {
		return false, "", err
	}
	return resp.Allowed, resp.Message, nil
}

// IsCuratingConfigured checks if curating mode is configured.
func (c *Client) IsCuratingConfigured() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.IsCuratingConfigured(ctx, &orlyaclv1.Empty{})
	if err != nil {
		return false, err
	}
	return resp.Value, nil
}

// === Paid ACL Methods ===

// SubscriptionInfo holds subscription details returned by GetSubscription.
type SubscriptionInfo struct {
	PubkeyHex string
	Alias     string
	Aliases   []string
	ExpiresAt time.Time
	CreatedAt time.Time
	HasAlias  bool
}

// SubscribePubkey activates a subscription for a pubkey.
func (c *Client) SubscribePubkey(pubkey string, expiresAt time.Time, invoiceHash, alias string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.client.SubscribePubkey(ctx, &orlyaclv1.SubscribeRequest{
		Pubkey:      pubkey,
		ExpiresAt:   expiresAt.Unix(),
		InvoiceHash: invoiceHash,
		Alias:       alias,
	})
	return err
}

// UnsubscribePubkey removes a subscription.
func (c *Client) UnsubscribePubkey(pubkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.UnsubscribePubkey(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	return err
}

// IsSubscribedPaid checks if a pubkey has an active paid subscription.
func (c *Client) IsSubscribedPaid(pubkey string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.IsSubscribed(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return false, err
	}
	return resp.Value, nil
}

// GetSubscription returns subscription details for a pubkey.
func (c *Client) GetSubscription(pubkey string) (*SubscriptionInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetSubscription(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return nil, err
	}
	aliases := resp.Aliases
	if len(aliases) == 0 && resp.Alias != "" {
		aliases = []string{resp.Alias}
	}
	return &SubscriptionInfo{
		PubkeyHex: resp.Pubkey,
		Alias:     resp.Alias,
		Aliases:   aliases,
		ExpiresAt: time.Unix(resp.ExpiresAt, 0),
		CreatedAt: time.Unix(resp.CreatedAt, 0),
		HasAlias:  resp.HasAlias,
	}, nil
}

// ClaimAlias claims an email alias for a pubkey.
func (c *Client) ClaimAlias(alias, pubkey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.client.ClaimAlias(ctx, &orlyaclv1.ClaimAliasRequest{
		Alias:  alias,
		Pubkey: pubkey,
	})
	return err
}

// GetAliasByPubkey returns the alias for a pubkey, or "" if none.
func (c *Client) GetAliasByPubkey(pubkey string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetAliasByPubkey(ctx, &orlyaclv1.PubkeyRequest{
		Pubkey: pubkey,
	})
	if err != nil {
		return "", err
	}
	return resp.Alias, nil
}

// GetPubkeyByAlias returns the pubkey for an alias, or "" if not found.
func (c *Client) GetPubkeyByAlias(alias string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetPubkeyByAlias(ctx, &orlyaclv1.AliasRequest{
		Alias: alias,
	})
	if err != nil {
		return "", err
	}
	return resp.Pubkey, nil
}

// IsAliasTaken checks if an alias is already claimed.
func (c *Client) IsAliasTaken(alias string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.IsAliasTaken(ctx, &orlyaclv1.AliasRequest{
		Alias: alias,
	})
	if err != nil {
		return false, err
	}
	return resp.Value, nil
}
