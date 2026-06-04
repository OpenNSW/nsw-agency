package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// makeRuntimeEnvHandler returns a handler that writes window.__APP_CONFIG__ as a JS snippet.
// Using json.Marshal guarantees correct JSON string escaping (safe for all input values).
func makeRuntimeEnvHandler(cfg FrontendConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(cfg)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		if _, err = fmt.Fprintf(w, "window.__APP_CONFIG__ = %s;\n", data); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
	}
}

// makeSPAHandler serves static files from uiDir. For any path that is not a real
// file it falls back to index.html so that client-side routing works correctly.
// Hashed assets under /assets/ receive immutable cache headers.
func makeSPAHandler(uiDir string) http.Handler {
	fs := http.Dir(uiDir)
	fileServer := http.FileServer(fs)
	indexPath := filepath.Join(uiDir, "index.html")

	// Cache index.html in memory to avoid disk I/O on every SPA route fallback.
	indexContent, _ := os.ReadFile(indexPath)
	var modTime time.Time
	if stat, err := os.Stat(indexPath); err == nil {
		modTime = stat.ModTime()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		f, err := fs.Open(path)
		if err == nil {
			stat, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !stat.IsDir() {
				if strings.HasPrefix(path, "/assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else if path == "/index.html" {
					w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// /api/ misses must return a structured error, not HTML, to avoid confusing JSON parse failures in callers.
		if strings.HasPrefix(path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Non-GET/HEAD requests to unmatched routes are not SPA navigations; serving HTML would confuse REST clients.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if indexContent == nil {
			http.NotFound(w, r)
			return
		}

		// http.ServeContent avoids the redirect that http.ServeFile adds when the
		// URL path does not match the file name being served.
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
		http.ServeContent(w, r, "index.html", modTime, bytes.NewReader(indexContent))
	})
}
