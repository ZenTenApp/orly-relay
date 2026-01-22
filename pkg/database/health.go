//go:build !(js && wasm)

package database

import (
	"bytes"
	"fmt"
	"io"
	"time"

	"github.com/dgraph-io/badger/v4"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
)

// HealthReport contains the results of a database health check.
type HealthReport struct {
	// Scan metadata
	ScanStarted  time.Time
	ScanDuration time.Duration

	// Event counts
	CompactEvents  int64 // Events stored in compact format (cmp)
	LegacyEvents   int64 // Events in legacy format (evt)
	SmallEvents    int64 // Small inline events (sev)
	TotalEvents    int64 // Total events
	SerialIdCount  int64 // Serial to EventID mappings (sei)

	// Pubkey serial counts
	PubkeySerials int64 // pks entries (pubkey hash -> serial)
	SerialPubkeys int64 // spk entries (serial -> pubkey)

	// Graph edge counts
	EventPubkeyEdges int64 // epg entries
	PubkeyEventEdges int64 // peg entries
	EventEventEdges  int64 // eeg entries
	GraphEventEdges  int64 // gee entries

	// Index counts
	KindIndexes   int64 // kc- entries
	PubkeyIndexes int64 // pc- entries
	TagIndexes    int64 // tc- entries
	WordIndexes   int64 // wrd entries
	IdIndexes     int64 // eid entries

	// Issues found
	MissingSerialEventIds  int64 // cmp entries without corresponding sei
	OrphanedSerialEventIds int64 // sei entries without corresponding cmp
	PubkeySerialMismatches int64 // pks without matching spk or vice versa
	OrphanedIndexes        int64 // Index entries pointing to non-existent events

	// Sample of missing sei serials (for debugging)
	MissingSeiSamples []uint64

	// Health score (0-100)
	HealthScore int
}

// String returns a human-readable health report.
func (r *HealthReport) String() string {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "Database Health Report")
	fmt.Fprintln(&buf, "======================")
	fmt.Fprintf(&buf, "Scan duration: %v\n\n", r.ScanDuration)

	fmt.Fprintln(&buf, "Event Storage:")
	fmt.Fprintf(&buf, "  Compact events (cmp):    %d\n", r.CompactEvents)
	fmt.Fprintf(&buf, "  Legacy events (evt):     %d\n", r.LegacyEvents)
	fmt.Fprintf(&buf, "  Small events (sev):      %d\n", r.SmallEvents)
	fmt.Fprintf(&buf, "  Total events:            %d\n", r.TotalEvents)
	fmt.Fprintf(&buf, "  Serial->ID maps (sei):   %d\n\n", r.SerialIdCount)

	fmt.Fprintln(&buf, "Pubkey Mappings:")
	fmt.Fprintf(&buf, "  Pubkey serials (pks):    %d\n", r.PubkeySerials)
	fmt.Fprintf(&buf, "  Serial pubkeys (spk):    %d\n\n", r.SerialPubkeys)

	fmt.Fprintln(&buf, "Graph Edges:")
	fmt.Fprintf(&buf, "  Event->Pubkey (epg):     %d\n", r.EventPubkeyEdges)
	fmt.Fprintf(&buf, "  Pubkey->Event (peg):     %d\n", r.PubkeyEventEdges)
	fmt.Fprintf(&buf, "  Event->Event (eeg):      %d\n", r.EventEventEdges)
	fmt.Fprintf(&buf, "  Event<-Event (gee):      %d\n\n", r.GraphEventEdges)

	fmt.Fprintln(&buf, "Search Indexes:")
	fmt.Fprintf(&buf, "  Kind indexes (kc-):      %d\n", r.KindIndexes)
	fmt.Fprintf(&buf, "  Pubkey indexes (pc-):    %d\n", r.PubkeyIndexes)
	fmt.Fprintf(&buf, "  Tag indexes (tc-):       %d\n", r.TagIndexes)
	fmt.Fprintf(&buf, "  Word indexes (wrd):      %d\n", r.WordIndexes)
	fmt.Fprintf(&buf, "  ID indexes (eid):        %d\n\n", r.IdIndexes)

	fmt.Fprintln(&buf, "Issues Found:")
	fmt.Fprintf(&buf, "  Missing sei mappings:    %d", r.MissingSerialEventIds)
	if r.MissingSerialEventIds > 0 {
		fmt.Fprint(&buf, " (CRITICAL)")
	}
	fmt.Fprintln(&buf)
	fmt.Fprintf(&buf, "  Orphaned sei mappings:   %d\n", r.OrphanedSerialEventIds)
	fmt.Fprintf(&buf, "  Pubkey serial mismatch:  %d\n", r.PubkeySerialMismatches)
	fmt.Fprintf(&buf, "  Orphaned indexes:        %d\n\n", r.OrphanedIndexes)

	if len(r.MissingSeiSamples) > 0 {
		fmt.Fprintln(&buf, "Sample missing sei serials:")
		for i, s := range r.MissingSeiSamples {
			if i >= 10 {
				fmt.Fprintf(&buf, "  ... and %d more\n", len(r.MissingSeiSamples)-10)
				break
			}
			fmt.Fprintf(&buf, "  - %d\n", s)
		}
		fmt.Fprintln(&buf)
	}

	fmt.Fprintf(&buf, "Health Score: %d/100\n", r.HealthScore)

	if r.HealthScore < 50 {
		fmt.Fprintln(&buf, "\n⚠️  Database has critical issues. Run 'orly db repair' to fix.")
	} else if r.HealthScore < 80 {
		fmt.Fprintln(&buf, "\n⚠️  Database has some issues. Consider running 'orly db repair'.")
	} else {
		fmt.Fprintln(&buf, "\n✓ Database is healthy.")
	}

	return buf.String()
}

