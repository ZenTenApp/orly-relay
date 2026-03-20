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
	"time"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/nostr/encoders/event"
)

const maxLen = 500000000

// ImportEventsFromReader imports events from an io.Reader containing JSONL data
func (d *D) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	startTime := time.Now()
	log.I.F("import: starting import operation")

	// store to disk so we can return fast
	tmpPath := os.TempDir() + string(os.PathSeparator) + "orly"
	os.MkdirAll(tmpPath, 0700)
	tmp, err := os.CreateTemp(tmpPath, "")
	if chk.E(err) {
		return err
	}
	defer os.Remove(tmp.Name()) // Clean up temp file when done

	log.I.F("import: buffering upload to %s", tmp.Name())
	bufferStart := time.Now()
	bytesBuffered, err := io.Copy(tmp, rr)
	if chk.E(err) {
		return err
	}
	bufferElapsed := time.Since(bufferStart)
	log.I.F("import: buffered %.2f MB in %v (%.2f MB/sec)",
		float64(bytesBuffered)/1024/1024, bufferElapsed.Round(time.Millisecond),
		float64(bytesBuffered)/bufferElapsed.Seconds()/1024/1024)

	if _, err = tmp.Seek(0, 0); chk.E(err) {
		return err
	}

	processErr := d.processJSONLEvents(ctx, tmp)

	totalElapsed := time.Since(startTime)
	log.I.F("import: total operation time: %v", totalElapsed.Round(time.Millisecond))

	return processErr
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

	// Performance tracking
	startTime := time.Now()
	lastLogTime := startTime
	const logInterval = 5 * time.Second

	var count, total, skipped, policyRejected, unmarshalErrors, saveErrors int
	for scan.Scan() {
		select {
		case <-ctx.Done():
			log.I.F("import: context closed after %d events", count)
			return ctx.Err()
		default:
		}

		b := scan.Bytes()
		total += len(b) + 1
		if len(b) < 1 {
			skipped++
			continue
		}

		ev := event.New()
		if _, err := ev.Unmarshal(b); err != nil {
			// return the pooled buffer on error
			ev.Free()
			unmarshalErrors++
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
				policyRejected++
				continue
			}
			if !allowed {
				log.D.F("policy rejected event %x during sync import", ev.ID)
				ev.Free()
				policyRejected++
				continue
			}
			log.D.F("policy allowed event %x during sync import", ev.ID)
		}

		// Apply rate limiting before write operation if limiter is configured
		if d.rateLimiter != nil && d.rateLimiter.IsEnabled() {
			if _, err := d.rateLimiter.Wait(ctx, WriteOpType); err != nil {
				ev.Free()
				saveErrors++
				continue
			}
		}

		if _, err := d.SaveEvent(ctx, ev); err != nil {
			// return the pooled buffer on error paths too
			ev.Free()
			saveErrors++
			log.W.F("failed to save event: %v", err)
			continue
		}

		// return the pooled buffer after successful save
		ev.Free()
		b = nil
		count++

		// Progress logging every logInterval
		if time.Since(lastLogTime) >= logInterval {
			elapsed := time.Since(startTime)
			eventsPerSec := float64(count) / elapsed.Seconds()
			mbPerSec := float64(total) / elapsed.Seconds() / 1024 / 1024
			log.I.F("import: progress %d events saved, %.2f MB read, %.0f events/sec, %.2f MB/sec",
				count, float64(total)/1024/1024, eventsPerSec, mbPerSec)
			lastLogTime = time.Now()
			debug.FreeOSMemory()
		}
	}

	// Final summary
	elapsed := time.Since(startTime)
	eventsPerSec := float64(count) / elapsed.Seconds()
	mbPerSec := float64(total) / elapsed.Seconds() / 1024 / 1024
	log.I.F("import: completed - %d events saved, %.2f MB in %v (%.0f events/sec, %.2f MB/sec)",
		count, float64(total)/1024/1024, elapsed.Round(time.Millisecond), eventsPerSec, mbPerSec)
	if unmarshalErrors > 0 || saveErrors > 0 || policyRejected > 0 || skipped > 0 {
		log.I.F("import: stats - %d unmarshal errors, %d save errors, %d policy rejected, %d skipped empty lines",
			unmarshalErrors, saveErrors, policyRejected, skipped)
	}

	if err := scan.Err(); err != nil {
		return err
	}

	return nil
}
