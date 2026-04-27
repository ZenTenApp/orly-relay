# atomic

Type-safe atomic operations for Go primitive types. This package provides convenient wrappers around Go's `sync/atomic` package to ensure safe concurrent access to shared variables.

## Features

- **Type Safety**: Strongly typed wrappers prevent accidental misuse
- **Memory Safety**: Prevents race conditions in concurrent programs
- **Performance**: Zero-overhead wrappers around standard library atomics
- **Comprehensive Coverage**: Supports all common primitive types
- **Thread Safe**: All operations are atomic and safe for concurrent access

## Installation

```bash
go get git.smesh.lol/orly/pkg/utils/atomic
```

## Usage

The atomic package provides type-safe wrappers around Go's standard `sync/atomic` operations:

### Basic Operations

```go
import "git.smesh.lol/orly/pkg/utils/atomic"

// Integer types
var counter atomic.Uint64
counter.Store(42)
value := counter.Load()  // 42
counter.Add(8)          // 50

// Boolean operations
var flag atomic.Bool
flag.Store(true)
if flag.Load() {
    // Handle true case
}

// Pointer operations
var ptr atomic.Pointer[string]
ptr.Store(&someString)
loadedPtr := ptr.Load()
```

### Advanced Operations

```go
// Compare and swap
var value atomic.Int64
value.Store(100)
swapped := value.CompareAndSwap(100, 200)  // true, value is now 200

// Swap operations
oldValue := value.Swap(300)  // oldValue = 200, new value = 300

// Atomic add/subtract
delta := value.Add(50)  // Add 50, return new value (350)
value.Sub(25)           // Subtract 25, value is now 325
```

### Supported Types

| Type | Description |
|------|-------------|
| `Bool` | Atomic boolean operations |
| `Int32` | 32-bit signed integer |
| `Int64` | 64-bit signed integer |
| `Uint32` | 32-bit unsigned integer |
| `Uint64` | 64-bit unsigned integer |
| `Uintptr` | Pointer-sized unsigned integer |
| `Float64` | 64-bit floating point |
| `Pointer[T]` | Generic pointer type |

### Generic Pointer Example

```go
// Type-safe pointer operations
var config atomic.Pointer[Config]
config.Store(&myConfig)

// Load with type safety
currentConfig := config.Load()
if currentConfig != nil {
    // Use config with full type safety
}
```

## API Reference

All atomic types implement a consistent interface:

- `Load() T` - Atomically load the current value
- `Store(val T)` - Atomically store a new value
- `Swap(new T) T` - Atomically swap values, return old value
- `CompareAndSwap(old, new T) bool` - CAS operation

Additional methods by type:

**Integer Types (Int32, Int64, Uint32, Uint64):**
- `Add(delta T) T` - Atomically add delta, return new value
- `Sub(delta T) T` - Atomically subtract delta, return new value

**Float64:**
- `Add(delta float64) float64` - Atomically add delta

## Testing

The atomic package includes comprehensive tests:

### Running Tests

```bash
# Run atomic package tests
go test ./pkg/utils/atomic

# Run with verbose output
go test -v ./pkg/utils/atomic

# Run with race detection
go test -race ./pkg/utils/atomic

# Run with coverage
go test -cover ./pkg/utils/atomic
```

### Integration Testing

Part of the full test suite:

```bash
# Run all tests including atomic
./scripts/test.sh

# Run specific package tests
go test ./pkg/utils/...
```

### Test Coverage

Tests cover:
- All atomic operations
- Concurrent access patterns
- Race condition prevention
- Type safety
- Memory ordering guarantees

## Performance

The atomic wrappers have zero runtime overhead compared to direct `sync/atomic` calls. The Go compiler inlines all wrapper methods, resulting in identical generated code.

## Development

### Building

```bash
go build ./pkg/utils/atomic
```

### Code Quality

- Full test coverage with race detection
- Go best practices compliance
- Comprehensive documentation
- Thread-safe by design

## Examples

See the test files for comprehensive usage examples and edge cases.

## License

Part of the git.smesh.lol/orly project. See main LICENSE file.
