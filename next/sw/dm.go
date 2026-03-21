package main

import (
	"common/crypto/nip04"
	"common/crypto/nip44"
	"common/crypto/secp256k1"
	"common/crypto/sha256"
	"common/helpers"
	"common/jsbridge/idb"
	"common/jsbridge/sw"
	"common/nostr"
)

// processIncomingDM handles kind 4 and 1059 events.
func processIncomingDM(ev *nostr.Event) {
	if myPubkey == "" {
		return
	}
	if ev.Kind == 4 {
		processNip04DM(ev)
	} else if ev.Kind == 1059 {
		processNip17DM(ev)
	}
}

func processNip04DM(ev *nostr.Event) {
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
		saveDMRecord(peer, ev.PubKey, plaintext, ev.CreatedAt, "nip04", ev.ID)
	})
}

func processNip17DM(ev *nostr.Event) {
	if !hasKey {
		return
	}
	// Unwrap gift wrap: decrypt outer (seal), then inner (rumor).
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
	saveDMRecord(peer, senderPub, inner.Content, ts, "nip17", ev.ID)
}

func saveDMRecord(peer, from, content string, createdAt int64, protocol, eventID string) {
	dmID := dmDedupID(peer, content, createdAt)
	dmJSON := "{" +
		"\"id\":" + jstr(dmID) +
		",\"peer\":" + jstr(peer) +
		",\"from\":" + jstr(from) +
		",\"content\":" + jstr(content) +
		",\"created_at\":" + helpers.Itoa(createdAt) +
		",\"protocol\":" + jstr(protocol) +
		",\"eventId\":" + jstr(eventID) +
		"}"

	idb.SaveDM(dmJSON, func(result string) {
		if result != "duplicate" {
			broadcastToClients("[\"DM_RECEIVED\"," + dmJSON + "]")
		}
	})
}

func dmDedupID(peer, content string, createdAt int64) string {
	contentHash := sha256.Sum([]byte(content))
	window := helpers.Itoa(createdAt / 300)
	combined := sha256.Sum([]byte(peer + helpers.HexEncode(contentHash[:]) + window))
	return helpers.HexEncode(combined[:])
}

// sendDM sends a DM to a recipient via NIP-04 and NIP-17.
func sendDM(clientID, recipientPubkey, content string, relayURLs []string) {
	if myPubkey == "" || !hasKey {
		return
	}

	// NIP-04.
	sk04, ok := nip04.SharedKey(seckey, hexTo32(recipientPubkey))
	if ok {
		nip04.Encrypt(sk04, content, func(ciphertext string) {
			ev04 := &nostr.Event{
				Kind:      4,
				Content:   ciphertext,
				Tags:      nostr.Tags{{"p", recipientPubkey}},
				CreatedAt: sw.NowSeconds(),
			}
			aux := random32()
			if ev04.Sign(seckey, aux) {
				idb.SaveEvent(ev04.ToJSON(), func(_ bool) {})
				for _, url := range relayURLs {
					getConn(url).Publish(ev04)
				}
			}
		})
	}

	// NIP-17 (synchronous: NIP-44 + signing).
	sendNip17DM(recipientPubkey, content, relayURLs)

	// Save sent DM record.
	now := sw.NowSeconds()
	saveDMRecord(recipientPubkey, myPubkey, content, now, "nip17", "")
	sendToClient(clientID, "[\"DM_SENT\","+jstr(recipientPubkey)+",true,\"\"]")
}

func sendNip17DM(recipientPubkey, content string, relayURLs []string) {
	now := sw.NowSeconds()

	// Inner event (kind 14).
	inner := &nostr.Event{
		Kind:      14,
		Content:   content,
		Tags:      nostr.Tags{{"p", recipientPubkey}},
		CreatedAt: now,
	}
	aux := random32()
	if !inner.Sign(seckey, aux) {
		return
	}
	innerJSON := inner.ToJSON()

	// Recipient copy.
	recipientCK, ok := nip44.ConversationKey(seckey, hexTo32(recipientPubkey))
	if !ok {
		return
	}
	recipientSealContent := nip44.Encrypt(innerJSON, recipientCK, random32())
	recipientSeal := &nostr.Event{
		Kind:      13,
		Content:   recipientSealContent,
		Tags:      nostr.Tags{},
		CreatedAt: randomizeTimestamp(now),
	}
	aux = random32()
	if !recipientSeal.Sign(seckey, aux) {
		return
	}
	recipientWrap := giftWrap(recipientSeal, recipientPubkey, now)
	if recipientWrap != nil {
		for _, url := range relayURLs {
			getConn(url).Publish(recipientWrap)
		}
	}

	// Sender (self) copy.
	senderCK, ok := nip44.ConversationKey(seckey, hexTo32(myPubkey))
	if !ok {
		return
	}
	senderSealContent := nip44.Encrypt(innerJSON, senderCK, random32())
	senderSeal := &nostr.Event{
		Kind:      13,
		Content:   senderSealContent,
		Tags:      nostr.Tags{},
		CreatedAt: randomizeTimestamp(now),
	}
	aux = random32()
	if !senderSeal.Sign(seckey, aux) {
		return
	}
	senderWrap := giftWrap(senderSeal, myPubkey, now)
	if senderWrap != nil {
		for _, url := range relayURLs {
			getConn(url).Publish(senderWrap)
		}
	}
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
	// Subtract 0-2 days (simple deterministic-ish via random bytes).
	var b [4]byte
	subtle2 := random32()
	b[0] = subtle2[0]
	b[1] = subtle2[1]
	b[2] = subtle2[2]
	b[3] = subtle2[3]
	offset := int64(b[0])<<8 | int64(b[1])
	offset = (offset % (2 * 24 * 60 * 60))
	return baseTime - offset
}

