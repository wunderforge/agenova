// Copyright 2026 Agenova contributors.
// SPDX-License-Identifier: Apache-2.0

// Package landing serves only reviewed, public product material. It has no
// authentication, application API, filesystem browser or evidence-store access.
package landing

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type asset struct{ file, mime string }

var files = map[string]asset{
	"/":                                 {"index.html", "text/html; charset=utf-8"},
	"/index.html":                       {"index.html", "text/html; charset=utf-8"},
	"/landing/":                         {"index.html", "text/html; charset=utf-8"},
	"/landing/index.html":               {"index.html", "text/html; charset=utf-8"},
	"/landing/styles.css":               {"styles.css", "text/css; charset=utf-8"},
	"/landing/app.js":                   {"app.js", "text/javascript; charset=utf-8"},
	"/landing/assets/work-explorer.png": {"assets/work-explorer.png", "image/png"},
	"/landing/assets/work-evidence.png": {"assets/work-evidence.png", "image/png"},
}

// New uses the installed Portal build directory, never a request-supplied path.
// Exact asset names are intentional: adding a file cannot silently publish it.
func New(portalDirectory string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'none'; form-action 'none'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		item, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		f, err := os.Open(filepath.Join(portalDirectory, "landing", filepath.FromSlash(item.file)))
		if err != nil {
			http.Error(w, "Homepage unavailable.", http.StatusServiceUnavailable)
			return
		}
		defer f.Close()
		const limit = 8 << 20
		data, err := io.ReadAll(io.LimitReader(f, limit+1))
		if err != nil || len(data) > limit {
			http.Error(w, "Homepage unavailable.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", item.mime)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(data)
		}
	})
}
