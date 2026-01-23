package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:web/dist all:web/public
var adminFS embed.FS

// getAdminFS returns the embedded filesystem for the admin UI.
func getAdminFS() (http.FileSystem, error) {
	// Try dist first (built assets)
	distFS, err := fs.Sub(adminFS, "web/dist")
	if err == nil {
		// Check if dist has content
		entries, _ := fs.ReadDir(distFS, ".")
		if len(entries) > 0 {
			return http.FS(distFS), nil
		}
	}

	// Fall back to public (template)
	publicFS, err := fs.Sub(adminFS, "web/public")
	if err != nil {
		return nil, err
	}
	return http.FS(publicFS), nil
}

// serveAdminUI serves the embedded admin web UI.
func (s *AdminServer) serveAdminUI(w http.ResponseWriter, r *http.Request) {
	fsys, err := getAdminFS()
	if err != nil {
		http.Error(w, "Admin UI not available", http.StatusInternalServerError)
		return
	}

	// Strip /admin prefix from path
	urlPath := r.URL.Path
	if strings.HasPrefix(urlPath, "/admin") {
		urlPath = strings.TrimPrefix(urlPath, "/admin")
		if urlPath == "" {
			urlPath = "/"
		}
	}

	// Try to serve the file
	filePath := strings.TrimPrefix(urlPath, "/")
	if filePath == "" {
		filePath = "index.html"
	}

	// Check if file exists
	f, err := fsys.Open(filePath)
	if err != nil {
		// Serve index.html for SPA routing
		filePath = "index.html"
		f, err = fsys.Open(filePath)
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
	}
	f.Close()

	// Set content type
	switch path.Ext(filePath) {
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
	}

	// Serve the file
	r.URL.Path = "/" + filePath
	http.FileServer(fsys).ServeHTTP(w, r)
}
