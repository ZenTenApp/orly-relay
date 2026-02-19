package neo4j

import (
	"context"
	stdhex "encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/filter"
	"git.mleku.dev/mleku/nostr/encoders/hex"
	"git.mleku.dev/mleku/nostr/encoders/kind"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"lol.mleku.dev/log"
	"next.orly.dev/pkg/database"
	"next.orly.dev/pkg/database/indexes/types"
	"next.orly.dev/pkg/interfaces/store"
)

// QueryEvents retrieves events matching the given filter
func (n *N) QueryEvents(c context.Context, f *filter.F) (evs event.S, err error) {
	log.T.F("Neo4j QueryEvents called with filter: kinds=%v, authors=%d, tags=%v",
		f.Kinds != nil, f.Authors != nil && len(f.Authors.T) > 0, f.Tags != nil)
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
	// NIP-50 search queries use a separate path with relevance scoring
	if len(f.Search) > 0 {
		return n.QuerySearchEvents(c, f)
	}

	// Build Cypher query from Nostr filter
	cypher, params := n.buildCypherQuery(f, includeDeleteEvents)

	// Execute query
	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Parse response
	allEvents, err := n.parseEventsFromResult(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse events: %w", err)
	}

	// Filter replaceable events to only return the latest version
	// unless showAllVersions is true
	if showAllVersions {
		return allEvents, nil
	}

	// Separate events by type and filter replaceables
	replaceableEvents := make(map[string]*event.E)           // key: pubkey:kind
	paramReplaceableEvents := make(map[string]map[string]*event.E) // key: pubkey:kind -> d-tag -> event
	var regularEvents event.S

	for _, ev := range allEvents {
		if kind.IsReplaceable(ev.Kind) {
			// For replaceable events, keep only the latest per pubkey:kind
			key := hex.Enc(ev.Pubkey) + ":" + strconv.Itoa(int(ev.Kind))
			existing, exists := replaceableEvents[key]
			if !exists || ev.CreatedAt > existing.CreatedAt {
				replaceableEvents[key] = ev
			}
		} else if kind.IsParameterizedReplaceable(ev.Kind) {
			// For parameterized replaceable events, keep only the latest per pubkey:kind:d-tag
			key := hex.Enc(ev.Pubkey) + ":" + strconv.Itoa(int(ev.Kind))

			// Get the 'd' tag value
			dTag := ev.Tags.GetFirst([]byte("d"))
			var dValue string
			if dTag != nil && dTag.Len() > 1 {
				dValue = string(dTag.Value())
			}

			// Initialize inner map if needed
			if _, exists := paramReplaceableEvents[key]; !exists {
				paramReplaceableEvents[key] = make(map[string]*event.E)
			}

			// Keep only the newest version
			existing, exists := paramReplaceableEvents[key][dValue]
			if !exists || ev.CreatedAt > existing.CreatedAt {
				paramReplaceableEvents[key][dValue] = ev
			}
		} else {
			regularEvents = append(regularEvents, ev)
		}
	}

	// Combine results
	evs = make(event.S, 0, len(replaceableEvents)+len(paramReplaceableEvents)+len(regularEvents))

	for _, ev := range replaceableEvents {
		evs = append(evs, ev)
	}

	for _, innerMap := range paramReplaceableEvents {
		for _, ev := range innerMap {
			evs = append(evs, ev)
		}
	}

	evs = append(evs, regularEvents...)

	// Re-sort by timestamp (newest first)
	sort.Slice(evs, func(i, j int) bool {
		return evs[i].CreatedAt > evs[j].CreatedAt
	})

	// Re-apply limit after filtering
	if f.Limit != nil && len(evs) > int(*f.Limit) {
		evs = evs[:*f.Limit]
	}

	return evs, nil
}

