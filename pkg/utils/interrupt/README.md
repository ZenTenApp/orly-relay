# interrupt

Graceful shutdown handling for Go applications. This package provides utilities for handling OS signals (SIGINT, SIGTERM) to enable clean shutdowns and hot reloading capabilities.

## Features

- **Signal Handling**: Clean handling of SIGINT, SIGTERM, and SIGHUP signals
- **Graceful Shutdown**: Allows running goroutines to finish before exit
- **Hot Reload Support**: Trigger application reloads on SIGHUP
- **Context Integration**: Works seamlessly with Go's context package
- **Custom Callbacks**: Execute custom cleanup logic during shutdown
- **Non-blocking**: Doesn't block the main application loop

## Installation

```bash
go get git.smesh.lol/orly/pkg/utils/interrupt
```

## Usage

### Basic Shutdown Handling

```go
package main

import (
    "context"
    "log"
    "time"

    "git.smesh.lol/orly/pkg/utils/interrupt"
)

func main() {
    // Create interrupt handler
    handler := interrupt.New()

    // Start your application
    go func() {
        for {
            select {
            case <-handler.Shutdown():
                log.Println("Shutting down worker...")
                return
            default:
                // Do work
                time.Sleep(time.Second)
            }
        }
    }()

    // Wait for shutdown signal
    <-handler.Done()
    log.Println("Application stopped")
}
```

### Context Integration

```go
func worker(ctx context.Context) {
    handler := interrupt.New()

    // Create context that cancels on shutdown
    workCtx, cancel := context.WithCancel(ctx)
    defer cancel()

    go func() {
        <-handler.Shutdown()
        cancel()
    }()

    // Use workCtx for all operations
    for {
        select {
        case <-workCtx.Done():
            return
        default:
            // Do work with context
        }
    }
}
```

### Custom Shutdown Callbacks

```go
handler := interrupt.New()

// Add cleanup callbacks
handler.OnShutdown(func() {
    log.Println("Closing database connections...")
    db.Close()
})

handler.OnShutdown(func() {
    log.Println("Saving application state...")
    saveState()
})

// Callbacks execute in reverse order when shutdown occurs
<-handler.Done()
```

### Hot Reload Support

```go
handler := interrupt.New()

// Handle reload signals
go func() {
    for {
        select {
        case <-handler.Reload():
            log.Println("Reloading configuration...")
            reloadConfig()
        case <-handler.Shutdown():
            return
        }
    }
}()

<-handler.Done()
```

## API Reference

### Handler

The main interrupt handler type.

**Methods:**
- `New() *Handler` - Create a new interrupt handler
- `Shutdown() <-chan struct{}` - Channel closed on shutdown signals
- `Reload() <-chan struct{}` - Channel closed on reload signals (SIGHUP)
- `Done() <-chan struct{}` - Channel closed when all cleanup is complete
- `OnShutdown(func())` - Add a callback to run during shutdown
- `Wait()` - Block until shutdown signal received
- `IsShuttingDown() bool` - Check if shutdown is in progress

### Signal Handling

The package handles these signals:

- **SIGINT**: Interrupt (Ctrl+C) - Triggers graceful shutdown
- **SIGTERM**: Termination - Triggers graceful shutdown
- **SIGHUP**: Hangup - Triggers reload (can be customized)

### Shutdown Process

1. Signal received (SIGINT/SIGTERM)
2. Shutdown callbacks execute (in reverse order added)
3. Shutdown channel closes
4. Application can perform final cleanup
5. Done channel closes

## Testing

The interrupt package includes comprehensive tests:

### Running Tests

```bash
# Run interrupt package tests
go test ./pkg/utils/interrupt

# Run with verbose output
go test -v ./pkg/utils/interrupt

# Run with race detection
go test -race ./pkg/utils/interrupt
```

### Integration Testing

Part of the full test suite:

```bash
# Run all tests including interrupt
./scripts/test.sh

# Run specific package tests
go test ./pkg/utils/...
```

### Test Coverage

Tests cover:
- Signal handling for all supported signals
- Callback execution order and timing
- Context cancellation
- Concurrent access patterns
- Race condition prevention

### Example Test

```bash
# Test signal handling
go test -v ./pkg/utils/interrupt -run TestSignalHandling

# Test callback execution
go test -v ./pkg/utils/interrupt -run TestShutdownCallbacks
```

## Examples

### HTTP Server with Graceful Shutdown

```go
package main

import (
    "context"
    "log"
    "net/http"
    "time"

    "git.smesh.lol/orly/pkg/utils/interrupt"
)

func main() {
    handler := interrupt.New()

    server := &http.Server{
        Addr: ":8080",
        Handler: http.DefaultServeMux,
    }

    // Shutdown server gracefully
    handler.OnShutdown(func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        server.Shutdown(ctx)
    })

    go server.ListenAndServe()

    <-handler.Done()
    log.Println("Server stopped")
}
```

### Worker Pool with Cleanup

```go
func main() {
    handler := interrupt.New()

    // Start worker pool
    pool := NewWorkerPool(10)
    pool.Start()

    // Clean shutdown
    handler.OnShutdown(func() {
        log.Println("Stopping worker pool...")
        pool.Stop()
    })

    <-handler.Done()
}
```

## Development

### Building

```bash
go build ./pkg/utils/interrupt
```

### Code Quality

- Comprehensive test coverage
- Go best practices compliance
- Thread-safe design
- Proper signal handling
- No external dependencies

## Integration

This package integrates well with:

- HTTP servers (graceful shutdown)
- Database connections (cleanup)
- Worker pools (coordination)
- Long-running services (reload capability)

## License

Part of the git.smesh.lol/orly project. See main LICENSE file.