// HealthCheck performs a comprehensive health check of the database.
// It scans all index prefixes and verifies referential integrity.
func (d *D) HealthCheck(progress io.Writer) (report *HealthReport, err error) {
	report = &HealthReport{
		ScanStarted:       time.Now(),
		MissingSeiSamples: make([]uint64, 0, 100),
	}

	if progress != nil {
		fmt.Fprintln(progress, "Starting database health check...")
	}

	// Build prefix buffers for all index types
	cmpPrf := buildPrefix(indexes.CompactEventEnc(nil))
	seiPrf := buildPrefix(indexes.SerialEventIdEnc(nil))
	evtPrf := buildPrefix(indexes.EventEnc(nil))
	sevPrf := buildPrefix(indexes.SmallEventEnc(nil))
	pksPrf := buildPrefix(indexes.PubkeySerialEnc(nil, nil))
	spkPrf := buildPrefix(indexes.SerialPubkeyEnc(nil))
	epgPrf := buildPrefix(indexes.EventPubkeyGraphEnc(nil, nil, nil, nil))
	pegPrf := buildPrefix(indexes.PubkeyEventGraphEnc(nil, nil, nil, nil))
	eegPrf := buildPrefix(indexes.EventEventGraphEnc(nil, nil, nil, nil))
	geePrf := buildPrefix(indexes.GraphEventEventEnc(nil, nil, nil, nil))
	kcPrf := buildPrefix(indexes.KindEnc(nil, nil, nil))
	pcPrf := buildPrefix(indexes.PubkeyEnc(nil, nil, nil))
	tcPrf := buildPrefix(indexes.TagEnc(nil, nil, nil, nil))
	wrdPrf := buildPrefix(indexes.WordEnc(nil, nil))
	eidPrf := buildPrefix(indexes.IdEnc(nil, nil))

	// Phase 1: Count all entries with each prefix
	if progress != nil {
		fmt.Fprintln(progress, "Phase 1: Counting entries by prefix...")
	}

	err = d.View(func(txn *badger.Txn) error {
		// Count compact events
		report.CompactEvents = countPrefix(txn, cmpPrf)
		if progress != nil {
			fmt.Fprintf(progress, "  Compact events (cmp): %d\n", report.CompactEvents)
		}

		// Count serial->eventID mappings
		report.SerialIdCount = countPrefix(txn, seiPrf)
		if progress != nil {
			fmt.Fprintf(progress, "  Serial->ID maps (sei): %d\n", report.SerialIdCount)
		}

		// Count legacy events
		report.LegacyEvents = countPrefix(txn, evtPrf)
		report.SmallEvents = countPrefix(txn, sevPrf)
		report.TotalEvents = report.CompactEvents + report.LegacyEvents + report.SmallEvents
		if progress != nil {
			fmt.Fprintf(progress, "  Legacy events (evt): %d\n", report.LegacyEvents)
			fmt.Fprintf(progress, "  Small events (sev): %d\n", report.SmallEvents)
		}

		// Count pubkey serial mappings
		report.PubkeySerials = countPrefix(txn, pksPrf)
		report.SerialPubkeys = countPrefix(txn, spkPrf)
		if progress != nil {
			fmt.Fprintf(progress, "  Pubkey serials (pks): %d, (spk): %d\n", report.PubkeySerials, report.SerialPubkeys)
		}

		// Count graph edges
		report.EventPubkeyEdges = countPrefix(txn, epgPrf)
		report.PubkeyEventEdges = countPrefix(txn, pegPrf)
		report.EventEventEdges = countPrefix(txn, eegPrf)
		report.GraphEventEdges = countPrefix(txn, geePrf)
		if progress != nil {
			fmt.Fprintf(progress, "  Graph edges: epg=%d, peg=%d, eeg=%d, gee=%d\n",
				report.EventPubkeyEdges, report.PubkeyEventEdges, report.EventEventEdges, report.GraphEventEdges)
		}

		// Count search indexes
		report.KindIndexes = countPrefix(txn, kcPrf)
		report.PubkeyIndexes = countPrefix(txn, pcPrf)
		report.TagIndexes = countPrefix(txn, tcPrf)
		report.WordIndexes = countPrefix(txn, wrdPrf)
		report.IdIndexes = countPrefix(txn, eidPrf)
		if progress != nil {
			fmt.Fprintf(progress, "  Indexes: kc=%d, pc=%d, tc=%d, wrd=%d, eid=%d\n",
				report.KindIndexes, report.PubkeyIndexes, report.TagIndexes, report.WordIndexes, report.IdIndexes)
		}

		return nil
	})
	if chk.E(err) {
		return nil, err
	}

	// Phase 2: Check cmp->sei integrity (CRITICAL)
	if progress != nil {
		fmt.Fprintln(progress, "\nPhase 2: Checking compact event -> serial ID integrity...")
	}

	err = d.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{Prefix: cmpPrf})
		defer it.Close()

		checked := int64(0)
		for it.Rewind(); it.Valid(); it.Next() {
			// Extract serial from cmp key: prefix (3 bytes) + serial (5 bytes)
			key := it.Item().Key()
			if len(key) < 8 {
				continue
			}

			// Extract the serial
			serial := extractSerial(key[3:8])

			// Check if sei entry exists for this serial
			seiKey := buildSeiKey(serial)
			_, err := txn.Get(seiKey)
			if err == badger.ErrKeyNotFound {
				report.MissingSerialEventIds++
				if len(report.MissingSeiSamples) < 100 {
					report.MissingSeiSamples = append(report.MissingSeiSamples, serial)
				}
			} else if err != nil {
				log.W.F("error checking sei for serial %d: %v", serial, err)
			}

			checked++
			if progress != nil && checked%100000 == 0 {
				fmt.Fprintf(progress, "  Checked %d compact events, %d missing sei so far...\n",
					checked, report.MissingSerialEventIds)
			}
		}

		if progress != nil {
			fmt.Fprintf(progress, "  Checked %d compact events, found %d missing sei entries\n",
				checked, report.MissingSerialEventIds)
		}

		return nil
	})
	if chk.E(err) {
		return nil, err
	}

	// Phase 3: Check for orphaned sei entries (sei without cmp)
	if progress != nil {
		fmt.Fprintln(progress, "\nPhase 3: Checking for orphaned serial ID mappings...")
	}

	err = d.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{Prefix: seiPrf})
		defer it.Close()

		checked := int64(0)
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			if len(key) < 8 {
				continue
			}

			// Extract serial from sei key
			serial := extractSerial(key[3:8])

			// Check if cmp entry exists for this serial
			cmpKey := buildCmpKey(serial)
			_, err := txn.Get(cmpKey)
			if err == badger.ErrKeyNotFound {
				// Also check legacy evt format
				evtKey := buildEvtKey(serial)
				_, err2 := txn.Get(evtKey)
				if err2 == badger.ErrKeyNotFound {
					report.OrphanedSerialEventIds++
				}
			} else if err != nil {
				log.W.F("error checking cmp for serial %d: %v", serial, err)
			}

			checked++
			if progress != nil && checked%100000 == 0 {
				fmt.Fprintf(progress, "  Checked %d sei entries, %d orphaned so far...\n",
					checked, report.OrphanedSerialEventIds)
			}
		}

		if progress != nil {
			fmt.Fprintf(progress, "  Checked %d sei entries, found %d orphaned\n",
				checked, report.OrphanedSerialEventIds)
		}

		return nil
	})
	if chk.E(err) {
		return nil, err
	}

	// Phase 4: Check pubkey serial consistency
	if progress != nil {
		fmt.Fprintln(progress, "\nPhase 4: Checking pubkey serial consistency...")
	}

	err = d.View(func(txn *badger.Txn) error {
		// Check that pks count roughly matches spk count
		// A small difference is acceptable due to timing, but large differences indicate corruption
		diff := report.PubkeySerials - report.SerialPubkeys
		if diff < 0 {
			diff = -diff
		}
		// Allow 1% difference
		threshold := report.PubkeySerials / 100
		if threshold < 10 {
			threshold = 10
		}
		if diff > threshold {
			report.PubkeySerialMismatches = diff
			if progress != nil {
				fmt.Fprintf(progress, "  Found %d pubkey serial mismatches (pks=%d, spk=%d)\n",
					diff, report.PubkeySerials, report.SerialPubkeys)
			}
		} else if progress != nil {
			fmt.Fprintln(progress, "  Pubkey serial counts are consistent")
		}

		return nil
	})
	if chk.E(err) {
		return nil, err
	}

	// Calculate health score
	report.ScanDuration = time.Since(report.ScanStarted)
	report.HealthScore = calculateHealthScore(report)

	if progress != nil {
		fmt.Fprintf(progress, "\nHealth check complete. Score: %d/100\n", report.HealthScore)
	}

	return report, nil
}

