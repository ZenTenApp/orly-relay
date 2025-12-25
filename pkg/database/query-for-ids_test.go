package database

import (
	"testing"

	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"git.mleku.dev/mleku/nostr/encoders/timestamp"
	"next.orly.dev/pkg/utils"
)

func TestQueryForIds(t *testing.T) {
	// Use shared database (read-only test)
	db, ctx := GetSharedDB(t)
	events := GetSharedEvents(t)

	if len(events) < 2 {
		t.Fatalf("Need at least 2 saved events, got %d", len(events))
	}

	idTsPk, err := db.QueryForIds(
		ctx, &filter.F{
			Authors: tag.NewFromBytesSlice(events[1].Pubkey),
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for authors: %v", err)
	}

	if len(idTsPk) < 1 {
		t.Fatalf(
			"got unexpected number of results, expect at least 1, got %d",
			len(idTsPk),
		)
	}
	// Verify that all returned events have the correct author
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true
				if !utils.FastEqual(ev.Pubkey, events[1].Pubkey) {
					t.Fatalf(
						"result %d has incorrect author, got %x, expected %x",
						i, ev.Pubkey, events[1].Pubkey,
					)
				}
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}

	// Test querying by kind
	// Find an event with a specific kind
	testKind := kind.New(1) // Kind 1 is typically text notes
	kindFilter := kind.NewS(testKind)

	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Kinds: kindFilter,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for kinds: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events with the specified kind")
	}

	// Verify the results have the correct kind
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true
				if ev.Kind != testKind.K {
					t.Fatalf(
						"result %d has incorrect kind, got %d, expected %d",
						i, ev.Kind, testKind.K,
					)
				}
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}

	// Test querying by tag
	// Find an event with tags to use for testing
	var testTag *tag.T
	var testEventForTag = findEventWithTag(events)

	if testEventForTag != nil {
		// Get the first tag with at least 2 elements and first element of length 1
		for _, tg := range *testEventForTag.Tags {
			if tg.Len() >= 2 && len(tg.Key()) == 1 {
				testTag = tg
				break
			}
		}

		// Create a tags filter with the test tag
		tagsFilter := tag.NewS(testTag)

		idTsPk, err = db.QueryForIds(
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
				if utils.FastEqual(result.Id, ev.ID) {
					found = true

					// Check if the event has the tag we're looking for
					var hasTag bool
					for _, tg := range *ev.Tags {
						if tg.Len() >= 2 && len(tg.Key()) == 1 {
							if utils.FastEqual(
								tg.Key(), testTag.Key(),
							) && utils.FastEqual(tg.Value(), testTag.Value()) {
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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}

		// Test querying by kind and author
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Kinds:   kindFilter,
				Authors: tag.NewFromBytesSlice(events[1].Pubkey),
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for kinds and authors: %v", err)
		}

		// Verify we got results
		if len(idTsPk) > 0 {
			// Verify the results have the correct kind and author
			for i, result := range idTsPk {
				// Find the event with this ID
				var found bool
				for _, ev := range events {
					if utils.FastEqual(result.Id, ev.ID) {
						found = true
						if ev.Kind != testKind.K {
							t.Fatalf(
								"result %d has incorrect kind, got %d, expected %d",
								i, ev.Kind, testKind.K,
							)
						}
						if !utils.FastEqual(ev.Pubkey, events[1].Pubkey) {
							t.Fatalf(
								"result %d has incorrect author, got %x, expected %x",
								i, ev.Pubkey, events[1].Pubkey,
							)
						}
						break
					}
				}
				if !found {
					t.Fatalf(
						"result %d with ID %x not found in events", i,
						result.Id,
					)
				}
			}
		}

		// Test querying by kind and tag
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Kinds: kind.NewS(kind.New(testEventForTag.Kind)),
				Tags:  tagsFilter,
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for kinds and tags: %v", err)
		}

		// Verify we got results
		if len(idTsPk) == 0 {
			t.Fatal("did not find any events with the specified kind and tag")
		}

		// Verify the results have the correct kind and tag
		for i, result := range idTsPk {
			// Find the event with this ID
			var found bool
			for _, ev := range events {
				if utils.FastEqual(result.Id, ev.ID) {
					found = true
					if ev.Kind != testEventForTag.Kind {
						t.Fatalf(
							"result %d has incorrect kind, got %d, expected %d",
							i, ev.Kind, testEventForTag.Kind,
						)
					}

					// Check if the event has the tag we're looking for
					var hasTag bool
					for _, tg := range *ev.Tags {
						if tg.Len() >= 2 && len(tg.Key()) == 1 {
							if utils.FastEqual(
								tg.Key(), testTag.Key(),
							) && utils.FastEqual(tg.Value(), testTag.Value()) {
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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}

		// Test querying by kind, author, and tag
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Kinds:   kind.NewS(kind.New(testEventForTag.Kind)),
				Authors: tag.NewFromBytesSlice(testEventForTag.Pubkey),
				Tags:    tagsFilter,
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for kinds, authors, and tags: %v", err)
		}

		// Verify we got results
		if len(idTsPk) == 0 {
			t.Fatal("did not find any events with the specified kind, author, and tag")
		}

		// Verify the results have the correct kind, author, and tag
		for i, result := range idTsPk {
			// Find the event with this ID
			var found bool
			for _, ev := range events {
				if utils.FastEqual(result.Id, ev.ID) {
					found = true
					if ev.Kind != testEventForTag.Kind {
						t.Fatalf(
							"result %d has incorrect kind, got %d, expected %d",
							i, ev.Kind, testEventForTag.Kind,
						)
					}

					if !utils.FastEqual(ev.Pubkey, testEventForTag.Pubkey) {
						t.Fatalf(
							"result %d has incorrect author, got %x, expected %x",
							i, ev.Pubkey, testEventForTag.Pubkey,
						)
					}

					// Check if the event has the tag we're looking for
					var hasTag bool
					for _, tg := range *ev.Tags {
						if tg.Len() >= 2 && len(tg.Key()) == 1 {
							if utils.FastEqual(
								tg.Key(), testTag.Key(),
							) && utils.FastEqual(tg.Value(), testTag.Value()) {
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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}

		// Test querying by author and tag
		idTsPk, err = db.QueryForIds(
			ctx, &filter.F{
				Authors: tag.NewFromBytesSlice(testEventForTag.Pubkey),
				Tags:    tagsFilter,
			},
		)
		if err != nil {
			t.Fatalf("Failed to query for authors and tags: %v", err)
		}

		// Verify we got results
		if len(idTsPk) == 0 {
			t.Fatal("did not find any events with the specified author and tag")
		}

		// Verify the results have the correct author and tag
		for i, result := range idTsPk {
			// Find the event with this ID
			var found bool
			for _, ev := range events {
				if utils.FastEqual(result.Id, ev.ID) {
					found = true

					if !utils.FastEqual(ev.Pubkey, testEventForTag.Pubkey) {
						t.Fatalf(
							"result %d has incorrect author, got %x, expected %x",
							i, ev.Pubkey, testEventForTag.Pubkey,
						)
					}

					// Check if the event has the tag we're looking for
					var hasTag bool
					for _, tg := range *ev.Tags {
						if tg.Len() >= 2 && len(tg.Key()) == 1 {
							if utils.FastEqual(
								tg.Key(), testTag.Key(),
							) && utils.FastEqual(tg.Value(), testTag.Value()) {
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
				t.Fatalf(
					"result %d with ID %x not found in events", i, result.Id,
				)
			}
		}
	}

	// Test querying by created_at range
	// Use the timestamp from the middle event as a reference
	middleIndex := len(events) / 2
	middleEvent := events[middleIndex]

	// Create a timestamp range that includes events before and after the middle event
	sinceTime := new(timestamp.T)
	sinceTime.V = middleEvent.CreatedAt - 3600 // 1 hour before middle event

	untilTime := new(timestamp.T)
	untilTime.V = middleEvent.CreatedAt + 3600 // 1 hour after middle event

	idTsPk, err = db.QueryForIds(
		ctx, &filter.F{
			Since: sinceTime,
			Until: untilTime,
		},
	)
	if err != nil {
		t.Fatalf("Failed to query for created_at range: %v", err)
	}

	// Verify we got results
	if len(idTsPk) == 0 {
		t.Fatal("did not find any events in the specified time range")
	}

	// Verify the results exist in our events slice
	for i, result := range idTsPk {
		// Find the event with this ID
		var found bool
		for _, ev := range events {
			if utils.FastEqual(result.Id, ev.ID) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("result %d with ID %x not found in events", i, result.Id)
		}
	}
}
