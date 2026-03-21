package main

import (
	"common/helpers"
	"common/jsbridge/dom"
	"common/jsbridge/localstorage"
	"common/jsbridge/signer"
	"common/nostr"
	"common/relay"
)

const (
	lsKeyPubkey = "sm3sh-pubkey"
	lsKeyMode   = "sm3sh-mode"
	lsKeyTheme  = "sm3sh-theme"
)

var (
	pubkey []byte
	pubhex string
	isDark bool

	// Profile data from kind 0.
	profileName string
	profilePic  string
	profileTs   int64

	// DOM refs that need updating after creation.
	avatarEl      dom.Element
	nameEl        dom.Element
	appContainer  dom.Element
	feedContainer dom.Element
	statusEl      dom.Element
	popoverEl     dom.Element
	themeBtn      dom.Element

	// App root — content goes here, not body (snackbar stays outside).
	root dom.Element

	// Relay tracking.
	eventCount     int
	connectedCount int
	relayDots      []dom.Element
	popoverOpen    bool
)

var relays = []string{
	"wss://relay.orly.dev",
	"wss://nostr.wine",
	"wss://nostr.land",
}

func main() {
	isDark = localstorage.GetItem(lsKeyTheme) == "dark"
	if isDark {
		dom.AddClass(dom.Body(), "dark")
	}
	root = dom.GetElementById("app-root")
	stored := localstorage.GetItem(lsKeyPubkey)
	if stored != "" {
		pubhex = stored
		pubkey = helpers.HexDecode(stored)
		showApp()
	} else {
		showLogin()
	}
}

// --- Theme ---

func toggleTheme() {
	body := dom.Body()
	isDark = !isDark
	if isDark {
		dom.AddClass(body, "dark")
		localstorage.SetItem(lsKeyTheme, "dark")
	} else {
		dom.RemoveClass(body, "dark")
		localstorage.SetItem(lsKeyTheme, "light")
	}
	updateThemeIcon()
}

func updateThemeIcon() {
	if isDark {
		dom.SetInnerHTML(themeBtn, "&#x2600;&#xFE0F;") // ☀️ emoji sun
	} else {
		dom.SetInnerHTML(themeBtn, "&#x1F319;") // 🌙
	}
}

// --- Login screen ---

