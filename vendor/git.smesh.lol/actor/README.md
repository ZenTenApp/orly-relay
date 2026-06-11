# actor

Generic building blocks for CSP/actor-model concurrency in Go.

Each type maps to a common inter-goroutine communication pattern,
eliminating the per-actor boilerplate of defining request structs,
response channels, and caller methods.

## Types

| Type | Caller | Actor | Pattern |
|------|--------|-------|---------|
| `Func[Req, Resp]` | `Call(req) -> resp` | `msg.Req`, `msg.Reply(resp)` | function call |
| `Query[Resp]` | `Call() -> resp` | `msg.Reply(resp)` | getter |
| `Proc[Req]` | `Call(req)` | `msg.Req`, `msg.Done()` | setter / command |
| `Signal` | `Call()` | `msg.Done()` | trigger / reset |
| `Inbox[T]` | `Send(v)` / `TrySend(v)` | `<-i.Recv()` | fire-and-forget |
| `Lifecycle` | `Stop()` | `<-Stopping()`, `defer MarkDone()` | shutdown |

All synchronous types use unbuffered channels. The caller blocks until
the actor processes the message. This eliminates races where a buffered
write appears to succeed but the actor hasn't seen it yet.

## Example

```go
type Counter struct {
    inc   actor.Signal
    get   actor.Query[int]
    actor.Lifecycle
}

func NewCounter() *Counter {
    c := &Counter{
        inc:       actor.NewSignal(),
        get:       actor.NewQuery[int](),
        Lifecycle: actor.NewLifecycle(),
    }
    actor.Go(c.Lifecycle, c.run)
    return c
}

func (c *Counter) run() {
    var n int
    for {
        select {
        case <-c.Stopping():
            return
        case msg := <-c.inc.Recv():
            n++
            msg.Done()
        case msg := <-c.get.Recv():
            msg.Reply(n)
        }
    }
}

func (c *Counter) Inc()     { c.inc.Call() }
func (c *Counter) Get() int { return c.get.Call() }
```

## Design Principles

- **No sync primitives.** No mutexes, no atomics, no sync.Once. All
  coordination is via channels and select.
- **Unbuffered by default.** Synchronous types guarantee
  happens-before: when Call returns, the actor has processed the
  message. Use Inbox for intentionally async paths.
- **State ownership.** The actor goroutine owns all mutable state.
  Callers get copies via response channels, never references.
- **Composable.** Embed the channel types you need. No framework, no
  interface to satisfy, no registration.

## When to use what

- **Func** - caller needs a computed result based on arguments (map
  lookup, filtered query)
- **Query** - caller needs current state, no arguments (get count, get
  config)
- **Proc** - caller needs to mutate state and wait for confirmation
  (set value, record event)
- **Signal** - caller needs to trigger an action and wait (reset,
  flush, increment)
- **Inbox** - caller must not block (logging, metrics, notifications).
  Use TrySend to drop on backpressure.

## Install

```
go get git.smesh.lol/actor
```

## License

AGPL-3.0-or-later
