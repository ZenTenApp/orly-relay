# Consensus Protocols - Detailed Reference

Complete specifications and implementation details for major consensus protocols.

## Paxos Complete Specification

### Proposal Numbers

Proposal numbers must be:
- **Unique**: No two proposers use the same number
- **Totally ordered**: Any two can be compared

**Implementation**: `(round_number, proposer_id)` where proposer_id breaks ties.

### Single-Decree Paxos State

**Proposer state**:
```
proposal_number: int
value: any
```

**Acceptor state (persistent)**:
```
highest_promised: int    # Highest proposal number promised
accepted_proposal: int   # Number of accepted proposal (0 if none)
accepted_value: any      # Value of accepted proposal (null if none)
```

### Message Format

**Prepare** (Phase 1a):
```
{
  type: "PREPARE",
  proposal_number: n
}
```

**Promise** (Phase 1b):
```
{
  type: "PROMISE",
  proposal_number: n,
  accepted_proposal: m,    # null if nothing accepted
  accepted_value: v        # null if nothing accepted
}
```

**Accept** (Phase 2a):
```
{
  type: "ACCEPT",
  proposal_number: n,
  value: v
}
```

**Accepted** (Phase 2b):
```
{
  type: "ACCEPTED",
  proposal_number: n,
  value: v
}
```

### Proposer Algorithm

```
function propose(value):
    n = generate_proposal_number()

    # Phase 1: Prepare
    promises = []
    for acceptor in acceptors:
        send PREPARE(n) to acceptor

    wait until |promises| > |acceptors|/2 or timeout

    if timeout:
        return FAILED

    # Choose value
    highest = max(promises, key=p.accepted_proposal)
    if highest.accepted_value is not null:
        value = highest.accepted_value

    # Phase 2: Accept
    accepts = []
    for acceptor in acceptors:
        send ACCEPT(n, value) to acceptor

    wait until |accepts| > |acceptors|/2 or timeout

    if timeout:
        return FAILED

    return SUCCESS(value)
```

### Acceptor Algorithm

```
on receive PREPARE(n):
    if n > highest_promised:
        highest_promised = n
        persist(highest_promised)
        reply PROMISE(n, accepted_proposal, accepted_value)
    else:
        # Optionally reply NACK(highest_promised)
        ignore or reject

on receive ACCEPT(n, v):
    if n >= highest_promised:
        highest_promised = n
        accepted_proposal = n
        accepted_value = v
        persist(highest_promised, accepted_proposal, accepted_value)
        reply ACCEPTED(n, v)
    else:
        ignore or reject
```

### Multi-Paxos Optimization

**Stable leader**:
```
# Leader election (using Paxos or other method)
leader = elect_leader()

# Leader's Phase 1 for all future instances
leader sends PREPARE(n) for instance range [i, ∞)

# For each command:
function propose_as_leader(value, instance):
    # Skip Phase 1 if already leader
    for acceptor in acceptors:
        send ACCEPT(n, value, instance) to acceptor
    wait for majority ACCEPTED
    return SUCCESS
```

### Paxos Safety Proof Sketch

**Invariant**: If a value v is chosen for instance i, no other value can be chosen.

**Proof**:
1. Value chosen → accepted by majority with proposal n
2. Any higher proposal n' must contact majority
3. Majorities intersect → at least one acceptor has accepted v
4. New proposer adopts v (or higher already-accepted value)
5. By induction, all future proposals use v

## Raft Complete Specification

### State

**All servers (persistent)**:
```
currentTerm: int      # Latest term seen
votedFor: ServerId    # Candidate voted for in current term (null if none)
log[]: LogEntry       # Log entries
```

**All servers (volatile)**:
```
commitIndex: int      # Highest log index known to be committed
lastApplied: int      # Highest log index applied to state machine
```

**Leader (volatile, reinitialized after election)**:
```
nextIndex[]: int      # For each server, next log index to send
matchIndex[]: int     # For each server, highest log index replicated
```

**LogEntry**:
```
{
  term: int,
  command: any
}
```

### RequestVote RPC

**Request**:
```
{
  term: int,              # Candidate's term
  candidateId: ServerId,  # Candidate requesting vote
  lastLogIndex: int,      # Index of candidate's last log entry
  lastLogTerm: int        # Term of candidate's last log entry
}
```

**Response**:
```
{
  term: int,              # currentTerm, for candidate to update itself
  voteGranted: bool       # True if candidate received vote
}
```

**Receiver implementation**:
```
on receive RequestVote(term, candidateId, lastLogIndex, lastLogTerm):
    if term < currentTerm:
        return {term: currentTerm, voteGranted: false}

    if term > currentTerm:
        currentTerm = term
        votedFor = null
        convert to follower

    # Check if candidate's log is at least as up-to-date as ours
    ourLastTerm = log[len(log)-1].term if log else 0
    ourLastIndex = len(log) - 1

    logOK = (lastLogTerm > ourLastTerm) or
            (lastLogTerm == ourLastTerm and lastLogIndex >= ourLastIndex)

    if (votedFor is null or votedFor == candidateId) and logOK:
        votedFor = candidateId
        persist(currentTerm, votedFor)
        reset election timer
        return {term: currentTerm, voteGranted: true}

    return {term: currentTerm, voteGranted: false}
```

