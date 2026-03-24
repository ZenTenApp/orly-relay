package app

import (
	"context"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"next.orly.dev/pkg/lol/log"
)

//go:embed smesh3
var smesh3FS embed.FS

// Smesh3Server serves the sm3sh web client with optional hot-reload.
// When dir is set, serves from disk and watches for changes via fsnotify.
// Connected clients receive version updates over SSE.
type Smesh3Server struct {
	server   *http.Server
	listener net.Listener
	watcher  *fsnotify.Watcher
	port     int
	dir      string
	dataDir  string

	deployPub []byte // 32-byte x-only pubkey for /__deploy auth

	bus *busHub // inter-SW message bus

	mu       sync.RWMutex
	version  int64
	clients  map[chan int64]struct{}
	cancelFn context.CancelFunc
}

// NewSmesh3Server creates a new sm3sh HTTP server.
// If dir is non-empty, files are served from disk with hot-reload.
// dataDir is the storage root for marmot group state.
// deployPubHex is the hex-encoded pubkey authorized for /__deploy (empty disables).
func NewSmesh3Server(port int, dir, dataDir, deployPubHex string) *Smesh3Server {
	s := &Smesh3Server{
		port:    port,
		dir:     dir,
		dataDir: dataDir,
		bus:     newBusHub(),
		version: time.Now().UnixMilli(),
		clients: make(map[chan int64]struct{}),
	}
	if len(deployPubHex) == 64 {
		pub, err := hex.DecodeString(deployPubHex)
		if err == nil && len(pub) == 32 {
			s.deployPub = pub
		}
	}
	return s
}

// Start begins serving the sm3sh client.
func (s *Smesh3Server) Start(ctx context.Context) error {
	ctx, s.cancelFn = context.WithCancel(ctx)

	var fileHandler http.Handler
	if s.dir != "" {
		fileHandler = http.FileServer(http.Dir(s.dir))
		if err := s.startWatcher(ctx); err != nil {
			log.W.F("sm3sh: fsnotify failed, hot-reload disabled: %v", err)
		}
	} else {
		webDist, err := fs.Sub(smesh3FS, "smesh3")
		if err != nil {
			return fmt.Errorf("failed to load embedded sm3sh app: %w", err)
		}
		fileHandler = http.FileServer(http.FS(webDist))
	}

	mux := http.NewServeMux()

	// SSE endpoint for version updates.
	mux.HandleFunc("/__sse", s.handleSSE)

	// Version endpoint (quick poll fallback).
	mux.HandleFunc("/__version", s.handleVersion)

	// Marmot WebSocket endpoint for MLS-based DMs.
	mux.HandleFunc("/__marmot", s.handleMarmot)

	// Inter-SW message bus.
	mux.HandleFunc("/__bus", s.handleBus)

	// SW error reporting — logs errors from service worker to server console.
	mux.HandleFunc("/__sw-error", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		log.W.F("SW ERROR: %s", msg)
		w.WriteHeader(200)
	})

	// Browser log bridge — SWs and page POST log lines here, appended to /tmp/browser-debug.log.
	mux.HandleFunc("/__log", s.handleLog)

	// Signed bundle deploy endpoint.
	if len(s.deployPub) == 32 {
		mux.HandleFunc("/__deploy", s.handleDeploy)
		log.I.F("sm3sh: /__deploy enabled for pubkey %x", s.deployPub)
	}

	// File serving with MIME fix and SPA fallback.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Satellite subdomain routing — serve loader + SW files for isolated SWs.
		host := r.Host
		if idx := strings.IndexByte(host, ':'); idx >= 0 {
			host = host[:idx]
		}
		if swDir := satelliteSWDir(host); swDir != "" {
			serveSatellite(w, r, swDir, fileHandler)
			return
		}

		path := r.URL.Path

		if strings.HasSuffix(path, ".mjs") {
			w.Header().Set("Content-Type", "application/javascript")
		}
		if strings.Contains(path, "$sw") {
			w.Header().Set("Service-Worker-Allowed", "/")
		}

		// No caching in disk/dev mode; short cache in embedded/prod.
		if s.dir != "" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else if path == "/" || strings.HasSuffix(path, ".html") || path == "/sw.js" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		}

		if s.dir != "" {
			// Disk mode: check file exists, SPA fallback.
			cleanPath := filepath.Join(s.dir, filepath.Clean(path))
			if _, err := os.Stat(cleanPath); err != nil && path != "/" {
				r.URL.Path = "/"
			}
			fileHandler.ServeHTTP(w, r)
			return
		}

		// Embedded mode.
		if path == "/" {
			fileHandler.ServeHTTP(w, r)
			return
		}
		cleanPath := strings.TrimPrefix(path, "/")
		webDist, _ := fs.Sub(smesh3FS, "smesh3")
		if f, err := webDist.Open(cleanPath); err == nil {
			f.Close()
			fileHandler.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		fileHandler.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0, // SSE needs no write timeout
		IdleTimeout:  120 * time.Second,
	}

	var err error
	s.listener, err = net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("sm3sh: failed to listen on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.W.F("sm3sh server shutdown error: %v", err)
		}
	}()

	mode := "embedded"
	if s.dir != "" {
		mode = "disk:" + s.dir
	}
	log.I.F("sm3sh web client serving on http://%s (%s)", addr, mode)

	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.E.F("sm3sh server error: %v", err)
		}
	}()

	return nil
}

