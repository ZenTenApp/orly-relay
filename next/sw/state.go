package main

import (
	"common/helpers"
	"common/jsbridge/subtle"
	"common/jsbridge/sw"
	"common/nostr"
	"common/relay"
)

// Module state.
var (
	seckey      [32]byte
	hasKey      bool
	myPubkey    string
	writeRelays []string
	rpool       *relay.Pool
	clientSubs  map[string]*clientSub
	proxySubs   map[string]*proxySub
	dmSubIDs    map[string]bool
	dmRelayURLs []string
	cryptoCBs   map[int]func(string, string)
	nextCryptoID int
)

type clientSub struct {
	filter    *nostr.Filter
	filterRaw string
	clientID  string
}

type proxySub struct {
	remoteIDs  map[string]bool
	relayCount int
	eoseCount  int
	timer      sw.Timer
	done       bool
}

func initState() {
	rpool = relay.NewPool(16)
	clientSubs = make(map[string]*clientSub)
	proxySubs = make(map[string]*proxySub)
	dmSubIDs = make(map[string]bool)
	cryptoCBs = make(map[int]func(string, string))
}

func sendToClient(clientID, msg string) {
	sw.GetClientByID(clientID, func(c sw.Client, ok bool) {
		if ok {
			sw.PostMessageJSON(c, msg)
		}
	})
}

func broadcastToClients(msg string) {
	sw.MatchClients(func(c sw.Client) {
		sw.PostMessageJSON(c, msg)
	})
}

func jstr(s string) string { return helpers.JsonString(s) }

func hexTo32(s string) [32]byte {
	out, _ := helpers.HexDecode32(s)
	return out
}

func random32() [32]byte {
	var b [32]byte
	subtle.RandomBytes(b[:])
	return b
}

// mw is a JSON array message walker for parsing client messages.
type mw struct {
	s string
	i int
}

func newMW(s string) mw {
	w := mw{s: s}
	for w.i < len(s) && s[w.i] != '[' {
		w.i++
	}
	if w.i < len(s) {
		w.i++
	}
	return w
}

func (w *mw) sep() {
	for w.i < len(w.s) {
		c := w.s[w.i]
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' && c != ',' {
			break
		}
		w.i++
	}
}

func (w *mw) str() string {
	w.sep()
	if w.i >= len(w.s) || w.s[w.i] != '"' {
		return ""
	}
	w.i++
	start := w.i
	for w.i < len(w.s) && w.s[w.i] != '"' {
		if w.s[w.i] == '\\' {
			w.i++
		}
		w.i++
	}
	if w.i >= len(w.s) {
		return ""
	}
	result := w.s[start:w.i]
	w.i++
	return result
}

func (w *mw) num() int64 {
	w.sep()
	neg := false
	if w.i < len(w.s) && w.s[w.i] == '-' {
		neg = true
		w.i++
	}
	var n int64
	for w.i < len(w.s) && w.s[w.i] >= '0' && w.s[w.i] <= '9' {
		n = n*10 + int64(w.s[w.i]-'0')
		w.i++
	}
	if neg {
		n = -n
	}
	return n
}

func (w *mw) raw() string {
	w.sep()
	start := w.i
	w.i = skipval(w.s, w.i)
	if w.i < 0 {
		w.i = len(w.s)
		return ""
	}
	return w.s[start:w.i]
}

func (w *mw) strs() []string {
	w.sep()
	if w.i >= len(w.s) || w.s[w.i] != '[' {
		return nil
	}
	w.i++
	var out []string
	for {
		w.sep()
		if w.i >= len(w.s) {
			return out
		}
		if w.s[w.i] == ']' {
			w.i++
			return out
		}
		if w.s[w.i] != '"' {
			return out
		}
		w.i++
		start := w.i
		for w.i < len(w.s) && w.s[w.i] != '"' {
			if w.s[w.i] == '\\' {
				w.i++
			}
			w.i++
		}
		if w.i < len(w.s) {
			out = append(out, w.s[start:w.i])
			w.i++
		}
	}
}

func skipval(s string, i int) int {
	if i >= len(s) {
		return -1
	}
	switch s[i] {
	case '"':
		i++
		for i < len(s) {
			if s[i] == '\\' {
				i += 2
				continue
			}
			if s[i] == '"' {
				return i + 1
			}
			i++
		}
		return -1
	case '{':
		return skipBrack(s, i, '{', '}')
	case '[':
		return skipBrack(s, i, '[', ']')
	case 't':
		if i+4 <= len(s) {
			return i + 4
		}
		return -1
	case 'f':
		if i+5 <= len(s) {
			return i + 5
		}
		return -1
	case 'n':
		if i+4 <= len(s) {
			return i + 4
		}
		return -1
	default:
		for i < len(s) && s[i] != ',' && s[i] != '}' && s[i] != ']' && s[i] != ' ' && s[i] != '\n' {
			i++
		}
		return i
	}
}

func skipBrack(s string, i int, open, close byte) int {
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
			switch s[i] {
			case '"':
				inStr = true
			case open:
				depth++
			case close:
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
