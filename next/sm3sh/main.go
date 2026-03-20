package main

import (
	"common/helpers"
	"common/jsbridge/crypto"
	"common/jsbridge/dom"
	"common/jsbridge/localstorage"
	"common/nostr"
	"common/relay"
)

const (
	lsKeyPubkey = "sm3sh-pubkey"
	lsKeyMode   = "sm3sh-mode"
)

var (
	seckey []byte // volatile — in memory only
	pubkey []byte
	pubhex string

	feedContainer dom.Element
	statusEl      dom.Element
	eventCount    int
	appContainer  dom.Element
)

func main() {
	// Check for stored session.
	stored := localstorage.GetItem(lsKeyPubkey)
	if stored != "" {
		pubhex = stored
		pubkey = helpers.HexDecode(stored)
		showApp()
	} else {
		showLogin()
	}
}

// --- Login screen ---

func showLogin() {
	body := dom.Body()
	clearChildren(body)

	wrap := dom.CreateElement("div")
	dom.SetStyle(wrap, "display", "flex")
	dom.SetStyle(wrap, "alignItems", "center")
	dom.SetStyle(wrap, "justifyContent", "center")
	dom.SetStyle(wrap, "height", "100vh")
	dom.SetStyle(wrap, "flexDirection", "column")

	// Title.
	h1 := dom.CreateElement("h1")
	dom.SetTextContent(h1, "sm3sh")
	dom.SetStyle(h1, "color", "var(--accent)")
	dom.SetStyle(h1, "fontSize", "48px")
	dom.SetStyle(h1, "marginBottom", "4px")
	dom.AppendChild(wrap, h1)

	sub := dom.CreateElement("p")
	dom.SetTextContent(sub, "nostr client — tinygo \u2192 javascript")
	dom.SetStyle(sub, "color", "var(--muted)")
	dom.SetStyle(sub, "marginBottom", "32px")
	dom.AppendChild(wrap, sub)

	// Nsec input.
	input := dom.CreateElement("input")
	dom.SetAttribute(input, "type", "password")
	dom.SetAttribute(input, "placeholder", "nsec1...")
	dom.SetAttribute(input, "autocomplete", "off")
	dom.SetAttribute(input, "spellcheck", "false")
	dom.SetStyle(input, "width", "420px")
	dom.SetStyle(input, "maxWidth", "90vw")
	dom.SetStyle(input, "padding", "12px")
	dom.SetStyle(input, "fontFamily", "monospace")
	dom.SetStyle(input, "fontSize", "14px")
	dom.SetStyle(input, "border", "1px solid var(--muted)")
	dom.SetStyle(input, "borderRadius", "4px")
	dom.SetStyle(input, "background", "var(--bg)")
	dom.SetStyle(input, "color", "var(--fg)")
	dom.SetStyle(input, "outline", "none")
	dom.SetStyle(input, "marginBottom", "12px")
	dom.AppendChild(wrap, input)

	// Error message (hidden).
	errEl := dom.CreateElement("div")
	dom.SetStyle(errEl, "color", "#e55")
	dom.SetStyle(errEl, "fontSize", "13px")
	dom.SetStyle(errEl, "marginBottom", "12px")
	dom.SetStyle(errEl, "minHeight", "18px")
	dom.AppendChild(wrap, errEl)

	// Login button.
	btn := dom.CreateElement("button")
	dom.SetTextContent(btn, "login with nsec")
	dom.SetStyle(btn, "padding", "10px 32px")
	dom.SetStyle(btn, "fontFamily", "monospace")
	dom.SetStyle(btn, "fontSize", "14px")
	dom.SetStyle(btn, "background", "var(--accent)")
	dom.SetStyle(btn, "color", "#000")
	dom.SetStyle(btn, "border", "none")
	dom.SetStyle(btn, "borderRadius", "4px")
	dom.SetStyle(btn, "cursor", "pointer")
	dom.AppendChild(wrap, btn)

	cb := dom.RegisterCallback(func() {
		nsecStr := dom.GetProperty(input, "value")
		if nsecStr == "" {
			dom.SetTextContent(errEl, "enter your nsec")
			return
		}
		sk := helpers.DecodeNsec(nsecStr)
		if sk == nil {
			dom.SetTextContent(errEl, "invalid nsec")
			return
		}
		pk := crypto.PubKeyFromSecKey(sk)
		if pk == nil {
			dom.SetTextContent(errEl, "invalid key")
			return
		}

		// Store in memory.
		seckey = sk
		pubkey = pk
		pubhex = helpers.HexEncode(pk)

		// Persist pubkey only.
		localstorage.SetItem(lsKeyPubkey, pubhex)
		localstorage.SetItem(lsKeyMode, "nsec")

		// Zero the input.
		dom.SetProperty(input, "value", "")

		clearChildren(body)
		showApp()
	})
	dom.AddEventListener(btn, "click", cb)

	dom.AppendChild(body, wrap)
}

// --- Main app ---

