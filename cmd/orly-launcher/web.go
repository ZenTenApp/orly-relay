package main

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web/dist all:web/public
var adminFS embed.FS

// getAdminSubFS returns the embedded filesystem for the admin UI.
func getAdminSubFS() (fs.FS, error) {
	// Try dist first (built assets)
	distFS, err := fs.Sub(adminFS, "web/dist")
	if err == nil {
		// Check if dist has content
		entries, _ := fs.ReadDir(distFS, ".")
		if len(entries) > 0 {
			return distFS, nil
		}
	}

	// Fall back to public (template)
	return fs.Sub(adminFS, "web/public")
}

// serveAdminUI serves the embedded admin web UI.
func (s *AdminServer) serveAdminUI(w http.ResponseWriter, r *http.Request) {
	fsys, err := getAdminSubFS()
	if err != nil {
		http.Error(w, "Admin UI not available", http.StatusInternalServerError)
		return
	}

	// Strip /admin prefix from path
	urlPath := r.URL.Path
	if strings.HasPrefix(urlPath, "/admin") {
		urlPath = strings.TrimPrefix(urlPath, "/admin")
	}
	urlPath = strings.TrimPrefix(urlPath, "/")

	// Default to index.html
	if urlPath == "" {
		urlPath = "index.html"
	}

	// Try to open the file
	f, err := fsys.Open(urlPath)
	if err != nil {
		// For SPA routing, serve index.html for non-existent paths
		urlPath = "index.html"
		f, err = fsys.Open(urlPath)
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}
	defer f.Close()

	// Check if it's a directory
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if stat.IsDir() {
		// Try index.html in the directory
		f.Close()
		urlPath = path.Join(urlPath, "index.html")
		f, err = fsys.Open(urlPath)
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		defer f.Close()
	}

	// Set content type based on extension
	switch path.Ext(urlPath) {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	// Serve the file content directly
	io.Copy(w, f)
}
