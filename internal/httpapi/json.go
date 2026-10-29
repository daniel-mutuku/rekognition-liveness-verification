package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/daniel-mutuku/rekognition-liveness-verification/internal/verify"
)

// errorBody: Error is for people, Code is what clients switch on.
type errorBody struct {
	Error   string          `json:"error"`
	Code    string          `json:"code"`
	Reasons []verify.Reason `json:"reasons,omitempty"`
}

// writeJSON marshals before writing headers, so an encoding failure can still
// become a 500.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		status = http.StatusInternalServerError
		body = []byte(`{"error":"internal error","code":"internal"}`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(append(body, '\n'))
}

// writeError's msg reaches the client, so it must not contain internal details.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorBody{Error: msg, Code: code})
}
