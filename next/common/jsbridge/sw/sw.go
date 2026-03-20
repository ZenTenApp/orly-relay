package sw

// Service Worker jsbridge — lifecycle, cache, clients, fetch, SSE.

// Event is an opaque handle to a service worker event.
type Event int

// Client is an opaque handle to a service worker client (window/tab).
type Client int

// Cache is an opaque handle to a CacheStorage cache.
type Cache int

// Response is an opaque handle to a fetch Response.
type Response int

// SSE is an opaque handle to an EventSource connection.
type SSE int

// --- Lifecycle ---

// OnInstall registers the install handler. fn receives the event handle.
func OnInstall(fn func(Event)) { panic("jsbridge") }

// OnActivate registers the activate handler.
func OnActivate(fn func(Event)) { panic("jsbridge") }

// OnFetch registers the fetch handler.
func OnFetch(fn func(Event)) { panic("jsbridge") }

// OnMessage registers the message handler.
func OnMessage(fn func(Event)) { panic("jsbridge") }

// --- Event methods ---

// WaitUntil extends the event lifetime until done is called.
// Call done() when your async work is complete.
func WaitUntil(event Event, fn func(done func())) { panic("jsbridge") }

// RespondWith provides a response for a fetch event.
func RespondWith(event Event, resp Response) { panic("jsbridge") }

// RespondWithNetwork lets the fetch fall through to the network.
func RespondWithNetwork(event Event) { panic("jsbridge") }

// RespondWithCacheFirst tries cache, falls back to network.
// Must be called synchronously from the fetch handler.
func RespondWithCacheFirst(event Event) { panic("jsbridge") }

// GetRequestURL returns the request URL from a fetch event.
func GetRequestURL(event Event) string { panic("jsbridge") }

// GetRequestPath returns just the pathname from a fetch event.
func GetRequestPath(event Event) string { panic("jsbridge") }

// GetMessageData returns the string data from a message event.
func GetMessageData(event Event) string { panic("jsbridge") }

// --- SW globals ---

// SkipWaiting forces the waiting service worker to become active.
func SkipWaiting() { panic("jsbridge") }

// ClaimClients takes control of all clients. Calls done when complete.
func ClaimClients(done func()) { panic("jsbridge") }

// MatchClients gets all window clients. Calls fn with each client.
func MatchClients(fn func(Client)) { panic("jsbridge") }

// PostMessage sends a message to a client.
func PostMessage(client Client, msg string) { panic("jsbridge") }

// Navigate navigates a client to a URL.
func Navigate(client Client, url string) { panic("jsbridge") }

// --- Cache ---

// CacheOpen opens a named cache. Calls fn with the cache handle.
func CacheOpen(name string, fn func(Cache)) { panic("jsbridge") }

// CacheAddAll caches all the given URLs. Calls done when complete.
func CacheAddAll(cache Cache, urls []string, done func()) { panic("jsbridge") }

// CachePut stores a response in the cache. Calls done when complete.
func CachePut(cache Cache, url string, resp Response, done func()) { panic("jsbridge") }

// CacheMatch looks up a URL in all caches. Calls fn with response (0 if miss).
func CacheMatch(url string, fn func(Response)) { panic("jsbridge") }

// CacheDelete deletes a named cache. Calls done when complete.
func CacheDelete(name string, done func()) { panic("jsbridge") }

// --- Fetch ---

// Fetch fetches a URL. Calls fn with (response, ok).
func Fetch(url string, fn func(Response, bool)) { panic("jsbridge") }

// ResponseOK returns whether the response status is 200-299.
func ResponseOK(resp Response) bool { panic("jsbridge") }

// --- SSE ---

// SSEConnect opens an EventSource. Calls onMessage with each data string.
func SSEConnect(url string, onMessage func(string)) SSE { panic("jsbridge") }

// SSEClose closes an EventSource.
func SSEClose(id SSE) { panic("jsbridge") }

// --- Logging ---

// Log writes to console.log in the SW context.
func Log(msg string) { panic("jsbridge") }
