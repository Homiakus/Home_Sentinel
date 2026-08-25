package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/Homiakus/Home_Sentinel/internal/ai"
)

type Case struct {
	ID               string   `json:"id"`
	CameraID         string   `json:"camera_id"`
	FrameFiles       []string `json:"frame_files"`
	ExpectedActivity string   `json:"expected_activity,omitempty"`
	ExpectedRisk     ai.Risk  `json:"expected_risk,omitempty"`
}
type Manifest struct {
	Cases []Case `json:"cases"`
}
type CaseResult struct {
	ID            string            `json:"id"`
	Passed        bool              `json:"passed"`
	SchemaValid   bool              `json:"schema_valid"`
	ActivityMatch bool              `json:"activity_match,omitempty"`
	RiskMatch     bool              `json:"risk_match,omitempty"`
	Latency       time.Duration     `json:"latency"`
	Error         string            `json:"error,omitempty"`
	Analysis      ai.AnalysisResult `json:"analysis,omitempty"`
}
type Report struct {
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
	Total       int          `json:"total"`
	Passed      int          `json:"passed"`
	SchemaValid int          `json:"schema_valid"`
	Cases       []CaseResult `json:"cases"`
}

func LoadManifest(path string) (Manifest, string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, "", err
	}
	var m Manifest
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, "", err
	}
	if len(m.Cases) == 0 {
		return Manifest{}, "", errors.New("evaluation manifest has no cases")
	}
	return m, filepath.Dir(path), nil
}

func Run(ctx context.Context, provider ai.Provider, m Manifest, baseDir string) Report {
	report := Report{StartedAt: time.Now().UTC(), Total: len(m.Cases), Cases: make([]CaseResult, 0, len(m.Cases))}
	for _, c := range m.Cases {
		r := CaseResult{ID: c.ID}
		if c.ID == "" || len(c.FrameFiles) == 0 {
			r.Error = "case id and frame_files required"
			report.Cases = append(report.Cases, r)
			continue
		}
		frames := make([]ai.Frame, 0, len(c.FrameFiles))
		for i, name := range c.FrameFiles {
			path := name
			if !filepath.IsAbs(path) {
				path = filepath.Join(baseDir, filepath.FromSlash(name))
			}
			b, err := os.ReadFile(path)
			if err != nil {
				r.Error = err.Error()
				frames = nil
				break
			}
			frames = append(frames, ai.Frame{JPEG: b, Timestamp: time.Unix(int64(i), 0).UTC(), Score: 1})
		}
		if frames == nil {
			report.Cases = append(report.Cases, r)
			continue
		}
		start := time.Now()
		analysis, err := provider.Analyze(ctx, ai.AnalysisRequest{EventID: c.ID, CameraID: c.CameraID, Frames: frames})
		r.Latency = time.Since(start)
		r.Analysis = analysis
		if err != nil {
			r.Error = err.Error()
			report.Cases = append(report.Cases, r)
			continue
		}
		if err := analysis.Validate(); err == nil {
			r.SchemaValid = true
			report.SchemaValid++
		} else {
			r.Error = err.Error()
		}
		r.ActivityMatch = c.ExpectedActivity == "" || analysis.Activity == c.ExpectedActivity
		r.RiskMatch = c.ExpectedRisk == "" || analysis.Risk == c.ExpectedRisk
		r.Passed = r.SchemaValid && r.ActivityMatch && r.RiskMatch
		if r.Passed {
			report.Passed++
		}
		report.Cases = append(report.Cases, r)
	}
	report.FinishedAt = time.Now().UTC()
	return report
}
