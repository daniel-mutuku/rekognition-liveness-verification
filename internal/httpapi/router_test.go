package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var discard = slog.New(slog.DiscardHandler)

func TestRouter(t *testing.T) {
	router := newTestAPI(t).router

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string // exact body; empty = don't check
	}{
		{"healthz", http.MethodGet, "/healthz", http.StatusOK, `{"status":"ok"}` + "\n"},
		{"HEAD matches GET pattern", http.MethodHead, "/healthz", http.StatusOK, ""},
		{"wrong method", http.MethodPost, "/healthz", http.StatusMethodNotAllowed, ""},
		{"unknown path", http.MethodGet, "/nope", http.StatusNotFound, ""},
		{"unknown API path is JSON", http.MethodGet, "/api/nope", http.StatusNotFound,
			`{"error":"no such API endpoint","code":"not_found"}` + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && rr.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rr.Body.String(), tt.wantBody)
			}
			if tt.wantStatus == http.StatusMethodNotAllowed && rr.Header().Get("Allow") == "" {
				t.Error("405 response should carry an Allow header")
			}
		})
	}
}

func TestHealthzContentType(t *testing.T) {
	rr := httptest.NewRecorder()
	newTestAPI(t).router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Run("encodes value with status", func(t *testing.T) {
		rr := httptest.NewRecorder()
		writeJSON(rr, http.StatusCreated, map[string]int{"n": 1})

		if rr.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201", rr.Code)
		}
		if got := rr.Body.String(); got != `{"n":1}`+"\n" {
			t.Errorf("body = %q", got)
		}
	})

	t.Run("unencodable value becomes 500", func(t *testing.T) {
		rr := httptest.NewRecorder()
		writeJSON(rr, http.StatusOK, make(chan int))

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rr.Code)
		}
		assertErrorBody(t, rr, "internal error")
	})
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusUnprocessableEntity, "no_face", "no face found")

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rr.Code)
	}
	assertErrorBody(t, rr, "no face found")
}

func TestRecoverPanics(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := recoverPanics(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	assertErrorBody(t, rr, "internal error")
	if !strings.Contains(logs.String(), `"error":"boom"`) || !strings.Contains(logs.String(), `"stack"`) {
		t.Errorf("panic not logged with error and stack:\n%s", logs.String())
	}
}

func TestLogRequests(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := logRequests(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/brew", nil))

	var entry struct {
		Msg      string `json:"msg"`
		Method   string `json:"method"`
		Path     string `json:"path"`
		Status   int    `json:"status"`
		Duration *int64 `json:"duration"`
	}
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, logs.String())
	}
	if entry.Msg != "request" || entry.Method != "POST" || entry.Path != "/brew" ||
		entry.Status != http.StatusTeapot || entry.Duration == nil {
		t.Errorf("unexpected log entry: %s", logs.String())
	}
}

func TestLogRequestsImplicit200(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := logRequests(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(logs.String(), `"status":200`) {
		t.Errorf("want status 200 in log:\n%s", logs.String())
	}
}

func assertErrorBody(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rr.Body.String())
	}
	if body.Error != want {
		t.Errorf("error = %q, want %q", body.Error, want)
	}
}
