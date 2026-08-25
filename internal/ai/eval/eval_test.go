package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Homiakus/Home_Sentinel/internal/ai"
)

type fakeProvider struct{}

func (fakeProvider) Health(context.Context) ai.Health           { return ai.Health{Reachable: true} }
func (fakeProvider) Models(context.Context) ([]ai.Model, error) { return nil, nil }
func (fakeProvider) Analyze(context.Context, ai.AnalysisRequest) (ai.AnalysisResult, error) {
	return ai.AnalysisResult{Summary: "person approaches door", Activity: "approach", Persons: 1, Risk: ai.RiskLow, Confidence: .9}, nil
}

func TestRun(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "x.jpg"), []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Cases: []Case{{ID: "case1", CameraID: "front", FrameFiles: []string{"x.jpg"}, ExpectedActivity: "approach", ExpectedRisk: ai.RiskLow}}}
	r := Run(context.Background(), fakeProvider{}, m, d)
	if r.Total != 1 || r.Passed != 1 || r.SchemaValid != 1 {
		t.Fatalf("report=%+v", r)
	}
}
