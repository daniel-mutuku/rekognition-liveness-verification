// Package httpapi exposes verify.Service over HTTP and serves the web app.
package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

// NewRouter returns the root handler. web is served at /; nil means API only.
func NewRouter(logger *slog.Logger, svc *verify.Service, web http.Handler) http.Handler {
	mux := http.NewServeMux()
	a := &api{logger: logger, svc: svc}

	mux.HandleFunc("GET /healthz", handleHealthz)

	mux.HandleFunc("POST /api/verifications", a.create)
	mux.HandleFunc("GET /api/verifications/{id}", a.withID(a.get))
	mux.HandleFunc("POST /api/verifications/{id}/photo", a.withID(a.checkPhoto))
	mux.HandleFunc("POST /api/verifications/{id}/liveness-session", a.withID(a.startLiveness))
	mux.HandleFunc("POST /api/verifications/{id}/complete", a.withID(a.complete))

	// Keeps unknown API paths out of the SPA fallback.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})

	// No method in the pattern: "GET /" conflicts with "/api/" and ServeMux
	// panics. NewSPA checks the method itself.
	if web != nil {
		mux.Handle("/", web)
	}

	// Logging is outermost so it records the 500 from a recovered panic.
	return logRequests(logger, recoverPanics(logger, mux))
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
