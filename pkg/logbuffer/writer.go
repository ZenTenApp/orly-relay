package logbuffer

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BufferedWriter wraps an io.Writer and captures log entries
type BufferedWriter struct {
	original io.Writer
	buffer   *Buffer
	lineBuf  bytes.Buffer
}

// Log format regex patterns
// lol library format: "2024/01/15 10:30:45 file.go:123 [INF] message"
// or similar variations
var logPattern = regexp.MustCompile(`^(\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}(?:\.\d+)?)\s+([^\s:]+):(\d+)\s+\[([A-Z]{3})\]\s+(.*)$`)

// Simple format: "[level] message"
var simplePattern = regexp.MustCompile(`^\[([A-Z]{3})\]\s+(.*)$`)

// NewBufferedWriter creates a new BufferedWriter
func NewBufferedWriter(original io.Writer, buffer *Buffer) *BufferedWriter {
	return &BufferedWriter{
		original: original,
		buffer:   buffer,
	}
}

// Write implements io.Writer
func (w *BufferedWriter) Write(p []byte) (n int, err error) {
	// Always write to original first
	n, err = w.original.Write(p)

	// Store in buffer if we have one
	if w.buffer != nil {
		// Accumulate data in line buffer
		w.lineBuf.Write(p)

		// Process complete lines
		for {
			line, lineErr := w.lineBuf.ReadString('\n')
			if lineErr != nil {
				// Put back incomplete line
				if len(line) > 0 {
					w.lineBuf.WriteString(line)
				}
				break
			}

			// Parse and store the complete line
			entry := w.parseLine(strings.TrimSuffix(line, "\n"))
			if entry.Message != "" {
				w.buffer.Add(entry)
			}
		}
	}

	return
}

// parseLine parses a log line into a LogEntry
func (w *BufferedWriter) parseLine(line string) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now(),
		Message:   line,
		Level:     "INF",
	}

	// Try full pattern first
	if matches := logPattern.FindStringSubmatch(line); matches != nil {
		// Parse timestamp
		if t, err := time.Parse("2006/01/02 15:04:05", matches[1]); err == nil {
			entry.Timestamp = t
		} else if t, err := time.Parse("2006/01/02 15:04:05.000", matches[1]); err == nil {
			entry.Timestamp = t
		}

		entry.File = matches[2]
		if lineNum, err := strconv.Atoi(matches[3]); err == nil {
			entry.Line = lineNum
		}
		entry.Level = matches[4]
		entry.Message = matches[5]
		return entry
	}

	// Try simple pattern
	if matches := simplePattern.FindStringSubmatch(line); matches != nil {
		entry.Level = matches[1]
		entry.Message = matches[2]
		return entry
	}

	// Detect level from common prefixes
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "TRC") || strings.HasPrefix(line, "[TRC]") {
		entry.Level = "TRC"
	} else if strings.HasPrefix(line, "DBG") || strings.HasPrefix(line, "[DBG]") {
		entry.Level = "DBG"
	} else if strings.HasPrefix(line, "INF") || strings.HasPrefix(line, "[INF]") {
		entry.Level = "INF"
	} else if strings.HasPrefix(line, "WRN") || strings.HasPrefix(line, "[WRN]") {
		entry.Level = "WRN"
	} else if strings.HasPrefix(line, "ERR") || strings.HasPrefix(line, "[ERR]") {
		entry.Level = "ERR"
	} else if strings.HasPrefix(line, "FTL") || strings.HasPrefix(line, "[FTL]") {
		entry.Level = "FTL"
	}

	return entry
}

// currentLevel tracks the current log level (string)
var currentLevel = "info"

// GetCurrentLevel returns the current log level string
func GetCurrentLevel() string {
	return currentLevel
}

// SetCurrentLevel sets the current log level and returns it
func SetCurrentLevel(level string) string {
	level = strings.ToLower(level)
	// Validate level
	switch level {
	case "off", "fatal", "error", "warn", "info", "debug", "trace":
		currentLevel = level
	default:
		currentLevel = "info"
	}
	return currentLevel
}

