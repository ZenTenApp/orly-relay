package main

import "common/jsbridge/idb"

// Cache domain — pure event storage via IndexedDB.
// Knows nothing about subscriptions, relays, or encryption.

// cacheQuery queries IDB for events matching a filter.
func cacheQuery(filterRaw string, cb func(string)) {
	idb.QueryEvents(filterRaw, cb)
}

// cacheStore saves an event to IDB.
func cacheStore(evJSON string, cb func(bool)) {
	idb.SaveEvent(evJSON, cb)
}

// cacheSaveDM saves a DM record to IDB.
func cacheSaveDM(dmJSON string, cb func(string)) {
	idb.SaveDM(dmJSON, cb)
}

// cacheGetConversationList retrieves the DM conversation list.
func cacheGetConversationList(cb func(string)) {
	idb.GetConversationList(cb)
}

// cacheQueryDMs queries DM history for a peer.
func cacheQueryDMs(peer string, limit int, until int64, cb func(string)) {
	idb.QueryDMs(peer, limit, until, cb)
}
