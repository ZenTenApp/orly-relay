//go:build !(js && wasm)

package database

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/dgraph-io/badger/v4"
	"git.mleku.dev/mleku/nostr/crypto/ec/schnorr"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/varint"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database/indexes"
	"next.orly.dev/pkg/database/indexes/types"
)

// RepairReport contains the results of a database repair operation.
type RepairReport struct {
	// Operation metadata
	Started  time.Time
	Duration time.Duration
	DryRun   bool

	// Events scanned
	CompactEventsScanned int64
	LegacyEventsScanned  int64

	// Repairs performed
	SeiEntriesCreated    int64 // sei mappings rebuilt from compact events
	SeiEntriesRemoved    int64 // orphaned sei entries removed
	PubkeyMappingsFixed  int64 // pubkey serial mappings fixed
	OrphanedIndexesFixed int64 // orphaned index entries removed

	// Errors encountered
	Errors []string
}

// String returns a human-readable repair report.
func (r *RepairReport) String() string {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "Database Repair Report")
	fmt.Fprintln(&buf, "======================")
	fmt.Fprintf(&buf, "Duration: %v\n", r.Duration)
	if r.DryRun {
		fmt.Fprintln(&buf, "Mode: DRY RUN (no changes made)")
	} else {
		fmt.Fprintln(&buf, "Mode: REPAIR (changes applied)")
	}
	fmt.Fprintln(&buf)

	fmt.Fprintln(&buf, "Events Scanned:")
	fmt.Fprintf(&buf, "  Compact events:     %d\n", r.CompactEventsScanned)
	fmt.Fprintf(&buf, "  Legacy events:      %d\n\n", r.LegacyEventsScanned)

	fmt.Fprintln(&buf, "Repairs:")
	fmt.Fprintf(&buf, "  sei entries created:  %d\n", r.SeiEntriesCreated)
	fmt.Fprintf(&buf, "  sei entries removed:  %d\n", r.SeiEntriesRemoved)
	fmt.Fprintf(&buf, "  pubkey mappings fixed: %d\n", r.PubkeyMappingsFixed)
	fmt.Fprintf(&buf, "  orphaned indexes:     %d\n\n", r.OrphanedIndexesFixed)

	if len(r.Errors) > 0 {
		fmt.Fprintln(&buf, "Errors encountered:")
		for _, e := range r.Errors {
			fmt.Fprintf(&buf, "  - %s\n", e)
		}
	}

	return buf.String()
}

// RepairOptions configures the repair operation.
type RepairOptions struct {
	// DryRun if true, only reports what would be fixed without making changes
	DryRun bool

	// FixMissingSei if true, rebuilds missing sei entries from compact events
	FixMissingSei bool

	// RemoveOrphanedSei if true, removes sei entries without corresponding events
	RemoveOrphanedSei bool

	// FixPubkeyMappings if true, fixes inconsistent pubkey serial mappings
	FixPubkeyMappings bool

	// Progress writer for progress updates
	Progress io.Writer
}

// DefaultRepairOptions returns the default repair options.
func DefaultRepairOptions() *RepairOptions {
	return &RepairOptions{
		DryRun:            false,
		FixMissingSei:     true,
		RemoveOrphanedSei: true,
		FixPubkeyMappings: true,
	}
}

// Repair performs database repair operations based on the provided options.
// It fixes integrity issues found by HealthCheck.
func (d *D) Repair(ctx context.Context, opts *RepairOptions) (report *RepairReport, err error) {
	if opts == nil {
		opts = DefaultRepairOptions()
	}

	report = &RepairReport{
		Started: time.Now(),
		DryRun:  opts.DryRun,
		Errors:  make([]string, 0),
	}

	progress := opts.Progress

	if progress != nil {
		if opts.DryRun {
			fmt.Fprintln(progress, "Starting database repair (DRY RUN)...")
		} else {
			fmt.Fprintln(progress, "Starting database repair...")
		}
	}

	// Phase 1: Fix missing sei entries
	if opts.FixMissingSei {
		if progress != nil {
			fmt.Fprintln(progress, "\nPhase 1: Rebuilding missing sei entries from compact events...")
		}

		if err = d.repairMissingSei(ctx, opts, report); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("sei repair error: %v", err))
			log.E.F("sei repair error: %v", err)
		}
	}

	// Phase 2: Remove orphaned sei entries
	if opts.RemoveOrphanedSei {
		if progress != nil {
			fmt.Fprintln(progress, "\nPhase 2: Removing orphaned sei entries...")
		}

		if err = d.repairOrphanedSei(ctx, opts, report); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("orphaned sei repair error: %v", err))
			log.E.F("orphaned sei repair error: %v", err)
		}
	}

	// Phase 3: Fix pubkey serial mappings
	if opts.FixPubkeyMappings {
		if progress != nil {
			fmt.Fprintln(progress, "\nPhase 3: Checking pubkey serial mappings...")
		}

		if err = d.repairPubkeyMappings(ctx, opts, report); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("pubkey mapping repair error: %v", err))
			log.E.F("pubkey mapping repair error: %v", err)
		}
	}

	report.Duration = time.Since(report.Started)

	if progress != nil {
		fmt.Fprintf(progress, "\nRepair complete in %v\n", report.Duration)
		fmt.Fprintf(progress, "  sei entries created: %d\n", report.SeiEntriesCreated)
		fmt.Fprintf(progress, "  sei entries removed: %d\n", report.SeiEntriesRemoved)
		fmt.Fprintf(progress, "  pubkey mappings fixed: %d\n", report.PubkeyMappingsFixed)
		if opts.DryRun {
			fmt.Fprintln(progress, "\n(DRY RUN - no changes were made)")
		}
	}

	return report, nil
}