// Stop shuts down the sm3sh server.
func (s *Smesh3Server) Stop() {
	if s.cancelFn != nil {
		s.cancelFn()
	}
	if s.watcher != nil {
		s.watcher.Close()
	}
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}

// startWatcher sets up fsnotify on the disk directory.
func (s *Smesh3Server) startWatcher(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	s.watcher = w

	// Watch the root dir and all subdirectories (2 levels).
	if err := w.Add(s.dir); err != nil {
		return err
	}
	entries, _ := os.ReadDir(s.dir)
	for _, e := range entries {
		if e.IsDir() {
			subdir := filepath.Join(s.dir, e.Name())
			w.Add(subdir)
			sub2, _ := os.ReadDir(subdir)
			for _, e2 := range sub2 {
				if e2.IsDir() {
					w.Add(filepath.Join(subdir, e2.Name()))
				}
			}
		}
	}

	go s.watchLoop(ctx)
	log.I.F("sm3sh: watching %s for changes", s.dir)
	return nil
}

// watchLoop debounces fsnotify events and bumps the version.
func (s *Smesh3Server) watchLoop(ctx context.Context) {
	var debounce *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}
			// Debounce: wait 200ms for batch writes (rsync).
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(200*time.Millisecond, func() {
				s.bumpVersion()
			})
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.W.F("sm3sh: fsnotify error: %v", err)
		}
	}
}

// bumpVersion updates the version and notifies all SSE clients.
func (s *Smesh3Server) bumpVersion() {
	s.mu.Lock()
	s.version = time.Now().UnixMilli()
	v := s.version
	clients := make([]chan int64, 0, len(s.clients))
	for ch := range s.clients {
		clients = append(clients, ch)
	}
	s.mu.Unlock()

	log.I.F("sm3sh: files changed, version=%d, notifying %d clients", v, len(clients))
	for _, ch := range clients {
		select {
		case ch <- v:
		default:
			// Client channel full, skip.
		}
	}
}

// handleVersion returns the current version as plain text.
func (s *Smesh3Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	v := s.version
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "%d", v)
}

// handleSSE maintains a Server-Sent Events connection.
// Sends the current version on connect, then pushes on each change.
func (s *Smesh3Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan int64, 4)

	// Register client.
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	v := s.version
	s.mu.Unlock()

	// Send current version immediately.
	fmt.Fprintf(w, "data: %d\n\n", v)
	flusher.Flush()

	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	ctx := r.Context()
	// Heartbeat to detect dead connections.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case newVersion := <-ch:
			fmt.Fprintf(w, "data: %d\n\n", newVersion)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}

// satelliteSWDir returns the SW output directory for a satellite subdomain,
// or "" if the host is the main domain.
func satelliteSWDir(host string) string {
	if strings.HasPrefix(host, "marmot.") {
		return "$sw-marmot"
	}
	if strings.HasPrefix(host, "relay.") {
		return "$sw-relay"
	}
	return ""
}

// serveSatellite handles requests on satellite subdomains.
// Serves a minimal loader.html for "/" and delegates SW file serving to fileHandler.
func serveSatellite(w http.ResponseWriter, r *http.Request, swDir string, fileHandler http.Handler) {
	path := r.URL.Path
	if path == "/" || path == "/loader.html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		fmt.Fprintf(w, `<!DOCTYPE html><html><body>
<script>
if('serviceWorker' in navigator){
  navigator.serviceWorker.register('./%s/$entry.mjs',{type:'module',scope:'/'})
    .then(()=>{
      console.log('%s SW registered');
      setInterval(()=>{
        if(navigator.serviceWorker.controller){
          navigator.serviceWorker.controller.postMessage('keepalive');
        }
      },20000);
    })
    .catch(e=>console.error('%s SW failed:',e));
}
</script>
</body></html>`, swDir, swDir, swDir)
		return
	}
	if strings.HasSuffix(path, ".mjs") {
		w.Header().Set("Content-Type", "application/javascript")
	}
	if strings.Contains(path, "$sw") {
		w.Header().Set("Service-Worker-Allowed", "/")
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	fileHandler.ServeHTTP(w, r)
}

const browserLogFile = "/tmp/browser-debug.log"

// handleLog receives POST log lines from browser SWs/page and appends to disk.
func (s *Smesh3Server) handleLog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	if r.Method == http.MethodOptions {
		w.WriteHeader(200)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil || len(body) == 0 {
		w.WriteHeader(200)
		return
	}
	f, err := os.OpenFile(browserLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		w.WriteHeader(200)
		return
	}
	defer f.Close()
	line := time.Now().Format("15:04:05.000") + " " + string(body) + "\n"
	f.WriteString(line)
	w.WriteHeader(200)
}
