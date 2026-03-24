package main

import (
	"common/crypto/sha256"
	"common/helpers"
)

// DMRecord represents a decrypted DM.
type DMRecord struct {
	ID        string
	Peer      string
	From      string
	Content   string
	CreatedAt int64
	Protocol  string
	EventID   string
}

func (r *DMRecord) ToJSON() string {
	return "{" +
		"\"id\":" + jstr(r.ID) +
		",\"peer\":" + jstr(r.Peer) +
		",\"from\":" + jstr(r.From) +
		",\"content\":" + jstr(r.Content) +
		",\"created_at\":" + helpers.Itoa(r.CreatedAt) +
		",\"protocol\":" + jstr(r.Protocol) +
		",\"eventId\":" + jstr(r.EventID) +
		"}"
}

func makeDMRecord(peer, from, content string, createdAt int64, protocol, eventID string) *DMRecord {
	return &DMRecord{
		ID:        dmDedupID(peer, content, createdAt),
		Peer:      peer,
		From:      from,
		Content:   content,
		CreatedAt: createdAt,
		Protocol:  protocol,
		EventID:   eventID,
	}
}

func dmDedupID(peer, content string, createdAt int64) string {
	contentHash := sha256.Sum([]byte(content))
	window := helpers.Itoa(createdAt / 300)
	combined := sha256.Sum([]byte(peer + helpers.HexEncode(contentHash[:]) + window))
	return helpers.HexEncode(combined[:])
}
