package embed

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed all:dist
var dashboardFS embed.FS

// SPAHandler serves the React SPA with fallback to index.html for client-side routing.
func SPAHandler() http.Handler {
	sub, err := fs.Sub(dashboardFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem for embedded SPA: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if path == "" {
			path = "index.html"
		}

		f, err := sub.Open(path)
		if err != nil {
			// File not found: serve index.html for SPA client-side routing
			f, err = sub.Open("index.html")
			if err != nil {
				http.Error(w, "SPA not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		defer f.Close()

		stat, _ := f.Stat()
		if seeker, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(w, r, stat.Name(), stat.ModTime(), seeker)
		}
	})
}
