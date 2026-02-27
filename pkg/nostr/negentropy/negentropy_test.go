package negentropy

import (
	"testing"
)

func TestVectorBasic(t *testing.T) {
	v := NewVector()
	v.Insert(1000, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	v.Insert(2000, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	v.Insert(3000, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	v.Seal()

	if v.Size() != 3 {
		t.Errorf("expected size 3, got %d", v.Size())
	}

	// Test fingerprint
	fp := v.Fingerprint(0, 3)
	if fp == EmptyFingerprint {
		t.Error("expected non-empty fingerprint")
	}
}

func TestNegentropy(t *testing.T) {
	// Create two storages with overlapping data
	v1 := NewVector()
	v1.Insert(1000, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	v1.Insert(2000, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	v1.Insert(3000, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	v1.Seal()

	v2 := NewVector()
	v2.Insert(1000, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	v2.Insert(2000, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	v2.Insert(4000, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	v2.Seal()

	// Client initiates
	client := New(v1, DefaultFrameSizeLimit)
	server := New(v2, DefaultFrameSizeLimit)

	msg, err := client.Start()
	if err != nil {
		t.Fatalf("client.Start failed: %v", err)
	}

	// Server responds
	response, complete, err := server.Reconcile(msg)
	if err != nil {
		t.Fatalf("server.Reconcile failed: %v", err)
	}

	t.Logf("Round 1: complete=%v, response len=%d", complete, len(response))

	// Continue until complete
	for !complete {
		response, complete, err = client.Reconcile(response)
		if err != nil {
			t.Fatalf("reconcile failed: %v", err)
		}
		t.Logf("Round: complete=%v, response len=%d", complete, len(response))

		if !complete {
			response, complete, err = server.Reconcile(response)
			if err != nil {
				t.Fatalf("reconcile failed: %v", err)
			}
			t.Logf("Round: complete=%v, response len=%d", complete, len(response))
		}
	}

	// Check results
	clientHaves := client.CollectHaves()
	clientHaveNots := client.CollectHaveNots()
	serverHaves := server.CollectHaves()
	serverHaveNots := server.CollectHaveNots()

	t.Logf("Client haves: %v", clientHaves)
	t.Logf("Client haveNots: %v", clientHaveNots)
	t.Logf("Server haves: %v", serverHaves)
	t.Logf("Server haveNots: %v", serverHaveNots)
}

func TestEncoding(t *testing.T) {
	// Test VarInt encoding/decoding
	testCases := []uint64{0, 1, 127, 128, 16383, 16384, 1<<21 - 1, 1 << 21, 1<<63 - 1}

	for _, n := range testCases {
		encoded := EncodeVarInt(n)
		dec := NewDecoder(encoded)
		decoded, err := dec.ReadVarInt()
		if err != nil {
			t.Errorf("failed to decode %d: %v", n, err)
			continue
		}
		if decoded != n {
			t.Errorf("expected %d, got %d", n, decoded)
		}
	}
}

// TestConvergenceLargeSets tests that negentropy converges correctly with larger
// sets where each side has unique events the other doesn't.
func TestConvergenceLargeSets(t *testing.T) {
	sharedCount := 200
	clientOnlyCount := 50
	serverOnlyCount := 150

	v1 := NewVector()
	v2 := NewVector()

	for i := 0; i < sharedCount; i++ {
		ts := int64(1000 + i)
		id := makeTestID(i)
		v1.Insert(ts, id)
		v2.Insert(ts, id)
	}

	clientOnlyIDs := make(map[string]bool)
	for i := 0; i < clientOnlyCount; i++ {
		ts := int64(2000 + i)
		id := makeTestID(10000 + i)
		v1.Insert(ts, id)
		clientOnlyIDs[id] = true
	}

	serverOnlyIDs := make(map[string]bool)
	for i := 0; i < serverOnlyCount; i++ {
		ts := int64(500 + i)
		id := makeTestID(20000 + i)
		v2.Insert(ts, id)
		serverOnlyIDs[id] = true
	}

	v1.Seal()
	v2.Seal()

	runConvergenceTest(t, "basic", v1, v2, clientOnlyIDs, serverOnlyIDs,
		clientOnlyCount, serverOnlyCount)
}

// TestConvergenceInterspersed tests the critical case where client-only and
// server-only events are interspersed among shared events. This exercises the
// bug where splitRange used GetBound(bucketEnd) instead of upperBound for the
// last bucket, causing range misalignment when boundary events don't exist in
// the responder's storage.
func TestConvergenceInterspersed(t *testing.T) {
	v1 := NewVector()
	v2 := NewVector()

	clientOnlyIDs := make(map[string]bool)
	serverOnlyIDs := make(map[string]bool)
	clientOnlyCount := 0
	serverOnlyCount := 0

	// Create events at timestamps 0..499. Every 5th timestamp has a
	// client-only event, every 7th has a server-only event, others are shared.
	for i := 0; i < 500; i++ {
		ts := int64(1000 + i)
		if i%5 == 0 && i%7 != 0 {
			// Client-only: interspersed among shared events
			id := makeTestID(30000 + i)
			v1.Insert(ts, id)
			clientOnlyIDs[id] = true
			clientOnlyCount++
		} else if i%7 == 0 && i%5 != 0 {
			// Server-only: interspersed among shared events
			id := makeTestID(40000 + i)
			v2.Insert(ts, id)
			serverOnlyIDs[id] = true
			serverOnlyCount++
		} else {
			// Shared event
			id := makeTestID(50000 + i)
			v1.Insert(ts, id)
			v2.Insert(ts, id)
		}
	}

	v1.Seal()
	v2.Seal()

	runConvergenceTest(t, "interspersed", v1, v2, clientOnlyIDs, serverOnlyIDs,
		clientOnlyCount, serverOnlyCount)
}

// runConvergenceTest runs the full negentropy protocol and verifies correctness.
func runConvergenceTest(t *testing.T, name string, v1, v2 *Vector,
	clientOnlyIDs, serverOnlyIDs map[string]bool,
	expectedClientHaves, expectedClientHaveNots int) {
	t.Helper()

	t.Logf("[%s] Client has %d items, Server has %d items", name, v1.Size(), v2.Size())
	t.Logf("[%s] Expected client-only: %d, server-only: %d",
		name, expectedClientHaves, expectedClientHaveNots)

	client := New(v1, DefaultFrameSizeLimit)
	server := New(v2, DefaultFrameSizeLimit)

	msg, err := client.Start()
	if err != nil {
		t.Fatalf("client.Start failed: %v", err)
	}

	rounds := 0
	complete := false
	for !complete {
		rounds++
		if rounds > 20 {
			t.Fatalf("[%s] too many rounds (%d), protocol did not converge", name, rounds)
		}

		response, serverComplete, err := server.Reconcile(msg)
		if err != nil {
			t.Fatalf("[%s] server.Reconcile round %d failed: %v", name, rounds, err)
		}

		if serverComplete {
			msg, complete, err = client.Reconcile(response)
			if err != nil {
				t.Fatalf("[%s] client.Reconcile final round %d failed: %v", name, rounds, err)
			}
			break
		}

		msg, complete, err = client.Reconcile(response)
		if err != nil {
			t.Fatalf("[%s] client.Reconcile round %d failed: %v", name, rounds, err)
		}
	}

	t.Logf("[%s] Converged in %d rounds", name, rounds)

	clientHaves := client.CollectHaves()
	clientHaveNots := client.CollectHaveNots()

	t.Logf("[%s] Client haves: %d (expect %d), haveNots: %d (expect %d)",
		name, len(clientHaves), expectedClientHaves,
		len(clientHaveNots), expectedClientHaveNots)

	if len(clientHaves) != expectedClientHaves {
		t.Errorf("[%s] expected %d client haves, got %d",
			name, expectedClientHaves, len(clientHaves))
	}
	for _, id := range clientHaves {
		if !clientOnlyIDs[id] {
			t.Errorf("[%s] unexpected client have: %s", name, truncateID(id))
		}
	}

	if len(clientHaveNots) != expectedClientHaveNots {
		t.Errorf("[%s] expected %d client haveNots, got %d",
			name, expectedClientHaveNots, len(clientHaveNots))
	}
	for _, id := range clientHaveNots {
		if !serverOnlyIDs[id] {
			t.Errorf("[%s] unexpected client haveNot: %s", name, truncateID(id))
		}
	}
}

// makeTestID creates a deterministic 64-char hex ID from an integer.
func makeTestID(n int) string {
	// Create a 32-byte ID by repeating a pattern derived from n
	var buf [32]byte
	for i := 0; i < 32; i++ {
		buf[i] = byte((n >> (8 * (i % 4))) & 0xFF)
	}
	return encodeHex(buf[:])
}

func TestBoundEncoding(t *testing.T) {
	bounds := []Bound{
		MinBound(),
		MaxBound(),
		{Item{Timestamp: 1234567890, ID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}},
	}

	for _, b := range bounds {
		enc := NewEncoder(256)
		enc.WriteBound(b)

		dec := NewDecoder(enc.Bytes())
		decoded, err := dec.ReadBound()
		if err != nil {
			t.Errorf("failed to decode bound %v: %v", b, err)
			continue
		}

		// For MaxBound, we only compare timestamps since ID is empty
		if b.IsMax() {
			if !decoded.IsMax() {
				t.Errorf("expected MaxBound, got %v", decoded)
			}
		} else if b.Timestamp != decoded.Timestamp {
			t.Errorf("timestamp mismatch: expected %d, got %d", b.Timestamp, decoded.Timestamp)
		}
	}
}
