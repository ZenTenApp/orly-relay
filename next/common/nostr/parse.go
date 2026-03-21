package nostr

// Minimal JSON parsing for Nostr relay messages.
// No encoding/json. Hand-rolled for speed.

// ParseEvent parses a JSON event object into an Event.
func ParseEvent(s string) *Event {
	ev := &Event{}
	i := skipWS(s, 0)
	if i >= len(s) || s[i] != '{' {
		return nil
	}
	i++
	for i < len(s) {
		i = skipWS(s, i)
		if i >= len(s) {
			return nil
		}
		if s[i] == '}' {
			return ev
		}
		if s[i] == ',' {
			i++
			continue
		}
		// Key.
		key, ni := parseString(s, i)
		if ni < 0 {
			return nil
		}
		i = skipWS(s, ni)
		if i >= len(s) || s[i] != ':' {
			return nil
		}
		i = skipWS(s, i+1)

		switch key {
		case "id":
			ev.ID, i = parseString(s, i)
			if i < 0 {
				return nil
			}
		case "pubkey":
			ev.PubKey, i = parseString(s, i)
			if i < 0 {
				return nil
			}
		case "created_at":
			ev.CreatedAt, i = parseInt(s, i)
			if i < 0 {
				return nil
			}
		case "kind":
			var k int64
			k, i = parseInt(s, i)
			if i < 0 {
				return nil
			}
			ev.Kind = int(k)
		case "content":
			ev.Content, i = parseString(s, i)
			if i < 0 {
				return nil
			}
		case "sig":
			ev.Sig, i = parseString(s, i)
			if i < 0 {
				return nil
			}
		case "tags":
			ev.Tags, i = parseTags(s, i)
			if i < 0 {
				return nil
			}
		default:
			// Skip unknown field value.
			i = skipValue(s, i)
			if i < 0 {
				return nil
			}
		}
	}
	return ev
}

// ParseRelayMessage parses a relay message array.
// Returns (label, subscriptionID, payload) where:
//   - EVENT:  label="EVENT", subID set, payload = event JSON string
//   - EOSE:   label="EOSE", subID set
//   - OK:     label="OK", subID = eventID, payload = "true:<msg>" or "false:<msg>"
//   - NOTICE: label="NOTICE", payload = message
//   - AUTH:   label="AUTH", payload = challenge
func ParseRelayMessage(s string) (label, subID, payload string) {
	i := skipWS(s, 0)
	if i >= len(s) || s[i] != '[' {
		return
	}
	i = skipWS(s, i+1)

	// First element: label string.
	label, i = parseString(s, i)
	if i < 0 {
		label = ""
		return
	}

	switch label {
	case "EVENT":
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		subID, i = parseString(s, i)
		if i < 0 {
			return
		}
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		// Rest until closing ] is the event JSON.
		start := i
		i = skipValue(s, i)
		if i < 0 {
			return
		}
		payload = s[start:i]

	case "EOSE":
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		subID, i = parseString(s, i)

	case "OK":
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		subID, i = parseString(s, i) // actually eventID
		if i < 0 {
			return
		}
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		// Boolean.
		ok := false
		if i+4 <= len(s) && s[i:i+4] == "true" {
			ok = true
			i += 4
		} else if i+5 <= len(s) && s[i:i+5] == "false" {
			i += 5
		}
		// Optional message.
		i = skipWS(s, i)
		msg := ""
		if i < len(s) && s[i] == ',' {
			i = skipWS(s, i+1)
			msg, i = parseString(s, i)
		}
		if ok {
			payload = "true:" + msg
		} else {
			payload = "false:" + msg
		}

	case "NOTICE":
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		payload, i = parseString(s, i)

	case "AUTH":
		i = skipWS(s, i)
		if i >= len(s) || s[i] != ',' {
			return
		}
		i = skipWS(s, i+1)
		payload, i = parseString(s, i)
	}

	return
}

