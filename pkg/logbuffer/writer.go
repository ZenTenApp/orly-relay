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
// lol library format: "1703500000000000ℹ️ message /path/to/file.go:123"
// - Unix microseconds timestamp
// - Level emoji (☠️, 🚨, ⚠️, ℹ️, 🔎, 👻)
// - Message
// - File:line location
var lolPattern = regexp.MustCompile(`^(\d{16})([☠️🚨⚠️ℹ️🔎👻]+)\s*(.*?)\s+([^\s]+:\d+)$`)

// Simpler pattern for when emoji detection fails - just capture timestamp and rest
var simplePattern = regexp.MustCompile(`^(\d{13,16})\s*(.*)$`)

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

// emojiToLevel maps lol library level emojis to level strings
var emojiToLevel = map[string]string{
	"☠️":  "FTL",
	"🚨":  "ERR",
	"⚠️":  "WRN",
	"ℹ️":  "INF",
	"🔎":  "DBG",
	"👻":  "TRC",
}

// parseLine parses a log line into a LogEntry
func (w *BufferedWriter) parseLine(line string) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now(),
		Message:   line,
		Level:     "INF",
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return entry
	}

	// Try lol pattern first: "1703500000000000ℹ️ message /path/to/file.go:123"
	if matches := lolPattern.FindStringSubmatch(line); matches != nil {
		// Parse Unix microseconds timestamp
		if usec, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			entry.Timestamp = time.UnixMicro(usec)
		}

		// Map emoji to level
		if level, ok := emojiToLevel[matches[2]]; ok {
			entry.Level = level
		}

		entry.Message = strings.TrimSpace(matches[3])

		// Parse file:line
		loc := matches[4]
		if idx := strings.LastIndex(loc, ":"); idx > 0 {
			entry.File = loc[:idx]
			if lineNum, err := strconv.Atoi(loc[idx+1:]); err == nil {
				entry.Line = lineNum
			}
		}
		return entry
	}

	// Try simple pattern - just grab timestamp and rest as message
	if matches := simplePattern.FindStringSubmatch(line); matches != nil {
		if usec, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			// Could be microseconds or milliseconds
			if len(matches[1]) >= 16 {
				entry.Timestamp = time.UnixMicro(usec)
			} else {
				entry.Timestamp = time.UnixMilli(usec)
			}
		}
		rest := strings.TrimSpace(matches[2])

		// Try to detect level from emoji in the rest
		for emoji, level := range emojiToLevel {
			if strings.HasPrefix(rest, emoji) {
				entry.Level = level
				rest = strings.TrimPrefix(rest, emoji)
				rest = strings.TrimSpace(rest)
				break
			}
		}

		entry.Message = rest
		return entry
	}

	// Fallback: just store the whole line as message
	entry.Message = line
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

