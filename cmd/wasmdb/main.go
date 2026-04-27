//go:build js && wasm

// Package main provides the WASM entry point for the WasmDB database.
// It initializes the IndexedDB-backed Nostr event store and exposes
// the database API to JavaScript via the global `wasmdb` object.
//
// Build with:
//   GOOS=js GOARCH=wasm go build -o wasmdb.wasm ./cmd/wasmdb
//
// Usage in JavaScript:
//   // Load wasm_exec.js first (Go WASM runtime)
//   const go = new Go();
//   const result = await WebAssembly.instantiateStreaming(fetch('wasmdb.wasm'), go.importObject);
//   go.run(result.instance);
//
//   // Wait for ready
//   while (!wasmdb.isReady()) {
//     await new Promise(resolve => setTimeout(resolve, 100));
//   }
//
//   // Use the API
//   await wasmdb.saveEvent(JSON.stringify(event));
//   const events = await wasmdb.queryEvents(JSON.stringify({kinds: [1], limit: 10}));
package main

import (
	"context"
	"fmt"

	"git.smesh.lol/orly/pkg/database"
	"git.smesh.lol/orly/pkg/wasmdb"
)

func main() {
	// Create context for the database
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the database with default config
	cfg := &database.DatabaseConfig{
		DataDir:  ".",
		LogLevel: "info",
	}

	db, err := wasmdb.NewWithConfig(ctx, cancel, cfg)
	if err != nil {
		fmt.Printf("Failed to initialize WasmDB: %v\n", err)
		return
	}

	// Register the JavaScript bridge
	wasmdb.RegisterJSBridge(db, ctx, cancel)

	fmt.Println("WasmDB initialized and JavaScript bridge registered")

	// Wait for the database to be ready
	<-db.Ready()
	fmt.Println("WasmDB ready to serve requests")

	// Keep the WASM module running
	// This is necessary because Go's main() returning would terminate the WASM instance
	select {}
}
