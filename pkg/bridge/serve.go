package bridge

import (
	"embed"
	"net/http"
)

//go:embed compose/compose.html compose/decrypt.html
var composeFS embed.FS

// ComposeHandler returns an HTTP handler that serves the compose form.
func ComposeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := composeFS.ReadFile("compose/compose.html")
		if err != nil {
			http.Error(w, "compose form not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}

// DecryptHandler returns an HTTP handler that serves the decrypt page.
func DecryptHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := composeFS.ReadFile("compose/decrypt.html")
		if err != nil {
			http.Error(w, "decrypt page not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	}
}