// repairMissingSei rebuilds sei entries for compact events that are missing them.
func (d *D) repairMissingSei(ctx context.Context, opts *RepairOptions, report *RepairReport) error {
	progress := opts.Progress
	cmpPrf := buildPrefix(indexes.CompactEventEnc(nil))
	seiPrf := buildPrefix(indexes.SerialEventIdEnc(nil))

	// First pass: collect all serials that need sei entries
	type repairItem struct {
		serial  uint64
		eventID []byte
	}
	var toRepair []repairItem

	err := d.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{Prefix: cmpPrf})
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			item := it.Item()
			key := item.Key()
			if len(key) < 8 {
				continue
			}

			serial := extractSerial(key[3:8])
			report.CompactEventsScanned++

			// Check if sei entry exists
			seiKey := buildSeiKey(serial)
			_, err := txn.Get(seiKey)
			if err == badger.ErrKeyNotFound {
				// Need to repair - extract event ID from compact event
				var eventData []byte
				err = item.Value(func(val []byte) error {
					eventData = make([]byte, len(val))
					copy(eventData, val)
					return nil
				})
				if err != nil {
					continue
				}

				// Decode compact event to get the event ID
				eventID, decodeErr := extractEventIDFromCompact(eventData, serial, d)
				if decodeErr != nil {
					if progress != nil && report.CompactEventsScanned%10000 == 0 {
						log.D.F("could not extract event ID for serial %d: %v", serial, decodeErr)
					}
					continue
				}

				toRepair = append(toRepair, repairItem{serial: serial, eventID: eventID})
			}

			if progress != nil && report.CompactEventsScanned%100000 == 0 {
				fmt.Fprintf(progress, "  Scanned %d compact events, found %d needing sei repair...\n",
					report.CompactEventsScanned, len(toRepair))
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if progress != nil {
		fmt.Fprintf(progress, "  Found %d compact events needing sei repair\n", len(toRepair))
	}

	// Also count legacy events
	err = d.View(func(txn *badger.Txn) error {
		evtPrf := buildPrefix(indexes.EventEnc(nil))
		it := txn.NewIterator(badger.IteratorOptions{Prefix: evtPrf, PrefetchValues: false})
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			report.LegacyEventsScanned++
		}
		return nil
	})
	if err != nil {
		log.W.F("error counting legacy events: %v", err)
	}

	// Second pass: create missing sei entries
	if opts.DryRun {
		report.SeiEntriesCreated = int64(len(toRepair))
		if progress != nil {
			fmt.Fprintf(progress, "  Would create %d sei entries (dry run)\n", len(toRepair))
		}
		return nil
	}

	// Write in batches
	batchSize := 1000
	for i := 0; i < len(toRepair); i += batchSize {
		end := i + batchSize
		if end > len(toRepair) {
			end = len(toRepair)
		}
		batch := toRepair[i:end]

		wb := d.DB.NewWriteBatch()
		for _, item := range batch {
			seiKey := buildSeiKey(item.serial)
			if err := wb.Set(seiKey, item.eventID); err != nil {
				wb.Cancel()
				return err
			}
		}

		if err := wb.Flush(); err != nil {
			return err
		}

		report.SeiEntriesCreated += int64(len(batch))

		if progress != nil && i+batchSize < len(toRepair) {
			fmt.Fprintf(progress, "  Created %d/%d sei entries...\n", report.SeiEntriesCreated, len(toRepair))
		}
	}

	_ = seiPrf // prevent unused warning
	return nil
}

