package neo4j

import (
	"context"
	"fmt"
	"strings"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
)

// QueryEvents retrieves events matching the given filter
func (n *N) QueryEvents(c context.Context, f *filter.F) (evs event.S, err error) {
	return n.QueryEventsWithOptions(c, f, false, false)
}

// QueryAllVersions retrieves all versions of events matching the filter
func (n *N) QueryAllVersions(c context.Context, f *filter.F) (evs event.S, err error) {
	return n.QueryEventsWithOptions(c, f, false, true)
}

// QueryEventsWithOptions retrieves events with specific options
func (n *N) QueryEventsWithOptions(
	c context.Context, f *filter.F, includeDeleteEvents bool, showAllVersions bool,
) (evs event.S, err error) {
	// Build Cypher query from Nostr filter
	cypher, params := n.buildCypherQuery(f, includeDeleteEvents)

	// Execute query
	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Parse response
	evs, err = n.parseEventsFromResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}

	return evs, nil
}

// buildCypherQuery constructs a Cypher query from a Nostr filter
// This is the core translation layer between Nostr's REQ filter format and Neo4j's Cypher
func (n *N) buildCypherQuery(f *filter.F, includeDeleteEvents bool) (string, map[string]any) {
	params := make(map[string]any)
	var whereClauses []string

	// Start with basic MATCH clause
	matchClause := "MATCH (e:Event)"

	// IDs filter - uses exact match or prefix matching
	if len(f.Ids.T) > 0 {
		idConditions := make([]string, len(f.Ids.T))
		for i, id := range f.Ids.T {
			paramName := fmt.Sprintf("id_%d", i)
			hexID := hex.Enc(id)

			// Handle prefix matching for partial IDs
			if len(id) < 32 { // Full event ID is 32 bytes (64 hex chars)
				idConditions[i] = fmt.Sprintf("e.id STARTS WITH $%s", paramName)
			} else {
				idConditions[i] = fmt.Sprintf("e.id = $%s", paramName)
			}
			params[paramName] = hexID
		}
		whereClauses = append(whereClauses, "("+strings.Join(idConditions, " OR ")+")")
	}

	// Authors filter - supports prefix matching for partial pubkeys
	if len(f.Authors.T) > 0 {
		authorConditions := make([]string, len(f.Authors.T))
		for i, author := range f.Authors.T {
			paramName := fmt.Sprintf("author_%d", i)
			hexAuthor := hex.Enc(author)

			// Handle prefix matching for partial pubkeys
			if len(author) < 32 { // Full pubkey is 32 bytes (64 hex chars)
				authorConditions[i] = fmt.Sprintf("e.pubkey STARTS WITH $%s", paramName)
			} else {
				authorConditions[i] = fmt.Sprintf("e.pubkey = $%s", paramName)
			}
			params[paramName] = hexAuthor
		}
		whereClauses = append(whereClauses, "("+strings.Join(authorConditions, " OR ")+")")
	}

	// Kinds filter - matches event types
	if len(f.Kinds.K) > 0 {
		kinds := make([]int64, len(f.Kinds.K))
		for i, k := range f.Kinds.K {
			kinds[i] = int64(k.K)
		}
		params["kinds"] = kinds
		whereClauses = append(whereClauses, "e.kind IN $kinds")
	}

	// Time range filters - for temporal queries
	if f.Since != nil {
		params["since"] = f.Since.V
		whereClauses = append(whereClauses, "e.created_at >= $since")
	}
	if f.Until != nil {
		params["until"] = f.Until.V
		whereClauses = append(whereClauses, "e.created_at <= $until")
	}

	// Tag filters - this is where Neo4j's graph capabilities shine
	// We can efficiently traverse tag relationships
	tagIndex := 0
	for _, tagValues := range *f.Tags {
		if len(tagValues.T) > 0 {
			tagVarName := fmt.Sprintf("t%d", tagIndex)
			tagTypeParam := fmt.Sprintf("tagType_%d", tagIndex)
			tagValuesParam := fmt.Sprintf("tagValues_%d", tagIndex)

			// Add tag relationship to MATCH clause
			matchClause += fmt.Sprintf(" OPTIONAL MATCH (e)-[:TAGGED_WITH]->(%s:Tag)", tagVarName)

			// The first element is the tag type (e.g., "e", "p", etc.)
			tagType := string(tagValues.T[0])

			// Convert remaining tag values to strings (skip first element which is the type)
			tagValueStrings := make([]string, len(tagValues.T)-1)
			for i, tv := range tagValues.T[1:] {
				tagValueStrings[i] = string(tv)
			}

			// Add WHERE conditions for this tag
			params[tagTypeParam] = tagType
			params[tagValuesParam] = tagValueStrings
			whereClauses = append(whereClauses,
				fmt.Sprintf("(%s.type = $%s AND %s.value IN $%s)",
					tagVarName, tagTypeParam, tagVarName, tagValuesParam))

			tagIndex++
		}
	}

	// Exclude delete events unless requested
	if !includeDeleteEvents {
		whereClauses = append(whereClauses, "e.kind <> 5")
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Build RETURN clause with all event properties
	returnClause := `
RETURN e.id AS id,
       e.kind AS kind,
       e.created_at AS created_at,
       e.content AS content,
       e.sig AS sig,
       e.pubkey AS pubkey,
       e.tags AS tags,
       e.serial AS serial`

	// Add ordering (most recent first)
	orderClause := " ORDER BY e.created_at DESC"

	// Add limit if specified
	limitClause := ""
	if f.Limit != nil && *f.Limit > 0 {
		params["limit"] = *f.Limit
		limitClause = " LIMIT $limit"
	}

	// Combine all parts
	cypher := matchClause + whereClause + returnClause + orderClause + limitClause

	return cypher, params
}

// parseEventsFromResult converts Neo4j query results to Nostr events
func (n *N) parseEventsFromResult(result *CollectedResult) ([]*event.E, error) {
	events := make([]*event.E, 0)
	ctx := context.Background()

	// Iterate through result records
	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			continue
		}

		// Parse event fields
		idRaw, _ := record.Get("id")
		kindRaw, _ := record.Get("kind")
		createdAtRaw, _ := record.Get("created_at")
		contentRaw, _ := record.Get("content")
		sigRaw, _ := record.Get("sig")
		pubkeyRaw, _ := record.Get("pubkey")
		tagsRaw, _ := record.Get("tags")

		idStr, _ := idRaw.(string)
		kind, _ := kindRaw.(int64)
		createdAt, _ := createdAtRaw.(int64)
		content, _ := contentRaw.(string)
		sigStr, _ := sigRaw.(string)
		pubkeyStr, _ := pubkeyRaw.(string)
		tagsStr, _ := tagsRaw.(string)

		// Decode hex strings
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

		// Parse tags from JSON
		tags := tag.NewS()
		if tagsStr != "" {
			_ = tags.UnmarshalJSON([]byte(tagsStr))
		}

		// Create event
		e := &event.E{
			Kind:      uint16(kind),
			CreatedAt: createdAt,
			Content:   []byte(content),
			Tags:      tags,
		}

		// Copy fixed-size arrays
		copy(e.ID[:], id)
		copy(e.Sig[:], sig)
		copy(e.Pubkey[:], pubkey)

		events = append(events, e)
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return events, nil
}

