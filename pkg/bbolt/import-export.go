//go:build !(js && wasm)

package bbolt

import (
	"bufio"
	"context"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"

	"git.mleku.dev/mleku/nostr/encoders/event"
)

const maxLen = 500000000

// ImportEventsFromReader imports events from an io.Reader containing JSONL data
func (b *B) ImportEventsFromReader(ctx context.Context, rr io.Reader) error {
	startTime := time.Now()
	log.I.F("bbolt import: starting import operation")

	// Store to disk so we can return fast
	tmpPath := os.TempDir() + string(os.PathSeparator) + "orly"
	os.MkdirAll(tmpPath, 0700)
	tmp, err := os.CreateTemp(tmpPath, "")
	if chk.E(err) {
		return err
	}
	defer os.Remove(tmp.Name()) // Clean up temp file when done

	log.I.F("bbolt import: buffering upload to %s", tmp.Name())
	bufferStart := time.Now()
	bytesBuffered, err := io.Copy(tmp, rr)
	if chk.E(err) {
		return err
	}
	bufferElapsed := time.Since(bufferStart)
	log.I.F("bbolt import: buffered %.2f MB in %v (%.2f MB/sec)",
		float64(bytesBuffered)/1024/1024, bufferElapsed.Round(time.Millisecond),
		float64(bytesBuffered)/bufferElapsed.Seconds()/1024/1024)

	if _, err = tmp.Seek(0, 0); chk.E(err) {
		return err
	}

	processErr := b.processJSONLEvents(ctx, tmp)

	totalElapsed := time.Since(startTime)
	log.I.F("bbolt import: total operation time: %v", totalElapsed.Round(time.Millisecond))

	return processErr
}

// ImportEventsFromStrings imports events from a slice of JSON strings with policy filtering
func (b *B) ImportEventsFromStrings(ctx context.Context, eventJSONs []string, policyManager interface {
	CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error)
}) error {
	// Create a reader from the string slice
	reader := strings.NewReader(strings.Join(eventJSONs, "\n"))
	return b.processJSONLEventsWithPolicy(ctx, reader, policyManager)
}

// processJSONLEvents processes JSONL events from a reader
func (b *B) processJSONLEvents(ctx context.Context, rr io.Reader) error {
	return b.processJSONLEventsWithPolicy(ctx, rr, nil)
}

// processJSONLEventsWithPolicy processes JSONL events from a reader with optional policy filtering
func (b *B) processJSONLEventsWithPolicy(ctx context.Context, rr io.Reader, policyManager interface {
	CheckPolicy(action string, ev *event.E, pubkey []byte, remote string) (bool, error)
}) error {
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
			log.I.F("bbolt import: context closed after %d events", count)
			return ctx.Err()
		default:
		}

		line := scan.Bytes()
		total += len(line) + 1
		if len(line) < 1 {
			skipped++
			continue
		}

		ev := event.New()
		if _, err := ev.Unmarshal(line); err != nil {
			// return the pooled buffer on error
			ev.Free()
			unmarshalErrors++
			log.W.F("bbolt import: failed to unmarshal event: %v", err)
			continue
		}

		// Apply policy checking if policy manager is provided
		if policyManager != nil {
			// For sync imports, we treat events as coming from system/trusted source
			// Use nil pubkey and empty remote to indicate system-level import
			allowed, policyErr := policyManager.CheckPolicy("write", ev, nil, "")
			if policyErr != nil {
				log.W.F("bbolt import: policy check failed for event %x: %v", ev.ID, policyErr)
				ev.Free()
				policyRejected++
				continue
			}
			if !allowed {
				log.D.F("bbolt import: policy rejected event %x during sync import", ev.ID)
				ev.Free()
				policyRejected++
				continue
			}
			log.D.F("bbolt import: policy allowed event %x during sync import", ev.ID)
		}

		if _, err := b.SaveEvent(ctx, ev); err != nil {
			// return the pooled buffer on error paths too
			ev.Free()
			saveErrors++
			log.W.F("bbolt import: failed to save event: %v", err)
			continue
		}

		// return the pooled buffer after successful save
		ev.Free()
		line = nil
		count++

		// Progress logging every logInterval
		if time.Since(lastLogTime) >= logInterval {
			elapsed := time.Since(startTime)
			eventsPerSec := float64(count) / elapsed.Seconds()
			mbPerSec := float64(total) / elapsed.Seconds() / 1024 / 1024
			log.I.F("bbolt import: progress %d events saved, %.2f MB read, %.0f events/sec, %.2f MB/sec",
				count, float64(total)/1024/1024, eventsPerSec, mbPerSec)
			lastLogTime = time.Now()
			debug.FreeOSMemory()
		}
	}

	// Flush any remaining batched events
	if b.batcher != nil {
		b.batcher.Flush()
	}

	// Final summary
	elapsed := time.Since(startTime)
	eventsPerSec := float64(count) / elapsed.Seconds()
	mbPerSec := float64(total) / elapsed.Seconds() / 1024 / 1024
	log.I.F("bbolt import: completed - %d events saved, %.2f MB in %v (%.0f events/sec, %.2f MB/sec)",
		count, float64(total)/1024/1024, elapsed.Round(time.Millisecond), eventsPerSec, mbPerSec)
	if unmarshalErrors > 0 || saveErrors > 0 || policyRejected > 0 || skipped > 0 {
		log.I.F("bbolt import: stats - %d unmarshal errors, %d save errors, %d policy rejected, %d skipped empty lines",
			unmarshalErrors, saveErrors, policyRejected, skipped)
	}

	if err := scan.Err(); err != nil {
		return err
	}

	return nil
}

// Import imports events from a reader (legacy interface).
func (b *B) Import(rr io.Reader) {
	ctx := context.Background()
	if err := b.ImportEventsFromReader(ctx, rr); err != nil {
		log.E.F("bbolt import: error: %v", err)
	}
}

// Export exports events to a writer.
func (b *B) Export(c context.Context, w io.Writer, pubkeys ...[]byte) {
	// TODO: Implement export functionality
	log.W.F("bbolt export: not yet implemented")
}
