package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewSPA serves the built web app. Unknown extensionless paths get index.html;
// missing assets are a real 404. Hashed assets/ are immutable, everything else
// is revalidated.
func NewSPA(files fs.FS) http.Handler {
	fileServer := http.FileServerFS(files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		info, err := fs.Stat(files, name)
		switch {
		case err == nil && !info.IsDir():
			if strings.HasPrefix(name, "assets/") {
				h.Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				h.Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
		case path.Ext(name) != "":
			http.NotFound(w, r)
		default:
			h.Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, files, "index.html")
		}
	})
}
