package dom

// Element is an opaque handle to a browser DOM element.
type Element int

// Body returns a handle to document.body.
func Body() Element { panic("jsbridge") }

// CreateElement creates a new DOM element with the given tag.
func CreateElement(tag string) Element { panic("jsbridge") }

// CreateTextNode creates a text node.
func CreateTextNode(text string) Element { panic("jsbridge") }

// GetElementById finds an element by its ID attribute.
func GetElementById(id string) Element { panic("jsbridge") }

// QuerySelector finds the first element matching a CSS selector.
func QuerySelector(sel string) Element { panic("jsbridge") }

// AppendChild adds a child element to a parent.
func AppendChild(parent, child Element) { panic("jsbridge") }

// RemoveChild removes a child element from a parent.
func RemoveChild(parent, child Element) { panic("jsbridge") }

// SetAttribute sets an attribute on an element.
func SetAttribute(el Element, name, value string) { panic("jsbridge") }

// SetTextContent sets the text content of an element.
func SetTextContent(el Element, text string) { panic("jsbridge") }

// SetInnerHTML sets the inner HTML of an element.
func SetInnerHTML(el Element, html string) { panic("jsbridge") }

// SetStyle sets a CSS style property on an element.
func SetStyle(el Element, prop, value string) { panic("jsbridge") }

// AddClass adds a CSS class to an element.
func AddClass(el Element, cls string) { panic("jsbridge") }

// RemoveClass removes a CSS class from an element.
func RemoveClass(el Element, cls string) { panic("jsbridge") }

// SetProperty sets a JS property on an element.
func SetProperty(el Element, prop, value string) { panic("jsbridge") }

// GetProperty gets a JS property from an element as a string.
func GetProperty(el Element, prop string) string { panic("jsbridge") }

// RegisterCallback registers a Go function as a JS event callback.
// Returns a callback ID for use with AddEventListener.
func RegisterCallback(fn func()) int { panic("jsbridge") }

// ReleaseCallback releases a registered callback.
func ReleaseCallback(id int) { panic("jsbridge") }

// AddEventListener adds an event listener. callbackId is from RegisterCallback.
func AddEventListener(el Element, event string, callbackId int) { panic("jsbridge") }

// RemoveEventListener removes an event listener.
func RemoveEventListener(el Element, event string, callbackId int) { panic("jsbridge") }

// RequestAnimationFrame schedules a function to run before the next repaint.
func RequestAnimationFrame(fn func()) { panic("jsbridge") }

// SetTimeout schedules a function to run after ms milliseconds. Returns timer ID.
func SetTimeout(fn func(), ms int) int { panic("jsbridge") }

// FirstChild returns the first child element, or 0 if none.
func FirstChild(el Element) Element { panic("jsbridge") }

// NextSibling returns the next sibling element, or 0 if none.
func NextSibling(el Element) Element { panic("jsbridge") }

// InsertBefore inserts newChild before refChild in parent.
func InsertBefore(parent, newChild, refChild Element) { panic("jsbridge") }

// ReleaseElement releases a DOM element handle.
func ReleaseElement(el Element) { panic("jsbridge") }

// FetchText fetches a URL and calls fn with the response text.
func FetchText(url string, fn func(string)) { panic("jsbridge") }

// FetchRelayInfo fetches NIP-11 relay info document from URL. Adds Accept header.
func FetchRelayInfo(url string, fn func(string)) { panic("jsbridge") }

// ConsoleLog logs a message to the browser console.
func ConsoleLog(msg string) { panic("jsbridge") }
