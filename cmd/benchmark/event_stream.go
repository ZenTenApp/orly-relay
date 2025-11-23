package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
)

// EventStream manages disk-based event generation to avoid memory bloat
type EventStream struct {
	baseDir   string
	count     int
	chunkSize int
	rng       *rand.Rand
}

// NewEventStream creates a new event stream that stores events on disk
func NewEventStream(baseDir string, count int) (*EventStream, error) {
	// Create events directory
	eventsDir := filepath.Join(baseDir, "events")
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}

	return &EventStream{
		baseDir:   eventsDir,
		count:     count,
		chunkSize: 1000, // Store 1000 events per file to balance I/O
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

// Generate creates all events and stores them in chunk files
func (es *EventStream) Generate() error {
	numChunks := (es.count + es.chunkSize - 1) / es.chunkSize

	for chunk := 0; chunk < numChunks; chunk++ {
		chunkFile := filepath.Join(es.baseDir, fmt.Sprintf("chunk_%04d.jsonl", chunk))
		f, err := os.Create(chunkFile)
		if err != nil {
			return fmt.Errorf("failed to create chunk file %s: %w", chunkFile, err)
		}

		writer := bufio.NewWriter(f)
		startIdx := chunk * es.chunkSize
		endIdx := min(startIdx+es.chunkSize, es.count)

		for i := startIdx; i < endIdx; i++ {
			ev, err := es.generateEvent(i)
			if err != nil {
				f.Close()
				return fmt.Errorf("failed to generate event %d: %w", i, err)
			}

			// Marshal event to JSON
			eventJSON, err := json.Marshal(ev)
			if err != nil {
				f.Close()
				return fmt.Errorf("failed to marshal event %d: %w", i, err)
			}

			// Write JSON line
			if _, err := writer.Write(eventJSON); err != nil {
				f.Close()
				return fmt.Errorf("failed to write event %d: %w", i, err)
			}
			if _, err := writer.WriteString("\n"); err != nil {
				f.Close()
				return fmt.Errorf("failed to write newline after event %d: %w", i, err)
			}
		}

		if err := writer.Flush(); err != nil {
			f.Close()
			return fmt.Errorf("failed to flush chunk file %s: %w", chunkFile, err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close chunk file %s: %w", chunkFile, err)
		}

		if (chunk+1)%10 == 0 || chunk == numChunks-1 {
			fmt.Printf("  Generated %d/%d events (%.1f%%)\n",
				endIdx, es.count, float64(endIdx)/float64(es.count)*100)
		}
	}

	return nil
}

// generateEvent creates a single event with realistic size distribution
func (es *EventStream) generateEvent(index int) (*event.E, error) {
	// Create signer for this event
	keys, err := p8k.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}
	if err := keys.Generate(); err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	ev := event.New()
	ev.Kind = 1 // Text note
	ev.CreatedAt = timestamp.Now().I64()

	// Add some tags for realism
	numTags := es.rng.Intn(5)
	tags := make([]*tag.T, 0, numTags)
	for i := 0; i < numTags; i++ {
		tags = append(tags, tag.NewFromBytesSlice(
			[]byte("t"),
			[]byte(fmt.Sprintf("tag%d", es.rng.Intn(100))),
		))
	}
	ev.Tags = tag.NewS(tags...)

	// Generate content with log-distributed size
	contentSize := es.generateLogDistributedSize()
	ev.Content = []byte(es.generateRandomContent(contentSize))

	// Sign the event
	if err := ev.Sign(keys); err != nil {
		return nil, fmt.Errorf("failed to sign event: %w", err)
	}

	return ev, nil
}

// generateLogDistributedSize generates sizes following a power law distribution
// This creates realistic size distribution:
// - Most events are small (< 1KB)
// - Some events are medium (1-10KB)
// - Few events are large (10-100KB)
func (es *EventStream) generateLogDistributedSize() int {
	// Use power law with exponent 4.0 for strong skew toward small sizes
	const powerExponent = 4.0
	uniform := es.rng.Float64()
	skewed := math.Pow(uniform, powerExponent)

	// Scale to max size of 100KB
	const maxSize = 100 * 1024
	size := int(skewed * maxSize)

	// Ensure minimum size of 10 bytes
	if size < 10 {
		size = 10
	}

	return size
}

// generateRandomContent creates random text content of specified size
func (es *EventStream) generateRandomContent(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 \n"
	content := make([]byte, size)
	for i := range content {
		content[i] = charset[es.rng.Intn(len(charset))]
	}
	return string(content)
}

// GetEventChannel returns a channel that streams events from disk
// bufferSize controls memory usage - larger buffers improve throughput but use more memory
func (es *EventStream) GetEventChannel(bufferSize int) (<-chan *event.E, <-chan error) {
	eventChan := make(chan *event.E, bufferSize)
	errChan := make(chan error, 1)

	go func() {
		defer close(eventChan)
		defer close(errChan)

		numChunks := (es.count + es.chunkSize - 1) / es.chunkSize

		for chunk := 0; chunk < numChunks; chunk++ {
			chunkFile := filepath.Join(es.baseDir, fmt.Sprintf("chunk_%04d.jsonl", chunk))
			f, err := os.Open(chunkFile)
			if err != nil {
				errChan <- fmt.Errorf("failed to open chunk file %s: %w", chunkFile, err)
				return
			}

			scanner := bufio.NewScanner(f)
			// Increase buffer size for large events
			buf := make([]byte, 0, 64*1024)
			scanner.Buffer(buf, 1024*1024) // Max 1MB per line

			for scanner.Scan() {
				var ev event.E
				if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
					f.Close()
					errChan <- fmt.Errorf("failed to unmarshal event: %w", err)
					return
				}
				eventChan <- &ev
			}

			if err := scanner.Err(); err != nil {
				f.Close()
				errChan <- fmt.Errorf("error reading chunk file %s: %w", chunkFile, err)
				return
			}

			f.Close()
		}
	}()

	return eventChan, errChan
}

// ForEach iterates over all events without loading them all into memory
func (es *EventStream) ForEach(fn func(*event.E) error) error {
	numChunks := (es.count + es.chunkSize - 1) / es.chunkSize

	for chunk := 0; chunk < numChunks; chunk++ {
		chunkFile := filepath.Join(es.baseDir, fmt.Sprintf("chunk_%04d.jsonl", chunk))
		f, err := os.Open(chunkFile)
		if err != nil {
			return fmt.Errorf("failed to open chunk file %s: %w", chunkFile, err)
		}

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			var ev event.E
			if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
				f.Close()
				return fmt.Errorf("failed to unmarshal event: %w", err)
			}

			if err := fn(&ev); err != nil {
				f.Close()
				return err
			}
		}

		if err := scanner.Err(); err != nil {
			f.Close()
			return fmt.Errorf("error reading chunk file %s: %w", chunkFile, err)
		}

		f.Close()
	}

	return nil
}
