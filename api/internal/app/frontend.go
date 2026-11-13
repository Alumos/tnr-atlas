package app

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

// Frontend serves the Node-built SPA in the same Go process as the API.
func Frontend(api http.Handler, dir string) http.Handler {
	root := os.DirFS(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			w.WriteHeader(405)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		info, err := fs.Stat(root, name)
		if err != nil || info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Cache-Control", "no-cache")
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public,max-age=31536000,immutable")
		}
		original := name
		if acceptsGzip(r.Header.Get("Accept-Encoding")) {
			if _, err := fs.Stat(root, name+".gz"); err == nil {
				name += ".gz"
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Vary", "Accept-Encoding")
			}
		}
		file, err := os.Open(dir + "/" + name)
		if err != nil {
			http.Error(w, "Frontend build missing", 503)
			return
		}
		defer file.Close()
		info, err = file.Stat()
		if err != nil {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, original, info.ModTime(), file)
	})
}
func acceptsGzip(header string) bool {
	for _, part := range strings.Split(header, ",") {
		bits := strings.Split(strings.TrimSpace(part), ";")
		if bits[0] != "gzip" {
			continue
		}
		quality := 1.0
		for _, parameter := range bits[1:] {
			parameter = strings.TrimSpace(parameter)
			if strings.HasPrefix(parameter, "q=") {
				v, err := strconv.ParseFloat(strings.TrimPrefix(parameter, "q="), 64)
				if err != nil {
					return false
				}
				quality = v
			}
		}
		return quality > 0
	}
	return false
}
