package database

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"sort"
	"testing"

	"lol.mleku.dev/chk"
	"git.mleku.dev/mleku/nostr/interfaces/signer/p8k"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/event/examples"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

var benchDB *D
var benchCtx context.Context
var benchCancel context.CancelFunc
var benchEvents []*event.E
var benchTempDir string

func setupBenchDB(b *testing.B) {
	b.Helper()
	if benchDB != nil {
		return // Already set up
	}
	var err error
	benchTempDir, err = os.MkdirTemp("", "bench-db-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	benchCtx, benchCancel = context.WithCancel(context.Background())
	benchDB, err = New(benchCtx, benchCancel, benchTempDir, "error")
	if err != nil {
		b.Fatalf("Failed to create DB: %v", err)
	}

	// Load events from examples
	scanner := bufio.NewScanner(bytes.NewBuffer(examples.Cache))
	scanner.Buffer(make([]byte, 0, 1_000_000_000), 1_000_000_000)
	benchEvents = make([]*event.E, 0, 1000)

	for scanner.Scan() {
		chk.E(scanner.Err())
		b := scanner.Bytes()
		ev := event.New()
		if _, err = ev.Unmarshal(b); chk.E(err) {
			ev.Free()
			continue
		}
		benchEvents = append(benchEvents, ev)
	}

	// Sort events by CreatedAt
	sort.Slice(benchEvents, func(i, j int) bool {
		return benchEvents[i].CreatedAt < benchEvents[j].CreatedAt
	})

	// Save events to database for benchmarks
	for _, ev := range benchEvents {
		_, _ = benchDB.SaveEvent(benchCtx, ev)
	}
}

func BenchmarkSaveEvent(b *testing.B) {
	setupBenchDB(b)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// Create a simple test event
		signer := p8k.MustNew()
		if err := signer.Generate(); err != nil {
			b.Fatal(err)
		}
		ev := event.New()
		ev.Pubkey = signer.Pub()
		ev.Kind = kind.TextNote.K
		ev.Content = []byte("benchmark test event")
		if err := ev.Sign(signer); err != nil {
			b.Fatal(err)
		}
		_, _ = benchDB.SaveEvent(benchCtx, ev)
	}
}

func BenchmarkQueryEvents(b *testing.B) {
	setupBenchDB(b)
	b.ResetTimer()
	b.ReportAllocs()
	f := &filter.F{
		Kinds: kind.NewS(kind.New(1)),
		Limit:  pointerOf(uint(100)),
	}
	for i := 0; i < b.N; i++ {
		_, _ = benchDB.QueryEvents(benchCtx, f)
	}
}

func BenchmarkQueryForIds(b *testing.B) {
	setupBenchDB(b)
	b.ResetTimer()
	b.ReportAllocs()
	f := &filter.F{
		Authors: tag.NewFromBytesSlice(benchEvents[0].Pubkey),
		Kinds:    kind.NewS(kind.New(1)),
		Limit:    pointerOf(uint(100)),
	}
	for i := 0; i < b.N; i++ {
		_, _ = benchDB.QueryForIds(benchCtx, f)
	}
}

func BenchmarkFetchEventsBySerials(b *testing.B) {
	setupBenchDB(b)
	// Get some serials first
	var idxs []Range
	idxs, _ = GetIndexesFromFilter(&filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	var serials []*types.Uint40
	if len(idxs) > 0 {
		serials, _ = benchDB.GetSerialsByRange(idxs[0])
		if len(serials) > 100 {
			serials = serials[:100]
		}
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchDB.FetchEventsBySerials(serials)
	}
}

func BenchmarkGetSerialsByRange(b *testing.B) {
	setupBenchDB(b)
	var idxs []Range
	idxs, _ = GetIndexesFromFilter(&filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	if len(idxs) == 0 {
		b.Skip("No indexes to test")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchDB.GetSerialsByRange(idxs[0])
	}
}

func BenchmarkGetFullIdPubkeyBySerials(b *testing.B) {
	setupBenchDB(b)
	var idxs []Range
	idxs, _ = GetIndexesFromFilter(&filter.F{
		Kinds: kind.NewS(kind.New(1)),
	})
	var serials []*types.Uint40
	if len(idxs) > 0 {
		serials, _ = benchDB.GetSerialsByRange(idxs[0])
		if len(serials) > 100 {
			serials = serials[:100]
		}
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchDB.GetFullIdPubkeyBySerials(serials)
	}
}

func BenchmarkGetSerialById(b *testing.B) {
	setupBenchDB(b)
	if len(benchEvents) == 0 {
		b.Skip("No events to test")
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		idx := i % len(benchEvents)
		_, _ = benchDB.GetSerialById(benchEvents[idx].ID)
	}
}

func BenchmarkGetSerialsByIds(b *testing.B) {
	setupBenchDB(b)
	if len(benchEvents) < 10 {
		b.Skip("Not enough events to test")
	}
	ids := tag.New()
	for i := 0; i < 10 && i < len(benchEvents); i++ {
		ids.T = append(ids.T, benchEvents[i].ID)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = benchDB.GetSerialsByIds(ids)
	}
}

func pointerOf[T any](v T) *T {
	return &v
}

