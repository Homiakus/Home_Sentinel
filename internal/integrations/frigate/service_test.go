package frigate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/cameras"
	"github.com/Homiakus/Home_Sentinel/internal/database"
	"github.com/Homiakus/Home_Sentinel/internal/hardware"
	"github.com/Homiakus/Home_Sentinel/internal/locks"
	"github.com/Homiakus/Home_Sentinel/internal/repository"
)

func TestServicePlanApplyAndReconcile(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, database.Options{Path: filepath.Join(t.TempDir(), "frigate-service.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	current := map[string]any{"manual": map[string]any{"keep": true}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`"0.16-test"`)) })
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewEncoder(w).Encode(current)
	})
	mux.HandleFunc("/api/config/schema.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"type":"object"}`)) })
	mux.HandleFunc("/api/go2rtc/streams", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	mux.HandleFunc("/api/config/save", func(w http.ResponseWriter, r *http.Request) {
		var next map[string]any
		if err := json.NewDecoder(r.Body).Decode(&next); err != nil {
			http.Error(w, "bad", 400)
			return
		}
		mu.Lock()
		current = next
		mu.Unlock()
		_, _ = w.Write([]byte(`{"success":true}`))
	})
	mux.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"success":true}`)) })
	server := httptest.NewServer(mux)
	defer server.Close()
	client, _ := NewClient(ClientOptions{BaseURL: server.URL})
	svc := &Service{Client: client, Cameras: &cameras.Service{Store: repository.NewStore[cameras.Camera](db, repository.KindCamera)}, Hardware: hardware.Recommendation{}, State: repository.NewStore[AppliedState](db, repository.KindIntegration), Locks: locks.New(), SecretSink: CredentialDirectorySink{Dir: filepath.Join(t.TempDir(), "credentials")}}
	plan, err := svc.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Preflight.Valid {
		t.Fatal("preflight invalid")
	}
	res, err := svc.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied {
		t.Fatalf("result=%+v", res)
	}
	report, err := svc.Reconcile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !report.InSync {
		t.Fatalf("drift=%+v", report.Drift)
	}
	mu.Lock()
	_, manual := current["manual"]
	mu.Unlock()
	if !manual {
		t.Fatal("manual Frigate section was lost")
	}
}
