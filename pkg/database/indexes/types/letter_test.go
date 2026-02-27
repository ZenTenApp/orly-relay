package types

import (
	"bytes"
	"testing"

	"next.orly.dev/pkg/lol/chk"
)

func TestLetter_New(t *testing.T) {
	// Test with a valid letter
	l := new(Letter)
	l.Set('A')
	if l == nil {
		t.Fatal("New() returned nil")
	}
	if l.Letter() != 'A' {
		t.Errorf(
			"New('A') created a Letter with letter %c, want %c", l.Letter(),
			'A',
		)
	}
}

func TestLetter_Set(t *testing.T) {
	// Create a Letter with a known value
	l := new(Letter)
	l.Set('A')

	// Test Set
	l.Set('B')
	if l.Letter() != 'B' {
		t.Errorf(
			"Set('B') did not set the letter correctly: got %c, want %c",
			l.Letter(), 'B',
		)
	}
}

func TestLetter(t *testing.T) {
	// Create a Letter with a known value
	l := new(Letter)
	l.Set('A')

	// Test Letter
	if l.Letter() != 'A' {
		t.Errorf("Letter() returned %c, want %c", l.Letter(), 'A')
	}
}

func TestLetter_MarshalWriteUnmarshalRead(t *testing.T) {
	// Create a Letter with a known value
	l1 := new(Letter)
	l1.Set('A')
	// Test MarshalWrite
	buf := new(bytes.Buffer)
	err := l1.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Verify the written bytes
	if buf.Len() != 1 || buf.Bytes()[0] != 'A' {
		t.Errorf("MarshalWrite wrote %v, want [%d]", buf.Bytes(), 'A')
	}

	// Test UnmarshalRead
	l2 := new(Letter)
	l2.Set('B') // Start with a different value
	err = l2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if l2.Letter() != 'A' {
		t.Errorf("UnmarshalRead read %c, want %c", l2.Letter(), 'A')
	}
}

func TestLetter_UnmarshalReadWithEmptyReader(t *testing.T) {
	// Create a Letter with a known value
	l := new(Letter)
	l.Set('A')

	// Test UnmarshalRead with an empty reader
	err := l.UnmarshalRead(bytes.NewBuffer([]byte{}))
	if err == nil {
		t.Errorf("UnmarshalRead should have failed with an empty reader")
	}
}

func TestLetter_EdgeCases(t *testing.T) {
	// Test with minimum value (0)
	l1 := new(Letter)
	if l1.Letter() != 0 {
		t.Errorf(
			"New(0) created a Letter with letter %d, want %d", l1.Letter(), 0,
		)
	}

	// Test with maximum value (255)
	l2 := new(Letter)
	l2.Set(255)
	if l2.Letter() != 255 {
		t.Errorf(
			"New(255) created a Letter with letter %d, want %d", l2.Letter(),
			255,
		)
	}

	// Test with special characters
	specialChars := []byte{
		'\n', '\t', '\r', ' ', '!', '"', '#', '$', '%', '&', '\'', '(', ')',
		'*', '+', ',', '-', '.', '/',
	}
	for _, c := range specialChars {
		l := new(Letter)
		l.Set(c)
		if l.Letter() != c {
			t.Errorf(
				"New(%d) created a Letter with letter %d, want %d", c,
				l.Letter(), c,
			)
		}
	}
}
