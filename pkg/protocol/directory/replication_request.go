package directory

import (
	"encoding/json"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/tag"
)

// ReplicationRequestContent represents the JSON content of a Directory Event
// Replication Request event (Kind 39104).
type ReplicationRequestContent struct {
	Events []*event.E `json:"events"`
}

// DirectoryEventReplicationRequest represents a complete Directory Event
// Replication Request event (Kind 39104) with typed access to its components.
type DirectoryEventReplicationRequest struct {
	Event       *event.E
	Content     *ReplicationRequestContent
	RequestID   string
	TargetRelay string
}

// NewDirectoryEventReplicationRequest creates a new Directory Event Replication
// Request event.
func NewDirectoryEventReplicationRequest(
	pubkey []byte,
	requestID, targetRelay string,
	events []*event.E,
) (derr *DirectoryEventReplicationRequest, err error) {

	// Validate required fields
	if len(pubkey) != 32 {
		return nil, errorf.E("pubkey must be 32 bytes")
	}
	if requestID == "" {
		return nil, errorf.E("request ID is required")
	}
	if targetRelay == "" {
		return nil, errorf.E("target relay is required")
	}
	if len(events) == 0 {
		return nil, errorf.E("at least one event is required")
	}

	// Validate all events
	for i, ev := range events {
		if ev == nil {
			return nil, errorf.E("event %d cannot be nil", i)
		}
		// Verify event signature
		if _, err = ev.Verify(); chk.E(err) {
			return nil, errorf.E("invalid signature for event %d: %w", i, err)
		}
	}

	// Create content
	content := &ReplicationRequestContent{
		Events: events,
	}

	// Marshal content to JSON
	var contentBytes []byte
	if contentBytes, err = json.Marshal(content); chk.E(err) {
		return
	}

	// Create base event
	ev := CreateBaseEvent(pubkey, DirectoryEventReplicationRequestKind)
	ev.Content = contentBytes

	// Add required tags
	ev.Tags.Append(tag.NewFromAny(string(RequestIDTag), requestID))
	ev.Tags.Append(tag.NewFromAny(string(RelayTag), targetRelay))

	derr = &DirectoryEventReplicationRequest{
		Event:       ev,
		Content:     content,
		RequestID:   requestID,
		TargetRelay: targetRelay,
	}

	return
}

// ParseDirectoryEventReplicationRequest parses an event into a
// DirectoryEventReplicationRequest structure with validation.
func ParseDirectoryEventReplicationRequest(ev *event.E) (derr *DirectoryEventReplicationRequest, err error) {
	if ev == nil {
		return nil, errorf.E("event cannot be nil")
	}

	// Validate event kind
	if ev.Kind != DirectoryEventReplicationRequestKind.K {
		return nil, errorf.E("invalid event kind: expected %d, got %d",
			DirectoryEventReplicationRequestKind.K, ev.Kind)
	}

	// Parse content
	var content ReplicationRequestContent
	if len(ev.Content) > 0 {
		if err = json.Unmarshal(ev.Content, &content); chk.E(err) {
			return nil, errorf.E("failed to parse content: %w", err)
		}
	}

	// Extract required tags
	requestIDTag := ev.Tags.GetFirst(RequestIDTag)
	if requestIDTag == nil {
		return nil, errorf.E("missing request_id tag")
	}

	relayTag := ev.Tags.GetFirst(RelayTag)
	if relayTag == nil {
		return nil, errorf.E("missing relay tag")
	}

	derr = &DirectoryEventReplicationRequest{
		Event:       ev,
		Content:     &content,
		RequestID:   string(requestIDTag.Value()),
		TargetRelay: string(relayTag.Value()),
	}

	return
}