// buildPrefix creates a prefix buffer from an encoder.
func buildPrefix(enc *indexes.T) []byte {
	buf := new(bytes.Buffer)
	if err := enc.MarshalWrite(buf); err != nil {
		return nil
	}
	// Return only the prefix part (3 bytes)
	b := buf.Bytes()
	if len(b) >= 3 {
		return b[:3]
	}
	return b
}

// countPrefix counts the number of entries with the given prefix.
func countPrefix(txn *badger.Txn, prefix []byte) int64 {
	it := txn.NewIterator(badger.IteratorOptions{
		Prefix:         prefix,
		PrefetchValues: false,
	})
	defer it.Close()

	var count int64
	for it.Rewind(); it.Valid(); it.Next() {
		count++
	}
	return count
}

// extractSerial extracts a 40-bit serial from 5 bytes (big-endian).
func extractSerial(b []byte) uint64 {
	if len(b) < 5 {
		return 0
	}
	return (uint64(b[0]) << 32) |
		(uint64(b[1]) << 24) |
		(uint64(b[2]) << 16) |
		(uint64(b[3]) << 8) |
		uint64(b[4])
}

// buildSeiKey builds a sei (serial->eventID) key for the given serial.
func buildSeiKey(serial uint64) []byte {
	ser := new(types.Uint40)
	ser.Set(serial)
	buf := new(bytes.Buffer)
	indexes.SerialEventIdEnc(ser).MarshalWrite(buf)
	return buf.Bytes()
}

