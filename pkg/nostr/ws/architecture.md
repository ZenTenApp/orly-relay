# WebSocket Write Multiplexing Architecture

This document explains how ORLY handles concurrent writes to WebSocket connections safely and efficiently.

## Overview

ORLY uses a **single-writer pattern** with channel-based coordination to multiplex writes from multiple goroutines to each WebSocket connection. This prevents concurrent write panics and ensures message ordering.

### Key Design Principle

**Each WebSocket connection has exactly ONE dedicated writer goroutine, but MANY producer goroutines can safely queue messages through a buffered channel.** This is the standard Go solution for the "multiple producers, single consumer" concurrency pattern.

### Why This Matters

The gorilla/websocket library (and WebSockets in general) don't allow concurrent writes - attempting to write from multiple goroutines causes panics. ORLY's channel-based approach elegantly serializes all writes while maintaining high throughput.

## Architecture Components

### 1. Per-Connection Write Channel

Each `Listener` (WebSocket connection) has a dedicated write channel defined in [`app/listener.go:35`](../../app/listener.go#L35):

```go
type Listener struct {
    writeChan  chan publish.WriteRequest  // Buffered channel (capacity: 100)
    writeDone  chan struct{}              // Signals writer exit
    // ... other fields
}
```

Created during connection setup in [`app/handle-websocket.go:94`](../../app/handle-websocket.go#L94):

```go
listener := &Listener{
    writeChan:  make(chan publish.WriteRequest, 100),
    writeDone:  make(chan struct{}),
    // ...
}
```

### 2. Single Write Worker Goroutine

The `writeWorker()` defined in [`app/listener.go:133-201`](../../app/listener.go#L133-L201) is the **ONLY** goroutine allowed to call `conn.WriteMessage()`:

```go
func (l *Listener) writeWorker() {
    defer close(l.writeDone)

    for {
        select {
        case <-l.ctx.Done():
            return
        case req, ok := <-l.writeChan:
            if !ok {
                return // Channel closed
            }

            if req.IsPing {
                // Send ping control frame
                l.conn.WriteControl(websocket.PingMessage, nil, deadline)
            } else if req.IsControl {
                // Send control message
                l.conn.WriteControl(req.MsgType, req.Data, req.Deadline)
            } else {
                // Send regular message
                l.conn.SetWriteDeadline(time.Now().Add(DefaultWriteTimeout))
                l.conn.WriteMessage(req.MsgType, req.Data)
            }
        }
    }
}
```

Started once per connection in [`app/handle-websocket.go:102`](../../app/handle-websocket.go#L102):

```go
go listener.writeWorker()
```

### 3. Write Request Structure

All write operations are wrapped in a `WriteRequest` defined in [`pkg/protocol/publish/publisher.go:13-19`](../protocol/publish/publisher.go#L13-L19):

```go
type WriteRequest struct {
    Data      []byte
    MsgType   int        // websocket.TextMessage, PingMessage, etc.
    IsControl bool       // Control frame?
    Deadline  time.Time  // For control messages
    IsPing    bool       // Special ping handling
}
```

### 4. Multiple Write Producers

Several goroutines send write requests to the channel:

#### A. Listener.Write() - Main Write Interface

Used by protocol handlers (EVENT, REQ, COUNT, etc.) in [`app/listener.go:88-108`](../../app/listener.go#L88-L108):

```go
func (l *Listener) Write(p []byte) (n int, err error) {
    select {
    case l.writeChan <- publish.WriteRequest{
        Data: p,
        MsgType: websocket.TextMessage,
    }:
        return len(p), nil
    case <-time.After(DefaultWriteTimeout):
        return 0, errorf.E("write channel timeout")
    }
}
```

#### B. Subscription Goroutines

Each active subscription runs a goroutine that receives events from the publisher and forwards them in [`app/handle-req.go:696-736`](../../app/handle-req.go#L696-L736):

```go
// Subscription goroutine (one per REQ)
go func() {
    for {
        select {
        case ev := <-evC:  // Receive from publisher
            res := eventenvelope.NewFrom(subID, ev)
            if err = res.Write(l); err != nil {  // ← Sends to writeChan
                log.E.F("failed to write event")
            }
        }
    }
}()
```

#### C. Pinger Goroutine

Sends periodic pings in [`app/handle-websocket.go:252-283`](../../app/handle-websocket.go#L252-L283):

```go
func (s *Server) Pinger(ctx context.Context, listener *Listener, ticker *time.Ticker) {
    for {
        select {
        case <-ticker.C:
            // Send ping with special flag
            listener.writeChan <- publish.WriteRequest{
                IsPing:  true,
                MsgType: pingCount,
            }
        }
    }
}
```

## Message Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    WebSocket Connection                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
         ┌────────────────────────────────────────┐
         │          Listener (per conn)           │
         │  writeChan: chan WriteRequest (100)    │
         └────────────────────────────────────────┘
                      ▲   ▲   ▲   ▲
                      │   │   │   │
        ┌─────────────┼───┼───┼───┼─────────────┐
        │ PRODUCERS (Multiple Goroutines)         │
        ├─────────────────────────────────────────┤
        │ 1. Handler goroutine                    │
        │    └─> Write(okMsg) ───────────────┐    │
        │                                     │    │
        │ 2. Subscription goroutine (REQ1)   │    │
        │    └─> Write(event1) ──────────────┼──┐ │
        │                                     │  │ │
        │ 3. Subscription goroutine (REQ2)   │  │ │
        │    └─> Write(event2) ──────────────┼──┼─┤
        │                                     │  │ │
        │ 4. Pinger goroutine                 │  │ │
        │    └─> writeChan <- PING ──────────┼──┼─┼┐
        └─────────────────────────────────────┼──┼─┼┤
                                              ▼  ▼ ▼▼
                        ┌──────────────────────────────┐
                        │   writeChan (buffered)       │
                        │   [req1][req2][ping][req3]   │
                        └──────────────────────────────┘
                                      │
                                      ▼
        ┌─────────────────────────────────────────────┐
        │ CONSUMER (Single Writer Goroutine)          │
        ├─────────────────────────────────────────────┤
        │  writeWorker() ─── ONLY goroutine allowed   │
        │                    to call WriteMessage()   │
        └─────────────────────────────────────────────┘
                                      │
                                      ▼
                        conn.WriteMessage(msgType, data)
                                      │
                                      ▼
                            ┌─────────────────┐
                            │  Client Browser │
                            └─────────────────┘
```

## Publisher Integration

The publisher system also uses the write channel map defined in [`app/publisher.go:25-26`](../../app/publisher.go#L25-L26):

```go
type WriteChanMap map[*websocket.Conn]chan publish.WriteRequest

type P struct {
    WriteChans WriteChanMap  // Maps conn → write channel
    // ...
}
```

### Event Publication Flow

When an event is published (see [`app/publisher.go:153-268`](../../app/publisher.go#L153-L268)):

1. Publisher finds matching subscriptions
2. For each match, sends event to subscription's receiver channel
3. Subscription goroutine receives event
4. Subscription calls `Write(l)` which enqueues to `writeChan`
5. Write worker dequeues and writes to WebSocket

### Two-Level Queue System

ORLY uses **TWO** channel layers:

1. **Receiver channels** (subscription → handler) for event delivery
2. **Write channels** (handler → WebSocket) for actual I/O

This separation provides:

- **Subscription-level backpressure**: Slow subscribers don't block event processing
- **Connection-level serialization**: All writes to a single WebSocket are ordered
- **Independent lifetimes**: Subscriptions can be cancelled without closing the connection

This architecture matches patterns used in production relays like [khatru](https://github.com/fiatjaf/khatru) and enables ORLY to handle thousands of concurrent subscriptions efficiently.

## Key Features

### 1. Thread-Safe Concurrent Writes

Multiple goroutines can safely queue messages without any mutexes - the channel provides synchronization.

### 2. Backpressure Handling

Writes use a timeout (see [`app/listener.go:104`](../../app/listener.go#L104)):

```go
case <-time.After(DefaultWriteTimeout):
    return 0, errorf.E("write channel timeout")
```

If the channel is full (100 messages buffered), writes timeout rather than blocking indefinitely.

### 3. Graceful Shutdown

Connection cleanup in [`app/handle-websocket.go:184-187`](../../app/handle-websocket.go#L184-L187):

```go
// Close write channel to signal worker to exit
close(listener.writeChan)
// Wait for write worker to finish
<-listener.writeDone
```

Ensures all queued messages are sent before closing the connection.

### 4. Ping Priority

Pings use a special `IsPing` flag so the write worker can prioritize them during heavy traffic, preventing timeout disconnections.

## Configuration Constants

Defined in [`app/handle-websocket.go:19-28`](../../app/handle-websocket.go#L19-L28):

```go
const (
    DefaultWriteWait      = 10 * time.Second  // Write deadline for normal messages
    DefaultPongWait       = 60 * time.Second  // Time to wait for pong response
    DefaultPingWait       = 30 * time.Second  // Interval between pings
    DefaultWriteTimeout   = 3 * time.Second   // Timeout for write channel send
    DefaultMaxMessageSize = 512000            // Max incoming message size (512KB)
    ClientMessageSizeLimit = 100 * 1024 * 1024 // Max client message size (100MB)
)
```

## Benefits of This Design

✅ **No concurrent write panics** - single writer guarantee
✅ **High throughput** - buffered channel (100 messages)
✅ **Fair ordering** - FIFO queue semantics
✅ **Simple producer code** - just send to channel
✅ **Backpressure management** - timeout on full queue
✅ **Clean shutdown** - channel close signals completion
✅ **Priority handling** - pings can be prioritized

## Performance Characteristics

- **Channel buffer size**: 100 messages per connection
- **Write timeout**: 3 seconds before declaring channel blocked
- **Ping interval**: 30 seconds to keep connections alive
- **Pong timeout**: 60 seconds before considering connection dead

This pattern is the standard Go idiom for serializing operations and is used throughout high-performance network services.

## Related Documentation

- [Nostr Protocol Implementation](../protocol/README.md)
- [Publisher System](../protocol/publish/README.md)
- [Event Handling](../../app/handle-websocket.go)
- [Subscription Management](../../app/handle-req.go)