// QueryDeleteEventsByTargetId retrieves delete events targeting a specific event ID
func (n *N) QueryDeleteEventsByTargetId(c context.Context, targetEventId []byte) (
	evs event.S, err error,
) {
	targetIDStr := hex.Enc(targetEventId)

	// Query for kind 5 events that reference this event
	// This uses Neo4j's graph traversal to find delete events
	cypher := `
MATCH (target:Event {id: $targetId})
MATCH (e:Event {kind: 5})-[:REFERENCES]->(target)
RETURN e.id AS id,
       e.kind AS kind,
       e.created_at AS created_at,
       e.content AS content,
       e.sig AS sig,
       e.pubkey AS pubkey,
       e.tags AS tags,
       e.serial AS serial
ORDER BY e.created_at DESC`

	params := map[string]any{"targetId": targetIDStr}

	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query delete events: %w", err)
	}

	evs, err = n.parseEventsFromResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse delete events: %w", err)
	}

	return evs, nil
}

// QueryForSerials retrieves event serials matching a filter
func (n *N) QueryForSerials(c context.Context, f *filter.F) (
	serials types.Uint40s, err error,
) {
	// Build query but only return serial numbers
	cypher, params := n.buildCypherQuery(f, false)

	// Replace RETURN clause to only fetch serials
	returnClause := " RETURN e.serial AS serial"
	cypherParts := strings.Split(cypher, "RETURN")
	if len(cypherParts) < 2 {
		return nil, fmt.Errorf("invalid query structure")
	}

	// Rebuild query with serial-only return
	cypher = cypherParts[0] + returnClause
	if strings.Contains(cypherParts[1], "ORDER BY") {
		orderPart := " ORDER BY" + strings.Split(cypherParts[1], "ORDER BY")[1]
		cypher += orderPart
	}

	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query serials: %w", err)
	}

	// Parse serials from result
	serials = make([]*types.Uint40, 0)
	ctx := context.Background()

	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			continue
		}

		serialRaw, found := record.Get("serial")
		if !found {
			continue
		}

		serialVal, ok := serialRaw.(int64)
		if !ok {
			continue
		}

		serial := types.Uint40{}
		serial.Set(uint64(serialVal))
		serials = append(serials, &serial)
	}

	return serials, nil
}

