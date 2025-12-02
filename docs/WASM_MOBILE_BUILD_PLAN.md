# Plan: Enable js/wasm, iOS, and Android Builds

This document outlines the work required to enable ORLY and the nostr library to build successfully for WebAssembly (js/wasm), iOS (ios/arm64), and Android (android/arm64).

## Current Build Status

| Platform | Status | Notes |
|----------|--------|-------|
| linux/amd64 | ✅ Works | Uses libsecp256k1 via purego |
| darwin/arm64 | ✅ Works | Uses pure Go p256k1 |
| darwin/amd64 | ✅ Works | Uses pure Go p256k1 |
| windows/amd64 | ✅ Works | Uses pure Go p256k1 |
| android/arm64 | ✅ Works | Uses pure Go p256k1 |
| js/wasm | ❌ Fails | Missing platform stubs (planned for hackpadfs work) |
| ios/arm64 | ⚠️ Requires gomobile | See iOS section below |

---

## Issue 1: js/wasm Build Failures

### Problem
Two packages fail to compile for js/wasm due to missing platform-specific implementations:

1. **`next.orly.dev/pkg/utils/interrupt`** - Missing `Restart()` function
2. **`git.mleku.dev/mleku/nostr/ws`** - Missing `getConnectionOptions()` function

### Root Cause Analysis

#### 1.1 interrupt package
The `Restart()` function is defined with build tags:
- `restart.go` → `//go:build linux`
- `restart_darwin.go` → `//go:build darwin`
- `restart_windows.go` → `//go:build windows`

But `main.go` calls `Restart()` unconditionally on line 66, causing undefined symbol on js/wasm.

#### 1.2 ws package
The `getConnectionOptions()` function is defined in `connection_options.go` with:
```go
//go:build !js
```

This correctly excludes js/wasm, but no alternative implementation exists for js/wasm, so `connection.go` line 28 fails.

### Solution

#### 1.1 Fix interrupt package (ORLY)

Create a new file `restart_other.go`:

```go
//go:build !linux && !darwin && !windows

package interrupt

import (
    "lol.mleku.dev/log"
    "os"
)

// Restart is not supported on this platform - just exit
func Restart() {
    log.W.Ln("restart not supported on this platform, exiting")
    os.Exit(0)
}
```

#### 1.2 Fix ws package (nostr library)

Create a new file `connection_options_js.go`:

```go
//go:build js

package ws

import (
    "crypto/tls"
    "net/http"
)

// getConnectionOptions returns nil on js/wasm as we use browser WebSocket API
func getConnectionOptions(
    requestHeader http.Header, tlsConfig *tls.Config,
) *websocket.Dialer {
    // On js/wasm, gorilla/websocket doesn't work - need to use browser APIs
    // This is a stub that allows compilation; actual WebSocket usage would
    // need a js/wasm-compatible implementation
    return nil
}
```

**However**, this alone won't make WebSocket work - the entire `ws` package uses `gorilla/websocket` which doesn't support js/wasm. A proper fix requires:

Option A: Use conditional compilation to swap in a js/wasm WebSocket implementation (e.g., `nhooyr.io/websocket` which supports js/wasm)

Option B: Make the `ws` package optional with build tags so js/wasm builds exclude it entirely

**Recommended**: Option B - exclude the ws client package on js/wasm since ORLY is a server, not a client.

---

## Issue 2: iOS Build Failure

### Problem
```
ios/arm64 requires external (cgo) linking, but cgo is not enabled
```

### Root Cause
iOS requires CGO for all executables due to Apple's linking requirements. This is a fundamental Go limitation - you cannot build iOS binaries with `CGO_ENABLED=0`.

### Solution

#### Option A: Accept CGO requirement for iOS
Build with CGO enabled and provide a cross-compilation toolchain:

```bash
CGO_ENABLED=1 CC=clang GOOS=ios GOARCH=arm64 go build
```

This requires:
1. Xcode with iOS SDK installed
2. Cross-compilation from macOS (or complex cross-toolchain setup)

#### Option B: Create a library instead of executable
Instead of building a standalone binary, build ORLY as a Go library that can be called from Swift/Objective-C:

