//go:build js && wasm

package wasmdb

import (
	"fmt"
	"syscall/js"
	"time"

	"lol.mleku.dev"
)

// logger provides logging functionality for the wasmdb package
// It outputs to the browser console via console.log/warn/error
type logger struct {
	level int
}

// NewLogger creates a new logger with the specified level
func NewLogger(level int) *logger {
	return &logger{level: level}
}

// SetLogLevel changes the logging level
func (l *logger) SetLogLevel(level int) {
	l.level = level
}

// formatMessage creates a formatted log message with timestamp
func (l *logger) formatMessage(level, format string, args ...interface{}) string {
	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("[%s] [wasmdb] [%s] %s",
		time.Now().Format("15:04:05.000"),
		level,
		msg,
	)
}

// Debugf logs a debug message
func (l *logger) Debugf(format string, args ...interface{}) {
	if l.level <= lol.Debug {
		msg := l.formatMessage("DEBUG", format, args...)
		js.Global().Get("console").Call("log", msg)
	}
}

// Infof logs an info message
func (l *logger) Infof(format string, args ...interface{}) {
	if l.level <= lol.Info {
		msg := l.formatMessage("INFO", format, args...)
		js.Global().Get("console").Call("log", msg)
	}
}

// Warnf logs a warning message
func (l *logger) Warnf(format string, args ...interface{}) {
	if l.level <= lol.Warn {
		msg := l.formatMessage("WARN", format, args...)
		js.Global().Get("console").Call("warn", msg)
	}
}

// Errorf logs an error message
func (l *logger) Errorf(format string, args ...interface{}) {
	if l.level <= lol.Error {
		msg := l.formatMessage("ERROR", format, args...)
		js.Global().Get("console").Call("error", msg)
	}
}

// Fatalf logs a fatal message (does not exit in WASM)
func (l *logger) Fatalf(format string, args ...interface{}) {
	msg := l.formatMessage("FATAL", format, args...)
	js.Global().Get("console").Call("error", msg)
}
