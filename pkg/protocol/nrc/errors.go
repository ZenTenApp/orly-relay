package nrc

import "errors"

var (
	// ErrUnauthorized is returned when a client is not authorized.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidSession is returned when a session ID is invalid or not found.
	ErrInvalidSession = errors.New("invalid session")
	// ErrTooManySubscriptions is returned when a session has too many subscriptions.
	ErrTooManySubscriptions = errors.New("too many subscriptions")
	// ErrInvalidMessageType is returned when the message type is invalid.
	ErrInvalidMessageType = errors.New("invalid message type")
	// ErrSessionExpired is returned when a session has expired.
	ErrSessionExpired = errors.New("session expired")
	// ErrDecryptionFailed is returned when message decryption fails.
	ErrDecryptionFailed = errors.New("decryption failed")
	// ErrEncryptionFailed is returned when message encryption fails.
	ErrEncryptionFailed = errors.New("encryption failed")
	// ErrRelayConnectionFailed is returned when connection to the local relay fails.
	ErrRelayConnectionFailed = errors.New("relay connection failed")
	// ErrRendezvousConnectionFailed is returned when connection to the rendezvous relay fails.
	ErrRendezvousConnectionFailed = errors.New("rendezvous relay connection failed")
)
