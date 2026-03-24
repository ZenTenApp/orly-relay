package main

import (
	"common/helpers"
	"common/jsbridge/dom"
	"common/jsbridge/localstorage"
	"common/jsbridge/signer"
	"common/nostr"
)

const (
	version     = "v0.65.18"
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
	feedContainer dom.Element
	feedLoader    dom.Element
	statusEl      dom.Element
	popoverEl     dom.Element
	themeBtn      dom.Element
	pageTitleEl   dom.Element
	feedPage      dom.Element
	msgPage       dom.Element
	profilePage   dom.Element
	sidebarFeed   dom.Element
	sidebarMsg    dom.Element
	activePage    string
	profileViewPK string

	// App root — content goes here, not body (snackbar stays outside).
	root dom.Element

	// Messaging protocol selector.
	msgProtocol  = "marmot" // "nip04", "nip17", "marmot"
	msgProtoBtns map[string]dom.Element

	// Messaging UI state.
	msgListContainer   dom.Element // conversation list view
	msgThreadContainer dom.Element // thread view (header + messages + compose)
	msgThreadMessages  dom.Element // scrollable message area
	msgComposeInput    dom.Element // textarea
	msgCurrentPeer     string      // hex pubkey of open thread, "" when on list
	msgView            string      // "list" or "thread"
	marmotInited       bool

	// Relay tracking — parallel slices, grown dynamically.
	relayURLs     []string
	relayDots     []dom.Element
	relayLabels   []dom.Element
	relayUserPick []bool // true = from user's kind 10002

	eventCount  int
	popoverOpen bool

	// Author profile cache.
	authorNames   map[string]string       // pubkey hex -> display name
	authorPics    map[string]string       // pubkey hex -> avatar URL
	authorContent map[string]string       // pubkey hex -> full kind 0 content JSON
	authorTs      map[string]int64        // pubkey hex -> created_at of cached kind 0
	authorRelays map[string][]string     // pubkey hex -> relay URLs from kind 10002
	pendingNotes map[string][]dom.Element // pubkey hex -> author header divs awaiting profile
	fetchedK0    map[string]bool         // pubkey hex -> already tried kind 0 fetch
	fetchedK10k  map[string]bool         // pubkey hex -> already tried kind 10002 fetch
	seenEvents   map[string]bool         // event ID -> already rendered
	authorSubPK  map[string]string       // subID -> pubkey hex for author profile subs

	// Relay frequency — how many kind 10002 lists include each relay URL.
	relayFreq    map[string]int
	idbLoaded    bool

	// Profile page tab state.
	profileTab           string
	profileTabContent    dom.Element
	profileTabBtns       map[string]dom.Element
	authorFollows        map[string][]string
	authorMutes          map[string][]string
	profileNotesSeen     map[string]bool
	activeProfileNoteSub string
)

const orlyRelay = "wss://relay.orly.dev"

var defaultRelays = []string{
	orlyRelay,
	"wss://nostr.wine",
	"wss://nostr.land",
}