// ParseFilter parses a JSON filter object into a Filter.
func ParseFilter(s string) *Filter {
	f := &Filter{}
	i := skipWS(s, 0)
	if i >= len(s) || s[i] != '{' {
		return nil
	}
	i++
	for i < len(s) {
		i = skipWS(s, i)
		if i >= len(s) {
			return nil
		}
		if s[i] == '}' {
			return f
		}
		if s[i] == ',' {
			i++
			continue
		}
		key, ni := parseString(s, i)
		if ni < 0 {
			return nil
		}
		i = skipWS(s, ni)
		if i >= len(s) || s[i] != ':' {
			return nil
		}
		i = skipWS(s, i+1)

		switch key {
		case "ids":
			f.IDs, i = parseStrArray(s, i)
		case "authors":
			f.Authors, i = parseStrArray(s, i)
		case "kinds":
			f.Kinds, i = parseIntArray(s, i)
		case "since":
			f.Since, i = parseInt(s, i)
		case "until":
			f.Until, i = parseInt(s, i)
		case "limit":
			var l int64
			l, i = parseInt(s, i)
			f.Limit = int(l)
		default:
			if len(key) == 2 && key[0] == '#' {
				if f.Tags == nil {
					f.Tags = make(map[string][]string)
				}
				f.Tags[key], i = parseStrArray(s, i)
			} else {
				i = skipValue(s, i)
			}
		}
		if i < 0 {
			return nil
		}
	}
	return f
}

// ParseEventsJSON parses a JSON array of event objects.
func ParseEventsJSON(s string) []*Event {
	i := skipWS(s, 0)
	if i >= len(s) || s[i] != '[' {
		return nil
	}
	i++
	var events []*Event
	for {
		i = skipWS(s, i)
		if i >= len(s) {
			return events
		}
		if s[i] == ']' {
			return events
		}
		if s[i] == ',' {
			i++
			continue
		}
		start := i
		i = skipValue(s, i)
		if i < 0 {
			return events
		}
		ev := ParseEvent(s[start:i])
		if ev != nil {
			events = append(events, ev)
		}
	}
}

func parseStrArray(s string, i int) ([]string, int) {
	i = skipWS(s, i)
	if i >= len(s) || s[i] != '[' {
		return nil, -1
	}
	i++
	var out []string
	for {
		i = skipWS(s, i)
		if i >= len(s) {
			return nil, -1
		}
		if s[i] == ']' {
			return out, i + 1
		}
		if s[i] == ',' {
			i++
			continue
		}
		v, ni := parseString(s, i)
		if ni < 0 {
			return nil, -1
		}
		out = append(out, v)
		i = ni
	}
}

func parseIntArray(s string, i int) ([]int, int) {
	i = skipWS(s, i)
	if i >= len(s) || s[i] != '[' {
		return nil, -1
	}
	i++
	var out []int
	for {
		i = skipWS(s, i)
		if i >= len(s) {
			return nil, -1
		}
		if s[i] == ']' {
			return out, i + 1
		}
		if s[i] == ',' {
			i++
			continue
		}
		n, ni := parseInt(s, i)
		if ni < 0 {
			return nil, -1
		}
		out = append(out, int(n))
		i = ni
	}
}

// --- Low-level JSON parsing ---

