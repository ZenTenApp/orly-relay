// Package neterr defines interfaces for network error handling.
package neterr

// TimeoutError is an interface for errors that can indicate a timeout.
// This is compatible with net.Error's Timeout() method.
type TimeoutError interface {
	Timeout() bool
}
