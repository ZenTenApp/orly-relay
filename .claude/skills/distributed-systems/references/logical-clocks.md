# Logical Clocks - Implementation Reference

Detailed implementations and algorithms for causality tracking.

## Lamport Clock Implementation

### Data Structure

```go
type LamportClock struct {
    counter uint64
    mu      sync.Mutex
}

func NewLamportClock() *LamportClock {
    return &LamportClock{counter: 0}
}
```

### Operations

```go
// Tick increments clock for local event
func (c *LamportClock) Tick() uint64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.counter++
    return c.counter
}

// Send returns timestamp for outgoing message
func (c *LamportClock) Send() uint64 {
    return c.Tick()
}

// Receive updates clock based on incoming message timestamp
func (c *LamportClock) Receive(msgTime uint64) uint64 {
    c.mu.Lock()
    defer c.mu.Unlock()

    if msgTime > c.counter {
        c.counter = msgTime
    }
    c.counter++
    return c.counter
}

// Time returns current clock value without incrementing
func (c *LamportClock) Time() uint64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.counter
}
```

### Usage Example

```go
// Process A
clockA := NewLamportClock()
e1 := clockA.Tick()      // Event 1: time=1
msgTime := clockA.Send() // Send: time=2

// Process B
clockB := NewLamportClock()
e2 := clockB.Tick()           // Event 2: time=1
e3 := clockB.Receive(msgTime) // Receive: time=3 (max(1,2)+1)
```

## Vector Clock Implementation

### Data Structure

```go
type VectorClock struct {
    clocks map[string]uint64  // processID -> logical time
    self   string             // this process's ID
    mu     sync.RWMutex
}

func NewVectorClock(processID string, allProcesses []string) *VectorClock {
    clocks := make(map[string]uint64)
    for _, p := range allProcesses {
        clocks[p] = 0
    }
    return &VectorClock{
        clocks: clocks,
        self:   processID,
    }
}
```

### Operations

```go
// Tick increments own clock
func (vc *VectorClock) Tick() map[string]uint64 {
    vc.mu.Lock()
    defer vc.mu.Unlock()

    vc.clocks[vc.self]++
    return vc.copy()
}

// Send returns copy of vector for message
func (vc *VectorClock) Send() map[string]uint64 {
    return vc.Tick()
}

// Receive merges incoming vector and increments
func (vc *VectorClock) Receive(incoming map[string]uint64) map[string]uint64 {
    vc.mu.Lock()
    defer vc.mu.Unlock()

    // Merge: take max of each component
    for pid, time := range incoming {
        if time > vc.clocks[pid] {
            vc.clocks[pid] = time
        }
    }

    // Increment own clock
    vc.clocks[vc.self]++
    return vc.copy()
}

// copy returns a copy of the vector
func (vc *VectorClock) copy() map[string]uint64 {
    result := make(map[string]uint64)
    for k, v := range vc.clocks {
        result[k] = v
    }
    return result
}
```

### Comparison Functions

```go
// Compare returns ordering relationship between two vectors
type Ordering int

const (
    Equal      Ordering = iota // V1 == V2
    HappenedBefore            // V1 < V2
    HappenedAfter             // V1 > V2
    Concurrent                // V1 || V2
)

func Compare(v1, v2 map[string]uint64) Ordering {
    less := false
    greater := false

    // Get all keys
    allKeys := make(map[string]bool)
    for k := range v1 {
        allKeys[k] = true
    }
    for k := range v2 {
        allKeys[k] = true
    }

    for k := range allKeys {
        t1 := v1[k] // 0 if not present
        t2 := v2[k]

        if t1 < t2 {
            less = true
        }
        if t1 > t2 {
            greater = true
        }
    }

    if !less && !greater {
        return Equal
    }
    if less && !greater {
        return HappenedBefore
    }
    if greater && !less {
        return HappenedAfter
    }
    return Concurrent
}

// IsConcurrent checks if two events are concurrent
func IsConcurrent(v1, v2 map[string]uint64) bool {
    return Compare(v1, v2) == Concurrent
}

// HappenedBefore checks if v1 -> v2 (v1 causally precedes v2)
func HappenedBefore(v1, v2 map[string]uint64) bool {
    return Compare(v1, v2) == HappenedBefore
}
```

## Interval Tree Clock Implementation

### Data Structures

```go
// ID represents the identity tree
type ID struct {
    IsLeaf bool
    Value  int   // 0 or 1 for leaves
    Left   *ID   // nil for leaves
    Right  *ID
}

// Stamp represents the event tree
type Stamp struct {
    Base  int
    Left  *Stamp // nil for leaf stamps
    Right *Stamp
}

// ITC combines ID and Stamp
type ITC struct {
    ID    *ID
    Stamp *Stamp
}
```

### ID Operations

