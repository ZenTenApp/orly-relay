//go:build !(js && wasm)

package database

import (
	"io"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// Import a collection of events in line structured minified JSON format (JSONL).
func (d *D) Import(rr io.Reader) {
	go func() {
		if err := d.ImportEventsFromReader(d.ctx, rr); chk.E(err) {
			log.E.F("import failed: %v", err)
		}
	}()
}
