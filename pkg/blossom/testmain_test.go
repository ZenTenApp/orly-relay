package blossom

import (
	"io"
	"os"
	"testing"

	"next.orly.dev/pkg/lol"
	"next.orly.dev/pkg/lol/log"
)

func TestMain(m *testing.M) {
	// Disable all logging during tests unless explicitly enabled
	if os.Getenv("TEST_LOG") == "" {
		// Set log level to Off to suppress all logs
		lol.SetLogLevel("off")
		// Also redirect output to discard
		lol.Writer = io.Discard
		// Disable all log printers
		log.T = lol.GetNullPrinter()
		log.D = lol.GetNullPrinter()
		log.I = lol.GetNullPrinter()
		log.W = lol.GetNullPrinter()
		log.E = lol.GetNullPrinter()
		log.F = lol.GetNullPrinter()
	}

	// Run tests
	os.Exit(m.Run())
}
