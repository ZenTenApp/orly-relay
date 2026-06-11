package app

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"git.smesh.lol/orly/pkg/lol/errorf"
	"git.smesh.lol/orly/pkg/lol/log"
	"git.smesh.lol/orly/app/config"
	"git.smesh.lol/orly/pkg/acl"
	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/filter"
	"git.smesh.lol/orly/pkg/protocol/publish"
	"git.smesh.lol/orly/pkg/utils"
	atomicutils "git.smesh.lol/orly/pkg/utils/atomic"
)

// -- subscription actor request types --

type subSetReq struct {
	id     string
	cancel context.CancelFunc
}
type subDeleteReq struct {
	id string
}
type subCancelAllReq struct {
	resp chan struct{} // buffered 1: ack
}
type subGetReq struct {
	id   string
	resp chan subGetResp // buffered 1
}
type subGetResp struct {
	cancel context.CancelFunc
	exists bool
}

// -- handler tracking types --

type handlerTrackReq struct{}
type handlerDoneReq struct{}
type handlerWaitReq struct {
	resp chan struct{} // buffered 1: ack when count==0
}

type Listener struct {
	// Server is the embedded server reference.
	*Server
	conn *websocket.Conn
	ctx              context.Context
	cancel           context.CancelFunc
	remote           string
	connectionID     string
	req              *http.Request
	challenge        atomicutils.Bytes
	authedPubkey     atomicutils.Bytes
	startTime        time.Time
	isBlacklisted    bool
	blacklistTimeout time.Time
	writeChan        chan publish.WriteRequest
	writeDone        chan struct{}
	// Message processing queue for async handling
	messageQueue     chan messageRequest
	processingDone   chan struct{}

	// Handler tracking via channels (replaces sync.WaitGroup)
	handlerTrackCh   chan handlerTrackReq
	handlerDoneCh    chan handlerDoneReq
	handlerWaitCh    chan handlerWaitReq
	handlerActorDone chan struct{}

	handlerSem       chan struct{}       // Limits concurrent message handlers per connection

	// Auth gate channel (replaces sync.RWMutex)
	// When auth is processing, this channel is drained (locked).
	// Non-auth messages wait to read from authGate before proceeding.
	authGate         chan struct{} // buffered 1, starts with one token

	// Subscription actor channels (replaces sync.Mutex + map)
	subSetCh         chan subSetReq
	subDeleteCh      chan subDeleteReq
	subCancelAllCh   chan subCancelAllReq
	subGetCh         chan subGetReq
	subActorDone     chan struct{}

	// Flow control counters (atomic for concurrent access)
	droppedMessages      atomic.Int64
	queryCostAccumulator atomic.Int64
	// Diagnostics: per-connection counters
	msgCount   int
	reqCount   int
	eventCount int
}

type messageRequest struct {
	data   []byte
	remote string
}

func (l *Listener) initActors() {
	// Handler tracking actor
	l.handlerTrackCh = make(chan handlerTrackReq, 128)
	l.handlerDoneCh = make(chan handlerDoneReq, 128)
	l.handlerWaitCh = make(chan handlerWaitReq)
	l.handlerActorDone = make(chan struct{})
	go l.handlerTrackingActor()

	// Auth gate: starts unlocked (one token available)
	l.authGate = make(chan struct{}, 1)
	l.authGate <- struct{}{}

	// Subscription actor
	l.subSetCh = make(chan subSetReq, 16)
	l.subDeleteCh = make(chan subDeleteReq, 16)
	l.subCancelAllCh = make(chan subCancelAllReq)
	l.subGetCh = make(chan subGetReq)
	l.subActorDone = make(chan struct{})
	go l.subscriptionActor()
}

func (l *Listener) handlerTrackingActor() {
	defer close(l.handlerActorDone)
	count := 0
	var waiters []chan struct{}
	for {
		select {
		case <-l.handlerTrackCh:
			count++
		case <-l.handlerDoneCh:
			count--
			if count == 0 && len(waiters) > 0 {
				for _, w := range waiters {
					close(w)
				}
				waiters = nil
			}
		case req := <-l.handlerWaitCh:
			if count == 0 {
				close(req.resp)
			} else {
				waiters = append(waiters, req.resp)
			}
		case <-l.ctx.Done():
			// drain remaining
			for {
				select {
				case <-l.handlerDoneCh:
					count--
				default:
					goto drained
				}
			}
		drained:
			for _, w := range waiters {
				close(w)
			}
			return
		}
	}
}

