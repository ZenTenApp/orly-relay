# NIP-77 Implementation Analysis and Fix

## Executive Summary

This document analyzes the NIP-77 negentropy implementation in ORLY and documents fixes made to ensure compatibility with strfry and other NIP-77 compliant implementations.

## Key Issues Fixed

### Issue 1: Role Reversal in NEG-OPEN Handling (CRITICAL)

**Location:** `pkg/sync/negentropy/server/service.go:HandleNegOpen()`

**Problem:** The relay was incorrectly calling `neg.Start()` when no initial message was provided, which violates the NIP-77 protocol specification.

**NIP-77 Specification:**
- The **client (initiator)** creates a Negentropy object, adds all events, seals it, and calls `initiate()` to create the initial message
- The **relay (responder)** creates a Negentropy object, adds all events, seals it, and calls `reconcile()` with the client's initial message

**Incorrect Code:**
```go
if len(req.InitialMessage) > 0 {
    respMsg, complete, err = neg.Reconcile(req.InitialMessage)
} else {
    // WRONG: Relay should never call Start() - only the initiator does this
    respMsg, err = neg.Start()
}
```

**Fixed Code:**
```go
// The client (initiator) MUST provide an initial message in NEG-OPEN.
if len(req.InitialMessage) == 0 {
    return &negentropyv1.NegOpenResponse{
        Error: "blocked: NEG-OPEN requires initialMessage from initiator",
    }, nil
}

// Process the client's initial message
respMsg, complete, err := neg.Reconcile(req.InitialMessage)
```

### Issue 2: Premature Event Transmission

**Location:** `app/handle-negentropy.go:HandleNegOpen()` and `HandleNegMsg()`

**Problem:** Events for `haveIDs` were being sent in every round of reconciliation, instead of only when reconciliation was complete.

**Explanation:**
- The negentropy protocol involves multiple rounds of `NEG-MSG` exchanges
- In intermediate rounds, `haveIDs` and `needIDs` are partial results
- The final sets are only available when `reconcile()` returns `complete=true`
- Sending events prematurely could result in:
  - Incomplete event sets being sent
  - Wasted bandwidth on events that aren't actually needed

**Fixed Code:**
```go
// Only send events when reconciliation is complete
if complete {
    log.D.F("NEG-OPEN: reconciliation complete for %s, sending %d events", 
             subscriptionID, len(haveIDs))
    if len(haveIDs) > 0 {
        if err := l.sendEventsForIDs(subscriptionID, haveIDs); err != nil {
            log.E.F("failed to send events for NEG-OPEN: %v", err)
        }
    }
}
```

## How strfry sync Command Works

### Architecture

The strfry `sync` command is a **one-shot CLI tool**, not a persistent daemon:

```bash
# CLI invocation connects, syncs, disconnects
./strfry sync wss://relay.example.com --filter '{"kinds":[0,1]}' --dir both
```

**Key characteristics:**
1. Runs as a separate process, not part of the relay daemon
2. Opens a WebSocket connection, performs sync, closes connection
3. Uses `--dir` flag to control sync direction:
   - `down`: Download events from remote relay (pull)
   - `up`: Upload events to remote relay (push)
   - `both`: Bidirectional sync (default)
   - `none`: Only perform reconciliation, no event transfer

### Protocol Flow (strfry sync as initiator)

```
┌─────────────┐                    ┌─────────────┐
│   strfry    │  (initiator)       │   relay     │  (responder)
│   (sync)    │                    │  (orly)     │
└──────┬──────┘                    └──────┬──────┘
       │                                  │
       │ 1. Build local storage          │
       │ 2. neg.Start() → initialMsg     │
       │                                  │
       ├────── NEG-OPEN ───────────────►│
       │  subID, filter, initialMsg      │
       │                                  │
       │                                  │ 3. Build local storage
       │                                  │ 4. neg.Reconcile(initialMsg)
       │                                  │    → response, haveIDs, needIDs
       │                                  │
       │◄───── NEG-MSG ─────────────────┤
       │       response (hex)            │
       │                                  │
       │ 5. neg.Reconcile(response)      │
       │    → nextMsg, haveIDs, needIDs  │
       │                                  │
       │  (repeat NEG-MSG until complete) │
       │                                  │
       ├────── NEG-CLOSE ──────────────►│
       │                                  │
       │ 6. Exchange events:             │
       │    - Send haveIDs as EVENT      │
       │    - Receive needIDs as EVENT   │
       │    - Send OK responses          │
       │                                  │
       └────── disconnect ──────────────►│
```

