//go:build !(js && wasm)

package database

import (
	"context"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// CleanupKind3WithoutPTags scans for kind 3 follow list events that have no p tags
// and deletes them. This cleanup is needed because the directory spider may have
// saved malformed events that lost their tags during serialization.
func (d *D) CleanupKind3WithoutPTags(ctx context.Context) error {
	log.I.F("database: starting cleanup of kind 3 events without p tags")

	startTime := time.Now()

	// Query for all kind 3 events
	f := &filter.F{
		Kinds: kind.NewS(kind.FollowList),
	}

	events, err := d.QueryEvents(ctx, f)
	if chk.E(err) {
		return err
	}

	deletedCount := 0

	// Check each event for p tags
	for _, ev := range events {
		hasPTag := false

		if ev.Tags != nil && ev.Tags.Len() > 0 {
			// Look for at least one p tag
			for _, tag := range *ev.Tags {
				if tag != nil && tag.Len() >= 2 {
					key := tag.Key()
					if len(key) == 1 && key[0] == 'p' {
						hasPTag = true
						break
					}
				}
			}
		}

		// Delete events without p tags
		if !hasPTag {
			log.W.F("database: deleting kind 3 event without p tags from pubkey %x", ev.Pubkey)
			if err := d.DeleteEvent(ctx, ev.ID); chk.E(err) {
				log.E.F("database: failed to delete kind 3 event %x: %v", ev.ID, err)
				continue
			}
			deletedCount++
		}
	}

	duration := time.Since(startTime)

	if deletedCount > 0 {
		log.I.F("database: cleanup completed in %v - deleted %d kind 3 events without p tags (scanned %d total)",
			duration, deletedCount, len(events))
	} else {
		log.I.F("database: cleanup completed in %v - no kind 3 events needed deletion (scanned %d total)",
			duration, len(events))
	}

	return nil
}