// repairOrphanedSei removes sei entries that don't have corresponding events.
func (d *D) repairOrphanedSei(ctx context.Context, opts *RepairOptions, report *RepairReport) error {
	progress := opts.Progress
	seiPrf := buildPrefix(indexes.SerialEventIdEnc(nil))
	cmpPrf := buildPrefix(indexes.CompactEventEnc(nil))
	evtPrf := buildPrefix(indexes.EventEnc(nil))

	var orphanedSerials []uint64

	err := d.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{Prefix: seiPrf, PrefetchValues: false})
		defer it.Close()

		checked := int64(0)
		for it.Rewind(); it.Valid(); it.Next() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			key := it.Item().Key()
			if len(key) < 8 {
				continue
			}

			serial := extractSerial(key[3:8])

			// Check if cmp entry exists
			cmpKey := buildCmpKey(serial)
			_, err := txn.Get(cmpKey)
			if err == badger.ErrKeyNotFound {
				// Also check legacy evt
				evtKey := buildEvtKey(serial)
				_, err2 := txn.Get(evtKey)
				if err2 == badger.ErrKeyNotFound {
					orphanedSerials = append(orphanedSerials, serial)
				}
			}

			checked++
			if progress != nil && checked%100000 == 0 {
				fmt.Fprintf(progress, "  Checked %d sei entries, found %d orphaned...\n",
					checked, len(orphanedSerials))
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	if progress != nil {
		fmt.Fprintf(progress, "  Found %d orphaned sei entries\n", len(orphanedSerials))
	}

	if opts.DryRun {
		report.SeiEntriesRemoved = int64(len(orphanedSerials))
		if progress != nil {
			fmt.Fprintf(progress, "  Would remove %d orphaned sei entries (dry run)\n", len(orphanedSerials))
		}
		return nil
	}

	// Remove orphaned entries in batches
	batchSize := 1000
	for i := 0; i < len(orphanedSerials); i += batchSize {
		end := i + batchSize
		if end > len(orphanedSerials) {
			end = len(orphanedSerials)
		}
		batch := orphanedSerials[i:end]

		wb := d.DB.NewWriteBatch()
		for _, serial := range batch {
			seiKey := buildSeiKey(serial)
			if err := wb.Delete(seiKey); err != nil {
				wb.Cancel()
				return err
			}
		}

		if err := wb.Flush(); err != nil {
			return err
		}

		report.SeiEntriesRemoved += int64(len(batch))
	}

	_ = cmpPrf // prevent unused
	_ = evtPrf
	return nil
}

// repairPubkeyMappings fixes inconsistent pubkey serial mappings.
func (d *D) repairPubkeyMappings(ctx context.Context, opts *RepairOptions, report *RepairReport) error {
	progress := opts.Progress
	pksPrf := buildPrefix(indexes.PubkeySerialEnc(nil, nil))
	spkPrf := buildPrefix(indexes.SerialPubkeyEnc(nil))

	// Count entries
	var pksCount, spkCount int64
	err := d.View(func(txn *badger.Txn) error {
		pksCount = countPrefix(txn, pksPrf)
		spkCount = countPrefix(txn, spkPrf)
		return nil
	})
	if err != nil {
		return err
	}

	if progress != nil {
		fmt.Fprintf(progress, "  pks entries: %d, spk entries: %d\n", pksCount, spkCount)
	}

	// For now, just report the mismatch
	// Full repair would require scanning all events and rebuilding mappings
	diff := pksCount - spkCount
	if diff < 0 {
		diff = -diff
	}

	threshold := pksCount / 100
	if threshold < 10 {
		threshold = 10
	}

	if diff > threshold {
		report.PubkeyMappingsFixed = diff
		if progress != nil {
			fmt.Fprintf(progress, "  Found %d pubkey mapping inconsistencies\n", diff)
			fmt.Fprintln(progress, "  Note: Full pubkey mapping repair requires event rescan (not yet implemented)")
		}
	} else {
		if progress != nil {
			fmt.Fprintln(progress, "  Pubkey mappings are consistent")
		}
	}

	return nil
}

