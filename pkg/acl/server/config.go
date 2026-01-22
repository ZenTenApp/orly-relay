// Package server provides a shared gRPC ACL server implementation.
package server

import "time"

// Config holds configuration for the ACL gRPC server.
type Config struct {
	// Listen is the gRPC server listen address
	Listen string

	// ACLMode is the active ACL mode (none, follows, managed, curating)
	ACLMode string

	// LogLevel is the logging level
	LogLevel string

	// Owner and admin lists
	Owners []string
	Admins []string

	// Bootstrap relays for follow list syncing
	BootstrapRelays []string

	// Relay addresses (self)
	RelayAddresses []string

	// Follows ACL configuration
	FollowListFrequency     time.Duration
	FollowsThrottleEnabled  bool
	FollowsThrottlePerEvent time.Duration
	FollowsThrottleMaxDelay time.Duration
}
