package console

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

// dist holds the embedded console frontend files.
// The files are embedded from console/dist during build.
// This variable will contain a .gitkeep placeholder during development
// if the console hasn't been built.
//
//go:embed all:dist
var dist embed.FS

// Config holds runtime configuration values that are injected into the
// console frontend via /config.js. Add new fields here when the frontend
// needs additional server-side configuration.
type Config struct {
	ClerkPublishableKey string `json:"CLERK_PUBLISHABLE_KEY"`
}

const configScriptTemplate = `window.__CONFIG__ = %s;`

// Handler returns an http.Handler that serves the embedded console files.
// It implements SPA (Single Page Application) routing by serving index.html
// for any path that doesn't match a static file.
//
// The provided Config is serialised once and served as /config.js on every
// request, allowing the frontend to read environment-specific values at
// runtime without rebuilding the Docker image.
func Handler(cfg Config) (http.Handler, error) {
	// Strip the "dist" prefix from the embedded filesystem
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}

	config, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	configScript := []byte(fmt.Sprintf(configScriptTemplate, string(config)))

	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Clean the path
		if path == "/" {
			path = "/index.html"
		}

		// Remove leading slash for fs.Open
		filePath := strings.TrimPrefix(path, "/")

		// Serve dynamic runtime config for the console frontend.
		// This replaces the static config.js from the build with
		// environment-specific values (e.g. Clerk publishable key).
		if filePath == "config.js" {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Write(configScript) //nolint:errcheck
			return
		}

		// Check if the file exists
		if f, err := fsys.Open(filePath); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// For SPA routing: serve index.html for paths that don't match static files
		// This allows client-side routing to work correctly
		indexFile, err := fsys.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer indexFile.Close()

		stat, err := indexFile.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Serve index.html with proper content type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(readSeeker))
	}), nil
}

// readSeeker combines io.Reader and io.Seeker interfaces
// which is required by http.ServeContent
type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}
