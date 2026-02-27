package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"next.orly.dev/pkg/lol/chk"
	"next.orly.dev/pkg/lol/log"
)

// Updater manages versioned binary updates with symlinks.
// Directory structure:
//
//	~/.local/share/orly/bin/
//	  versions/
//	    v0.55.10/
//	      orly
//	      orly-db-badger
//	      orly-acl-follows
//	      orly-launcher
//	    v0.55.11/
//	      ...
//	  current -> versions/v0.55.11 (symlink)
//	  orly -> current/orly (symlink)
//	  orly-db-badger -> current/orly-db-badger (symlink)
//	  ...
type Updater struct {
	binDir      string // Base directory for binaries
	versionsDir string // Directory containing version subdirectories
}

// VersionInfo contains information about an installed version.
type VersionInfo struct {
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	IsCurrent   bool      `json:"is_current"`
	Binaries    []string  `json:"binaries"`
}

// NewUpdater creates a new Updater.
func NewUpdater(binDir string) *Updater {
	return &Updater{
		binDir:      binDir,
		versionsDir: filepath.Join(binDir, "versions"),
	}
}

// CurrentVersion returns the currently active version.
func (u *Updater) CurrentVersion() string {
	currentLink := filepath.Join(u.binDir, "current")
	target, err := os.Readlink(currentLink)
	if err != nil {
		return "unknown"
	}
	// Extract version from path like "versions/v0.55.10"
	return filepath.Base(target)
}

// ListVersions returns all installed versions.
func (u *Updater) ListVersions() []VersionInfo {
	var versions []VersionInfo

	entries, err := os.ReadDir(u.versionsDir)
	if err != nil {
		return versions
	}

	currentVersion := u.CurrentVersion()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		versionDir := filepath.Join(u.versionsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// List binaries in this version
		binaries, _ := u.listBinaries(versionDir)

		versions = append(versions, VersionInfo{
			Version:     entry.Name(),
			InstalledAt: info.ModTime(),
			IsCurrent:   entry.Name() == currentVersion,
			Binaries:    binaries,
		})
	}

	// Sort by version descending (newest first)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	return versions
}

// listBinaries returns the list of binary files in a directory.
func (u *Updater) listBinaries(dir string) ([]string, error) {
	var binaries []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		// Check if executable
		if info.Mode()&0111 != 0 {
			binaries = append(binaries, entry.Name())
		}
	}
	return binaries, nil
}

// Update downloads binaries from URLs and installs them as a new version.
func (u *Updater) Update(version string, urls map[string]string) ([]string, error) {
	// Create version directory
	versionDir := filepath.Join(u.versionsDir, version)
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create version directory: %w", err)
	}

	var downloadedFiles []string

	// Download each binary
	for name, url := range urls {
		destPath := filepath.Join(versionDir, name)
		log.I.F("downloading %s from %s", name, url)

		if err := u.downloadFile(destPath, url); chk.E(err) {
			// Clean up on failure
			os.RemoveAll(versionDir)
			return nil, fmt.Errorf("failed to download %s: %w", name, err)
		}

		// Make executable
		if err := os.Chmod(destPath, 0755); err != nil {
			os.RemoveAll(versionDir)
			return nil, fmt.Errorf("failed to chmod %s: %w", name, err)
		}

		downloadedFiles = append(downloadedFiles, name)
	}

	// Update symlinks
	if err := u.activateVersion(version); chk.E(err) {
		return downloadedFiles, fmt.Errorf("failed to activate version: %w", err)
	}

	log.I.F("successfully updated to version %s", version)
	return downloadedFiles, nil
}

// downloadFile downloads a file from a URL.
func (u *Updater) downloadFile(destPath, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// activateVersion updates symlinks to point to the specified version.
func (u *Updater) activateVersion(version string) error {
	versionDir := filepath.Join(u.versionsDir, version)

	// Verify version directory exists
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		return fmt.Errorf("version %s not found", version)
	}

	// Update 'current' symlink
	currentLink := filepath.Join(u.binDir, "current")
	tempLink := currentLink + ".tmp"

	// Create new symlink to temp location
	relPath, _ := filepath.Rel(u.binDir, versionDir)
	if err := os.Symlink(relPath, tempLink); err != nil {
		return fmt.Errorf("failed to create temp symlink: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempLink, currentLink); err != nil {
		os.Remove(tempLink)
		return fmt.Errorf("failed to update current symlink: %w", err)
	}

	// Update individual binary symlinks
	binaries, err := u.listBinaries(versionDir)
	if err != nil {
		return fmt.Errorf("failed to list binaries: %w", err)
	}

	for _, binary := range binaries {
		binaryLink := filepath.Join(u.binDir, binary)
		tempBinaryLink := binaryLink + ".tmp"
		targetPath := filepath.Join("current", binary)

		// Create new symlink
		if err := os.Symlink(targetPath, tempBinaryLink); err != nil {
			log.W.F("failed to create symlink for %s: %v", binary, err)
			continue
		}

		// Atomic rename
		if err := os.Rename(tempBinaryLink, binaryLink); err != nil {
			os.Remove(tempBinaryLink)
			log.W.F("failed to update symlink for %s: %v", binary, err)
		}
	}

	return nil
}

// Rollback reverts to the previous version.
func (u *Updater) Rollback() error {
	versions := u.ListVersions()
	if len(versions) < 2 {
		return fmt.Errorf("no previous version available for rollback")
	}

	// Find current version index
	currentVersion := u.CurrentVersion()
	var previousVersion string

	for i, v := range versions {
		if v.Version == currentVersion && i+1 < len(versions) {
			previousVersion = versions[i+1].Version
			break
		}
	}

	if previousVersion == "" {
		return fmt.Errorf("could not determine previous version")
	}

	log.I.F("rolling back from %s to %s", currentVersion, previousVersion)
	return u.activateVersion(previousVersion)
}

// GetBinaryVersion attempts to get the version from a binary using -v flag.
func (u *Updater) GetBinaryVersion(binaryPath string) string {
	cmd := exec.Command(binaryPath, "-v")
	output, err := cmd.Output()
	if err != nil {
		// Try --version
		cmd = exec.Command(binaryPath, "--version")
		output, err = cmd.Output()
		if err != nil {
			return "unknown"
		}
	}
	return strings.TrimSpace(string(output))
}

// EnsureDirectories creates the required directory structure.
func (u *Updater) EnsureDirectories() error {
	return os.MkdirAll(u.versionsDir, 0755)
}
