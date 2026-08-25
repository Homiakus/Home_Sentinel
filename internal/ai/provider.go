package ai

import (
	"context"
	"errors"
	"time"
)

type Health struct {
	Reachable bool   `json:"reachable"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

type Model struct {
	Name              string    `json:"name"`
	SizeBytes         int64     `json:"size_bytes,omitempty"`
	Digest            string    `json:"digest,omitempty"`
	ModifiedAt        time.Time `json:"modified_at,omitempty"`
	Family            string    `json:"family,omitempty"`
	ParameterSize     string    `json:"parameter_size,omitempty"`
	QuantizationLevel string    `json:"quantization_level,omitempty"`
}

type Frame struct {
	JPEG      []byte    `json:"-"`
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score,omitempty"`
}

type AnalysisRequest struct {
	EventID  string  `json:"event_id"`
	CameraID string  `json:"camera_id"`
	Prompt   string  `json:"prompt,omitempty"`
	Frames   []Frame `json:"-"`
}

type Risk string

const (
	RiskUnknown Risk = "unknown"
	RiskLow     Risk = "low"
	RiskMedium  Risk = "medium"
	RiskHigh    Risk = "high"
)

type AnalysisResult struct {
	Summary         string        `json:"summary"`
	Activity        string        `json:"activity"`
	Persons         int           `json:"persons"`
	VehiclePresent  bool          `json:"vehicle_present"`
	PackagePresent  bool          `json:"package_present"`
	Risk            Risk          `json:"risk"`
	Confidence      float64       `json:"confidence"`
	RawDescription  string        `json:"raw_description,omitempty"`
	Model           string        `json:"model,omitempty"`
	QueueDuration   time.Duration `json:"queue_duration,omitempty"`
	LoadDuration    time.Duration `json:"load_duration,omitempty"`
	Inference       time.Duration `json:"inference_duration,omitempty"`
	TotalDuration   time.Duration `json:"total_duration,omitempty"`
	PromptTokens    int           `json:"prompt_tokens,omitempty"`
	GeneratedTokens int           `json:"generated_tokens,omitempty"`
}

func (r AnalysisResult) Validate() error {
	if len(r.Summary) == 0 || len(r.Summary) > 1000 {
		return errors.New("AI summary missing or too long")
	}
	if len(r.Activity) == 0 || len(r.Activity) > 128 {
		return errors.New("AI activity missing or too long")
	}
	if r.Persons < 0 || r.Persons > 100 {
		return errors.New("AI persons count out of range")
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return errors.New("AI confidence out of range")
	}
	switch r.Risk {
	case RiskUnknown, RiskLow, RiskMedium, RiskHigh:
	default:
		return errors.New("AI risk value invalid")
	}
	return nil
}

type Provider interface {
	Health(context.Context) Health
	Models(context.Context) ([]Model, error)
	Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}

type DisabledProvider struct{}

func (DisabledProvider) Health(context.Context) Health           { return Health{Error: "AI disabled"} }
func (DisabledProvider) Models(context.Context) ([]Model, error) { return nil, nil }
func (DisabledProvider) Analyze(context.Context, AnalysisRequest) (AnalysisResult, error) {
	return AnalysisResult{}, errors.New("AI disabled")
}
