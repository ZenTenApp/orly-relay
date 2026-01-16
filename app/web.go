package app

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist
var reactAppFS embed.FS

// GetReactAppFS returns a http.FileSystem from the embedded React app
func GetReactAppFS() http.FileSystem {
	webDist, err := fs.Sub(reactAppFS, "web/dist")
	if err != nil {
		panic("Failed to load embedded web app: " + err.Error())
	}
	return http.FS(webDist)
}

// ServeEmbeddedWeb serves the embedded web application
func ServeEmbeddedWeb(w http.ResponseWriter, r *http.Request) {
	// Serve the embedded web app
	http.FileServer(GetReactAppFS()).ServeHTTP(w, r)
}

// GetEmbeddedWebFS returns the raw embedded filesystem for branding initialization.
// This is used by the init-branding command to extract default assets.
func GetEmbeddedWebFS() embed.FS {
	return reactAppFS
}