### AppendEntries RPC

**Request**:
```
{
  term: int,              # Leader's term
  leaderId: ServerId,     # For follower to redirect clients
  prevLogIndex: int,      # Index of log entry preceding new ones
  prevLogTerm: int,       # Term of prevLogIndex entry
  entries[]: LogEntry,    # Log entries to store (empty for heartbeat)
  leaderCommit: int       # Leader's commitIndex
}
```

**Response**:
```
{
  term: int,              # currentTerm, for leader to update itself
  success: bool           # True if follower had matching prevLog entry
}
```

**Receiver implementation**:
```
on receive AppendEntries(term, leaderId, prevLogIndex, prevLogTerm, entries, leaderCommit):
    if term < currentTerm:
        return {term: currentTerm, success: false}

    reset election timer

    if term > currentTerm:
        currentTerm = term
        votedFor = null

    convert to follower

    # Check log consistency
    if prevLogIndex >= len(log) or
       (prevLogIndex >= 0 and log[prevLogIndex].term != prevLogTerm):
        return {term: currentTerm, success: false}

    # Append new entries (handling conflicts)
    for i, entry in enumerate(entries):
        index = prevLogIndex + 1 + i
        if index < len(log):
            if log[index].term != entry.term:
                # Delete conflicting entry and all following
                log = log[:index]
                log.append(entry)
        else:
            log.append(entry)

    persist(currentTerm, votedFor, log)

    # Update commit index
    if leaderCommit > commitIndex:
        commitIndex = min(leaderCommit, len(log) - 1)

    return {term: currentTerm, success: true}
```

### Leader Behavior

```
on becoming leader:
    for each server:
        nextIndex[server] = len(log)
        matchIndex[server] = 0

    start sending heartbeats

on receiving client command:
    append entry to local log
    persist log
    send AppendEntries to all followers

on receiving AppendEntries response from server:
    if response.success:
        matchIndex[server] = prevLogIndex + len(entries)
        nextIndex[server] = matchIndex[server] + 1

        # Update commit index
        for N from commitIndex+1 to len(log)-1:
            if log[N].term == currentTerm and
               |{s : matchIndex[s] >= N}| > |servers|/2:
                commitIndex = N
    else:
        nextIndex[server] = max(1, nextIndex[server] - 1)
        retry AppendEntries with lower prevLogIndex

on commitIndex update:
    while lastApplied < commitIndex:
        lastApplied++
        apply log[lastApplied].command to state machine
```

### Election Timeout

```
on election timeout (follower or candidate):
    currentTerm++
    convert to candidate
    votedFor = self
    persist(currentTerm, votedFor)
    reset election timer
    votes = 1  # Vote for self

    for each server except self:
        send RequestVote(currentTerm, self, lastLogIndex, lastLogTerm)

    wait for responses or timeout:
        if received votes > |servers|/2:
            become leader
        if received AppendEntries from valid leader:
            become follower
        if timeout:
            start new election
```

## PBFT Complete Specification

### Message Types

**REQUEST**:
```
{
  type: "REQUEST",
  operation: o,           # Operation to execute
  timestamp: t,           # Client timestamp (for reply matching)
  client: c               # Client identifier
}
```

**PRE-PREPARE**:
```
{
  type: "PRE-PREPARE",
  view: v,                # Current view number
  sequence: n,            # Sequence number
  digest: d,              # Hash of request
  request: m              # The request message
}
signature(primary)
```

**PREPARE**:
```
{
  type: "PREPARE",
  view: v,
  sequence: n,
  digest: d,
  replica: i              # Sending replica
}
signature(replica_i)
```

**COMMIT**:
```
{
  type: "COMMIT",
  view: v,
  sequence: n,
  digest: d,
  replica: i
}
signature(replica_i)
```

**REPLY**:
```
{
  type: "REPLY",
  view: v,
  timestamp: t,
  client: c,
  replica: i,
  result: r               # Execution result
}
signature(replica_i)
```

### Replica State

```
view: int                       # Current view
sequence: int                   # Last assigned sequence number (primary)
log[]: {request, prepares, commits, state}  # Log of requests
prepared_certificates: {}       # Prepared certificates (2f+1 prepares)
committed_certificates: {}      # Committed certificates (2f+1 commits)
h: int                          # Low water mark
H: int                          # High water mark (h + L)
```

### Normal Operation Protocol

