//go:build !(js && wasm)

package bbolt

import (
	"fmt"
	"runtime"
	"strings"

	"go.uber.org/atomic"
	"lol.mleku.dev"
	"lol.mleku.dev/log"
)

// Logger wraps the lol logger for BBolt
type Logger struct {
	Level atomic.Int32
	Label string
}

// NewLogger creates a new Logger instance
func NewLogger(level int, dataDir string) *Logger {
	l := &Logger{Label: "bbolt"}
	l.Level.Store(int32(level))
	return l
}

// SetLogLevel updates the log level
func (l *Logger) SetLogLevel(level int) {
	l.Level.Store(int32(level))
}

// Tracef logs a trace message
func (l *Logger) Tracef(format string, args ...interface{}) {
	if l.Level.Load() >= int32(lol.Trace) {
		s := l.Label + ": " + format
		txt := fmt.Sprintf(s, args...)
		_, file, line, _ := runtime.Caller(2)
		log.T.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Debugf logs a debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	if l.Level.Load() >= int32(lol.Debug) {
		s := l.Label + ": " + format
		txt := fmt.Sprintf(s, args...)
		_, file, line, _ := runtime.Caller(2)
		log.D.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Infof logs an info message
func (l *Logger) Infof(format string, args ...interface{}) {
	if l.Level.Load() >= int32(lol.Info) {
		s := l.Label + ": " + format
		txt := fmt.Sprintf(s, args...)
		_, file, line, _ := runtime.Caller(2)
		log.I.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Warningf logs a warning message
func (l *Logger) Warningf(format string, args ...interface{}) {
	if l.Level.Load() >= int32(lol.Warn) {
		s := l.Label + ": " + format
		txt := fmt.Sprintf(s, args...)
		_, file, line, _ := runtime.Caller(2)
		log.W.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Errorf logs an error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	if l.Level.Load() >= int32(lol.Error) {
		s := l.Label + ": " + format
		txt := fmt.Sprintf(s, args...)
		_, file, line, _ := runtime.Caller(2)
		log.E.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}