// Validate performs comprehensive validation of a DirectoryEventReplicationRequest.
func (derr *DirectoryEventReplicationRequest) Validate() (err error) {
	if derr == nil {
		return errorf.E("DirectoryEventReplicationRequest cannot be nil")
	}

	if derr.Event == nil {
		return errorf.E("event cannot be nil")
	}

	// Validate event signature
	if _, err = derr.Event.Verify(); chk.E(err) {
		return errorf.E("invalid event signature: %w", err)
	}

	// Validate required fields
	if derr.RequestID == "" {
		return errorf.E("request ID is required")
	}

	if derr.TargetRelay == "" {
		return errorf.E("target relay is required")
	}

	if derr.Content == nil {
		return errorf.E("content cannot be nil")
	}

	if len(derr.Content.Events) == 0 {
		return errorf.E("at least one event is required")
	}

	// Validate all events in the request
	for i, ev := range derr.Content.Events {
		if ev == nil {
			return errorf.E("event %d cannot be nil", i)
		}
		// Verify event signature
		if _, err = ev.Verify(); chk.E(err) {
			return errorf.E("invalid signature for event %d: %w", i, err)
		}
	}

	return nil
}

// GetRequestID returns the unique request identifier.
func (derr *DirectoryEventReplicationRequest) GetRequestID() string {
	return derr.RequestID
}

// GetTargetRelay returns the target relay URL.
func (derr *DirectoryEventReplicationRequest) GetTargetRelay() string {
	return derr.TargetRelay
}

// GetEvents returns the list of events to replicate.
func (derr *DirectoryEventReplicationRequest) GetEvents() []*event.E {
	if derr.Content == nil {
		return nil
	}
	return derr.Content.Events
}

// GetEventCount returns the number of events in the request.
func (derr *DirectoryEventReplicationRequest) GetEventCount() int {
	if derr.Content == nil {
		return 0
	}
	return len(derr.Content.Events)
}

// HasEvents returns true if the request contains events.
func (derr *DirectoryEventReplicationRequest) HasEvents() bool {
	return derr.GetEventCount() > 0
}

// GetEventByIndex returns the event at the specified index, or nil if out of bounds.
func (derr *DirectoryEventReplicationRequest) GetEventByIndex(index int) *event.E {
	events := derr.GetEvents()
	if index < 0 || index >= len(events) {
		return nil
	}
	return events[index]
}

// ContainsEventKind returns true if the request contains events of the specified kind.
func (derr *DirectoryEventReplicationRequest) ContainsEventKind(kind uint16) bool {
	for _, ev := range derr.GetEvents() {
		if ev.Kind == kind {
			return true
		}
	}
	return false
}

// GetEventsByKind returns all events of the specified kind.
func (derr *DirectoryEventReplicationRequest) GetEventsByKind(kind uint16) []*event.E {
	var result []*event.E
	for _, ev := range derr.GetEvents() {
		if ev.Kind == kind {
			result = append(result, ev)
		}
	}
	return result
}

// GetDirectoryEvents returns only the directory events from the request.
func (derr *DirectoryEventReplicationRequest) GetDirectoryEvents() []*event.E {
	var result []*event.E
	for _, ev := range derr.GetEvents() {
		if IsDirectoryEventKind(ev.Kind) {
			result = append(result, ev)
		}
	}
	return result
}

// GetNonDirectoryEvents returns only the non-directory events from the request.
func (derr *DirectoryEventReplicationRequest) GetNonDirectoryEvents() []*event.E {
	var result []*event.E
	for _, ev := range derr.GetEvents() {
		if !IsDirectoryEventKind(ev.Kind) {
			result = append(result, ev)
		}
	}
	return result
}

// GetEventsByAuthor returns all events from the specified author.
func (derr *DirectoryEventReplicationRequest) GetEventsByAuthor(pubkey []byte) []*event.E {
	var result []*event.E
	for _, ev := range derr.GetEvents() {
		if len(ev.Pubkey) == len(pubkey) {
			match := true
			for i := range pubkey {
				if ev.Pubkey[i] != pubkey[i] {
					match = false
					break
				}
			}
			if match {
				result = append(result, ev)
			}
		}
	}
	return result
}
