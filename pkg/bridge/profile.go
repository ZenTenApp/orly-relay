package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/kind"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/ws"
	"git.smesh.lol/orly/pkg/lol/log"
)

// bridgeAbout returns the desired about text for the bridge profile.
func (b *Bridge) bridgeAbout() string {
	url := b.publicRelayURL()
	if url == "" {
		url = b.cfg.RelayURL
	}
	return "nostr to email bridge at " + url
}

// publishProfile publishes a kind 0 metadata event for the bridge.
// Checks the existing profile first and only publishes if the about
// text doesn't match the current config.
func (b *Bridge) publishProfile() error {
	wantAbout := b.bridgeAbout()

	// Fetch current kind 0 to check if update is needed.
	existing := b.relay.FetchKind0(b.ctx, b.sign.Pub())
	if existing != nil {
		currentAbout := extractJSONString(string(existing.Content), "about")
		currentPicture := extractJSONString(string(existing.Content), "picture")
		currentNip05 := extractJSONString(string(existing.Content), "nip05")
		wantPicture := "https://relay.orly.dev/static/marmot-bridge-avatar.png"
		wantNip05 := "bridge@" + b.cfg.Domain
		if currentAbout == wantAbout && currentPicture == wantPicture && currentNip05 == wantNip05 {
			log.D.F("bridge profile already up to date")
			return nil
		}
	}

	// Build profile preserving existing fields if any, updating about.
	profile := make(map[string]string)
	if existing != nil {
		// Preserve name/picture/etc from existing profile.
		for _, key := range []string{"name", "picture", "display_name", "website", "lud16", "nip05", "banner"} {
			val := extractJSONString(string(existing.Content), key)
			if val != "" {
				profile[key] = val
			}
		}
	}
	if profile["name"] == "" {
		profile["name"] = "marmot bridge"
	}
	if profile["picture"] == "" {
		profile["picture"] = "https://relay.orly.dev/static/marmot-bridge-avatar.png"
	}
	if profile["nip05"] == "" {
		profile["nip05"] = "bridge@" + b.cfg.Domain
	}
	profile["about"] = wantAbout

	content, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	ev := &event.E{
		Content:   content,
		CreatedAt: time.Now().Unix(),
		Kind:      0,
		Tags:      tag.NewS(),
	}
	if err := ev.Sign(b.sign); err != nil {
		return fmt.Errorf("sign profile event: %w", err)
	}
	if err := b.relay.Publish(b.ctx, ev); err != nil {
		return fmt.Errorf("publish profile event: %w", err)
	}

	log.I.F("published kind 0 profile: %s", wantAbout)
	return nil
}

// extractJSONString extracts a string value from a JSON object by key.
// Minimal parser — no dependencies.
func extractJSONString(json, key string) string {
	needle := "\"" + key + "\""
	i := strings.Index(json, needle)
	if i < 0 {
		return ""
	}
	i += len(needle)
	// Skip : and whitespace
	for i < len(json) && (json[i] == ':' || json[i] == ' ' || json[i] == '\t') {
		i++
	}
	if i >= len(json) || json[i] != '"' {
		return ""
	}
	i++ // skip opening quote
	var b strings.Builder
	for i < len(json) {
		if json[i] == '\\' && i+1 < len(json) {
			i++
			switch json[i] {
			case '"', '\\', '/':
				b.WriteByte(json[i])
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte('\\')
				b.WriteByte(json[i])
			}
			i++
			continue
		}
		if json[i] == '"' {
			return b.String()
		}
		b.WriteByte(json[i])
		i++
	}
	return ""
}

// publicRelayURL returns the public relay URL for relay list events.
// Prefers PublicRelayURL, falls back to RelayURL.
func (b *Bridge) publicRelayURL() string {
	if b.cfg.PublicRelayURL != "" {
		return b.cfg.PublicRelayURL
	}
	return b.cfg.RelayURL
}

