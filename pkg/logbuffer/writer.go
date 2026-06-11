package logbuffer

import (
	"bytes"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// --- Actor request/response types ---

type bwWriteReq struct {
	data []byte
	resp chan struct{}
}

// BufferedWriter wraps an io.Writer and captures log entries.
// The lineBuf state is owned by the actor goroutine.
type BufferedWriter struct {
	original io.Writer
	buffer   *Buffer
	writeCh  chan bwWriteReq
	stop     chan struct{}
	done     chan struct{}
}

// Log format regex patterns
var lolPattern = regexp.MustCompile(`^(\d{16})([☠️🚨⚠️ℹ️🔎👻]+)\s*(.*?)\s+([^\s]+:\d+)$`)
var simplePattern = regexp.MustCompile(`^(\d{13,16})\s*(.*)$`)

// NewBufferedWriter creates a new BufferedWriter
func NewBufferedWriter(original io.Writer, buffer *Buffer) *BufferedWriter {
	w := &BufferedWriter{
		original: original,
		buffer:   buffer,
		writeCh:  make(chan bwWriteReq, 128),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go w.actorLoop()
	return w
}

func (w *BufferedWriter) actorLoop() {
	defer close(w.done)

	var lineBuf bytes.Buffer

	for {
		select {
		case <-w.stop:
			return
		case req := <-w.writeCh:
			lineBuf.Write(req.data)

			for {
				line, lineErr := lineBuf.ReadString('\n')
				if lineErr != nil {
					if len(line) > 0 {
						lineBuf.WriteString(line)
					}
					break
				}

				entry := parseLine(strings.TrimSuffix(line, "\n"))
				if entry.Message != "" && w.buffer != nil {
					w.buffer.Add(entry)
				}
			}
			req.resp <- struct{}{}
		}
	}
}

// Shutdown stops the actor goroutine.
func (w *BufferedWriter) Shutdown() {
	close(w.stop)
	<-w.done
}

// Write implements io.Writer
func (w *BufferedWriter) Write(p []byte) (n int, err error) {
	// Always write to original first
	n, err = w.original.Write(p)

	// Store in buffer if we have one
	if w.buffer != nil {
		data := make([]byte, len(p))
		copy(data, p)
		req := bwWriteReq{data: data, resp: make(chan struct{}, 1)}
		w.writeCh <- req
		<-req.resp
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
func parseLine(line string) LogEntry {
	entry := LogEntry{
		Timestamp: time.Now(),
		Message:   line,
		Level:     "INF",
	}

	line = strings.TrimSpace(line)
	if line == "" {
		return entry
	}

	if matches := lolPattern.FindStringSubmatch(line); matches != nil {
		if usec, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			entry.Timestamp = time.UnixMicro(usec)
		}
		if level, ok := emojiToLevel[matches[2]]; ok {
			entry.Level = level
		}
		entry.Message = strings.TrimSpace(matches[3])
		loc := matches[4]
		if idx := strings.LastIndex(loc, ":"); idx > 0 {
			entry.File = loc[:idx]
			if lineNum, err := strconv.Atoi(loc[idx+1:]); err == nil {
				entry.Line = lineNum
			}
		}
		return entry
	}

	if matches := simplePattern.FindStringSubmatch(line); matches != nil {
		if usec, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			if len(matches[1]) >= 16 {
				entry.Timestamp = time.UnixMicro(usec)
			} else {
				entry.Timestamp = time.UnixMilli(usec)
			}
		}
		rest := strings.TrimSpace(matches[2])
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
	switch level {
	case "off", "fatal", "error", "warn", "info", "debug", "trace":
		currentLevel = level
	default:
		currentLevel = "info"
	}
	return currentLevel
}
