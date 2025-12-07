package neo4j

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/hex"
)

// SocialEventProcessor handles kind 0, 3, 1984, 10000 events for social graph management
type SocialEventProcessor struct {
	db *N
}

// NewSocialEventProcessor creates a new social event processor
func NewSocialEventProcessor(db *N) *SocialEventProcessor {
	return &SocialEventProcessor{db: db}
}

// ProcessedSocialEvent represents a processed social graph event in Neo4j
type ProcessedSocialEvent struct {
	EventID           string
	EventKind         int
	Pubkey            string
	CreatedAt         int64
	ProcessedAt       int64
	RelationshipCount int
	SupersededBy      *string // nil if still active
}

// ProcessSocialEvent routes events to appropriate handlers based on kind
func (p *SocialEventProcessor) ProcessSocialEvent(ctx context.Context, ev *event.E) error {
	switch ev.Kind {
	case 0:
		return p.processProfileMetadata(ctx, ev)
	case 3:
		return p.processContactList(ctx, ev)
	case 1984:
		return p.processReport(ctx, ev)
	case 10000:
		return p.processMuteList(ctx, ev)
	default:
		return fmt.Errorf("unsupported social event kind: %d", ev.Kind)
	}
}

// processProfileMetadata handles kind 0 events (profile metadata)
func (p *SocialEventProcessor) processProfileMetadata(ctx context.Context, ev *event.E) error {
	pubkey := hex.Enc(ev.Pubkey[:])
	eventID := hex.Enc(ev.ID[:])

	// Parse profile JSON from content
	var profile map[string]interface{}
	if err := json.Unmarshal(ev.Content, &profile); err != nil {
		p.db.Logger.Warningf("invalid profile JSON in event %s: %v", eventID, err)
		return nil // Don't fail, just skip profile update
	}

	// Update NostrUser node with profile data
	cypher := `
		MERGE (user:NostrUser {pubkey: $pubkey})
		ON CREATE SET
			user.created_at = timestamp(),
			user.first_seen_event = $event_id
		ON MATCH SET
			user.last_profile_update = $created_at
		SET
			user.name = $name,
			user.about = $about,
			user.picture = $picture,
			user.nip05 = $nip05,
			user.lud16 = $lud16,
			user.display_name = $display_name,
			user.npub = $npub
	`

	params := map[string]any{
		"pubkey":       pubkey,
		"event_id":     eventID,
		"created_at":   ev.CreatedAt,
		"name":         getStringFromMap(profile, "name"),
		"about":        getStringFromMap(profile, "about"),
		"picture":      getStringFromMap(profile, "picture"),
		"nip05":        getStringFromMap(profile, "nip05"),
		"lud16":        getStringFromMap(profile, "lud16"),
		"display_name": getStringFromMap(profile, "display_name"),
		"npub":         "", // TODO: compute npub from pubkey
	}

	_, err := p.db.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	p.db.Logger.Infof("updated profile for user %s", safePrefix(pubkey, 16))
	return nil
}

// processContactList handles kind 3 events (follow lists)
func (p *SocialEventProcessor) processContactList(ctx context.Context, ev *event.E) error {
	authorPubkey := hex.Enc(ev.Pubkey[:])
	eventID := hex.Enc(ev.ID[:])

	// 1. Check for existing contact list
	existingEvent, err := p.getLatestSocialEvent(ctx, authorPubkey, 3)
	if err != nil {
		return fmt.Errorf("failed to check existing contact list: %w", err)
	}

	// 2. Reject if this event is older than existing
	if existingEvent != nil && existingEvent.CreatedAt >= ev.CreatedAt {
		p.db.Logger.Infof("rejecting older contact list event %s (existing: %s)",
			safePrefix(eventID, 16), safePrefix(existingEvent.EventID, 16))
		return nil // Not an error, just skip
	}

	// 3. Extract p-tags to get new follows list
	newFollows := extractPTags(ev)

	// 4. Get old follows list if replacing an existing event
	var oldFollows []string
	var oldEventID string
	if existingEvent != nil {
		oldEventID = existingEvent.EventID
		oldFollows, err = p.getFollowsForEvent(ctx, oldEventID)
		if err != nil {
			return fmt.Errorf("failed to get old follows: %w", err)
		}
	}

	// 5. Compute diff
	added, removed := diffStringSlices(oldFollows, newFollows)

	// 6. Update graph in transaction
	err = p.updateContactListGraph(ctx, UpdateContactListParams{
		AuthorPubkey:   authorPubkey,
		NewEventID:     eventID,
		OldEventID:     oldEventID,
		CreatedAt:      ev.CreatedAt,
		AddedFollows:   added,
		RemovedFollows: removed,
		TotalFollows:   len(newFollows),
	})

	if err != nil {
		return fmt.Errorf("failed to update contact list graph: %w", err)
	}

	p.db.Logger.Infof("processed contact list: author=%s, event=%s, added=%d, removed=%d, total=%d",
		safePrefix(authorPubkey, 16), safePrefix(eventID, 16), len(added), len(removed), len(newFollows))

	return nil
}

