package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/ai"
)

func TestClientModelsAndStructuredVision(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tags", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"name": "vision:8b", "size": 123, "digest": "abc", "details": map[string]any{"family": "vision", "parameter_size": "8B", "quantization_level": "Q4"}}}})
	})
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatal(err)
		}
		if in["stream"] != false {
			t.Fatal("stream must be false")
		}
		if _, ok := in["format"].(map[string]any); !ok {
			t.Fatalf("format=%T", in["format"])
		}
		json.NewEncoder(w).Encode(map[string]any{"model": "vision:8b", "message": map[string]any{"content": `{"summary":"A person approaches the door","activity":"approach","persons":1,"vehicle_present":false,"package_present":false,"risk":"low","confidence":0.91}`}, "total_duration": int64(time.Second), "load_duration": int64(100 * time.Millisecond), "prompt_eval_count": 50, "eval_count": 20})
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"version": "test"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c, err := New(Options{BaseURL: srv.URL, Model: "vision:8b"})
	if err != nil {
		t.Fatal(err)
	}
	if !c.Health(context.Background()).Reachable {
		t.Fatal("health false")
	}
	models, err := c.Models(context.Background())
	if err != nil || len(models) != 1 {
		t.Fatalf("models=%v err=%v", models, err)
	}
	res, err := c.Analyze(context.Background(), ai.AnalysisRequest{Frames: []ai.Frame{{JPEG: []byte("fake-jpeg")}}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Persons != 1 || res.Risk != ai.RiskLow || res.Inference != 900*time.Millisecond {
		t.Fatalf("result=%+v", res)
	}
}
