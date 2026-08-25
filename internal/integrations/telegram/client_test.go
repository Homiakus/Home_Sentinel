package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientGetMeAndUpdates(t *testing.T) {
	const token = "123:secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/bot"+token+"/") {
			t.Fatalf("path=%s", r.URL.Path)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": map[string]any{"id": 1, "is_bot": true, "first_name": "Sentinel", "username": "sentinel_bot"}})
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": []any{map[string]any{"update_id": 5, "message": map[string]any{"message_id": 1, "from": map[string]any{"id": 42, "is_bot": false, "first_name": "A"}, "chat": map[string]any{"id": 42, "type": "private"}, "text": "/status"}}}})
		default:
			t.Fatalf("method=%s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c, err := New(Options{Token: token, BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	me, err := c.GetMe(context.Background())
	if err != nil || me.Username != "sentinel_bot" {
		t.Fatalf("me=%+v err=%v", me, err)
	}
	u, err := c.GetUpdates(context.Background(), 0, 1)
	if err != nil || len(u) != 1 || u[0].UpdateID != 5 {
		t.Fatalf("updates=%+v err=%v", u, err)
	}
}
func TestClientErrorDoesNotExposeToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(map[string]any{"ok": false, "error_code": 500, "description": "server error"})
	}))
	defer srv.Close()
	c, _ := New(Options{Token: "999:supersecret", BaseURL: srv.URL})
	_, err := c.GetMe(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Fatal("token leaked in error")
	}
}
