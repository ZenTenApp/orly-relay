package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// LatencyRecorder writes latency measurements to disk to avoid memory bloat
type LatencyRecorder struct {
	file   *os.File
	writer *bufio.Writer
	mu     sync.Mutex
	count  int64
}

// LatencyStats contains calculated latency statistics
type LatencyStats struct {
	Avg        time.Duration
	P90        time.Duration
	P95        time.Duration
	P99        time.Duration
	Bottom10   time.Duration
	Count      int64
}

// NewLatencyRecorder creates a new latency recorder that writes to disk
func NewLatencyRecorder(baseDir string, testName string) (*LatencyRecorder, error) {
	latencyFile := filepath.Join(baseDir, fmt.Sprintf("latency_%s.bin", testName))
	f, err := os.Create(latencyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create latency file: %w", err)
	}

	return &LatencyRecorder{
		file:   f,
		writer: bufio.NewWriter(f),
		count:  0,
	}, nil
}

// Record writes a latency measurement to disk (8 bytes per measurement)
func (lr *LatencyRecorder) Record(latency time.Duration) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	// Write latency as 8-byte value (int64 nanoseconds)
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(latency.Nanoseconds()))

	if _, err := lr.writer.Write(buf); err != nil {
		return fmt.Errorf("failed to write latency: %w", err)
	}

	lr.count++
	return nil
}

// Close flushes and closes the latency file
func (lr *LatencyRecorder) Close() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if err := lr.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush latency file: %w", err)
	}

	if err := lr.file.Close(); err != nil {
		return fmt.Errorf("failed to close latency file: %w", err)
	}

	return nil
}

// CalculateStats reads all latencies from disk, sorts them, and calculates statistics
// This is done on-demand to avoid keeping all latencies in memory during the test
func (lr *LatencyRecorder) CalculateStats() (*LatencyStats, error) {
	lr.mu.Lock()
	filePath := lr.file.Name()
	count := lr.count
	lr.mu.Unlock()

	// If no measurements, return zeros
	if count == 0 {
		return &LatencyStats{
			Avg:        0,
			P90:        0,
			P95:        0,
			P99:        0,
			Bottom10:   0,
			Count:      0,
		}, nil
	}

	// Open file for reading
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open latency file for reading: %w", err)
	}
	defer f.Close()

	// Read all latencies into memory temporarily for sorting
	latencies := make([]time.Duration, 0, count)
	buf := make([]byte, 8)
	reader := bufio.NewReader(f)

	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("failed to read latency data: %w", err)
		}
		if n != 8 {
			break
		}

		nanos := binary.LittleEndian.Uint64(buf)
		latencies = append(latencies, time.Duration(nanos))
	}

	// Check if we actually got any latencies
	if len(latencies) == 0 {
		return &LatencyStats{
			Avg:        0,
			P90:        0,
			P95:        0,
			P99:        0,
			Bottom10:   0,
			Count:      0,
		}, nil
	}

	// Sort for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	// Calculate statistics
	stats := &LatencyStats{
		Count: int64(len(latencies)),
	}

	// Average
	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}
	stats.Avg = sum / time.Duration(len(latencies))

	// Percentiles
	stats.P90 = latencies[int(float64(len(latencies))*0.90)]
	stats.P95 = latencies[int(float64(len(latencies))*0.95)]
	stats.P99 = latencies[int(float64(len(latencies))*0.99)]

	// Bottom 10% average
	bottom10Count := int(float64(len(latencies)) * 0.10)
	if bottom10Count > 0 {
		var bottom10Sum time.Duration
		for i := 0; i < bottom10Count; i++ {
			bottom10Sum += latencies[i]
		}
		stats.Bottom10 = bottom10Sum / time.Duration(bottom10Count)
	}

	return stats, nil
}