// buildCmpKey builds a cmp (compact event) key for the given serial.
func buildCmpKey(serial uint64) []byte {
	ser := new(types.Uint40)
	ser.Set(serial)
	buf := new(bytes.Buffer)
	indexes.CompactEventEnc(ser).MarshalWrite(buf)
	return buf.Bytes()
}

// buildEvtKey builds an evt (legacy event) key for the given serial.
func buildEvtKey(serial uint64) []byte {
	ser := new(types.Uint40)
	ser.Set(serial)
	buf := new(bytes.Buffer)
	indexes.EventEnc(ser).MarshalWrite(buf)
	return buf.Bytes()
}

// calculateHealthScore calculates a health score from 0-100 based on the report.
func calculateHealthScore(r *HealthReport) int {
	score := 100

	// Missing sei is critical - each one costs 1 point, max 50 point penalty
	if r.MissingSerialEventIds > 0 {
		penalty := int(r.MissingSerialEventIds)
		if penalty > 50 {
			penalty = 50
		}
		score -= penalty
	}

	// Orphaned sei is less critical - each one costs 0.1 points, max 20 point penalty
	if r.OrphanedSerialEventIds > 0 {
		penalty := int(r.OrphanedSerialEventIds / 10)
		if penalty > 20 {
			penalty = 20
		}
		score -= penalty
	}

	// Pubkey mismatches cost 0.5 points each, max 20 point penalty
	if r.PubkeySerialMismatches > 0 {
		penalty := int(r.PubkeySerialMismatches / 2)
		if penalty > 20 {
			penalty = 20
		}
		score -= penalty
	}

	// Orphaned indexes cost 0.01 points each, max 10 point penalty
	if r.OrphanedIndexes > 0 {
		penalty := int(r.OrphanedIndexes / 100)
		if penalty > 10 {
			penalty = 10
		}
		score -= penalty
	}

	if score < 0 {
		score = 0
	}
	return score
}
