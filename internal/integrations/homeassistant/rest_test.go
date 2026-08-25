package homeassistant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRESTClientAuthStatesAndAllowlist(t *testing.T) {
	var called string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/":
			_ = json.NewEncoder(w).Encode(APIInfo{Message: "API running."})
		case "/api/config":
			_ = json.NewEncoder(w).Encode(ConfigInfo{Version: "2026.8.1", Components: []string{"mqtt", "frigate"}})
		case "/api/states/sensor.home_sentinel":
			_ = json.NewEncoder(w).Encode(State{EntityID: "sensor.home_sentinel", State: "online"})
		case "/api/services/light/turn_on":
			called = "light.turn_on"
			_ = json.NewEncoder(w).Encode([]State{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c, err := NewRESTClient(RESTOptions{BaseURL: srv.URL, Token: "test-token", AllowedActions: []Action{{Domain: "light", Service: "turn_on"}}})
	if err != nil {
		t.Fatal(err)
	}
	if info, err := c.Ping(context.Background()); err != nil || info.Message == "" {
		t.Fatalf("ping=%+v err=%v", info, err)
	}
	cfg, err := c.Config(context.Background())
	if err != nil || !HasComponent(cfg, "mqtt") {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
	if _, err := c.State(context.Background(), "sensor.home_sentinel"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CallAction(context.Background(), Action{Domain: "light", Service: "turn_off"}, nil); err == nil {
		t.Fatal("expected allowlist rejection")
	}
	if _, err := c.CallAction(context.Background(), Action{Domain: "light", Service: "turn_on"}, map[string]any{"entity_id": "light.entry"}); err != nil {
		t.Fatal(err)
	}
	if called != "light.turn_on" {
		t.Fatalf("called=%s", called)
	}
}

func TestRESTClientRejectsInvalidEntity(t *testing.T) {
	c, err := NewRESTClient(RESTOptions{BaseURL: "http://127.0.0.1:8123", Token: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.State(context.Background(), "../../api/config"); err == nil {
		t.Fatal("expected invalid entity id")
	}
}