```bash
CGO_ENABLED=1 GOOS=ios GOARCH=arm64 go build -buildmode=c-archive -o liborly.a
```

This creates a static library usable in iOS apps.

#### Option C: Use gomobile
Use the `gomobile` tool which handles iOS cross-compilation:

```bash
gomobile bind -target=ios ./pkg/...
```

**Recommendation**: Option A or B depending on use case. For a relay server, iOS support may not be practical anyway (iOS backgrounding restrictions, network limitations).

---

## Issue 3: Android Build Failure (RESOLVED)

### Problem
```
# github.com/ebitengine/purego
dlfcn_android.go:21:13: undefined: cgo.Dlopen
```

### Root Cause
Android uses the Linux kernel, so Go's `GOOS=android` still matches the `linux` build tag. This meant our `*_linux.go` files (which import purego) were being compiled for Android.

### Solution (Implemented)

Updated all build tags in `crypto/p8k/` to explicitly exclude Android:

**Linux files** (`*_linux.go`):
```go
//go:build linux && !android && !purego
```

**Other platform files** (`*_other.go`):
```go
//go:build !linux || android || purego
```

This ensures Android uses the pure Go `p256k1.mleku.dev` implementation instead of trying to load libsecp256k1 via purego.

### Verification
```bash
GOOS=android GOARCH=arm64 CGO_ENABLED=0 go build -o orly-android-arm64
# Successfully produces 33MB ARM64 ELF binary
```

---

## Implementation Plan

### Phase 1: js/wasm Support (Low effort)

| Task | Repository | Effort |
|------|------------|--------|
| Create `restart_other.go` stub | ORLY | 5 min |
| Create `connection_options_js.go` stub OR exclude ws package | nostr | 15 min |
| Test js/wasm build compiles | Both | 5 min |

**Note**: This enables *compilation* but not *functionality*. Running ORLY in a browser would require significant additional work (no filesystem, no listening sockets, etc.).

### Phase 2: Android Support (Medium effort)

| Task | Repository | Effort |
|------|------------|--------|
| Audit purego imports - ensure Linux-only | nostr | 30 min |
| Add build tags to any files importing purego | nostr | 15 min |
| Test android/arm64 build | Both | 5 min |

### Phase 3: iOS Support (High effort, questionable value)

| Task | Repository | Effort |
|------|------------|--------|
| Set up iOS cross-compilation environment | - | 2-4 hours |
| Modify build scripts for CGO_ENABLED=1 | ORLY | 30 min |
| Create c-archive or gomobile bindings | ORLY | 2-4 hours |
| Test on iOS simulator/device | - | 1-2 hours |

**Recommendation**: iOS support should be deprioritized unless there's a specific use case. A Nostr relay is a server, and iOS imposes severe restrictions on background network services.

---

## Quick Wins (Do First)

### 1. Create `restart_other.go` in ORLY

```go
//go:build !linux && !darwin && !windows

package interrupt

import (
    "lol.mleku.dev/log"
    "os"
)

func Restart() {
    log.W.Ln("restart not supported on this platform, exiting")
    os.Exit(0)
}
```

### 2. Exclude ws package from js/wasm in nostr library

Modify `connection.go` to have a build tag:
```go
//go:build !js

package ws
// ... rest of file
```

Create `connection_js.go`:
```go
//go:build js

package ws

// Stub package for js/wasm - WebSocket client not supported
// Use browser's native WebSocket API instead
```

### 3. Audit purego usage

Ensure all files that import `github.com/ebitengine/purego` have:
```go
//go:build linux && !purego
```

---

## Estimated Total Effort

| Platform | Compilation | Full Functionality |
|----------|-------------|-------------------|
| js/wasm | 1 hour | Not practical (server) |
| android/arm64 | 1-2 hours | Possible with NDK |
| ios/arm64 | 4-8 hours | Limited (iOS restrictions) |

---

---

## iOS with gomobile

Since iOS requires CGO and you cannot use Xcode without an Apple ID, the `gomobile` approach is the best option. This creates a framework that can be integrated into iOS apps.

### Prerequisites

1. **Install gomobile**:
   ```bash
   go install golang.org/x/mobile/cmd/gomobile@latest
   gomobile init
   ```