// DM subscriptions.
func handleDMSub(_ string, relayURLs []string) {
	if myPubkey == "" || len(relayURLs) == 0 {
		return
	}
	dmRelayURLs = relayURLs

	// Close existing DM subs.
	for rSubID := range dmSubIDs {
		for _, url := range rpool.URLs() {
			c := rpool.Get(url)
			if c != nil && c.IsOpen() {
				c.CloseSubscription(rSubID)
			}
		}
	}
	dmSubIDs = make(map[string]bool)
	openDMSubs(relayURLs)
}

func openDMSubs(relayURLs []string) {
	for _, url := range relayURLs {
		suffix := urlSuffix(url)
		id1 := "dm4in_" + suffix
		id2 := "dm4out_" + suffix
		id3 := "dm17_" + suffix
		dmSubIDs[id1] = true
		dmSubIDs[id2] = true
		dmSubIDs[id3] = true

		c := getConn(url)
		c.Subscribe(id1, []*nostr.Filter{{Kinds: []int{4}, Tags: map[string][]string{"#p": {myPubkey}}, Limit: 100}})
		c.Subscribe(id2, []*nostr.Filter{{Kinds: []int{4}, Authors: []string{myPubkey}, Limit: 100}})
		c.Subscribe(id3, []*nostr.Filter{{Kinds: []int{1059}, Tags: map[string][]string{"#p": {myPubkey}}, Limit: 100}})
	}
}

func handleDMList(clientID string) {
	idb.GetConversationList(func(listJSON string) {
		sendToClient(clientID, "[\"DM_LIST\","+listJSON+"]")
	})
}

func handleDMHistory(clientID, peer string, limit int, until int64) {
	if limit <= 0 {
		limit = 50
	}
	idb.QueryDMs(peer, limit, until, func(msgsJSON string) {
		sendToClient(clientID, "[\"DM_HISTORY\","+jstr(peer)+","+msgsJSON+"]")
	})
}

// Identity broadcast.
func broadcastIdentity(clientID, pubkey string, relayURLs []string) {
	filterJSON := "{\"authors\":[" + jstr(pubkey) + "],\"kinds\":[0,3,10002,10050,10051]}"
	idb.QueryEvents(filterJSON, func(eventsJSON string) {
		events := nostr.ParseEventsJSON(eventsJSON)
		byKind := make(map[int]*nostr.Event)
		for _, ev := range events {
			if prev, ok := byKind[ev.Kind]; !ok || ev.CreatedAt > prev.CreatedAt {
				byKind[ev.Kind] = ev
			}
		}

		// Auto-create 10050/10051 if missing.
		userRelays := relayURLs
		if relayEv, ok := byKind[10002]; ok {
			userRelays = nil
			for _, t := range relayEv.Tags.GetAll("r") {
				userRelays = append(userRelays, t.Value())
			}
		}
		if len(userRelays) == 0 {
			userRelays = writeRelays
		}

		if _, ok := byKind[10050]; !ok && hasKey && len(userRelays) > 0 {
			ev := createRelayListEvent(10050, pubkey, userRelays)
			if ev != nil {
				idb.SaveEvent(ev.ToJSON(), func(_ bool) {})
				byKind[10050] = ev
			}
		}
		if _, ok := byKind[10051]; !ok && hasKey && len(userRelays) > 0 {
			ev := createRelayListEvent(10051, pubkey, userRelays)
			if ev != nil {
				idb.SaveEvent(ev.ToJSON(), func(_ bool) {})
				byKind[10051] = ev
			}
		}

		// Send all to relays.
		count := 0
		for _, ev := range byKind {
			for _, url := range relayURLs {
				getConn(url).Publish(ev)
			}
			count++
		}
		sendToClient(clientID, "[\"BROADCAST_DONE\","+helpers.Itoa(int64(count))+","+helpers.Itoa(int64(len(relayURLs)))+"]")
	})
}

func createRelayListEvent(kind int, _ string, relays []string) *nostr.Event {
	tagKey := "relay"
	var tags nostr.Tags
	for _, r := range relays {
		tags = append(tags, nostr.Tag{tagKey, r})
	}
	ev := &nostr.Event{
		Kind:      kind,
		Content:   "",
		Tags:      tags,
		CreatedAt: sw.NowSeconds(),
	}
	aux := random32()
	if !ev.Sign(seckey, aux) {
		return nil
	}
	return ev
}
