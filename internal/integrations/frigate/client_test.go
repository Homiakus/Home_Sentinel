package frigate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientVersionConfigAndErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`"0.16.2"`)) })
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"cameras":{}}`)) })
	mux.HandleFunc("/api/config/save", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("save_option") != "saveonly" {
			t.Errorf("missing saveonly")
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("missing auth")
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/events/bad", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "missing", http.StatusNotFound) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := NewClient(ClientOptions{BaseURL: srv.URL, BearerToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	v, err := c.Version(context.Background())
	if err != nil || v != "0.16.2" {
		t.Fatalf("version=%q err=%v", v, err)
	}
	cfg, err := c.Config(context.Background())
	if err != nil || cfg["cameras"] == nil {
		t.Fatalf("config=%v err=%v", cfg, err)
	}
	if err := c.SaveConfig(context.Background(), []byte(`{"cameras":{}}`), true); err != nil {
		t.Fatal(err)
	}
	_, err = c.Event(context.Background(), "bad")
	var he *HTTPError
	if !errors.As(err, &he) || he.StatusCode != 404 {
		t.Fatalf("expected HTTPError: %v", err)
	}
}

func TestClientNetworkError(t *testing.T) {
	c, err := NewClient(ClientOptions{BaseURL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Version(context.Background())
	var ne *NetworkError
	if !errors.As(err, &ne) {
		t.Fatalf("expected network error: %T %v", err, err)
	}
}

func TestOpenMediaAllowlistAndStreaming(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/cam_test/latest.jpg", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing bearer token")
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpeg"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := NewClient(ClientOptions{BaseURL: srv.URL, BearerToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.OpenMedia(context.Background(), "cam_test/latest.jpg", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for _, path := range []string{"../config", "events/bad/id/snapshot.jpg", "events/a/../../config", "http://other/evil"} {
		if _, err := c.OpenMedia(context.Background(), path, nil); err == nil {
			t.Fatalf("unsafe media path accepted: %q", path)
		}
	}
}
