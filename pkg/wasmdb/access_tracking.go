//go:build js && wasm

package wasmdb

// RecordEventAccess is a stub for WasmDB.
// Access tracking is not implemented for the WebAssembly backend as it's
// primarily used for client-side applications where storage management
// is handled differently.
func (w *W) RecordEventAccess(serial uint64, connectionID string) error {
	// No-op for WasmDB
	return nil
}

// GetEventAccessInfo is a stub for WasmDB.
func (w *W) GetEventAccessInfo(serial uint64) (lastAccess int64, accessCount uint32, err error) {
	// No-op for WasmDB - return zeros
	return 0, 0, nil
}

// GetLeastAccessedEvents is a stub for WasmDB.
func (w *W) GetLeastAccessedEvents(limit int, minAgeSec int64) (serials []uint64, err error) {
	// No-op for WasmDB - return empty slice
	return nil, nil
}