// processMuteList handles kind 10000 events (mute lists)
func (p *SocialEventProcessor) processMuteList(ctx context.Context, ev *event.E) error {
	authorPubkey := hex.Enc(ev.Pubkey[:])
	eventID := hex.Enc(ev.ID[:])

	// Check for existing mute list
	existingEvent, err := p.getLatestSocialEvent(ctx, authorPubkey, 10000)
	if err != nil {
		return fmt.Errorf("failed to check existing mute list: %w", err)
	}

	// Reject if older
	if existingEvent != nil && existingEvent.CreatedAt >= ev.CreatedAt {
		p.db.Logger.Infof("rejecting older mute list event %s", safePrefix(eventID, 16))
		return nil
	}

	// Extract p-tags
	newMutes := extractPTags(ev)

	// Get old mutes
	var oldMutes []string
	var oldEventID string
	if existingEvent != nil {
		oldEventID = existingEvent.EventID
		oldMutes, err = p.getMutesForEvent(ctx, oldEventID)
		if err != nil {
			return fmt.Errorf("failed to get old mutes: %w", err)
		}
	}

	// Compute diff
	added, removed := diffStringSlices(oldMutes, newMutes)

	// Update graph
	err = p.updateMuteListGraph(ctx, UpdateMuteListParams{
		AuthorPubkey:  authorPubkey,
		NewEventID:    eventID,
		OldEventID:    oldEventID,
		CreatedAt:     ev.CreatedAt,
		AddedMutes:    added,
		RemovedMutes:  removed,
		TotalMutes:    len(newMutes),
	})

	if err != nil {
		return fmt.Errorf("failed to update mute list graph: %w", err)
	}

	p.db.Logger.Infof("processed mute list: author=%s, event=%s, added=%d, removed=%d",
		safePrefix(authorPubkey, 16), safePrefix(eventID, 16), len(added), len(removed))

	return nil
}

