package nostr

// Filter is a Nostr subscription filter (NIP-01).
type Filter struct {
	IDs     []string
	Authors []string
	Kinds   []int
	Tags    map[string][]string // "#e" -> ["abc"], "#p" -> ["def"]
	Since   int64
	Until   int64
	Limit   int
	Proxy   []string // _proxy extension: relay URLs to fetch from via orly relay
}

// Matches checks if an event passes the filter.
func (f *Filter) Matches(e *Event) bool {
	if len(f.IDs) > 0 && !containsStr(f.IDs, e.ID) {
		return false
	}
	if len(f.Authors) > 0 && !containsStr(f.Authors, e.PubKey) {
		return false
	}
	if len(f.Kinds) > 0 && !containsInt(f.Kinds, e.Kind) {
		return false
	}
	if f.Since > 0 && e.CreatedAt < f.Since {
		return false
	}
	if f.Until > 0 && e.CreatedAt > f.Until {
		return false
	}
	if f.Tags != nil {
		for key, values := range f.Tags {
			if len(key) < 2 || key[0] != '#' {
				continue
			}
			tagKey := string(key[1:])
			if !eventHasTagValue(e, tagKey, values) {
				return false
			}
		}
	}
	return true
}

// Serialize returns the filter as a JSON object string for REQ messages.
func (f *Filter) Serialize() string {
	buf := make([]byte, 0, 128)
	buf = append(buf, '{')
	first := true

	if len(f.IDs) > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"ids\":"...)
		buf = appendStrArray(buf, f.IDs)
	}
	if len(f.Authors) > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"authors\":"...)
		buf = appendStrArray(buf, f.Authors)
	}
	if len(f.Kinds) > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"kinds\":["...)
		for i, k := range f.Kinds {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = append(buf, intToStr(k)...)
		}
		buf = append(buf, ']')
	}
	if f.Tags != nil {
		for key, values := range f.Tags {
			buf = appendField(buf, &first)
			buf = append(buf, '"')
			buf = append(buf, key...)
			buf = append(buf, "\":"...)
			buf = appendStrArray(buf, values)
		}
	}
	if f.Since > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"since\":"...)
		buf = append(buf, i64ToStr(f.Since)...)
	}
	if f.Until > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"until\":"...)
		buf = append(buf, i64ToStr(f.Until)...)
	}
	if f.Limit > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"limit\":"...)
		buf = append(buf, intToStr(f.Limit)...)
	}
	if len(f.Proxy) > 0 {
		buf = appendField(buf, &first)
		buf = append(buf, "\"_proxy\":"...)
		buf = appendStrArray(buf, f.Proxy)
	}

	buf = append(buf, '}')
	return string(buf)
}

func appendField(buf []byte, first *bool) []byte {
	if !*first {
		buf = append(buf, ',')
	}
	*first = false
	return buf
}

func appendStrArray(buf []byte, ss []string) []byte {
	buf = append(buf, '[')
	for i, s := range ss {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '"')
		buf = append(buf, s...)
		buf = append(buf, '"')
	}
	buf = append(buf, ']')
	return buf
}

func i64ToStr(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func containsInt(ns []int, n int) bool {
	for _, v := range ns {
		if v == n {
			return true
		}
	}
	return false
}

func eventHasTagValue(e *Event, tagKey string, values []string) bool {
	for _, t := range e.Tags {
		if len(t) > 1 && t[0] == tagKey {
			for _, v := range values {
				if t[1] == v {
					return true
				}
			}
		}
	}
	return false
}
