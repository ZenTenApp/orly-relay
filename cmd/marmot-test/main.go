package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"git.smesh.lol/orly/pkg/nostr/encoders/hex"
	"git.smesh.lol/orly/pkg/nostr/interfaces/signer/p8k"
)

type mReq struct {
	Method    string   `json:"method"`
	Nsec      string   `json:"nsec,omitempty"`
	Pubkey    string   `json:"pubkey,omitempty"`
	Sig       string   `json:"sig,omitempty"`
	Recipient string   `json:"recipient,omitempty"`
	Content   string   `json:"content,omitempty"`
	Relays    []string `json:"relays,omitempty"`
	Alias     string   `json:"alias,omitempty"`
}

type mResp struct {
	Method     string   `json:"method"`
	OK         bool     `json:"ok,omitempty"`
	Error      string   `json:"error,omitempty"`
	Peer       string   `json:"peer,omitempty"`
	Content    string   `json:"content,omitempty"`
	Ts         int64    `json:"ts,omitempty"`
	Groups     []string `json:"groups,omitempty"`
	Pubkey     string   `json:"pubkey,omitempty"`
	Subscribed bool     `json:"subscribed,omitempty"`
	Relay      string   `json:"relay,omitempty"`
	NumGroups  int      `json:"num_groups,omitempty"`
}

func main() {
	base := "ws://127.0.0.1:8090"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}

	alice, _ := p8k.New()
	alice.Generate()
	bob, _ := p8k.New()
	bob.Generate()

	aliceSec := hex.Enc(alice.Sec())
	bobSec := hex.Enc(bob.Sec())
	alicePub := hex.Enc(alice.Pub())
	bobPub := hex.Enc(bob.Pub())

	fmt.Printf("alice: %s\nbob:   %s\n\n", alicePub, bobPub)

	// --- Phase 1: Basic DM flow ---

	ac := dial(base)
	defer ac.Close()
	step("alice auth", func() {
		send(ac, mReq{Method: "auth", Nsec: aliceSec})
		r := expect(ac, "auth", 15)
		check(r.OK, "auth", r.Error)
	})

	bc := dial(base)
	defer bc.Close()
	step("bob auth", func() {
		send(bc, mReq{Method: "auth", Nsec: bobSec})
		r := expect(bc, "auth", 15)
		check(r.OK, "auth", r.Error)
	})

	step("alice publish_kp", func() {
		send(ac, mReq{Method: "publish_kp"})
		r := expect(ac, "publish_kp", 15)
		check(r.OK, "publish_kp", r.Error)
	})
	step("bob publish_kp", func() {
		send(bc, mReq{Method: "publish_kp"})
		r := expect(bc, "publish_kp", 15)
		check(r.OK, "publish_kp", r.Error)
	})

	step("alice subscribe", func() {
		send(ac, mReq{Method: "subscribe"})
		r := expect(ac, "subscribe", 10)
		check(r.OK, "subscribe", r.Error)
	})
	step("bob subscribe", func() {
		send(bc, mReq{Method: "subscribe"})
		r := expect(bc, "subscribe", 10)
		check(r.OK, "subscribe", r.Error)
	})

	fmt.Println("  waiting 3s for KP propagation...")
	time.Sleep(3 * time.Second)

	msg1 := "hello-" + time.Now().Format("150405")
	step("alice send_dm → bob", func() {
		send(ac, mReq{Method: "send_dm", Recipient: bobPub, Content: msg1})
		r := expect(ac, "send_dm", 30)
		check(r.OK, "send_dm", r.Error)
	})

	step("bob receive dm", func() {
		r := expectDM(bc, 30)
		if r.Content != msg1 {
			fail("content mismatch: got %q want %q", r.Content, msg1)
		}
		fmt.Printf("content=%q ", r.Content)
	})

	// --- Phase 2: Status ---

	step("alice status", func() {
		send(ac, mReq{Method: "status"})
		r := expect(ac, "status", 5)
		check(r.OK, "status", r.Error)
		if r.Pubkey != alicePub {
			fail("wrong pubkey: %s", r.Pubkey)
		}
		if !r.Subscribed {
			fail("not subscribed")
		}
		if r.NumGroups < 1 {
			fail("no groups")
		}
		fmt.Printf("sub=%v groups=%d relay=%s ", r.Subscribed, r.NumGroups, r.Relay)
	})

	step("bob status", func() {
		send(bc, mReq{Method: "status"})
		r := expect(bc, "status", 5)
		check(r.OK, "status", r.Error)
		if !r.Subscribed {
			fail("not subscribed")
		}
		fmt.Printf("sub=%v groups=%d ", r.Subscribed, r.NumGroups)
	})

	// --- Phase 3: Reset both + re-establish ---
	// Both sides reset → clean slate → re-establish group from scratch.

	step("alice reset", func() {
		send(ac, mReq{Method: "reset"})
		r := expect(ac, "reset", 15)
		check(r.OK, "reset", r.Error)
	})
	step("bob reset", func() {
		send(bc, mReq{Method: "reset"})
		r := expect(bc, "reset", 15)
		check(r.OK, "reset", r.Error)
	})

	step("bob status after reset", func() {
		send(bc, mReq{Method: "status"})
		r := expect(bc, "status", 5)
		check(r.OK, "status", r.Error)
		if r.Subscribed {
			fail("should not be subscribed after reset")
		}
		if r.NumGroups != 0 {
			fail("groups should be 0 after reset, got %d", r.NumGroups)
		}
		fmt.Printf("sub=%v groups=%d ", r.Subscribed, r.NumGroups)
	})

	step("alice re-publish_kp", func() {
		send(ac, mReq{Method: "publish_kp"})
		r := expect(ac, "publish_kp", 15)
		check(r.OK, "publish_kp", r.Error)
	})
	step("bob re-publish_kp", func() {
		send(bc, mReq{Method: "publish_kp"})
		r := expect(bc, "publish_kp", 15)
		check(r.OK, "publish_kp", r.Error)
	})

	step("alice re-subscribe", func() {
		send(ac, mReq{Method: "subscribe"})
		r := expect(ac, "subscribe", 10)
		check(r.OK, "subscribe", r.Error)
	})
	step("bob re-subscribe", func() {
		send(bc, mReq{Method: "subscribe"})
		r := expect(bc, "subscribe", 10)
		check(r.OK, "subscribe", r.Error)
	})

	time.Sleep(2 * time.Second)
	msg2 := "after-reset-" + time.Now().Format("150405")
	step("alice send_dm after reset", func() {
		send(ac, mReq{Method: "send_dm", Recipient: bobPub, Content: msg2})
		r := expect(ac, "send_dm", 30)
		check(r.OK, "send_dm", r.Error)
	})

	step("bob receive after reset", func() {
		r := expectDM(bc, 30)
		if r.Content != msg2 {
			fail("content mismatch: got %q want %q", r.Content, msg2)
		}
		fmt.Printf("content=%q ", r.Content)
	})

	// --- Phase 4: Alias resolution ---

	step("resolve_alias jb55@damus.io", func() {
		send(ac, mReq{Method: "resolve_alias", Alias: "jb55@damus.io"})
		r := expect(ac, "resolve_alias", 15)
		check(r.OK, "resolve_alias", r.Error)
		if len(r.Pubkey) != 64 {
			fail("invalid pubkey: %s", r.Pubkey)
		}
		// jb55's known pubkey
		if r.Pubkey != "32e1827635450ebb3c5a7d12c1f8e7b2b514439ac10a67eef3d9fd9c5c68e245" {
			fail("wrong pubkey: %s", r.Pubkey)
		}
		fmt.Printf("pubkey=%s..%s ", r.Pubkey[:8], r.Pubkey[56:])
	})

	step("list_groups final", func() {
		send(ac, mReq{Method: "list_groups"})
		r := expect(ac, "list_groups", 5)
		check(r.OK, "list_groups", r.Error)
		fmt.Printf("groups=%d ", len(r.Groups))
	})

	fmt.Println("\nALL PASS")
}

