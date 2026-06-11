package app

import (
	"context"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"git.smesh.lol/orly/pkg/lol/log"
)

//go:embed smesh3
var smesh3FS embed.FS

// -- smesh3 actor request types --

type smesh3GetVersionReq struct {
	resp chan int64 // buffered 1
}
type smesh3RegisterClientReq struct {
	ch   chan string
	resp chan int64 // buffered 1: returns current version
}
type smesh3UnregisterClientReq struct {
	ch chan string
}
type smesh3RecordPendingFileReq struct {
	path string
}
type smesh3FlushReq struct {
	resp chan smesh3FlushResp // buffered 1
}
type smesh3FlushResp struct {
	version int64
	files   []string
	clients []chan string
}
type smesh3FullRefreshReq struct {
	resp chan smesh3FullRefreshResp // buffered 1
}
type smesh3FullRefreshResp struct {
	version int64
	clients []chan string
}

// Smesh3Server serves the smesh web client with optional hot-reload.
// When dir is set, serves from disk and watches for changes via fsnotify.
// Connected clients receive version updates over SSE.
type Smesh3Server struct {
	server   *http.Server
	listener net.Listener
	watcher  *fsnotify.Watcher
	port     int
	dir      string

	deployPub []byte // 32-byte x-only pubkey for /__deploy auth
	clientTag string // client tag for published events (NIP-89)

	// Actor channels for state management (version, clients, pendingFiles)
	getVersionCh     chan smesh3GetVersionReq
	registerCh       chan smesh3RegisterClientReq
	unregisterCh     chan smesh3UnregisterClientReq
	recordPendingCh  chan smesh3RecordPendingFileReq
	flushCh          chan smesh3FlushReq
	fullRefreshCh    chan smesh3FullRefreshReq
	actorStop        chan struct{}
	actorDone        chan struct{}

	// Deploy actor channels
	deployBeginCh   chan deployBeginReq
	deployGetCh     chan deployGetReq
	deploySetPartCh chan deploySetPartReq
	deployTakeCh    chan deployTakeReq
	deployStopCh    chan struct{}
	deployDone      chan struct{}

	version  int64 // only read in extractAndSwap after actor response
	cancelFn context.CancelFunc
}

// NewSmesh3Server creates a new smesh HTTP server.
func NewSmesh3Server(port int, dir, deployPubHex, clientTag string) *Smesh3Server {
	s := &Smesh3Server{
		port:      port,
		dir:       dir,
		clientTag: clientTag,
		version:   time.Now().UnixMilli(),
	}
	if len(deployPubHex) == 64 {
		pub, err := hex.DecodeString(deployPubHex)
		if err == nil && len(pub) == 32 {
			s.deployPub = pub
		}
	}
	return s
}

