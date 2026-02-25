package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"git.mleku.dev/mleku/nostr/encoders/event"
	"git.mleku.dev/mleku/nostr/encoders/tag"
	"lol.mleku.dev/log"
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

	log.I.F("published kind 0 profile (%d fields) for bridge identity", len(profile))
	return nil
}
