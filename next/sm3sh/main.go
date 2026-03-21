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
	version     = "v0.65.13"
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

	// Relay tracking — parallel slices, grown dynamically.
	relayURLs      []string
	relayConns     []*relay.Conn
	relayDots      []dom.Element
	relayLabels    []dom.Element
	relayConnected []bool
	relayUserPick  []bool // true = from user's kind 10002
	relayHasProxy  []bool // true = relay supports _proxy extension

	eventCount  int
	popoverOpen bool

	// Author profile cache.
	authorNames  map[string]string       // pubkey hex -> display name
	authorPics   map[string]string       // pubkey hex -> avatar URL
	authorTs     map[string]int64        // pubkey hex -> created_at of cached kind 0
	authorRelays map[string][]string     // pubkey hex -> relay URLs from kind 10002
	pendingNotes map[string][]dom.Element // pubkey hex -> author header divs awaiting profile
	fetchedK0    map[string]bool         // pubkey hex -> already tried kind 0 fetch
	fetchedK10k  map[string]bool         // pubkey hex -> already tried kind 10002 fetch
)

var defaultRelays = []string{
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
	// Init profile cache maps.
	authorNames = make(map[string]string)
	authorPics = make(map[string]string)
	authorTs = make(map[string]int64)
	authorRelays = make(map[string][]string)
	pendingNotes = make(map[string][]dom.Element)
	fetchedK0 = make(map[string]bool)
	fetchedK10k = make(map[string]bool)

	// === Top bar ===
	bar := dom.CreateElement("div")
	dom.SetStyle(bar, "display", "flex")
	dom.SetStyle(bar, "alignItems", "center")
	dom.SetStyle(bar, "padding", "8px 16px")

	dom.SetStyle(bar, "background", "var(--bg2)")
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
	dom.SetStyle(avatarEl, "objectFit", "cover")
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
	dom.SetStyle(logout, "background", "color-mix(in srgb, var(--fg) 40%, transparent)")
	dom.SetStyle(logout, "border", "none")
	dom.SetStyle(logout, "color", "var(--fg)")
	dom.SetStyle(logout, "borderRadius", "4px")
	dom.SetStyle(logout, "height", "32px")
	dom.SetStyle(logout, "padding", "0 16px")
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

	dom.SetStyle(bottomBar, "background", "var(--bg2)")
	dom.SetStyle(bottomBar, "fontSize", "12px")
	dom.SetStyle(bottomBar, "color", "var(--fg)")
	dom.SetStyle(bottomBar, "cursor", "pointer")
	dom.SetStyle(bottomBar, "zIndex", "100")

	statusEl = dom.CreateElement("span")
	dom.SetTextContent(statusEl, "connecting...")
	dom.AppendChild(bottomBar, statusEl)

	dom.AddEventListener(statusEl, "click", dom.RegisterCallback(func() {
		togglePopover()
	}))

	ver := dom.CreateElement("span")
	dom.SetTextContent(ver, "sm3sh "+version)
	dom.SetStyle(ver, "marginLeft", "auto")
	dom.SetStyle(ver, "color", "var(--accent)")
	dom.AppendChild(bottomBar, ver)

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
	dom.AppendChild(root, popoverEl)

	// Add default relays and connect.
	for _, url := range defaultRelays {
		addRelay(url, false)
	}

	go fetchProfile()
}