2. **Create a bindable package**:
   gomobile can only bind packages that export types and functions suitable for mobile. You'll need to create a simplified API layer.

### Creating a Bindable API

Create a new package (e.g., `pkg/mobile/`) with a simplified interface:

```go
// pkg/mobile/relay.go
package mobile

import (
    "context"
    // ... minimal imports
)

// Relay represents an embedded Nostr relay
type Relay struct {
    // internal fields
}

// NewRelay creates a new relay instance
func NewRelay(dataDir string, port int) (*Relay, error) {
    // Initialize relay with mobile-friendly defaults
}

// Start begins accepting connections
func (r *Relay) Start() error {
    // Start the relay server
}

// Stop gracefully shuts down the relay
func (r *Relay) Stop() error {
    // Shutdown
}

// GetPublicKey returns the relay's public key
func (r *Relay) GetPublicKey() string {
    // Return npub
}
```

### Building the iOS Framework

```bash
# Build iOS framework (requires macOS)
gomobile bind -target=ios -o ORLY.xcframework ./pkg/mobile

# This produces ORLY.xcframework which can be added to Xcode projects
```

### Limitations of gomobile

1. **Only certain types are bindable**:
   - Basic types (int, float, string, bool, []byte)
   - Structs with exported fields of bindable types
   - Interfaces with methods using bindable types
   - Error return values

2. **No channels or goroutines in API**:
   The public API must be synchronous or use callbacks

3. **Callbacks require interfaces**:
   ```go
   // Define callback interface
   type EventHandler interface {
       OnEvent(eventJSON string)
   }

   // Accept callback in API
   func (r *Relay) SetEventHandler(h EventHandler) {
       // Store and use callback
   }
   ```

### Alternative: Building a Static Library

If you want more control, build as a C archive:

```bash
# From macOS with Xcode command line tools
CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
  go build -buildmode=c-archive -o liborly.a ./pkg/mobile

# This produces:
# - liborly.a (static library)
# - liborly.h (C header file)
```

This can be linked into any iOS project using the C header.

### Recommended Next Steps for iOS

1. Create `pkg/mobile/` with a minimal, mobile-friendly API
2. Test gomobile binding on Linux first: `gomobile bind -target=android ./pkg/mobile`
3. Once Android binding works, the iOS binding will use the same API
4. Find someone with macOS to run `gomobile bind -target=ios`

---

## Appendix: File Changes Summary

### nostr Repository (`git.mleku.dev/mleku/nostr`) - COMPLETED

| File | Change |
|------|--------|
| `crypto/p8k/secp_linux.go` | Build tag: `linux && !android && !purego` |
| `crypto/p8k/schnorr_linux.go` | Build tag: `linux && !android && !purego` |
| `crypto/p8k/ecdh_linux.go` | Build tag: `linux && !android && !purego` |
| `crypto/p8k/recovery_linux.go` | Build tag: `linux && !android && !purego` |
| `crypto/p8k/utils_linux.go` | Build tag: `linux && !android && !purego` |
| `crypto/p8k/secp_other.go` | Build tag: `!linux \|\| android \|\| purego` |
| `crypto/p8k/schnorr_other.go` | Build tag: `!linux \|\| android \|\| purego` |
| `crypto/p8k/ecdh_other.go` | Build tag: `!linux \|\| android \|\| purego` |
| `crypto/p8k/recovery_other.go` | Build tag: `!linux \|\| android \|\| purego` |
| `crypto/p8k/utils_other.go` | Build tag: `!linux \|\| android \|\| purego` |
| `crypto/p8k/constants.go` | NEW - shared constants (no build tags) |

### ORLY Repository (`next.orly.dev`)

| File | Change |
|------|--------|
| `go.mod` | Added `replace` directive for local nostr library |

### Future Work (js/wasm)

| File | Action Needed |
|------|---------------|
| `pkg/utils/interrupt/restart_other.go` | CREATE - stub `Restart()` for unsupported platforms |
| `nostr/ws/connection.go` | MODIFY - add `//go:build !js` or exclude package |
| `nostr/ws/connection_js.go` | CREATE - stub for js/wasm |
