package neo4j

import (
	"fmt"
	"runtime"
	"strings"

	"go.uber.org/atomic"
	"lol.mleku.dev"
	"lol.mleku.dev/log"
)

// NewLogger creates a new dgraph logger.
func NewLogger(logLevel int, label string) (l *logger) {
	l = &logger{Label: label}
	l.Level.Store(int32(logLevel))
	return
}

type logger struct {
	Level atomic.Int32
	Label string
}

// SetLogLevel atomically adjusts the log level to the given log level code.
func (l *logger) SetLogLevel(level int) {
	l.Level.Store(int32(level))
}

// Errorf is a log printer for this level of message.
func (l *logger) Errorf(s string, i ...interface{}) {
	if l.Level.Load() >= lol.Error {
		s = l.Label + ": " + s
		txt := fmt.Sprintf(s, i...)
		_, file, line, _ := runtime.Caller(2)
		log.E.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Warningf is a log printer for this level of message.
func (l *logger) Warningf(s string, i ...interface{}) {
	if l.Level.Load() >= lol.Warn {
		s = l.Label + ": " + s
		txt := fmt.Sprintf(s, i...)
		_, file, line, _ := runtime.Caller(2)
		log.W.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Infof is a log printer for this level of message.
func (l *logger) Infof(s string, i ...interface{}) {
	if l.Level.Load() >= lol.Info {
		s = l.Label + ": " + s
		txt := fmt.Sprintf(s, i...)
		_, file, line, _ := runtime.Caller(2)
		log.I.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}

// Debugf is a log printer for this level of message.
func (l *logger) Debugf(s string, i ...interface{}) {
	if l.Level.Load() >= lol.Debug {
		s = l.Label + ": " + s
		txt := fmt.Sprintf(s, i...)
		_, file, line, _ := runtime.Caller(2)
		log.D.F("%s\n%s:%d", strings.TrimSpace(txt), file, line)
	}
}
