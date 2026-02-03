package console

import (
	"embed"
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

// Handler returns an http.Handler that serves the embedded console files.
// It implements SPA (Single Page Application) routing by serving index.html
// for any path that doesn't match a static file.
func Handler() (http.Handler, error) {
	// Strip the "dist" prefix from the embedded filesystem
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Clean the path
		if path == "/" {
			path = "/index.html"
		}

		// Remove leading slash for fs.Open
		filePath := strings.TrimPrefix(path, "/")

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
