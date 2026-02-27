package p256k1

import (
	"crypto/rand"
	"errors"
)

// Context flags
const (
	ContextSign   = 1 << 0
	ContextVerify = 1 << 1
	ContextNone   = 0
)

// Context represents a secp256k1 context
type Context struct {
	flags       uint
	ecmultGenCtx *EcmultGenContext
	// In a real implementation, this would also contain:
	// - ecmult context for verification
	// - callback functions
	// - randomization state
}

// CallbackFunction represents an error callback
type CallbackFunction func(message string, data interface{})

// Default callback that panics on illegal arguments
func defaultIllegalCallback(message string, data interface{}) {
	panic("illegal argument: " + message)
}

// Default callback that panics on errors
func defaultErrorCallback(message string, data interface{}) {
	panic("error: " + message)
}

// ContextCreate creates a new secp256k1 context
func ContextCreate(flags uint) *Context {
	ctx := &Context{
		flags: flags,
	}
	
	// Initialize generator context if needed for signing
	if flags&ContextSign != 0 {
		ctx.ecmultGenCtx = NewEcmultGenContext()
	}
	
	// Initialize verification context if needed
	if flags&ContextVerify != 0 {
		// In a real implementation, this would initialize ecmult tables
	}
	
	return ctx
}

// ContextDestroy destroys a secp256k1 context
func ContextDestroy(ctx *Context) {
	if ctx == nil {
		return
	}
	
	// Clear sensitive data
	if ctx.ecmultGenCtx != nil {
		// Clear generator context
		ctx.ecmultGenCtx.initialized = false
	}
	
	// Zero out the context
	ctx.flags = 0
	ctx.ecmultGenCtx = nil
}

// ContextRandomize randomizes the context to provide protection against side-channel attacks
func ContextRandomize(ctx *Context, seed32 []byte) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}
	
	var seedBytes [32]byte
	
	if seed32 != nil {
		if len(seed32) != 32 {
			return errors.New("seed must be 32 bytes")
		}
		copy(seedBytes[:], seed32)
	} else {
		// Generate random seed
		if _, err := rand.Read(seedBytes[:]); err != nil {
			return err
		}
	}
	
	// In a real implementation, this would:
	// 1. Randomize the precomputed tables
	// 2. Add blinding to prevent side-channel attacks
	// 3. Update the context state
	
	// For now, we just validate the input
	return nil
}

// Global static context (read-only, for verification only)
var ContextStatic = &Context{
	flags:        ContextVerify,
	ecmultGenCtx: nil, // No signing capability
}

// Helper functions for argument checking

// argCheck checks a condition and calls the illegal callback if false
func (ctx *Context) argCheck(condition bool, message string) bool {
	if !condition {
		defaultIllegalCallback(message, nil)
		return false
	}
	return true
}

// argCheckVoid is like argCheck but for void functions
func (ctx *Context) argCheckVoid(condition bool, message string) {
	if !condition {
		defaultIllegalCallback(message, nil)
	}
}

// Capability checking

// canSign returns true if the context can be used for signing
func (ctx *Context) canSign() bool {
	return ctx != nil && (ctx.flags&ContextSign) != 0 && ctx.ecmultGenCtx != nil
}

// canVerify returns true if the context can be used for verification
func (ctx *Context) canVerify() bool {
	return ctx != nil && (ctx.flags&ContextVerify) != 0
}