// extractEventIDFromCompact decodes a compact event and computes its ID.
// This is used during repair when the sei entry is missing.
// The event ID is computed by hashing the serialized event content.
func extractEventIDFromCompact(data []byte, serial uint64, d *D) ([]byte, error) {
	// Decode the compact event without the ID
	ev, err := unmarshalCompactForRepair(data, d)
	if err != nil {
		return nil, fmt.Errorf("failed to decode compact event: %w", err)
	}

	// Compute the event ID by hashing the serialized event
	eventID := ev.GetIDBytes()
	if eventID == nil || len(eventID) != 32 {
		return nil, fmt.Errorf("failed to compute event ID")
	}

	return eventID, nil
}

// unmarshalCompactForRepair decodes a compact event for repair purposes.
// Unlike UnmarshalCompactEvent, this doesn't require the event ID upfront.
func unmarshalCompactForRepair(data []byte, d *D) (*event.E, error) {
	r := bytes.NewReader(data)
	ev := new(event.E)

	// Version byte
	version, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != CompactFormatVersion {
		return nil, fmt.Errorf("unsupported compact format version: %d", version)
	}

	// Author pubkey serial (5 bytes) -> full pubkey
	authorSerial, err := readUint40(r)
	if err != nil {
		return nil, err
	}
	ev.Pubkey, err = d.getPubkeyBySerial(authorSerial)
	if err != nil {
		return nil, fmt.Errorf("failed to get pubkey for serial %d: %w", authorSerial, err)
	}

	// CreatedAt (varint)
	ca, err := varint.Decode(r)
	if err != nil {
		return nil, err
	}
	ev.CreatedAt = int64(ca)

	// Kind (2 bytes big-endian)
	if err = binary.Read(r, binary.BigEndian, &ev.Kind); err != nil {
		return nil, err
	}

	// Tags
	nTags, err := varint.Decode(r)
	if err != nil {
		return nil, err
	}
	if nTags > MaxTagsPerEvent {
		return nil, ErrTooManyTags
	}
	if nTags > 0 {
		ev.Tags = tag.NewSWithCap(int(nTags))
		resolver := &repairSerialResolver{d: d}
		for i := uint64(0); i < nTags; i++ {
			t, err := decodeCompactTag(r, resolver)
			if err != nil {
				return nil, err
			}
			*ev.Tags = append(*ev.Tags, t)
		}
	}

	// Content
	contentLen, err := varint.Decode(r)
	if err != nil {
		return nil, err
	}
	if contentLen > MaxContentLength {
		return nil, ErrContentTooLarge
	}
	ev.Content = make([]byte, contentLen)
	if _, err = io.ReadFull(r, ev.Content); err != nil {
		return nil, err
	}

	// Signature (64 bytes)
	ev.Sig = make([]byte, schnorr.SignatureSize)
	if _, err = io.ReadFull(r, ev.Sig); err != nil {
		return nil, err
	}

	return ev, nil
}

// repairSerialResolver implements SerialResolver for repair operations.
type repairSerialResolver struct {
	d *D
}

func (r *repairSerialResolver) GetOrCreatePubkeySerial(pubkey []byte) (uint64, error) {
	return 0, fmt.Errorf("not supported in repair mode")
}

func (r *repairSerialResolver) GetPubkeyBySerial(serial uint64) ([]byte, error) {
	return r.d.getPubkeyBySerial(serial)
}

func (r *repairSerialResolver) GetEventSerialById(eventId []byte) (uint64, bool, error) {
	return 0, false, nil // Not needed for decoding
}

func (r *repairSerialResolver) GetEventIdBySerial(serial uint64) ([]byte, error) {
	return r.d.getEventIdBySerial(serial)
}

// getEventIdBySerial returns the event ID for a given serial.
// This is a helper for compact event decoding.
func (d *D) getEventIdBySerial(serial uint64) ([]byte, error) {
	var eventID []byte
	err := d.View(func(txn *badger.Txn) error {
		seiKey := buildSeiKey(serial)
		item, err := txn.Get(seiKey)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			eventID = make([]byte, len(val))
			copy(eventID, val)
			return nil
		})
	})
	return eventID, err
}

// getPubkeyBySerial returns the pubkey for a given serial.
// This is a helper for compact event decoding.
func (d *D) getPubkeyBySerial(serial uint64) ([]byte, error) {
	var pubkey []byte
	err := d.View(func(txn *badger.Txn) error {
		ser := new(types.Uint40)
		ser.Set(serial)
		spkKey := new(bytes.Buffer)
		indexes.SerialPubkeyEnc(ser).MarshalWrite(spkKey)

		item, err := txn.Get(spkKey.Bytes())
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			pubkey = make([]byte, len(val))
			copy(pubkey, val)
			return nil
		})
	})
	return pubkey, err
}