// publishRelayList publishes kind 10002 (relay list) and kind 10050 (DM inbox
// relays) events pointing to the bridge's home relay.
func (b *Bridge) publishRelayList() error {
	pubURL := b.publicRelayURL()
	if pubURL == "" {
		return nil
	}

	relayURL := []byte(pubURL)

	// Kind 10002: general relay list (read + write)
	relayListEv := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.RelayListMetadata.K,
		Tags: tag.NewS(
			tag.NewFromBytesSlice([]byte("r"), relayURL),
		),
	}
	if err := relayListEv.Sign(b.sign); err != nil {
		return fmt.Errorf("sign relay list event: %w", err)
	}
	if err := b.relay.Publish(b.ctx, relayListEv); err != nil {
		return fmt.Errorf("publish relay list event: %w", err)
	}
	log.D.F("published kind 10002 relay list: %s", pubURL)

	// Kind 10050: DM inbox relays (NIP-17)
	dmRelayEv := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.DMRelaysList.K,
		Tags: tag.NewS(
			tag.NewFromBytesSlice([]byte("relay"), relayURL),
		),
	}
	if err := dmRelayEv.Sign(b.sign); err != nil {
		return fmt.Errorf("sign DM relay list event: %w", err)
	}
	if err := b.relay.Publish(b.ctx, dmRelayEv); err != nil {
		return fmt.Errorf("publish DM relay list event: %w", err)
	}
	log.D.F("published kind 10050 DM relay list: %s", pubURL)

	return nil
}

// broadcastRelays is the set of popular relays to broadcast identity events to.
var broadcastRelays = []string{
	"wss://relay.damus.io",
	"wss://nos.lol",
	"wss://relay.nostr.band",
	"wss://purplepag.es",
	"wss://relay.primal.net",
}

// broadcastIdentity publishes the bridge's kind 0, kind 10002, and kind 10050
// events to popular relays so outbox-model clients can discover the bridge.
// Errors are logged but not fatal.
func (b *Bridge) broadcastIdentity() {
	pubURL := b.publicRelayURL()
	if pubURL == "" {
		return
	}

	// Build all three events
	var events []*event.E

	// Kind 0 profile
	{
		profile := map[string]string{
			"name":    "marmot bridge",
			"about":   b.bridgeAbout(),
			"picture": "https://relay.orly.dev/static/marmot-bridge-avatar.png",
			"nip05":   "bridge@" + b.cfg.Domain,
		}
		content, err := json.Marshal(profile)
		if err == nil {
			ev := &event.E{
				Content:   content,
				CreatedAt: time.Now().Unix(),
				Kind:      0,
				Tags:      tag.NewS(),
			}
			if err := ev.Sign(b.sign); err == nil {
				events = append(events, ev)
			}
		}
	}

	relayURL := []byte(pubURL)

	// Kind 10002
	relayListEv := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.RelayListMetadata.K,
		Tags: tag.NewS(
			tag.NewFromBytesSlice([]byte("r"), relayURL),
		),
	}
	if err := relayListEv.Sign(b.sign); err == nil {
		events = append(events, relayListEv)
	}

	// Kind 10050
	dmRelayEv := &event.E{
		CreatedAt: time.Now().Unix(),
		Kind:      kind.DMRelaysList.K,
		Tags: tag.NewS(
			tag.NewFromBytesSlice([]byte("relay"), relayURL),
		),
	}
	if err := dmRelayEv.Sign(b.sign); err == nil {
		events = append(events, dmRelayEv)
	}

	// Add MLS key package and relay list events if MLS is enabled
	if b.mlsClient != nil {
		if kpEv, err := b.mlsKeyPackageEvent(); err == nil {
			events = append(events, kpEv)
		}
		if kprEv, err := b.mlsKeyPackageRelaysEvent(pubURL); err == nil {
			events = append(events, kprEv)
		}
	}

	if len(events) == 0 {
		return
	}

	log.I.F("broadcasting %d identity events to %d relays", len(events), len(broadcastRelays))

	for _, relayURL := range broadcastRelays {
		go func(url string) {
			ctx, cancel := context.WithTimeout(b.ctx, 10*time.Second)
			defer cancel()

			conn, err := ws.RelayConnect(ctx, url)
			if err != nil {
				log.D.F("broadcast: connect to %s failed: %v", url, err)
				return
			}
			defer conn.Close()

			for _, ev := range events {
				if err := conn.Publish(ctx, ev); err != nil {
					log.D.F("broadcast: publish kind %d to %s failed: %v", ev.Kind, url, err)
				}
			}
			log.D.F("broadcast: published %d events to %s", len(events), url)
		}(relayURL)
	}
}
