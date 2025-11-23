package dgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
)

// FetchEventBySerial retrieves an event by its serial number
func (d *D) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	serial := ser.Get()

	query := fmt.Sprintf(`{
		event(func: eq(event.serial, %d)) {
			event.id
			event.kind
			event.created_at
			event.content
			event.sig
			event.pubkey
			event.tags
		}
	}`, serial)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event by serial: %w", err)
	}

	evs, err := d.parseEventsFromResponse(resp.Json)
	if err != nil {
		return nil, err
	}

	if len(evs) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	return evs[0], nil
}

// FetchEventsBySerials retrieves multiple events by their serial numbers
func (d *D) FetchEventsBySerials(serials []*types.Uint40) (
	events map[uint64]*event.E, err error,
) {
	if len(serials) == 0 {
		return make(map[uint64]*event.E), nil
	}

	// Build a filter for multiple serials using OR conditions
	serialConditions := make([]string, len(serials))
	for i, ser := range serials {
		serialConditions[i] = fmt.Sprintf("eq(event.serial, %d)", ser.Get())
	}
	serialFilter := strings.Join(serialConditions, " OR ")

	// Query with proper batch filtering
	query := fmt.Sprintf(`{
		events(func: has(event.serial)) @filter(%s) {
			event.id
			event.kind
			event.created_at
			event.content
			event.sig
			event.pubkey
			event.tags
			event.serial
		}
	}`, serialFilter)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events by serials: %w", err)
	}

	// Parse the response including serial numbers
	var result struct {
		Events []struct {
			ID        string `json:"event.id"`
			Kind      int    `json:"event.kind"`
			CreatedAt int64  `json:"event.created_at"`
			Content   string `json:"event.content"`
			Sig       string `json:"event.sig"`
			Pubkey    string `json:"event.pubkey"`
			Tags      string `json:"event.tags"`
			Serial    int64  `json:"event.serial"`
		} `json:"events"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	// Map events by their serial numbers
	events = make(map[uint64]*event.E)
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

		events[uint64(ev.Serial)] = e
	}

	return events, nil
}

// GetSerialById retrieves the serial number for an event ID
func (d *D) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	idStr := hex.Enc(id)

	query := fmt.Sprintf(`{
		event(func: eq(event.id, %q)) {
			event.serial
		}
	}`, idStr)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to get serial by ID: %w", err)
	}

	var result struct {
		Event []struct {
			Serial int64 `json:"event.serial"`
		} `json:"event"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	if len(result.Event) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	ser = &types.Uint40{}
	ser.Set(uint64(result.Event[0].Serial))

	return ser, nil
}

// GetSerialsByIds retrieves serial numbers for multiple event IDs
func (d *D) GetSerialsByIds(ids *tag.T) (
	serials map[string]*types.Uint40, err error,
) {
	serials = make(map[string]*types.Uint40)

	if len(ids.T) == 0 {
		return serials, nil
	}

	// Build batch query for all IDs at once
	idConditions := make([]string, 0, len(ids.T))
	idMap := make(map[string][]byte) // Map hex ID to original bytes

	for _, idBytes := range ids.T {
		if len(idBytes) > 0 {
			idStr := hex.Enc(idBytes)
			idConditions = append(idConditions, fmt.Sprintf("eq(event.id, %q)", idStr))
			idMap[idStr] = idBytes
		}
	}

	if len(idConditions) == 0 {
		return serials, nil
	}

	// Create single query with OR conditions
	idFilter := strings.Join(idConditions, " OR ")
	query := fmt.Sprintf(`{
		events(func: has(event.id)) @filter(%s) {
			event.id
			event.serial
		}
	}`, idFilter)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to batch query serials by IDs: %w", err)
	}

	var result struct {
		Events []struct {
			ID     string `json:"event.id"`
			Serial int64  `json:"event.serial"`
		} `json:"events"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	// Map results back
	for _, ev := range result.Events {
		serial := types.Uint40{}
		serial.Set(uint64(ev.Serial))
		serials[ev.ID] = &serial
	}

	return serials, nil
}

// GetSerialsByIdsWithFilter retrieves serials with a filter function
func (d *D) GetSerialsByIdsWithFilter(
	ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool,
) (serials map[string]*types.Uint40, err error) {
	serials = make(map[string]*types.Uint40)

	if fn == nil {
		// No filter, just return all
		return d.GetSerialsByIds(ids)
	}

	// With filter, need to fetch events
	for _, id := range ids.T {
		if len(id) > 0 {
			serial, err := d.GetSerialById(id)
			if err != nil {
				continue
			}

			ev, err := d.FetchEventBySerial(serial)
			if err != nil {
				continue
			}

			if fn(ev, serial) {
				serials[string(id)] = serial
			}
		}
	}

	return serials, nil
}

// GetSerialsByRange retrieves serials within a range
func (d *D) GetSerialsByRange(idx database.Range) (
	serials types.Uint40s, err error,
) {
	// Range represents a byte-prefix range for index scanning
	// For dgraph, we need to convert this to a query on indexed fields
	// The range is typically used for scanning event IDs or other hex-encoded keys

	if len(idx.Start) == 0 && len(idx.End) == 0 {
		return nil, fmt.Errorf("empty range provided")
	}

	startStr := hex.Enc(idx.Start)
	endStr := hex.Enc(idx.End)

	// Query for events with IDs in the specified range
	query := fmt.Sprintf(`{
		events(func: ge(event.id, %q)) @filter(le(event.id, %q)) {
			event.serial
		}
	}`, startStr, endStr)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to query serials by range: %w", err)
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

// GetFullIdPubkeyBySerial retrieves ID and pubkey for a serial number
func (d *D) GetFullIdPubkeyBySerial(ser *types.Uint40) (
	fidpk *store.IdPkTs, err error,
) {
	serial := ser.Get()

	query := fmt.Sprintf(`{
		event(func: eq(event.serial, %d)) {
			event.id
			event.pubkey
			event.created_at
		}
	}`, serial)

	resp, err := d.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("failed to get ID and pubkey by serial: %w", err)
	}

	var result struct {
		Event []struct {
			ID        string `json:"event.id"`
			Pubkey    string `json:"event.pubkey"`
			CreatedAt int64  `json:"event.created_at"`
		} `json:"event"`
	}

	if err = json.Unmarshal(resp.Json, &result); err != nil {
		return nil, err
	}

	if len(result.Event) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	id, err := hex.Dec(result.Event[0].ID)
	if err != nil {
		return nil, err
	}

	pubkey, err := hex.Dec(result.Event[0].Pubkey)
	if err != nil {
		return nil, err
	}

	fidpk = &store.IdPkTs{
		Id:  id,
		Pub: pubkey,
		Ts:  result.Event[0].CreatedAt,
		Ser: serial,
	}

	return fidpk, nil
}

// GetFullIdPubkeyBySerials retrieves IDs and pubkeys for multiple serials
func (d *D) GetFullIdPubkeyBySerials(sers []*types.Uint40) (
	fidpks []*store.IdPkTs, err error,
) {
	fidpks = make([]*store.IdPkTs, 0, len(sers))

	for _, ser := range sers {
		fidpk, err := d.GetFullIdPubkeyBySerial(ser)
		if err != nil {
			continue // Skip errors, continue with others
		}
		fidpks = append(fidpks, fidpk)
	}

	return fidpks, nil
}
