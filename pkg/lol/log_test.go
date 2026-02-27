package lol

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestLogLevels(t *testing.T) {
	// Test that log levels are correctly ordered
	if !(Off < Fatal && Fatal < Error && Error < Warn && Warn < Info && Info < Debug && Debug < Trace) {
		t.Error("Log levels are not correctly ordered")
	}

	// Test that LevelNames matches the constants
	expectedLevelNames := []string{
		"off", "fatal", "error", "warn", "info", "debug", "trace",
	}
	for i, name := range expectedLevelNames {
		if LevelNames[i] != name {
			t.Errorf("LevelNames[%d] = %s, want %s", i, LevelNames[i], name)
		}
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected int
	}{
		{"off", Off},
		{"fatal", Fatal},
		{"error", Error},
		{"warn", Warn},
		{"info", Info},
		{"debug", Debug},
		{"trace", Trace},
		{"unknown", Info}, // Default to Info for unknown levels
	}

	for _, test := range tests {
		t.Run(
			test.level, func(t *testing.T) {
				result := GetLogLevel(test.level)
				if result != test.expected {
					t.Errorf(
						"GetLogLevel(%q) = %d, want %d", test.level, result,
						test.expected,
					)
				}
			},
		)
	}
}

func TestSetLogLevel(t *testing.T) {
	// Save original level
	originalLevel := Level.Load()
	defer SetLoggers(int(originalLevel)) // Restore original level after test

	tests := []struct {
		level    string
		expected int32
	}{
		{"off", Off},
		{"fatal", Fatal},
		{"error", Error},
		{"warn", Warn},
		{"info", Info},
		{"debug", Debug},
		{"trace", Trace},
		{"unknown", Trace}, // Should default to Trace for unknown levels
	}

	for _, test := range tests {
		t.Run(
			test.level, func(t *testing.T) {
				SetLogLevel(test.level)
				result := Level.Load()
				if result != test.expected {
					t.Errorf(
						"After SetLogLevel(%q), Level = %d, want %d",
						test.level, result, test.expected,
					)
				}
			},
		)
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		args     []any
		expected string
	}{
		{[]any{}, ""},
		{[]any{"hello"}, "hello"},
		{[]any{"hello", "world"}, "hello world"},
		{[]any{1, 2, 3}, "1 2 3"},
		{[]any{1, "hello", 3.14}, "1 hello 3.14"},
	}

	for i, test := range tests {
		t.Run(
			fmt.Sprintf("case_%d", i), func(t *testing.T) {
				result := JoinStrings(test.args...)
				if result != test.expected {
					t.Errorf(
						"JoinStrings(%v) = %q, want %q", test.args, result,
						test.expected,
					)
				}
			},
		)
	}
}

func TestGetLoc(t *testing.T) {
	loc := GetLoc(1)
	if !strings.Contains(loc, "log_test.go") {
		t.Errorf("GetLoc(1) = %q, expected to contain 'log_test.go'", loc)
	}
}

func TestGetPrinter(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	// Set log level to Info
	originalLevel := Level.Load()
	Level.Store(int32(Info))
	defer Level.Store(originalLevel) // Restore original level

	// Create a printer for Debug level
	printer := GetPrinter(int32(Debug), &buf, 1)

	// Test Ln method - should not print because Debug > Info
	buf.Reset()
	printer.Ln("test message")
	if buf.String() != "" {
		t.Errorf(
			"printer.Ln() printed when level is too high: %q", buf.String(),
		)
	}

	// Set log level to Debug
	Level.Store(int32(Debug))

	// Test Ln method - should print now
	buf.Reset()
	printer.Ln("test message")
	output := buf.String()
	if output == "" {
		t.Error("printer.Ln() did not print when it should have")
	}
	if !strings.Contains(output, "test message") {
		t.Errorf(
			"printer.Ln() output %q does not contain 'test message'", output,
		)
	}

	// Test F method
	buf.Reset()
	printer.F("formatted %s", "message")
	output = buf.String()
	if !strings.Contains(output, "formatted message") {
		t.Errorf(
			"printer.F() output %q does not contain 'formatted message'",
			output,
		)
	}

	// Test S method
	buf.Reset()
	printer.S("spew message")
	output = buf.String()
	if !strings.Contains(output, "spew message") {
		t.Errorf(
			"printer.S() output %q does not contain 'spew message'", output,
		)
	}

	// Test C method
	buf.Reset()
	printer.C(func() string { return "closure message" })
	output = buf.String()
	if !strings.Contains(output, "closure message") {
		t.Errorf(
			"printer.C() output %q does not contain 'closure message'", output,
		)
	}

	// Test Chk method with nil error
	buf.Reset()
	result := printer.Chk(nil)
	if result != false {
		t.Error("printer.Chk(nil) returned true, expected false")
	}
	if buf.String() != "" {
		t.Errorf("printer.Chk(nil) printed output: %q", buf.String())
	}

	// Test Chk method with error
	buf.Reset()
	testErr := errors.New("test error")
	result = printer.Chk(testErr)
	if result != true {
		t.Error("printer.Chk(error) returned false, expected true")
	}
	if !strings.Contains(buf.String(), "test error") {
		t.Errorf(
			"printer.Chk(error) output %q does not contain 'test error'",
			buf.String(),
		)
	}

	// Test Err method
	buf.Reset()
	err := printer.Err("error %s", "message")
	if err == nil {
		t.Error("printer.Err() returned nil error")
	}
	if err.Error() != "error message" {
		t.Errorf(
			"printer.Err() returned error with message %q, expected 'error message'",
			err.Error(),
		)
	}
	// Check if the message was logged
	if !strings.Contains(buf.String(), "error message") {
		t.Errorf(
			"printer.Err() output %q does not contain 'error message'",
			buf.String(),
		)
	}
}

func TestGetNullPrinter(t *testing.T) {
	printer := GetNullPrinter()

	// Test that Ln, F, S, C methods don't panic
	printer.Ln("test")
	printer.F("test %s", "format")
	printer.S("test")
	printer.C(func() string { return "test" })

	// Test Chk method
	if !printer.Chk(errors.New("test")) {
		t.Error("GetNullPrinter().Chk(error) returned false, expected true")
	}
	if printer.Chk(nil) {
		t.Error("GetNullPrinter().Chk(nil) returned true, expected false")
	}

	// Test Err method
	err := printer.Err("test %s", "error")
	if err == nil {
		t.Error("GetNullPrinter().Err() returned nil error")
	}
	if err.Error() != "test error" {
		t.Errorf(
			"GetNullPrinter().Err() returned error with message %q, expected 'test error'",
			err.Error(),
		)
	}
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	log, check, errorf := New(&buf, 1)

	// Verify that all components are created
	if log == nil {
		t.Error("New() returned nil Log")
	}
	if check == nil {
		t.Error("New() returned nil Check")
	}
	if errorf == nil {
		t.Error("New() returned nil Errorf")
	}

	// Test that the log functions work
	originalLevel := Level.Load()
	Level.Store(int32(Debug))
	defer Level.Store(originalLevel)

	buf.Reset()
	log.D.Ln("test message")
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf(
			"log.D.Ln() output %q doesn't contain 'test message'",
			buf.String(),
		)
	}
}