func dial(base string) *websocket.Conn {
	u, _ := url.Parse(base + "/__marmot")
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fail("dial %s: %v", u, err)
	}
	return c
}

func send(c *websocket.Conn, req mReq) {
	data, _ := json.Marshal(req)
	c.WriteMessage(websocket.TextMessage, data)
}

func expect(c *websocket.Conn, method string, timeoutSec int) mResp {
	c.SetReadDeadline(time.Now().Add(time.Duration(timeoutSec) * time.Second))
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			fail("expect %s: %v", method, err)
		}
		var r mResp
		json.Unmarshal(raw, &r)
		if r.Method == method {
			return r
		}
	}
}

func expectDM(c *websocket.Conn, timeoutSec int) mResp {
	c.SetReadDeadline(time.Now().Add(time.Duration(timeoutSec) * time.Second))
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			fail("expect dm_received: %v", err)
		}
		var r mResp
		json.Unmarshal(raw, &r)
		if r.Method == "dm_received" {
			return r
		}
	}
}

func step(name string, fn func()) {
	fmt.Printf("  %s... ", name)
	fn()
	fmt.Println("OK")
}

func check(ok bool, what, errMsg string) {
	if !ok {
		fail("%s: %s", what, errMsg)
	}
}

func fail(format string, args ...any) {
	fmt.Printf("FAIL: "+format+"\n", args...)
	os.Exit(1)
}
