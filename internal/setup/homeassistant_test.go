package setup

import (
	"context"
	"encoding/json"
	ha "github.com/Homiakus/Home_Sentinel/internal/integrations/homeassistant"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeAssistantProbeExplainsAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", http.StatusUnauthorized) }))
	defer srv.Close()
	d := (&HomeAssistantSetup{}).Probe(context.Background(), srv.URL, "bad")
	if d.OK || d.Code != "AUTH_FAILED" {
		t.Fatalf("diag=%+v", d)
	}
}
func TestHomeAssistantProbeSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_ = json.NewEncoder(w).Encode(ha.APIInfo{Message: "API running."})
		case "/api/config":
			_ = json.NewEncoder(w).Encode(ha.ConfigInfo{Version: "2026.8.1"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	d := (&HomeAssistantSetup{}).Probe(context.Background(), srv.URL, "good")
	if !d.OK || d.Version != "2026.8.1" {
		t.Fatalf("diag=%+v", d)
	}
}
