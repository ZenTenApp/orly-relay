// Simple static file server for testing the standalone dashboard
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	dir := flag.String("dir", "", "directory to serve (default: app/web/dist)")
	flag.Parse()

	// Find the dist directory
	serveDir := *dir
	if serveDir == "" {
		// Try to find it relative to current directory or executable
		candidates := []string{
			"app/web/dist",
			"../app/web/dist",
			"../../app/web/dist",
		}

		// Also check relative to executable
		if exe, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exe)
			candidates = append(candidates,
				filepath.Join(exeDir, "app/web/dist"),
				filepath.Join(exeDir, "../app/web/dist"),
			)
		}

		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				serveDir = candidate
				break
			}
		}

		if serveDir == "" {
			log.Fatal("Could not find dist directory. Use -dir flag to specify.")
		}
	}

	absDir, _ := filepath.Abs(serveDir)
	fmt.Printf("Serving %s on http://localhost:%d\n", absDir, *port)
	fmt.Println("Press Ctrl+C to stop")

	// Create file server with SPA fallback
	fs := http.FileServer(http.Dir(serveDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file
		path := filepath.Join(serveDir, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// File doesn't exist, serve index.html for SPA routing
			http.ServeFile(w, r, filepath.Join(serveDir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf(":%d", *port)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
