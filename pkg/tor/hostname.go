package tor

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"git.smesh.lol/actor"
	"git.smesh.lol/orly/pkg/lol/chk"
	"git.smesh.lol/orly/pkg/lol/log"
)

// HostnameWatcher watches the Tor hidden service hostname file for changes.
// When Tor creates or updates a hidden service, it writes the .onion address
// to a file called "hostname" in the HiddenServiceDir.
// All mutable state is owned by the actor goroutine.
type HostnameWatcher struct {
	hsDir     string
	onChange  actor.Proc[func(string)]
	address   actor.Query[string]
	actor.Lifecycle
}

// NewHostnameWatcher creates a new hostname watcher for the given HiddenServiceDir.
func NewHostnameWatcher(hsDir string) *HostnameWatcher {
	return &HostnameWatcher{
		hsDir:     hsDir,
		onChange:  actor.NewProc[func(string)](),
		address:   actor.NewQuery[string](),
		Lifecycle: actor.NewLifecycle(),
	}
}

// OnChange sets a callback function to be called when the hostname changes.
func (w *HostnameWatcher) OnChange(fn func(string)) {
	w.onChange.Call(fn)
}

// Start begins watching the hostname file.
func (w *HostnameWatcher) Start() error {
	actor.Go(w.Lifecycle, w.actorLoop)
	return nil
}

// Shutdown stops the hostname watcher.
func (w *HostnameWatcher) Shutdown() {
	w.Stop()
}

// Address returns the current .onion address.
func (w *HostnameWatcher) Address() string {
	return w.address.Call()
}

// actorLoop is the actor goroutine that owns address and onChange state.
func (w *HostnameWatcher) actorLoop() {
	var address string
	var onChangeFn func(string)

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
		case <-w.Stopping():
			return

		case msg := <-w.onChange.Recv():
			onChangeFn = msg.Req
			msg.Done()

		case msg := <-w.address.Recv():
			msg.Reply(address)

		case <-ticker.C:
			addr, err := w.readHostnameFile()
			if err != nil {
				log.T.F("hostname read: %v", err)
				continue
			}
			if addr != "" && addr != address {
				oldAddr := address
				address = addr
				if onChangeFn != nil && addr != oldAddr {
					onChangeFn(addr)
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
