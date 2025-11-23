package directory

import (
	"encoding/json"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/errorf"
	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
)

// EventResult represents the result of processing a single event in a
// replication request.
type EventResult struct {
	EventID string            `json:"event_id"`
	Status  ReplicationStatus `json:"status"`
	Error   string            `json:"error,omitempty"`
}

// ReplicationResponseContent represents the JSON content of a Directory Event
// Replication Response event (Kind 39105).
type ReplicationResponseContent struct {
	RequestID string         `json:"request_id"`
	Results   []*EventResult `json:"results"`
}

// DirectoryEventReplicationResponse represents a complete Directory Event
// Replication Response event (Kind 39105) with typed access to its components.
type DirectoryEventReplicationResponse struct {
	Event       *event.E
	Content     *ReplicationResponseContent
	RequestID   string
	Status      ReplicationStatus
	ErrorMsg    string
	SourceRelay string
}

// NewDirectoryEventReplicationResponse creates a new Directory Event Replication
// Response event.
func NewDirectoryEventReplicationResponse(
	pubkey []byte,
	requestID string,
	status ReplicationStatus,
	errorMsg, sourceRelay string,
	results []*EventResult,
) (derr *DirectoryEventReplicationResponse, err error) {

	// Validate required fields
	if len(pubkey) != 32 {
		return nil, errorf.E("pubkey must be 32 bytes")
	}
	if requestID == "" {
		return nil, errorf.E("request ID is required")
	}
	if err = ValidateReplicationStatus(string(status)); chk.E(err) {
		return
	}
	if sourceRelay == "" {
		return nil, errorf.E("source relay is required")
	}

	// Create content
	content := &ReplicationResponseContent{
		RequestID: requestID,
		Results:   results,
	}

	// Marshal content to JSON
	var contentBytes []byte
	if contentBytes, err = json.Marshal(content); chk.E(err) {
		return
	}

	// Create base event
	ev := CreateBaseEvent(pubkey, DirectoryEventReplicationResponseKind)
	ev.Content = contentBytes

	// Add required tags
	ev.Tags.Append(tag.NewFromAny(string(RequestIDTag), requestID))
	ev.Tags.Append(tag.NewFromAny(string(StatusTag), string(status)))
	ev.Tags.Append(tag.NewFromAny(string(RelayTag), sourceRelay))

	// Add optional error tag
	if errorMsg != "" {
		ev.Tags.Append(tag.NewFromAny(string(ErrorTag), errorMsg))
	}

	derr = &DirectoryEventReplicationResponse{
		Event:       ev,
		Content:     content,
		RequestID:   requestID,
		Status:      status,
		ErrorMsg:    errorMsg,
		SourceRelay: sourceRelay,
	}

	return
}

// ParseDirectoryEventReplicationResponse parses an event into a
// DirectoryEventReplicationResponse structure with validation.
func ParseDirectoryEventReplicationResponse(ev *event.E) (derr *DirectoryEventReplicationResponse, err error) {
	if ev == nil {
		return nil, errorf.E("event cannot be nil")
	}

	// Validate event kind
	if ev.Kind != DirectoryEventReplicationResponseKind.K {
		return nil, errorf.E("invalid event kind: expected %d, got %d",
			DirectoryEventReplicationResponseKind.K, ev.Kind)
	}

	// Parse content
	var content ReplicationResponseContent
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

	statusTag := ev.Tags.GetFirst(StatusTag)
	if statusTag == nil {
		return nil, errorf.E("missing status tag")
	}

	relayTag := ev.Tags.GetFirst(RelayTag)
	if relayTag == nil {
		return nil, errorf.E("missing relay tag")
	}

	// Validate status
	status := ReplicationStatus(statusTag.Value())
	if err = ValidateReplicationStatus(string(status)); chk.E(err) {
		return
	}

	// Extract optional error tag
	var errorMsg string
	errorTag := ev.Tags.GetFirst(ErrorTag)
	if errorTag != nil {
		errorMsg = string(errorTag.Value())
	}

	derr = &DirectoryEventReplicationResponse{
		Event:       ev,
		Content:     &content,
		RequestID:   string(requestIDTag.Value()),
		Status:      status,
		ErrorMsg:    errorMsg,
		SourceRelay: string(relayTag.Value()),
	}

	return
}