**Primary (replica p = v mod n)**:
```
on receive REQUEST(m) from client:
    if not primary for current view:
        forward to primary
        return

    n = assign_sequence_number()
    d = hash(m)

    broadcast PRE-PREPARE(v, n, d, m) to all replicas
    add to log
```

**All replicas**:
```
on receive PRE-PREPARE(v, n, d, m) from primary:
    if v != current_view:
        ignore
    if already accepted pre-prepare for (v, n) with different digest:
        ignore
    if not in_view_as_backup(v):
        ignore
    if not h < n <= H:
        ignore  # Outside sequence window

    # Valid pre-prepare
    add to log
    broadcast PREPARE(v, n, d, i) to all replicas

on receive PREPARE(v, n, d, j) from replica j:
    if v != current_view:
        ignore

    add to log[n].prepares

    if |log[n].prepares| >= 2f and not already_prepared(v, n, d):
        # Prepared certificate complete
        mark as prepared
        broadcast COMMIT(v, n, d, i) to all replicas

on receive COMMIT(v, n, d, j) from replica j:
    if v != current_view:
        ignore

    add to log[n].commits

    if |log[n].commits| >= 2f + 1 and prepared(v, n, d):
        # Committed certificate complete
        if all entries < n are committed:
            execute(m)
            send REPLY(v, t, c, i, result) to client
```

### View Change Protocol

**Timeout trigger**:
```
on request timeout (no progress):
    view_change_timeout++
    broadcast VIEW-CHANGE(v+1, n, C, P, i)

    where:
      n = last stable checkpoint sequence number
      C = checkpoint certificate (2f+1 checkpoint messages)
      P = set of prepared certificates for messages after n
```

**VIEW-CHANGE**:
```
{
  type: "VIEW-CHANGE",
  view: v,                      # New view number
  sequence: n,                  # Checkpoint sequence
  checkpoints: C,               # Checkpoint certificate
  prepared: P,                  # Set of prepared certificates
  replica: i
}
signature(replica_i)
```

**New primary (p' = v mod n)**:
```
on receive 2f VIEW-CHANGE for view v:
    V = set of valid view-change messages

    # Compute O: set of requests to re-propose
    O = {}
    for seq in max_checkpoint_seq(V) to max_seq(V):
        if exists prepared certificate for seq in V:
            O[seq] = request from certificate
        else:
            O[seq] = null-request  # No-op

    broadcast NEW-VIEW(v, V, O)

    # Re-run protocol for requests in O
    for seq, request in O:
        if request != null:
            send PRE-PREPARE(v, seq, hash(request), request)
```

**NEW-VIEW**:
```
{
  type: "NEW-VIEW",
  view: v,
  view_changes: V,              # 2f+1 view-change messages
  pre_prepares: O               # Set of pre-prepare messages
}
signature(primary)
```

### Checkpointing

Periodic stable checkpoints to garbage collect logs:

```
every K requests:
    state_hash = hash(state_machine_state)
    broadcast CHECKPOINT(n, state_hash, i)

on receive 2f+1 CHECKPOINT for (n, d):
    if all digests match:
        create stable checkpoint
        h = n  # Move low water mark
        garbage_collect(entries < n)
```

## HotStuff Protocol

Linear complexity BFT using threshold signatures.

### Key Innovation

- **Three-phase**: prepare → pre-commit → commit → decide
- **Pipelining**: Next proposal starts before current finishes
- **Threshold signatures**: O(n) total messages instead of O(n²)

### Message Flow

```
Phase 1 (Prepare):
  Leader: broadcast PREPARE(v, node)
  Replicas: sign and send partial signature to leader
  Leader: aggregate into prepare certificate QC

Phase 2 (Pre-commit):
  Leader: broadcast PRE-COMMIT(v, QC_prepare)
  Replicas: sign and send partial signature
  Leader: aggregate into pre-commit certificate

Phase 3 (Commit):
  Leader: broadcast COMMIT(v, QC_precommit)
  Replicas: sign and send partial signature
  Leader: aggregate into commit certificate

Phase 4 (Decide):
  Leader: broadcast DECIDE(v, QC_commit)
  Replicas: execute and commit
```

### Pipelining

```
Block k:   [prepare] [pre-commit] [commit] [decide]
Block k+1:          [prepare] [pre-commit] [commit] [decide]
Block k+2:                   [prepare] [pre-commit] [commit] [decide]
```

Each phase of block k+1 piggybacks on messages for block k.

## Protocol Comparison Matrix

| Feature | Paxos | Raft | PBFT | HotStuff |
|---------|-------|------|------|----------|
| Fault model | Crash | Crash | Byzantine | Byzantine |
| Fault tolerance | f with 2f+1 | f with 2f+1 | f with 3f+1 | f with 3f+1 |
| Message complexity | O(n) | O(n) | O(n²) | O(n) |
| Leader required | No (helps) | Yes | Yes | Yes |
| Phases | 2 | 2 | 3 | 3 |
| View change | Complex | Simple | Complex | Simple |