### Event Exchange After Reconciliation

After the negentropy reconciliation completes:

**If `--dir up` or `--dir both`:**
```
strfry → EVENT → orly (for each ID in strfry's haveIDs)
orly   → OK    → strfry
```

**If `--dir down` or `--dir both`:**
```
strfry → REQ (ids=needIDs) → orly
orly   → EVENT → strfry (for each requested ID)
orly   → EOSE  → strfry
strfry → CLOSE → orly
```

## Protocol Compatibility

### What ORLY Now Does Correctly

1. **As Responder (receiving NEG-OPEN):**
   - ✅ Builds local storage from filter
   - ✅ Creates Negentropy instance (responder mode)
   - ✅ Calls `Reconcile()` with client's initial message
   - ✅ Returns NEG-MSG response
   - ✅ Accumulates haveIDs/needIDs across rounds
   - ✅ Sends events for haveIDs only when complete

2. **As Initiator (when syncing to peers):**
   - ✅ Calls `Start()` to generate initial message
   - ✅ Sends NEG-OPEN with initial message
   - ✅ Processes NEG-MSG responses
   - ✅ Handles received EVENT messages

### Message Formats

**NEG-OPEN (client → relay):**
```json
["NEG-OPEN", "sub123", {"kinds":[0,1]}, "a1b2c3..."]
```

**NEG-MSG (bidirectional):**
```json
["NEG-MSG", "sub123", "d4e5f6..."]
```

**NEG-CLOSE (client → relay):**
```json
["NEG-CLOSE", "sub123"]
```

**NEG-ERR (relay → client):**
```json
["NEG-ERR", "sub123", "blocked: too many events"]
```

## Testing with strfry

### ORLY as Sync Target (strfry pulls from ORLY)

```bash
# On machine with strfry
./strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1]}' --dir down
```

Expected flow:
1. strfry builds local storage
2. strfry sends NEG-OPEN with initial message
3. ORLY responds with NEG-MSG
4. Multiple NEG-MSG rounds until complete
5. strfry sends NEG-CLOSE
6. strfry sends REQ for events it needs
7. ORLY responds with EVENT messages
8. Connection closes

### ORLY as Sync Source (strfry pushes to ORLY)

```bash
# On machine with strfry
./strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1]}' --dir up
```

Expected flow:
1. strfry builds local storage
2. strfry sends NEG-OPEN with initial message
3. ORLY responds with NEG-MSG
4. Multiple NEG-MSG rounds until complete
5. strfry sends NEG-CLOSE
6. strfry sends EVENT messages for events ORLY needs
7. ORLY responds with OK messages
8. Connection closes

### Bidirectional Sync

```bash
./strfry sync wss://your-orly-relay.com --filter '{"kinds": [0, 1]}' --dir both
```

This combines both flows - events flow in both directions based on what each side is missing.

## Files Modified

1. **`pkg/sync/negentropy/server/service.go`**
   - Fixed role reversal in HandleNegOpen
   - Removed incorrect `neg.Start()` call for responder role
   - Added proper error handling for missing initial message

2. **`app/handle-negentropy.go`**
   - Fixed premature event transmission
   - Events for haveIDs now only sent when `complete=true`
   - Improved logging for better debugging

## Verification

To verify the fix works correctly:

```bash
# 1. Start ORLY relay with negentropy enabled
export ORLY_NEGENTROPY_ENABLED=true
./orly

# 2. From another machine with strfry, test sync
./strfry sync wss://your-orly-relay.com --filter '{"kinds": [1]}' --dir both

# 3. Check ORLY logs for:
#    - "NEG-OPEN: built storage with N events"
#    - "NEG-OPEN: reconcile complete=true/false"
#    - "NEG-OPEN: reconciliation complete for X, sending Y events"
```

## References

- [NIP-77 Specification](https://github.com/nostr-protocol/nips/blob/master/77.md)
- [strfry sync command](https://github.com/hoytech/strfry/blob/master/src/apps/mesh/cmd_sync.cpp)
- [strfry negentropy protocol docs](https://github.com/hoytech/strfry/blob/master/docs/negentropy.md)
- [Negentropy library](https://github.com/hoytech/negentropy)
