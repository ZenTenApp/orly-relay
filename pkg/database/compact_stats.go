//go:build !(js && wasm)

package database

import (
	"bytes"
	"sync/atomic"

	"github.com/dgraph-io/badger/v4"
	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
	"next.orly.dev/pkg/database/indexes"
)

// CompactStorageStats holds statistics about compact vs legacy storage.
type CompactStorageStats struct {
	// Event counts
	CompactEvents int64 // Number of events in compact format (cmp prefix)
	LegacyEvents  int64 // Number of events in legacy format (evt/sev prefixes)
	TotalEvents   int64 // Total events

	// Storage sizes
	CompactBytes int64 // Total bytes used by compact format
	LegacyBytes  int64 // Total bytes used by legacy format (would be used without compact)

	// Savings
	BytesSaved      int64   // Bytes saved by using compact format
	PercentSaved    float64 // Percentage of space saved
	AverageCompact  float64 // Average compact event size
	AverageLegacy   float64 // Average legacy event size (estimated)

	// Serial mappings
	SerialEventIdEntries int64 // Number of sei (serial -> event ID) mappings
	SerialEventIdBytes   int64 // Bytes used by sei mappings
}

// CompactStorageStats calculates storage statistics for compact event storage.
// This scans the database to provide accurate metrics on space savings.
func (d *D) CompactStorageStats() (stats CompactStorageStats, err error) {
	if err = d.View(func(txn *badger.Txn) error {
		// Count compact events (cmp prefix)
		cmpPrf := new(bytes.Buffer)
		if err = indexes.CompactEventEnc(nil).MarshalWrite(cmpPrf); chk.E(err) {
			return err
		}

		it := txn.NewIterator(badger.IteratorOptions{Prefix: cmpPrf.Bytes()})
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			stats.CompactEvents++
			stats.CompactBytes += int64(len(item.Key())) + int64(item.ValueSize())
		}
		it.Close()

		// Count legacy evt entries
		evtPrf := new(bytes.Buffer)
		if err = indexes.EventEnc(nil).MarshalWrite(evtPrf); chk.E(err) {
			return err
		}

		it = txn.NewIterator(badger.IteratorOptions{Prefix: evtPrf.Bytes()})
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			stats.LegacyEvents++
			stats.LegacyBytes += int64(len(item.Key())) + int64(item.ValueSize())
		}
		it.Close()

		// Count legacy sev entries
		sevPrf := new(bytes.Buffer)
		if err = indexes.SmallEventEnc(nil).MarshalWrite(sevPrf); chk.E(err) {
			return err
		}

		it = txn.NewIterator(badger.IteratorOptions{Prefix: sevPrf.Bytes()})
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			stats.LegacyEvents++
			stats.LegacyBytes += int64(len(item.Key())) // sev stores data in key
		}
		it.Close()

		// Count SerialEventId mappings (sei prefix)
		seiPrf := new(bytes.Buffer)
		if err = indexes.SerialEventIdEnc(nil).MarshalWrite(seiPrf); chk.E(err) {
			return err
		}

		it = txn.NewIterator(badger.IteratorOptions{Prefix: seiPrf.Bytes()})
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			stats.SerialEventIdEntries++
			stats.SerialEventIdBytes += int64(len(item.Key())) + int64(item.ValueSize())
		}
		it.Close()

		return nil
	}); chk.E(err) {
		return
	}

	stats.TotalEvents = stats.CompactEvents + stats.LegacyEvents

	// Calculate averages
	if stats.CompactEvents > 0 {
		stats.AverageCompact = float64(stats.CompactBytes) / float64(stats.CompactEvents)
	}
	if stats.LegacyEvents > 0 {
		stats.AverageLegacy = float64(stats.LegacyBytes) / float64(stats.LegacyEvents)
	}

	// Estimate savings: compare compact size to what legacy size would be
	// For events that are in compact format, estimate legacy size based on typical ratios
	// A typical event has:
	// - 32 bytes event ID (saved in compact: stored separately in sei)
	// - 32 bytes pubkey (saved: replaced by 5-byte serial)
	// - For e-tags: 32 bytes each (saved: replaced by 5-byte serial when known)
	// - For p-tags: 32 bytes each (saved: replaced by 5-byte serial)
	// Conservative estimate: compact format is ~60% of legacy size for typical events
	if stats.CompactEvents > 0 && stats.AverageCompact > 0 {
		// Estimate what the legacy size would have been
		estimatedLegacyForCompact := float64(stats.CompactBytes) / 0.60 // 60% compression ratio
		stats.BytesSaved = int64(estimatedLegacyForCompact) - stats.CompactBytes - stats.SerialEventIdBytes
		if stats.BytesSaved < 0 {
			stats.BytesSaved = 0
		}
		totalWithoutCompact := estimatedLegacyForCompact + float64(stats.LegacyBytes)
		totalWithCompact := float64(stats.CompactBytes + stats.LegacyBytes + stats.SerialEventIdBytes)
		if totalWithoutCompact > 0 {
			stats.PercentSaved = (1.0 - totalWithCompact/totalWithoutCompact) * 100.0
		}
	}

	return stats, nil
}

// compactSaveCounter tracks cumulative bytes saved by compact format
var compactSaveCounter atomic.Int64

// LogCompactSavings logs the storage savings achieved by compact format.
// Call this periodically or after significant operations.
func (d *D) LogCompactSavings() {
	stats, err := d.CompactStorageStats()
	if err != nil {
		log.W.F("failed to get compact storage stats: %v", err)
		return
	}

	if stats.TotalEvents == 0 {
		return
	}

	log.I.F("📊 Compact storage stats: %d compact events, %d legacy events",
		stats.CompactEvents, stats.LegacyEvents)
	log.I.F("   Compact size: %.2f MB, Legacy size: %.2f MB",
		float64(stats.CompactBytes)/(1024.0*1024.0),
		float64(stats.LegacyBytes)/(1024.0*1024.0))
	log.I.F("   Serial mappings (sei): %d entries, %.2f KB",
		stats.SerialEventIdEntries,
		float64(stats.SerialEventIdBytes)/1024.0)

	if stats.CompactEvents > 0 {
		log.I.F("   Average compact event: %.0f bytes, estimated legacy: %.0f bytes",
			stats.AverageCompact, stats.AverageCompact/0.60)
		log.I.F("   Estimated savings: %.2f MB (%.1f%%)",
			float64(stats.BytesSaved)/(1024.0*1024.0),
			stats.PercentSaved)
	}

	// Also log serial cache stats
	cacheStats := d.SerialCacheStats()
	log.I.F("   Serial cache: %d/%d pubkeys, %d/%d event IDs, ~%.2f MB memory",
		cacheStats.PubkeysCached, cacheStats.PubkeysMaxSize,
		cacheStats.EventIdsCached, cacheStats.EventIdsMaxSize,
		float64(cacheStats.TotalMemoryBytes)/(1024.0*1024.0))
}

// TrackCompactSaving records bytes saved for a single event.
// Call this during event save to track cumulative savings.
func TrackCompactSaving(legacySize, compactSize int) {
	saved := legacySize - compactSize
	if saved > 0 {
		compactSaveCounter.Add(int64(saved))
	}
}

// GetCumulativeCompactSavings returns total bytes saved across all compact saves.
func GetCumulativeCompactSavings() int64 {
	return compactSaveCounter.Load()
}

// ResetCompactSavingsCounter resets the cumulative savings counter.
func ResetCompactSavingsCounter() {
	compactSaveCounter.Store(0)
}
