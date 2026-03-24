package main

import "common/jsbridge/idb"

// Cache domain — pure event storage via IndexedDB.

func cacheQuery(filterRaw string, cb func(string)) {
	idb.QueryEvents(filterRaw, cb)
}

func cacheStore(evJSON string, cb func(bool)) {
	idb.SaveEvent(evJSON, cb)
}

func cacheSaveDM(dmJSON string, cb func(string)) {
	idb.SaveDM(dmJSON, cb)
}

func cacheGetConversationList(cb func(string)) {
	idb.GetConversationList(cb)
}

func cacheQueryDMs(peer string, limit int, until int64, cb func(string)) {
	idb.QueryDMs(peer, limit, until, cb)
}
