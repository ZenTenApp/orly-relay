// Package nostr implements core Nostr protocol types.
package nostr

// Tag is a single tag: ["e", "abc123", "wss://relay"] etc.
type Tag []string

// Tags is a list of tags.
type Tags []Tag

// Key returns the tag key (first element), or empty string.
func (t Tag) Key() string {
	if len(t) > 0 {
		return t[0]
	}
	return ""
}

// Value returns the tag value (second element), or empty string.
func (t Tag) Value() string {
	if len(t) > 1 {
		return t[1]
	}
	return ""
}

// Relay returns the third element (relay hint), or empty string.
func (t Tag) Relay() string {
	if len(t) > 2 {
		return t[2]
	}
	return ""
}

// Marker returns the fourth element (e-tag marker), or empty string.
func (t Tag) Marker() string {
	if len(t) > 3 {
		return t[3]
	}
	return ""
}

// GetFirst returns the first tag matching key, or nil.
func (ts Tags) GetFirst(key string) Tag {
	for _, t := range ts {
		if len(t) > 0 && t[0] == key {
			return t
		}
	}
	return nil
}

// GetAll returns all tags matching key.
func (ts Tags) GetAll(key string) Tags {
	var out Tags
	for _, t := range ts {
		if len(t) > 0 && t[0] == key {
			out = append(out, t)
		}
	}
	return out
}

// GetD returns the "d" tag value (for replaceable events).
func (ts Tags) GetD() string {
	t := ts.GetFirst("d")
	if t != nil {
		return t.Value()
	}
	return ""
}

// ContainsValue checks if any tag with the given key has the given value.
func (ts Tags) ContainsValue(key, value string) bool {
	for _, t := range ts {
		if len(t) > 1 && t[0] == key && t[1] == value {
			return true
		}
	}
	return false
}
