package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"next.orly.dev/pkg/nostr/encoders/event"
	"next.orly.dev/pkg/nostr/encoders/kind"
	"next.orly.dev/pkg/nostr/encoders/tag"
	"next.orly.dev/pkg/nostr/ws"
	"next.orly.dev/pkg/lol/log"
)

// parseProfileTemplate reads a profile template file in email-header format
// (key: value lines, one per line, blank line ends headers). Returns a map
// of profile fields suitable for JSON marshaling as kind 0 content.
func parseProfileTemplate(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	profile := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			break // blank line ends headers
		}
		if line[0] == '#' {
			continue // skip comments
		}
		i := strings.IndexByte(line, ':')
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(line[:i]))
		val := strings.TrimSpace(line[i+1:])
		if val != "" {
			profile[key] = val
		}
	}
	return profile, nil
}

// publishProfile reads the profile template and publishes a kind 0 metadata
// event to the relay. Silently returns nil if the template file doesn't exist.
func (b *Bridge) publishProfile() error {
	path := b.cfg.ProfilePath
	if path == "" {
		return nil
	}

	profile, err := parseProfileTemplate(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.D.F("no profile template at %s, skipping kind 0 publish", path)
			return nil
		}
		return fmt.Errorf("parse profile template %s: %w", path, err)
	}
	if len(profile) == 0 {
		log.D.F("profile template %s is empty, skipping kind 0 publish", path)
		return nil
	}

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

	log.D.F("published kind 0 profile (%d fields) for bridge identity", len(profile))
	return nil
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
	if b.cfg.ProfilePath != "" {
		profile, err := parseProfileTemplate(b.cfg.ProfilePath)
		if err == nil && len(profile) > 0 {
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
