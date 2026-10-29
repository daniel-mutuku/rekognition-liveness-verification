package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPA(t *testing.T) {
	files := fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html><title>app</title>")},
		"assets/index-abc12.js": {Data: []byte("console.log(1)")},
		"favicon.svg":           {Data: []byte("<svg/>")},
	}
	a := newTestAPI(t)
	router := NewRouter(discard, a.svc, NewSPA(files))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // substring
		wantCache  string
	}{
		{"root is index.html", "GET", "/", 200, "<title>app</title>", "no-cache"},
		{"refresh with ?id= is index.html", "GET", "/?id=4a29ec62-3ded-4d08-a513-d971536c5d17", 200, "<title>app</title>", "no-cache"},
		{"unknown page falls back to index.html", "GET", "/result", 200, "<title>app</title>", "no-cache"},
		{"hashed asset is cached for a year", "GET", "/assets/index-abc12.js", 200, "console.log", "public, max-age=31536000, immutable"},
		{"other files revalidate", "GET", "/favicon.svg", 200, "<svg/>", "no-cache"},
		{"missing asset is a real 404", "GET", "/assets/index-old99.js", 404, "", ""},
		{"API still wins over the fallback", "GET", "/api/nope", 404, `"code":"not_found"`, ""},
		{"health check still works", "GET", "/healthz", 200, `"ok"`, ""},
		{"writes are not allowed", "POST", "/", 405, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(tt.method, tt.path, nil))
			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d\n%s", rr.Code, tt.wantStatus, rr.Body)
			}
			if !strings.Contains(rr.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rr.Body, tt.wantBody)
			}
			if got := rr.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
		})
	}
}

func TestNoSPAWithoutWebDir(t *testing.T) {
	rr := httptest.NewRecorder()
	newTestAPI(t).router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("GET / without a web app = %d, want 404", rr.Code)
	}
}
