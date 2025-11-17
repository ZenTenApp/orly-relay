package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/filter"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
)

// QueryEvents retrieves events matching the given filter
func (d *D) QueryEvents(c context.Context, f *filter.F) (evs event.S, err error) {
	return d.QueryEventsWithOptions(c, f, false, false)
}

// QueryAllVersions retrieves all versions of events matching the filter
func (d *D) QueryAllVersions(c context.Context, f *filter.F) (evs event.S, err error) {
	return d.QueryEventsWithOptions(c, f, false, true)
}

// QueryEventsWithOptions retrieves events with specific options
func (d *D) QueryEventsWithOptions(
	c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool,
) (evs event.S, err error) {
	// Build DQL query from Nostr filter
	query := d.buildDQLQuery(f, includeDeleteEvents)

	// Execute query
	resp, err := d.Query(c, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Parse response
	evs, err = d.parseEventsFromResponse(resp.Json)
	if err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}

	return evs, nil
}

// buildDQLQuery constructs a DQL query from a Nostr filter
func (d *D) buildDQLQuery(f *filter.F, includeDeleteEvents bool) string {
	return d.buildDQLQueryWithFields(f, includeDeleteEvents, []string{
		"uid",
		"event.id",
		"event.kind",
		"event.created_at",
		"event.content",
		"event.sig",
		"event.pubkey",
		"event.tags",
	})
}

// buildDQLQueryWithFields constructs a DQL query with custom field selection
func (d *D) buildDQLQueryWithFields(f *filter.F, includeDeleteEvents bool, fields []string) string {
	var conditions []string
	var funcQuery string

	// IDs filter
	if len(f.Ids.T) > 0 {
		idConditions := make([]string, len(f.Ids.T))
		for i, id := range f.Ids.T {
			// Handle prefix matching
			if len(id) < 64 {
				// Prefix search
				idConditions[i] = fmt.Sprintf("regexp(event.id, /^%s/)", hex.Enc(id))
			} else {
				idConditions[i] = fmt.Sprintf("eq(event.id, %q)", hex.Enc(id))
			}
		}
		if len(idConditions) == 1 {
			funcQuery = idConditions[0]
		} else {
			conditions = append(conditions, "("+strings.Join(idConditions, " OR ")+")")
		}
	}

	// Authors filter
	if len(f.Authors.T) > 0 {
		authorConditions := make([]string, len(f.Authors.T))
		for i, author := range f.Authors.T {
			// Handle prefix matching
			if len(author) < 64 {
				authorConditions[i] = fmt.Sprintf("regexp(event.pubkey, /^%s/)", hex.Enc(author))
			} else {
				authorConditions[i] = fmt.Sprintf("eq(event.pubkey, %q)", hex.Enc(author))
			}
		}
		if funcQuery == "" && len(authorConditions) == 1 {
			funcQuery = authorConditions[0]
		} else {
			conditions = append(conditions, "("+strings.Join(authorConditions, " OR ")+")")
		}
	}

	// Kinds filter
	if len(f.Kinds.K) > 0 {
		kindConditions := make([]string, len(f.Kinds.K))
		for i, kind := range f.Kinds.K {
			kindConditions[i] = fmt.Sprintf("eq(event.kind, %d)", kind)
		}
		conditions = append(conditions, "("+strings.Join(kindConditions, " OR ")+")")
	}

	// Time range filters
	if f.Since != nil {
		conditions = append(conditions, fmt.Sprintf("ge(event.created_at, %d)", f.Since.V))
	}
	if f.Until != nil {
		conditions = append(conditions, fmt.Sprintf("le(event.created_at, %d)", f.Until.V))
	}

	// Tag filters
	for _, tagValues := range *f.Tags {
		if len(tagValues.T) > 0 {
			tagConditions := make([]string, len(tagValues.T))
			for i, tagValue := range tagValues.T {
				// This is a simplified tag query - in production you'd want to use facets
				tagConditions[i] = fmt.Sprintf("eq(tag.value, %q)", string(tagValue))
			}
			conditions = append(conditions, "("+strings.Join(tagConditions, " OR ")+")")
		}
	}

	// Exclude delete events unless requested
	if !includeDeleteEvents {
		conditions = append(conditions, "NOT eq(event.kind, 5)")
	}

	// Build the final query
	if funcQuery == "" {
		funcQuery = "has(event.id)"
	}

	filterStr := ""
	if len(conditions) > 0 {
		filterStr = " @filter(" + strings.Join(conditions, " AND ") + ")"
	}

	// Add ordering and limit
	orderBy := ", orderdesc: event.created_at"
	limitStr := ""
	if *f.Limit > 0 {
		limitStr = fmt.Sprintf(", first: %d", f.Limit)
	}

	// Build field list
	fieldStr := strings.Join(fields, "\n\t\t\t")

	query := fmt.Sprintf(`{
		events(func: %s%s%s%s) {
			%s
		}
	}`, funcQuery, filterStr, orderBy, limitStr, fieldStr)

	return query
}

