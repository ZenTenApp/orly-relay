// Package mode provides a global ACL mode indicator that can be read by
// packages that need to know the current access control mode without creating
// circular dependencies.
package mode

import "next.orly.dev/pkg/utils/atomic"

// ACLMode holds the current ACL mode as a string.
// This is set by the ACL package when configured and can be read by other
// packages (like database) to adjust their behavior.
var ACLMode atomic.String

// IsOpen returns true if the ACL mode is "none" (open relay mode).
// In open mode, security filtering (expiration, deletion, privileged events)
// should be disabled.
func IsOpen() bool {
	return ACLMode.Load() == "none"
}
