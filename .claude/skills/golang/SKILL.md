---
name: golang
description: This skill should be used when writing, debugging, reviewing, or discussing Go (Golang) code. Provides comprehensive Go programming expertise including idiomatic patterns, standard library, concurrency, error handling, testing, and best practices based on official go.dev documentation.
---

# Go Programming Expert

## Purpose

This skill provides expert-level assistance with Go programming language development, covering language fundamentals, idiomatic patterns, concurrency, error handling, standard library usage, testing, and best practices.

## When to Use

Activate this skill when:
- Writing Go code
- Debugging Go programs
- Reviewing Go code for best practices
- Answering questions about Go language features
- Implementing Go-specific patterns (goroutines, channels, interfaces)
- Setting up Go projects and modules
- Writing Go tests

## Core Principles

When writing Go code, always follow these principles:

1. **Named Return Variables**: ALWAYS use named return variables and prefer naked returns for cleaner code
2. **Error Handling**: Use `lol.mleku.dev/log` and the `chk/errorf` for error checking and creating new errors
3. **Idiomatic Code**: Write clear, idiomatic Go code following Effective Go guidelines
4. **Simplicity**: Favor simplicity and clarity over cleverness
5. **Composition**: Prefer composition over inheritance
6. **Explicit**: Be explicit rather than implicit

## Key Go Concepts

### Functions with Named Returns

Always use named return values:
```go
func divide(a, b float64) (result float64, err error) {
    if b == 0 {
        err = errorf.New("division by zero")
        return
    }
    result = a / b
    return
}
```

### Error Handling

Use the specified error handling packages:
```go
import "lol.mleku.dev/log"

// Error checking with chk
if err := doSomething(); chk.E(err) {
    return
}

// Creating errors with errorf
err := errorf.New("something went wrong")
err := errorf.Errorf("failed to process: %v", value)
```

### Interfaces and Composition

Go uses implicit interface implementation:
```go
type Reader interface {
    Read(p []byte) (n int, err error)
}

// Any type with a Read method implements Reader
type File struct {
    name string
}

func (f *File) Read(p []byte) (n int, err error) {
    // Implementation
    return
}
```

### Interface Design - CRITICAL RULES

**Rule 1: Define interfaces in a dedicated package (e.g., `pkg/interfaces/<name>/`)**
- Interfaces provide isolation between packages and enable dependency inversion
- Keeping interfaces in a dedicated package prevents circular dependencies
- Each interface package should be minimal (just the interface, no implementations)

**Rule 2: NEVER use type assertions with interface literals**
- **NEVER** write `.(interface{ Method() Type })` - this is non-idiomatic and unmaintainable
- Interface literals cannot be documented, tested for satisfaction, or reused

```go
// BAD - interface literal in type assertion (NEVER DO THIS)
if checker, ok := obj.(interface{ Check() bool }); ok {
    checker.Check()
}

// GOOD - use defined interface from dedicated package
import "myproject/pkg/interfaces/checker"

if c, ok := obj.(checker.Checker); ok {
    c.Check()
}
```

**Rule 3: Resolving Circular Dependencies**
- If a circular dependency occurs, move the interface to `pkg/interfaces/`
- The implementing type stays in its original package
- The consuming code imports only the interface package
- Pattern:
  ```
  pkg/interfaces/foo/   <- interface definition (no dependencies)
       ↑           ↑
  pkg/bar/         pkg/baz/
  (implements)     (consumes via interface)
  ```

**Rule 4: Verify interface satisfaction at compile time**
```go
// Add this line to ensure *MyType implements MyInterface
var _ MyInterface = (*MyType)(nil)
```

### Concurrency

Use goroutines and channels for concurrent programming:
```go
// Launch goroutine
go doWork()

// Channels
ch := make(chan int, 10)
ch <- 42
value := <-ch

// Select statement
select {
case msg := <-ch1:
    // Handle
case <-time.After(time.Second):
    // Timeout
}

// Sync primitives
var mu sync.Mutex
mu.Lock()
defer mu.Unlock()
```

### Testing

Use table-driven tests as the default pattern:
```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name     string
        a, b     int
        expected int
    }{
        {"positive", 2, 3, 5},
        {"negative", -1, -1, -2},
        {"zero", 0, 5, 5},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := Add(tt.a, tt.b)
            if result != tt.expected {
                t.Errorf("got %d, want %d", result, tt.expected)
            }
        })
    }
}
```

## Reference Materials

For detailed information, consult the reference files:

- **references/effective-go-summary.md** - Key points from Effective Go including formatting, naming, control structures, functions, data allocation, methods, interfaces, concurrency principles, and error handling philosophy

- **references/common-patterns.md** - Practical Go patterns including:
  - Design patterns (Functional Options, Builder, Singleton, Factory, Strategy)
  - Concurrency patterns (Worker Pool, Pipeline, Fan-Out/Fan-In, Timeout, Rate Limiting, Circuit Breaker)
  - Error handling patterns (Error Wrapping, Sentinel Errors, Custom Error Types)
  - Resource management patterns
  - Testing patterns

- **references/quick-reference.md** - Quick syntax cheatsheet with common commands, format verbs, standard library snippets, and best practices checklist

## Best Practices Summary

1. **Naming Conventions**
   - Use camelCase for variables and functions
   - Use PascalCase for exported names
   - Keep names short but descriptive
   - Interface names often end in -er (Reader, Writer, Handler)

2. **Error Handling**
   - Always check errors
   - Use named return values
   - Use lol.mleku.dev/log and chk/errorf

3. **Code Organization**
   - One package per directory
   - Use internal/ for non-exported packages
   - Use cmd/ for applications
   - Use pkg/ for reusable libraries

4. **Concurrency**
   - Don't communicate by sharing memory; share memory by communicating
   - Always close channels from sender
   - Use defer for cleanup

5. **Documentation**
   - Comment all exported names
   - Start comments with the name being described
   - Use godoc format

6. **Configuration - CRITICAL**
   - **NEVER** use `os.Getenv()` scattered throughout packages
   - **ALWAYS** centralize environment variable parsing in a single config package (e.g., `app/config/`)
   - Pass configuration via structs, not by reading environment directly
   - This ensures discoverability, documentation, and testability of all config options

7. **Constants - CRITICAL**
   - **ALWAYS** define named constants for values used more than a few times
   - **ALWAYS** define named constants if multiple packages depend on the same value
   - Constants shared across packages belong in a dedicated package (e.g., `pkg/constants/`)
   - Magic numbers and strings are forbidden
   ```go
   // BAD - magic number
   if size > 1024 {

   // GOOD - named constant
   const MaxBufferSize = 1024
   if size > MaxBufferSize {
   ```

## Common Commands

```bash
go run main.go          # Run program
go build                # Compile
go test                 # Run tests
go test -v              # Verbose tests
go test -cover          # Test coverage
go test -race           # Race detection
go fmt                  # Format code
go vet                  # Lint code
go mod tidy             # Clean dependencies
go get package          # Add dependency
```

## Official Resources

All guidance is based on official Go documentation:
- Go Website: https://go.dev
- Documentation: https://go.dev/doc/
- Effective Go: https://go.dev/doc/effective_go
- Language Specification: https://go.dev/ref/spec
- Standard Library: https://pkg.go.dev/std
- Go Tour: https://go.dev/tour/

