//go:build !(js && wasm)

package database

import (
	"io"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// Import a collection of events in line structured minified JSON format (JSONL).
// This runs synchronously to ensure the reader remains valid during processing.
// The actual event processing happens after buffering to a temp file, so the
// caller can close the reader after Import returns.
func (d *D) Import(rr io.Reader) {
	if err := d.ImportEventsFromReader(d.ctx, rr); chk.E(err) {
		log.E.F("import failed: %v", err)
	}
}