func (l *Listener) subscriptionActor() {
	defer close(l.subActorDone)
	subs := make(map[string]context.CancelFunc)
	for {
		select {
		case req := <-l.subSetCh:
			subs[req.id] = req.cancel
		case req := <-l.subDeleteCh:
			if cancelFunc, exists := subs[req.id]; exists {
				cancelFunc()
				delete(subs, req.id)
			}
		case req := <-l.subCancelAllCh:
			for _, cancelFunc := range subs {
				cancelFunc()
			}
			subs = make(map[string]context.CancelFunc)
			close(req.resp)
		case req := <-l.subGetCh:
			cancelFunc, exists := subs[req.id]
			req.resp <- subGetResp{cancel: cancelFunc, exists: exists}
		case <-l.ctx.Done():
			for _, cancelFunc := range subs {
				cancelFunc()
			}
			return
		}
	}
}

// HandlerAdd increments the handler count (replaces handlerWg.Add(1))
func (l *Listener) HandlerAdd() {
	l.handlerTrackCh <- handlerTrackReq{}
}

// HandlerDone decrements the handler count (replaces handlerWg.Done())
func (l *Listener) HandlerDone() {
	l.handlerDoneCh <- handlerDoneReq{}
}

// HandlerWait blocks until all handlers complete (replaces handlerWg.Wait())
func (l *Listener) HandlerWait() {
	resp := make(chan struct{}, 1)
	l.handlerWaitCh <- handlerWaitReq{resp: resp}
	<-resp
}

// SubSet stores a subscription cancel function
func (l *Listener) SubSet(id string, cancel context.CancelFunc) {
	l.subSetCh <- subSetReq{id: id, cancel: cancel}
}

// SubDelete cancels and removes a subscription
func (l *Listener) SubDelete(id string) {
	l.subDeleteCh <- subDeleteReq{id: id}
}

// SubCancelAll cancels all subscriptions and waits for ack
func (l *Listener) SubCancelAll() {
	resp := make(chan struct{}, 1)
	l.subCancelAllCh <- subCancelAllReq{resp: resp}
	<-resp
}

// SubGet retrieves a subscription's cancel function
func (l *Listener) SubGet(id string) (context.CancelFunc, bool) {
	resp := make(chan subGetResp, 1)
	l.subGetCh <- subGetReq{id: id, resp: resp}
	r := <-resp
	return r.cancel, r.exists
}

// AuthLock acquires the auth gate exclusively (for AUTH processing)
func (l *Listener) AuthLock() {
	<-l.authGate
}

// AuthUnlock releases the auth gate
func (l *Listener) AuthUnlock() {
	l.authGate <- struct{}{}
}

// AuthRLock waits for auth to not be processing, then returns immediately
// This is a read-side check - just verify the token is there and put it back
func (l *Listener) AuthRLock() {
	// Not needed for the current usage pattern where auth is synchronous
	// and non-auth messages just need to wait for auth to complete
}

// AuthRUnlock is a no-op for the read side
func (l *Listener) AuthRUnlock() {
}

// Ctx returns the listener's context
func (l *Listener) Ctx() context.Context {
	return l.ctx
}

// ServerContext returns the server's context
func (l *Listener) ServerContext() context.Context {
	return l.Server.Context()
}

// ServerConfig returns the server's configuration
func (l *Listener) ServerConfig() *config.C {
	return l.Server.GetConfig()
}

// ServerDatabase returns the server's database instance
func (l *Listener) ServerDatabase() database.Database {
	return l.Server.Database()
}

// DroppedMessages returns the total number of messages that were dropped
func (l *Listener) DroppedMessages() int {
	return int(l.droppedMessages.Load())
}

// RemainingCapacity returns the number of slots available in the message processing queue
func (l *Listener) RemainingCapacity() int {
	return cap(l.messageQueue) - len(l.messageQueue)
}

