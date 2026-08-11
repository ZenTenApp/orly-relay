// Command nip40test is a minimal websocket client used by scripts/test-nip40.sh
// to exercise NIP-40 expiration on a locally-running relay. It uses the repo's
// own p8k signer and the vendored gorilla/websocket client, so it builds with
// no external dependencies.
//
// Usage:
//
//	nip40test genkey <keyfile>
//	nip40test publish <ws-url> <keyfile> <expiry-unix> <content>
//	    Signs/publishes a kind-1 note tagged ["expiration", "<unix>"], then
//	    prints:  <event-id-hex> <pubkey-hex> <accepted:bool>
//	nip40test query <ws-url> <keyfile> <event-id-hex>
//	    REQ-by-author, prints FOUND or NOT_FOUND for the given event id.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/nostr/encoders/event"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/encoders/tag"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
	"github.com/gorilla/websocket"
)

func fail(format string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf("nip40test: "+format, args...))
	os.Exit(1)
}

func loadOrCreateSigner(keyfile string) *p8k.Signer {
	s := p8k.MustNew()
	if data, err := os.ReadFile(keyfile); err == nil && len(data) > 0 {
		sec, derr := hex.Dec(strings.TrimSpace(string(data)))
		if derr != nil {
			fail("invalid secret in %s: %v", keyfile, derr)
		}
		if err := s.InitSec(sec); chk.E(err) {
			fail("InitSec: %v", err)
		}
		return s
	}
	if err := s.Generate(); chk.E(err) {
		fail("Generate: %v", err)
	}
	if err := os.WriteFile(keyfile, []byte(hex.Enc(s.Sec())), 0o600); err != nil {
		fail("write keyfile: %v", err)
	}
	return s
}

func connect(wsURL string) *websocket.Conn {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fail("dial %s: %v", wsURL, err)
	}
	return conn
}

func readWait(conn *websocket.Conn, timeout time.Duration) ([]byte, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func cmdPublish(wsURL, keyfile, expiryStr, content string) {
	if _, err := strconv.ParseInt(expiryStr, 10, 64); err != nil {
		fail("bad expiry: %v", err)
	}
	s := loadOrCreateSigner(keyfile)

	ev := event.New()
	ev.Kind = 1
	ev.CreatedAt = time.Now().Unix()
	ev.Content = []byte(content)
	ev.Pubkey = s.Pub()
	ev.Tags = tag.NewS(tag.NewFromAny("expiration", expiryStr))
	if err := ev.Sign(s); err != nil {
		fail("sign: %v", err)
	}

	conn := connect(wsURL)
	defer conn.Close()

	wire := fmt.Sprintf("[\"EVENT\",%s]", ev.Serialize())
	if err := conn.WriteMessage(websocket.TextMessage, []byte(wire)); err != nil {
		fail("write: %v", err)
	}

	idHex := hex.Enc(ev.ID[:])
	for {
		msg, err := readWait(conn, 10*time.Second)
		if err != nil {
			fail("no OK for event %s: %v", idHex, err)
		}
		var raw []any
		if json.Unmarshal(msg, &raw) != nil {
			continue
		}
		if len(raw) >= 4 && raw[0] == "OK" && raw[1] == idHex {
			fmt.Printf("%s %s %v\n", idHex, hex.Enc(s.Pub()), raw[2])
			return
		}
	}
}

func cmdQuery(wsURL, keyfile, wantID string) {
	s := p8k.MustNew()
	data, err := os.ReadFile(keyfile)
	if err != nil {
		fail("read keyfile %s: %v", keyfile, err)
	}
	sec, derr := hex.Dec(strings.TrimSpace(string(data)))
	if derr != nil {
		fail("invalid secret: %v", derr)
	}
	if err := s.InitSec(sec); chk.E(err) {
		fail("InitSec: %v", err)
	}
	pubHex := hex.Enc(s.Pub())

	conn := connect(wsURL)
	defer conn.Close()

	req := fmt.Sprintf("[\"REQ\",\"n\",{\"authors\":[\"%s\"]}]", pubHex)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(req)); err != nil {
		fail("write REQ: %v", err)
	}

	found := false
	for {
		msg, err := readWait(conn, 10*time.Second)
		if err != nil {
			break
		}
		var raw []any
		if json.Unmarshal(msg, &raw) != nil {
			continue
		}
		if len(raw) == 0 {
			continue
		}
		switch raw[0] {
		case "EVENT":
			if len(raw) >= 3 {
				if m, ok := raw[2].(map[string]any); ok {
					if id, ok := m["id"].(string); ok && id == wantID {
						found = true
					}
				}
			}
		case "EOSE":
			if found {
				fmt.Println("FOUND")
			} else {
				fmt.Println("NOT_FOUND")
			}
			return
		}
	}
	fail("subscription ended without EOSE")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nip40test <genkey|publish|query> [args...]")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "genkey":
		if len(os.Args) != 3 {
			fail("genkey <keyfile>")
		}
		s := p8k.MustNew()
		if err := s.Generate(); chk.E(err) {
			fail("generate: %v", err)
		}
		if err := os.WriteFile(os.Args[2], []byte(hex.Enc(s.Sec())), 0o600); err != nil {
			fail("write: %v", err)
		}
		fmt.Println("wrote", os.Args[2])
	case "publish":
		if len(os.Args) != 6 {
			fail("publish <ws-url> <keyfile> <expiry-unix> <content>")
		}
		cmdPublish(os.Args[2], os.Args[3], os.Args[4], os.Args[5])
	case "query":
		if len(os.Args) != 5 {
			fail("query <ws-url> <keyfile> <event-id-hex>")
		}
		cmdQuery(os.Args[2], os.Args[3], os.Args[4])
	default:
		fail("unknown command %q", os.Args[1])
	}
}