```go
// NewSeedID creates initial full ID (1)
func NewSeedID() *ID {
    return &ID{IsLeaf: true, Value: 1}
}

// Fork splits an ID into two
func (id *ID) Fork() (*ID, *ID) {
    if id.IsLeaf {
        if id.Value == 0 {
            // Cannot fork zero ID
            return &ID{IsLeaf: true, Value: 0},
                   &ID{IsLeaf: true, Value: 0}
        }
        // Split full ID into left and right halves
        return &ID{
                IsLeaf: false,
                Left:   &ID{IsLeaf: true, Value: 1},
                Right:  &ID{IsLeaf: true, Value: 0},
            },
            &ID{
                IsLeaf: false,
                Left:   &ID{IsLeaf: true, Value: 0},
                Right:  &ID{IsLeaf: true, Value: 1},
            }
    }

    // Fork from non-leaf: give half to each
    if id.Left.IsLeaf && id.Left.Value == 0 {
        // Left is zero, fork right
        newRight1, newRight2 := id.Right.Fork()
        return &ID{IsLeaf: false, Left: id.Left, Right: newRight1},
               &ID{IsLeaf: false, Left: &ID{IsLeaf: true, Value: 0}, Right: newRight2}
    }
    if id.Right.IsLeaf && id.Right.Value == 0 {
        // Right is zero, fork left
        newLeft1, newLeft2 := id.Left.Fork()
        return &ID{IsLeaf: false, Left: newLeft1, Right: id.Right},
               &ID{IsLeaf: false, Left: newLeft2, Right: &ID{IsLeaf: true, Value: 0}}
    }

    // Both have IDs, split
    return &ID{IsLeaf: false, Left: id.Left, Right: &ID{IsLeaf: true, Value: 0}},
           &ID{IsLeaf: false, Left: &ID{IsLeaf: true, Value: 0}, Right: id.Right}
}

// Join merges two IDs
func Join(id1, id2 *ID) *ID {
    if id1.IsLeaf && id1.Value == 0 {
        return id2
    }
    if id2.IsLeaf && id2.Value == 0 {
        return id1
    }
    if id1.IsLeaf && id2.IsLeaf && id1.Value == 1 && id2.Value == 1 {
        return &ID{IsLeaf: true, Value: 1}
    }

    // Normalize to non-leaf
    left1 := id1.Left
    right1 := id1.Right
    left2 := id2.Left
    right2 := id2.Right

    if id1.IsLeaf {
        left1 = id1
        right1 = id1
    }
    if id2.IsLeaf {
        left2 = id2
        right2 = id2
    }

    newLeft := Join(left1, left2)
    newRight := Join(right1, right2)

    return normalize(&ID{IsLeaf: false, Left: newLeft, Right: newRight})
}

func normalize(id *ID) *ID {
    if !id.IsLeaf {
        if id.Left.IsLeaf && id.Right.IsLeaf &&
           id.Left.Value == id.Right.Value {
            return &ID{IsLeaf: true, Value: id.Left.Value}
        }
    }
    return id
}
```

### Stamp Operations

```go
// NewStamp creates initial stamp (0)
func NewStamp() *Stamp {
    return &Stamp{Base: 0}
}

// Event increments the stamp for the given ID
func Event(id *ID, stamp *Stamp) *Stamp {
    if id.IsLeaf {
        if id.Value == 1 {
            return &Stamp{Base: stamp.Base + 1}
        }
        return stamp // Cannot increment with zero ID
    }

    // Non-leaf ID: fill where we have ID
    if id.Left.IsLeaf && id.Left.Value == 1 {
        // Have left ID, increment left
        newLeft := Event(&ID{IsLeaf: true, Value: 1}, getLeft(stamp))
        return normalizeStamp(&Stamp{
            Base:  stamp.Base,
            Left:  newLeft,
            Right: getRight(stamp),
        })
    }
    if id.Right.IsLeaf && id.Right.Value == 1 {
        newRight := Event(&ID{IsLeaf: true, Value: 1}, getRight(stamp))
        return normalizeStamp(&Stamp{
            Base:  stamp.Base,
            Left:  getLeft(stamp),
            Right: newRight,
        })
    }

    // Both non-zero, choose lower side
    leftMax := maxStamp(getLeft(stamp))
    rightMax := maxStamp(getRight(stamp))

    if leftMax <= rightMax {
        return normalizeStamp(&Stamp{
            Base:  stamp.Base,
            Left:  Event(id.Left, getLeft(stamp)),
            Right: getRight(stamp),
        })
    }
    return normalizeStamp(&Stamp{
        Base:  stamp.Base,
        Left:  getLeft(stamp),
        Right: Event(id.Right, getRight(stamp)),
    })
}

func getLeft(s *Stamp) *Stamp {
    if s.Left == nil {
        return &Stamp{Base: 0}
    }
    return s.Left
}

func getRight(s *Stamp) *Stamp {
    if s.Right == nil {
        return &Stamp{Base: 0}
    }
    return s.Right
}

func maxStamp(s *Stamp) int {
    if s.Left == nil && s.Right == nil {
        return s.Base
    }
    left := 0
    right := 0
    if s.Left != nil {
        left = maxStamp(s.Left)
    }
    if s.Right != nil {
        right = maxStamp(s.Right)
    }
    max := left
    if right > max {
        max = right
    }
    return s.Base + max
}

// JoinStamps merges two stamps
func JoinStamps(s1, s2 *Stamp) *Stamp {
    // Take max at each level
    base := s1.Base
    if s2.Base > base {
        base = s2.Base
    }

    // Adjust for base difference
    adj1 := s1.Base
    adj2 := s2.Base

    return normalizeStamp(&Stamp{
        Base:  base,
        Left:  joinStampsRecursive(s1.Left, s2.Left, adj1-base, adj2-base),
        Right: joinStampsRecursive(s1.Right, s2.Right, adj1-base, adj2-base),
    })
}

func normalizeStamp(s *Stamp) *Stamp {
    if s.Left == nil && s.Right == nil {
        return s
    }
    if s.Left != nil && s.Right != nil {
        if s.Left.Base > 0 && s.Right.Base > 0 {
            min := s.Left.Base
            if s.Right.Base < min {
                min = s.Right.Base
            }
            return &Stamp{
                Base:  s.Base + min,
                Left:  &Stamp{Base: s.Left.Base - min, Left: s.Left.Left, Right: s.Left.Right},
                Right: &Stamp{Base: s.Right.Base - min, Left: s.Right.Left, Right: s.Right.Right},
            }
        }
    }
    return s
}
```