func (s *Smesh3Server) startActor(ctx context.Context) {
	s.getVersionCh = make(chan smesh3GetVersionReq)
	s.registerCh = make(chan smesh3RegisterClientReq)
	s.unregisterCh = make(chan smesh3UnregisterClientReq, 16)
	s.recordPendingCh = make(chan smesh3RecordPendingFileReq, 16)
	s.flushCh = make(chan smesh3FlushReq)
	s.fullRefreshCh = make(chan smesh3FullRefreshReq)
	s.actorStop = make(chan struct{})
	s.actorDone = make(chan struct{})
	s.deployStopCh = make(chan struct{})

	go func() {
		defer close(s.actorDone)
		version := s.version
		clients := make(map[chan string]struct{})
		pendingFiles := make(map[string]struct{})

		for {
			select {
			case req := <-s.getVersionCh:
				req.resp <- version
			case req := <-s.registerCh:
				clients[req.ch] = struct{}{}
				req.resp <- version
			case req := <-s.unregisterCh:
				delete(clients, req.ch)
			case req := <-s.recordPendingCh:
				pendingFiles[req.path] = struct{}{}
			case req := <-s.flushCh:
				version = time.Now().UnixMilli()
				s.version = version
				files := make([]string, 0, len(pendingFiles))
				for f := range pendingFiles {
					files = append(files, f)
				}
				pendingFiles = make(map[string]struct{})
				chs := make([]chan string, 0, len(clients))
				for ch := range clients {
					chs = append(chs, ch)
				}
				req.resp <- smesh3FlushResp{version: version, files: files, clients: chs}
			case req := <-s.fullRefreshCh:
				version = time.Now().UnixMilli()
				s.version = version
				chs := make([]chan string, 0, len(clients))
				for ch := range clients {
					chs = append(chs, ch)
				}
				req.resp <- smesh3FullRefreshResp{version: version, clients: chs}
			case <-s.actorStop:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Start begins serving the smesh client.
func (s *Smesh3Server) Start(ctx context.Context) error {
	ctx, s.cancelFn = context.WithCancel(ctx)

	// Start actor goroutine
	s.startActor(ctx)

	// Start deploy actor if deploy is enabled
	if len(s.deployPub) == 32 {
		s.startDeployActor()
	}

	var fileHandler http.Handler
	if s.dir != "" {
		fileHandler = http.FileServer(http.Dir(s.dir))
		if err := s.startWatcher(ctx); err != nil {
			log.W.F("smesh: fsnotify failed, hot-reload disabled: %v", err)
		}
	} else {
		webDist, err := fs.Sub(smesh3FS, "smesh3")
		if err != nil {
			return fmt.Errorf("failed to load embedded smesh app: %w", err)
		}
		fileHandler = http.FileServer(http.FS(webDist))
	}

	mux := http.NewServeMux()

	// SSE endpoint for version updates.
	mux.HandleFunc("/__sse", s.handleSSE)

	// Version endpoint (quick poll fallback).
	mux.HandleFunc("/__version", s.handleVersion)

	// Client tag for published events.
	mux.HandleFunc("/__client-tag", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprint(w, s.clientTag)
	})

	// Signed bundle deploy endpoint.
	if len(s.deployPub) == 32 {
		mux.HandleFunc("/__deploy", s.handleDeploy)
		log.I.F("smesh: /__deploy enabled for pubkey %x", s.deployPub)
	}

	// Satellite SW loader pages
	for _, swDir := range []string{"$sw-relay"} {
		dir := swDir
		mux.HandleFunc("/"+dir+"/loader.html", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			fmt.Fprintf(w, `<!DOCTYPE html><html><body>
<script>
if('serviceWorker' in navigator){
  function sendPort(w){
    var ch=new MessageChannel();
    w.postMessage({type:'bus-port'},[ch.port2]);
    ch.port1.onmessage=function(ev){
      var d=ev.data;
      if(typeof d==='string'&&d.length>0&&d[0]==='{')
        window.parent.postMessage(d,'*');
    };
    console.log('%s port sent to SW');
  }
  async function boot(){
    var regs=await navigator.serviceWorker.getRegistrations();
    for(var r of regs){if(r.scope.includes('/%s/'))await r.unregister();}
    var reg=await navigator.serviceWorker.register('/%s/$entry.mjs',{type:'module',scope:'/%s/'});
    console.log('%s SW registered');
    var sw=reg.active;
    if(sw) sendPort(sw);
    reg.addEventListener('updatefound',function(){
      var n=reg.installing;
      if(n) n.addEventListener('statechange',function(){
        if(n.state==='activated') sendPort(n);
      });
    });
    if(!sw){
      sw=reg.installing||reg.waiting;
      if(sw) sw.addEventListener('statechange',function(){if(sw.state==='activated')sendPort(sw);});
    }
  }
  boot().catch(e=>console.error('%s SW boot failed:',e));
  window.addEventListener('message',ev=>{
    var d=ev.data;
    if(typeof d==='string'&&d.length>0&&d[0]==='{'&&navigator.serviceWorker.controller){
      navigator.serviceWorker.controller.postMessage(d);
    }
  });
  setInterval(()=>{
    if(navigator.serviceWorker.controller)navigator.serviceWorker.controller.postMessage('keepalive');
  },20000);
}
</script>
</body></html>`, dir, dir, dir, dir, dir, dir)
		})
	}

	// File serving with MIME fix and SPA fallback.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
		return fmt.Errorf("smesh: failed to listen on %s: %w", addr, err)
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			log.W.F("smesh server shutdown error: %v", err)
		}
	}()

	mode := "embedded"
	if s.dir != "" {
		mode = "disk:" + s.dir
	}
	log.I.F("smesh web client serving on http://%s (%s)", addr, mode)

	go func() {
		if err := s.server.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.E.F("smesh server error: %v", err)
		}
	}()

	// Extra listeners for SW origin isolation
	for _, ip := range []string{"127.0.0.2", "127.0.0.3", "127.0.0.4"} {
		swAddr := fmt.Sprintf("%s:%d", ip, s.port)
		ln, err := net.Listen("tcp", swAddr)
		if err != nil {
			log.W.F("smesh: failed to listen on %s: %v", swAddr, err)
			continue
		}
		srv := &http.Server{
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  120 * time.Second,
		}
		log.I.F("smesh SW origin serving on http://%s", swAddr)
		go func() {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				log.E.F("smesh SW server error on %s: %v", swAddr, err)
			}
		}()
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
		}()
	}

	return nil
}