func showApp() {
	body := dom.Body()

	// Top bar.
	bar := dom.CreateElement("div")
	dom.SetStyle(bar, "display", "flex")
	dom.SetStyle(bar, "justifyContent", "space-between")
	dom.SetStyle(bar, "alignItems", "center")
	dom.SetStyle(bar, "padding", "8px 16px")
	dom.SetStyle(bar, "borderBottom", "1px solid var(--muted)")

	title := dom.CreateElement("span")
	dom.SetTextContent(title, "sm3sh")
	dom.SetStyle(title, "color", "var(--accent)")
	dom.SetStyle(title, "fontWeight", "bold")
	dom.AppendChild(bar, title)

	// Show npub (truncated).
	npubStr := helpers.EncodeNpub(pubkey)
	id := dom.CreateElement("span")
	dom.SetStyle(id, "color", "var(--muted)")
	dom.SetStyle(id, "fontSize", "12px")
	if len(npubStr) > 20 {
		dom.SetTextContent(id, npubStr[:12]+"..."+npubStr[len(npubStr)-4:])
	}
	dom.AppendChild(bar, id)

	// Logout button.
	logout := dom.CreateElement("button")
	dom.SetTextContent(logout, "logout")
	dom.SetStyle(logout, "fontFamily", "monospace")
	dom.SetStyle(logout, "fontSize", "12px")
	dom.SetStyle(logout, "background", "none")
	dom.SetStyle(logout, "border", "1px solid var(--muted)")
	dom.SetStyle(logout, "color", "var(--fg)")
	dom.SetStyle(logout, "borderRadius", "4px")
	dom.SetStyle(logout, "padding", "4px 12px")
	dom.SetStyle(logout, "cursor", "pointer")
	logoutCb := dom.RegisterCallback(func() {
		doLogout()
	})
	dom.AddEventListener(logout, "click", logoutCb)
	dom.AppendChild(bar, logout)

	dom.AppendChild(body, bar)

	// App container.
	appContainer = dom.CreateElement("div")
	dom.SetStyle(appContainer, "padding", "16px")
	dom.AppendChild(body, appContainer)

	// Status line.
	statusEl = dom.CreateElement("div")
	dom.SetStyle(statusEl, "color", "var(--muted)")
	dom.SetStyle(statusEl, "fontSize", "13px")
	dom.SetStyle(statusEl, "marginBottom", "12px")
	dom.SetTextContent(statusEl, "connecting...")
	dom.AppendChild(appContainer, statusEl)

	// Feed.
	feedContainer = dom.CreateElement("div")
	dom.AppendChild(appContainer, feedContainer)

	go connectRelay()
}

func doLogout() {
	// Zero key material.
	for i := range seckey {
		seckey[i] = 0
	}
	seckey = nil
	pubkey = nil
	pubhex = ""

	localstorage.RemoveItem(lsKeyPubkey)
	localstorage.RemoveItem(lsKeyMode)

	body := dom.Body()
	clearChildren(body)
	showLogin()
}

// --- Relay + feed ---

func connectRelay() {
	url := "wss://relay.damus.io"
	conn := relay.Dial(url)

	if !conn.WaitOpen() {
		dom.SetTextContent(statusEl, "failed: "+url)
		dom.SetStyle(statusEl, "color", "#e55")
		return
	}

	dom.SetTextContent(statusEl, "connected: "+url)

	filter := &nostr.Filter{
		Kinds: []int{1},
		Limit: 20,
	}
	sub := conn.Subscribe("feed", []*nostr.Filter{filter})
	sub.OnEvent = func(ev *nostr.Event) {
		eventCount++
		renderNote(ev)
	}
	sub.OnEOSE = func() {
		dom.SetTextContent(statusEl, url+" | "+itoa(eventCount)+" events")
	}
}

func renderNote(ev *nostr.Event) {
	note := dom.CreateElement("div")
	dom.SetStyle(note, "borderBottom", "1px solid var(--muted)")
	dom.SetStyle(note, "padding", "12px 0")

	// Author.
	author := dom.CreateElement("div")
	dom.SetStyle(author, "fontSize", "12px")
	dom.SetStyle(author, "color", "var(--muted)")
	dom.SetStyle(author, "marginBottom", "4px")
	npub := helpers.EncodeNpub(helpers.HexDecode(ev.PubKey))
	if len(npub) > 20 {
		dom.SetTextContent(author, npub[:12]+"..."+npub[len(npub)-4:])
	} else {
		dom.SetTextContent(author, helpers.PubkeyShort(ev.PubKey))
	}
	dom.AppendChild(note, author)

	// Content.
	content := dom.CreateElement("div")
	dom.SetStyle(content, "fontSize", "14px")
	dom.SetStyle(content, "lineHeight", "1.5")
	dom.SetStyle(content, "wordBreak", "break-word")
	text := ev.Content
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	dom.SetTextContent(content, text)
	dom.AppendChild(note, content)

	// Prepend (newest first).
	first := dom.FirstChild(feedContainer)
	if first != 0 {
		dom.InsertBefore(feedContainer, note, first)
	} else {
		dom.AppendChild(feedContainer, note)
	}
}

// --- Helpers ---

func clearChildren(el dom.Element) {
	dom.SetInnerHTML(el, "")
}

func itoa(n int) string {
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
