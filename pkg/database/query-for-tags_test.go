package database

import (
	"bytes"
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

func TestQueryForTags(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	// Find an event with tags to use for testing
	testEvent := findEventWithTag(events)

	if testEvent == nil {
		t.Skip("No suitable event with tags found for testing")
	}

	// Get the first tag with at least 2 elements and first element of length 1
	var testTag *tag.T
	for _, tg := range *testEvent.Tags {
		if tg.Len() >= 2 && len(tg.Key()) == 1 {
			testTag = tg
			break
		}
	}

	// Test querying by tag only
	// Create a tags filter with the test tag
	tagsFilter := tag.NewS(testTag)

	idTsPk, err := db.QueryForIds(
		ctx, &filter.F{
			Tags: tagsFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for tags: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with the specified tag")
	}

	// Verify the results have the correct tag
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if bytes.Equal(result.Id[:], ev.ID[:]) {
				found = true

				// Check if the event has the tag we're looking for
				var hasTag bool
				for _, tg := range *ev.Tags {
					if tg.Len() >= 2 && len(tg.Key()) == 1 {
						if bytes.Equal(
							tg.Key(), testTag.Key(),
						) && bytes.Equal(tg.Value(), testTag.Value()) {
							hasTag = true
							break
						}
					}
				}

				if !hasTag {
					t.Fatalf(
						"result %d does not have the expected tag",
						i,
					)
				}

				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}
}