// Stop shuts down the smesh server.
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
	// Stop deploy actor if it was started
	if s.deployStopCh != nil {
		select {
		case <-s.deployStopCh:
			// already closed
		default:
			close(s.deployStopCh)
		}
	}
	if s.deployDone != nil {
		<-s.deployDone
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
	log.I.F("smesh: watching %s for changes", s.dir)
	return nil
}

// watchLoop debounces fsnotify events, tracks changed files, and notifies clients.
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
			// Record the changed file path.
			rel, err := filepath.Rel(s.dir, ev.Name)
			if err == nil {
				path := "/" + filepath.ToSlash(rel)
				if path == "/index.html" {
					path = "/"
				}
				s.recordPendingCh <- smesh3RecordPendingFileReq{path: path}
			}
			// Debounce: wait 200ms for batch writes (rsync).
			if debounce != nil {
				debounce.Stop()
			}
			debounce = time.AfterFunc(200*time.Millisecond, func() {
				s.flushChanges()
			})
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			log.W.F("smesh: fsnotify error: %v", err)
		}
	}
}

// flushChanges collects pending file changes, bumps version, and notifies SSE clients.
func (s *Smesh3Server) flushChanges() {
	resp := make(chan smesh3FlushResp, 1)
	s.flushCh <- smesh3FlushReq{resp: resp}
	r := <-resp

	// Build JSON: {"v":123,"files":["/smesh3.mjs"]}
	var sb strings.Builder
	sb.WriteString(`{"v":`)
	sb.WriteString(strconv.FormatInt(r.version, 10))
	sb.WriteString(`,"files":[`)
	for i, f := range r.files {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('"')
		sb.WriteString(f)
		sb.WriteByte('"')
	}
	sb.WriteString("]}")
	msg := sb.String()

	log.I.F("smesh: %d files changed, version=%d, notifying %d clients", len(r.files), r.version, len(r.clients))
	for _, ch := range r.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// notifyFullRefresh bumps version and notifies with empty file list (full refresh).
func (s *Smesh3Server) notifyFullRefresh() {
	resp := make(chan smesh3FullRefreshResp, 1)
	s.fullRefreshCh <- smesh3FullRefreshReq{resp: resp}
	r := <-resp

	msg := `{"v":` + strconv.FormatInt(r.version, 10) + `}`

	log.I.F("smesh: full refresh, version=%d, notifying %d clients", r.version, len(r.clients))
	for _, ch := range r.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

// handleVersion returns the current version as plain text.
func (s *Smesh3Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	resp := make(chan int64, 1)
	s.getVersionCh <- smesh3GetVersionReq{resp: resp}
	v := <-resp
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprintf(w, "%d", v)
}

// handleSSE maintains a Server-Sent Events connection.
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

	ch := make(chan string, 4)

	// Register client.
	resp := make(chan int64, 1)
	s.registerCh <- smesh3RegisterClientReq{ch: ch, resp: resp}
	v := <-resp

	// Send current version immediately.
	fmt.Fprintf(w, "data: {\"v\":%d}\n\n", v)
	flusher.Flush()

	defer func() {
		s.unregisterCh <- smesh3UnregisterClientReq{ch: ch}
	}()

	ctx := r.Context()
	// Heartbeat to detect dead connections.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
