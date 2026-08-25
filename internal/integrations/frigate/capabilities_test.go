package frigate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeCapabilitiesActionable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`"0.16.0"`)) })
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	mux.HandleFunc("/api/go2rtc/streams", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, _ := NewClient(ClientOptions{BaseURL: srv.URL})
	caps, diag, err := ProbeCapabilities(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	if caps.Go2RTCStreams || diag.Compatible || len(diag.Reasons) == 0 {
		t.Fatalf("caps=%+v diag=%+v", caps, diag)
	}
}
