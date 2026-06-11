package tor

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// onChangeReq sets the onChange callback.
type onChangeReq struct {
	fn   func(string)
	resp chan struct{}
}

// addressReq queries the current address.
type addressReq struct {
	resp chan string
}

// HostnameWatcher watches the Tor hidden service hostname file for changes.
// When Tor creates or updates a hidden service, it writes the .onion address
// to a file called "hostname" in the HiddenServiceDir.
type HostnameWatcher struct {
	hsDir string

	onChangeCh chan onChangeReq
	addressCh  chan addressReq

	stop chan struct{}
	done chan struct{}
}

// NewHostnameWatcher creates a new hostname watcher for the given HiddenServiceDir.
func NewHostnameWatcher(hsDir string) *HostnameWatcher {
	return &HostnameWatcher{
		hsDir:      hsDir,
		onChangeCh: make(chan onChangeReq),
		addressCh:  make(chan addressReq),
		stop:       make(chan struct{}),
		done:       make(chan struct{}),
	}
}

// OnChange sets a callback function to be called when the hostname changes.
func (w *HostnameWatcher) OnChange(fn func(string)) {
	resp := make(chan struct{}, 1)
	w.onChangeCh <- onChangeReq{fn: fn, resp: resp}
	<-resp
}

// Start begins watching the hostname file.
func (w *HostnameWatcher) Start() error {
	go w.run()
	return nil
}

// Stop stops the hostname watcher.
func (w *HostnameWatcher) Stop() {
	close(w.stop)
	<-w.done
}

// Address returns the current .onion address.
func (w *HostnameWatcher) Address() string {
	resp := make(chan string, 1)
	w.addressCh <- addressReq{resp: resp}
	return <-resp
}

// run is the actor goroutine that owns address and onChange state.
func (w *HostnameWatcher) run() {
	defer close(w.done)

	var address string
	var onChange func(string)

	// Try initial read
	if addr, err := w.readHostnameFile(); err != nil {
		log.D.F("hostname file not yet available: %v", err)
	} else if addr != "" {
		address = addr
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return

		case req := <-w.onChangeCh:
			onChange = req.fn
			close(req.resp)

		case req := <-w.addressCh:
			req.resp <- address

		case <-ticker.C:
			addr, err := w.readHostnameFile()
			if err != nil {
				log.T.F("hostname read: %v", err)
				continue
			}
			if addr != "" && addr != address {
				oldAddr := address
				address = addr
				if onChange != nil && addr != oldAddr {
					onChange(addr)
				}
			}
		}
	}
}

// readHostnameFile reads the hostname file and returns the address.
func (w *HostnameWatcher) readHostnameFile() (string, error) {
	path := filepath.Join(w.hsDir, "hostname")

	data, err := os.ReadFile(path)
	if chk.T(err) {
		return "", err
	}

	addr := strings.TrimSpace(string(data))
	return addr, nil
}

// HostnameFilePath returns the path to the hostname file.
func (w *HostnameWatcher) HostnameFilePath() string {
	return filepath.Join(w.hsDir, "hostname")
}
