package app

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"lol.mleku.dev/errorf"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/acl"
	"next.orly.dev/pkg/database"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/utils"
	atomicutils "next.orly.dev/pkg/utils/atomic"
)

type Listener struct {
	*Server
	conn             *websocket.Conn
	ctx              context.Context
	cancel           context.CancelFunc // Cancel function for this listener's context
	remote           string
	req              *http.Request
	challenge        atomicutils.Bytes
	authedPubkey     atomicutils.Bytes
	startTime        time.Time
	isBlacklisted    bool      // Marker to identify blacklisted IPs
	blacklistTimeout time.Time // When to timeout blacklisted connections
	writeChan        chan publish.WriteRequest // Channel for write requests (back to queued approach)
	writeDone        chan struct{}     // Closed when write worker exits
	// Message processing queue for async handling
	messageQueue     chan messageRequest // Buffered channel for message processing
	processingDone   chan struct{}       // Closed when message processor exits
	handlerWg        sync.WaitGroup      // Tracks spawned message handler goroutines
	handlerSem       chan struct{}       // Limits concurrent message handlers per connection
	authProcessing   sync.RWMutex        // Ensures AUTH completes before other messages check authentication
	// Flow control counters (atomic for concurrent access)
	droppedMessages  atomic.Int64 // Messages dropped due to full queue
	// Diagnostics: per-connection counters
	msgCount   int
	reqCount   int
	eventCount int
	// Subscription tracking for cleanup
	subscriptions    map[string]context.CancelFunc // Map of subscription ID to cancel function
	subscriptionsMu  sync.Mutex                     // Protects subscriptions map
}

type messageRequest struct {
	data   []byte
	remote string
}

// Ctx returns the listener's context, but creates a new context for each operation
// to prevent cancellation from affecting subsequent operations
func (l *Listener) Ctx() context.Context {
	return l.ctx
}

// DroppedMessages returns the total number of messages that were dropped
// because the message processing queue was full.
func (l *Listener) DroppedMessages() int {
	return int(l.droppedMessages.Load())
}

// RemainingCapacity returns the number of slots available in the message processing queue.
func (l *Listener) RemainingCapacity() int {
	return cap(l.messageQueue) - len(l.messageQueue)
}

// QueueMessage queues a message for asynchronous processing.
// Returns true if the message was queued, false if the queue was full.
func (l *Listener) QueueMessage(data []byte, remote string) bool {
	req := messageRequest{data: data, remote: remote}
	select {
	case l.messageQueue <- req:
		return true
	default:
		l.droppedMessages.Add(1)
		return false
	}
}


func (l *Listener) Write(p []byte) (n int, err error) {
	// Defensive: recover from any panic when sending to closed channel
	defer func() {
		if r := recover(); r != nil {
			log.D.F("ws->%s write panic recovered (channel likely closed): %v", l.remote, r)
			err = errorf.E("write channel closed")
			n = 0
		}
	}()

	// Send write request to channel - non-blocking with timeout
	select {
	case <-l.ctx.Done():
		return 0, l.ctx.Err()
	case l.writeChan <- publish.WriteRequest{Data: p, MsgType: websocket.TextMessage, IsControl: false}:
		return len(p), nil
	case <-time.After(DefaultWriteTimeout):
		log.E.F("ws->%s write channel timeout", l.remote)
		return 0, errorf.E("write channel timeout")
	}
}

// WriteControl sends a control message through the write channel
func (l *Listener) WriteControl(messageType int, data []byte, deadline time.Time) (err error) {
	// Defensive: recover from any panic when sending to closed channel
	defer func() {
		if r := recover(); r != nil {
			log.D.F("ws->%s writeControl panic recovered (channel likely closed): %v", l.remote, r)
			err = errorf.E("write channel closed")
		}
	}()

	select {
	case <-l.ctx.Done():
		return l.ctx.Err()
	case l.writeChan <- publish.WriteRequest{Data: data, MsgType: messageType, IsControl: true, Deadline: deadline}:
		return nil
	case <-time.After(DefaultWriteTimeout):
		log.E.F("ws->%s writeControl channel timeout", l.remote)
		return errorf.E("writeControl channel timeout")
	}
}

