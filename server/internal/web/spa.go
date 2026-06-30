package web

import (
	"io/fs"
	"net/http"
	"os"
	"strings"
)

// SPAHandler serves the built client from dir at runtime (real files, else
// index.html for client-side deep links). The server does NOT embed the client —
// they are separate projects combined only in the build/runtime image (the
// Dockerfile copies the client's dist next to the binary and sets CLIENT_DIR).
// With no dir (local API-only dev), it returns a short notice; run the client
// under Vite separately.
func SPAHandler(dir string) http.Handler {
	if dir == "" {
		return apiOnly()
	}
	root := os.DirFS(dir)
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return apiOnly()
	}
	fileServer := http.FileServer(http.FS(root))
	index, _ := fs.ReadFile(root, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := strings.TrimPrefix(r.URL.Path, "/"); p != "" {
			if f, err := root.Open(p); err == nil {
				_ = f.Close()
				// Vite emits content-hashed filenames under /assets, so they are
				// safe to cache forever; everything else stays fresh.
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// index.html must not be cached, so new deploys are picked up immediately.
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}

func apiOnly() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Calories API. In dev, run the client separately: pnpm --dir client dev"))
	})
}
