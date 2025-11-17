package neo4j

import (
	"context"
	"fmt"

	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/encoders/event"
	"next.orly.dev/pkg/encoders/hex"
	"next.orly.dev/pkg/encoders/tag"
	"next.orly.dev/pkg/interfaces/store"
)

// FetchEventBySerial retrieves an event by its serial number
func (n *N) FetchEventBySerial(ser *types.Uint40) (ev *event.E, err error) {
	serial := ser.Get()

	cypher := `
MATCH (e:Event {serial: $serial})
RETURN e.id AS id,
       e.kind AS kind,
       e.created_at AS created_at,
       e.content AS content,
       e.sig AS sig,
       e.pubkey AS pubkey,
       e.tags AS tags`

	params := map[string]any{"serial": int64(serial)}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch event by serial: %w", err)
	}

	evs, err := n.parseEventsFromResult(result)
	if err != nil {
		return nil, err
	}

	if len(evs) == 0 {
		return nil, fmt.Errorf("event not found")
	}

	return evs[0], nil
}

// FetchEventsBySerials retrieves multiple events by their serial numbers
func (n *N) FetchEventsBySerials(serials []*types.Uint40) (
	events map[uint64]*event.E, err error,
) {
	if len(serials) == 0 {
		return make(map[uint64]*event.E), nil
	}

	// Build list of serial numbers
	serialNums := make([]int64, len(serials))
	for i, ser := range serials {
		serialNums[i] = int64(ser.Get())
	}

	cypher := `
MATCH (e:Event)
WHERE e.serial IN $serials
RETURN e.id AS id,
       e.kind AS kind,
       e.created_at AS created_at,
       e.content AS content,
       e.sig AS sig,
       e.pubkey AS pubkey,
       e.tags AS tags,
       e.serial AS serial`

	params := map[string]any{"serials": serialNums}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch events by serials: %w", err)
	}

	// Parse events and map by serial
	events = make(map[uint64]*event.E)
	ctx := context.Background()

	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if !ok {
		return events, nil
	}

	for neo4jResult.Next(ctx) {
		record := neo4jResult.Record()
		if record == nil {
			continue
		}

		recordMap, ok := (*record).(map[string]any)
		if !ok {
			continue
		}

		// Parse event
		idStr, _ := recordMap["id"].(string)
		kind, _ := recordMap["kind"].(int64)
		createdAt, _ := recordMap["created_at"].(int64)
		content, _ := recordMap["content"].(string)
		sigStr, _ := recordMap["sig"].(string)
		pubkeyStr, _ := recordMap["pubkey"].(string)
		tagsStr, _ := recordMap["tags"].(string)
		serialVal, _ := recordMap["serial"].(int64)

		id, err := hex.Dec(idStr)
		if err != nil {
			continue
		}
		sig, err := hex.Dec(sigStr)
		if err != nil {
			continue
		}
		pubkey, err := hex.Dec(pubkeyStr)
		if err != nil {
			continue
		}

		tags := tag.NewS()
		if tagsStr != "" {
			_ = tags.UnmarshalJSON([]byte(tagsStr))
		}

		e := &event.E{
			Kind:      uint16(kind),
			CreatedAt: createdAt,
			Content:   []byte(content),
			Tags:      tags,
		}

		copy(e.ID[:], id)
		copy(e.Sig[:], sig)
		copy(e.Pubkey[:], pubkey)

		events[uint64(serialVal)] = e
	}

	return events, nil
}

