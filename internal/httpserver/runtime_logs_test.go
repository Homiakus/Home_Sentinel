package httpserver

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/app"
	"github.com/Homiakus/Home_Sentinel/internal/config"
	"github.com/Homiakus/Home_Sentinel/internal/telemetry"
)

func TestEnableRuntimeLogsInjectsSettingsUIAndProtectsAPI(t *testing.T) {
	a, err := app.New(config.Default(), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	s := New(a)
	logs := telemetry.NewLogBuffer(16, 1024)
	_, _ = logs.Write([]byte("runtime ready\n"))
	s.EnableRuntimeLogs(logs)

	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(indexResp, indexReq)
	if indexResp.Code != http.StatusOK {
		t.Fatalf("index status=%d body=%s", indexResp.Code, indexResp.Body.String())
	}
	if !strings.Contains(indexResp.Body.String(), "/assets/settings-logs.js") {
		t.Fatal("settings log UI script was not injected")
	}

	logsReq := httptest.NewRequest(http.MethodGet, "/api/v1/system/logs?limit=10", nil)
	logsResp := httptest.NewRecorder()
	s.Handler().ServeHTTP(logsResp, logsReq)
	if logsResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("unauthenticated log API status=%d body=%s", logsResp.Code, logsResp.Body.String())
	}
	if !strings.Contains(logsResp.Body.String(), "AUTH_NOT_READY") {
		t.Fatalf("expected protected log endpoint, body=%s", logsResp.Body.String())
	}
}
