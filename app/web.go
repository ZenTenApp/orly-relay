package app

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed smesh/dist
var reactAppFS embed.FS

var webDistFS fs.FS

func getWebDistFS() fs.FS {
	if webDistFS == nil {
		var err error
		webDistFS, err = fs.Sub(reactAppFS, "smesh/dist")
		if err != nil {
			panic("Failed to load embedded web app: " + err.Error())
		}
	}
	return webDistFS
}

// GetReactAppFS returns a http.FileSystem from the embedded React app
func GetReactAppFS() http.FileSystem {
	return http.FS(getWebDistFS())
}

// ServeEmbeddedWeb serves the embedded web application with SPA fallback routing.
// For paths that don't map to a real file, serve index.html so React router can handle them.
func ServeEmbeddedWeb(w http.ResponseWriter, r *http.Request) {
	distFS := getWebDistFS()
	fileServer := http.FileServer(http.FS(distFS))

	path := r.URL.Path
	if path == "/" {
		fileServer.ServeHTTP(w, r)
		return
	}

	// Check if the path maps to a real file in the embedded FS
	cleanPath := strings.TrimPrefix(path, "/")
	if f, err := distFS.Open(cleanPath); err == nil {
		f.Close()
		fileServer.ServeHTTP(w, r)
		return
	}

	// SPA fallback: serve index.html for client-side routing
	r.URL.Path = "/"
	fileServer.ServeHTTP(w, r)
}

// GetEmbeddedWebFS returns the raw embedded filesystem for branding initialization.
// This is used by the init-branding command to extract default assets.
func GetEmbeddedWebFS() embed.FS {
	return reactAppFS
}