// QueryForIds retrieves event IDs matching a filter
func (n *N) QueryForIds(c context.Context, f *filter.F) (
	idPkTs []*store.IdPkTs, err error,
) {
	// Build query but only return ID, pubkey, created_at, serial
	cypher, params := n.buildCypherQuery(f, false)

	// Replace RETURN clause
	returnClause := `
 RETURN e.id AS id,
        e.pubkey AS pubkey,
        e.created_at AS created_at,
        e.serial AS serial`

	cypherParts := strings.Split(cypher, "RETURN")
	if len(cypherParts) < 2 {
		return nil, fmt.Errorf("invalid query structure")
	}

	cypher = cypherParts[0] + returnClause
	if strings.Contains(cypherParts[1], "ORDER BY") {
		orderPart := " ORDER BY" + strings.Split(cypherParts[1], "ORDER BY")[1]
		cypher += orderPart
	}

	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to query IDs: %w", err)
	}

	// Parse IDs from result
	idPkTs = make([]*store.IdPkTs, 0)
	ctx := context.Background()

	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			continue
		}

		idRaw, _ := record.Get("id")
		pubkeyRaw, _ := record.Get("pubkey")
		createdAtRaw, _ := record.Get("created_at")
		serialRaw, _ := record.Get("serial")

		idStr, _ := idRaw.(string)
		pubkeyStr, _ := pubkeyRaw.(string)
		createdAt, _ := createdAtRaw.(int64)
		serialVal, _ := serialRaw.(int64)

		id, err := hex.Dec(idStr)
		if err != nil {
			continue
		}
		pubkey, err := hex.Dec(pubkeyStr)
		if err != nil {
			continue
		}

		idPkTs = append(idPkTs, &store.IdPkTs{
			Id:  id,
			Pub: pubkey,
			Ts:  createdAt,
			Ser: uint64(serialVal),
		})
	}

	return idPkTs, nil
}

// CountEvents counts events matching a filter
func (n *N) CountEvents(c context.Context, f *filter.F) (
	count int, approximate bool, err error,
) {
	// Build query but only count results
	cypher, params := n.buildCypherQuery(f, false)

	// Replace RETURN clause with COUNT
	returnClause := " RETURN count(e) AS count"
	cypherParts := strings.Split(cypher, "RETURN")
	if len(cypherParts) < 2 {
		return 0, false, fmt.Errorf("invalid query structure")
	}

	// Remove ORDER BY and LIMIT for count query
	cypher = cypherParts[0] + returnClause
	delete(params, "limit") // Remove limit parameter if it exists

	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return 0, false, fmt.Errorf("failed to count events: %w", err)
	}

	// Parse count from result
	ctx := context.Background()

	if result.Next(ctx) {
		record := result.Record()
		if record != nil {
			countRaw, found := record.Get("count")
			if found {
				countVal, ok := countRaw.(int64)
				if ok {
					count = int(countVal)
				}
			}
		}
	}

	return count, false, nil
}
