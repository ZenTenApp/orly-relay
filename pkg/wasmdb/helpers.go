//go:build js && wasm

package wasmdb

import (
	"syscall/js"

	"github.com/hack-pad/safejs"
)

// safeValueToBytes converts a safejs.Value to a []byte
// This handles Uint8Array, ArrayBuffer, and strings from IndexedDB
func safeValueToBytes(val safejs.Value) []byte {
	if val.IsUndefined() || val.IsNull() {
		return nil
	}

	// Get global Uint8Array and ArrayBuffer constructors
	uint8ArrayType := safejs.MustGetGlobal("Uint8Array")
	arrayBufferType := safejs.MustGetGlobal("ArrayBuffer")

	// Check if it's a Uint8Array
	isUint8Array, _ := val.InstanceOf(uint8ArrayType)
	if isUint8Array {
		length, err := val.Length()
		if err != nil {
			return nil
		}
		buf := make([]byte, length)
		// Copy bytes - we need to iterate since safejs doesn't have CopyBytesToGo
		for i := 0; i < length; i++ {
			elem, err := val.Index(i)
			if err != nil {
				return nil
			}
			intVal, err := elem.Int()
			if err != nil {
				return nil
			}
			buf[i] = byte(intVal)
		}
		return buf
	}

	// Check if it's an ArrayBuffer
	isArrayBuffer, _ := val.InstanceOf(arrayBufferType)
	if isArrayBuffer {
		// Create a Uint8Array view of the ArrayBuffer
		uint8Array, err := uint8ArrayType.New(val)
		if err != nil {
			return nil
		}
		return safeValueToBytes(uint8Array)
	}

	// Try to treat it as a typed array-like object
	length, err := val.Length()
	if err == nil && length > 0 {
		buf := make([]byte, length)
		for i := 0; i < length; i++ {
			elem, err := val.Index(i)
			if err != nil {
				return nil
			}
			intVal, err := elem.Int()
			if err != nil {
				return nil
			}
			buf[i] = byte(intVal)
		}
		return buf
	}

	// Last resort: check if it's a string (for string keys in IndexedDB)
	if val.Type() == safejs.TypeString {
		str, err := val.String()
		if err == nil {
			return []byte(str)
		}
	}

	return nil
}

// bytesToSafeValue converts a []byte to a safejs.Value (Uint8Array)
func bytesToSafeValue(buf []byte) safejs.Value {
	if buf == nil {
		return safejs.Null()
	}

	uint8Array := js.Global().Get("Uint8Array").New(len(buf))
	js.CopyBytesToJS(uint8Array, buf)
	return safejs.Safe(uint8Array)
}

// cryptoRandom fills the provided byte slice with cryptographically secure random bytes
// using the Web Crypto API (crypto.getRandomValues) or Node.js crypto.randomFillSync
func cryptoRandom(buf []byte) error {
	if len(buf) == 0 {
		return nil
	}

	// First try browser's crypto.getRandomValues
	crypto := js.Global().Get("crypto")
	if crypto.IsUndefined() {
		// Fallback to msCrypto for older IE
		crypto = js.Global().Get("msCrypto")
	}

	if !crypto.IsUndefined() {
		// Try getRandomValues (browser API)
		getRandomValues := crypto.Get("getRandomValues")
		if !getRandomValues.IsUndefined() && getRandomValues.Type() == js.TypeFunction {
			// Create a Uint8Array to receive random bytes
			uint8Array := js.Global().Get("Uint8Array").New(len(buf))

			// Call crypto.getRandomValues - may throw in Node.js
			defer func() {
				// Recover from panic if this method doesn't work
				recover()
			}()
			getRandomValues.Invoke(uint8Array)

			// Copy the random bytes to our Go slice
			js.CopyBytesToGo(buf, uint8Array)
			return nil
		}

		// Try randomFillSync (Node.js API)
		randomFillSync := crypto.Get("randomFillSync")
		if !randomFillSync.IsUndefined() && randomFillSync.Type() == js.TypeFunction {
			uint8Array := js.Global().Get("Uint8Array").New(len(buf))
			randomFillSync.Invoke(uint8Array)
			js.CopyBytesToGo(buf, uint8Array)
			return nil
		}
	}

	// Try to load Node.js crypto module via require
	requireFunc := js.Global().Get("require")
	if !requireFunc.IsUndefined() && requireFunc.Type() == js.TypeFunction {
		nodeCrypto := requireFunc.Invoke("crypto")
		if !nodeCrypto.IsUndefined() {
			randomFillSync := nodeCrypto.Get("randomFillSync")
			if !randomFillSync.IsUndefined() && randomFillSync.Type() == js.TypeFunction {
				uint8Array := js.Global().Get("Uint8Array").New(len(buf))
				randomFillSync.Invoke(uint8Array)
				js.CopyBytesToGo(buf, uint8Array)
				return nil
			}
		}
	}

	return errNoCryptoAPI
}

// errNoCryptoAPI is returned when the Web Crypto API is not available
type cryptoAPIError struct{}

func (cryptoAPIError) Error() string { return "Web Crypto API not available" }

var errNoCryptoAPI = cryptoAPIError{}