func showLogin() {
	clearChildren(root)

	wrap := dom.CreateElement("div")
	dom.SetStyle(wrap, "display", "flex")
	dom.SetStyle(wrap, "alignItems", "center")
	dom.SetStyle(wrap, "justifyContent", "center")
	dom.SetStyle(wrap, "height", "100vh")
	dom.SetStyle(wrap, "flexDirection", "column")

	// Smesh loader animation.
	loader := dom.CreateElement("div")
	dom.SetStyle(loader, "width", "180px")
	dom.SetStyle(loader, "height", "180px")
	dom.SetStyle(loader, "marginBottom", "16px")
	dom.FetchText("./smesh-loader.svg", func(svg string) {
		dom.SetInnerHTML(loader, svg)
	})
	dom.AppendChild(wrap, loader)

	// Title.
	h1 := dom.CreateElement("h1")
	dom.SetTextContent(h1, "sm3sh")
	dom.SetStyle(h1, "color", "var(--accent)")
	dom.SetStyle(h1, "fontSize", "48px")
	dom.SetStyle(h1, "marginBottom", "4px")
	dom.AppendChild(wrap, h1)

	sub := dom.CreateElement("p")
	dom.SetTextContent(sub, "nostr client \u2014 tinygo \u2192 javascript")
	dom.SetStyle(sub, "color", "var(--muted)")
	dom.SetStyle(sub, "marginBottom", "32px")
	dom.AppendChild(wrap, sub)

	// Error message.
	errEl := dom.CreateElement("div")
	dom.SetStyle(errEl, "color", "#e55")
	dom.SetStyle(errEl, "fontSize", "13px")
	dom.SetStyle(errEl, "marginBottom", "12px")
	dom.SetStyle(errEl, "minHeight", "18px")
	dom.AppendChild(wrap, errEl)

	// Login button.
	btn := dom.CreateElement("button")
	dom.SetTextContent(btn, "login with extension")
	dom.SetAttribute(btn, "type", "button")
	dom.SetStyle(btn, "padding", "10px 32px")
	dom.SetStyle(btn, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(btn, "fontSize", "14px")
	dom.SetStyle(btn, "background", "var(--accent)")
	dom.SetStyle(btn, "color", "#000")
	dom.SetStyle(btn, "border", "none")
	dom.SetStyle(btn, "borderRadius", "4px")
	dom.SetStyle(btn, "cursor", "pointer")
	dom.AppendChild(wrap, btn)

	dom.AppendChild(root, wrap)

	cb := dom.RegisterCallback(func() {
		if !signer.HasSigner() {
			dom.SetTextContent(errEl, "install a NIP-07 extension (nos2x, Alby, etc)")
			return
		}
		dom.SetTextContent(btn, "requesting...")
		signer.GetPublicKey(func(hex string) {
			if hex == "" {
				dom.SetTextContent(errEl, "login failed or was denied")
				dom.SetTextContent(btn, "login with extension")
				return
			}
			pubhex = hex
			pubkey = helpers.HexDecode(hex)
			localstorage.SetItem(lsKeyPubkey, pubhex)
			localstorage.SetItem(lsKeyMode, "extension")
			clearChildren(root)
			showApp()
		})
	})
	dom.AddEventListener(btn, "click", cb)

}

// --- Main app ---

func showApp() {

	// === Top bar ===
	bar := dom.CreateElement("div")
	dom.SetStyle(bar, "display", "flex")
	dom.SetStyle(bar, "alignItems", "center")
	dom.SetStyle(bar, "padding", "8px 16px")
	dom.SetStyle(bar, "borderBottom", "2px solid var(--muted)")
	dom.SetStyle(bar, "background", "var(--bg)")
	dom.SetStyle(bar, "position", "sticky")
	dom.SetStyle(bar, "top", "0")
	dom.SetStyle(bar, "zIndex", "100")

	// Left: avatar + name.
	left := dom.CreateElement("div")
	dom.SetStyle(left, "display", "flex")
	dom.SetStyle(left, "alignItems", "center")
	dom.SetStyle(left, "gap", "8px")
	dom.SetStyle(left, "flex", "1")
	dom.SetStyle(left, "minWidth", "0")

	avatarEl = dom.CreateElement("img")
	dom.SetAttribute(avatarEl, "width", "28")
	dom.SetAttribute(avatarEl, "height", "28")
	dom.SetStyle(avatarEl, "borderRadius", "50%")
	dom.SetStyle(avatarEl, "display", "none")
	dom.SetAttribute(avatarEl, "onerror", "this.style.display='none'")
	dom.AppendChild(left, avatarEl)

	nameEl = dom.CreateElement("span")
	dom.SetStyle(nameEl, "fontSize", "13px")
	dom.SetStyle(nameEl, "overflow", "hidden")
	dom.SetStyle(nameEl, "textOverflow", "ellipsis")
	dom.SetStyle(nameEl, "whiteSpace", "nowrap")
	npubStr := helpers.EncodeNpub(pubkey)
	if len(npubStr) > 20 {
		dom.SetTextContent(nameEl, npubStr[:12]+"..."+npubStr[len(npubStr)-4:])
	}
	dom.AppendChild(left, nameEl)
	dom.AppendChild(bar, left)

	// Center: dendrite logo.
	logo := dom.CreateElement("div")
	dom.SetStyle(logo, "width", "32px")
	dom.SetStyle(logo, "height", "32px")
	dom.SetStyle(logo, "flexShrink", "0")
	dom.FetchText("./smesh-loader.svg", func(svg string) {
		dom.SetInnerHTML(logo, svg)
		svgEl := dom.FirstChild(logo)
		if svgEl != 0 {
			dom.SetAttribute(svgEl, "width", "100%")
			dom.SetAttribute(svgEl, "height", "100%")
		}
	})
	dom.AppendChild(bar, logo)

	// Right: theme toggle + logout.
	right := dom.CreateElement("div")
	dom.SetStyle(right, "display", "flex")
	dom.SetStyle(right, "alignItems", "center")
	dom.SetStyle(right, "gap", "8px")
	dom.SetStyle(right, "flex", "1")
	dom.SetStyle(right, "justifyContent", "flex-end")

	themeBtn = dom.CreateElement("button")
	dom.SetStyle(themeBtn, "background", "color-mix(in srgb, var(--fg) 40%, transparent)")
	dom.SetStyle(themeBtn, "border", "none")
	dom.SetStyle(themeBtn, "borderRadius", "50%")
	dom.SetStyle(themeBtn, "width", "32px")
	dom.SetStyle(themeBtn, "height", "32px")
	dom.SetStyle(themeBtn, "fontSize", "16px")
	dom.SetStyle(themeBtn, "cursor", "pointer")
	dom.SetStyle(themeBtn, "padding", "0")
	dom.SetStyle(themeBtn, "display", "flex")
	dom.SetStyle(themeBtn, "alignItems", "center")
	dom.SetStyle(themeBtn, "justifyContent", "center")
	dom.SetStyle(themeBtn, "lineHeight", "1")
	updateThemeIcon()
	dom.AddEventListener(themeBtn, "click", dom.RegisterCallback(func() {
		toggleTheme()
	}))
	dom.AppendChild(right, themeBtn)

	logout := dom.CreateElement("button")
	dom.SetTextContent(logout, "logout")
	dom.SetStyle(logout, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(logout, "fontSize", "12px")
	dom.SetStyle(logout, "background", "none")
	dom.SetStyle(logout, "border", "2px solid var(--muted)")
	dom.SetStyle(logout, "color", "var(--fg)")
	dom.SetStyle(logout, "borderRadius", "4px")
	dom.SetStyle(logout, "padding", "6px 16px")
	dom.SetStyle(logout, "cursor", "pointer")
	dom.AddEventListener(logout, "click", dom.RegisterCallback(func() {
		doLogout()
	}))
	dom.AppendChild(right, logout)
	dom.AppendChild(bar, right)

	dom.AppendChild(root, bar)

	// === Main content ===
	appContainer = dom.CreateElement("div")
	dom.SetStyle(appContainer, "padding", "16px")
	dom.SetStyle(appContainer, "paddingBottom", "52px")
	feedContainer = dom.CreateElement("div")
	dom.AppendChild(appContainer, feedContainer)
	dom.AppendChild(root, appContainer)

	// === Bottom status bar ===
	bottomBar := dom.CreateElement("div")
	dom.SetStyle(bottomBar, "position", "fixed")
	dom.SetStyle(bottomBar, "bottom", "0")
	dom.SetStyle(bottomBar, "left", "0")
	dom.SetStyle(bottomBar, "right", "0")
	dom.SetStyle(bottomBar, "height", "36px")
	dom.SetStyle(bottomBar, "display", "flex")
	dom.SetStyle(bottomBar, "alignItems", "center")
	dom.SetStyle(bottomBar, "padding", "0 16px")
	dom.SetStyle(bottomBar, "borderTop", "2px solid var(--muted)")
	dom.SetStyle(bottomBar, "background", "var(--bg)")
	dom.SetStyle(bottomBar, "fontSize", "12px")
	dom.SetStyle(bottomBar, "color", "var(--fg)")
	dom.SetStyle(bottomBar, "cursor", "pointer")
	dom.SetStyle(bottomBar, "zIndex", "100")

	statusEl = dom.CreateElement("span")
	dom.SetTextContent(statusEl, "connecting...")
	dom.AppendChild(bottomBar, statusEl)

	dom.AddEventListener(bottomBar, "click", dom.RegisterCallback(func() {
		togglePopover()
	}))
	dom.AppendChild(root, bottomBar)

	// === Relay popover (hidden) ===
	popoverEl = dom.CreateElement("div")
	dom.SetStyle(popoverEl, "position", "fixed")
	dom.SetStyle(popoverEl, "bottom", "37px")
	dom.SetStyle(popoverEl, "left", "0")
	dom.SetStyle(popoverEl, "right", "0")
	dom.SetStyle(popoverEl, "background", "var(--bg2)")
	dom.SetStyle(popoverEl, "borderTop", "1px solid var(--border)")
	dom.SetStyle(popoverEl, "padding", "12px 16px")
	dom.SetStyle(popoverEl, "fontSize", "12px")
	dom.SetStyle(popoverEl, "display", "none")
	dom.SetStyle(popoverEl, "zIndex", "99")

	relayDots = make([]dom.Element, len(relays))
	for i, url := range relays {
		row := dom.CreateElement("div")
		dom.SetStyle(row, "padding", "3px 0")

		dot := dom.CreateElement("span")
		dom.SetTextContent(dot, "\u25CF") // ●
		dom.SetStyle(dot, "color", "var(--muted)")
		dom.SetStyle(dot, "marginRight", "8px")
		relayDots[i] = dot
		dom.AppendChild(row, dot)

		label := dom.CreateElement("span")
		dom.SetTextContent(label, url)
		dom.AppendChild(row, label)

		dom.AppendChild(popoverEl, row)
	}
	dom.AppendChild(root, popoverEl)

	go connectRelays()
	go fetchProfile()
}

func togglePopover() {
	popoverOpen = !popoverOpen
	if popoverOpen {
		dom.SetStyle(popoverEl, "display", "block")
	} else {
		dom.SetStyle(popoverEl, "display", "none")
	}
}

// --- Relay connections ---

func connectRelays() {
	for i, url := range relays {
		u := url
		idx := i
		conn := relay.Dial(u)
		conn.OnReady(func(ok bool) {
			if !ok {
				dom.SetStyle(relayDots[idx], "color", "#e55")
				return
			}
			connectedCount++
			dom.SetStyle(relayDots[idx], "color", "#5b5")
			updateStatus()

			// Profile data from this relay too.
			profSub := conn.Subscribe("prof", []*nostr.Filter{{
				Authors: []string{pubhex},
				Kinds:   []int{0, 10002, 10050},
				Limit:   5,
			}})
			profSub.OnEvent = func(ev *nostr.Event) {
				handleProfileEvent(ev)
			}

			// Feed.
			feedSub := conn.Subscribe("feed", []*nostr.Filter{{
				Kinds: []int{1},
				Limit: 20,
			}})
			feedSub.OnEvent = func(ev *nostr.Event) {
				eventCount++
				renderNote(ev)
			}
			feedSub.OnEOSE = func() {
				updateStatus()
			}
		})
	}
}

// fetchProfile queries purplepag.es for relay lists and profile metadata.
func fetchProfile() {
	conn := relay.Dial("wss://purplepag.es")
	conn.OnReady(func(ok bool) {
		if !ok {
			return
		}
		sub := conn.Subscribe("prof", []*nostr.Filter{{
			Authors: []string{pubhex},
			Kinds:   []int{0, 10002, 10050},
			Limit:   5,
		}})
		sub.OnEvent = func(ev *nostr.Event) {
			handleProfileEvent(ev)
		}
		sub.OnEOSE = func() {
			sub.Close()
			conn.Close()
		}
	})
}

func handleProfileEvent(ev *nostr.Event) {
	switch ev.Kind {
	case 0:
		if ev.CreatedAt <= profileTs {
			return
		}
		profileTs = ev.CreatedAt
		name := helpers.JsonGetString(ev.Content, "display_name")
		if name == "" {
			name = helpers.JsonGetString(ev.Content, "name")
		}
		pic := helpers.JsonGetString(ev.Content, "picture")
		if name != "" {
			profileName = name
			dom.SetTextContent(nameEl, name)
		}
		if pic != "" {
			profilePic = pic
			dom.SetAttribute(avatarEl, "src", pic)
			dom.SetStyle(avatarEl, "display", "block")
		}
	case 10002:
		// NIP-65 relay list — stored for future use.
		_ = ev.Tags.GetAll("r")
	case 10050:
		// DM inbox relay list — stored for future use.
		_ = ev.Tags.GetAll("relay")
	}
}

func updateStatus() {
	dom.SetTextContent(statusEl,
		itoa(connectedCount)+"/"+itoa(len(relays))+" relays | "+itoa(eventCount)+" events")
}

// --- Feed rendering ---

func renderNote(ev *nostr.Event) {
	note := dom.CreateElement("div")
	dom.SetStyle(note, "borderBottom", "1px solid var(--border)")
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

// --- Logout ---

func doLogout() {
	pubkey = nil
	pubhex = ""
	profileName = ""
	profilePic = ""
	profileTs = 0
	connectedCount = 0
	eventCount = 0
	popoverOpen = false

	localstorage.RemoveItem(lsKeyPubkey)
	localstorage.RemoveItem(lsKeyMode)

	clearChildren(root)
	showLogin()
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
