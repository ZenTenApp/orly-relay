package idb

// IndexedDB jsbridge — purpose-built event and DM storage.
// The JS runtime handles all IndexedDB internals (schema, cursors, indices).

// Open opens/creates the database. Calls fn when ready.
func Open(fn func()) { panic("jsbridge") }

// SaveEvent saves a Nostr event (JSON string). Calls fn with true if new.
func SaveEvent(eventJSON string, fn func(bool)) { panic("jsbridge") }

// QueryEvents queries events matching a filter (JSON string).
// Calls fn with a JSON array of matching events.
func QueryEvents(filterJSON string, fn func(string)) { panic("jsbridge") }

// SaveDM saves a DM record (JSON string).
// Calls fn with result: "saved", "duplicate", or "upgraded".
func SaveDM(dmJSON string, fn func(string)) { panic("jsbridge") }

// QueryDMs queries DMs for a peer pubkey.
// until is Unix timestamp or 0 for no upper bound.
// Calls fn with a JSON array of DM records.
func QueryDMs(peer string, limit int, until int64, fn func(string)) { panic("jsbridge") }

// GetConversationList returns all DM conversations.
// Calls fn with a JSON array of conversation summaries.
func GetConversationList(fn func(string)) { panic("jsbridge") }

// ClearDMsByPeer deletes all DM records for a given peer pubkey.
// Calls fn when complete.
func ClearDMsByPeer(peer string, fn func()) { panic("jsbridge") }

// SetVersion sets the expected app version for epoch checks.
// On Open, if stored version != this version, all data stores are flushed.
func SetVersion(v string) { panic("jsbridge") }
