package main

import (
	"common/crypto/nip04"
	"common/crypto/nip44"
	"common/crypto/secp256k1"
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/registry"
	"common/jsbridge/subtle"
	"common/jsbridge/sw"
	"common/nostr"
)

func hexTo32(s string) [32]byte {
	out, _ := helpers.HexDecode32(s)
	return out
}

func random32() [32]byte {
	var b [32]byte
	subtle.RandomBytes(b[:])
	return b
}

func signEvent(ev *nostr.Event) bool {
	sk := registry.Seckey()
	if sk == "" {
		return false
	}
	aux := random32()
	return ev.Sign(hexTo32(sk), aux)
}

func init() {
	registry.OnEncryptNip04(func(pubkey, content string, cb func(string)) {
		encryptNip04DM(pubkey, content, func(ev *nostr.Event) {
			cb(ev.ToJSON())
		})
	})
	registry.OnEncryptNip17(func(pubkey, content string, cb func(string, string)) {
		rw, sw := encryptNip17DM(pubkey, content)
		rJSON, sJSON := "", ""
		if rw != nil {
			rJSON = rw.ToJSON()
		}
		if sw != nil {
			sJSON = sw.ToJSON()
		}
		cb(rJSON, sJSON)
	})
	registry.OnDecryptDM(func(evJSON string, cb func(string)) {
		ev := nostr.ParseEvent(evJSON)
		if ev == nil {
			return
		}
		decryptIncomingDM(ev, func(rec *dmRecord) {
			cb(rec.toJSON())
		})
	})
}

func main() {}

// dmRecord represents a decrypted DM.
type dmRecord struct {
	id, peer, from, content string
	createdAt               int64
	protocol, eventID       string
}

func (r *dmRecord) toJSON() string {
	return "{" +
		"\"id\":" + helpers.JsonString(r.id) +
		",\"peer\":" + helpers.JsonString(r.peer) +
		",\"from\":" + helpers.JsonString(r.from) +
		",\"content\":" + helpers.JsonString(r.content) +
		",\"created_at\":" + helpers.Itoa(r.createdAt) +
		",\"protocol\":" + helpers.JsonString(r.protocol) +
		",\"eventId\":" + helpers.JsonString(r.eventID) +
		"}"
}

func decryptIncomingDM(ev *nostr.Event, cb func(*dmRecord)) {
	pub := registry.Pubkey()
	if pub == "" {
		return
	}
	if ev.Kind == 4 {
		decryptNip04(ev, pub, cb)
	} else if ev.Kind == 1059 {
		decryptNip17(ev, pub, cb)
	}
}

func decryptNip04(ev *nostr.Event, myPub string, cb func(*dmRecord)) {
	pTag := ev.Tags.GetFirst("p")
	if pTag == nil {
		return
	}
	recipient := pTag.Value()
	isMine := ev.PubKey == myPub
	isForMe := recipient == myPub
	if !isMine && !isForMe {
		return
	}
	peer := recipient
	if !isMine {
		peer = ev.PubKey
	}
	sk, ok := nip04.SharedKey(hexTo32(registry.Seckey()), hexTo32(peer))
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

func decryptNip17(ev *nostr.Event, myPub string, cb func(*dmRecord)) {
	if !registry.HasKey() {
		return
	}
	sk := hexTo32(registry.Seckey())
	ck, ok := nip44.ConversationKey(sk, hexTo32(ev.PubKey))
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
	ck2, ok := nip44.ConversationKey(sk, hexTo32(seal.PubKey))
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
	isMine := senderPub == myPub
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

func makeDMRecord(peer, from, content string, createdAt int64, protocol, eventID string) *dmRecord {
	return &dmRecord{
		id:        dmDedupID(peer, content, createdAt),
		peer:      peer,
		from:      from,
		content:   content,
		createdAt: createdAt,
		protocol:  protocol,
		eventID:   eventID,
	}
}

func dmDedupID(peer, content string, createdAt int64) string {
	contentHash := sha256.Sum([]byte(content))
	window := helpers.Itoa(createdAt / 300)
	combined := sha256.Sum([]byte(peer + helpers.HexEncode(contentHash[:]) + window))
	return helpers.HexEncode(combined[:])
}

func encryptNip04DM(recipientPubkey, content string, cb func(*nostr.Event)) {
	sk04, ok := nip04.SharedKey(hexTo32(registry.Seckey()), hexTo32(recipientPubkey))
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
		if signEvent(ev) {
			cb(ev)
		}
	})
}

func encryptNip17DM(recipientPubkey, content string) (*nostr.Event, *nostr.Event) {
	now := sw.NowSeconds()
	sk := hexTo32(registry.Seckey())
	myPub := registry.Pubkey()

	inner := &nostr.Event{
		Kind:      14,
		Content:   content,
		Tags:      nostr.Tags{{"p", recipientPubkey}},
		CreatedAt: now,
	}
	if !signEvent(inner) {
		return nil, nil
	}
	innerJSON := inner.ToJSON()

	recipientCK, ok := nip44.ConversationKey(sk, hexTo32(recipientPubkey))
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
	if !signEvent(recipientSeal) {
		return nil, nil
	}
	recipientWrap := giftWrap(recipientSeal, recipientPubkey, now)

	senderCK, ok := nip44.ConversationKey(sk, hexTo32(myPub))
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
	if !signEvent(senderSeal) {
		return recipientWrap, nil
	}
	senderWrap := giftWrap(senderSeal, myPub, now)

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
