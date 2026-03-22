package main

import (
	"common/crypto/nip04"
	"common/crypto/nip44"
	"common/crypto/secp256k1"
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/sw"
	"common/nostr"
)

// DM Crypto domain — encrypt/decrypt pipeline.
// Pure computation: returns results, never touches network or cache.

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

// decryptIncomingDM decrypts a kind 4 or 1059 event.
// Calls cb with the record on success, does nothing on failure.
func decryptIncomingDM(ev *nostr.Event, cb func(*DMRecord)) {
	if myPubkey == "" {
		return
	}
	if ev.Kind == 4 {
		decryptNip04(ev, cb)
	} else if ev.Kind == 1059 {
		decryptNip17(ev, cb)
	}
}

func decryptNip04(ev *nostr.Event, cb func(*DMRecord)) {
	pTag := ev.Tags.GetFirst("p")
	if pTag == nil {
		return
	}
	recipient := pTag.Value()
	isMine := ev.PubKey == myPubkey
	isForMe := recipient == myPubkey
	if !isMine && !isForMe {
		return
	}
	peer := recipient
	if !isMine {
		peer = ev.PubKey
	}

	sk, ok := nip04.SharedKey(seckey, hexTo32(peer))
	if !ok {
		return
	}
	nip04.Decrypt(sk, ev.Content, func(plaintext string) {
		if plaintext == "" {
			return
		}
		cb(makeDMRecord(peer, ev.PubKey, plaintext, ev.CreatedAt, "nip04", ev.ID))
	})
}

func decryptNip17(ev *nostr.Event, cb func(*DMRecord)) {
	if !hasKey {
		return
	}
	ck, ok := nip44.ConversationKey(seckey, hexTo32(ev.PubKey))
	if !ok {
		return
	}
	sealJSON, ok := nip44.Decrypt(ev.Content, ck)
	if !ok {
		return
	}
	seal := nostr.ParseEvent(sealJSON)
	if seal == nil || seal.Kind != 13 {
		return
	}

	ck2, ok := nip44.ConversationKey(seckey, hexTo32(seal.PubKey))
	if !ok {
		return
	}
	innerJSON, ok := nip44.Decrypt(seal.Content, ck2)
	if !ok {
		return
	}
	inner := nostr.ParseEvent(innerJSON)
	if inner == nil || inner.Kind != 14 {
		return
	}

	senderPub := seal.PubKey
	pTag := inner.Tags.GetFirst("p")
	recipient := ""
	if pTag != nil {
		recipient = pTag.Value()
	}
	isMine := senderPub == myPubkey
	peer := senderPub
	if isMine {
		peer = recipient
	}
	if peer == "" {
		return
	}

	ts := inner.CreatedAt
	if ts == 0 {
		ts = ev.CreatedAt
	}
	cb(makeDMRecord(peer, senderPub, inner.Content, ts, "nip17", ev.ID))
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

// encryptNip04DM encrypts content for recipient using NIP-04.
// Calls cb with the signed event on success.
func encryptNip04DM(recipientPubkey, content string, cb func(*nostr.Event)) {
	sk04, ok := nip04.SharedKey(seckey, hexTo32(recipientPubkey))
	if !ok {
		return
	}
	nip04.Encrypt(sk04, content, func(ciphertext string) {
		ev := &nostr.Event{
			Kind:      4,
			Content:   ciphertext,
			Tags:      nostr.Tags{{"p", recipientPubkey}},
			CreatedAt: sw.NowSeconds(),
		}
		if identitySignEvent(ev) {
			cb(ev)
		}
	})
}

// encryptNip17DM encrypts content for recipient using NIP-17 gift wrap.
// Returns two wrapped events (recipient copy, sender copy).
func encryptNip17DM(recipientPubkey, content string) (*nostr.Event, *nostr.Event) {
	now := sw.NowSeconds()

	inner := &nostr.Event{
		Kind:      14,
		Content:   content,
		Tags:      nostr.Tags{{"p", recipientPubkey}},
		CreatedAt: now,
	}
	if !identitySignEvent(inner) {
		return nil, nil
	}
	innerJSON := inner.ToJSON()

	// Recipient copy.
	recipientCK, ok := nip44.ConversationKey(seckey, hexTo32(recipientPubkey))
	if !ok {
		return nil, nil
	}
	recipientSealContent := nip44.Encrypt(innerJSON, recipientCK, random32())
	recipientSeal := &nostr.Event{
		Kind:      13,
		Content:   recipientSealContent,
		Tags:      nostr.Tags{},
		CreatedAt: randomizeTimestamp(now),
	}
	if !identitySignEvent(recipientSeal) {
		return nil, nil
	}
	recipientWrap := giftWrap(recipientSeal, recipientPubkey, now)

	// Sender (self) copy.
	senderCK, ok := nip44.ConversationKey(seckey, hexTo32(myPubkey))
	if !ok {
		return recipientWrap, nil
	}
	senderSealContent := nip44.Encrypt(innerJSON, senderCK, random32())
	senderSeal := &nostr.Event{
		Kind:      13,
		Content:   senderSealContent,
		Tags:      nostr.Tags{},
		CreatedAt: randomizeTimestamp(now),
	}
	if !identitySignEvent(senderSeal) {
		return recipientWrap, nil
	}
	senderWrap := giftWrap(senderSeal, myPubkey, now)

	return recipientWrap, senderWrap
}

func giftWrap(seal *nostr.Event, recipientPubkey string, baseTime int64) *nostr.Event {
	ephKey := random32()
	_, ok := secp256k1.PubKeyFromSecKey(ephKey)
	if !ok {
		return nil
	}
	ck, ok := nip44.ConversationKey(ephKey, hexTo32(recipientPubkey))
	if !ok {
		return nil
	}
	wrapContent := nip44.Encrypt(seal.ToJSON(), ck, random32())
	wrap := &nostr.Event{
		Kind:      1059,
		Content:   wrapContent,
		Tags:      nostr.Tags{{"p", recipientPubkey}},
		CreatedAt: randomizeTimestamp(baseTime),
	}
	aux := random32()
	if !wrap.Sign(ephKey, aux) {
		return nil
	}
	return wrap
}

func randomizeTimestamp(baseTime int64) int64 {
	r := random32()
	offset := int64(r[0])<<24 | int64(r[1])<<16 | int64(r[2])<<8 | int64(r[3])
	if offset < 0 {
		offset = -offset
	}
	offset = offset % (2 * 24 * 60 * 60)
	return baseTime - offset
}