// buildCypherQuery constructs a Cypher query from a Nostr filter
// This is the core translation layer between Nostr's REQ filter format and Neo4j's Cypher
func (n *N) buildCypherQuery(f *filter.F, includeDeleteEvents bool) (string, map[string]any) {
	// NIP-50 search: delegate to word-based graph query
	if len(f.Search) > 0 {
		return n.buildSearchCypherQuery(f, includeDeleteEvents)
	}

	params := make(map[string]any)
	var whereClauses []string

	// Start with basic MATCH clause
	matchClause := "MATCH (e:Event)"

	// IDs filter - uses exact match or prefix matching
	// Note: IDs can be either binary (32 bytes) or hex strings (64 chars)
	// We need to normalize to lowercase hex for consistent Neo4j matching
	if f.Ids != nil && len(f.Ids.T) > 0 {
		idConditions := make([]string, 0, len(f.Ids.T))
		for i, id := range f.Ids.T {
			if len(id) == 0 {
				continue // Skip empty IDs
			}
			paramName := fmt.Sprintf("id_%d", i)

			// Normalize to lowercase hex using our utility function
			// This handles both binary-encoded IDs and hex string IDs (including uppercase)
			hexID := NormalizePubkeyHex(id)
			if hexID == "" {
				continue
			}

			// Handle prefix matching for partial IDs
			// After normalization, check hex length (should be 64 for full ID)
			if len(hexID) < 64 {
				idConditions = append(idConditions, fmt.Sprintf("e.id STARTS WITH $%s", paramName))
			} else {
				idConditions = append(idConditions, fmt.Sprintf("e.id = $%s", paramName))
			}
			params[paramName] = hexID
		}
		if len(idConditions) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(idConditions, " OR ")+")")
		}
	}

	// Authors filter - supports prefix matching for partial pubkeys
	// Note: Authors can be either binary (32 bytes) or hex strings (64 chars)
	// We need to normalize to lowercase hex for consistent Neo4j matching
	if f.Authors != nil && len(f.Authors.T) > 0 {
		authorConditions := make([]string, 0, len(f.Authors.T))
		for i, author := range f.Authors.T {
			if len(author) == 0 {
				continue // Skip empty authors
			}
			paramName := fmt.Sprintf("author_%d", i)

			// Normalize to lowercase hex using our utility function
			// This handles both binary-encoded pubkeys and hex string pubkeys (including uppercase)
			hexAuthor := NormalizePubkeyHex(author)
			log.T.F("Neo4j author filter: raw_len=%d, normalized=%q", len(author), hexAuthor)
			if hexAuthor == "" {
				continue
			}

			// Handle prefix matching for partial pubkeys
			// After normalization, check hex length (should be 64 for full pubkey)
			if len(hexAuthor) < 64 {
				authorConditions = append(authorConditions, fmt.Sprintf("e.pubkey STARTS WITH $%s", paramName))
			} else {
				authorConditions = append(authorConditions, fmt.Sprintf("e.pubkey = $%s", paramName))
			}
			params[paramName] = hexAuthor
		}
		if len(authorConditions) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(authorConditions, " OR ")+")")
		}
	}

	// Kinds filter - matches event types
	if f.Kinds != nil && len(f.Kinds.K) > 0 {
		kinds := make([]int64, len(f.Kinds.K))
		for i, k := range f.Kinds.K {
			kinds[i] = int64(k.K)
		}
		params["kinds"] = kinds
		whereClauses = append(whereClauses, "e.kind IN $kinds")
	}

	// Time range filters - for temporal queries
	// Note: Check both pointer and value - a zero timestamp (Unix epoch 1970) is almost
	// certainly not a valid constraint as Nostr events didn't exist then
	if f.Since != nil && f.Since.V > 0 {
		params["since"] = f.Since.V
		whereClauses = append(whereClauses, "e.created_at >= $since")
	}
	if f.Until != nil && f.Until.V > 0 {
		params["until"] = f.Until.V
		whereClauses = append(whereClauses, "e.created_at <= $until")
	}

	// Tag filters - this is where Neo4j's graph capabilities shine
	// We use EXISTS subqueries to efficiently filter events by tags
	// This ensures events are only returned if they have matching tags
	tagIndex := 0
	if f.Tags != nil {
		for _, tagValues := range *f.Tags {
			if len(tagValues.T) > 0 {
				tagTypeParam := fmt.Sprintf("tagType_%d", tagIndex)
				tagValuesParam := fmt.Sprintf("tagValues_%d", tagIndex)

				// The first element is the tag type (e.g., "e", "p", "#e", "#p", etc.)
				// Filter tags may have "#" prefix (e.g., "#d" for d-tag filters)
				// Event tags are stored without prefix, so we must strip it
				tagTypeBytes := tagValues.T[0]
				var tagType string
				if len(tagTypeBytes) > 0 && tagTypeBytes[0] == '#' {
					tagType = string(tagTypeBytes[1:]) // Strip "#" prefix
				} else {
					tagType = string(tagTypeBytes)
				}

				log.T.F("Neo4j tag filter: type=%q (raw=%q, len=%d)", tagType, string(tagTypeBytes), len(tagTypeBytes))

				// Convert remaining tag values to strings (skip first element which is the type)
				// For e/p tags, use NormalizePubkeyHex to handle binary encoding and uppercase hex
				tagValueStrings := make([]string, 0, len(tagValues.T)-1)
				for _, tv := range tagValues.T[1:] {
					if tagType == "e" || tagType == "p" {
						// Normalize e/p tag values to lowercase hex (handles binary encoding)
						normalized := NormalizePubkeyHex(tv)
						log.T.F("Neo4j tag filter: %s-tag value normalized: %q (raw len=%d, binary=%v)",
							tagType, normalized, len(tv), IsBinaryEncoded(tv))
						if normalized != "" {
							tagValueStrings = append(tagValueStrings, normalized)
						}
					} else {
						// For other tags, use direct string conversion
						val := string(tv)
						log.T.F("Neo4j tag filter: %s-tag value: %q (len=%d)", tagType, val, len(val))
						tagValueStrings = append(tagValueStrings, val)
					}
				}

				// Skip if no valid values after normalization
				if len(tagValueStrings) == 0 {
					log.W.F("Neo4j tag filter: no valid values for tag type %q, skipping", tagType)
					continue
				}

				log.T.F("Neo4j tag filter: type=%s, values=%v", tagType, tagValueStrings)

				// Use EXISTS subquery to filter events that have matching tags
				// This is more correct than OPTIONAL MATCH because it requires the tag to exist
				params[tagTypeParam] = tagType
				params[tagValuesParam] = tagValueStrings
				whereClauses = append(whereClauses,
					fmt.Sprintf("EXISTS { MATCH (e)-[:TAGGED_WITH]->(t:Tag) WHERE t.type = $%s AND t.value IN $%s }",
						tagTypeParam, tagValuesParam))

				tagIndex++
			}
		}
	}

	// Exclude delete events unless requested
	if !includeDeleteEvents {
		whereClauses = append(whereClauses, "e.kind <> 5")
	}

	// Filter out expired events (NIP-40) unless querying by explicit IDs
	// Events with expiration > 0 that have passed are hidden from results
	// EXCEPT when the query includes specific event IDs (allowing explicit lookup)
	hasExplicitIds := f.Ids != nil && len(f.Ids.T) > 0
	if !hasExplicitIds {
		params["now"] = time.Now().Unix()
		// Show events where either: no expiration (expiration = 0) OR expiration hasn't passed yet
		whereClauses = append(whereClauses, "(e.expiration = 0 OR e.expiration > $now)")
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

	// Add limit - use the smaller of requested limit and configured max limit
	// This prevents unbounded queries that could exhaust memory
	limitClause := ""
	requestedLimit := 0
	if f.Limit != nil && *f.Limit > 0 {
		requestedLimit = int(*f.Limit)
	}

	// Apply the configured query result limit as a safety cap
	// If queryResultLimit is 0 (unlimited), only use the requested limit
	effectiveLimit := requestedLimit
	if n.queryResultLimit > 0 {
		if effectiveLimit == 0 || effectiveLimit > n.queryResultLimit {
			effectiveLimit = n.queryResultLimit
		}
	}

	if effectiveLimit > 0 {
		params["limit"] = effectiveLimit
		limitClause = " LIMIT $limit"
	}

	// Combine all parts
	cypher := matchClause + whereClause + returnClause + orderClause + limitClause

	// Log the generated query for debugging
	log.T.F("Neo4j query: %s", cypher)
	// Log params at trace level for debugging
	var paramSummary strings.Builder
	for k, v := range params {
		switch val := v.(type) {
		case []string:
			if len(val) <= 3 {
				paramSummary.WriteString(fmt.Sprintf("%s: %v ", k, val))
			} else {
				paramSummary.WriteString(fmt.Sprintf("%s: [%d values] ", k, len(val)))
			}
		case []int64:
			paramSummary.WriteString(fmt.Sprintf("%s: %v ", k, val))
		default:
			paramSummary.WriteString(fmt.Sprintf("%s: %v ", k, v))
		}
	}
	log.T.F("Neo4j params: %s", paramSummary.String())

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

		// Create event with decoded binary fields
		e := &event.E{
			ID:        id,
			Pubkey:    pubkey,
			Kind:      uint16(kind),
			CreatedAt: createdAt,
			Content:   []byte(content),
			Tags:      tags,
			Sig:       sig,
		}

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

	// Rebuild query with serial-only return, preserving ORDER BY and LIMIT
	cypher = cypherParts[0] + returnClause
	remainder := cypherParts[1]
	if strings.Contains(remainder, "ORDER BY") {
		orderAndLimit := " ORDER BY" + strings.Split(remainder, "ORDER BY")[1]
		cypher += orderAndLimit
	} else if strings.Contains(remainder, "LIMIT") {
		// No ORDER BY but has LIMIT
		limitPart := " LIMIT" + strings.Split(remainder, "LIMIT")[1]
		cypher += limitPart
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

	// Rebuild query preserving ORDER BY and LIMIT
	cypher = cypherParts[0] + returnClause
	remainder := cypherParts[1]
	if strings.Contains(remainder, "ORDER BY") {
		orderAndLimit := " ORDER BY" + strings.Split(remainder, "ORDER BY")[1]
		cypher += orderAndLimit
	} else if strings.Contains(remainder, "LIMIT") {
		// No ORDER BY but has LIMIT
		limitPart := " LIMIT" + strings.Split(remainder, "LIMIT")[1]
		cypher += limitPart
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

		ipkts := store.NewIdPkTs(id, pubkey, createdAt, uint64(serialVal))
		idPkTs = append(idPkTs, &ipkts)
	}

	return idPkTs, nil
}

// buildSearchCypherQuery constructs a Cypher query for NIP-50 word search.
// It matches events via HAS_WORD relationships to Word nodes, counts matches
// per event, and returns matchCount for Go-side relevance scoring.
func (n *N) buildSearchCypherQuery(f *filter.F, includeDeleteEvents bool) (string, map[string]any) {
	params := make(map[string]any)

	// Tokenize the search string using the same tokenizer as indexing
	searchTokens := database.TokenWords(f.Search)
	if len(searchTokens) == 0 {
		// No valid search tokens — return query that matches nothing
		return "MATCH (e:Event) WHERE false RETURN e.id AS id, e.kind AS kind, e.created_at AS created_at, e.content AS content, e.sig AS sig, e.pubkey AS pubkey, e.tags AS tags, e.serial AS serial, 0 AS matchCount", params
	}

	wordHashes := make([]string, len(searchTokens))
	for i, wt := range searchTokens {
		wordHashes[i] = stdhex.EncodeToString(wt.Hash)
	}
	params["wordHashes"] = wordHashes

	// Build WHERE clauses for additional filters (authors, kinds, time, tags)
	var whereClauses []string

	// Authors filter
	if f.Authors != nil && len(f.Authors.T) > 0 {
		authorConditions := make([]string, 0, len(f.Authors.T))
		for i, author := range f.Authors.T {
			if len(author) == 0 {
				continue
			}
			paramName := fmt.Sprintf("author_%d", i)
			hexAuthor := NormalizePubkeyHex(author)
			if hexAuthor == "" {
				continue
			}
			if len(hexAuthor) < 64 {
				authorConditions = append(authorConditions, fmt.Sprintf("e.pubkey STARTS WITH $%s", paramName))
			} else {
				authorConditions = append(authorConditions, fmt.Sprintf("e.pubkey = $%s", paramName))
			}
			params[paramName] = hexAuthor
		}
		if len(authorConditions) > 0 {
			whereClauses = append(whereClauses, "("+strings.Join(authorConditions, " OR ")+")")
		}
	}

	// Kinds filter
	if f.Kinds != nil && len(f.Kinds.K) > 0 {
		kinds := make([]int64, len(f.Kinds.K))
		for i, k := range f.Kinds.K {
			kinds[i] = int64(k.K)
		}
		params["kinds"] = kinds
		whereClauses = append(whereClauses, "e.kind IN $kinds")
	}

	// Time range filters
	if f.Since != nil && f.Since.V > 0 {
		params["since"] = f.Since.V
		whereClauses = append(whereClauses, "e.created_at >= $since")
	}
	if f.Until != nil && f.Until.V > 0 {
		params["until"] = f.Until.V
		whereClauses = append(whereClauses, "e.created_at <= $until")
	}

	// Tag filters
	tagIndex := 0
	if f.Tags != nil {
		for _, tagValues := range *f.Tags {
			if len(tagValues.T) > 0 {
				tagTypeParam := fmt.Sprintf("tagType_%d", tagIndex)
				tagValuesParam := fmt.Sprintf("tagValues_%d", tagIndex)

				tagTypeBytes := tagValues.T[0]
				var tagType string
				if len(tagTypeBytes) > 0 && tagTypeBytes[0] == '#' {
					tagType = string(tagTypeBytes[1:])
				} else {
					tagType = string(tagTypeBytes)
				}

				tagValueStrings := make([]string, 0, len(tagValues.T)-1)
				for _, tv := range tagValues.T[1:] {
					if tagType == "e" || tagType == "p" {
						normalized := NormalizePubkeyHex(tv)
						if normalized != "" {
							tagValueStrings = append(tagValueStrings, normalized)
						}
					} else {
						tagValueStrings = append(tagValueStrings, string(tv))
					}
				}

				if len(tagValueStrings) == 0 {
					continue
				}

				params[tagTypeParam] = tagType
				params[tagValuesParam] = tagValueStrings
				whereClauses = append(whereClauses,
					fmt.Sprintf("EXISTS { MATCH (e)-[:TAGGED_WITH]->(t:Tag) WHERE t.type = $%s AND t.value IN $%s }",
						tagTypeParam, tagValuesParam))
				tagIndex++
			}
		}
	}

	// Exclude delete events
	if !includeDeleteEvents {
		whereClauses = append(whereClauses, "e.kind <> 5")
	}

	// Expiration filter (NIP-40)
	hasExplicitIds := f.Ids != nil && len(f.Ids.T) > 0
	if !hasExplicitIds {
		params["now"] = time.Now().Unix()
		whereClauses = append(whereClauses, "(e.expiration = 0 OR e.expiration > $now)")
	}

	// Build additional WHERE string
	additionalWhere := ""
	if len(whereClauses) > 0 {
		additionalWhere = " AND " + strings.Join(whereClauses, " AND ")
	}

	// Limit: only apply the safety cap (queryResultLimit) in Cypher.
	// The user-requested f.Limit is applied in Go after relevance scoring,
	// so that recency-boosted events aren't prematurely discarded.
	limitClause := ""
	if n.queryResultLimit > 0 {
		params["limit"] = n.queryResultLimit
		limitClause = "\nLIMIT $limit"
	}

	// Core search query: traverse word graph, count matches per event
	cypher := `
MATCH (e:Event)-[:HAS_WORD]->(w:Word)
WHERE w.hash IN $wordHashes` + additionalWhere + `
WITH e, count(DISTINCT w) AS matchCount
RETURN e.id AS id,
       e.kind AS kind,
       e.created_at AS created_at,
       e.content AS content,
       e.sig AS sig,
       e.pubkey AS pubkey,
       e.tags AS tags,
       e.serial AS serial,
       matchCount
ORDER BY matchCount DESC, e.created_at DESC` + limitClause

	log.T.F("Neo4j search query: %s", cypher)

	return cypher, params
}

// searchScoredEvent pairs a parsed event with its search match count for scoring.
type searchScoredEvent struct {
	Event      *event.E
	MatchCount int
}

// QuerySearchEvents performs a NIP-50 word search query with relevance scoring.
// Events are scored using a 50/50 blend of match count and recency, matching
// the Badger backend's scoring algorithm.
func (n *N) QuerySearchEvents(c context.Context, f *filter.F) (evs event.S, err error) {
	cypher, params := n.buildSearchCypherQuery(f, false)

	result, err := n.ExecuteRead(c, cypher, params)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	// Parse results with match counts
	var scored []searchScoredEvent
	ctx := context.Background()

	for result.Next(ctx) {
		record := result.Record()
		if record == nil {
			continue
		}

		// Parse event fields (same as parseEventsFromResult)
		idRaw, _ := record.Get("id")
		kindRaw, _ := record.Get("kind")
		createdAtRaw, _ := record.Get("created_at")
		contentRaw, _ := record.Get("content")
		sigRaw, _ := record.Get("sig")
		pubkeyRaw, _ := record.Get("pubkey")
		tagsRaw, _ := record.Get("tags")
		matchCountRaw, _ := record.Get("matchCount")

		idStr, _ := idRaw.(string)
		kindVal, _ := kindRaw.(int64)
		createdAt, _ := createdAtRaw.(int64)
		content, _ := contentRaw.(string)
		sigStr, _ := sigRaw.(string)
		pubkeyStr, _ := pubkeyRaw.(string)
		tagsStr, _ := tagsRaw.(string)
		matchCount, _ := matchCountRaw.(int64)

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
			ID:        id,
			Pubkey:    pubkey,
			Kind:      uint16(kindVal),
			CreatedAt: createdAt,
			Content:   []byte(content),
			Tags:      tags,
			Sig:       sig,
		}

		scored = append(scored, searchScoredEvent{Event: e, MatchCount: int(matchCount)})
	}

	if err := result.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	if len(scored) == 0 {
		return nil, nil
	}

	// Apply 50/50 relevance scoring (match count + recency)
	maxCount := 0
	var minTs, maxTs int64
	for i, s := range scored {
		if s.MatchCount > maxCount {
			maxCount = s.MatchCount
		}
		ts := s.Event.CreatedAt
		if i == 0 {
			minTs, maxTs = ts, ts
		} else {
			if ts < minTs {
				minTs = ts
			}
			if ts > maxTs {
				maxTs = ts
			}
		}
	}

	tsSpan := float64(maxTs - minTs)

	sort.Slice(scored, func(i, j int) bool {
		ci := float64(scored[i].MatchCount) / math.Max(float64(maxCount), 1)
		cj := float64(scored[j].MatchCount) / math.Max(float64(maxCount), 1)

		var ai, aj float64
		if tsSpan > 0 {
			ai = float64(scored[i].Event.CreatedAt-minTs) / tsSpan
			aj = float64(scored[j].Event.CreatedAt-minTs) / tsSpan
		}

		si := 0.5*ci + 0.5*ai
		sj := 0.5*cj + 0.5*aj

		if si == sj {
			return scored[i].Event.CreatedAt > scored[j].Event.CreatedAt
		}
		return si > sj
	})

	// Extract sorted events
	evs = make(event.S, len(scored))
	for i, s := range scored {
		evs[i] = s.Event
	}

	// Apply limit
	if f.Limit != nil && len(evs) > int(*f.Limit) {
		evs = evs[:*f.Limit]
	}

	return evs, nil
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
