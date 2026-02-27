package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/envelopes/authenvelope"
	"next.orly.dev/pkg/nostr/encoders/hex"
	"next.orly.dev/pkg/protocol/publish"
	"next.orly.dev/pkg/nostr/utils/units"
)

const (
	DefaultWriteWait      = 10 * time.Second
	DefaultPongWait       = 60 * time.Second
	DefaultPingWait       = DefaultPongWait / 2
	DefaultWriteTimeout   = 3 * time.Second
	// DefaultMaxMessageSize is the maximum message size for WebSocket connections
	// Increased from 512KB to 10MB to support large kind 3 follow lists (10k+ follows)
	// and other large events without truncation
	DefaultMaxMessageSize = 10 * 1024 * 1024 // 10MB
	// ClientMessageSizeLimit is the maximum message size that clients can handle
	// This is set to 100MB to allow large messages
	ClientMessageSizeLimit = 100 * 1024 * 1024 // 100MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for proxy compatibility
	},
}

func (s *Server) HandleWebsocket(w http.ResponseWriter, r *http.Request) {
	remote := GetRemoteFromReq(r)

	// Log comprehensive proxy information for debugging
	LogProxyInfo(r, "WebSocket connection from "+remote)
	if len(s.Config.IPWhitelist) > 0 {
		for _, ip := range s.Config.IPWhitelist {
			log.T.F("checking IP whitelist: %s", ip)
			if strings.HasPrefix(remote, ip) {
				log.T.F("IP whitelisted %s", remote)
				goto whitelist
			}
		}
		log.T.F("IP not whitelisted: %s", remote)
		return
	}
whitelist:
	// Extract IP from remote (strip port)
	ip := remote
	if idx := strings.LastIndex(remote, ":"); idx != -1 {
		ip = remote[:idx]
	}

	// Check per-IP connection limit (hard limit 10)
	maxConnPerIP := s.Config.MaxConnectionsPerIP
	if maxConnPerIP <= 0 {
		maxConnPerIP = 10
	}
	if maxConnPerIP > 10 {
		maxConnPerIP = 10 // Hard limit
	}

	s.connPerIPMu.Lock()
	currentConns := s.connPerIP[ip]
	if currentConns >= maxConnPerIP {
		s.connPerIPMu.Unlock()
		log.W.F("connection limit exceeded for IP %s: %d/%d connections", ip, currentConns, maxConnPerIP)
		http.Error(w, "too many connections from your IP", http.StatusTooManyRequests)
		return
	}
	s.connPerIP[ip]++
	s.connPerIPMu.Unlock()

	// Track global connection count
	s.activeConnCount.Add(1)

	// Decrement connection counts when this function returns
	defer func() {
		s.activeConnCount.Add(-1)
		s.connPerIPMu.Lock()
		s.connPerIP[ip]--
		if s.connPerIP[ip] <= 0 {
			delete(s.connPerIP, ip)
		}
		s.connPerIPMu.Unlock()
	}()

	// Global adaptive load check — refuse or delay connections under load
	if s.rateLimiter != nil && s.rateLimiter.IsEnabled() {
		s.rateLimiter.SetActiveConnections(s.activeConnCount.Load())

		if !s.rateLimiter.ShouldAcceptConnection() {
			log.W.F("refusing connection from %s: system overloaded", ip)
			http.Error(w, "server overloaded, try later", http.StatusServiceUnavailable)
			return
		}

		if delay := s.rateLimiter.ConnectionDelay(); delay > 0 {
			log.D.F("delaying connection from %s by %v (load mitigation)", ip, delay)
			time.Sleep(delay)
		}
	}

	// Progressive per-IP delay: each additional connection from the same IP adds delay
	if currentConns > 0 {
		perIPDelay := time.Duration(currentConns) * 200 * time.Millisecond
		if perIPDelay > 2*time.Second {
			perIPDelay = 2 * time.Second
		}
		log.D.F("per-IP delay for %s: %v (%d connections)", ip, perIPDelay, currentConns+1)
		time.Sleep(perIPDelay)
	}

	// Create an independent context for this connection
	// This context will be cancelled when the connection closes or server shuts down
	ctx, cancel := context.WithCancel(s.Ctx)
	defer cancel()
	var err error
	var conn *websocket.Conn

	// Configure upgrader for this connection
	// Use reasonable buffer sizes (64KB) instead of max message size (10MB)
	// to prevent memory exhaustion with many connections
	upgrader.ReadBufferSize = 64 * 1024  // 64KB
	upgrader.WriteBufferSize = 64 * 1024 // 64KB

	if conn, err = upgrader.Upgrade(w, r, nil); chk.E(err) {
		log.E.F("websocket accept failed from %s: %v", remote, err)
		return
	}
	log.T.F("websocket accepted from %s path=%s", remote, r.URL.String())

	// Set read limit immediately after connection is established
	conn.SetReadLimit(DefaultMaxMessageSize)
	log.D.F("set read limit to %d bytes (%d MB) for %s", DefaultMaxMessageSize, DefaultMaxMessageSize/units.Mb, remote)

	// Set initial read deadline - pong handler will extend it when pongs are received
	conn.SetReadDeadline(time.Now().Add(DefaultPongWait))

	// Add pong handler to extend read deadline when client responds to pings
	conn.SetPongHandler(func(string) error {
		log.T.F("received PONG from %s, extending read deadline", remote)
		return conn.SetReadDeadline(time.Now().Add(DefaultPongWait))
	})

	defer conn.Close()
	// Determine handler semaphore size from config
	handlerSemSize := s.Config.MaxHandlersPerConnection
	if handlerSemSize <= 0 {
		handlerSemSize = 100 // Default if not configured
	}

	now := time.Now()
	listener := &Listener{
		ctx:            ctx,
		cancel:         cancel,
		Server:         s,
		conn:           conn,
		remote:         remote,
		connectionID:   fmt.Sprintf("%s-%d", remote, now.UnixNano()), // Unique connection ID for access tracking
		req:            r,
		startTime:      now,
		writeChan:      make(chan publish.WriteRequest, 100), // Buffered channel for writes
		writeDone:      make(chan struct{}),
		messageQueue:   make(chan messageRequest, 100), // Buffered channel for message processing
		processingDone: make(chan struct{}),
		handlerSem:     make(chan struct{}, handlerSemSize), // Limits concurrent handlers
		subscriptions:  make(map[string]context.CancelFunc),
	}

	// Start write worker goroutine
	go listener.writeWorker()

	// Start message processor goroutine
	go listener.messageProcessor()

	// Register write channel with publisher
	if socketPub := listener.publishers.GetSocketPublisher(); socketPub != nil {
		socketPub.SetWriteChan(conn, listener.writeChan)
	}

	// Check for blacklisted IPs
	listener.isBlacklisted = s.isIPBlacklisted(remote)
	if listener.isBlacklisted {
		log.W.F("detected blacklisted IP %s, marking connection for timeout", remote)
		listener.blacklistTimeout = time.Now().Add(time.Minute) // Timeout after 1 minute
	}
	chal := make([]byte, 32)
	rand.Read(chal)
	listener.challenge.Store([]byte(hex.Enc(chal)))
	// Send AUTH challenge if ACL mode requires it, or if auth is required/required for writes
	if s.Config.ACLMode != "none" || s.Config.AuthRequired || s.Config.AuthToWrite {
		log.D.F("sending AUTH challenge to %s", remote)
		if err = authenvelope.NewChallengeWith(listener.challenge.Load()).
			Write(listener); chk.E(err) {
			log.E.F("failed to send AUTH challenge to %s: %v", remote, err)
			return
		}
		log.D.F("AUTH challenge sent successfully to %s", remote)
	}
	ticker := time.NewTicker(DefaultPingWait)
	// Don't pass cancel to Pinger - it should not be able to cancel the connection context
	go s.Pinger(ctx, listener, ticker)
	defer func() {
		log.D.F("closing websocket connection from %s", remote)

		// Cancel all active subscriptions first
		listener.subscriptionsMu.Lock()
		for subID, cancelFunc := range listener.subscriptions {
			log.D.F("cancelling subscription %s for %s", subID, remote)
			cancelFunc()
		}
		listener.subscriptions = nil
		listener.subscriptionsMu.Unlock()

		// Cancel context and stop pinger
		cancel()
		ticker.Stop()

		// Cancel all subscriptions for this connection at publisher level
		log.D.F("removing subscriptions from publisher for %s", remote)
		listener.publishers.Receive(&W{
			Cancel: true,
			Conn:   listener.conn,
			remote: listener.remote,
		})

		// Log detailed connection statistics
		dur := time.Since(listener.startTime)
		log.D.F(
			"ws connection closed %s: msgs=%d, REQs=%d, EVENTs=%d, dropped=%d, duration=%v",
			remote, listener.msgCount, listener.reqCount, listener.eventCount,
			listener.DroppedMessages(), dur,
		)

		// Log any remaining connection state
		if listener.authedPubkey.Load() != nil {
			log.D.F("ws connection %s was authenticated", remote)
		} else {
			log.D.F("ws connection %s was not authenticated", remote)
		}

		// Close message queue to signal processor to exit
		close(listener.messageQueue)
		// Wait for message processor to finish
		<-listener.processingDone

		// Wait for all spawned message handlers to complete
		// This is critical to prevent "send on closed channel" panics
		log.D.F("ws->%s waiting for message handlers to complete", remote)
		listener.handlerWg.Wait()
		log.D.F("ws->%s all message handlers completed", remote)

		// Close write channel to signal worker to exit
		close(listener.writeChan)
		// Wait for write worker to finish
		<-listener.writeDone
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if blacklisted connection has timed out
		if listener.isBlacklisted && time.Now().After(listener.blacklistTimeout) {
			log.W.F("blacklisted IP %s timeout reached, closing connection", remote)
			return
		}

		var typ int
		var msg []byte
		log.T.F("waiting for message from %s", remote)

		// Don't set read deadline here - it's set initially and extended by pong handler
		// This prevents premature timeouts on idle connections with active subscriptions
		if ctx.Err() != nil {
			return
		}

		// Block waiting for message; rely on pings and context cancellation to detect dead peers
		// The read deadline is managed by the pong handler which extends it when pongs are received
		typ, msg, err = conn.ReadMessage()

		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseNormalClosure,    // 1000
				websocket.CloseGoingAway,        // 1001
				websocket.CloseNoStatusReceived, // 1005
				websocket.CloseAbnormalClosure,  // 1006
				4537,                            // some client seems to send many of these
			) {
				log.I.F("websocket connection closed from %s: %v", remote, err)
			}
			cancel() // Cancel context like khatru does
			return
		}
		if typ == websocket.PingMessage {
			log.D.F("received PING from %s, sending PONG", remote)
			// Send pong directly (like khatru does)
			if err = conn.WriteMessage(websocket.PongMessage, nil); err != nil {
				log.E.F("failed to send PONG to %s: %v", remote, err)
				return
			}
			continue
		}
		// Log message size for debugging
		if len(msg) > 1000 { // Only log for larger messages
			log.D.F("received large message from %s: %d bytes", remote, len(msg))
		}
		// log.T.F("received message from %s: %s", remote, string(msg))

		// Queue message for asynchronous processing
		if !listener.QueueMessage(msg, remote) {
			log.W.F("ws->%s message queue full, dropping message (capacity=%d)", remote, cap(listener.messageQueue))
		}
	}
}

func (s *Server) Pinger(
	ctx context.Context, listener *Listener, ticker *time.Ticker,
) {
	defer func() {
		log.D.F("pinger shutting down")
		ticker.Stop()
		// Recover from panic if channel is closed
		if r := recover(); r != nil {
			log.D.F("pinger recovered from panic (channel likely closed): %v", r)
		}
	}()
	pingCount := 0
	for {
		select {
		case <-ctx.Done():
			log.T.F("pinger context cancelled after %d pings", pingCount)
			return
		case <-ticker.C:
			pingCount++
			// Send ping request through write channel - this allows pings to interrupt other writes
			select {
			case <-ctx.Done():
				return
			case listener.writeChan <- publish.WriteRequest{IsPing: true, MsgType: pingCount}:
				// Ping request queued successfully
			case <-time.After(DefaultWriteTimeout):
				log.E.F("ping #%d channel timeout - connection may be overloaded", pingCount)
				return
			}
		}
	}
}