// processReport handles kind 1984 events (reports)
func (p *SocialEventProcessor) processReport(ctx context.Context, ev *event.E) error {
	reporterPubkey := hex.Enc(ev.Pubkey[:])
	eventID := hex.Enc(ev.ID[:])

	// Extract report target and type from tags
	// Format: ["p", "reported_pubkey", "report_type"]
	var reportedPubkey string
	var reportType string = "other" // default

	for _, t := range *ev.Tags {
		if len(t.T) >= 2 && string(t.T[0]) == "p" {
			// Use ExtractPTagValue to handle binary encoding and normalize to lowercase
			reportedPubkey = ExtractPTagValue(t)
			if len(t.T) >= 3 {
				reportType = string(t.T[2])
			}
			break // Use first p-tag
		}
	}

	if reportedPubkey == "" {
		p.db.Logger.Warningf("report event %s has no p-tag, skipping", safePrefix(eventID, 16))
		return nil
	}

	// Create REPORTS relationship
	// Note: WITH is required between CREATE and MERGE in Cypher
	cypher := `
		// Create event tracking node
		CREATE (evt:ProcessedSocialEvent {
			event_id: $event_id,
			event_kind: 1984,
			pubkey: $reporter_pubkey,
			created_at: $created_at,
			processed_at: timestamp(),
			relationship_count: 1,
			superseded_by: null
		})

		// WITH required to transition from CREATE to MERGE
		WITH evt

		// Create or get reporter and reported users
		MERGE (reporter:NostrUser {pubkey: $reporter_pubkey})
		MERGE (reported:NostrUser {pubkey: $reported_pubkey})

		// Create REPORTS relationship
		CREATE (reporter)-[:REPORTS {
			created_by_event: $event_id,
			created_at: $created_at,
			relay_received_at: timestamp(),
			report_type: $report_type
		}]->(reported)
	`

	params := map[string]any{
		"event_id":        eventID,
		"reporter_pubkey": reporterPubkey,
		"reported_pubkey": reportedPubkey,
		"created_at":      ev.CreatedAt,
		"report_type":     reportType,
	}

	_, err := p.db.ExecuteWrite(ctx, cypher, params)
	if err != nil {
		return fmt.Errorf("failed to create report: %w", err)
	}

	p.db.Logger.Infof("processed report: reporter=%s, reported=%s, type=%s",
		safePrefix(reporterPubkey, 16), safePrefix(reportedPubkey, 16), reportType)

	return nil
}

// UpdateContactListParams holds parameters for contact list graph update
type UpdateContactListParams struct {
	AuthorPubkey   string
	NewEventID     string
	OldEventID     string
	CreatedAt      int64
	AddedFollows   []string
	RemovedFollows []string
	TotalFollows   int
}