// GetSerialById retrieves the serial number for an event ID
func (n *N) GetSerialById(id []byte) (ser *types.Uint40, err error) {
	idStr := hex.Enc(id)

	cypher := "MATCH (e:Event {id: $id}) RETURN e.serial AS serial"
	params := map[string]any{"id": idStr}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get serial by ID: %w", err)
	}

	ctx := context.Background()
	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if !ok {
		return nil, fmt.Errorf("invalid result type")
	}

	if neo4jResult.Next(ctx) {
		record := neo4jResult.Record()
		if record != nil {
			recordMap, ok := (*record).(map[string]any)
			if ok {
				if serialVal, ok := recordMap["serial"].(int64); ok {
					ser = &types.Uint40{}
					ser.Set(uint64(serialVal))
					return ser, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("event not found")
}

// GetSerialsByIds retrieves serial numbers for multiple event IDs
func (n *N) GetSerialsByIds(ids *tag.T) (
	serials map[string]*types.Uint40, err error,
) {
	serials = make(map[string]*types.Uint40)

	if len(ids.T) == 0 {
		return serials, nil
	}

	// Extract ID strings
	idStrs := make([]string, 0, len(ids.T))
	for _, idTag := range ids.T {
		if len(idTag) >= 2 {
			idStrs = append(idStrs, string(idTag[1]))
		}
	}

	if len(idStrs) == 0 {
		return serials, nil
	}

	cypher := `
MATCH (e:Event)
WHERE e.id IN $ids
RETURN e.id AS id, e.serial AS serial`

	params := map[string]any{"ids": idStrs}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get serials by IDs: %w", err)
	}

	ctx := context.Background()
	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if !ok {
		return serials, nil
	}

	for neo4jResult.Next(ctx) {
		record := neo4jResult.Record()
		if record == nil {
			continue
		}

		recordMap, ok := (*record).(map[string]any)
		if !ok {
			continue
		}

		idStr, _ := recordMap["id"].(string)
		serialVal, _ := recordMap["serial"].(int64)

		serial := &types.Uint40{}
		serial.Set(uint64(serialVal))
		serials[idStr] = serial
	}

	return serials, nil
}

// GetSerialsByIdsWithFilter retrieves serials with a filter function
func (n *N) GetSerialsByIdsWithFilter(
	ids *tag.T, fn func(ev *event.E, ser *types.Uint40) bool,
) (serials map[string]*types.Uint40, err error) {
	serials = make(map[string]*types.Uint40)

	if fn == nil {
		// No filter, just return all
		return n.GetSerialsByIds(ids)
	}

	// With filter, need to fetch events
	for _, idTag := range ids.T {
		if len(idTag) < 2 {
			continue
		}

		idBytes, err := hex.Dec(string(idTag[1]))
		if err != nil {
			continue
		}

		serial, err := n.GetSerialById(idBytes)
		if err != nil {
			continue
		}

		ev, err := n.FetchEventBySerial(serial)
		if err != nil {
			continue
		}

		if fn(ev, serial) {
			serials[string(idTag[1])] = serial
		}
	}

	return serials, nil
}

// GetSerialsByRange retrieves serials within a range
func (n *N) GetSerialsByRange(idx database.Range) (
	serials types.Uint40s, err error,
) {
	// This would need to be implemented based on how ranges are defined
	// For now, returning not implemented
	err = fmt.Errorf("not implemented")
	return
}

// GetFullIdPubkeyBySerial retrieves ID and pubkey for a serial number
func (n *N) GetFullIdPubkeyBySerial(ser *types.Uint40) (
	fidpk *store.IdPkTs, err error,
) {
	serial := ser.Get()

	cypher := `
MATCH (e:Event {serial: $serial})
RETURN e.id AS id,
       e.pubkey AS pubkey,
       e.created_at AS created_at`

	params := map[string]any{"serial": int64(serial)}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get ID and pubkey by serial: %w", err)
	}

	ctx := context.Background()
	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if !ok {
		return nil, fmt.Errorf("invalid result type")
	}

	if neo4jResult.Next(ctx) {
		record := neo4jResult.Record()
		if record != nil {
			recordMap, ok := (*record).(map[string]any)
			if ok {
				idStr, _ := recordMap["id"].(string)
				pubkeyStr, _ := recordMap["pubkey"].(string)
				createdAt, _ := recordMap["created_at"].(int64)

				id, err := hex.Dec(idStr)
				if err != nil {
					return nil, err
				}

				pubkey, err := hex.Dec(pubkeyStr)
				if err != nil {
					return nil, err
				}

				fidpk = &store.IdPkTs{
					Id:  id,
					Pub: pubkey,
					Ts:  createdAt,
					Ser: serial,
				}

				return fidpk, nil
			}
		}
	}

	return nil, fmt.Errorf("event not found")
}

// GetFullIdPubkeyBySerials retrieves IDs and pubkeys for multiple serials
func (n *N) GetFullIdPubkeyBySerials(sers []*types.Uint40) (
	fidpks []*store.IdPkTs, err error,
) {
	fidpks = make([]*store.IdPkTs, 0, len(sers))

	if len(sers) == 0 {
		return fidpks, nil
	}

	// Build list of serial numbers
	serialNums := make([]int64, len(sers))
	for i, ser := range sers {
		serialNums[i] = int64(ser.Get())
	}

	cypher := `
MATCH (e:Event)
WHERE e.serial IN $serials
RETURN e.id AS id,
       e.pubkey AS pubkey,
       e.created_at AS created_at,
       e.serial AS serial`

	params := map[string]any{"serials": serialNums}

	result, err := n.ExecuteRead(context.Background(), cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get IDs and pubkeys by serials: %w", err)
	}

	ctx := context.Background()
	neo4jResult, ok := result.(interface {
		Next(context.Context) bool
		Record() *interface{}
		Err() error
	})
	if !ok {
		return fidpks, nil
	}

	for neo4jResult.Next(ctx) {
		record := neo4jResult.Record()
		if record == nil {
			continue
		}

		recordMap, ok := (*record).(map[string]any)
		if !ok {
			continue
		}

		idStr, _ := recordMap["id"].(string)
		pubkeyStr, _ := recordMap["pubkey"].(string)
		createdAt, _ := recordMap["created_at"].(int64)
		serialVal, _ := recordMap["serial"].(int64)

		id, err := hex.Dec(idStr)
		if err != nil {
			continue
		}

		pubkey, err := hex.Dec(pubkeyStr)
		if err != nil {
			continue
		}

		fidpks = append(fidpks, &store.IdPkTs{
			Id:  id,
			Pub:  pubkey,
			Ts:  createdAt,
			Ser: uint64(serialVal),
		})
	}

	return fidpks, nil
}
