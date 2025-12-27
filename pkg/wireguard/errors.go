package wireguard

import "errors"

var (
	// ErrInvalidKeyLength is returned when a key is not exactly 32 bytes.
	ErrInvalidKeyLength = errors.New("invalid key length: must be 32 bytes")

	// ErrServerNotRunning is returned when an operation requires a running server.
	ErrServerNotRunning = errors.New("wireguard server not running")

	// ErrEndpointRequired is returned when WireGuard is enabled but no endpoint is set.
	ErrEndpointRequired = errors.New("ORLY_WG_ENDPOINT is required when WireGuard is enabled")

	// ErrInvalidNetwork is returned when the network CIDR is invalid.
	ErrInvalidNetwork = errors.New("invalid network CIDR")

	// ErrPeerNotFound is returned when a peer lookup fails.
	ErrPeerNotFound = errors.New("peer not found")

	// ErrIPExhausted is returned when no more IPs are available in the network.
	ErrIPExhausted = errors.New("no more IP addresses available in network")
)