// updateContactListGraph performs atomic graph update for contact list changes
func (p *SocialEventProcessor) updateContactListGraph(ctx context.Context, params UpdateContactListParams) error {
	// We need to break this into separate operations because Neo4j's UNWIND
	// produces zero rows for empty arrays, which stops query execution.
	// Also, complex query chains with OPTIONAL MATCH can have issues.

	// Step 1: Create the ProcessedSocialEvent and NostrUser nodes
	createCypher := `
		// Get or create author node first
		MERGE (author:NostrUser {pubkey: $author_pubkey})
		ON CREATE SET author.created_at = timestamp()

		// Create new ProcessedSocialEvent tracking node
		CREATE (new:ProcessedSocialEvent {
			event_id: $new_event_id,
			event_kind: 3,
			pubkey: $author_pubkey,
			created_at: $created_at,
			processed_at: timestamp(),
			relationship_count: $total_follows,
			superseded_by: null
		})

		RETURN author.pubkey AS author_pubkey
	`

	createParams := map[string]any{
		"author_pubkey": params.AuthorPubkey,
		"new_event_id":  params.NewEventID,
		"created_at":    params.CreatedAt,
		"total_follows": params.TotalFollows,
	}

	_, err := p.db.ExecuteWrite(ctx, createCypher, createParams)
	if err != nil {
		return fmt.Errorf("failed to create ProcessedSocialEvent: %w", err)
	}

	// Step 2: Mark old event as superseded (if it exists)
	if params.OldEventID != "" {
		supersedeCypher := `
			MATCH (old:ProcessedSocialEvent {event_id: $old_event_id})
			SET old.superseded_by = $new_event_id
		`
		supersedeParams := map[string]any{
			"old_event_id": params.OldEventID,
			"new_event_id": params.NewEventID,
		}
		// Ignore errors - old event may not exist
		p.db.ExecuteWrite(ctx, supersedeCypher, supersedeParams)

		// Step 3: Update unchanged FOLLOWS to point to new event
		// Always update relationships that aren't being removed
		updateCypher := `
			MATCH (author:NostrUser {pubkey: $author_pubkey})-[f:FOLLOWS]->(followed:NostrUser)
			WHERE f.created_by_event = $old_event_id
			  AND NOT followed.pubkey IN $removed_follows
			SET f.created_by_event = $new_event_id,
			    f.created_at = $created_at
		`
		updateParams := map[string]any{
			"author_pubkey":   params.AuthorPubkey,
			"old_event_id":    params.OldEventID,
			"new_event_id":    params.NewEventID,
			"created_at":      params.CreatedAt,
			"removed_follows": params.RemovedFollows,
		}
		p.db.ExecuteWrite(ctx, updateCypher, updateParams)

		// Step 4: Remove FOLLOWS for removed follows
		if len(params.RemovedFollows) > 0 {
			removeCypher := `
				MATCH (author:NostrUser {pubkey: $author_pubkey})-[f:FOLLOWS]->(followed:NostrUser)
				WHERE f.created_by_event = $old_event_id
				  AND followed.pubkey IN $removed_follows
				DELETE f
			`
			removeParams := map[string]any{
				"author_pubkey":   params.AuthorPubkey,
				"old_event_id":    params.OldEventID,
				"removed_follows": params.RemovedFollows,
			}
			p.db.ExecuteWrite(ctx, removeCypher, removeParams)
		}
	}

	// Step 5: Create new FOLLOWS relationships for added follows
	// Process in batches to avoid memory issues
	const batchSize = 500
	for i := 0; i < len(params.AddedFollows); i += batchSize {
		end := i + batchSize
		if end > len(params.AddedFollows) {
			end = len(params.AddedFollows)
		}
		batch := params.AddedFollows[i:end]

		followsCypher := `
			MATCH (author:NostrUser {pubkey: $author_pubkey})
			UNWIND $added_follows AS followed_pubkey
			MERGE (followed:NostrUser {pubkey: followed_pubkey})
			ON CREATE SET followed.created_at = timestamp()
			MERGE (author)-[f:FOLLOWS]->(followed)
			ON CREATE SET
				f.created_by_event = $new_event_id,
				f.created_at = $created_at,
				f.relay_received_at = timestamp()
			ON MATCH SET
				f.created_by_event = $new_event_id,
				f.created_at = $created_at
		`

		followsParams := map[string]any{
			"author_pubkey": params.AuthorPubkey,
			"new_event_id":  params.NewEventID,
			"created_at":    params.CreatedAt,
			"added_follows": batch,
		}

		if _, err := p.db.ExecuteWrite(ctx, followsCypher, followsParams); err != nil {
			return fmt.Errorf("failed to create FOLLOWS batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// UpdateMuteListParams holds parameters for mute list graph update
type UpdateMuteListParams struct {
	AuthorPubkey  string
	NewEventID    string
	OldEventID    string
	CreatedAt     int64
	AddedMutes    []string
	RemovedMutes  []string
	TotalMutes    int
}

// updateMuteListGraph performs atomic graph update for mute list changes
func (p *SocialEventProcessor) updateMuteListGraph(ctx context.Context, params UpdateMuteListParams) error {
	// We need to break this into separate operations because Neo4j's UNWIND
	// produces zero rows for empty arrays, which stops query execution.

	// Step 1: Create the ProcessedSocialEvent and NostrUser nodes
	createCypher := `
		// Get or create author node first
		MERGE (author:NostrUser {pubkey: $author_pubkey})
		ON CREATE SET author.created_at = timestamp()

		// Create new ProcessedSocialEvent tracking node
		CREATE (new:ProcessedSocialEvent {
			event_id: $new_event_id,
			event_kind: 10000,
			pubkey: $author_pubkey,
			created_at: $created_at,
			processed_at: timestamp(),
			relationship_count: $total_mutes,
			superseded_by: null
		})

		RETURN author.pubkey AS author_pubkey
	`

	createParams := map[string]any{
		"author_pubkey": params.AuthorPubkey,
		"new_event_id":  params.NewEventID,
		"created_at":    params.CreatedAt,
		"total_mutes":   params.TotalMutes,
	}

	_, err := p.db.ExecuteWrite(ctx, createCypher, createParams)
	if err != nil {
		return fmt.Errorf("failed to create ProcessedSocialEvent: %w", err)
	}

	// Step 2: Mark old event as superseded (if it exists)
	if params.OldEventID != "" {
		supersedeCypher := `
			MATCH (old:ProcessedSocialEvent {event_id: $old_event_id})
			SET old.superseded_by = $new_event_id
		`
		supersedeParams := map[string]any{
			"old_event_id": params.OldEventID,
			"new_event_id": params.NewEventID,
		}
		p.db.ExecuteWrite(ctx, supersedeCypher, supersedeParams)

		// Step 3: Update unchanged MUTES to point to new event
		// Always update relationships that aren't being removed
		updateCypher := `
			MATCH (author:NostrUser {pubkey: $author_pubkey})-[m:MUTES]->(muted:NostrUser)
			WHERE m.created_by_event = $old_event_id
			  AND NOT muted.pubkey IN $removed_mutes
			SET m.created_by_event = $new_event_id,
			    m.created_at = $created_at
		`
		updateParams := map[string]any{
			"author_pubkey": params.AuthorPubkey,
			"old_event_id":  params.OldEventID,
			"new_event_id":  params.NewEventID,
			"created_at":    params.CreatedAt,
			"removed_mutes": params.RemovedMutes,
		}
		p.db.ExecuteWrite(ctx, updateCypher, updateParams)

		// Step 4: Remove MUTES for removed mutes
		if len(params.RemovedMutes) > 0 {
			removeCypher := `
				MATCH (author:NostrUser {pubkey: $author_pubkey})-[m:MUTES]->(muted:NostrUser)
				WHERE m.created_by_event = $old_event_id
				  AND muted.pubkey IN $removed_mutes
				DELETE m
			`
			removeParams := map[string]any{
				"author_pubkey": params.AuthorPubkey,
				"old_event_id":  params.OldEventID,
				"removed_mutes": params.RemovedMutes,
			}
			p.db.ExecuteWrite(ctx, removeCypher, removeParams)
		}
	}

	// Step 5: Create new MUTES relationships for added mutes
	// Process in batches to avoid memory issues
	const batchSize = 500
	for i := 0; i < len(params.AddedMutes); i += batchSize {
		end := i + batchSize
		if end > len(params.AddedMutes) {
			end = len(params.AddedMutes)
		}
		batch := params.AddedMutes[i:end]

		mutesCypher := `
			MATCH (author:NostrUser {pubkey: $author_pubkey})
			UNWIND $added_mutes AS muted_pubkey
			MERGE (muted:NostrUser {pubkey: muted_pubkey})
			ON CREATE SET muted.created_at = timestamp()
			MERGE (author)-[m:MUTES]->(muted)
			ON CREATE SET
				m.created_by_event = $new_event_id,
				m.created_at = $created_at,
				m.relay_received_at = timestamp()
			ON MATCH SET
				m.created_by_event = $new_event_id,
				m.created_at = $created_at
		`

		mutesParams := map[string]any{
			"author_pubkey": params.AuthorPubkey,
			"new_event_id":  params.NewEventID,
			"created_at":    params.CreatedAt,
			"added_mutes":   batch,
		}

		if _, err := p.db.ExecuteWrite(ctx, mutesCypher, mutesParams); err != nil {
			return fmt.Errorf("failed to create MUTES batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// getLatestSocialEvent retrieves the most recent non-superseded event of a given kind for a pubkey
func (p *SocialEventProcessor) getLatestSocialEvent(ctx context.Context, pubkey string, kind int) (*ProcessedSocialEvent, error) {
	cypher := `
		MATCH (evt:ProcessedSocialEvent {pubkey: $pubkey, event_kind: $kind})
		WHERE evt.superseded_by IS NULL
		RETURN evt.event_id AS event_id,
		       evt.created_at AS created_at,
		       evt.relationship_count AS relationship_count
		ORDER BY evt.created_at DESC
		LIMIT 1
	`

	params := map[string]any{
		"pubkey": pubkey,
		"kind":   kind,
	}

	result, err := p.db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	if result.Next(ctx) {
		record := result.Record()
		return &ProcessedSocialEvent{
			EventID:           record.Values[0].(string),
			CreatedAt:         record.Values[1].(int64),
			RelationshipCount: int(record.Values[2].(int64)),
		}, nil
	}

	return nil, nil // No existing event
}

// getFollowsForEvent retrieves the list of followed pubkeys for a specific event
func (p *SocialEventProcessor) getFollowsForEvent(ctx context.Context, eventID string) ([]string, error) {
	cypher := `
		MATCH (author:NostrUser)-[f:FOLLOWS]->(followed:NostrUser)
		WHERE f.created_by_event = $event_id
		RETURN collect(followed.pubkey) AS pubkeys
	`

	params := map[string]any{
		"event_id": eventID,
	}

	result, err := p.db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	if result.Next(ctx) {
		record := result.Record()
		pubkeysRaw := record.Values[0].([]interface{})
		pubkeys := make([]string, len(pubkeysRaw))
		for i, p := range pubkeysRaw {
			pubkeys[i] = p.(string)
		}
		return pubkeys, nil
	}

	return []string{}, nil
}

// getMutesForEvent retrieves the list of muted pubkeys for a specific event
func (p *SocialEventProcessor) getMutesForEvent(ctx context.Context, eventID string) ([]string, error) {
	cypher := `
		MATCH (author:NostrUser)-[m:MUTES]->(muted:NostrUser)
		WHERE m.created_by_event = $event_id
		RETURN collect(muted.pubkey) AS pubkeys
	`

	params := map[string]any{
		"event_id": eventID,
	}

	result, err := p.db.ExecuteRead(ctx, cypher, params)
	if err != nil {
		return nil, err
	}

	if result.Next(ctx) {
		record := result.Record()
		pubkeysRaw := record.Values[0].([]interface{})
		pubkeys := make([]string, len(pubkeysRaw))
		for i, p := range pubkeysRaw {
			pubkeys[i] = p.(string)
		}
		return pubkeys, nil
	}

	return []string{}, nil
}

// BatchProcessContactLists processes multiple contact list events in order
func (p *SocialEventProcessor) BatchProcessContactLists(ctx context.Context, events []*event.E) error {
	// Group by author
	byAuthor := make(map[string][]*event.E)
	for _, ev := range events {
		if ev.Kind != 3 {
			continue
		}
		pubkey := hex.Enc(ev.Pubkey[:])
		byAuthor[pubkey] = append(byAuthor[pubkey], ev)
	}

	// Process each author's events in chronological order
	for pubkey, authorEvents := range byAuthor {
		// Sort by created_at (oldest first)
		sort.Slice(authorEvents, func(i, j int) bool {
			return authorEvents[i].CreatedAt < authorEvents[j].CreatedAt
		})

		// Process in order
		for _, ev := range authorEvents {
			if err := p.processContactList(ctx, ev); err != nil {
				return fmt.Errorf("batch process failed for %s: %w", pubkey, err)
			}
		}
	}

	return nil
}

// Helper functions

// extractPTags extracts unique pubkeys from p-tags
// Uses ExtractPTagValue to properly handle binary-encoded tag values
// and normalizes to lowercase hex for consistent Neo4j storage
func extractPTags(ev *event.E) []string {
	seen := make(map[string]bool)
	var pubkeys []string

	for _, t := range *ev.Tags {
		if len(t.T) >= 2 && string(t.T[0]) == "p" {
			// Use ExtractPTagValue to handle binary encoding and normalize to lowercase
			pubkey := ExtractPTagValue(t)
			if IsValidHexPubkey(pubkey) && !seen[pubkey] {
				seen[pubkey] = true
				pubkeys = append(pubkeys, pubkey)
			}
		}
	}

	return pubkeys
}

// diffStringSlices computes added and removed elements between old and new slices
func diffStringSlices(old, new []string) (added, removed []string) {
	oldSet := make(map[string]bool)
	for _, s := range old {
		oldSet[s] = true
	}

	newSet := make(map[string]bool)
	for _, s := range new {
		newSet[s] = true
		if !oldSet[s] {
			added = append(added, s)
		}
	}

	for _, s := range old {
		if !newSet[s] {
			removed = append(removed, s)
		}
	}

	return
}

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