// QueueMessage queues a message for asynchronous processing.
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
	if !utf8.Valid(p) {
		log.E.F("ws->%s dropping message with invalid UTF-8 (%d bytes)", l.remote, len(p))
		return 0, errorf.E("invalid UTF-8")
	}
	defer func() {
		if r := recover(); r != nil {
			log.D.F("ws->%s write panic recovered (channel likely closed): %v", l.remote, r)
			err = errorf.E("write channel closed")
			n = 0
		}
	}()

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

// SendEvent sends an event to the client.
func (l *Listener) SendEvent(ev *event.E) error {
	if ev == nil {
		return nil
	}
	data := ev.Serialize()
	_, err := l.Write(data)
	return err
}

// IsConnected returns whether the client connection is still active.
func (l *Listener) IsConnected() bool {
	select {
	case <-l.ctx.Done():
		return false
	default:
		return l.conn != nil
	}
}

// WriteControl sends a control message through the write channel
func (l *Listener) WriteControl(messageType int, data []byte, deadline time.Time) (err error) {
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
func (l *Listener) writeWorker() {
	defer func() {
		if l.ctx.Err() != nil {
			if socketPub := l.publishers.GetSocketPublisher(); socketPub != nil {
				log.D.F("ws->%s write worker: unregistering write channel (connection closing)", l.remote)
				socketPub.SetWriteChan(l.conn, nil)
			}
		} else {
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

			if l.conn == nil {
				log.T.F("ws->%s skipping write (no connection)", l.remote)
				continue
			}

			var err error
			if req.IsPing {
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
				err = l.conn.WriteControl(req.MsgType, req.Data, req.Deadline)
				if err != nil {
					log.E.F("ws->%s control write failed: %v", l.remote, err)
					return
				}
			} else {
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

			// Acquire the auth gate token to ensure AUTH is processed before subsequent messages
			l.AuthLock()

			isAuthMessage := len(req.data) > 7 && bytes.HasPrefix(req.data, []byte(`["AUTH"`))

			if isAuthMessage {
				log.D.F("ws->%s processing AUTH synchronously with lock", req.remote)
				l.HandleMessage(req.data, req.remote)
				l.AuthUnlock()
			} else {
				l.AuthUnlock()

				// Acquire semaphore to limit concurrent handlers
				select {
				case l.handlerSem <- struct{}{}:
				case <-l.ctx.Done():
					return
				}
				l.HandlerAdd()
				go func(data []byte, remote string) {
					defer func() {
						<-l.handlerSem
						l.HandlerDone()
					}()
					l.HandleMessage(data, remote)
				}(req.data, req.remote)
			}
		}
	}
}

// getManagedACL returns the managed ACL instance if available
func (l *Listener) getManagedACL() *database.ManagedACL {
	for _, aclInstance := range acl.Registry.ACLs() {
		if aclInstance.Type() == "managed" {
			if managed, ok := aclInstance.(*acl.Managed); ok {
				return managed.GetManagedACL()
			}
		}
	}
	return nil
}

// getFollowsThrottleDelay returns the progressive throttle delay for follows or social ACL mode.
func (l *Listener) getFollowsThrottleDelay(ev *event.E) time.Duration {
	mode := acl.Registry.GetMode()
	switch mode {
	case "follows":
		for _, aclInstance := range acl.Registry.ACLs() {
			if follows, ok := aclInstance.(*acl.Follows); ok {
				return follows.GetThrottleDelay(ev.Pubkey, l.remote)
			}
		}
	case "social":
		for _, aclInstance := range acl.Registry.ACLs() {
			if social, ok := aclInstance.(*acl.Social); ok {
				return social.GetThrottleDelay(ev.Pubkey, l.remote)
			}
		}
	}
	return 0
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
	if len(authedPubkey) == 0 {
		return false
	}
	if len(privatePubkey) > 0 && utils.FastEqual(authedPubkey, privatePubkey) {
		return true
	}
	accessLevel := acl.Registry.GetAccessLevel(authedPubkey, l.remote)
	if accessLevel == "admin" || accessLevel == "owner" {
		return true
	}
	return false
}