// Validate performs comprehensive validation of a DirectoryEventReplicationResponse.
func (derr *DirectoryEventReplicationResponse) Validate() (err error) {
	if derr == nil {
		return errorf.E("DirectoryEventReplicationResponse cannot be nil")
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

	if err = ValidateReplicationStatus(string(derr.Status)); chk.E(err) {
		return
	}

	if derr.SourceRelay == "" {
		return errorf.E("source relay is required")
	}

	if derr.Content == nil {
		return errorf.E("content cannot be nil")
	}

	// Validate that content request ID matches tag request ID
	if derr.Content.RequestID != derr.RequestID {
		return errorf.E("content request ID does not match tag request ID")
	}

	// Validate event results
	for i, result := range derr.Content.Results {
		if result == nil {
			return errorf.E("result %d cannot be nil", i)
		}
		if result.EventID == "" {
			return errorf.E("result %d missing event ID", i)
		}
		if err = ValidateReplicationStatus(string(result.Status)); chk.E(err) {
			return errorf.E("result %d has invalid status: %w", i, err)
		}
	}

	return nil
}

// NewEventResult creates a new EventResult.
func NewEventResult(eventID string, status ReplicationStatus, errorMsg string) *EventResult {
	return &EventResult{
		EventID: eventID,
		Status:  status,
		Error:   errorMsg,
	}
}

// GetRequestID returns the request ID this response corresponds to.
func (derr *DirectoryEventReplicationResponse) GetRequestID() string {
	return derr.RequestID
}

// GetStatus returns the overall replication status.
func (derr *DirectoryEventReplicationResponse) GetStatus() ReplicationStatus {
	return derr.Status
}

// GetErrorMsg returns the error message, if any.
func (derr *DirectoryEventReplicationResponse) GetErrorMsg() string {
	return derr.ErrorMsg
}

// GetSourceRelay returns the relay that sent this response.
func (derr *DirectoryEventReplicationResponse) GetSourceRelay() string {
	return derr.SourceRelay
}

// GetResults returns the list of individual event results.
func (derr *DirectoryEventReplicationResponse) GetResults() []*EventResult {
	if derr.Content == nil {
		return nil
	}
	return derr.Content.Results
}

// GetResultCount returns the number of event results.
func (derr *DirectoryEventReplicationResponse) GetResultCount() int {
	if derr.Content == nil {
		return 0
	}
	return len(derr.Content.Results)
}

// HasResults returns true if the response contains event results.
func (derr *DirectoryEventReplicationResponse) HasResults() bool {
	return derr.GetResultCount() > 0
}

// IsSuccess returns true if the overall replication was successful.
func (derr *DirectoryEventReplicationResponse) IsSuccess() bool {
	return derr.Status == ReplicationStatusSuccess
}

// IsError returns true if the overall replication failed.
func (derr *DirectoryEventReplicationResponse) IsError() bool {
	return derr.Status == ReplicationStatusError
}

// IsPending returns true if the replication is still pending.
func (derr *DirectoryEventReplicationResponse) IsPending() bool {
	return derr.Status == ReplicationStatusPending
}

// GetSuccessfulResults returns all results with success status.
func (derr *DirectoryEventReplicationResponse) GetSuccessfulResults() []*EventResult {
	var results []*EventResult
	for _, result := range derr.GetResults() {
		if result.Status == ReplicationStatusSuccess {
			results = append(results, result)
		}
	}
	return results
}

// GetFailedResults returns all results with error status.
func (derr *DirectoryEventReplicationResponse) GetFailedResults() []*EventResult {
	var results []*EventResult
	for _, result := range derr.GetResults() {
		if result.Status == ReplicationStatusError {
			results = append(results, result)
		}
	}
	return results
}

// GetPendingResults returns all results with pending status.
func (derr *DirectoryEventReplicationResponse) GetPendingResults() []*EventResult {
	var results []*EventResult
	for _, result := range derr.GetResults() {
		if result.Status == ReplicationStatusPending {
			results = append(results, result)
		}
	}
	return results
}

// GetResultByEventID returns the result for a specific event ID, or nil if not found.
func (derr *DirectoryEventReplicationResponse) GetResultByEventID(eventID string) *EventResult {
	for _, result := range derr.GetResults() {
		if result.EventID == eventID {
			return result
		}
	}
	return nil
}

// GetSuccessCount returns the number of successfully replicated events.
func (derr *DirectoryEventReplicationResponse) GetSuccessCount() int {
	return len(derr.GetSuccessfulResults())
}

// GetFailureCount returns the number of failed event replications.
func (derr *DirectoryEventReplicationResponse) GetFailureCount() int {
	return len(derr.GetFailedResults())
}

// GetPendingCount returns the number of pending event replications.
func (derr *DirectoryEventReplicationResponse) GetPendingCount() int {
	return len(derr.GetPendingResults())
}

// GetSuccessRate returns the success rate as a percentage (0-100).
func (derr *DirectoryEventReplicationResponse) GetSuccessRate() float64 {
	total := derr.GetResultCount()
	if total == 0 {
		return 0
	}
	return float64(derr.GetSuccessCount()) / float64(total) * 100
}

// EventResult methods

// IsSuccess returns true if this event result was successful.
func (er *EventResult) IsSuccess() bool {
	return er.Status == ReplicationStatusSuccess
}

// IsError returns true if this event result failed.
func (er *EventResult) IsError() bool {
	return er.Status == ReplicationStatusError
}

// IsPending returns true if this event result is pending.
func (er *EventResult) IsPending() bool {
	return er.Status == ReplicationStatusPending
}

// HasError returns true if this event result has an error message.
func (er *EventResult) HasError() bool {
	return er.Error != ""
}