// writeWorker is the single goroutine that handles all writes to the websocket connection.
// This serializes all writes to prevent concurrent write panics and allows pings to interrupt writes.
func (l *Listener) writeWorker() {
	defer func() {
		// Only unregister write channel if connection is actually dead/closing
		// Unregister if:
		// 1. Context is cancelled (connection closing)
		// 2. Channel was closed (connection closing)
		// 3. Connection error occurred (already handled inline)
		if l.ctx.Err() != nil {
			// Connection is closing - safe to unregister
			if socketPub := l.publishers.GetSocketPublisher(); socketPub != nil {
				log.D.F("ws->%s write worker: unregistering write channel (connection closing)", l.remote)
				socketPub.SetWriteChan(l.conn, nil)
			}
		} else {
			// Exiting for other reasons (timeout, etc.) but connection may still be valid
			log.D.F("ws->%s write worker exiting unexpectedly", l.remote)
		}
		close(l.writeDone)
	}()

	for {
		select {
		case <-l.ctx.Done():
			log.D.F("ws->%s write worker context cancelled", l.remote)
			return
		case req, ok := <-l.writeChan:
			if !ok {
				log.D.F("ws->%s write channel closed", l.remote)
				return
			}

			// Skip writes if no connection (unit tests)
			if l.conn == nil {
				log.T.F("ws->%s skipping write (no connection)", l.remote)
				continue
			}

			// Handle the write request
			var err error
			if req.IsPing {
				// Special handling for ping messages
				log.D.F("sending PING #%d", req.MsgType)
				deadline := time.Now().Add(DefaultWriteTimeout)
				err = l.conn.WriteControl(websocket.PingMessage, nil, deadline)
				if err != nil {
					if !strings.HasSuffix(err.Error(), "use of closed network connection") {
						log.E.F("error writing ping: %v; closing websocket", err)
					}
					return
				}
			} else if req.IsControl {
				// Control message
				err = l.conn.WriteControl(req.MsgType, req.Data, req.Deadline)
				if err != nil {
					log.E.F("ws->%s control write failed: %v", l.remote, err)
					return
				}
			} else {
				// Regular message
				l.conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))
				err = l.conn.WriteMessage(req.MsgType, req.Data)
				if err != nil {
					log.E.F("ws->%s write failed: %v", l.remote, err)
					return
				}
			}
		}
	}
}

// messageProcessor is the goroutine that processes messages asynchronously.
// This prevents the websocket read loop from blocking on message processing.
func (l *Listener) messageProcessor() {
	defer func() {
		close(l.processingDone)
	}()

	for {
		select {
		case <-l.ctx.Done():
			log.D.F("ws->%s message processor context cancelled", l.remote)
			return
		case req, ok := <-l.messageQueue:
			if !ok {
				log.D.F("ws->%s message queue closed", l.remote)
				return
			}

			// Lock immediately to ensure AUTH is processed before subsequent messages
			// are dequeued. This prevents race conditions where EVENT checks authentication
			// before AUTH completes.
			l.authProcessing.Lock()

			// Check if this is an AUTH message by looking for the ["AUTH" prefix
			isAuthMessage := len(req.data) > 7 && bytes.HasPrefix(req.data, []byte(`["AUTH"`))

			if isAuthMessage {
				// Process AUTH message synchronously while holding lock
				// This blocks the messageProcessor from dequeuing the next message
				// until authentication is complete and authedPubkey is set
				log.D.F("ws->%s processing AUTH synchronously with lock", req.remote)
				l.HandleMessage(req.data, req.remote)
				// Unlock after AUTH completes so subsequent messages see updated authedPubkey
				l.authProcessing.Unlock()
			} else {
				// Not AUTH - unlock immediately and process concurrently
				// The next message can now be dequeued (possibly another non-AUTH to process concurrently)
				l.authProcessing.Unlock()

				// Acquire semaphore to limit concurrent handlers (blocking with context awareness)
				select {
				case l.handlerSem <- struct{}{}:
					// Semaphore acquired
				case <-l.ctx.Done():
					return
				}
				l.handlerWg.Add(1)
				go func(data []byte, remote string) {
					defer func() {
						<-l.handlerSem // Release semaphore
						l.handlerWg.Done()
					}()
					l.HandleMessage(data, remote)
				}(req.data, req.remote)
			}
		}
	}
}

// getManagedACL returns the managed ACL instance if available
func (l *Listener) getManagedACL() *database.ManagedACL {
	// Get the managed ACL instance from the ACL registry
	for _, aclInstance := range acl.Registry.ACL {
		if aclInstance.Type() == "managed" {
			if managed, ok := aclInstance.(*acl.Managed); ok {
				return managed.GetManagedACL()
			}
		}
	}
	return nil
}

// QueryEvents queries events using the database QueryEvents method
func (l *Listener) QueryEvents(ctx context.Context, f *filter.F) (event.S, error) {
	return l.DB.QueryEvents(ctx, f)
}

// QueryAllVersions queries events using the database QueryAllVersions method
func (l *Listener) QueryAllVersions(ctx context.Context, f *filter.F) (event.S, error) {
	return l.DB.QueryAllVersions(ctx, f)
}

// canSeePrivateEvent checks if the authenticated user can see an event with a private tag
func (l *Listener) canSeePrivateEvent(authedPubkey, privatePubkey []byte) (canSee bool) {
	// If no authenticated user, deny access
	if len(authedPubkey) == 0 {
		return false
	}

	// If the authenticated user matches the private tag pubkey, allow access
	if len(privatePubkey) > 0 && utils.FastEqual(authedPubkey, privatePubkey) {
		return true
	}

	// Check if user is an admin or owner (they can see all private events)
	accessLevel := acl.Registry.GetAccessLevel(authedPubkey, l.remote)
	if accessLevel == "admin" || accessLevel == "owner" {
		return true
	}

	// Default deny
	return false
}
