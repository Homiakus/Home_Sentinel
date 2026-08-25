package homeassistant

import (
	"context"
	"encoding/json"
	"github.com/Homiakus/Home_Sentinel/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActionBridgeOnlyNamedBinding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode([]State{}) }))
	defer srv.Close()
	a := Action{Domain: "light", Service: "turn_on"}
	rest, err := NewRESTClient(RESTOptions{BaseURL: srv.URL, Token: "x", AllowedActions: []Action{a}})
	if err != nil {
		t.Fatal(err)
	}
	bridge, err := NewActionBridge(rest, []ActionBinding{{Name: "entry_light_on", Action: a, Payload: map[string]any{"entity_id": "light.entry"}}})
	if err != nil {
		t.Fatal(err)
	}
	cor, _ := domain.NewID("cor")
	if _, err := bridge.Execute(context.Background(), "arbitrary", "admin", cor); err == nil {
		t.Fatal("expected unauthorized binding")
	}
	if _, err := bridge.Execute(context.Background(), "entry_light_on", "admin", cor); err != nil {
		t.Fatal(err)
	}
}
