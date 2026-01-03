package tor

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lol.mleku.dev/chk"
	"lol.mleku.dev/log"
)

// HostnameWatcher watches the Tor hidden service hostname file for changes.
// When Tor creates or updates a hidden service, it writes the .onion address
// to a file called "hostname" in the HiddenServiceDir.
type HostnameWatcher struct {
	hsDir    string
	address  string
	onChange func(string)

	stopCh chan struct{}
	mu     sync.RWMutex
}

// NewHostnameWatcher creates a new hostname watcher for the given HiddenServiceDir.
func NewHostnameWatcher(hsDir string) *HostnameWatcher {
	return &HostnameWatcher{
		hsDir:  hsDir,
		stopCh: make(chan struct{}),
	}
}

// OnChange sets a callback function to be called when the hostname changes.
func (w *HostnameWatcher) OnChange(fn func(string)) {
	w.mu.Lock()
	w.onChange = fn
	w.mu.Unlock()
}

// Start begins watching the hostname file.
func (w *HostnameWatcher) Start() error {
	// Try to read immediately
	if err := w.readHostname(); err != nil {
		log.D.F("hostname file not yet available: %v", err)
	}

	// Start polling goroutine
	go w.poll()

	return nil
}

// Stop stops the hostname watcher.
func (w *HostnameWatcher) Stop() {
	close(w.stopCh)
}

// Address returns the current .onion address.
func (w *HostnameWatcher) Address() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.address
}

// poll periodically checks the hostname file for changes.
func (w *HostnameWatcher) poll() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if err := w.readHostname(); err != nil {
				// Only log at trace level to avoid spam
				log.T.F("hostname read: %v", err)
			}
		}
	}
}

// readHostname reads the hostname file and updates the address if changed.
func (w *HostnameWatcher) readHostname() error {
	path := filepath.Join(w.hsDir, "hostname")

	data, err := os.ReadFile(path)
	if chk.T(err) {
		return err
	}

	// Parse the address (file contains "xyz.onion\n")
	addr := strings.TrimSpace(string(data))
	if addr == "" {
		return nil
	}

	w.mu.Lock()
	oldAddr := w.address
	w.address = addr
	onChange := w.onChange
	w.mu.Unlock()

	// Call callback if address changed
	if addr != oldAddr && onChange != nil {
		onChange(addr)
	}

	return nil
}

// HostnameFilePath returns the path to the hostname file.
func (w *HostnameWatcher) HostnameFilePath() string {
	return filepath.Join(w.hsDir, "hostname")
}
