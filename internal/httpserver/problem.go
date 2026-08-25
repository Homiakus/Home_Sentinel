package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type Problem struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Details   any    `json:"details,omitempty"`
}

func requestIDFrom(ctx context.Context) string { v, _ := ctx.Value(requestIDKey).(string); return v }
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, Problem{Code: code, Message: message, RequestID: requestIDFrom(r.Context())})
}
