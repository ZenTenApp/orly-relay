package types

import (
	"bytes"
	"testing"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
)

func TestTimestamp_FromInt(t *testing.T) {
	// Test with a positive value
	ts := &Timestamp{}
	ts.FromInt(12345)
	if ts.val != 12345 {
		t.Errorf(
			"FromInt(12345) did not set the value correctly: got %d, want %d",
			ts.val, 12345,
		)
	}

	// Test with a negative value
	ts = &Timestamp{}
	ts.FromInt(-12345)
	if ts.val != -12345 {
		t.Errorf(
			"FromInt(-12345) did not set the value correctly: got %d, want %d",
			ts.val, -12345,
		)
	}

	// Test with zero
	ts = &Timestamp{}
	ts.FromInt(0)
	if ts.val != 0 {
		t.Errorf(
			"FromInt(0) did not set the value correctly: got %d, want %d",
			ts.val, 0,
		)
	}
}

func TestTimestamp_FromInt64(t *testing.T) {
	// Test with a positive value
	ts := &Timestamp{}
	ts.FromInt64(12345)
	if ts.val != 12345 {
		t.Errorf(
			"FromInt64(12345) did not set the value correctly: got %d, want %d",
			ts.val, 12345,
		)
	}

	// Test with a negative value
	ts = &Timestamp{}
	ts.FromInt64(-12345)
	if ts.val != -12345 {
		t.Errorf(
			"FromInt64(-12345) did not set the value correctly: got %d, want %d",
			ts.val, -12345,
		)
	}

	// Test with zero
	ts = &Timestamp{}
	ts.FromInt64(0)
	if ts.val != 0 {
		t.Errorf(
			"FromInt64(0) did not set the value correctly: got %d, want %d",
			ts.val, 0,
		)
	}

	// Test with a large value
	ts = &Timestamp{}
	largeValue := int64(1) << 60
	ts.FromInt64(largeValue)
	if ts.val != largeValue {
		t.Errorf(
			"FromInt64(%d) did not set the value correctly: got %d, want %d",
			largeValue, ts.val, largeValue,
		)
	}
}

func TestTimestamp_FromBytes(t *testing.T) {
	// Create a number.Uint64 with a known value
	v := new(Uint64)
	v.Set(12345)

	// Marshal it to bytes
	buf := new(bytes.Buffer)
	err := v.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Test FromBytes
	ts, err := FromBytes(buf.Bytes())
	if chk.E(err) {
		t.Fatalf("FromBytes failed: %v", err)
	}
	if ts.val != 12345 {
		t.Errorf(
			"FromBytes did not set the value correctly: got %d, want %d",
			ts.val, 12345,
		)
	}

	// Test with invalid bytes
	_, err = FromBytes([]byte{1})
	if err == nil {
		t.Errorf("FromBytes should have failed with invalid bytes")
	}
}

func TestTimestamp_ToTimestamp(t *testing.T) {
	// Test with a positive value
	ts := &Timestamp{val: 12345}
	timestamp := ts.ToTimestamp()
	if timestamp != 12345 {
		t.Errorf("ToTimestamp() returned %d, want %d", timestamp, 12345)
	}

	// Test with a negative value
	ts = &Timestamp{val: -12345}
	timestamp = ts.ToTimestamp()
	if timestamp != -12345 {
		t.Errorf("ToTimestamp() returned %d, want %d", timestamp, -12345)
	}

	// Test with zero
	ts = &Timestamp{val: 0}
	timestamp = ts.ToTimestamp()
	if timestamp != 0 {
		t.Errorf("ToTimestamp() returned %d, want %d", timestamp, 0)
	}
}

func TestTimestamp_Bytes(t *testing.T) {
	// Test with a positive value
	ts := &Timestamp{val: 12345}
	b, err := ts.Bytes()
	if chk.E(err) {
		t.Fatalf("Bytes() failed: %v", err)
	}

	// Verify the bytes
	v := new(Uint64)
	err = v.UnmarshalRead(bytes.NewBuffer(b))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}
	if v.Get() != 12345 {
		t.Errorf("Bytes() returned bytes for %d, want %d", v.Get(), 12345)
	}

	// Skip negative value test for Bytes() since uint64 can't represent negative values
	// Instead, we'll test that MarshalWrite and UnmarshalRead work correctly with negative values
	// in the TestMarshalWriteUnmarshalRead function
}

func TestTimestamp_MarshalWriteUnmarshalRead(t *testing.T) {
	// Test with a positive value
	ts1 := &Timestamp{val: 12345}
	buf := new(bytes.Buffer)
	err := ts1.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Test UnmarshalRead
	ts2 := &Timestamp{}
	err = ts2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if ts2.val != 12345 {
		t.Errorf("UnmarshalRead read %d, want %d", ts2.val, 12345)
	}

	// Test with a negative value
	ts1 = &Timestamp{val: -12345}
	buf = new(bytes.Buffer)
	err = ts1.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	// Test UnmarshalRead
	ts2 = &Timestamp{}
	err = ts2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	// Verify the read value
	if ts2.val != -12345 {
		t.Errorf("UnmarshalRead read %d, want %d", ts2.val, -12345)
	}
}

func TestTimestamp_WithCurrentTime(t *testing.T) {
	// Get the current time
	now := time.Now().Unix()

	// Create a timestamp with the current time
	ts := &Timestamp{}
	ts.FromInt64(now)

	// Verify the value
	if ts.val != now {
		t.Errorf(
			"FromInt64(%d) did not set the value correctly: got %d, want %d",
			now, ts.val, now,
		)
	}

	// Test ToTimestamp
	timestamp := ts.ToTimestamp()
	if timestamp != now {
		t.Errorf("ToTimestamp() returned %d, want %d", timestamp, now)
	}

	// Test MarshalWrite and UnmarshalRead
	buf := new(bytes.Buffer)
	err := ts.MarshalWrite(buf)
	if chk.E(err) {
		t.Fatalf("MarshalWrite failed: %v", err)
	}

	ts2 := &Timestamp{}
	err = ts2.UnmarshalRead(bytes.NewBuffer(buf.Bytes()))
	if chk.E(err) {
		t.Fatalf("UnmarshalRead failed: %v", err)
	}

	if ts2.val != now {
		t.Errorf("UnmarshalRead read %d, want %d", ts2.val, now)
	}
}