// parseEventsFromResponse converts Dgraph JSON response to Nostr events
func (d *D) parseEventsFromResponse(jsonData []byte) ([]*event.E, error) {
	var result struct {
		Events []struct {
			UID       string `json:"uid"`
			ID        string `json:"event.id"`
			Kind      int    `json:"event.kind"`
			CreatedAt int64  `json:"event.created_at"`
			Content   string `json:"event.content"`
			Sig       string `json:"event.sig"`
			Pubkey    string `json:"event.pubkey"`
			Tags      string `json:"event.tags"`
		} `json:"events"`
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	events := make([]*event.E, 0, len(result.Events))
	for _, ev := range result.Events {
		// Decode hex strings
		id, err := hex.Dec(ev.ID)
		if err != nil {
			continue
		}
		sig, err := hex.Dec(ev.Sig)
		if err != nil {
			continue
		}
		pubkey, err := hex.Dec(ev.Pubkey)
		if err != nil {
			continue
		}

		// Parse tags from JSON
		var tags tag.S
		if ev.Tags != "" {
			if err := json.Unmarshal([]byte(ev.Tags), &tags); err != nil {
				continue
			}
		}

		// Create event
		e := &event.E{
			Kind:      uint16(ev.Kind),
			CreatedAt: ev.CreatedAt,
			Content:   []byte(ev.Content),
			Tags:      &tags,
		}

		// Copy fixed-size arrays
		copy(e.ID[:], id)
		copy(e.Sig[:], sig)
		copy(e.Pubkey[:], pubkey)

		events = append(events, e)
	}

	return events, nil
}

// QueryDeleteEventsByTargetId retrieves delete events targeting a specific event ID
func (d *D) QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (
	evs event.S, err error,
) {
	targetIDStr := hex.Enc(targetEventId)

	// Query for kind 5 events that reference this event
	query := fmt.Sprintf(`{
		events(func: eq(event.kind, 5)) {
			uid
			event.id
			event.kind
			event.created_at
			event.content
			event.sig
			event.pubkey
			event.tags
			references @filter(eq(event.id, %q)) {
				event.id
			}
		}
	}`, targetIDStr)

	resp, err := d.Query(c, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query delete events: %w", err)
	}

	evs, err = d.parseEventsFromResponse(resp.Json)
	if err != nil {
		return nil, fmt.Errorf("failed to parse delete events: %w", err)
	}

	return evs, nil
}

// QueryForSerials retrieves event serials matching a filter
func (d *D) QueryForSerials(c context.Context, f *filter.F) (
	serials types.Uint40s, err error,
) {
	// Build query requesting only serial numbers
	query := d.buildDQLQueryWithFields(f, false, []string{"event.serial"})

	resp, err := d.Query(c, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query serials: %w", err)
	}

	var result struct {
		Events []struct {
			Serial int64 `json:"event.serial"`
		} `json:"events"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	serials = make([]*types.Uint40, 0, len(result.Events))
	for _, ev := range result.Events {
		serial := types.Uint40{}
		serial.Set(uint64(ev.Serial))
		serials = append(serials, &serial)
	}

	return serials, nil
}

// QueryForIds retrieves event IDs matching a filter
func (d *D) QueryForIds(c context.Context, f *filter.F) (
	idPkTs []*store.IdPkTs, err error,
) {
	// Build query requesting only ID, pubkey, created_at, serial
	query := d.buildDQLQueryWithFields(f, false, []string{
		"event.id",
		"event.pubkey",
		"event.created_at",
		"event.serial",
	})

	resp, err := d.Query(c, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query IDs: %w", err)
	}

	var result struct {
		Events []struct {
			ID        string `json:"event.id"`
			Pubkey    string `json:"event.pubkey"`
			CreatedAt int64  `json:"event.created_at"`
			Serial    int64  `json:"event.serial"`
		} `json:"events"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	idPkTs = make([]*store.IdPkTs, 0, len(result.Events))
	for _, ev := range result.Events {
		id, err := hex.Dec(ev.ID)
		if err != nil {
			continue
		}
		pubkey, err := hex.Dec(ev.Pubkey)
		if err != nil {
			continue
		}
		idPkTs = append(idPkTs, &store.IdPkTs{
			Id:  id,
			Pub: pubkey,
			Ts:  ev.CreatedAt,
			Ser: uint64(ev.Serial),
		})
	}

	return idPkTs, nil
}

// CountEvents counts events matching a filter
func (d *D) CountEvents(c context.Context, f *filter.F) (
	count int, approximate bool, err error,
) {
	// Build query requesting only count
	query := d.buildDQLQueryWithFields(f, false, []string{"count(uid)"})

	resp, err := d.Query(c, query)
	if err != nil {
		return 0, false, fmt.Errorf("failed to count events: %w", err)
	}

	var result struct {
		Events []struct {
			Count int `json:"count"`
		} `json:"events"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return 0, false, err
	}

	if len(result.Events) > 0 {
		count = result.Events[0].Count
	}

	return count, false, nil
}
