package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/config"
)

func TestEndpoints(t *testing.T) {
	a, err := app.New(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	s := New(a)
	for _, path := range []string{"/healthz", "/readyz", "/api/v1/system", "/"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.http.Handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: got %d", path, w.Code)
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s: missing request id", path)
		}
	}
}
