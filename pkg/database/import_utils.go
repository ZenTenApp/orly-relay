//go:build !(js && wasm)

// Package database provides shared import utilities for events
package database

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"git.mleku.dev/mleku/nostr/encoders/event"
)

const maxLen = 500000000

// ImportEventsFromReader imports events from an io.Reader containing JSONL data
func (d *D) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	// store to disk so we can return fast
	tmpPath := os.TempDir() + string(os.PathSeparator) + "orly"
	os.MkdirAll(tmpPath, 0700)
	tmp, err := os.CreateTemp(tmpPath, "")
	if chk.E(err) {
		return err
	}
	defer os.Remove(tmp.Name()) // Clean up temp file when done

	log.I.F("buffering upload to %s", tmp.Name())
	if _, err = io.Copy(tmp, rr); chk.E(err) {
		return err
	}
	if _, err = tmp.Seek(0, 0); chk.E(err) {
		return err
	}

	return d.processJSONLEvents(ctx, tmp)
}

// ImportEventsFromStrings imports events from a slice of JSON strings with policy filtering
func (d *D) ImportEventsFromStrings(ctx context.Context, eventJSONs []string, policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) }) error {
	// Create a reader from the string slice
	reader := strings.NewReader(strings.Join(eventJSONs, "\n"))
	return d.processJSONLEventsWithPolicy(ctx, reader, policyManager)
}

// processJSONLEvents processes JSONL events from a reader
func (d *D) processJSONLEvents(ctx context.Context, rr io.Reader) error {
	return d.processJSONLEventsWithPolicy(ctx, rr, nil)
}

// processJSONLEventsWithPolicy processes JSONL events from a reader with optional policy filtering
func (d *D) processJSONLEventsWithPolicy(ctx context.Context, rr io.Reader, policyManager interface{ CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error) }) error {
	// Create a scanner to read the buffer line by line
	scan := bufio.NewScanner(rr)
	scanBuf := make([]byte, maxLen)
	scan.Buffer(scanBuf, maxLen)

	var count, total int
	for scan.Scan() {
		select {
		case <-ctx.Done():
			log.I.F("context closed")
			return ctx.Err()
		default:
		}

		b := scan.Bytes()
		total += len(b) + 1
		if len(b) < 1 {
			continue
		}

		ev := event.New()
		if _, err := ev.Unmarshal(b); err != nil {
			// return the pooled buffer on error
			ev.Free()
			log.W.F("failed to unmarshal event: %v", err)
			continue
		}

		// Apply policy checking if policy manager is provided
		if policyManager != nil {
			// For sync imports, we treat events as coming from system/trusted source
			// Use nil pubkey and empty remote to indicate system-level import
			allowed, policyErr := policyManager.CheckPolicy("write", ev, nil, "")
			if policyErr != nil {
				log.W.F("policy check failed for event %x: %v", ev.ID, policyErr)
				ev.Free()
				continue
			}
			if !allowed {
				log.D.F("policy rejected event %x during sync import", ev.ID)
				ev.Free()
				continue
			}
			log.D.F("policy allowed event %x during sync import", ev.ID)
		}

		if _, err := d.SaveEvent(ctx, ev); err != nil {
			// return the pooled buffer on error paths too
			ev.Free()
			log.W.F("failed to save event: %v", err)
			continue
		}

		// return the pooled buffer after successful save
		ev.Free()
		b = nil
		count++
		if count%100 == 0 {
			log.I.F("processed %d events", count)
			debug.FreeOSMemory()
		}
	}

	log.I.F("read %d bytes and saved %d events", total, count)
	if err := scan.Err(); err != nil {
		return err
	}

	return nil
}