// addRelay adds a relay to the list, creates its popover row, and connects.
// userPick=true means it came from the user's kind 10002 relay list.
func addRelay(url string, userPick bool) {
	url = normalizeURL(url)
	// Dedup.
	for i, u := range relayURLs {
		if u == url {
			// Promote to user pick if needed.
			if userPick && !relayUserPick[i] {
				relayUserPick[i] = true
				dom.SetStyle(relayLabels[i], "fontWeight", "bold")
			}
			return
		}
	}

	idx := len(relayURLs)
	relayURLs = append(relayURLs, url)
	relayConns = append(relayConns, nil)
	relayConnected = append(relayConnected, false)
	relayUserPick = append(relayUserPick, userPick)
	relayHasProxy = append(relayHasProxy, false)

	// Popover row.
	row := dom.CreateElement("div")
	dom.SetStyle(row, "padding", "3px 0")

	dot := dom.CreateElement("span")
	dom.SetTextContent(dot, "\u25CF") // ●
	dom.SetStyle(dot, "color", "var(--muted)")
	dom.SetStyle(dot, "marginRight", "8px")
	relayDots = append(relayDots, dot)
	dom.AppendChild(row, dot)

	label := dom.CreateElement("span")
	dom.SetTextContent(label, url)
	if userPick {
		dom.SetStyle(label, "fontWeight", "bold")
	}
	relayLabels = append(relayLabels, label)
	dom.AppendChild(row, label)

	dom.AppendChild(popoverEl, row)

	// Fetch NIP-11 relay info to check for _proxy support.
	infoURL := wsToHTTP(url)
	dom.FetchRelayInfo(infoURL, func(body string) {
		if len(body) > 0 {
			pq := strIndex(body, "\"proxy_query\"")
			if pq >= 0 {
				chunk := body[pq:]
				if strIndex(chunk, "\"enabled\":true") >= 0 || strIndex(chunk, "\"enabled\": true") >= 0 {
					relayHasProxy[idx] = true
				}
			}
		}
	})

	// Connect.
	conn := relay.Dial(url)
	conn.OnReady(func(ok bool) {
		if !ok {
			dom.SetStyle(relayDots[idx], "color", "#e55")
			return
		}
		relayConns[idx] = conn
		relayConnected[idx] = true
		dom.SetStyle(relayDots[idx], "color", "#5b5")
		updateStatus()

		// Profile data.
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

func togglePopover() {
	popoverOpen = !popoverOpen
	if popoverOpen {
		dom.SetStyle(popoverEl, "display", "block")
	} else {
		dom.SetStyle(popoverEl, "display", "none")
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
		name := helpers.JsonGetString(ev.Content, "name")
		if name == "" {
			name = helpers.JsonGetString(ev.Content, "display_name")
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
		// NIP-65 relay list — connect to user's preferred relays.
		tags := ev.Tags.GetAll("r")
		if tags != nil {
			for _, tag := range tags {
				url := tag.Value()
				if url != "" {
					addRelay(url, true)
				}
			}
		}
	case 10050:
		// DM inbox relay list — stored for future use.
		_ = ev.Tags.GetAll("relay")
	}
}

func updateStatus() {
	connected := 0
	for _, c := range relayConnected {
		if c {
			connected++
		}
	}
	dom.SetTextContent(statusEl,
		itoa(connected)+"/"+itoa(len(relayURLs))+" relays | "+itoa(eventCount)+" events")
}

// --- Feed rendering ---

func renderNote(ev *nostr.Event) {
	note := dom.CreateElement("div")
	dom.SetStyle(note, "borderBottom", "1px solid var(--border)")
	dom.SetStyle(note, "padding", "12px 0")

	// Author header: avatar + name.
	header := dom.CreateElement("div")
	dom.SetStyle(header, "display", "flex")
	dom.SetStyle(header, "alignItems", "center")
	dom.SetStyle(header, "gap", "8px")
	dom.SetStyle(header, "marginBottom", "4px")

	avatar := dom.CreateElement("img")
	dom.SetAttribute(avatar, "width", "24")
	dom.SetAttribute(avatar, "height", "24")
	dom.SetStyle(avatar, "borderRadius", "50%")
	dom.SetStyle(avatar, "objectFit", "cover")
	dom.SetStyle(avatar, "flexShrink", "0")

	nameSpan := dom.CreateElement("span")
	dom.SetStyle(nameSpan, "fontSize", "12px")
	dom.SetStyle(nameSpan, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(nameSpan, "fontWeight", "bold")
	dom.SetStyle(nameSpan, "color", "var(--muted)")

	pk := ev.PubKey
	if pic, ok := authorPics[pk]; ok && pic != "" {
		dom.SetAttribute(avatar, "src", pic)
		dom.SetAttribute(avatar, "onerror", "this.style.display='none'")
	} else {
		dom.SetStyle(avatar, "display", "none")
	}
	if name, ok := authorNames[pk]; ok && name != "" {
		dom.SetTextContent(nameSpan, name)
	} else {
		npub := helpers.EncodeNpub(helpers.HexDecode(pk))
		if len(npub) > 20 {
			dom.SetTextContent(nameSpan, npub[:12]+"..."+npub[len(npub)-4:])
		}
	}

	dom.AppendChild(header, avatar)
	dom.AppendChild(header, nameSpan)
	dom.AppendChild(note, header)

	// Queue profile fetch if not cached.
	if _, cached := authorNames[pk]; !cached && !fetchedK0[pk] {
		pendingNotes[pk] = append(pendingNotes[pk], header)
		fetchAuthorProfile(pk)
	}

	// Content.
	content := dom.CreateElement("div")
	dom.SetStyle(content, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(content, "fontSize", "14px")
	dom.SetStyle(content, "lineHeight", "1.5")
	dom.SetStyle(content, "wordBreak", "break-word")
	text := ev.Content
	if len(text) > 500 {
		text = text[:500] + "..."
	}
	dom.SetInnerHTML(content, renderMarkdown(text))
	dom.AppendChild(note, content)

	// Prepend (newest first).
	first := dom.FirstChild(feedContainer)
	if first != 0 {
		dom.InsertBefore(feedContainer, note, first)
	} else {
		dom.AppendChild(feedContainer, note)
	}
}

// getProxyConn returns the first connected relay that supports _proxy, or nil.
func getProxyConn() *relay.Conn {
	for i, c := range relayConns {
		if c != nil && relayConnected[i] && relayHasProxy[i] {
			return c
		}
	}
	return nil
}

// getAnyConn returns the first connected relay, or nil.
func getAnyConn() *relay.Conn {
	for i, c := range relayConns {
		if c != nil && relayConnected[i] {
			return c
		}
	}
	return nil
}

var profileSubCounter int

// fetchAuthorProfile fetches kind 0 for an author via a proxy-capable relay.
func fetchAuthorProfile(pk string) {
	if fetchedK0[pk] {
		return
	}
	fetchedK0[pk] = true

	proxyConn := getProxyConn()
	anyConn := getAnyConn()
	if proxyConn == nil && anyConn == nil {
		fetchedK0[pk] = false // retry later
		return
	}

	profileSubCounter++
	subID := "ap-" + itoa(profileSubCounter)

	if proxyConn != nil {
		// Use _proxy to fetch from author's relays or purplepag.es.
		proxy := authorRelays[pk]
		if len(proxy) == 0 {
			proxy = []string{"wss://purplepag.es"}
		}
		sub := proxyConn.Subscribe(subID, []*nostr.Filter{{
			Authors: []string{pk},
			Kinds:   []int{0},
			Limit:   1,
			Proxy:   proxy,
		}})
		sub.OnEvent = func(ev *nostr.Event) {
			if ev.Kind == 0 {
				applyAuthorProfile(pk, ev)
			}
		}
		sub.OnEOSE = func() {
			sub.Close()
			if _, got := authorNames[pk]; !got && !fetchedK10k[pk] {
				fetchAuthorRelayList(pk)
			}
		}
	} else {
		// No proxy relay — plain fetch from connected relay.
		sub := anyConn.Subscribe(subID, []*nostr.Filter{{
			Authors: []string{pk},
			Kinds:   []int{0},
			Limit:   1,
		}})
		sub.OnEvent = func(ev *nostr.Event) {
			if ev.Kind == 0 {
				applyAuthorProfile(pk, ev)
			}
		}
		sub.OnEOSE = func() {
			sub.Close()
		}
	}
}

// fetchAuthorRelayList fetches kind 10002 for an author, then retries kind 0.
func fetchAuthorRelayList(pk string) {
	if fetchedK10k[pk] {
		return
	}
	fetchedK10k[pk] = true

	proxyConn := getProxyConn()
	if proxyConn == nil {
		return
	}

	profileSubCounter++
	subID := "ar-" + itoa(profileSubCounter)

	sub := proxyConn.Subscribe(subID, []*nostr.Filter{{
		Authors: []string{pk},
		Kinds:   []int{10002},
		Limit:   1,
		Proxy:   []string{"wss://purplepag.es"},
	}})
	sub.OnEvent = func(ev *nostr.Event) {
		if ev.Kind == 10002 {
			tags := ev.Tags.GetAll("r")
			if tags != nil {
				var urls []string
				for _, tag := range tags {
					u := tag.Value()
					if u != "" {
						urls = append(urls, u)
					}
				}
				if len(urls) > 0 {
					authorRelays[pk] = urls
				}
			}
		}
	}
	sub.OnEOSE = func() {
		sub.Close()
		if len(authorRelays[pk]) > 0 {
			fetchedK0[pk] = false
			fetchAuthorProfile(pk)
		}
	}
}

// applyAuthorProfile updates cache and all pending note headers for a pubkey.
func applyAuthorProfile(pk string, ev *nostr.Event) {
	if ev.CreatedAt <= authorTs[pk] {
		return
	}
	authorTs[pk] = ev.CreatedAt
	name := helpers.JsonGetString(ev.Content, "name")
	if name == "" {
		name = helpers.JsonGetString(ev.Content, "display_name")
	}
	pic := helpers.JsonGetString(ev.Content, "picture")
	if name != "" {
		authorNames[pk] = name
	}
	if pic != "" {
		authorPics[pk] = pic
	}

	// Update logged-in user's header too.
	if pk == pubhex {
		if name != "" {
			profileName = name
			dom.SetTextContent(nameEl, name)
		}
		if pic != "" {
			profilePic = pic
			dom.SetAttribute(avatarEl, "src", pic)
			dom.SetStyle(avatarEl, "display", "block")
		}
	}

	// Update all pending note headers.
	if headers, ok := pendingNotes[pk]; ok {
		for _, h := range headers {
			updateNoteHeader(h, name, pic)
		}
		delete(pendingNotes, pk)
	}
}

// updateNoteHeader fills in avatar+name on a note's author header div.
func updateNoteHeader(header dom.Element, name, pic string) {
	// First child is <img>, second is <span>.
	img := dom.FirstChild(header)
	if img == 0 {
		return
	}
	span := dom.NextSibling(img)
	if pic != "" {
		dom.SetAttribute(img, "src", pic)
		dom.SetAttribute(img, "onerror", "this.style.display='none'")
		dom.SetStyle(img, "display", "")
	}
	if name != "" {
		dom.SetTextContent(span, name)
	}
}

// --- Logout ---

func doLogout() {
	pubkey = nil
	pubhex = ""
	profileName = ""
	profilePic = ""
	profileTs = 0
	eventCount = 0
	popoverOpen = false

	// Reset relay tracking.
	relayURLs = nil
	relayDots = nil
	relayLabels = nil
	relayConnected = nil
	relayUserPick = nil

	localstorage.RemoveItem(lsKeyPubkey)
	localstorage.RemoveItem(lsKeyMode)

	clearChildren(root)
	showLogin()
}

// --- Markdown rendering ---
// All functions use string concatenation and indexOf — no byte-level ops.
// tinyjs compiles Go strings to JS strings (UTF-16); byte indexing corrupts emoji.

// renderMarkdown converts note text to safe HTML.
func renderMarkdown(s string) string {
	s = strReplace(s, "&", "&amp;")
	s = strReplace(s, "<", "&lt;")
	s = strReplace(s, ">", "&gt;")
	s = strReplace(s, "\"", "&quot;")
	s = wrapDelimited(s, "`", "<code>", "</code>")
	s = wrapDelimited(s, "**", "<strong>", "</strong>")
	s = wrapDelimited(s, "*", "<em>", "</em>")
	s = autoLinkURLs(s)
	s = strReplace(s, "\n", "<br>")
	return s
}

// strReplace replaces all occurrences of old with new using indexOf.
func strReplace(s, old, nw string) string {
	out := ""
	for {
		idx := strIndex(s, old)
		if idx < 0 {
			return out + s
		}
		out += s[:idx] + nw
		s = s[idx+len(old):]
	}
}

// wrapDelimited finds matching pairs of delim and wraps content in open/close tags.
func wrapDelimited(s, delim, open, close string) string {
	out := ""
	for {
		start := strIndex(s, delim)
		if start < 0 {
			return out + s
		}
		end := strIndex(s[start+len(delim):], delim)
		if end < 0 {
			return out + s
		}
		end += start + len(delim)
		inner := s[start+len(delim) : end]
		if len(inner) == 0 {
			out += s[:start+len(delim)]
			s = s[start+len(delim):]
			continue
		}
		out += s[:start] + open + inner + close
		s = s[end+len(delim):]
	}
}

func autoLinkURLs(s string) string {
	out := ""
	for {
		hi := strIndex(s, "https://")
		lo := strIndex(s, "http://")
		idx := -1
		if hi >= 0 && (lo < 0 || hi <= lo) {
			idx = hi
		} else if lo >= 0 {
			idx = lo
		}
		if idx < 0 {
			return out + s
		}
		out += s[:idx]
		s = s[idx:]
		// Find end of URL.
		end := 0
		for end < len(s) {
			c := s[end : end+1]
			if c == " " || c == "\n" || c == "\r" || c == "\t" || c == "<" || c == ">" {
				break
			}
			end++
		}
		// Trim trailing punctuation.
		for end > 0 {
			c := s[end-1 : end]
			if c == "." || c == "," || c == ")" || c == ";" {
				end--
			} else {
				break
			}
		}
		url := s[:end]
		if isImageURL(url) {
			out += "<img src=\"" + url + "\" style=\"display:block;max-width:100%;border-radius:8px;margin:4px 0\" loading=\"lazy\">"
		} else {
			out += "<a href=\"" + url + "\" target=\"_blank\" rel=\"noopener\" style=\"color:var(--accent);word-break:break-all\">" + url + "</a>"
		}
		s = s[end:]
	}
}

func isImageURL(url string) bool {
	u := toLower(url)
	return hasSuffix(u, ".jpg") || hasSuffix(u, ".jpeg") || hasSuffix(u, ".png") ||
		hasSuffix(u, ".gif") || hasSuffix(u, ".webp") || hasSuffix(u, ".svg")
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// strIndex finds substring in string. Returns -1 if not found.
func strIndex(s, sub string) int {
	sl := len(sub)
	for i := 0; i <= len(s)-sl; i++ {
		if s[i:i+sl] == sub {
			return i
		}
	}
	return -1
}

// --- Helpers ---

// wsToHTTP converts wss:// to https:// and ws:// to http://.
func wsToHTTP(u string) string {
	if len(u) > 6 && u[:6] == "wss://" {
		return "https://" + u[6:]
	}
	if len(u) > 5 && u[:5] == "ws://" {
		return "http://" + u[5:]
	}
	return u
}

// normalizeURL strips trailing slashes and lowercases the scheme+host.
func normalizeURL(u string) string {
	for len(u) > 0 && u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	// Lowercase scheme and host (before first / after ://).
	if len(u) > 6 && (u[:6] == "wss://" || u[:6] == "ws:///") {
		rest := u[6:]
		slash := strIndex(rest, "/")
		if slash < 0 {
			return u[:6] + toLower(rest)
		}
		return u[:6] + toLower(rest[:slash]) + rest[slash:]
	}
	return u
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

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