func main() {
	themePref := localstorage.GetItem(lsKeyTheme)
	if themePref != "" {
		isDark = themePref == "dark"
	} else {
		isDark = dom.PrefersDark()
	}
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

// --- Sidebar ---

const svgFeed = `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M3 3h12M6 7h9M6 11h9M3 15h12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>`
const svgChat = `<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M2 3h14v9H10l-2 3-2-3H2z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/></svg>`

func makeSidebarIcon(svgHTML string, active bool) dom.Element {
	btn := dom.CreateElement("button")
	dom.SetStyle(btn, "width", "36px")
	dom.SetStyle(btn, "height", "36px")
	dom.SetStyle(btn, "border", "none")
	dom.SetStyle(btn, "borderRadius", "6px")
	dom.SetStyle(btn, "cursor", "pointer")
	dom.SetStyle(btn, "display", "flex")
	dom.SetStyle(btn, "alignItems", "center")
	dom.SetStyle(btn, "justifyContent", "center")
	dom.SetStyle(btn, "padding", "0")
	dom.SetStyle(btn, "color", "var(--fg)")
	if active {
		dom.SetStyle(btn, "background", "var(--accent)")
		dom.SetStyle(btn, "color", "#000")
	} else {
		dom.SetStyle(btn, "background", "transparent")
	}
	dom.SetInnerHTML(btn, svgHTML)
	return btn
}

func switchPage(name string) {
	if name == activePage {
		return
	}
	closeProfileNoteSub()
	if activePage == "profile" {
		profileViewPK = ""
	}
	activePage = name
	dom.SetTextContent(pageTitleEl, name)

	// Hide all pages.
	dom.SetStyle(feedPage, "display", "none")
	dom.SetStyle(msgPage, "display", "none")
	dom.SetStyle(profilePage, "display", "none")

	// Clear sidebar highlights.
	dom.SetStyle(sidebarFeed, "background", "transparent")
	dom.SetStyle(sidebarFeed, "color", "var(--fg)")
	dom.SetStyle(sidebarMsg, "background", "transparent")
	dom.SetStyle(sidebarMsg, "color", "var(--fg)")

	switch name {
	case "feed":
		dom.SetStyle(feedPage, "display", "block")
		dom.SetStyle(sidebarFeed, "background", "var(--accent)")
		dom.SetStyle(sidebarFeed, "color", "#000")
	case "messaging":
		dom.SetStyle(msgPage, "display", "block")
		dom.SetStyle(sidebarMsg, "background", "var(--accent)")
		dom.SetStyle(sidebarMsg, "color", "#000")
		dom.PostToSW("[\"PAGE\",\"messaging\"]")
		initMessaging()
	case "profile":
		dom.SetStyle(profilePage, "display", "block")
	}
}

func makeProtoBtn(label string) dom.Element {
	btn := dom.CreateElement("button")
	dom.SetTextContent(btn, label)
	dom.SetStyle(btn, "padding", "6px 16px")
	dom.SetStyle(btn, "border", "none")
	dom.SetStyle(btn, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(btn, "fontSize", "12px")
	dom.SetStyle(btn, "cursor", "default")
	dom.SetStyle(btn, "background", "transparent")
	dom.SetStyle(btn, "color", "var(--fg)")
	return btn
}

func selectMsgProtocol(id string) {
	msgProtocol = id
	for pid, btn := range msgProtoBtns {
		if pid == id {
			dom.SetStyle(btn, "background", "var(--accent)")
			dom.SetStyle(btn, "color", "#000")
		} else {
			dom.SetStyle(btn, "background", "transparent")
			dom.SetStyle(btn, "color", "var(--fg)")
		}
	}
}

// --- Main app ---

func showApp() {
	// Init profile cache maps.
	authorNames = make(map[string]string)
	authorPics = make(map[string]string)
	authorContent = make(map[string]string)
	authorTs = make(map[string]int64)
	authorRelays = make(map[string][]string)
	pendingNotes = make(map[string][]dom.Element)
	fetchedK0 = make(map[string]bool)
	fetchedK10k = make(map[string]bool)
	relayFreq = make(map[string]int)
	authorSubPK = make(map[string]string)
	seenEvents = make(map[string]bool)
	authorFollows = make(map[string][]string)
	authorMutes = make(map[string][]string)
	profileNotesSeen = make(map[string]bool)
	profileTabBtns = make(map[string]dom.Element)

	// Set up SW communication.
	dom.OnSWMessage(onSWMessage)
	dom.PostToSW("[\"SET_PUBKEY\"," + jstr(pubhex) + "]")

	// Load cached profiles from IndexedDB.
	dom.IDBGetAll("profiles", func(key, val string) {
		name := helpers.JsonGetString(val, "name")
		pic := helpers.JsonGetString(val, "picture")
		if name != "" {
			authorNames[key] = name
		}
		if pic != "" {
			authorPics[key] = pic
		}
	}, func() {
		idbLoaded = true
	})

	// === Top bar ===
	bar := dom.CreateElement("div")
	dom.SetStyle(bar, "display", "flex")
	dom.SetStyle(bar, "alignItems", "center")
	dom.SetStyle(bar, "padding", "8px 16px")
	dom.SetStyle(bar, "height", "48px")
	dom.SetStyle(bar, "boxSizing", "border-box")
	dom.SetStyle(bar, "background", "var(--bg2)")
	dom.SetStyle(bar, "position", "fixed")
	dom.SetStyle(bar, "top", "0")
	dom.SetStyle(bar, "left", "0")
	dom.SetStyle(bar, "right", "0")
	dom.SetStyle(bar, "zIndex", "100")

	// Left: page title.
	left := dom.CreateElement("div")
	dom.SetStyle(left, "display", "flex")
	dom.SetStyle(left, "alignItems", "center")
	dom.SetStyle(left, "flex", "1")
	dom.SetStyle(left, "minWidth", "0")

	pageTitleEl = dom.CreateElement("span")
	dom.SetStyle(pageTitleEl, "fontSize", "18px")
	dom.SetStyle(pageTitleEl, "fontWeight", "bold")
	dom.SetTextContent(pageTitleEl, "feed")
	dom.AppendChild(left, pageTitleEl)
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

	// === Main layout: sidebar + content ===
	mainLayout := dom.CreateElement("div")
	dom.SetStyle(mainLayout, "position", "fixed")
	dom.SetStyle(mainLayout, "top", "48px")
	dom.SetStyle(mainLayout, "bottom", "36px")
	dom.SetStyle(mainLayout, "left", "0")
	dom.SetStyle(mainLayout, "right", "0")
	dom.SetStyle(mainLayout, "display", "flex")

	// Sidebar.
	sidebar := dom.CreateElement("div")
	dom.SetStyle(sidebar, "width", "44px")
	dom.SetStyle(sidebar, "flexShrink", "0")
	dom.SetStyle(sidebar, "background", "var(--bg2)")
	dom.SetStyle(sidebar, "display", "flex")
	dom.SetStyle(sidebar, "flexDirection", "column")
	dom.SetStyle(sidebar, "alignItems", "center")
	dom.SetStyle(sidebar, "paddingTop", "8px")
	dom.SetStyle(sidebar, "gap", "4px")

	sidebarFeed = makeSidebarIcon(svgFeed, true)
	dom.AddEventListener(sidebarFeed, "click", dom.RegisterCallback(func() {
		switchPage("feed")
	}))
	dom.AppendChild(sidebar, sidebarFeed)

	sidebarMsg = makeSidebarIcon(svgChat, false)
	dom.AddEventListener(sidebarMsg, "click", dom.RegisterCallback(func() {
		switchPage("messaging")
	}))
	dom.AppendChild(sidebar, sidebarMsg)

	dom.AppendChild(mainLayout, sidebar)

	// Content area.
	contentArea := dom.CreateElement("div")
	dom.SetStyle(contentArea, "flex", "1")
	dom.SetStyle(contentArea, "overflowY", "auto")

	// Feed page.
	feedPage = dom.CreateElement("div")
	dom.SetStyle(feedPage, "padding", "16px")

	// Loading spinner — shown until first feed event arrives.
	feedLoader = dom.CreateElement("div")
	dom.SetStyle(feedLoader, "display", "flex")
	dom.SetStyle(feedLoader, "flexDirection", "column")
	dom.SetStyle(feedLoader, "alignItems", "center")
	dom.SetStyle(feedLoader, "justifyContent", "center")
	dom.SetStyle(feedLoader, "padding", "64px 0")
	loaderImg := dom.CreateElement("div")
	dom.SetStyle(loaderImg, "width", "120px")
	dom.SetStyle(loaderImg, "height", "120px")
	dom.FetchText("./smesh-loader.svg", func(svg string) {
		dom.SetInnerHTML(loaderImg, svg)
		svgEl := dom.FirstChild(loaderImg)
		if svgEl != 0 {
			dom.SetAttribute(svgEl, "width", "100%")
			dom.SetAttribute(svgEl, "height", "100%")
		}
	})
	dom.AppendChild(feedLoader, loaderImg)
	loaderText := dom.CreateElement("div")
	dom.SetTextContent(loaderText, "connecting...")
	dom.SetStyle(loaderText, "marginTop", "16px")
	dom.SetStyle(loaderText, "color", "var(--muted)")
	dom.SetStyle(loaderText, "fontSize", "14px")
	dom.AppendChild(feedLoader, loaderText)
	dom.AppendChild(feedPage, feedLoader)

	feedContainer = dom.CreateElement("div")
	dom.AppendChild(feedPage, feedContainer)
	dom.AppendChild(contentArea, feedPage)

	// Messaging page.
	msgPage = dom.CreateElement("div")
	dom.SetStyle(msgPage, "padding", "16px")
	dom.SetStyle(msgPage, "display", "none")
	dom.SetStyle(msgPage, "position", "relative")
	dom.SetStyle(msgPage, "height", "100%")
	dom.SetStyle(msgPage, "boxSizing", "border-box")

	// Protocol selector bar.
	protoBar := dom.CreateElement("div")
	dom.SetStyle(protoBar, "display", "flex")
	dom.SetStyle(protoBar, "gap", "0")
	dom.SetStyle(protoBar, "marginBottom", "16px")
	dom.SetStyle(protoBar, "border", "1px solid var(--border)")
	dom.SetStyle(protoBar, "borderRadius", "6px")
	dom.SetStyle(protoBar, "overflow", "hidden")
	dom.SetStyle(protoBar, "width", "fit-content")

	msgProtoBtns = make(map[string]dom.Element)

	// Unrolled — tinyjs range loops over struct slices can alias the label.
	btnNip04 := makeProtoBtn("nip-04")
	dom.SetStyle(btnNip04, "opacity", "0.3")
	dom.SetStyle(btnNip04, "pointerEvents", "none")
	msgProtoBtns["nip04"] = btnNip04
	dom.AppendChild(protoBar, btnNip04)

	btnNip17 := makeProtoBtn("nip-17")
	dom.SetStyle(btnNip17, "opacity", "0.3")
	dom.SetStyle(btnNip17, "pointerEvents", "none")
	msgProtoBtns["nip17"] = btnNip17
	dom.AppendChild(protoBar, btnNip17)

	btnMarmot := makeProtoBtn("marmot")
	dom.SetStyle(btnMarmot, "background", "var(--accent)")
	dom.SetStyle(btnMarmot, "color", "#000")
	dom.SetStyle(btnMarmot, "cursor", "pointer")
	dom.AddEventListener(btnMarmot, "click", dom.RegisterCallback(func() {
		selectMsgProtocol("marmot")
	}))
	msgProtoBtns["marmot"] = btnMarmot
	dom.AppendChild(protoBar, btnMarmot)

	dom.AppendChild(msgPage, protoBar)

	// Conversation list view.
	msgListContainer = dom.CreateElement("div")
	dom.AppendChild(msgPage, msgListContainer)

	// Thread view (hidden by default).
	msgThreadContainer = dom.CreateElement("div")
	dom.SetStyle(msgThreadContainer, "display", "none")
	dom.SetStyle(msgThreadContainer, "flexDirection", "column")
	dom.SetStyle(msgThreadContainer, "position", "absolute")
	dom.SetStyle(msgThreadContainer, "top", "0")
	dom.SetStyle(msgThreadContainer, "left", "0")
	dom.SetStyle(msgThreadContainer, "right", "0")
	dom.SetStyle(msgThreadContainer, "bottom", "0")
	dom.SetStyle(msgThreadContainer, "background", "var(--bg)")
	dom.AppendChild(msgPage, msgThreadContainer)

	msgView = "list"

	dom.AppendChild(contentArea, msgPage)

	// Profile page.
	profilePage = dom.CreateElement("div")
	dom.SetStyle(profilePage, "display", "none")
	dom.AppendChild(contentArea, profilePage)

	dom.AppendChild(mainLayout, contentArea)
	dom.AppendChild(root, mainLayout)
	activePage = "feed"

	// === Bottom status bar ===
	bottomBar := dom.CreateElement("div")
	dom.SetStyle(bottomBar, "position", "fixed")
	dom.SetStyle(bottomBar, "bottom", "0")
	dom.SetStyle(bottomBar, "left", "0")
	dom.SetStyle(bottomBar, "right", "0")
	dom.SetStyle(bottomBar, "height", "36px")
	dom.SetStyle(bottomBar, "display", "flex")
	dom.SetStyle(bottomBar, "alignItems", "center")
	dom.SetStyle(bottomBar, "padding", "0 12px")
	dom.SetStyle(bottomBar, "gap", "8px")
	dom.SetStyle(bottomBar, "background", "var(--bg2)")
	dom.SetStyle(bottomBar, "fontSize", "12px")
	dom.SetStyle(bottomBar, "color", "var(--fg)")
	dom.SetStyle(bottomBar, "zIndex", "100")

	// Avatar + name.
	avatarEl = dom.CreateElement("img")
	dom.SetAttribute(avatarEl, "width", "20")
	dom.SetAttribute(avatarEl, "height", "20")
	dom.SetStyle(avatarEl, "borderRadius", "50%")
	dom.SetStyle(avatarEl, "objectFit", "cover")
	dom.SetStyle(avatarEl, "display", "none")
	dom.SetAttribute(avatarEl, "onerror", "this.style.display='none'")
	dom.AppendChild(bottomBar, avatarEl)

	nameEl = dom.CreateElement("span")
	dom.SetStyle(nameEl, "fontSize", "12px")
	dom.SetStyle(nameEl, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(nameEl, "fontWeight", "bold")
	dom.SetStyle(nameEl, "overflow", "hidden")
	dom.SetStyle(nameEl, "textOverflow", "ellipsis")
	dom.SetStyle(nameEl, "whiteSpace", "nowrap")
	dom.SetStyle(nameEl, "maxWidth", "120px")
	npubStr := helpers.EncodeNpub(pubkey)
	if len(npubStr) > 20 {
		dom.SetTextContent(nameEl, npubStr[:12]+"..."+npubStr[len(npubStr)-4:])
	}
	dom.AppendChild(bottomBar, nameEl)

	dom.SetStyle(avatarEl, "cursor", "pointer")
	dom.SetStyle(nameEl, "cursor", "pointer")
	dom.AddEventListener(avatarEl, "click", dom.RegisterCallback(func() {
		showProfile(pubhex)
	}))
	dom.AddEventListener(nameEl, "click", dom.RegisterCallback(func() {
		showProfile(pubhex)
	}))

	sep := dom.CreateElement("span")
	dom.SetTextContent(sep, "|")
	dom.SetStyle(sep, "color", "var(--muted)")
	dom.AppendChild(bottomBar, sep)

	statusEl = dom.CreateElement("span")
	dom.SetTextContent(statusEl, "connecting...")
	dom.SetStyle(statusEl, "cursor", "pointer")
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
	dom.SetStyle(popoverEl, "left", "44px")
	dom.SetStyle(popoverEl, "right", "0")
	dom.SetStyle(popoverEl, "background", "var(--bg2)")
	dom.SetStyle(popoverEl, "borderTop", "1px solid var(--border)")
	dom.SetStyle(popoverEl, "padding", "12px 16px")
	dom.SetStyle(popoverEl, "fontSize", "12px")
	dom.SetStyle(popoverEl, "display", "none")
	dom.SetStyle(popoverEl, "zIndex", "99")
	dom.AppendChild(root, popoverEl)

	// Add default relays.
	for _, url := range defaultRelays {
		addRelay(url, false)
	}

	// Tell SW about relays and subscribe.
	sendWriteRelays()
	subscribeProfile()
	subscribeFeed()
}

// addRelay adds a relay to the list and creates its popover row.
// userPick=true means it came from the user's kind 10002 relay list.
func addRelay(url string, userPick bool) {
	url = normalizeURL(url)
	// Dedup.
	for i, u := range relayURLs {
		if u == url {
			if userPick && !relayUserPick[i] {
				relayUserPick[i] = true
				dom.SetStyle(relayLabels[i], "fontWeight", "bold")
			}
			return
		}
	}

	relayURLs = append(relayURLs, url)
	relayUserPick = append(relayUserPick, userPick)

	// Popover row.
	row := dom.CreateElement("div")
	dom.SetStyle(row, "padding", "3px 0")

	dot := dom.CreateElement("span")
	dom.SetTextContent(dot, "\u25CF")
	dom.SetStyle(dot, "color", "#5b5")
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
	updateStatus()
}

func togglePopover() {
	popoverOpen = !popoverOpen
	if popoverOpen {
		dom.SetStyle(popoverEl, "display", "block")
	} else {
		dom.SetStyle(popoverEl, "display", "none")
	}
}

func subscribeProfile() {
	proxy := make([]string, 0, len(relayURLs)+1)
	proxy = append(proxy, "wss://purplepag.es")
	proxy = append(proxy, relayURLs...)
	dom.PostToSW(buildProxyMsg("prof",
		"{\"authors\":["+jstr(pubhex)+"],\"kinds\":[0,3,10002,10000,10050],\"_proxy\":"+jstrArr(proxy)+",\"limit\":8}",
		[]string{orlyRelay}))
}

func subscribeFeed() {
	dom.PostToSW(buildProxyMsg("feed", "{\"kinds\":[1],\"limit\":20}", relayURLs))
}

func sendWriteRelays() {
	msg := "[\"SET_WRITE_RELAYS\",["
	for i, url := range relayURLs {
		if i > 0 {
			msg += ","
		}
		msg += jstr(url)
	}
	dom.PostToSW(msg + "]]")
}

func buildProxyMsg(subID, filterJSON string, urls []string) string {
	msg := "[\"PROXY\"," + jstr(subID) + "," + filterJSON + ",["
	for i, url := range urls {
		if i > 0 {
			msg += ","
		}
		msg += jstr(url)
	}
	return msg + "]]"
}

func jstr(s string) string {
	return "\"" + jsonEsc(s) + "\""
}

// jstrArr builds a JSON array of quoted strings.
func jstrArr(ss []string) string {
	s := "["
	for i, v := range ss {
		if i > 0 {
			s += ","
		}
		s += jstr(v)
	}
	return s + "]"
}

// --- SW message handling ---

func onSWMessage(raw string) {
	if raw == "update-available" {
		dom.PostToSW("activate-update")
		return
	}
	if len(raw) < 5 || raw[0] != '[' {
		return
	}
	typ, pos := nextStr(raw, 1)
	switch typ {
	case "EVENT":
		subID, pos2 := nextStr(raw, pos)
		evJSON := extractValue(raw, pos2)
		if evJSON == "" {
			return
		}
		ev := nostr.ParseEvent(evJSON)
		if ev == nil {
			return
		}
		dispatchEvent(subID, ev)
	case "EOSE":
		subID, _ := nextStr(raw, pos)
		dispatchEOSE(subID)
	case "DM_LIST":
		listJSON := extractValue(raw, pos)
		renderConversationList(listJSON)
	case "DM_HISTORY":
		peer, pos2 := nextStr(raw, pos)
		msgsJSON := extractValue(raw, pos2)
		renderThreadMessages(peer, msgsJSON)
	case "DM_RECEIVED":
		dmJSON := extractValue(raw, pos)
		handleDMReceived(dmJSON)
	case "DM_SENT":
		// Confirmation — optimistic render already done.
	case "MLS_GROUPS":
		// Store for future use.
	}
}

func dispatchEvent(subID string, ev *nostr.Event) {
	if subID == "prof" {
		handleProfileEvent(ev)
	} else if subID == "feed" {
		if seenEvents[ev.ID] {
			return
		}
		seenEvents[ev.ID] = true
		eventCount++
		if feedLoader != 0 {
			dom.RemoveChild(feedPage, feedLoader)
			feedLoader = 0
		}
		renderNote(ev)
	} else if subID == "ap-batch" {
		if ev.Kind == 0 {
			applyAuthorProfile(ev.PubKey, ev)
		}
	} else if len(subID) > 3 && subID[:3] == "ap-" {
		if ev.Kind == 0 {
			applyAuthorProfile(ev.PubKey, ev)
		} else if ev.Kind == 3 {
			var pks []string
			for _, tag := range ev.Tags.GetAll("p") {
				if v := tag.Value(); v != "" {
					pks = append(pks, v)
				}
			}
			authorFollows[ev.PubKey] = pks
		} else if ev.Kind == 10002 {
			recordRelayFreq(ev)
		} else if ev.Kind == 10000 {
			var pks []string
			for _, tag := range ev.Tags.GetAll("p") {
				if v := tag.Value(); v != "" {
					pks = append(pks, v)
				}
			}
			authorMutes[ev.PubKey] = pks
		}
	} else if len(subID) > 3 && subID[:3] == "pn-" {
		if profileNotesSeen[ev.ID] {
			return
		}
		profileNotesSeen[ev.ID] = true
		renderProfileNote(ev)
	}
}

func dispatchEOSE(subID string) {
	if subID == "feed" {
		if feedLoader != 0 {
			dom.RemoveChild(feedPage, feedLoader)
			feedLoader = 0
		}
		updateStatus()
		retryMissingProfiles()
	} else if subID == "ap-batch" {
		dom.PostToSW("[\"CLOSE\",\"ap-batch\"]")
	} else if len(subID) > 3 && subID[:3] == "ap-" {
		dom.PostToSW("[\"CLOSE\"," + jstr(subID) + "]")
		pk, ok := authorSubPK[subID]
		if !ok {
			return
		}
		delete(authorSubPK, subID)
		if _, got := authorNames[pk]; !got {
			if rels, ok := authorRelays[pk]; ok && len(rels) > 0 && !fetchedK10k[pk] {
				fetchedK10k[pk] = true
				fetchedK0[pk] = false
				fetchAuthorProfile(pk)
			}
		}
	}
}

// nextStr extracts the next quoted string from s starting at pos.
func nextStr(s string, pos int) (string, int) {
	for pos < len(s) && s[pos] != '"' {
		pos++
	}
	if pos >= len(s) {
		return "", pos
	}
	pos++
	start := pos
	for pos < len(s) {
		if s[pos] == '\\' {
			pos += 2
			continue
		}
		if s[pos] == '"' {
			break
		}
		pos++
	}
	if pos >= len(s) {
		return "", pos
	}
	val := s[start:pos]
	pos++
	for pos < len(s) && (s[pos] == ',' || s[pos] == ' ') {
		pos++
	}
	return val, pos
}

// extractValue extracts a JSON object/array value starting at pos.
func extractValue(s string, pos int) string {
	for pos < len(s) && (s[pos] == ',' || s[pos] == ' ') {
		pos++
	}
	if pos >= len(s) {
		return ""
	}
	if s[pos] != '{' && s[pos] != '[' {
		return ""
	}
	start := pos
	depth := 0
	for pos < len(s) {
		c := s[pos]
		if c == '{' || c == '[' {
			depth++
		}
		if c == '}' || c == ']' {
			depth--
			if depth == 0 {
				return s[start : pos+1]
			}
		}
		if c == '"' {
			pos++
			for pos < len(s) && s[pos] != '"' {
				if s[pos] == '\\' {
					pos++
				}
				pos++
			}
		}
		pos++
	}
	return s[start:]
}

func handleProfileEvent(ev *nostr.Event) {
	switch ev.Kind {
	case 0:
		if ev.CreatedAt <= profileTs {
			return
		}
		profileTs = ev.CreatedAt
		authorContent[pubhex] = ev.Content
		name := helpers.JsonGetString(ev.Content, "name")
		if name == "" {
			name = helpers.JsonGetString(ev.Content, "display_name")
		}
		pic := helpers.JsonGetString(ev.Content, "picture")
		if name != "" {
			profileName = name
			authorNames[pubhex] = name
			dom.SetTextContent(nameEl, name)
		}
		if pic != "" {
			profilePic = pic
			authorPics[pubhex] = pic
			dom.SetAttribute(avatarEl, "src", pic)
			dom.SetStyle(avatarEl, "display", "block")
		}
		if profileViewPK == pubhex {
			renderProfilePage(pubhex)
		}
	case 3:
		var pks []string
		for _, tag := range ev.Tags.GetAll("p") {
			if v := tag.Value(); v != "" {
				pks = append(pks, v)
			}
		}
		authorFollows[pubhex] = pks
	case 10000:
		var pks []string
		for _, tag := range ev.Tags.GetAll("p") {
			if v := tag.Value(); v != "" {
				pks = append(pks, v)
			}
		}
		authorMutes[pubhex] = pks
	case 10002:
		// NIP-65 relay list — add user's preferred relays.
		recordRelayFreq(ev)
		for _, tag := range ev.Tags.GetAll("r") {
			url := tag.Value()
			if url != "" {
				addRelay(url, true)
			}
		}
		sendWriteRelays()
		subscribeFeed()
	case 10050:
		// DM inbox relay list — stored for future use.
		_ = ev.Tags.GetAll("relay")
	}
}

func updateStatus() {
	dom.SetTextContent(statusEl,
		itoa(len(relayURLs))+" relays | "+itoa(eventCount)+" events")
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
	dom.SetStyle(header, "cursor", "pointer")
	headerPK := ev.PubKey
	dom.AddEventListener(header, "click", dom.RegisterCallback(func() {
		showProfile(headerPK)
	}))

	avatar := dom.CreateElement("img")
	dom.SetAttribute(avatar, "width", "24")
	dom.SetAttribute(avatar, "height", "24")
	dom.SetStyle(avatar, "borderRadius", "50%")
	dom.SetStyle(avatar, "objectFit", "cover")
	dom.SetStyle(avatar, "flexShrink", "0")

	nameSpan := dom.CreateElement("span")
	dom.SetStyle(nameSpan, "fontSize", "18px")
	dom.SetStyle(nameSpan, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(nameSpan, "fontWeight", "bold")
	dom.SetStyle(nameSpan, "color", "var(--fg)")

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
	truncated := len(text) > 500
	if truncated {
		text = text[:500] + "..."
	}
	dom.SetInnerHTML(content, renderMarkdown(text))
	dom.AppendChild(note, content)

	if truncated {
		more := dom.CreateElement("span")
		dom.SetTextContent(more, "show more")
		dom.SetStyle(more, "color", "var(--accent)")
		dom.SetStyle(more, "cursor", "pointer")
		dom.SetStyle(more, "fontSize", "13px")
		dom.SetStyle(more, "display", "inline-block")
		dom.SetStyle(more, "marginTop", "4px")
		fullContent := ev.Content
		expanded := false
		dom.AddEventListener(more, "click", dom.RegisterCallback(func() {
			expanded = !expanded
			if expanded {
				dom.SetInnerHTML(content, renderMarkdown(fullContent))
				dom.SetTextContent(more, "show less")
			} else {
				dom.SetInnerHTML(content, renderMarkdown(fullContent[:500]+"..."))
				dom.SetTextContent(more, "show more")
			}
		}))
		dom.AppendChild(note, more)
	}

	// Prepend (newest first).
	first := dom.FirstChild(feedContainer)
	if first != 0 {
		dom.InsertBefore(feedContainer, note, first)
	} else {
		dom.AppendChild(feedContainer, note)
	}
}

var profileSubCounter int

// topRelays returns the n most frequently seen relay URLs from kind 10002 events.
func topRelays(n int) []string {
	if relayFreq == nil {
		return nil
	}
	// Simple selection sort — n is small.
	type kv struct {
		url   string
		count int
	}
	var all []kv
	for url, count := range relayFreq {
		all = append(all, kv{url, count})
	}
	// Sort descending by count.
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].count > all[i].count {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	var result []string
	for i := 0; i < len(all) && i < n; i++ {
		result = append(result, all[i].url)
	}
	return result
}

// recordRelayFreq records relay URLs from a kind 10002 event into the frequency table.
func recordRelayFreq(ev *nostr.Event) {
	tags := ev.Tags.GetAll("r")
	if tags == nil {
		return
	}
	var urls []string
	for _, tag := range tags {
		u := tag.Value()
		if u != "" {
			urls = append(urls, u)
			if _, ok := relayFreq[u]; ok {
				relayFreq[u] = relayFreq[u] + 1
			} else {
				relayFreq[u] = 1
			}
		}
	}
	if len(urls) > 0 {
		authorRelays[ev.PubKey] = urls
	}
}

// buildProxy builds a _proxy relay list for a pubkey.
// Always includes purplepag.es for metadata discovery.
func buildProxy(pk string) []string {
	pp := "wss://purplepag.es"
	if rels, ok := authorRelays[pk]; ok && len(rels) > 0 {
		return appendUnique(rels, pp)
	}
	top := topRelays(5)
	if len(top) > 0 {
		return appendUnique(top, pp)
	}
	return []string{pp}
}

func appendUnique(list []string, val string) []string {
	for _, v := range list {
		if v == val {
			return list
		}
	}
	return append(list, val)
}

// fetchAuthorProfile fetches kind 0 + kind 10002 for an author via SW PROXY.
func fetchAuthorProfile(pk string) {
	if fetchedK0[pk] {
		return
	}
	fetchedK0[pk] = true

	profileSubCounter++
	subID := "ap-" + itoa(profileSubCounter)
	authorSubPK[subID] = pk

	proxyRelays := buildProxy(pk)
	dom.PostToSW(buildProxyMsg(subID,
		"{\"authors\":["+jstr(pk)+"],\"kinds\":[0,3,10002,10000],\"_proxy\":"+jstrArr(proxyRelays)+",\"limit\":6}",
		[]string{orlyRelay}))
}

// retryMissingProfiles batches all pubkeys that still lack a name
// into a single PROXY request after the feed finishes loading.
func retryMissingProfiles() {
	var missing []string
	for pk := range pendingNotes {
		if _, ok := authorNames[pk]; !ok {
			missing = append(missing, pk)
		}
	}
	if len(missing) == 0 {
		return
	}

	// Build JSON authors array.
	authors := "["
	for i, pk := range missing {
		if i > 0 {
			authors += ","
		}
		authors += jstr(pk)
	}
	authors += "]"

	// Route through orly relay with _proxy to external relays.
	proxy := []string{"wss://purplepag.es"}
	proxy = append(proxy, relayURLs...)
	top := topRelays(3)
	for _, u := range top {
		proxy = appendUnique(proxy, u)
	}

	dom.PostToSW(buildProxyMsg("ap-batch",
		"{\"authors\":"+authors+",\"kinds\":[0],\"_proxy\":"+jstrArr(proxy)+",\"limit\":"+itoa(len(missing))+"}",
		[]string{orlyRelay}))
}

// applyAuthorProfile updates cache and all pending note headers for a pubkey.
func applyAuthorProfile(pk string, ev *nostr.Event) {
	if ev.CreatedAt <= authorTs[pk] {
		return
	}
	authorTs[pk] = ev.CreatedAt
	authorContent[pk] = ev.Content
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

	// Cache to IndexedDB.
	if name != "" || pic != "" {
		dom.IDBPut("profiles", pk, "{\"name\":\""+jsonEsc(name)+"\",\"picture\":\""+jsonEsc(pic)+"\"}")
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

	// Re-render profile page if viewing this author.
	if profileViewPK == pk {
		renderProfilePage(pk)
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

// --- Profile page ---

func showProfile(pk string) {
	profileViewPK = pk

	// Ensure we have full kind 0 content. If not, fetch it.
	if _, ok := authorContent[pk]; !ok {
		fetchedK0[pk] = false
		fetchAuthorProfile(pk)
	}

	renderProfilePage(pk)

	// Use the author's name as page title, fall back to "profile".
	title := "profile"
	if name, ok := authorNames[pk]; ok && name != "" {
		title = name
	}
	activePage = "" // force switchPage to run
	switchPage("profile")
	dom.SetTextContent(pageTitleEl, title)
}

func renderProfilePage(pk string) {
	savedTab := profileTab
	clearChildren(profilePage)
	closeProfileNoteSub()
	profileNotesSeen = make(map[string]bool)

	content := authorContent[pk]
	name := authorNames[pk]
	pic := authorPics[pk]
	about := helpers.JsonGetString(content, "about")
	website := helpers.JsonGetString(content, "website")
	nip05 := helpers.JsonGetString(content, "nip05")
	lud16 := helpers.JsonGetString(content, "lud16")
	banner := helpers.JsonGetString(content, "banner")

	// Banner — full width, 200px, cover.
	if banner != "" {
		bannerEl := dom.CreateElement("img")
		dom.SetAttribute(bannerEl, "src", banner)
		dom.SetStyle(bannerEl, "width", "100%")
		dom.SetStyle(bannerEl, "height", "200px")
		dom.SetStyle(bannerEl, "objectFit", "cover")
		dom.SetStyle(bannerEl, "objectPosition", "center")
		dom.SetStyle(bannerEl, "display", "block")
		dom.SetAttribute(bannerEl, "onerror", "this.style.display='none'")
		dom.AppendChild(profilePage, bannerEl)
	}

	// User info card — glass effect, overlapping banner.
	card := dom.CreateElement("div")
	dom.SetStyle(card, "background", "color-mix(in srgb, var(--bg) 85%, transparent)")
	dom.SetStyle(card, "backdropFilter", "blur(8px)")
	dom.SetStyle(card, "borderRadius", "8px")
	dom.SetStyle(card, "padding", "16px")
	if banner != "" {
		dom.SetStyle(card, "margin", "-48px 16px 0")
	} else {
		dom.SetStyle(card, "margin", "16px")
	}
	dom.SetStyle(card, "position", "relative")

	// Top row: avatar + info.
	topRow := dom.CreateElement("div")
	dom.SetStyle(topRow, "display", "flex")
	dom.SetStyle(topRow, "gap", "16px")
	dom.SetStyle(topRow, "alignItems", "flex-start")

	if pic != "" {
		av := dom.CreateElement("img")
		dom.SetAttribute(av, "src", pic)
		dom.SetAttribute(av, "width", "64")
		dom.SetAttribute(av, "height", "64")
		dom.SetStyle(av, "borderRadius", "50%")
		dom.SetStyle(av, "objectFit", "cover")
		dom.SetStyle(av, "flexShrink", "0")
		dom.SetStyle(av, "border", "3px solid var(--bg)")
		dom.SetAttribute(av, "onerror", "this.style.display='none'")
		dom.AppendChild(topRow, av)
	}

	info := dom.CreateElement("div")
	dom.SetStyle(info, "minWidth", "0")
	dom.SetStyle(info, "flex", "1")

	if name != "" {
		nameSpan := dom.CreateElement("div")
		dom.SetTextContent(nameSpan, name)
		dom.SetStyle(nameSpan, "fontSize", "20px")
		dom.SetStyle(nameSpan, "fontWeight", "bold")
		dom.SetStyle(nameSpan, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
		dom.AppendChild(info, nameSpan)
	}

	if nip05 != "" {
		nip05El := dom.CreateElement("div")
		dom.SetTextContent(nip05El, nip05)
		dom.SetStyle(nip05El, "color", "var(--muted)")
		dom.SetStyle(nip05El, "fontSize", "13px")
		dom.AppendChild(info, nip05El)
	}

	// npub (truncated).
	npubBytes := helpers.HexDecode(pk)
	npubStr := helpers.EncodeNpub(npubBytes)
	npubEl := dom.CreateElement("div")
	dom.SetStyle(npubEl, "color", "var(--muted)")
	dom.SetStyle(npubEl, "fontSize", "12px")
	dom.SetStyle(npubEl, "marginTop", "2px")
	if len(npubStr) > 20 {
		dom.SetTextContent(npubEl, npubStr[:16]+"..."+npubStr[len(npubStr)-8:])
	} else {
		dom.SetTextContent(npubEl, npubStr)
	}
	dom.AppendChild(info, npubEl)

	// Website + lightning inline.
	if website != "" || lud16 != "" {
		metaRow := dom.CreateElement("div")
		dom.SetStyle(metaRow, "display", "flex")
		dom.SetStyle(metaRow, "gap", "12px")
		dom.SetStyle(metaRow, "marginTop", "6px")
		dom.SetStyle(metaRow, "fontSize", "12px")
		if website != "" {
			wEl := dom.CreateElement("span")
			dom.SetStyle(wEl, "color", "var(--accent)")
			dom.SetStyle(wEl, "wordBreak", "break-all")
			dom.SetTextContent(wEl, website)
			dom.AppendChild(metaRow, wEl)
		}
		if lud16 != "" {
			lEl := dom.CreateElement("span")
			dom.SetStyle(lEl, "color", "var(--muted)")
			dom.SetTextContent(lEl, "\xE2\x9A\xA1 "+lud16)
			dom.AppendChild(metaRow, lEl)
		}
		dom.AppendChild(info, metaRow)
	}

	dom.AppendChild(topRow, info)
	dom.AppendChild(card, topRow)

	// Message button — only for other users.
	if pk != pubhex {
		msgBtn := dom.CreateElement("button")
		dom.SetTextContent(msgBtn, "message")
		dom.SetStyle(msgBtn, "padding", "6px 16px")
		dom.SetStyle(msgBtn, "fontFamily", "'Fira Code', monospace")
		dom.SetStyle(msgBtn, "fontSize", "12px")
		dom.SetStyle(msgBtn, "background", "var(--accent)")
		dom.SetStyle(msgBtn, "color", "#000")
		dom.SetStyle(msgBtn, "border", "none")
		dom.SetStyle(msgBtn, "borderRadius", "4px")
		dom.SetStyle(msgBtn, "cursor", "pointer")
		dom.SetStyle(msgBtn, "marginTop", "12px")
		peerPK := pk
		dom.AddEventListener(msgBtn, "click", dom.RegisterCallback(func() {
			switchPage("messaging")
			openThread(peerPK)
		}))
		dom.AppendChild(card, msgBtn)
	}

	dom.AppendChild(profilePage, card)

	// About/bio.
	if about != "" {
		aboutEl := dom.CreateElement("div")
		dom.SetStyle(aboutEl, "padding", "12px 16px")
		dom.SetStyle(aboutEl, "fontSize", "14px")
		dom.SetStyle(aboutEl, "lineHeight", "1.5")
		dom.SetStyle(aboutEl, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
		dom.SetStyle(aboutEl, "wordBreak", "break-word")
		aboutTruncated := len(about) > 300
		aboutText := about
		if aboutTruncated {
			aboutText = about[:300] + "..."
		}
		dom.SetInnerHTML(aboutEl, renderMarkdown(aboutText))
		dom.AppendChild(profilePage, aboutEl)

		if aboutTruncated {
			more := dom.CreateElement("span")
			dom.SetTextContent(more, "show more")
			dom.SetStyle(more, "color", "var(--accent)")
			dom.SetStyle(more, "cursor", "pointer")
			dom.SetStyle(more, "fontSize", "13px")
			dom.SetStyle(more, "display", "inline-block")
			dom.SetStyle(more, "padding", "0 16px 8px")
			aboutExpanded := false
			dom.AddEventListener(more, "click", dom.RegisterCallback(func() {
				aboutExpanded = !aboutExpanded
				if aboutExpanded {
					dom.SetInnerHTML(aboutEl, renderMarkdown(about))
					dom.SetTextContent(more, "show less")
				} else {
					dom.SetInnerHTML(aboutEl, renderMarkdown(about[:300]+"..."))
					dom.SetTextContent(more, "show more")
				}
			}))
			dom.AppendChild(profilePage, more)
		}
	}

	// Tab bar.
	tabBar := dom.CreateElement("div")
	dom.SetStyle(tabBar, "display", "flex")
	dom.SetStyle(tabBar, "gap", "0")
	dom.SetStyle(tabBar, "margin", "0 16px")
	dom.SetStyle(tabBar, "border", "1px solid var(--border)")
	dom.SetStyle(tabBar, "borderRadius", "6px")
	dom.SetStyle(tabBar, "overflow", "hidden")

	profileTabBtns = make(map[string]dom.Element)

	// Unrolled — tinyjs range/loop closure aliasing.
	tabNotes := makeProtoBtn("notes")
	dom.SetStyle(tabNotes, "cursor", "pointer")
	profileTabBtns["notes"] = tabNotes
	tabNotesPK := pk
	dom.AddEventListener(tabNotes, "click", dom.RegisterCallback(func() {
		selectProfileTab("notes", tabNotesPK)
	}))
	dom.AppendChild(tabBar, tabNotes)

	tabFollows := makeProtoBtn("follows")
	dom.SetStyle(tabFollows, "cursor", "pointer")
	profileTabBtns["follows"] = tabFollows
	tabFollowsPK := pk
	dom.AddEventListener(tabFollows, "click", dom.RegisterCallback(func() {
		selectProfileTab("follows", tabFollowsPK)
	}))
	dom.AppendChild(tabBar, tabFollows)

	tabRelays := makeProtoBtn("relays")
	dom.SetStyle(tabRelays, "cursor", "pointer")
	profileTabBtns["relays"] = tabRelays
	tabRelaysPK := pk
	dom.AddEventListener(tabRelays, "click", dom.RegisterCallback(func() {
		selectProfileTab("relays", tabRelaysPK)
	}))
	dom.AppendChild(tabBar, tabRelays)

	tabMutes := makeProtoBtn("mutes")
	dom.SetStyle(tabMutes, "cursor", "pointer")
	profileTabBtns["mutes"] = tabMutes
	tabMutesPK := pk
	dom.AddEventListener(tabMutes, "click", dom.RegisterCallback(func() {
		selectProfileTab("mutes", tabMutesPK)
	}))
	dom.AppendChild(tabBar, tabMutes)

	dom.AppendChild(profilePage, tabBar)

	// Tab content container.
	profileTabContent = dom.CreateElement("div")
	dom.SetStyle(profileTabContent, "padding", "8px 0")
	dom.AppendChild(profilePage, profileTabContent)

	// Restore or default tab.
	profileTab = ""
	if savedTab != "" {
		selectProfileTab(savedTab, pk)
	} else {
		selectProfileTab("notes", pk)
	}

	// Update title.
	if name != "" && activePage == "profile" {
		dom.SetTextContent(pageTitleEl, name)
	}
}

func profileMetaRow(icon, text, link string) dom.Element {
	row := dom.CreateElement("div")
	dom.SetStyle(row, "padding", "4px 0")
	dom.SetStyle(row, "display", "flex")
	dom.SetStyle(row, "alignItems", "center")
	dom.SetStyle(row, "gap", "8px")

	iconEl := dom.CreateElement("span")
	dom.SetTextContent(iconEl, icon)
	dom.AppendChild(row, iconEl)

	if link != "" {
		href := link
		if strIndex(href, "://") < 0 {
			href = "https://" + href
		}
		a := dom.CreateElement("a")
		dom.SetAttribute(a, "href", href)
		dom.SetAttribute(a, "target", "_blank")
		dom.SetAttribute(a, "rel", "noopener")
		dom.SetStyle(a, "color", "var(--accent)")
		dom.SetStyle(a, "wordBreak", "break-all")
		dom.SetTextContent(a, text)
		dom.AppendChild(row, a)
	} else {
		span := dom.CreateElement("span")
		dom.SetStyle(span, "color", "var(--fg)")
		dom.SetTextContent(span, text)
		dom.AppendChild(row, span)
	}
	return row
}

// --- Profile tab functions ---

func closeProfileNoteSub() {
	if activeProfileNoteSub != "" {
		dom.PostToSW("[\"CLOSE\"," + jstr(activeProfileNoteSub) + "]")
		activeProfileNoteSub = ""
	}
}

func selectProfileTab(tab, pk string) {
	if tab == profileTab {
		return
	}
	closeProfileNoteSub()
	profileTab = tab
	clearChildren(profileTabContent)

	for id, btn := range profileTabBtns {
		if id == tab {
			dom.SetStyle(btn, "background", "var(--accent)")
			dom.SetStyle(btn, "color", "#000")
		} else {
			dom.SetStyle(btn, "background", "transparent")
			dom.SetStyle(btn, "color", "var(--fg)")
		}
	}

	switch tab {
	case "notes":
		renderProfileNotes(pk)
	case "follows":
		renderProfileFollows(pk)
	case "relays":
		renderProfileRelays(pk)
	case "mutes":
		renderProfileMutes(pk)
	}
}

func renderProfileNotes(pk string) {
	profileNotesSeen = make(map[string]bool)
	profileSubCounter++
	subID := "pn-" + itoa(profileSubCounter)
	activeProfileNoteSub = subID
	proxyRelays := buildProxy(pk)
	dom.PostToSW(buildProxyMsg(subID,
		"{\"authors\":["+jstr(pk)+"],\"kinds\":[1],\"limit\":20}",
		proxyRelays))
}

func renderProfileNote(ev *nostr.Event) {
	if profileTabContent == 0 || profileTab != "notes" {
		return
	}
	note := dom.CreateElement("div")
	dom.SetStyle(note, "borderBottom", "1px solid var(--border)")
	dom.SetStyle(note, "padding", "12px 16px")

	content := dom.CreateElement("div")
	dom.SetStyle(content, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(content, "fontSize", "14px")
	dom.SetStyle(content, "lineHeight", "1.5")
	dom.SetStyle(content, "wordBreak", "break-word")
	text := ev.Content
	truncated := len(text) > 500
	if truncated {
		text = text[:500] + "..."
	}
	dom.SetInnerHTML(content, renderMarkdown(text))
	dom.AppendChild(note, content)

	if truncated {
		more := dom.CreateElement("span")
		dom.SetTextContent(more, "show more")
		dom.SetStyle(more, "color", "var(--accent)")
		dom.SetStyle(more, "cursor", "pointer")
		dom.SetStyle(more, "fontSize", "13px")
		dom.SetStyle(more, "display", "inline-block")
		dom.SetStyle(more, "marginTop", "4px")
		fullContent := ev.Content
		expanded := false
		dom.AddEventListener(more, "click", dom.RegisterCallback(func() {
			expanded = !expanded
			if expanded {
				dom.SetInnerHTML(content, renderMarkdown(fullContent))
				dom.SetTextContent(more, "show less")
			} else {
				dom.SetInnerHTML(content, renderMarkdown(fullContent[:500]+"..."))
				dom.SetTextContent(more, "show more")
			}
		}))
		dom.AppendChild(note, more)
	}

	// Timestamp.
	if ev.CreatedAt > 0 {
		ts := dom.CreateElement("div")
		dom.SetTextContent(ts, formatTime(ev.CreatedAt))
		dom.SetStyle(ts, "color", "var(--muted)")
		dom.SetStyle(ts, "fontSize", "12px")
		dom.SetStyle(ts, "marginTop", "4px")
		dom.AppendChild(note, ts)
	}

	dom.AppendChild(profileTabContent, note)
}

func renderProfileFollows(pk string) {
	follows, ok := authorFollows[pk]
	if !ok || len(follows) == 0 {
		empty := dom.CreateElement("div")
		dom.SetTextContent(empty, "no follows data")
		dom.SetStyle(empty, "padding", "16px")
		dom.SetStyle(empty, "color", "var(--muted)")
		dom.SetStyle(empty, "fontSize", "13px")
		dom.AppendChild(profileTabContent, empty)
		return
	}

	countEl := dom.CreateElement("div")
	dom.SetTextContent(countEl, itoa(len(follows))+" following")
	dom.SetStyle(countEl, "padding", "8px 16px")
	dom.SetStyle(countEl, "color", "var(--muted)")
	dom.SetStyle(countEl, "fontSize", "12px")
	dom.AppendChild(profileTabContent, countEl)

	for i := 0; i < len(follows); i++ {
		fpk := follows[i]
		row := makeProfileRow(fpk)
		dom.AppendChild(profileTabContent, row)
	}
}

func renderProfileRelays(pk string) {
	relays, ok := authorRelays[pk]
	if !ok || len(relays) == 0 {
		empty := dom.CreateElement("div")
		dom.SetTextContent(empty, "no relay data")
		dom.SetStyle(empty, "padding", "16px")
		dom.SetStyle(empty, "color", "var(--muted)")
		dom.SetStyle(empty, "fontSize", "13px")
		dom.AppendChild(profileTabContent, empty)
		return
	}

	for i := 0; i < len(relays); i++ {
		rURL := relays[i]
		row := dom.CreateElement("div")
		dom.SetStyle(row, "padding", "10px 16px")
		dom.SetStyle(row, "borderBottom", "1px solid var(--border)")
		dom.SetStyle(row, "cursor", "pointer")
		dom.SetStyle(row, "fontSize", "13px")

		urlEl := dom.CreateElement("span")
		dom.SetTextContent(urlEl, rURL)
		dom.SetStyle(urlEl, "color", "var(--accent)")
		dom.AppendChild(row, urlEl)

		clickURL := rURL
		dom.AddEventListener(row, "click", dom.RegisterCallback(func() {
			showRelayInfo(clickURL)
		}))
		dom.AppendChild(profileTabContent, row)
	}
}

func renderProfileMutes(pk string) {
	mutes, ok := authorMutes[pk]
	if !ok || len(mutes) == 0 {
		empty := dom.CreateElement("div")
		dom.SetTextContent(empty, "no mutes data")
		dom.SetStyle(empty, "padding", "16px")
		dom.SetStyle(empty, "color", "var(--muted)")
		dom.SetStyle(empty, "fontSize", "13px")
		dom.AppendChild(profileTabContent, empty)
		return
	}

	countEl := dom.CreateElement("div")
	dom.SetTextContent(countEl, itoa(len(mutes))+" muted")
	dom.SetStyle(countEl, "padding", "8px 16px")
	dom.SetStyle(countEl, "color", "var(--muted)")
	dom.SetStyle(countEl, "fontSize", "12px")
	dom.AppendChild(profileTabContent, countEl)

	for i := 0; i < len(mutes); i++ {
		mpk := mutes[i]
		row := makeProfileRow(mpk)
		dom.AppendChild(profileTabContent, row)
	}
}

func makeProfileRow(pk string) dom.Element {
	row := dom.CreateElement("div")
	dom.SetStyle(row, "display", "flex")
	dom.SetStyle(row, "alignItems", "center")
	dom.SetStyle(row, "gap", "10px")
	dom.SetStyle(row, "padding", "10px 16px")
	dom.SetStyle(row, "borderBottom", "1px solid var(--border)")
	dom.SetStyle(row, "cursor", "pointer")

	av := dom.CreateElement("img")
	dom.SetAttribute(av, "width", "32")
	dom.SetAttribute(av, "height", "32")
	dom.SetStyle(av, "borderRadius", "50%")
	dom.SetStyle(av, "objectFit", "cover")
	dom.SetStyle(av, "flexShrink", "0")
	if pic, ok := authorPics[pk]; ok && pic != "" {
		dom.SetAttribute(av, "src", pic)
	} else {
		dom.SetStyle(av, "display", "none")
	}
	dom.SetAttribute(av, "onerror", "this.style.display='none'")
	dom.AppendChild(row, av)

	nameSpan := dom.CreateElement("span")
	dom.SetStyle(nameSpan, "fontSize", "14px")
	dom.SetStyle(nameSpan, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	if name, ok := authorNames[pk]; ok && name != "" {
		dom.SetTextContent(nameSpan, name)
	} else {
		npub := helpers.EncodeNpub(helpers.HexDecode(pk))
		if len(npub) > 20 {
			dom.SetTextContent(nameSpan, npub[:12]+"..."+npub[len(npub)-4:])
		}
	}
	dom.AppendChild(row, nameSpan)

	rowPK := pk
	dom.AddEventListener(row, "click", dom.RegisterCallback(func() {
		showProfile(rowPK)
	}))

	if _, cached := authorNames[pk]; !cached && !fetchedK0[pk] {
		fetchAuthorProfile(pk)
	}

	return row
}

// --- Relay info page ---

func showRelayInfo(url string) {
	profileViewPK = ""
	closeProfileNoteSub()
	clearChildren(profilePage)

	hdr := dom.CreateElement("div")
	dom.SetStyle(hdr, "display", "flex")
	dom.SetStyle(hdr, "alignItems", "center")
	dom.SetStyle(hdr, "gap", "10px")
	dom.SetStyle(hdr, "padding", "16px")
	dom.SetStyle(hdr, "borderBottom", "1px solid var(--border)")

	backBtn := dom.CreateElement("button")
	dom.SetInnerHTML(backBtn, "&#x2190;")
	dom.SetStyle(backBtn, "background", "none")
	dom.SetStyle(backBtn, "border", "none")
	dom.SetStyle(backBtn, "fontSize", "20px")
	dom.SetStyle(backBtn, "cursor", "pointer")
	dom.SetStyle(backBtn, "color", "var(--fg)")
	dom.SetStyle(backBtn, "padding", "0")
	dom.AddEventListener(backBtn, "click", dom.RegisterCallback(func() {
		switchPage("feed")
	}))
	dom.AppendChild(hdr, backBtn)

	urlEl := dom.CreateElement("span")
	dom.SetTextContent(urlEl, url)
	dom.SetStyle(urlEl, "fontWeight", "bold")
	dom.SetStyle(urlEl, "fontSize", "14px")
	dom.SetStyle(urlEl, "wordBreak", "break-all")
	dom.AppendChild(hdr, urlEl)
	dom.AppendChild(profilePage, hdr)

	loading := dom.CreateElement("div")
	dom.SetTextContent(loading, "loading...")
	dom.SetStyle(loading, "padding", "16px")
	dom.SetStyle(loading, "color", "var(--muted)")
	dom.AppendChild(profilePage, loading)

	activePage = ""
	switchPage("profile")
	dom.SetTextContent(pageTitleEl, "relay info")

	// Convert wss→https for NIP-11 HTTP fetch.
	httpURL := url
	if len(httpURL) > 6 && httpURL[:6] == "wss://" {
		httpURL = "https://" + httpURL[6:]
	} else if len(httpURL) > 5 && httpURL[:5] == "ws://" {
		httpURL = "http://" + httpURL[5:]
	}

	dom.FetchRelayInfo(httpURL, func(body string) {
		dom.RemoveChild(profilePage, loading)
		if body == "" {
			errEl := dom.CreateElement("div")
			dom.SetTextContent(errEl, "failed to fetch relay info")
			dom.SetStyle(errEl, "padding", "16px")
			dom.SetStyle(errEl, "color", "#e55")
			dom.AppendChild(profilePage, errEl)
			return
		}
		renderRelayInfoBody(body)
	})
}

func renderRelayInfoBody(body string) {
	container := dom.CreateElement("div")
	dom.SetStyle(container, "padding", "16px")

	name := helpers.JsonGetString(body, "name")
	desc := helpers.JsonGetString(body, "description")
	pk := helpers.JsonGetString(body, "pubkey")
	contact := helpers.JsonGetString(body, "contact")
	software := helpers.JsonGetString(body, "software")
	ver := helpers.JsonGetString(body, "version")

	if name != "" {
		el := dom.CreateElement("div")
		dom.SetTextContent(el, name)
		dom.SetStyle(el, "fontSize", "20px")
		dom.SetStyle(el, "fontWeight", "bold")
		dom.SetStyle(el, "marginBottom", "8px")
		dom.SetStyle(el, "fontFamily", "system-ui, sans-serif")
		dom.AppendChild(container, el)
	}

	if desc != "" {
		el := dom.CreateElement("div")
		dom.SetInnerHTML(el, renderMarkdown(desc))
		dom.SetStyle(el, "fontSize", "14px")
		dom.SetStyle(el, "lineHeight", "1.5")
		dom.SetStyle(el, "marginBottom", "12px")
		dom.SetStyle(el, "wordBreak", "break-word")
		dom.AppendChild(container, el)
	}

	if contact != "" {
		dom.AppendChild(container, profileMetaRow("@", contact, ""))
	}
	if pk != "" {
		npub := helpers.EncodeNpub(helpers.HexDecode(pk))
		short := npub
		if len(short) > 20 {
			short = short[:16] + "..." + short[len(short)-8:]
		}
		dom.AppendChild(container, profileMetaRow("pk", short, ""))
	}
	if software != "" {
		label := software
		if ver != "" {
			label += " " + ver
		}
		dom.AppendChild(container, profileMetaRow("sw", label, ""))
	}

	dom.AppendChild(profilePage, container)
}

// --- Messaging ---

func relayURLsJSON() string {
	msg := "["
	for i, url := range relayURLs {
		if i > 0 {
			msg += ","
		}
		msg += jstr(url)
	}
	return msg + "]"
}

func formatTime(ts int64) string {
	if ts == 0 {
		return ""
	}
	secs := ts % 86400
	h := itoa(int(secs / 3600))
	m := itoa(int((secs % 3600) / 60))
	if len(h) < 2 {
		h = "0" + h
	}
	if len(m) < 2 {
		m = "0" + m
	}
	return h + ":" + m
}

func initMessaging() {
	// Request conversation list.
	dom.PostToSW("[\"DM_LIST\"]")

	// Subscribe to incoming DMs on relays.
	dom.PostToSW("[\"DM_SUB\"," + relayURLsJSON() + "]")

	// Init marmot if not already done.
	if !marmotInited {
		marmotInited = true
		dom.PostToSW("[\"MLS_INIT\"," + relayURLsJSON() + "]")
		dom.PostToSW("[\"MLS_PUBLISH_KP\"," + relayURLsJSON() + "]")
	}
}

func renderConversationList(listJSON string) {
	if msgView != "list" {
		return
	}
	clearChildren(msgListContainer)

	// "New chat" button.
	newBtn := dom.CreateElement("button")
	dom.SetTextContent(newBtn, "+ new chat")
	dom.SetStyle(newBtn, "display", "block")
	dom.SetStyle(newBtn, "width", "100%")
	dom.SetStyle(newBtn, "padding", "10px")
	dom.SetStyle(newBtn, "marginBottom", "8px")
	dom.SetStyle(newBtn, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(newBtn, "fontSize", "13px")
	dom.SetStyle(newBtn, "background", "var(--bg2)")
	dom.SetStyle(newBtn, "border", "1px solid var(--border)")
	dom.SetStyle(newBtn, "borderRadius", "6px")
	dom.SetStyle(newBtn, "color", "var(--accent)")
	dom.SetStyle(newBtn, "cursor", "pointer")
	dom.SetStyle(newBtn, "textAlign", "left")
	dom.AddEventListener(newBtn, "click", dom.RegisterCallback(func() {
		showNewChatInput()
	}))
	dom.AppendChild(msgListContainer, newBtn)

	// Parse the list JSON array: [{peer,lastMessage,lastTs,from}, ...]
	if listJSON == "" || listJSON == "[]" {
		empty := dom.CreateElement("div")
		dom.SetStyle(empty, "color", "var(--muted)")
		dom.SetStyle(empty, "textAlign", "center")
		dom.SetStyle(empty, "marginTop", "48px")
		dom.SetTextContent(empty, "no conversations yet")
		dom.AppendChild(msgListContainer, empty)
		return
	}

	// Walk the JSON array manually — each element is an object.
	i := 0
	for i < len(listJSON) && listJSON[i] != '[' {
		i++
	}
	i++ // skip '['
	for i < len(listJSON) {
		// Find next object.
		for i < len(listJSON) && listJSON[i] != '{' {
			if listJSON[i] == ']' {
				return
			}
			i++
		}
		if i >= len(listJSON) {
			break
		}
		// Extract the object.
		objStart := i
		depth := 0
		for i < len(listJSON) {
			if listJSON[i] == '{' {
				depth++
			} else if listJSON[i] == '}' {
				depth--
				if depth == 0 {
					i++
					break
				}
			} else if listJSON[i] == '"' {
				i++
				for i < len(listJSON) && listJSON[i] != '"' {
					if listJSON[i] == '\\' {
						i++
					}
					i++
				}
			}
			i++
		}
		obj := listJSON[objStart:i]

		peer := helpers.JsonGetString(obj, "peer")
		lastMsg := helpers.JsonGetString(obj, "lastMessage")
		lastTs := jsonGetNum(obj, "lastTs")
		if peer == "" {
			continue
		}

		renderConversationRow(peer, lastMsg, lastTs)
	}
}

func renderConversationRow(peer, lastMsg string, lastTs int64) {
	row := dom.CreateElement("div")
	dom.SetStyle(row, "display", "flex")
	dom.SetStyle(row, "alignItems", "center")
	dom.SetStyle(row, "gap", "10px")
	dom.SetStyle(row, "padding", "10px 4px")
	dom.SetStyle(row, "borderBottom", "1px solid var(--border)")
	dom.SetStyle(row, "cursor", "pointer")

	// Avatar.
	av := dom.CreateElement("img")
	dom.SetAttribute(av, "width", "32")
	dom.SetAttribute(av, "height", "32")
	dom.SetStyle(av, "borderRadius", "50%")
	dom.SetStyle(av, "objectFit", "cover")
	dom.SetStyle(av, "flexShrink", "0")
	if pic, ok := authorPics[peer]; ok && pic != "" {
		dom.SetAttribute(av, "src", pic)
	} else {
		dom.SetStyle(av, "background", "var(--bg2)")
	}
	dom.SetAttribute(av, "onerror", "this.style.display='none'")
	dom.AppendChild(row, av)

	// Name + preview column.
	col := dom.CreateElement("div")
	dom.SetStyle(col, "flex", "1")
	dom.SetStyle(col, "minWidth", "0")

	nameSpan := dom.CreateElement("div")
	dom.SetStyle(nameSpan, "fontSize", "14px")
	dom.SetStyle(nameSpan, "fontWeight", "bold")
	dom.SetStyle(nameSpan, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(nameSpan, "overflow", "hidden")
	dom.SetStyle(nameSpan, "textOverflow", "ellipsis")
	dom.SetStyle(nameSpan, "whiteSpace", "nowrap")
	if name, ok := authorNames[peer]; ok && name != "" {
		dom.SetTextContent(nameSpan, name)
	} else {
		npub := helpers.EncodeNpub(helpers.HexDecode(peer))
		if len(npub) > 20 {
			dom.SetTextContent(nameSpan, npub[:12]+"..."+npub[len(npub)-4:])
		} else {
			dom.SetTextContent(nameSpan, npub)
		}
	}
	dom.AppendChild(col, nameSpan)

	preview := dom.CreateElement("div")
	dom.SetStyle(preview, "fontSize", "12px")
	dom.SetStyle(preview, "color", "var(--muted)")
	dom.SetStyle(preview, "overflow", "hidden")
	dom.SetStyle(preview, "textOverflow", "ellipsis")
	dom.SetStyle(preview, "whiteSpace", "nowrap")
	if len(lastMsg) > 80 {
		lastMsg = lastMsg[:80] + "..."
	}
	dom.SetTextContent(preview, lastMsg)
	dom.AppendChild(col, preview)
	dom.AppendChild(row, col)

	// Timestamp.
	if lastTs > 0 {
		tsSpan := dom.CreateElement("span")
		dom.SetStyle(tsSpan, "fontSize", "11px")
		dom.SetStyle(tsSpan, "color", "var(--muted)")
		dom.SetStyle(tsSpan, "flexShrink", "0")
		dom.SetTextContent(tsSpan, formatTime(lastTs))
		dom.AppendChild(row, tsSpan)
	}

	rowPeer := peer
	dom.AddEventListener(row, "click", dom.RegisterCallback(func() {
		openThread(rowPeer)
	}))

	// Lazy profile fetch.
	if _, cached := authorNames[peer]; !cached && !fetchedK0[peer] {
		fetchAuthorProfile(peer)
	}

	dom.AppendChild(msgListContainer, row)
}

func showNewChatInput() {
	// Check if input row already exists (first child after button).
	fc := dom.FirstChild(msgListContainer)
	if fc != 0 {
		ns := dom.NextSibling(fc)
		if ns != 0 {
			tag := dom.GetProperty(ns, "tagName")
			if tag == "DIV" {
				id := dom.GetProperty(ns, "id")
				if id == "new-chat-row" {
					return // already showing
				}
			}
		}
	}

	inputRow := dom.CreateElement("div")
	dom.SetAttribute(inputRow, "id", "new-chat-row")
	dom.SetStyle(inputRow, "display", "flex")
	dom.SetStyle(inputRow, "gap", "8px")
	dom.SetStyle(inputRow, "marginBottom", "8px")

	inp := dom.CreateElement("input")
	dom.SetAttribute(inp, "type", "text")
	dom.SetAttribute(inp, "placeholder", "npub or hex pubkey")
	dom.SetStyle(inp, "flex", "1")
	dom.SetStyle(inp, "padding", "8px")
	dom.SetStyle(inp, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(inp, "fontSize", "12px")
	dom.SetStyle(inp, "background", "var(--bg)")
	dom.SetStyle(inp, "border", "1px solid var(--border)")
	dom.SetStyle(inp, "borderRadius", "4px")
	dom.SetStyle(inp, "color", "var(--fg)")

	goBtn := dom.CreateElement("button")
	dom.SetTextContent(goBtn, "go")
	dom.SetStyle(goBtn, "padding", "8px 16px")
	dom.SetStyle(goBtn, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(goBtn, "fontSize", "12px")
	dom.SetStyle(goBtn, "background", "var(--accent)")
	dom.SetStyle(goBtn, "color", "#000")
	dom.SetStyle(goBtn, "border", "none")
	dom.SetStyle(goBtn, "borderRadius", "4px")
	dom.SetStyle(goBtn, "cursor", "pointer")

	submitNewChat := func() {
		val := dom.GetProperty(inp, "value")
		if val == "" {
			return
		}
		var hexPK string
		if len(val) == 64 {
			// Assume hex pubkey.
			hexPK = val
		} else if len(val) > 4 && val[:4] == "npub" {
			decoded := helpers.DecodeNpub(val)
			if decoded == nil {
				return
			}
			hexPK = helpers.HexEncode(decoded)
		} else {
			return
		}
		openThread(hexPK)
	}

	dom.AddEventListener(goBtn, "click", dom.RegisterCallback(submitNewChat))
	// Enter key triggers the "go" button via inline handler.
	dom.SetAttribute(inp, "onkeydown", "if(event.key==='Enter'){event.preventDefault();this.nextSibling.click()}")

	dom.AppendChild(inputRow, inp)
	dom.AppendChild(inputRow, goBtn)

	// Insert after the "new chat" button.
	btn := dom.FirstChild(msgListContainer)
	if btn != 0 {
		ns := dom.NextSibling(btn)
		if ns != 0 {
			dom.InsertBefore(msgListContainer, inputRow, ns)
		} else {
			dom.AppendChild(msgListContainer, inputRow)
		}
	} else {
		dom.AppendChild(msgListContainer, inputRow)
	}
}

func openThread(peer string) {
	msgCurrentPeer = peer
	msgView = "thread"

	// Hide list, show thread.
	dom.SetStyle(msgListContainer, "display", "none")
	dom.SetStyle(msgThreadContainer, "display", "flex")

	// Build thread UI.
	clearChildren(msgThreadContainer)

	// Header: back + avatar + name.
	hdr := dom.CreateElement("div")
	dom.SetStyle(hdr, "display", "flex")
	dom.SetStyle(hdr, "alignItems", "center")
	dom.SetStyle(hdr, "gap", "10px")
	dom.SetStyle(hdr, "padding", "12px 16px")
	dom.SetStyle(hdr, "borderBottom", "1px solid var(--border)")
	dom.SetStyle(hdr, "flexShrink", "0")

	backBtn := dom.CreateElement("button")
	dom.SetInnerHTML(backBtn, "&#x2190;") // ←
	dom.SetStyle(backBtn, "background", "none")
	dom.SetStyle(backBtn, "border", "none")
	dom.SetStyle(backBtn, "fontSize", "20px")
	dom.SetStyle(backBtn, "cursor", "pointer")
	dom.SetStyle(backBtn, "color", "var(--fg)")
	dom.SetStyle(backBtn, "padding", "0")
	dom.AddEventListener(backBtn, "click", dom.RegisterCallback(func() {
		closeThread()
	}))
	dom.AppendChild(hdr, backBtn)

	av := dom.CreateElement("img")
	dom.SetAttribute(av, "width", "28")
	dom.SetAttribute(av, "height", "28")
	dom.SetStyle(av, "borderRadius", "50%")
	dom.SetStyle(av, "objectFit", "cover")
	dom.SetStyle(av, "flexShrink", "0")
	if pic, ok := authorPics[peer]; ok && pic != "" {
		dom.SetAttribute(av, "src", pic)
	} else {
		dom.SetStyle(av, "display", "none")
	}
	dom.SetAttribute(av, "onerror", "this.style.display='none'")
	dom.AppendChild(hdr, av)

	nameSpan := dom.CreateElement("span")
	dom.SetStyle(nameSpan, "fontSize", "15px")
	dom.SetStyle(nameSpan, "fontWeight", "bold")
	dom.SetStyle(nameSpan, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	if name, ok := authorNames[peer]; ok && name != "" {
		dom.SetTextContent(nameSpan, name)
	} else {
		npub := helpers.EncodeNpub(helpers.HexDecode(peer))
		if len(npub) > 20 {
			dom.SetTextContent(nameSpan, npub[:12]+"..."+npub[len(npub)-4:])
		}
	}
	dom.AppendChild(hdr, nameSpan)
	dom.AppendChild(msgThreadContainer, hdr)

	// Message area.
	msgThreadMessages = dom.CreateElement("div")
	dom.SetStyle(msgThreadMessages, "flex", "1")
	dom.SetStyle(msgThreadMessages, "overflowY", "auto")
	dom.SetStyle(msgThreadMessages, "padding", "12px 16px")
	dom.AppendChild(msgThreadContainer, msgThreadMessages)

	// Compose area.
	compose := dom.CreateElement("div")
	dom.SetStyle(compose, "display", "flex")
	dom.SetStyle(compose, "gap", "8px")
	dom.SetStyle(compose, "padding", "8px 16px")
	dom.SetStyle(compose, "borderTop", "1px solid var(--border)")
	dom.SetStyle(compose, "flexShrink", "0")

	msgComposeInput = dom.CreateElement("textarea")
	dom.SetAttribute(msgComposeInput, "rows", "1")
	dom.SetAttribute(msgComposeInput, "placeholder", "message...")
	dom.SetStyle(msgComposeInput, "flex", "1")
	dom.SetStyle(msgComposeInput, "padding", "8px")
	dom.SetStyle(msgComposeInput, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(msgComposeInput, "fontSize", "13px")
	dom.SetStyle(msgComposeInput, "background", "var(--bg)")
	dom.SetStyle(msgComposeInput, "border", "1px solid var(--border)")
	dom.SetStyle(msgComposeInput, "borderRadius", "4px")
	dom.SetStyle(msgComposeInput, "color", "var(--fg)")
	dom.SetStyle(msgComposeInput, "resize", "none")
	dom.SetAttribute(msgComposeInput, "onkeydown", "if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();this.nextSibling.click()}")
	dom.AppendChild(compose, msgComposeInput)

	sendBtn := dom.CreateElement("button")
	dom.SetTextContent(sendBtn, "send")
	dom.SetStyle(sendBtn, "padding", "8px 16px")
	dom.SetStyle(sendBtn, "fontFamily", "'Fira Code', monospace")
	dom.SetStyle(sendBtn, "fontSize", "13px")
	dom.SetStyle(sendBtn, "background", "var(--accent)")
	dom.SetStyle(sendBtn, "color", "#000")
	dom.SetStyle(sendBtn, "border", "none")
	dom.SetStyle(sendBtn, "borderRadius", "4px")
	dom.SetStyle(sendBtn, "cursor", "pointer")
	dom.SetStyle(sendBtn, "alignSelf", "flex-end")
	dom.AddEventListener(sendBtn, "click", dom.RegisterCallback(func() {
		sendMessage()
	}))
	dom.AppendChild(compose, sendBtn)

	dom.AppendChild(msgThreadContainer, compose)

	// Fetch profile if needed.
	if _, cached := authorNames[peer]; !cached && !fetchedK0[peer] {
		fetchAuthorProfile(peer)
	}

	// Request history.
	dom.PostToSW("[\"DM_HISTORY\"," + jstr(peer) + ",50,0]")
}

func closeThread() {
	msgCurrentPeer = ""
	msgView = "list"

	dom.SetStyle(msgThreadContainer, "display", "none")
	dom.SetStyle(msgListContainer, "display", "block")

	// Refresh list.
	dom.PostToSW("[\"DM_LIST\"]")
}

func renderThreadMessages(peer, msgsJSON string) {
	if peer != msgCurrentPeer {
		return
	}
	if msgsJSON == "" || msgsJSON == "[]" {
		return
	}

	// Parse messages array — each element is a DMRecord object.
	// IDB returns newest-first; collect then reverse for oldest-at-top.
	type dmMsg struct {
		from    string
		content string
		ts      int64
	}
	var msgs []dmMsg

	i := 0
	for i < len(msgsJSON) && msgsJSON[i] != '[' {
		i++
	}
	i++
	for i < len(msgsJSON) {
		for i < len(msgsJSON) && msgsJSON[i] != '{' {
			if msgsJSON[i] == ']' {
				goto done
			}
			i++
		}
		if i >= len(msgsJSON) {
			break
		}
		objStart := i
		depth := 0
		for i < len(msgsJSON) {
			if msgsJSON[i] == '{' {
				depth++
			} else if msgsJSON[i] == '}' {
				depth--
				if depth == 0 {
					i++
					break
				}
			} else if msgsJSON[i] == '"' {
				i++
				for i < len(msgsJSON) && msgsJSON[i] != '"' {
					if msgsJSON[i] == '\\' {
						i++
					}
					i++
				}
			}
			i++
		}
		obj := msgsJSON[objStart:i]

		from := helpers.JsonGetString(obj, "from")
		content := helpers.JsonGetString(obj, "content")
		ts := jsonGetNum(obj, "created_at")
		msgs = append(msgs, dmMsg{from, content, ts})
	}
done:

	// Reverse for oldest-first.
	for l, r := 0, len(msgs)-1; l < r; l, r = l+1, r-1 {
		msgs[l], msgs[r] = msgs[r], msgs[l]
	}

	clearChildren(msgThreadMessages)
	for _, m := range msgs {
		appendBubble(m.from, m.content, m.ts)
	}
	scrollToBottom()
}

func appendBubble(from, content string, ts int64) {
	isSent := from == pubhex

	wrap := dom.CreateElement("div")
	dom.SetStyle(wrap, "display", "flex")
	dom.SetStyle(wrap, "marginBottom", "6px")
	if isSent {
		dom.SetStyle(wrap, "justifyContent", "flex-end")
	}

	bubble := dom.CreateElement("div")
	dom.SetStyle(bubble, "maxWidth", "75%")
	dom.SetStyle(bubble, "padding", "8px 12px")
	dom.SetStyle(bubble, "borderRadius", "12px")
	dom.SetStyle(bubble, "fontSize", "14px")
	dom.SetStyle(bubble, "fontFamily", "system-ui, sans-serif, 'Noto Color Emoji'")
	dom.SetStyle(bubble, "lineHeight", "1.4")
	dom.SetStyle(bubble, "wordBreak", "break-word")
	if isSent {
		dom.SetStyle(bubble, "background", "var(--accent)")
		dom.SetStyle(bubble, "color", "#000")
	} else {
		dom.SetStyle(bubble, "background", "var(--bg2)")
		dom.SetStyle(bubble, "color", "var(--fg)")
	}
	dom.SetInnerHTML(bubble, renderMarkdown(content))

	// Timestamp below bubble.
	tsEl := dom.CreateElement("div")
	dom.SetStyle(tsEl, "fontSize", "10px")
	dom.SetStyle(tsEl, "color", "var(--muted)")
	dom.SetStyle(tsEl, "marginTop", "2px")
	if isSent {
		dom.SetStyle(tsEl, "textAlign", "right")
	}
	dom.SetTextContent(tsEl, formatTime(ts))

	outer := dom.CreateElement("div")
	dom.AppendChild(outer, bubble)
	dom.AppendChild(outer, tsEl)
	dom.AppendChild(wrap, outer)
	dom.AppendChild(msgThreadMessages, wrap)
}

func scrollToBottom() {
	dom.SetProperty(msgThreadMessages, "scrollTop", "999999")
}

func sendMessage() {
	content := dom.GetProperty(msgComposeInput, "value")
	if content == "" || msgCurrentPeer == "" {
		return
	}

	// Clear input.
	dom.SetProperty(msgComposeInput, "value", "")

	// Route by protocol.
	if msgProtocol == "marmot" {
		dom.PostToSW("[\"MLS_SEND\"," + jstr(msgCurrentPeer) + "," + jstr(content) + "]")
	} else {
		dom.PostToSW("[\"SEND_DM\"," + jstr(msgCurrentPeer) + "," + jstr(content) + "," + relayURLsJSON() + "]")
	}

	// Optimistic render (ts=0 — timestamp not shown for "just sent").
	appendBubble(pubhex, content, 0)
	scrollToBottom()
}

func handleDMReceived(dmJSON string) {
	peer := helpers.JsonGetString(dmJSON, "peer")
	from := helpers.JsonGetString(dmJSON, "from")
	content := helpers.JsonGetString(dmJSON, "content")
	ts := jsonGetNum(dmJSON, "created_at")

	if msgView == "thread" && peer == msgCurrentPeer {
		// Don't double-render our own sent messages (already optimistic).
		if from == pubhex {
			return
		}
		appendBubble(from, content, ts)
		scrollToBottom()
	} else if msgView == "list" {
		// Refresh conversation list.
		dom.PostToSW("[\"DM_LIST\"]")
	}
}

// --- Logout ---

func doLogout() {
	// Tell SW to clean up.
	dom.PostToSW("[\"CLOSE\",\"prof\"]")
	dom.PostToSW("[\"CLOSE\",\"feed\"]")
	dom.PostToSW("[\"CLEAR_KEY\"]")

	pubkey = nil
	pubhex = ""
	profileName = ""
	profilePic = ""
	profileTs = 0
	eventCount = 0
	popoverOpen = false
	marmotInited = false
	msgCurrentPeer = ""
	msgView = "list"

	// Reset relay tracking.
	relayURLs = nil
	relayDots = nil
	relayLabels = nil
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

// jsonGetNum extracts a numeric value for a given key from a JSON object.
func jsonGetNum(s, key string) int64 {
	needle := "\"" + key + "\":"
	idx := strIndex(s, needle)
	if idx < 0 {
		return 0
	}
	idx += len(needle)
	// Skip whitespace.
	for idx < len(s) && (s[idx] == ' ' || s[idx] == '\t') {
		idx++
	}
	if idx >= len(s) {
		return 0
	}
	var n int64
	for idx < len(s) && s[idx] >= '0' && s[idx] <= '9' {
		n = n*10 + int64(s[idx]-'0')
		idx++
	}
	return n
}

// jsonEsc escapes a string for embedding in a JSON value.
func jsonEsc(s string) string {
	s = strReplace(s, "\\", "\\\\")
	s = strReplace(s, "\"", "\\\"")
	s = strReplace(s, "\n", "\\n")
	s = strReplace(s, "\r", "\\r")
	s = strReplace(s, "\t", "\\t")
	return s
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

// normalizeURL strips trailing slashes and lowercases the scheme+host.
func normalizeURL(u string) string {
	for len(u) > 0 && u[len(u)-1] == '/' {
		u = u[:len(u)-1]
	}
	// Lowercase scheme and host (before first / after ://).
	if len(u) > 6 && u[:6] == "wss://" {
		rest := u[6:]
		slash := strIndex(rest, "/")
		if slash < 0 {
			return u[:6] + toLower(rest)
		}
		return u[:6] + toLower(rest[:slash]) + rest[slash:]
	}
	if len(u) > 5 && u[:5] == "ws://" {
		rest := u[5:]
		slash := strIndex(rest, "/")
		if slash < 0 {
			return u[:5] + toLower(rest)
		}
		return u[:5] + toLower(rest[:slash]) + rest[slash:]
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
