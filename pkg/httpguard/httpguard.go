// Package httpguard provides application-level HTTP protection: bot User-Agent
// blocking and per-IP rate limiting. Designed for environments where the reverse
// proxy (e.g., Cloudron's nginx) cannot be customized.
package httpguard

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lol.mleku.dev/log"
)

// blockedBots is the list of User-Agent substrings to block. Matched
// case-insensitively. Sourced from relay.orly.dev Caddy config.
var blockedBots = []string{
	"semrushbot",
	"ahrefsbot",
	"mj12bot",
	"dotbot",
	"petalbot",
	"blexbot",
	"dataforseobot",
	"amazonbot",
	"meta-externalagent",
	"bytespider",
	"gptbot",
	"claudebot",
	"ccbot",
	"facebookbot",
}

// Config holds Guard configuration.
type Config struct {
	Enabled      bool
	BotBlock     bool
	RPM          int // HTTP requests per minute per IP
	WSPerMin     int // WebSocket upgrades per minute per IP
	IPBlacklist  []string
}

// Guard is the HTTP guard middleware.
type Guard struct {
	cfg     Config
	clients sync.Map // map[string]*clientState

	// cleanup tracking
	stopCleanup chan struct{}
}

type clientState struct {
	httpTokens  atomic.Int64
	wsTokens    atomic.Int64
	lastSeen    atomic.Int64 // unix seconds
}

const (
	cleanupInterval = 5 * time.Minute
	idleEvictTime   = 10 * time.Minute
)

// New creates a new Guard. Starts a background cleanup goroutine.
func New(cfg Config) *Guard {
	if cfg.RPM <= 0 {
		cfg.RPM = 120
	}
	if cfg.WSPerMin <= 0 {
		cfg.WSPerMin = 10
	}

	g := &Guard{
		cfg:         cfg,
		stopCleanup: make(chan struct{}),
	}

	go g.cleanupLoop()
	go g.refillLoop()

	return g
}

// Stop shuts down the cleanup goroutine.
func (g *Guard) Stop() {
	close(g.stopCleanup)
}

// Allow checks whether the request should be allowed. If blocked, it writes
// the appropriate HTTP response (403 or 429) and returns false. If allowed,
// returns true without touching the ResponseWriter.
func (g *Guard) Allow(w http.ResponseWriter, r *http.Request) bool {
	if !g.cfg.Enabled {
		return true
	}

	ip := extractIP(r)

	// IP blacklist
	for _, blocked := range g.cfg.IPBlacklist {
		if strings.HasPrefix(ip, blocked) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return false
		}
	}

	// Bot User-Agent blocking
	if g.cfg.BotBlock {
		ua := strings.ToLower(r.Header.Get("User-Agent"))
		for _, bot := range blockedBots {
			if strings.Contains(ua, bot) {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return false
			}
		}
	}

	// Rate limiting
	cs := g.getOrCreate(ip)
	now := time.Now().Unix()
	cs.lastSeen.Store(now)

	isWS := isWebSocketUpgrade(r)
	if isWS {
		if cs.wsTokens.Add(-1) < 0 {
			cs.wsTokens.Add(1) // restore
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return false
		}
	}

	if cs.httpTokens.Add(-1) < 0 {
		cs.httpTokens.Add(1) // restore
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return false
	}

	return true
}

func (g *Guard) getOrCreate(ip string) *clientState {
	if val, ok := g.clients.Load(ip); ok {
		return val.(*clientState)
	}
	cs := &clientState{}
	cs.httpTokens.Store(int64(g.cfg.RPM))
	cs.wsTokens.Store(int64(g.cfg.WSPerMin))
	actual, _ := g.clients.LoadOrStore(ip, cs)
	return actual.(*clientState)
}

// refillLoop refills token buckets every minute.
func (g *Guard) refillLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCleanup:
			return
		case <-ticker.C:
			g.clients.Range(func(key, value any) bool {
				cs := value.(*clientState)
				// Refill to max, don't exceed
				httpMax := int64(g.cfg.RPM)
				if cs.httpTokens.Load() < httpMax {
					cs.httpTokens.Store(httpMax)
				}
				wsMax := int64(g.cfg.WSPerMin)
				if cs.wsTokens.Load() < wsMax {
					cs.wsTokens.Store(wsMax)
				}
				return true
			})
		}
	}
}

// cleanupLoop evicts idle clients.
func (g *Guard) cleanupLoop() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-g.stopCleanup:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-idleEvictTime).Unix()
			evicted := 0
			g.clients.Range(func(key, value any) bool {
				cs := value.(*clientState)
				if cs.lastSeen.Load() < cutoff {
					g.clients.Delete(key)
					evicted++
				}
				return true
			})
			if evicted > 0 {
				log.D.F("httpguard: evicted %d idle client entries", evicted)
			}
		}
	}
}

func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain is the client
		if idx := strings.IndexByte(xff, ','); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to remote address
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}