func skipWS(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

func parseString(s string, i int) (string, int) {
	if i >= len(s) || s[i] != '"' {
		return "", -1
	}
	i++
	start := i
	// Use string concat, not []byte — tinyjs strings are UTF-16, byte ops corrupt emoji.
	result := ""
	for i < len(s) {
		if s[i] == '\\' {
			result += s[start:i]
			i++
			if i >= len(s) {
				return "", -1
			}
			switch s[i] {
			case '"', '\\', '/':
				result += s[i : i+1]
			case 'n':
				result += "\n"
			case 'r':
				result += "\r"
			case 't':
				result += "\t"
			case 'b':
				result += "\b"
			case 'f':
				result += "\f"
			case 'u':
				if i+4 >= len(s) {
					return "", -1
				}
				cp := hexVal(s[i+1])<<12 | hexVal(s[i+2])<<8 | hexVal(s[i+3])<<4 | hexVal(s[i+4])
				// Surrogate pair: \uD800-\uDBFF followed by \uDC00-\uDFFF.
				if cp >= 0xD800 && cp <= 0xDBFF && i+10 <= len(s) && s[i+5] == '\\' && s[i+6] == 'u' {
					lo := hexVal(s[i+7])<<12 | hexVal(s[i+8])<<8 | hexVal(s[i+9])<<4 | hexVal(s[i+10])
					if lo >= 0xDC00 && lo <= 0xDFFF {
						cp = 0x10000 + (cp-0xD800)*0x400 + (lo - 0xDC00)
						i += 6
					}
				}
				result += string(rune(cp))
				i += 4
			default:
				result += s[i : i+1]
			}
			i++
			start = i
			continue
		}
		if s[i] == '"' {
			result += s[start:i]
			return result, i + 1
		}
		i++
	}
	return "", -1
}

func hexVal(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		return int(c-'a') + 10
	}
	if c >= 'A' && c <= 'F' {
		return int(c-'A') + 10
	}
	return 0
}

func parseInt(s string, i int) (int64, int) {
	if i >= len(s) {
		return 0, -1
	}
	neg := false
	if s[i] == '-' {
		neg = true
		i++
	}
	if i >= len(s) || s[i] < '0' || s[i] > '9' {
		return 0, -1
	}
	var n int64
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int64(s[i]-'0')
		i++
	}
	if neg {
		n = -n
	}
	return n, i
}

func parseTags(s string, i int) (Tags, int) {
	if i >= len(s) || s[i] != '[' {
		return nil, -1
	}
	i++
	var tags Tags
	for {
		i = skipWS(s, i)
		if i >= len(s) {
			return nil, -1
		}
		if s[i] == ']' {
			return tags, i + 1
		}
		if s[i] == ',' {
			i++
			continue
		}
		// Parse inner array.
		if s[i] != '[' {
			return nil, -1
		}
		i++
		var tag Tag
		for {
			i = skipWS(s, i)
			if i >= len(s) {
				return nil, -1
			}
			if s[i] == ']' {
				i++
				break
			}
			if s[i] == ',' {
				i++
				continue
			}
			var val string
			val, i = parseString(s, i)
			if i < 0 {
				return nil, -1
			}
			tag = append(tag, val)
		}
		tags = append(tags, tag)
	}
}

// skipValue skips a JSON value (string, number, object, array, bool, null).
func skipValue(s string, i int) int {
	if i >= len(s) {
		return -1
	}
	switch s[i] {
	case '"':
		_, ni := parseString(s, i)
		return ni
	case '{':
		return skipBracketed(s, i, '{', '}')
	case '[':
		return skipBracketed(s, i, '[', ']')
	case 't': // true
		if i+4 <= len(s) {
			return i + 4
		}
		return -1
	case 'f': // false
		if i+5 <= len(s) {
			return i + 5
		}
		return -1
	case 'n': // null
		if i+4 <= len(s) {
			return i + 4
		}
		return -1
	default:
		// Number.
		for i < len(s) && s[i] != ',' && s[i] != '}' && s[i] != ']' && s[i] != ' ' && s[i] != '\n' {
			i++
		}
		return i
	}
}

func skipBracketed(s string, i int, open, close byte) int {
	if i >= len(s) || s[i] != open {
		return -1
	}
	depth := 1
	i++
	inStr := false
	for i < len(s) && depth > 0 {
		if inStr {
			if s[i] == '\\' {
				i++
			} else if s[i] == '"' {
				inStr = false
			}
		} else {
			if s[i] == '"' {
				inStr = true
			} else if s[i] == open {
				depth++
			} else if s[i] == close {
				depth--
			}
		}
		i++
	}
	if depth != 0 {
		return -1
	}
	return i
}
