package api

import (
	"encoding/json"
	"net/http"
)

// errorBody is the structured JSON error shape returned to clients: an HTTP
// status plus a human-readable message.
type errorBody struct {
	Error  string `json:"error"`
	Status string `json:"status,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	msg := "unexpected error"
	if err != nil {
		msg = err.Error()
	}
	writeJSON(w, code, errorBody{Error: msg, Status: http.StatusText(code)})
}