## Hybrid Logical Clock Implementation

```go
type HLC struct {
    l  int64      // logical component (physical time)
    c  int64      // counter
    mu sync.Mutex
}

func NewHLC() *HLC {
    return &HLC{l: 0, c: 0}
}

type HLCTimestamp struct {
    L int64
    C int64
}

func (hlc *HLC) physicalTime() int64 {
    return time.Now().UnixNano()
}

// Now returns current HLC timestamp for local/send event
func (hlc *HLC) Now() HLCTimestamp {
    hlc.mu.Lock()
    defer hlc.mu.Unlock()

    pt := hlc.physicalTime()

    if pt > hlc.l {
        hlc.l = pt
        hlc.c = 0
    } else {
        hlc.c++
    }

    return HLCTimestamp{L: hlc.l, C: hlc.c}
}

// Update updates HLC based on received timestamp
func (hlc *HLC) Update(received HLCTimestamp) HLCTimestamp {
    hlc.mu.Lock()
    defer hlc.mu.Unlock()

    pt := hlc.physicalTime()

    if pt > hlc.l && pt > received.L {
        hlc.l = pt
        hlc.c = 0
    } else if received.L > hlc.l {
        hlc.l = received.L
        hlc.c = received.C + 1
    } else if hlc.l > received.L {
        hlc.c++
    } else { // hlc.l == received.L
        if received.C > hlc.c {
            hlc.c = received.C + 1
        } else {
            hlc.c++
        }
    }

    return HLCTimestamp{L: hlc.l, C: hlc.c}
}

// Compare compares two HLC timestamps
func (t1 HLCTimestamp) Compare(t2 HLCTimestamp) int {
    if t1.L < t2.L {
        return -1
    }
    if t1.L > t2.L {
        return 1
    }
    if t1.C < t2.C {
        return -1
    }
    if t1.C > t2.C {
        return 1
    }
    return 0
}
```

## Causal Broadcast Implementation

```go
type CausalBroadcast struct {
    vc       *VectorClock
    pending  []PendingMessage
    deliver  func(Message)
    mu       sync.Mutex
}

type PendingMessage struct {
    Msg       Message
    Timestamp map[string]uint64
}

func NewCausalBroadcast(processID string, processes []string, deliver func(Message)) *CausalBroadcast {
    return &CausalBroadcast{
        vc:      NewVectorClock(processID, processes),
        pending: make([]PendingMessage, 0),
        deliver: deliver,
    }
}

// Broadcast sends a message to all processes
func (cb *CausalBroadcast) Broadcast(msg Message) map[string]uint64 {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    timestamp := cb.vc.Send()
    // Actual network broadcast would happen here
    return timestamp
}

// Receive handles an incoming message
func (cb *CausalBroadcast) Receive(msg Message, sender string, timestamp map[string]uint64) {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    // Add to pending
    cb.pending = append(cb.pending, PendingMessage{Msg: msg, Timestamp: timestamp})

    // Try to deliver pending messages
    cb.tryDeliver()
}

func (cb *CausalBroadcast) tryDeliver() {
    changed := true
    for changed {
        changed = false

        for i, pending := range cb.pending {
            if cb.canDeliver(pending.Timestamp) {
                // Deliver message
                cb.vc.Receive(pending.Timestamp)
                cb.deliver(pending.Msg)

                // Remove from pending
                cb.pending = append(cb.pending[:i], cb.pending[i+1:]...)
                changed = true
                break
            }
        }
    }
}

func (cb *CausalBroadcast) canDeliver(msgVC map[string]uint64) bool {
    currentVC := cb.vc.clocks

    for pid, msgTime := range msgVC {
        if pid == cb.vc.self {
            // Must be next expected from sender
            if msgTime != currentVC[pid]+1 {
                return false
            }
        } else {
            // All other dependencies must be satisfied
            if msgTime > currentVC[pid] {
                return false
            }
        }
    }
    return true
}
```
